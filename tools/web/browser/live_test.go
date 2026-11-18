//go:build live

package browser

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Run with: go test -tags live ./tools/web/browser/
func TestLiveIdleShutdownAndSessionRecovery(t *testing.T) {
	m := NewManager(Options{IdleTimeout: time.Second})
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	page := `data:text/html,<title>Test</title><h1>Hello</h1><a href="https://example.com">Example link</a>`
	r, err := m.Render(ctx, page, RenderOptions{})
	if err != nil || r.Title != "Test" {
		t.Fatalf("render: %+v %v", r, err)
	}

	s := NewSession(m)
	st, err := s.Navigate(ctx, page)
	if err != nil || st.Snapshot == "" {
		t.Fatalf("navigate: %+v %v", st, err)
	}
	t.Logf("snapshot:\n%s", st.Snapshot)

	time.Sleep(2500 * time.Millisecond)
	m.mu.Lock()
	closed := m.browser == nil
	m.mu.Unlock()
	if !closed {
		t.Fatal("browser should close after the idle timeout")
	}

	if _, err := s.Snapshot(ctx); !errors.Is(err, ErrNoPage) {
		t.Fatalf("expected ErrNoPage after the browser restarted, got %v", err)
	}
	if st, err := s.Navigate(ctx, page); err != nil || st.Title != "Test" {
		t.Fatalf("session should recover with navigate: %+v %v", st, err)
	}
}
