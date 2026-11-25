package main

import (
	"time"

	"pedro/shared"
)

type Conversation struct {
	ID        int64
	Title     string
	CreatedAt time.Time `ts_type:"string"`
	UpdatedAt time.Time `ts_type:"string"`
}

type Message struct {
	ID             int64
	ConversationID int64
	Role           string
	Content        string
	Attachments    string
	ToolCalls      string
	ToolCallID     string
	CreatedAt      time.Time `ts_type:"string"`
}

type Store interface {
	GetConversations() ([]Conversation, error)
	CreateConversation() (*Conversation, error)
	DeleteConversation(id int64) error
	DeleteAllConversations() error
	GetMessages(conversationID int64) ([]Message, error)
	SearchMessages(query string) (map[int64][]Message, error)
	AddMessage(conversationID int64, role, content, attachments, toolCalls, toolCallID string) (*Message, error)
	// DeleteMessagesFrom deletes the message at position fromIndex and every later one.
	DeleteMessagesFrom(conversationID int64, fromIndex int) error
	IncrementRequestCount(conversationID int64) (int, error)
	GetRequestCount(conversationID int64) (int, error)
	IncrementRequestTokens(conversationID int64, tokens int) (int, error)
	GetRequestTokens(conversationID int64) (int, error)
	IncrementGlobalRequestCount() (int, error)
	GetGlobalRequestCount() (int, error)
	AddLifetimeTokens(promptTokens, completionTokens int) error
	GetLifetimeTokens() (int, error)
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	DeleteSetting(key string) error
	GetSettings() (map[string]string, error)
	AddLLMDetails(conversationID int64, model string, requestCount int, messagesJSON string) error
	GetLLMDetails() ([]LLMDetailsEntry, error)
	ClearLLMDetails() error
	shared.MemoryBackend
	Close() error
}

// EventEmitter publishes backend events (stream chunks, counters, ...) to the frontend.
type EventEmitter interface {
	Emit(name string, data ...any)
}

// OpenDialogs asks the person to pick a file or folder. An empty path means they cancelled.
type OpenDialogs interface {
	PickFile() (string, error)
	PickFolder() (string, error)
}
