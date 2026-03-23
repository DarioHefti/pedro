package main

import (
	"context"
	"fmt"
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
		fmt.Println("Database error:", err.Error())
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
		llm, err := NewAzureClient(endpoint, apiKey, deployment, a.registry)
		if err != nil {
			fmt.Println("LLM init error:", err.Error())
			return
		}
		a.llm = llm
	}
}

// runChat invokes the LLM, streams chunks and tool-call events to the frontend,
// and returns the full assistant response. It is the single authoritative path
// for all LLM interactions to eliminate code duplication.
func (a *App) runChat(messages []Message, imageDataURLs []string) (string, error) {
	var response []byte
	err := a.llm.Chat(
		context.Background(),
		messages,
		imageDataURLs,
		func(chunk string) {
			response = append(response, chunk...)
			runtime.EventsEmit(a.ctx, "stream_chunk", chunk)
		},
		func(name, argsJSON string) {
			runtime.EventsEmit(a.ctx, "tool_call", name, argsJSON)
		},
	)
	if err != nil {
		return "", err
	}
	return string(response), nil
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

func (a *App) SendMessage(conversationID int64, content string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	if _, err := a.store.AddMessage(conversationID, "user", content); err != nil {
		return "Error: Failed to save message: " + err.Error()
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if a.llm == nil {
		return "Error: Please configure Azure AI settings first"
	}

	resp, err := a.runChat(messages, nil)
	if err != nil {
		return "Error: " + err.Error()
	}

	if _, saveErr := a.store.AddMessage(conversationID, "assistant", resp); saveErr != nil {
		fmt.Println("Warning: Failed to save assistant message:", saveErr.Error())
	}
	return resp
}

// SendMessageWithImages sends a user message alongside image data URLs.
// The plain text content is stored in the DB; images are passed directly to
// the LLM via the multimodal API and are not persisted.
func (a *App) SendMessageWithImages(conversationID int64, content string, imageDataURLs []string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	if _, err := a.store.AddMessage(conversationID, "user", content); err != nil {
		return "Error: Failed to save message: " + err.Error()
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if a.llm == nil {
		return "Error: Please configure Azure AI settings first"
	}

	resp, err := a.runChat(messages, imageDataURLs)
	if err != nil {
		return "Error: " + err.Error()
	}

	if _, saveErr := a.store.AddMessage(conversationID, "assistant", resp); saveErr != nil {
		fmt.Println("Warning: Failed to save assistant message:", saveErr.Error())
	}
	return resp
}

// RegenerateMessage removes the last assistant message and generates a new one.
func (a *App) RegenerateMessage(conversationID int64) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	// Find and remove the last assistant message
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
		return "Error: Please configure Azure AI settings first"
	}

	resp, err := a.runChat(messages, nil)
	if err != nil {
		return "Error: " + err.Error()
	}

	if _, saveErr := a.store.AddMessage(conversationID, "assistant", resp); saveErr != nil {
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

func (a *App) SaveSettings(endpoint, apiKey, deployment string) error {
	if a.store == nil {
		return nil
	}
	if err := a.store.SetSetting("azure_endpoint", endpoint); err != nil {
		return err
	}
	if err := a.store.SetSetting("azure_api_key", apiKey); err != nil {
		return err
	}
	if err := a.store.SetSetting("azure_deployment", deployment); err != nil {
		return err
	}
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

func (a *App) SetSetting(key, value string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SetSetting(key, value)
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
	if err := client.Chat(context.Background(), testMsg, nil, func(string) {}, nil); err != nil {
		return "Error: " + err.Error()
	}
	return "Connection successful!"
}
