package main

import (
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
	Attachments    string
	CreatedAt      time.Time
}

// Persona is a named instruction block prepended to outgoing user prompts when selected.
type Persona struct {
	ID        int64
	Name      string
	Prompt    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Store interface {
	GetConversations() ([]Conversation, error)
	CreateConversation() (*Conversation, error)
	DeleteConversation(id int64) error
	DeleteAllConversations() error
	GetMessages(conversationID int64) ([]Message, error)
	SearchMessages(query string) (map[int64][]Message, error)
	AddMessage(conversationID int64, role, content, attachments string) (*Message, error)
	DeleteMessage(conversationID int64, messageIndex int) error
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
	DeleteSetting(key string) error
	GetSettings() (map[string]string, error)
	GetPersonas() ([]Persona, error)
	CreatePersona(name, prompt string) (*Persona, error)
	UpdatePersona(id int64, name, prompt string) error
	DeletePersona(id int64) error
	Close() error
}
