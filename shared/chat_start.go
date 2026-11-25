package shared

import (
	"context"
	"time"
)

type chatStartKey struct{}

// WithChatStart records when the conversation a reply belongs to started. The system prompt
// gives the model that time rather than the current one, so the prompt stays the same for
// the whole chat and providers can keep serving it from their prompt cache.
func WithChatStart(ctx context.Context, started time.Time) context.Context {
	return context.WithValue(ctx, chatStartKey{}, started)
}

// ChatStart is when the reply's conversation started, or now when that is not known.
func ChatStart(ctx context.Context) time.Time {
	if started, ok := ctx.Value(chatStartKey{}).(time.Time); ok && !started.IsZero() {
		return started
	}
	return time.Now()
}
