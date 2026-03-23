package main

import (
	"context"
	"time"
)

type Conversation struct {
	ID        int64
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	ID             int64
	ConversationID int64
	Role           string
	Content        string
	CreatedAt      time.Time
}

type Store interface {
	GetConversations() ([]Conversation, error)
	GetConversation(id int64) (*Conversation, error)
	CreateConversation() (*Conversation, error)
	DeleteConversation(id int64) error
	GetMessages(conversationID int64) ([]Message, error)
	SearchMessages(query string) (map[int64][]Message, error)
	AddMessage(conversationID int64, role, content string) (*Message, error)
	DeleteMessage(conversationID int64, messageIndex int) error
	UpdateMessageContent(id int64, content string) error
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	GetSettings() (map[string]string, error)
	Close() error
}

type LLMClient interface {
	Chat(ctx context.Context, messages []Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error
}
