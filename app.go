package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"pedro/memory"
	"pedro/providers"
	"pedro/shared"
	"pedro/tools"

	"github.com/openai/openai-go"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type toolCallRecord struct {
	Name     string `json:"name"`
	ArgsJSON string `json:"argsJSON"`
	ID       string `json:"id"`
}

type App struct {
	ctx        context.Context
	store      Store
	llm        providers.LLMClient
	registry   *tools.Registry
	factory    *providers.Factory
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc
	extractor  *memory.Extractor
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
		registry: tools.New(db),
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

	// Create memory extractor
	if client, ok := llm.ExtractionClient().(openai.Client); ok {
		a.extractor = memory.NewExtractor(client, llm.ModelName(), a.store)
	}
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
		llmMessages[i] = providers.Message{Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID}
	}

	var response []byte
	var captured shared.CapturedRequest
	generatedMessages, err := a.llm.Chat(
		ctx,
		llmMessages,
		imageDataURLs,
		func(chunk string) {
			response = append(response, chunk...)
			runtime.EventsEmit(a.ctx, "stream_chunk", conversationID, chunk)
		},
		func(name, argsJSON, id string) {
			runtime.EventsEmit(a.ctx, "tool_call", conversationID, name, argsJSON)
		},
		func(usage shared.RequestUsage) {
			a.recordRequest(conversationID, usage)
		},
		func(capturedReq shared.CapturedRequest) {
			captured = capturedReq
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return string(response), nil
		}
		return "", err
	}

	// Persist each generated message (tool roundtrips + final assistant).
	// Merge all tool-calling assistant messages into a single one so the
	// frontend groups every tool call under one "tools used" card.
	var mergedToolCalls []toolCallRecord
	var toolResultMsgs []shared.Message
	for _, m := range generatedMessages {
		if m.Role == "assistant" && m.ToolCalls != "" {
			var tcs []toolCallRecord
			if err := json.Unmarshal([]byte(m.ToolCalls), &tcs); err == nil {
				mergedToolCalls = append(mergedToolCalls, tcs...)
			}
			continue
		}
		if m.Role == "tool" {
			toolResultMsgs = append(toolResultMsgs, m)
			continue
		}
		// Flush the merged assistant message before the first non-tool
		// message (tool result or final assistant).
		if len(mergedToolCalls) > 0 {
			mergedJSON, _ := json.Marshal(mergedToolCalls)
			if _, saveErr := a.store.AddMessage(conversationID, "assistant", "", "", string(mergedJSON), ""); saveErr != nil {
				fmt.Println("Warning: Failed to save merged tool-call message:", saveErr.Error())
			}
			for _, tr := range toolResultMsgs {
				if _, saveErr := a.store.AddMessage(conversationID, tr.Role, tr.Content, "", "", tr.ToolCallID); saveErr != nil {
					fmt.Println("Warning: Failed to save tool result message:", saveErr.Error())
				}
			}
			mergedToolCalls = nil
			toolResultMsgs = nil
		}
		if _, saveErr := a.store.AddMessage(conversationID, m.Role, m.Content, "", "", m.ToolCallID); saveErr != nil {
			fmt.Println("Warning: Failed to save message:", saveErr.Error())
		}
	}
	// Edge case: conversation ended with tool calls (shouldn't happen but be safe).
	if len(mergedToolCalls) > 0 {
		mergedJSON, _ := json.Marshal(mergedToolCalls)
		if _, saveErr := a.store.AddMessage(conversationID, "assistant", "", "", string(mergedJSON), ""); saveErr != nil {
			fmt.Println("Warning: Failed to save merged tool-call message:", saveErr.Error())
		}
		for _, tr := range toolResultMsgs {
			if _, saveErr := a.store.AddMessage(conversationID, tr.Role, tr.Content, "", "", tr.ToolCallID); saveErr != nil {
				fmt.Println("Warning: Failed to save tool result message:", saveErr.Error())
			}
		}
	}

	// Determine the final assistant text response.
	var finalText string
	for i := len(generatedMessages) - 1; i >= 0; i-- {
		if generatedMessages[i].Role == "assistant" && generatedMessages[i].ToolCalls == "" {
			finalText = generatedMessages[i].Content
			break
		}
	}

	// Persist the finalized payload actually sent to the provider together with
	// the assistant's reply (one row per top-level request, capturing the final
	// assembled context incl. tools and the resulting response).
	if captured.Messages != nil {
		a.recordLLMDetails(conversationID, captured, finalText)
	}

	return finalText, nil
}

