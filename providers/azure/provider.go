package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	azcache "github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
	"pedro/providers/openaiutil"
	"pedro/shared"
	"pedro/tools"
)

type Config struct {
	Endpoint   string
	Deployment string
	APIVersion string
	// TenantID is the Azure AD tenant for interactive login (optional).
	// Not the OpenAI resource URL — use Endpoint for that.
	TenantID string
}

func (c Config) Type() string { return "azure" }

func (c Config) Validate() error {
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

const authRecordKey = "azure_auth_record"
const authTenantBindKey = "azure_auth_tenant_id"
const authCacheNameKey = "azure_auth_cache_name"
const defaultAuthCacheName = "pedro-azure-token-cache"

// Shared credential singleton — survives provider re-creation so the
// in-memory token cache and persistent cache stay warm.
var (
	sharedCredential *azidentity.InteractiveBrowserCredential
	credentialMu     sync.Mutex
	credTenantKey    string // tenant option used to build sharedCredential; must match on reuse
)

func getOrCreateCredential(store shared.SettingsStore, tenantID string) (*azidentity.InteractiveBrowserCredential, error) {
	credentialMu.Lock()
	defer credentialMu.Unlock()

	if sharedCredential != nil && credTenantKey == tenantID {
		return sharedCredential, nil
	}
	sharedCredential = nil

	// false: HTTP clients (e.g. streaming) call GetToken without going through our
	// SignIn(); they must be allowed to trigger the browser when no silent token exists.
	opts := &azidentity.InteractiveBrowserCredentialOptions{
		DisableAutomaticAuthentication: false,
	}
	if tenantID != "" {
		opts.TenantID = tenantID
	}

	if cache, err := azcache.New(&azcache.Options{Name: authCacheName(store)}); err == nil {
		opts.Cache = cache
	}

	if store != nil {
		boundTenant, _ := store.GetSetting(authTenantBindKey)
		if boundTenant == tenantID {
			if data, err := store.GetSetting(authRecordKey); err == nil && data != "" {
				var record azidentity.AuthenticationRecord
				if err := json.Unmarshal([]byte(data), &record); err == nil {
					opts.AuthenticationRecord = record
				}
			}
		}
	}

	cred, err := azidentity.NewInteractiveBrowserCredential(opts)
	if err != nil {
		return nil, err
	}

	sharedCredential = cred
	credTenantKey = tenantID
	return sharedCredential, nil
}

func authCacheName(store shared.SettingsStore) string {
	if store == nil {
		return defaultAuthCacheName
	}
	if name, err := store.GetSetting(authCacheNameKey); err == nil && name != "" {
		return name
	}
	return defaultAuthCacheName
}

// ResetCredential forces recreation of credential (e.g. after sign out).
func ResetCredential() {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	sharedCredential = nil
	credTenantKey = ""
}

type Provider struct {
	client             openai.Client
	credential         *azidentity.InteractiveBrowserCredential
	config             Config
	registry           *tools.Registry
	store              shared.SettingsStore
	baseSystemPrompt   string
	customSystemPrompt string
	personaPrompt      string
	authenticated      bool
}

func (p *Provider) Name() string                    { return "azure" }
func (p *Provider) IsAuthenticated() bool            { return p.authenticated }
func (p *Provider) SetAuthenticated(a bool)          { p.authenticated = a }
func (p *Provider) SetBaseSystemPrompt(s string)     { p.baseSystemPrompt = s }
func (p *Provider) SetCustomSystemPrompt(s string)   { p.customSystemPrompt = s }
func (p *Provider) SetPersonaPrompt(s string)        { p.personaPrompt = s }

func ParseConfig(settings map[string]string) (shared.Config, error) {
	cfg := Config{
		Endpoint:   settings["azure_endpoint"],
		Deployment: settings["azure_deployment"],
		APIVersion: settings["azure_api_version"],
		TenantID:   strings.TrimSpace(settings["azure_tenant_id"]),
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = DefaultAPIVersion
	}
	return cfg, nil
}

// Build creates a fully-configured Azure-login provider.
func Build(cfg shared.Config, store shared.SettingsStore, registry *tools.Registry) (shared.LLMClient, error) {
	c, ok := cfg.(Config)
	if !ok {
		return nil, fmt.Errorf("azure: expected Config, got %T", cfg)
	}

	cred, err := getOrCreateCredential(store, c.TenantID)
	if err != nil {
		return nil, fmt.Errorf("creating browser credential: %w", err)
	}

	apiVersion := c.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	return &Provider{
		client: openai.NewClient(
			azure.WithEndpoint(c.Endpoint, apiVersion),
			azure.WithTokenCredential(cred),
		),
		credential: cred,
		config:     c,
		registry:   registry,
		store:      store,
	}, nil
}

func (p *Provider) SignIn(ctx context.Context) error {
	record, err := p.credential.Authenticate(ctx, &policy.TokenRequestOptions{
		Scopes: []string{cognitiveServicesScope},
	})
	if err != nil {
		return fmt.Errorf("sign in failed: %w", err)
	}

	if p.store != nil {
		if data, err := json.Marshal(record); err == nil {
			_ = p.store.SetSetting(authRecordKey, string(data))
			_ = p.store.SetSetting(authTenantBindKey, p.config.TenantID)
		}
	}

	_, err = p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{cognitiveServicesScope},
	})
	if err != nil {
		return fmt.Errorf("token verification failed: %w", err)
	}

	p.authenticated = true
	return nil
}

func (p *Provider) SignOut() error {
	p.authenticated = false
	if p.store != nil {
		_ = p.store.SetSetting(authRecordKey, "")
		_ = p.store.SetSetting(authTenantBindKey, "")
		_ = p.store.SetSetting(authCacheNameKey, fmt.Sprintf("%s-%d", defaultAuthCacheName, time.Now().UnixNano()))
	}
	ResetCredential()
	return nil
}

func (p *Provider) ensureAuthenticated(ctx context.Context) error {
	_, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{cognitiveServicesScope},
	})
	if err == nil {
		p.authenticated = true
		return nil
	}
	return p.SignIn(ctx)
}

func (p *Provider) Chat(ctx context.Context, messages []shared.Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error {
	if err := p.ensureAuthenticated(ctx); err != nil {
		return err
	}

	base := p.baseSystemPrompt
	if base == "" {
		base = shared.DefaultSystemPrompt
	}
	prompt := openaiutil.FullSystemPrompt(base, p.personaPrompt, p.customSystemPrompt)
	err := openaiutil.StreamingChat(ctx, p.client, p.config.Deployment, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall)
	if err == nil {
		return nil
	}

	// If streaming still fails with an interaction-required token error, sign in once and retry.
	if !requiresInteractiveAuth(err) {
		return err
	}
	if signInErr := p.SignIn(ctx); signInErr != nil {
		return fmt.Errorf("interactive authentication required: %w", signInErr)
	}
	return openaiutil.StreamingChat(ctx, p.client, p.config.Deployment, p.registry, messages, imageDataURLs, prompt, onChunk, onToolCall)
}

func requiresInteractiveAuth(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		msg := e.Error()
		if strings.Contains(msg, "InteractiveBrowserCredential can't acquire a token without user interaction") ||
			strings.Contains(msg, "Call Authenticate to authenticate a user interactively") {
			return true
		}
	}
	return false
}
