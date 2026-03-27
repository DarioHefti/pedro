package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

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
	`)
	return err
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
	rows, err := d.db.Query("SELECT id, conversation_id, role, content, created_at FROM messages WHERE conversation_id = ? ORDER BY created_at ASC", conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (d *Database) SearchMessages(query string) (map[int64][]Message, error) {
	rows, err := d.db.Query("SELECT id, conversation_id, role, content, created_at FROM messages WHERE content LIKE ? ORDER BY created_at ASC", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]Message)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		result[m.ConversationID] = append(result[m.ConversationID], m)
	}
	return result, nil
}

func (d *Database) AddMessage(conversationID int64, role, content string) (*Message, error) {
	_, err := d.db.Exec("INSERT INTO messages (conversation_id, role, content) VALUES (?, ?, ?)", conversationID, role, content)
	if err != nil {
		return nil, err
	}

	d.db.Exec("UPDATE conversations SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", conversationID)

	if role == "user" {
		firstMsg := content
		if len(firstMsg) > 50 {
			firstMsg = firstMsg[:50] + "..."
		}
		d.db.Exec("UPDATE conversations SET title = ? WHERE id = ? AND title = 'New Chat'", firstMsg, conversationID)
	}

	return &Message{ConversationID: conversationID, Role: role, Content: content}, nil
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
	// ON DELETE CASCADE (enforced via _foreign_keys=on DSN option) handles
	// the child messages rows automatically.
	_, err := d.db.Exec("DELETE FROM conversations WHERE id = ?", id)
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