// recordLLMDetails stores the fully assembled request sent to the LLM plus the
// assistant's response, correlated with the conversation. No-op when the store
// is nil.
func (a *App) recordLLMDetails(conversationID int64, captured shared.CapturedRequest, responseText string) {
	if a.store == nil {
		return
	}
	payload := map[string]any{
		"messages": captured.Messages,
		"tools":    captured.Tools,
		"response": responseText,
	}
	msgsJSON, err := json.Marshal(payload)
	if err != nil {
		return
	}
	model := ""
	if a.llm != nil {
		model = a.llm.ModelName()
	}
	global, err := a.store.GetGlobalRequestCount()
	if err != nil {
		global = 0
	}
	if err := a.store.AddLLMDetails(conversationID, model, global, string(msgsJSON)); err != nil {
		fmt.Println("AddLLMDetails error:", err.Error())
	}
}

// recordRequest is invoked once per completed HTTP request to the LLM provider.
// It increments the per-chat and global request counters, accumulates lifetime
// token stats, and emits a live update event for the UI.
func (a *App) recordRequest(conversationID int64, usage shared.RequestUsage) {
	if a.store == nil {
		return
	}

	perChat, err := a.store.IncrementRequestCount(conversationID)
	if err != nil {
		perChat = 0
	}
	global, err := a.store.IncrementGlobalRequestCount()
	if err != nil {
		global = 0
	}
	chatTokens := 0
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		requestTokens := usage.PromptTokens + usage.CompletionTokens
		chatTokens, _ = a.store.IncrementRequestTokens(conversationID, requestTokens)
		_ = a.store.AddLifetimeTokens(usage.PromptTokens, usage.CompletionTokens)
	}
	runtime.EventsEmit(
		a.ctx,
		"request_count_updated",
		conversationID,
		perChat,
		global,
		chatTokens,
		usage.PromptTokens,
		usage.CompletionTokens,
	)
}

func (a *App) AbortMessage() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFunc != nil {
		a.cancelFunc()
	}
}

var englishStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "must": true, "shall": true, "can": true,
	"need": true, "dare": true, "ought": true, "used": true, "to": true, "of": true,
	"in": true, "for": true, "on": true, "with": true, "at": true, "by": true, "from": true,
	"as": true, "into": true, "through": true, "during": true, "before": true, "after": true,
	"above": true, "below": true, "between": true, "under": true, "and": true, "but": true,
	"or": true, "yet": true, "so": true, "if": true, "because": true, "although": true,
	"though": true, "while": true, "where": true, "when": true, "that": true, "which": true,
	"who": true, "whom": true, "whose": true, "what": true, "this": true, "these": true,
	"those": true, "i": true, "me": true, "my": true, "myself": true, "we": true, "our": true,
	"ours": true, "ourselves": true, "you": true, "your": true, "yours": true, "yourself": true,
	"yourselves": true, "he": true, "him": true, "his": true, "himself": true, "she": true,
	"her": true, "hers": true, "herself": true, "it": true, "its": true, "itself": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true, "am": true,
	"having": true, "doing": true,
}

func extractKeywords(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var result []string
	for _, w := range words {
		w = strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		if len(w) >= 3 && !englishStopWords[w] {
			result = append(result, w)
		}
	}
	return result
}

func (a *App) relevantMemoriesSection(content string) string {
	if a.store == nil {
		return ""
	}

	// Retrieve all memories sorted by importance (highest first).
	records, err := a.store.GetMemories()
	if err != nil || len(records) == 0 {
		return ""
	}

	const maxMemories = 30
	const maxTokens = 1500
	if len(records) > maxMemories {
		records = records[:maxMemories]
	}

	// Group by category
	byCategory := make(map[string][]shared.MemoryRecord)
	for _, r := range records {
		cat := r.Category
		if cat == "" {
			cat = "other"
		}
		byCategory[cat] = append(byCategory[cat], r)
	}

	var b strings.Builder
	b.WriteString("## Memories\n")
	b.WriteString("The following memories are automatically available to personalize your response. Reference them naturally when relevant, but do not force them into the conversation.\n\n")

	totalTokens := 0
	for cat, items := range byCategory {
		b.WriteString(fmt.Sprintf("### %s\n", cat))
		for _, r := range items {
			label := ""
			switch {
			case r.Importance >= 5:
				label = " [critical]"
			case r.Importance >= 4:
				label = " [important]"
			}
			line := fmt.Sprintf("- %s: %s%s\n", r.Key, r.Value, label)
			totalTokens += len(line) / 4 // rough token estimator
			if totalTokens > maxTokens {
				b.WriteString("\n... (additional memories omitted to stay within context window)\n")
				return b.String()
			}
			b.WriteString(line)
		}
	}
	return b.String()
}

