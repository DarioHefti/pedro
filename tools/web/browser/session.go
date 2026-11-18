package browser

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// State is what the agent sees after every browser action.
type State struct {
	URL       string
	Title     string
	Snapshot  string
	Truncated bool
}

// Session is a single persistent tab the agent drives step by step:
// navigate, look at the snapshot, then act on elements by their ref.
// It is safe for concurrent use; actions are serialized.
type Session struct {
	m *Manager

	mu   sync.Mutex
	page *rod.Page
	gen  int
	refs map[string]proto.DOMBackendNodeID
}

// ErrNoPage is returned when an action needs a page but none is open.
var ErrNoPage = errors.New("no page is open; use the navigate action first")

// NewSession creates a session on top of the shared browser.
func NewSession(m *Manager) *Session {
	return &Session{m: m}
}

const elementTimeout = 5 * time.Second

// Navigate opens rawURL in the session tab.
func (s *Session) Navigate(ctx context.Context, rawURL string) (*State, error) {
	return s.act(ctx, true, func(p *rod.Page) error {
		return Settle(p, func() error { return p.Navigate(rawURL) })
	})
}

// Snapshot returns the current state without acting.
func (s *Session) Snapshot(ctx context.Context) (*State, error) {
	return s.act(ctx, false, func(*rod.Page) error { return nil })
}

// Click clicks the element with the given ref.
func (s *Session) Click(ctx context.Context, ref string) (*State, error) {
	return s.act(ctx, false, func(p *rod.Page) error {
		el, err := s.element(p, ref)
		if err != nil {
			return err
		}
		// Keep the session in this tab instead of spawning new ones.
		_, _ = el.Eval(`function() { const a = this.closest('a, form'); if (a) a.removeAttribute('target'); }`)
		return Settle(p, func() error {
			_ = el.Timeout(elementTimeout).ScrollIntoView()
			if err := el.Timeout(elementTimeout).Click(proto.InputMouseButtonLeft, 1); err != nil {
				// Covered or animated elements: fall back to a DOM click.
				if _, jsErr := el.Eval(`function() { this.click(); }`); jsErr != nil {
					return fmt.Errorf("click %s: %w", ref, err)
				}
			}
			return nil
		})
	})
}

// Type replaces the content of the field with the given ref. With submit,
// Enter is pressed afterwards.
func (s *Session) Type(ctx context.Context, ref, text string, submit bool) (*State, error) {
	return s.act(ctx, false, func(p *rod.Page) error {
		el, err := s.element(p, ref)
		if err != nil {
			return err
		}
		el = el.Timeout(elementTimeout)
		_ = el.ScrollIntoView()
		_ = el.SelectAllText()
		if err := el.Input(text); err != nil {
			return fmt.Errorf("type into %s: %w", ref, err)
		}
		if !submit {
			return nil
		}
		return Settle(p, func() error { return p.Keyboard.Press(input.Enter) })
	})
}

// Select chooses the option with the given visible text in a <select>.
func (s *Session) Select(ctx context.Context, ref, option string) (*State, error) {
	return s.act(ctx, false, func(p *rod.Page) error {
		el, err := s.element(p, ref)
		if err != nil {
			return err
		}
		return Settle(p, func() error {
			if err := el.Timeout(elementTimeout).Select([]string{option}, true, rod.SelectorTypeText); err != nil {
				return fmt.Errorf("select %q in %s: %w", option, ref, err)
			}
			return nil
		})
	})
}

var keys = map[string]input.Key{
	"enter": input.Enter, "tab": input.Tab, "escape": input.Escape, "backspace": input.Backspace,
	"delete": input.Delete, "space": input.Space, "arrowdown": input.ArrowDown, "arrowup": input.ArrowUp,
	"arrowleft": input.ArrowLeft, "arrowright": input.ArrowRight, "pagedown": input.PageDown,
	"pageup": input.PageUp, "home": input.Home, "end": input.End,
}

