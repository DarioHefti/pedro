package main

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openTestDatabase(t *testing.T) *Database {
	t.Helper()
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "pedro.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustConversation(t *testing.T, db *Database) int64 {
	t.Helper()
	conv, err := db.CreateConversation()
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return conv.ID
}

func mustAdd(t *testing.T, db *Database, convID int64, role, content string) {
	t.Helper()
	if _, err := db.AddMessage(convID, role, content, "", "", ""); err != nil {
		t.Fatalf("add message: %v", err)
	}
}

func contents(t *testing.T, db *Database, convID int64) []string {
	t.Helper()
	msgs, err := db.GetMessages(convID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}

func TestDatabase_ForeignKeysAreEnforced(t *testing.T) {
	db := openTestDatabase(t)
	var on int
	if err := db.db.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil || on != 1 {
		t.Fatalf("foreign_keys = %d (err %v), want 1", on, err)
	}

	// Deleting a conversation cascades to its messages.
	id := mustConversation(t, db)
	mustAdd(t, db, id, "user", "hello")
	if err := db.DeleteConversation(id); err != nil {
		t.Fatal(err)
	}
	var left int
	db.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&left)
	if left != 0 {
		t.Fatalf("%d messages survived their conversation", left)
	}

	// A message cannot be added to a conversation that no longer exists.
	if _, err := db.AddMessage(id, "user", "into the void", "", "", ""); err == nil {
		t.Fatal("adding to a deleted conversation should fail")
	}
}

func TestDatabase_MigrationRemovesOrphanedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pedro.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	keep := mustConversation(t, db)
	mustAdd(t, db, keep, "user", "kept")
	db.Close()

	// Simulate rows left behind while foreign keys were off.
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"INSERT INTO messages (conversation_id, role, content) VALUES (999, 'user', 'orphan')",
		"INSERT INTO llm_history (conversation_id, messages) VALUES (999, '{}')",
		"INSERT INTO llm_history (conversation_id, messages) VALUES (0, '{}')",
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	raw.Close()

	db = func() *Database {
		d, err := OpenDatabase(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		return d
	}()
	var messages, details int
	db.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messages)
	db.db.QueryRow("SELECT COUNT(*) FROM llm_history").Scan(&details)
	if messages != 1 {
		t.Fatalf("messages = %d, want only the kept one", messages)
	}
	if details != 1 {
		t.Fatalf("llm_history = %d, want only the unassigned (conversation 0) row", details)
	}
}

func TestDatabase_GetMessagesKeepsInsertionOrder(t *testing.T) {
	db := openTestDatabase(t)
	id := mustConversation(t, db)
	// All inserted within the same second: created_at ties, insertion order must win.
	want := []string{"question", "", "tool result", "answer", "follow-up"}
	for i, c := range want {
		role := []string{"user", "assistant", "tool", "assistant", "user"}[i]
		mustAdd(t, db, id, role, c)
	}
	if got := contents(t, db, id); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("order = %q, want %q", got, want)
	}
}

func TestDatabase_DeleteMessagesFrom(t *testing.T) {
	db := openTestDatabase(t)
	id := mustConversation(t, db)
	other := mustConversation(t, db)
	for _, c := range []string{"q1", "a1", "q2", "a2"} {
		mustAdd(t, db, id, "user", c)
	}
	mustAdd(t, db, other, "user", "untouched")

	if err := db.DeleteMessagesFrom(id, 2); err != nil {
		t.Fatal(err)
	}
	if got := contents(t, db, id); strings.Join(got, ",") != "q1,a1" {
		t.Fatalf("after delete from 2: %q", got)
	}
	if err := db.DeleteMessagesFrom(id, 10); err != nil {
		t.Fatalf("past the end should be a no-op, got %v", err)
	}
	if err := db.DeleteMessagesFrom(id, -1); err == nil {
		t.Fatal("negative index should fail")
	}
	if err := db.DeleteMessagesFrom(id, 0); err != nil {
		t.Fatal(err)
	}
	if got := contents(t, db, id); len(got) != 0 {
		t.Fatalf("after delete from 0: %q", got)
	}
	if got := contents(t, db, other); len(got) != 1 {
		t.Fatalf("other conversation changed: %q", got)
	}
}

func TestDatabase_TitleFromFirstUserMessage(t *testing.T) {
	db := openTestDatabase(t)
	title := func(id int64) string {
		convs, _ := db.GetConversations()
		for _, c := range convs {
			if c.ID == id {
				return c.Title
			}
		}
		return ""
	}

	id := mustConversation(t, db)
	mustAdd(t, db, id, "user", "Summarize this\n\n[File: notes.md]\n[Path: /tmp/notes.md]")
	mustAdd(t, db, id, "user", "A later question does not rename the chat")
	if got := title(id); got != "Summarize this" {
		t.Fatalf("title = %q", got)
	}

	long := mustConversation(t, db)
	mustAdd(t, db, long, "user", strings.Repeat("x", 60))
	if got := title(long); got != strings.Repeat("x", 50)+"..." {
		t.Fatalf("long title = %q", got)
	}
}

func TestDatabase_SearchMessagesGroupsByConversation(t *testing.T) {
	db := openTestDatabase(t)
	a := mustConversation(t, db)
	b := mustConversation(t, db)
	mustAdd(t, db, a, "user", "Kubernetes ingress")
	mustAdd(t, db, a, "assistant", "Use an ingress controller")
	mustAdd(t, db, b, "user", "Sourdough")

	got, err := db.SearchMessages("ingress")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[a]) != 2 {
		t.Fatalf("search = %#v", got)
	}
}