func (a *App) buildMemoryContext(content string) string {
	if section := a.relevantMemoriesSection(content); section != "" {
		return section
	}
	return ""
}

func (a *App) triggerMemoryExtraction(conversationID int64, userContent, assistantResponse string) {
	if a.extractor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	go func() {
		defer cancel()
		a.extractor.ExtractAndSave(ctx, userContent, assistantResponse, conversationID)
	}()
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

	if _, err := a.store.AddMessage(conversationID, "user", content, attachmentsJSON, "", ""); err != nil {
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
	a.llm.SetMemoryContext(a.buildMemoryContext(content))
	mergedImages := mergeImageDataURLsFromFileRefs(imageDataURLs, attachmentsJSON, content)
	resp, err := a.runChat(conversationID, messages, mergedImages)
	if err != nil {
		return "Error: " + err.Error()
	}

	a.triggerMemoryExtraction(conversationID, content, resp)

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

func (a *App) DeleteMessage(conversationID int64, messageIndex int) error {
	if a.store == nil {
		return nil
	}
	return a.store.DeleteMessage(conversationID, messageIndex)
}

func (a *App) SendMessage(conversationID int64, content string, selectedPersonaID string, attachmentsJSON string) string {
	return a.sendMessage(conversationID, content, nil, selectedPersonaID, attachmentsJSON)
}

func (a *App) SendMessageWithImages(conversationID int64, content string, imageDataURLs []string, selectedPersonaID string, attachmentsJSON string) string {
	return a.sendMessage(conversationID, content, imageDataURLs, selectedPersonaID, attachmentsJSON)
}

func (a *App) ResendMessage(conversationID int64, messageIndex int, selectedPersonaID string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if messageIndex < 0 || messageIndex >= len(messages) {
		return "Error: Invalid message index"
	}
	if messages[messageIndex].Role != "user" {
		return "Error: No user message to resend"
	}

	// Truncate everything after the selected user turn, then stream a fresh assistant reply.
	for i := len(messages) - 1; i > messageIndex; i-- {
		if err := a.store.DeleteMessage(conversationID, i); err != nil {
			return "Error: Failed to delete message: " + err.Error()
		}
	}

	messages, err = a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if a.llm == nil {
		return "Error: Please configure LLM provider settings first"
	}

	userMsg := messages[messageIndex]
	inlineImages := imageDataURLsFromAttachmentsJSON(userMsg.Attachments)
	mergedImages := mergeImageDataURLsFromFileRefs(inlineImages, userMsg.Attachments, userMsg.Content)

	a.llm.SetPersonaPrompt(a.personaPromptFromDB(selectedPersonaID))
	a.llm.SetMemoryContext(a.buildMemoryContext(userMsg.Content))
	resp, err := a.runChat(conversationID, messages, mergedImages)
	if err != nil {
		return "Error: " + err.Error()
	}

	a.triggerMemoryExtraction(conversationID, userMsg.Content, resp)

	return resp
}

func (a *App) RegenerateMessage(conversationID int64, messageIndex int, selectedPersonaID string) string {
	if a.store == nil {
		return "Error: Database not initialized"
	}

	messages, err := a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if messageIndex < 0 || messageIndex >= len(messages) {
		return "Error: Invalid message index"
	}
	if messages[messageIndex].Role != "assistant" {
		return "Error: No assistant message to regenerate"
	}

	// Regenerate from the selected assistant turn by truncating that turn and all
	// later turns, then streaming a replacement response from the preserved prefix.
	for i := len(messages) - 1; i >= messageIndex; i-- {
		if err := a.store.DeleteMessage(conversationID, i); err != nil {
			return "Error: Failed to delete message: " + err.Error()
		}
	}

	messages, err = a.store.GetMessages(conversationID)
	if err != nil {
		return "Error: Failed to get messages: " + err.Error()
	}

	if a.llm == nil {
		return "Error: Please configure LLM provider settings first"
	}

	// Find the last user message to use for memory context.
	var lastUserContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserContent = messages[i].Content
			break
		}
	}

	a.llm.SetPersonaPrompt(a.personaPromptFromDB(selectedPersonaID))
	a.llm.SetMemoryContext(a.buildMemoryContext(lastUserContent))
	resp, err := a.runChat(conversationID, messages, nil)
	if err != nil {
		return "Error: " + err.Error()
	}

	a.triggerMemoryExtraction(conversationID, lastUserContent, resp)

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

// OpenPath opens a file or folder with the OS default handler. Returns "" on success, else an error message.
// SaveFile shows a native save dialog and writes content to the selected path.
// Returns "" on success, else an error message.
func (a *App) SaveFile(defaultFilename string, content string) string {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
	})
	if err != nil || path == "" {
		return fmt.Sprintf("save cancelled: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("failed to write file: %v", err)
	}
	return ""
}

