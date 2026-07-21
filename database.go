package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"pedro/shared"
)

func atoiSafe(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func itoaSafe(n int) string {
	return strconv.Itoa(n)
}

type Database struct {
	db *sql.DB
}

func NewDatabase() (*Database, error) {
	appDataDir, err := os.UserConfigDir()
	if err != nil {
		appDataDir = "."
	}
	dbDir := filepath.Join(appDataDir, "Pedro")
	os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "pedro.db")

	// _foreign_keys=on  – enforce FK constraints (including ON DELETE CASCADE).
	// _busy_timeout=5000 – wait up to 5 s before returning SQLITE_BUSY.
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	// SQLite supports only one concurrent writer. Limiting the pool to a single
	// connection is the standard fix for SQLITE_BUSY with database/sql.
	db.SetMaxOpenConns(1)

	d := &Database{db: db}
	if err := d.init(); err != nil {
		return nil, err
	}
	d.migrate()
	return d, nil
}

func (d *Database) init() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			attachments TEXT NOT NULL DEFAULT '',
			tool_calls TEXT NOT NULL DEFAULT '',
			tool_call_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS personas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			prompt TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			category TEXT NOT NULL DEFAULT 'general',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS llm_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			request_count INTEGER NOT NULL DEFAULT 0,
			model TEXT NOT NULL DEFAULT '',
			messages TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_llm_history_created ON llm_history(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_memories_key ON memories(key);
		CREATE INDEX IF NOT EXISTS idx_memories_updated ON memories(updated_at);
	`)
	if err != nil {
		return err
	}

	// Conversation-id index on llm_history (separate statement so it doesn't
	// break init when the table already exists without the column).
	d.db.Exec("CREATE INDEX IF NOT EXISTS idx_llm_history_conv ON llm_history(conversation_id)")

	// FTS5 virtual table for full-text search on memories.
	_, err = d.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
			key, value, category,
			content=memories,
			content_rowid=id,
			tokenize='porter unicode61'
		);
	`)
	if err != nil {
		return err
	}

	// Triggers to keep FTS index in sync with the memories table.
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
			INSERT INTO memories_fts(rowid, key, value, category)
			VALUES (new.id, new.key, new.value, new.category);
		END;
	`)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, value, category)
			VALUES ('delete', old.id, old.key, old.value, old.category);
		END;
	`)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
			INSERT INTO memories_fts(memories_fts, rowid, key, value, category)
			VALUES ('delete', old.id, old.key, old.value, old.category);
			INSERT INTO memories_fts(rowid, key, value, category)
			VALUES (new.id, new.key, new.value, new.category);
		END;
	`)
	return err
}

func (d *Database) migrate() {
	// Add attachments column (idempotent — ALTER fails silently if column exists).
	d.db.Exec("ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT ''")
	// Add tool_calls column (idempotent — ALTER fails silently if column exists).
	d.db.Exec("ALTER TABLE messages ADD COLUMN tool_calls TEXT NOT NULL DEFAULT ''")
	// Add importance and source columns for memory extraction.
	d.db.Exec("ALTER TABLE memories ADD COLUMN importance INTEGER NOT NULL DEFAULT 3")
	d.db.Exec("ALTER TABLE memories ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'")
	// Add request_count column to track how many HTTP requests each chat made to the LLM.
	d.db.Exec("ALTER TABLE conversations ADD COLUMN request_count INTEGER NOT NULL DEFAULT 0")
	// Add request_tokens column to track the cumulative token usage per chat.
	d.db.Exec("ALTER TABLE conversations ADD COLUMN request_tokens INTEGER NOT NULL DEFAULT 0")
	// Add tool_call_id column for persisting tool roundtrip messages.
	d.db.Exec("ALTER TABLE messages ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT ''")
	// Recreate llm_history with conversation_id column if missing (data is ephemeral, safe to drop).
	if !d.columnExists("llm_history", "conversation_id") {
		d.db.Exec("DROP TABLE IF EXISTS llm_history")
		d.db.Exec(`
			CREATE TABLE llm_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				conversation_id INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				request_count INTEGER NOT NULL DEFAULT 0,
				model TEXT NOT NULL DEFAULT '',
				messages TEXT NOT NULL
			)
		`)
		d.db.Exec("CREATE INDEX IF NOT EXISTS idx_llm_history_created ON llm_history(created_at DESC)")
		d.db.Exec("CREATE INDEX IF NOT EXISTS idx_llm_history_conv ON llm_history(conversation_id)")
	}
	d.sanitizeExistingConversationTitles()
}

// columnExists checks whether a column exists in the given table.
func (d *Database) columnExists(table, column string) bool {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column)
	err := d.db.QueryRow(query).Scan(&count)
	return err == nil && count > 0
}

// IncrementRequestCount increments the per-conversation request counter by one
// and returns the new value. It is safe to call concurrently with MaxOpenConns(1).
func (d *Database) IncrementRequestCount(conversationID int64) (int, error) {
	if _, err := d.db.Exec(
		"UPDATE conversations SET request_count = request_count + 1 WHERE id = ?",
		conversationID,
	); err != nil {
		return 0, err
	}
	var count int
	if err := d.db.QueryRow(
		"SELECT request_count FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetRequestCount returns the number of LLM requests made in a conversation.
func (d *Database) GetRequestCount(conversationID int64) (int, error) {
	var count int
	err := d.db.QueryRow(
		"SELECT request_count FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetRequestTokens returns the cumulative token usage for a conversation.
func (d *Database) GetRequestTokens(conversationID int64) (int, error) {
	var tokens int
	err := d.db.QueryRow(
		"SELECT request_tokens FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&tokens)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return tokens, nil
}

// IncrementRequestTokens adds the given token count to the conversation total and
// returns the new cumulative value.
func (d *Database) IncrementRequestTokens(conversationID int64, tokens int) (int, error) {
	if _, err := d.db.Exec(
		"UPDATE conversations SET request_tokens = request_tokens + ? WHERE id = ?",
		tokens, conversationID,
	); err != nil {
		return 0, err
	}
	return d.GetRequestTokens(conversationID)
}

const globalRequestCountKey = "global_request_count"

// IncrementGlobalRequestCount increments the running global request total and
// returns the new value.
func (d *Database) IncrementGlobalRequestCount() (int, error) {
	current, err := d.GetSetting(globalRequestCountKey)
	if err != nil {
		return 0, err
	}
	next := 1
	if current != "" {
		next = atoiSafe(current) + 1
	}
	if err := d.SetSetting(globalRequestCountKey, itoaSafe(next)); err != nil {
		return 0, err
	}
	return next, nil
}

// GetGlobalRequestCount returns the running global request total.
func (d *Database) GetGlobalRequestCount() (int, error) {
	raw, err := d.GetSetting(globalRequestCountKey)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	return atoiSafe(raw), nil
}

const lifetimeTokensKey = "lifetime_tokens"

// AddLifetimeTokens accumulates the total prompt + completion tokens seen across
// all requests (for the Settings stats view).
func (d *Database) AddLifetimeTokens(promptTokens, completionTokens int) error {
	raw, err := d.GetSetting(lifetimeTokensKey)
	if err != nil {
		return err
	}
	total := 0
	if raw != "" {
		total = atoiSafe(raw)
	}
	total += promptTokens + completionTokens
	return d.SetSetting(lifetimeTokensKey, itoaSafe(total))
}

// GetLifetimeTokens returns the accumulated lifetime token total.
func (d *Database) GetLifetimeTokens() (int, error) {
	raw, err := d.GetSetting(lifetimeTokensKey)
	if err != nil {
		return 0, err
	}
	if raw == "" {
		return 0, nil
	}
	return atoiSafe(raw), nil
}

func (d *Database) sanitizeExistingConversationTitles() {
	rows, err := d.db.Query("SELECT id, title FROM conversations")
	if err != nil {
		return
	}
	defer rows.Close()

	type row struct {
		id    int64
		title string
	}
	var updates []row

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.title); err != nil {
			return
		}
		clean := sanitizeConversationTitle(r.title)
		if clean != r.title && clean != "" {
			updates = append(updates, row{id: r.id, title: clean})
		}
	}

	for _, u := range updates {
		d.db.Exec("UPDATE conversations SET title = ? WHERE id = ?", u.title, u.id)
	}
}

func (d *Database) CreateConversation() (*Conversation, error) {
	res, err := d.db.Exec("INSERT INTO conversations (title) VALUES (?)", "New Chat")
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Conversation{ID: id, Title: "New Chat"}, nil
}

func (d *Database) GetConversations() ([]Conversation, error) {
	rows, err := d.db.Query("SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, nil
}

func (d *Database) GetMessages(conversationID int64) ([]Message, error) {
	rows, err := d.db.Query("SELECT id, conversation_id, role, content, attachments, tool_calls, tool_call_id, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Attachments, &m.ToolCalls, &m.ToolCallID, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (d *Database) SearchMessages(query string) (map[int64][]Message, error) {
	rows, err := d.db.Query("SELECT id, conversation_id, role, content, attachments, tool_calls, tool_call_id, created_at FROM messages WHERE content LIKE ? ORDER BY created_at ASC", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]Message)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Attachments, &m.ToolCalls, &m.ToolCallID, &m.CreatedAt); err != nil {
			return nil, err
		}
		result[m.ConversationID] = append(result[m.ConversationID], m)
	}
	return result, nil
}

func (d *Database) AddMessage(conversationID int64, role, content, attachments, toolCalls, toolCallID string) (*Message, error) {
	_, err := d.db.Exec("INSERT INTO messages (conversation_id, role, content, attachments, tool_calls, tool_call_id) VALUES (?, ?, ?, ?, ?, ?)", conversationID, role, content, attachments, toolCalls, toolCallID)
	if err != nil {
		return nil, err
	}

	d.db.Exec("UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", conversationID)

	if role == "user" {
		firstMsg := sanitizeConversationTitle(content)
		if len(firstMsg) > 50 {
			firstMsg = firstMsg[:50] + "..."
		}
		if firstMsg != "" {
			d.db.Exec("UPDATE conversations SET title = ? WHERE id = ? AND title = 'New Chat'", firstMsg, conversationID)
		}
	}

	return &Message{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Attachments:    attachments,
		ToolCalls:      toolCalls,
		ToolCallID:     toolCallID,
	}, nil
}

func sanitizeConversationTitle(raw string) string {
	title := raw
	markers := []string{
		"\n\n[File:",
		"\n\n[Folder:",
		"\n\n[Path:",
		" [File:",
		" [Folder:",
		" [Path:",
	}
	cut := len(title)
	for _, m := range markers {
		if idx := strings.Index(title, m); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	if cut < len(title) {
		title = title[:cut]
	}
	return strings.TrimSpace(title)
}

func (d *Database) DeleteMessage(conversationID int64, messageIndex int) error {
	// Use QueryRow so the connection is released as soon as Scan returns,
	// before we call Exec. With MaxOpenConns(1) this avoids a deadlock where
	// an open *Rows holds the only connection while Exec waits for it.
	var id int64
	err := d.db.QueryRow(
		"SELECT id FROM messages WHERE conversation_id = ? ORDER BY id ASC LIMIT 1 OFFSET ?",
		conversationID, messageIndex,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("message not found at index %d", messageIndex)
	}
	if err != nil {
		return err
	}

	_, err = d.db.Exec("DELETE FROM messages WHERE id = ?", id)
	return err
}

func (d *Database) DeleteConversation(id int64) error {
	// Clean up llm_history entries for this conversation.
	d.db.Exec("DELETE FROM llm_history WHERE conversation_id = ?", id)
	// ON DELETE CASCADE (enforced via _foreign_keys=on DSN option) handles
	// the child messages rows automatically.
	_, err := d.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	return err
}

func (d *Database) DeleteAllConversations() error {
	d.db.Exec("DELETE FROM llm_history")
	_, err := d.db.Exec("DELETE FROM conversations")
	return err
}

func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *Database) SetSetting(key, value string) error {
	_, err := d.db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

func (d *Database) DeleteSetting(key string) error {
	_, err := d.db.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

func (d *Database) GetSettings() (map[string]string, error) {
	rows, err := d.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, nil
}

// LLMDetailsEntry is a single persisted final payload handed to the LLM provider.
type LLMDetailsEntry struct {
	ID             int64
	ConversationID int64
	ConversationTitle string
	CreatedAt      time.Time
	RequestCount   int
	Model          string
	Messages       string
}

// AddLLMDetails inserts a finalized LLM request payload for a conversation,
// replacing any previous entry for the same conversation (one row per conversation).
func (d *Database) AddLLMDetails(conversationID int64, model string, requestCount int, messagesJSON string) error {
	// Delete existing entry for this conversation before inserting (upsert).
	if _, err := d.db.Exec(
		"DELETE FROM llm_history WHERE conversation_id = ?",
		conversationID,
	); err != nil {
		return err
	}
	_, err := d.db.Exec(
		"INSERT INTO llm_history (conversation_id, model, request_count, messages) VALUES (?, ?, ?, ?)",
		conversationID, model, requestCount, messagesJSON,
	)
	return err
}

// GetLLMDetails returns the most recent entries, newest first, with conversation titles.
func (d *Database) GetLLMDetails() ([]LLMDetailsEntry, error) {
	rows, err := d.db.Query(`
		SELECT h.id, h.conversation_id, COALESCE(c.title, ''), h.created_at, h.request_count, h.model, h.messages
		FROM llm_history h
		LEFT JOIN conversations c ON c.id = h.conversation_id
		ORDER BY h.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LLMDetailsEntry
	for rows.Next() {
		var e LLMDetailsEntry
		if err := rows.Scan(&e.ID, &e.ConversationID, &e.ConversationTitle, &e.CreatedAt, &e.RequestCount, &e.Model, &e.Messages); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ClearLLMDetails removes every persisted request payload.
func (d *Database) ClearLLMDetails() error {
	_, err := d.db.Exec("DELETE FROM llm_history")
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetPersonas() ([]Persona, error) {
	rows, err := d.db.Query("SELECT id, name, prompt, created_at, updated_at FROM personas ORDER BY name COLLATE NOCASE ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Persona
	for rows.Next() {
		var p Persona
		if err := rows.Scan(&p.ID, &p.Name, &p.Prompt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (d *Database) CreatePersona(name, prompt string) (*Persona, error) {
	res, err := d.db.Exec("INSERT INTO personas (name, prompt) VALUES (?, ?)", name, prompt)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	var p Persona
	err = d.db.QueryRow(
		"SELECT id, name, prompt, created_at, updated_at FROM personas WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Name, &p.Prompt, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *Database) UpdatePersona(id int64, name, prompt string) error {
	res, err := d.db.Exec(
		"UPDATE personas SET name = ?, prompt = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		name, prompt, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("persona not found: %d", id)
	}
	return nil
}

func (d *Database) DeletePersona(id int64) error {
	res, err := d.db.Exec("DELETE FROM personas WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("persona not found: %d", id)
	}
	return nil
}

func (d *Database) GetMemories() ([]shared.MemoryRecord, error) {
	rows, err := d.db.Query("SELECT id, key, value, category, importance, source, updated_at FROM memories ORDER BY importance DESC, updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []shared.MemoryRecord
	for rows.Next() {
		var m shared.MemoryRecord
		if err := rows.Scan(&m.ID, &m.Key, &m.Value, &m.Category, &m.Importance, &m.Source, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (d *Database) GetMemoryKeys() ([]string, error) {
	rows, err := d.db.Query("SELECT key FROM memories ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (d *Database) SearchMemories(query string) ([]shared.MemoryRecord, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return d.GetMemories()
	}

	// Build FTS5 query: each term must match key, value, or category
	var ftsTerms []string
	for _, t := range terms {
		if len(t) < 3 {
			continue
		}
		ftsTerms = append(ftsTerms, t+"*") // prefix matching
	}
	if len(ftsTerms) == 0 {
		return d.GetMemories()
	}
	ftsQuery := strings.Join(ftsTerms, " ")

	// FTS5 search with BM25 ranking, combined with importance
	sqlStr := `SELECT m.id, m.key, m.value, m.category, m.importance, m.source, m.updated_at
		FROM memories m
		JOIN memories_fts fts ON fts.rowid = m.id
		WHERE memories_fts MATCH ?
		ORDER BY (bm25(fts) * -1 + m.importance * 0.3) DESC
		LIMIT 5`

	rows, err := d.db.Query(sqlStr, ftsQuery)
	if err != nil {
		// Fallback to LIKE search if FTS fails
		return d.searchMemoriesLike(query)
	}
	defer rows.Close()

	var list []shared.MemoryRecord
	for rows.Next() {
		var m shared.MemoryRecord
		if err := rows.Scan(&m.ID, &m.Key, &m.Value, &m.Category, &m.Importance, &m.Source, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (d *Database) searchMemoriesLike(query string) ([]shared.MemoryRecord, error) {
	terms := strings.Fields(strings.ToLower(query))
	var conditions []string
	var args []any
	for _, t := range terms {
		if len(t) < 3 {
			continue
		}
		conditions = append(conditions, "LOWER(key) LIKE ? OR LOWER(value) LIKE ?")
		args = append(args, "%"+t+"%", "%"+t+"%")
	}
	if len(conditions) == 0 {
		return d.GetMemories()
	}
	sqlStr := "SELECT id, key, value, category, importance, source, updated_at FROM memories WHERE " +
		strings.Join(conditions, " OR ") +
		" ORDER BY importance DESC, updated_at DESC LIMIT 5"
	rows, err := d.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []shared.MemoryRecord
	for rows.Next() {
		var m shared.MemoryRecord
		if err := rows.Scan(&m.ID, &m.Key, &m.Value, &m.Category, &m.Importance, &m.Source, &m.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (d *Database) SaveMemory(key, value, category string) error {
	if category == "" {
		category = "general"
	}
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO memories (key, value, category, importance, source, updated_at) VALUES (?, ?, ?, 3, 'manual', CURRENT_TIMESTAMP)",
		key, value, category,
	)
	return err
}

func (d *Database) SaveMemoryWithMeta(key, value, category, source string, importance int) error {
	if category == "" {
		category = "general"
	}
	if importance < 1 || importance > 5 {
		importance = 3
	}
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO memories (key, value, category, importance, source, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
		key, value, category, importance, source,
	)
	return err
}

func (d *Database) ForgetMemory(id int64) error {
	_, err := d.db.Exec("DELETE FROM memories WHERE id = ?", id)
	return err
}
