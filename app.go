package main

import (
	"context"
	"path/filepath"

	"pedro/tools"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	store    Store
	llm      LLMClient
	registry *tools.Registry
}

func NewApp() *App {
	db, err := NewDatabase()
	if err != nil {
		println("Database error:", err.Error())
		return &App{store: nil, llm: nil, registry: nil}
	}

	return &App{
		store:    db,
		registry: tools.New(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initLLM()
}

func (a *App) initLLM() {
	endpoint, _ := a.store.GetSetting("azure_endpoint")
	apiKey, _ := a.store.GetSetting("azure_api_key")
	deployment, _ := a.store.GetSetting("azure_deployment")

	if endpoint != "" && apiKey != "" && deployment != "" {
		a.llm, _ = NewAzureClient(endpoint, apiKey, deployment, a.registry)
	}
}

func (a *App) GetConversations() []Conversation {
	if a.store == nil {
		return []Conversation{}
	}
	convs, _ := a.store.GetConversations()
	return convs
}

func (a *App) GetMessages(conversationID int64) []Message {
	if a.store == nil {
		return []Message{}
	}
	msgs, _ := a.store.GetMessages(conversationID)
	return msgs
}

func (a *App) CreateConversation() *Conversation {
	if a.store == nil {
		return &Conversation{ID: 0, Title: "New Chat"}
	}
	conv, _ := a.store.CreateConversation()
	return conv
}

func (a *App) DeleteConversation(id int64) error {
	if a.store == nil {
		return nil
	}
	return a.store.DeleteConversation(id)
}

func (a *App) SendMessage(conversationID int64, content string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	_, addErr := a.store.AddMessage(conversationID, "user", content)
	if addErr != nil {
		return "Error: Failed to save message: " + addErr.Error()
	}

	messages, _ := a.store.GetMessages(conversationID)

	if a.llm == nil {
		return "Error: Please configure Azure AI settings first"
	}

	var response []byte
	err := a.llm.Chat(context.Background(), messages, nil,
		func(chunk string) {
			response = append(response, chunk...)
		},
		func(name, argsJSON string) {
			runtime.EventsEmit(a.ctx, "tool_call", name, argsJSON)
		},
	)
	if err != nil {
		return "Error: " + err.Error()
	}

	resp := string(response)
	a.store.AddMessage(conversationID, "assistant", resp)
	return resp
}

// SendMessageWithImages sends a user message alongside image data URLs.
// The plain text content is stored in the DB; images are passed directly to
// the LLM via the multimodal API and are not persisted.
func (a *App) SendMessageWithImages(conversationID int64, content string, imageDataURLs []string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	_, addErr := a.store.AddMessage(conversationID, "user", content)
	if addErr != nil {
		return "Error: Failed to save message: " + addErr.Error()
	}

	messages, _ := a.store.GetMessages(conversationID)

	if a.llm == nil {
		return "Error: Please configure Azure AI settings first"
	}

	var response []byte
	err := a.llm.Chat(context.Background(), messages, imageDataURLs,
		func(chunk string) {
			response = append(response, chunk...)
		},
		func(name, argsJSON string) {
			runtime.EventsEmit(a.ctx, "tool_call", name, argsJSON)
		},
	)
	if err != nil {
		return "Error: " + err.Error()
	}

	resp := string(response)
	a.store.AddMessage(conversationID, "assistant", resp)
	return resp
}

func (a *App) GetSettings() map[string]string {
	if a.store == nil {
		return map[string]string{}
	}
	settings, _ := a.store.GetSettings()
	return settings
}

func (a *App) SaveSettings(endpoint, apiKey, deployment string) error {
	if a.store == nil {
		return nil
	}
	a.store.SetSetting("azure_endpoint", endpoint)
	a.store.SetSetting("azure_api_key", apiKey)
	a.store.SetSetting("azure_deployment", deployment)
	a.initLLM()
	return nil
}

// SelectFile opens a native OS file dialog and returns the selected file path.
// Returns an empty string if the user cancels.
func (a *App) SelectFile() string {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select File",
	})
	if err != nil || path == "" {
		return ""
	}
	return filepath.ToSlash(path)
}

func (a *App) TestConnection(endpoint, apiKey, deployment string) string {
	if endpoint == "" || apiKey == "" || deployment == "" {
		return "Error: All fields are required"
	}

	client, err := NewAzureClient(endpoint, apiKey, deployment, nil)
	if err != nil {
		return "Error: " + err.Error()
	}

	testMsg := []Message{{Role: "user", Content: "Hi"}}
	err = client.Chat(context.Background(), testMsg, nil, func(chunk string) {}, nil)
	if err != nil {
		return "Error: " + err.Error()
	}
	return "Connection successful!"
}