func (a *App) OpenPath(path string) string {
	if path == "" {
		return "empty path"
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return "path must be absolute"
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Sprintf("cannot access path: %v", err)
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		// Avoid cmd parsing edge cases (&, |, etc.) by using the shell file handler directly.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
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
	if _, err := a.llm.Chat(context.Background(), testMsg, nil, func(string) {}, nil, nil, nil); err != nil {
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

func (a *App) GetMemories() []shared.MemoryRecord {
	if a.store == nil {
		return []shared.MemoryRecord{}
	}
	list, err := a.store.GetMemories()
	if err != nil {
		fmt.Println("GetMemories error:", err.Error())
		return []shared.MemoryRecord{}
	}
	return list
}

func (a *App) SaveMemory(key, value, category string) error {
	if a.store == nil {
		return errors.New("database not initialized")
	}
	return a.store.SaveMemory(key, value, category)
}

func (a *App) ForgetMemory(id int64) error {
	if a.store == nil {
		return errors.New("database not initialized")
	}
	return a.store.ForgetMemory(id)
}

// RequestCounts bundles the per-chat and global request tallies for a conversation.
type RequestCounts struct {
	PerChat       int `json:"perChat"`
	PerChatTokens int `json:"perChatTokens"`
	Global        int `json:"global"`
}

// GetRequestCounts returns the per-chat and global LLM request counts.
func (a *App) GetRequestCounts(conversationID int64) RequestCounts {
	if a.store == nil {
		return RequestCounts{}
	}
	perChat, err := a.store.GetRequestCount(conversationID)
	if err != nil {
		perChat = 0
	}
	perChatTokens, err := a.store.GetRequestTokens(conversationID)
	if err != nil {
		perChatTokens = 0
	}
	global, err := a.store.GetGlobalRequestCount()
	if err != nil {
		global = 0
	}
	return RequestCounts{PerChat: perChat, PerChatTokens: perChatTokens, Global: global}
}

// GetGlobalRequestCount returns the running global LLM request total.
func (a *App) GetGlobalRequestCount() int {
	if a.store == nil {
		return 0
	}
	global, err := a.store.GetGlobalRequestCount()
	if err != nil {
		return 0
	}
	return global
}

// LifetimeStats reports the cumulative LLM request count and token total.
type LifetimeStats struct {
	TotalRequests int `json:"totalRequests"`
	TotalTokens   int `json:"totalTokens"`
}

// GetLifetimeStats returns the cumulative request count and lifetime token total.
func (a *App) GetLifetimeStats() LifetimeStats {
	if a.store == nil {
		return LifetimeStats{}
	}
	totalRequests, err := a.store.GetGlobalRequestCount()
	if err != nil {
		totalRequests = 0
	}
	totalTokens, err := a.store.GetLifetimeTokens()
	if err != nil {
		totalTokens = 0
	}
	return LifetimeStats{TotalRequests: totalRequests, TotalTokens: totalTokens}
}

// GetLLMDetails returns the persisted final payloads (newest-first), capped at 20.
func (a *App) GetLLMDetails() []LLMDetailsEntry {
	if a.store == nil {
		return []LLMDetailsEntry{}
	}
	entries, err := a.store.GetLLMDetails()
	if err != nil {
		fmt.Println("GetLLMDetails error:", err.Error())
		return []LLMDetailsEntry{}
	}
	return entries
}

// ClearLLMDetails empties the persisted request details.
func (a *App) ClearLLMDetails() error {
	if a.store == nil {
		return nil
	}
	return a.store.ClearLLMDetails()
}