// Press presses a named key (Enter, Tab, Escape, ArrowDown, ...).
func (s *Session) Press(ctx context.Context, key string) (*State, error) {
	k, ok := keys[strings.ToLower(strings.TrimSpace(key))]
	if !ok {
		return nil, fmt.Errorf("unknown key %q (supported: Enter, Tab, Escape, Backspace, Delete, Space, ArrowUp/Down/Left/Right, PageUp, PageDown, Home, End)", key)
	}
	return s.act(ctx, false, func(p *rod.Page) error {
		return Settle(p, func() error { return p.Keyboard.Press(k) })
	})
}

// Scroll scrolls the page by most of a screen in direction "up" or "down".
func (s *Session) Scroll(ctx context.Context, direction string) (*State, error) {
	sign := 1
	if strings.EqualFold(direction, "up") {
		sign = -1
	}
	return s.act(ctx, false, func(p *rod.Page) error {
		_, err := p.Eval(`(sign) => window.scrollBy(0, sign * window.innerHeight * 0.8)`, sign)
		time.Sleep(400 * time.Millisecond) // lazy-loaded content
		return err
	})
}

// Back goes back in the tab's history.
func (s *Session) Back(ctx context.Context) (*State, error) {
	return s.act(ctx, false, func(p *rod.Page) error {
		return Settle(p, p.NavigateBack)
	})
}

// HTML returns the URL and rendered DOM of the current page.
func (s *Session) HTML(ctx context.Context) (pageURL, html string, err error) {
	err = s.withPage(ctx, false, func(p *rod.Page) error {
		info, err := p.Info()
		if err != nil {
			return err
		}
		pageURL = info.URL
		html, err = RenderedHTML(p)
		return err
	})
	return pageURL, html, err
}

// Close closes the session tab.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page != nil {
		_ = s.page.Close()
		s.page = nil
	}
	s.refs = nil
	return nil
}

// act runs an action and returns a fresh snapshot of the resulting page.
func (s *Session) act(ctx context.Context, open bool, action func(p *rod.Page) error) (*State, error) {
	var state *State
	err := s.withPage(ctx, open, func(p *rod.Page) error {
		if err := action(p); err != nil {
			return err
		}
		var err error
		state, err = s.snapshotLocked(p)
		return err
	})
	return state, err
}

// withPage runs fn on the session tab with the session lock held. With open,
// a missing tab (first use, or Chrome was restarted after being idle) is created.
func (s *Session) withPage(ctx context.Context, open bool, fn func(p *rod.Page) error) error {
	return s.m.Do(func(b *rod.Browser, gen int) error {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.page != nil && s.gen != gen {
			s.page, s.refs = nil, nil // the browser was restarted; the old tab is gone
		}
		if s.page == nil {
			if !open {
				return ErrNoPage
			}
			page, err := s.m.NewPage(b)
			if err != nil {
				return err
			}
			s.page, s.gen = page, gen
		}
		return fn(s.page.Context(ctx))
	})
}

func (s *Session) snapshotLocked(p *rod.Page) (*State, error) {
	tree, err := proto.AccessibilityGetFullAXTree{}.Call(p)
	if err != nil {
		return nil, fmt.Errorf("read accessibility tree: %w", err)
	}
	snap := buildSnapshot(tree.Nodes)
	s.refs = snap.refs

	info, err := p.Info()
	if err != nil {
		return nil, err
	}
	return &State{URL: info.URL, Title: info.Title, Snapshot: snap.text, Truncated: snap.truncated}, nil
}

func (s *Session) element(p *rod.Page, ref string) (*rod.Element, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "@")
	id, ok := s.refs[ref]
	if !ok {
		return nil, fmt.Errorf("unknown ref %q; refs change after every action, use the refs from the latest snapshot", ref)
	}
	obj, err := proto.DOMResolveNode{BackendNodeID: id}.Call(p)
	if err != nil {
		return nil, fmt.Errorf("element %s is no longer on the page; take a new snapshot", ref)
	}
	return p.ElementFromObject(obj.Object)
}
