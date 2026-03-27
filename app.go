package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"pedro/providers"
	"pedro/shared"
	"pedro/tools"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	store      Store
	llm        providers.LLMClient
	registry   *tools.Registry
	factory    *providers.Factory
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc
}

func NewApp() *App {
	db, err := NewDatabase()
	if err != nil {
		fmt.Println("Database error:", err.Error())
		return &App{store: nil, registry: nil, factory: nil}
	}

	factory := providers.NewFactory()
	providers.RegisterProviders(factory)

	return &App{
		store:    db,
		registry: tools.New(),
		factory:  factory,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initLLM()
}

func (a *App) initLLM() {
	settings, err := a.store.GetSettings()
	if err != nil {
		fmt.Println("GetSettings error:", err.Error())
		return
	}

	providerType := settings["provider_type"]
	if providerType == "" {
		return
	}

	cfg, err := a.factory.ParseSettings(settings)
	if err != nil {
		fmt.Println("Failed to parse provider config:", err.Error())
		return
	}

	llm, err := a.factory.Create(cfg, a.store, a.registry)
	if err != nil {
		fmt.Println("LLM init error:", err.Error())
		return
	}

	if settings["authenticated"] == "true" {
		llm.SetAuthenticated(true)
	}

	if basePrompt, ok := settings["system_prompt"]; ok && basePrompt != "" {
		llm.SetBaseSystemPrompt(basePrompt)
	}

	if customPrompt, ok := settings["custom_system_prompt"]; ok {
		llm.SetCustomSystemPrompt(customPrompt)
	}

	a.llm = llm
}

func (a *App) runChat(conversationID int64, messages []Message, imageDataURLs []string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelMu.Lock()
	a.cancelFunc = cancel
	a.cancelMu.Unlock()

	defer func() {
		cancel()
		a.cancelMu.Lock()
		a.cancelFunc = nil
		a.cancelMu.Unlock()
	}()

	llmMessages := make([]providers.Message, len(messages))
	for i, m := range messages {
		llmMessages[i] = providers.Message{Role: m.Role, Content: m.Content}
	}

	var response []byte
	err := a.llm.Chat(
		ctx,
		llmMessages,
		imageDataURLs,
		func(chunk string) {
			response = append(response, chunk...)
			runtime.EventsEmit(a.ctx, "stream_chunk", conversationID, chunk)
		},
		func(name, argsJSON string) {
			runtime.EventsEmit(a.ctx, "tool_call", conversationID, name, argsJSON)
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return string(response), nil
		}
		return "", err
	}
	return string(response), nil
}

func (a *App) AbortMessage() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
}

func (a *App) requireStore() error {
	if a.store == nil {
		return errors.New("database not initialized")
	}
	return nil
}

func (a *App) requireLLMConfigured() error {
	if a.llm == nil {
		return errors.New("please configure LLM provider settings first")
	}
	return nil
}

func (a *App) sendMessage(conversationID int64, content string, imageDataURLs []string, selectedPersonaID string, attachmentsJSON string) string {
	if err := a.requireStore(); err != nil {
		return "Error: " + err.Error()
	}

	if _, err := a.store.AddMessage(conversationID, "user", content, attachmentsJSON); err != nil {
		return "Error: Failed to save message: " + err.Error()
	}
	runtime.EventsEmit(a.ctx, "conversation_updated", conversationID)

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if err := a.requireLLMConfigured(); err != nil {
		return "Error: " + err.Error()
	}

	a.llm.SetPersonaPrompt(a.personaPromptFromDB(selectedPersonaID))
	resp, err := a.runChat(conversationID, messages, imageDataURLs)
	if err != nil {
		return "Error: " + err.Error()
	}

	if _, saveErr := a.store.AddMessage(conversationID, "assistant", resp, ""); saveErr != nil {
		fmt.Println("Warning: Failed to save assistant message:", saveErr.Error())
	}
	return resp
}

func (a *App) SignIn() string {
	if a.llm == nil {
		return "Error: No LLM provider configured – save your settings first"
	}
	if err := a.llm.SignIn(context.Background()); err != nil {
		return "Error: " + err.Error()
	}
	_ = a.store.SetSetting("authenticated", "true")
	return ""
}

func (a *App) SignOut() error {
	if a.llm != nil {
		_ = a.llm.SignOut()
	}
	_ = a.store.SetSetting("authenticated", "false")
	return nil
}

func (a *App) IsAuthenticated() bool {
	if a.llm == nil {
		return false
	}
	return a.llm.IsAuthenticated()
}

func (a *App) GetConversations() []Conversation {
	if a.store == nil {
		return []Conversation{}
	}
	convs, err := a.store.GetConversations()
	if err != nil {
		fmt.Println("GetConversations error:", err.Error())
		return []Conversation{}
	}
	return convs
}

func (a *App) GetMessages(conversationID int64) []Message {
	if a.store == nil {
		return []Message{}
	}
	msgs, err := a.store.GetMessages(conversationID)
	if err != nil {
		fmt.Println("GetMessages error:", err.Error())
		return []Message{}
	}
	return msgs
}

func (a *App) SearchMessages(query string) map[int64][]Message {
	if a.store == nil || query == "" {
		return map[int64][]Message{}
	}
	result, err := a.store.SearchMessages(query)
	if err != nil {
		fmt.Println("SearchMessages error:", err.Error())
		return map[int64][]Message{}
	}
	return result
}

