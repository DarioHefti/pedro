package main

import (
	"database/sql"
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

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

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

func (d *Database) UpdateMessageContent(id int64, content string) error {
	_, err := d.db.Exec("UPDATE messages SET content = ? WHERE id = ?", content, id)
	return err
}

func (d *Database) DeleteConversation(id int64) error {
	_, err := d.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	if err != nil {
		return err
	}
	_, err = d.db.Exec("DELETE FROM conversations WHERE id = ?", id)
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

func (d *Database) GetConversation(id int64) (*Conversation, error) {
	var c Conversation
	err := d.db.QueryRow("SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?", id).Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
