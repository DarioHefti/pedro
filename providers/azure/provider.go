package azure

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azcache "github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"pedro/shared"
	"pedro/tools"
)

type AzureConfig struct {
	Endpoint   string
	Deployment string
	APIVersion string
}

func (c AzureConfig) Type() string {
	return "azure"
}

func (c AzureConfig) Validate() error {
	if c.Endpoint == "" {
		return errors.New("azure endpoint is required")
	}
	if c.Deployment == "" {
		return errors.New("azure deployment is required")
	}
	return nil
}

const cognitiveServicesScope = "https://cognitiveservices.azure.com/.default"

const DefaultAPIVersion = "2024-12-01-preview"

type Provider struct {
	client             openai.Client
	credential         *azidentity.InteractiveBrowserCredential
	config             AzureConfig
	registry           *tools.Registry
	customSystemPrompt string
	authenticated      bool
}

func (p *Provider) SetAuthenticated(auth bool) {
	// With automatic auth, this is just a hint for UI state
	p.authenticated = auth
}

type Builder struct{}

var (
	sharedCredential *azidentity.InteractiveBrowserCredential
	credentialOnce   sync.Once
	credentialErr    error
)

func getOrCreateCredential() (*azidentity.InteractiveBrowserCredential, error) {
	credentialOnce.Do(func() {
		opts := &azidentity.InteractiveBrowserCredentialOptions{
			// Let SDK handle auth automatically - browser opens when needed,
			// tokens refresh silently when cached.
			DisableAutomaticAuthentication: false,
		}

		// Persistent cache with app-specific name for token storage
		persistentCache, err := azcache.New(&azcache.Options{
			Name: "pedro-azure-token-cache",
		})
		if err != nil {
			fmt.Printf("Warning: failed to create persistent cache: %v\n", err)
		} else {
			opts.Cache = persistentCache
		}

		sharedCredential, credentialErr = azidentity.NewInteractiveBrowserCredential(opts)
	})
	return sharedCredential, credentialErr
}

func (Builder) Build(registry *tools.Registry) (shared.LLMClient, error) {
	cred, err := getOrCreateCredential()
	if err != nil {
		return nil, fmt.Errorf("creating browser credential: %w", err)
	}

	return &Provider{
		client:     openai.Client{},
		credential: cred,
		registry:   registry,
	}, nil
}

func ParseConfig(settings map[string]string) (shared.Config, error) {
	cfg := AzureConfig{
		Endpoint:   settings["azure_endpoint"],
		Deployment: settings["azure_deployment"],
		APIVersion: settings["azure_api_version"],
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	return cfg, nil
}

func (p *Provider) Name() string {
	return "azure"
}

func (p *Provider) SetConfig(cfg AzureConfig) {
	p.config = cfg

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	p.client = openai.NewClient(
		azure.WithEndpoint(cfg.Endpoint, apiVersion),
		azure.WithTokenCredential(p.credential),
	)
}

func (p *Provider) SignIn(ctx context.Context) error {
	// Authenticate interactively - this opens the browser
	record, err := p.credential.Authenticate(ctx, &policy.TokenRequestOptions{
		Scopes: []string{cognitiveServicesScope},
	})
	if err != nil {
		return fmt.Errorf("sign in failed: %w", err)
	}
	fmt.Printf("SignIn: authenticated as %s\n", record.Username)

	// Verify the token is usable by acquiring it again (should be silent now)
	token, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{cognitiveServicesScope},
	})
	if err != nil {
		return fmt.Errorf("token verification failed: %w", err)
	}
	fmt.Printf("SignIn: token acquired, expires at %v\n", token.ExpiresOn)

	p.authenticated = true
	return nil
}

func (p *Provider) SignOut() error {
	p.authenticated = false
	return nil
}

func (p *Provider) IsAuthenticated() bool {
	return p.authenticated
}

func (p *Provider) SetCustomSystemPrompt(prompt string) {
	p.customSystemPrompt = prompt
}

func (p *Provider) getFullSystemPrompt(systemPrompt string) string {
	if p.customSystemPrompt == "" {
		return systemPrompt
	}
	return systemPrompt + "\n\n## Additional Instructions\n" + p.customSystemPrompt
}

func (p *Provider) toolDefinitions() []openai.ChatCompletionToolParam {
	if p.registry == nil {
		return nil
	}
	var result []openai.ChatCompletionToolParam
	for _, def := range p.registry.Definitions() {
		result = append(result, openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        def.Name,
				Description: openai.String(def.Description),
				Parameters:  openai.FunctionParameters(def.Parameters),
			},
		})
	}
	return result
}

func (p *Provider) buildInitialMessages(messages []shared.Message, imageDataURLs []string, systemPrompt string) []openai.ChatCompletionMessageParamUnion {
	result := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(p.getFullSystemPrompt(systemPrompt)),
	}

	totalUsers := 0
	for _, m := range messages {
		if m.Role == "user" {
			totalUsers++
		}
	}

	userCount := 0
	for _, m := range messages {
		switch m.Role {
		case "user":
			userCount++
			isLastUser := userCount == totalUsers
			if isLastUser && len(imageDataURLs) > 0 {
				parts := []openai.ChatCompletionContentPartUnionParam{
					openai.TextContentPart(m.Content),
				}
				for _, img := range imageDataURLs {
					parts = append(parts, openai.ImageContentPart(
						openai.ChatCompletionContentPartImageImageURLParam{
							URL:    img,
							Detail: "auto",
						},
					))
				}
				result = append(result, openai.UserMessage(parts))
			} else {
				result = append(result, openai.UserMessage(m.Content))
			}
		case "assistant":
			result = append(result, openai.AssistantMessage(m.Content))
		}
	}

	return result
}

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	// No auth check needed - SDK will automatically prompt for login if needed

	apiMessages := p.buildInitialMessages(messages, imageDataURLs, "")
	toolDefs := p.toolDefinitions()

	for {
		params := openai.ChatCompletionNewParams{
			Model:    openai.ChatModel(p.config.Deployment),
			Messages: apiMessages,
			Tools:    toolDefs,
		}

		stream := p.client.Chat.Completions.NewStreaming(ctx, params)
		acc := openai.ChatCompletionAccumulator{}

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					onChunk(choice.Delta.Content)
				}
			}
		}

		if err := stream.Err(); err != nil {
			return fmt.Errorf("streaming error: %w", err)
		}

		if len(acc.Choices) == 0 {
			break
		}

		msg := acc.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			break
		}

		apiMessages = append(apiMessages, msg.ToParam())

		for _, tc := range msg.ToolCalls {
			if onToolCall != nil {
				onToolCall(tc.Function.Name, tc.Function.Arguments)
			}
			result := p.registry.Execute(tc.Function.Name, tc.Function.Arguments)
			apiMessages = append(apiMessages, openai.ToolMessage(result, tc.ID))
		}
	}

	return nil
}