func (a *App) CreateConversation() *Conversation {
	if a.store == nil {
		return &Conversation{ID: 0, Title: "New Chat"}
	}
	conv, err := a.store.CreateConversation()
	if err != nil {
		fmt.Println("CreateConversation error:", err.Error())
		return &Conversation{ID: 0, Title: "New Chat"}
	}
	return conv
}

func (a *App) DeleteConversation(id int64) error {
	if a.store == nil {
		return nil
	}
	return a.store.DeleteConversation(id)
}

func (a *App) DeleteAllConversations() error {
	if a.store == nil {
		return nil
	}
	return a.store.DeleteAllConversations()
}

func (a *App) SendMessage(conversationID int64, content string, selectedPersonaID string, attachmentsJSON string) string {
	return a.sendMessage(conversationID, content, nil, selectedPersonaID, attachmentsJSON)
}

func (a *App) SendMessageWithImages(conversationID int64, content string, imageDataURLs []string, selectedPersonaID string, attachmentsJSON string) string {
	return a.sendMessage(conversationID, content, imageDataURLs, selectedPersonaID, attachmentsJSON)
}

func (a *App) RegenerateMessage(conversationID int64, selectedPersonaID string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	lastAssistantIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			lastAssistantIdx = i
			break
		}
	}

	if lastAssistantIdx == -1 {
		return "Error: No assistant message to regenerate"
	}

	if err := a.store.DeleteMessage(conversationID, lastAssistantIdx); err != nil {
		return "Error: Failed to delete message: " + err.Error()
	}

	messages, err = a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if a.llm == nil {
		return "Error: Please configure LLM provider settings first"
	}

	a.llm.SetPersonaPrompt(a.personaPromptFromDB(selectedPersonaID))
	resp, err := a.runChat(conversationID, messages, nil)
	if err != nil {
		return "Error: " + err.Error()
	}

	if _, saveErr := a.store.AddMessage(conversationID, "assistant", resp, ""); saveErr != nil {
		fmt.Println("Warning: Failed to save assistant message:", saveErr.Error())
	}
	return resp
}

func (a *App) GetSettings() map[string]string {
	if a.store == nil {
		return map[string]string{}
	}
	settings, err := a.store.GetSettings()
	if err != nil {
		fmt.Println("GetSettings error:", err.Error())
		return map[string]string{}
	}
	return settings
}

// GetDefaultSystemPrompt returns the built-in default system prompt so the frontend can offer "Reset to Default".
func (a *App) GetDefaultSystemPrompt() string {
	return shared.DefaultSystemPrompt
}

func (a *App) SaveSettings(settings map[string]string) error {
	if a.store == nil {
		return nil
	}

	if nextProvider, hasProvider := settings["provider_type"]; hasProvider {
		prevProvider, _ := a.store.GetSetting("provider_type")
		if prevProvider != "" && prevProvider != nextProvider {
			if a.llm != nil {
				_ = a.llm.SignOut()
			}
			_ = a.store.SetSetting("authenticated", "false")
		}
	}

	invalidateConnectionTest := false
	for key := range settings {
		if settingsKeyInvalidatesConnectionTest(key) {
			invalidateConnectionTest = true
			break
		}
	}

	for key, value := range settings {
		if err := a.store.SetSetting(key, value); err != nil {
			return err
		}
	}

	if invalidateConnectionTest {
		a.clearPersistedConnectionTest()
	}

	a.initLLM()
	return nil
}

func (a *App) SelectFile() string {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}

func (a *App) SelectFolder() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder",
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}

func (a *App) SetSetting(key, value string) error {
	if a.store == nil {
		return nil
	}
	if err := a.store.SetSetting(key, value); err != nil {
		return err
	}

	if settingsKeyInvalidatesConnectionTest(key) {
		a.clearPersistedConnectionTest()
	}

	if key == "system_prompt" && a.llm != nil {
		a.llm.SetBaseSystemPrompt(value)
	}

	if key == "custom_system_prompt" && a.llm != nil {
		a.llm.SetCustomSystemPrompt(value)
	}

	return nil
}

func (a *App) TestConnection() string {
	if a.llm == nil {
		return "Error: No LLM provider configured – save your settings first"
	}

	settings, err := a.store.GetSettings()
	if err != nil {
		settings = map[string]string{}
	}
	fp := connectionSettingsFingerprint(settings)

	if !a.llm.IsAuthenticated() {
		if err := a.llm.SignIn(context.Background()); err != nil {
			ret := "Error: Sign in failed: " + err.Error()
			a.persistConnectionTest(false, connectionTestFailureMessageForStore(ret), fp)
			return ret
		}
		_ = a.store.SetSetting("authenticated", "true")
	}

	testMsg := []providers.Message{{Role: "user", Content: "Hi"}}
	if err := a.llm.Chat(context.Background(), testMsg, nil, func(string) {}, nil); err != nil {
		ret := "Error: " + err.Error()
		a.persistConnectionTest(false, connectionTestFailureMessageForStore(ret), fp)
		return ret
	}

	ok := "Connection successful!"
	a.persistConnectionTest(true, ok, fp)
	return ok
}

func (a *App) GetAvailableProviders() []map[string]string {
	if a.factory == nil {
		return []map[string]string{}
	}

	descriptors := a.factory.RegisteredProviderDescriptors()
	result := make([]map[string]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		result = append(result, map[string]string{
			"id":   string(descriptor.ID),
			"name": descriptor.Name,
		})
	}
	return result
}
