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

// Persona is a named instruction block prepended to outgoing user prompts when selected.
type Persona struct {
	ID        int64
	Name      string
	Prompt    string
	CreatedAt time.Time `ts_type:"string"`
	UpdatedAt time.Time `ts_type:"string"`
}

type Store interface {
	GetConversations() ([]Conversation, error)
	CreateConversation() (*Conversation, error)
	DeleteConversation(id int64) error
	DeleteAllConversations() error
	GetMessages(conversationID int64) ([]Message, error)
	SearchMessages(query string) (map[int64][]Message, error)
	AddMessage(conversationID int64, role, content, attachments, toolCalls, toolCallID string) (*Message, error)
	DeleteMessage(conversationID int64, messageIndex int) error
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
	GetPersonas() ([]Persona, error)
	CreatePersona(name, prompt string) (*Persona, error)
	UpdatePersona(id int64, name, prompt string) error
	DeletePersona(id int64) error
	AddLLMDetails(conversationID int64, model string, requestCount int, messagesJSON string) error
	GetLLMDetails() ([]LLMDetailsEntry, error)
	ClearLLMDetails() error
	shared.MemoryBackend
	Close() error
}
