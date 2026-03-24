package shared

import (
	"context"
)

type LLMClient interface {
	Chat(ctx context.Context, messages []Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error
	SetCustomSystemPrompt(prompt string)
	SignIn(ctx context.Context) error
	SignOut() error
	IsAuthenticated() bool
	Name() string
}

type Config interface {
	Type() string
	Validate() error
}

type Settings map[string]string

type AuthStatus struct {
	Authenticated bool
	Message       string
}

type Message struct {
	Role    string
	Content string
}
