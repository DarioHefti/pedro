// Package pageindex keeps the pages the agent has read in a local full-text
// index (SQLite FTS5 with BM25 ranking) so it can search inside long pages
// and across whole sites without reading everything into the context.
package pageindex

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"pedro/tools/web/weburl"

	_ "modernc.org/sqlite"
)

// Page is a page to index.
type Page struct {
	URL      string
	Title    string
	Markdown string
}

// Hit is a matching chunk.
type Hit struct {
	URL     string
	Title   string
	Heading string
	Text    string
	Offset  int // where the chunk starts in the page's Markdown
}

// Query selects chunks. URL and Host are optional filters.
type Query struct {
	Text  string
	URL   string
	Host  string
	Limit int
}

// Index stores and searches read pages.
type Index interface {
	Add(p Page) error
	Get(rawURL string, maxAge time.Duration) (*Page, bool)
	Search(q Query) ([]Hit, error)
	CountHost(host string) int
	Close() error
}

// SQLiteIndex is an in-memory Index. It lives for the duration of the app and
// forgets the oldest pages beyond maxPages.
type SQLiteIndex struct {
	db       *sql.DB
	maxPages int
}

// NewSQLiteIndex creates an empty in-memory index.
func NewSQLiteIndex(maxPages int) (*SQLiteIndex, error) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		return nil, err
	}
	// Every pooled connection would get its own private memory database.
	db.SetMaxOpenConns(1)

	schema := `
		CREATE TABLE pages (
			id       INTEGER PRIMARY KEY,
			key      TEXT NOT NULL UNIQUE,
			url      TEXT NOT NULL,
			host     TEXT NOT NULL,
			title    TEXT NOT NULL,
			markdown TEXT NOT NULL,
			added_at INTEGER NOT NULL
		);
		CREATE INDEX pages_host ON pages(host);
		CREATE VIRTUAL TABLE chunks USING fts5(
			heading, content, page_id UNINDEXED, pos UNINDEXED,
			tokenize = 'unicode61 remove_diacritics 2'
		);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create page index: %w", err)
	}
	return &SQLiteIndex{db: db, maxPages: maxPages}, nil
}

// Add indexes p, replacing an earlier version of the same URL.
func (x *SQLiteIndex) Add(p Page) error {
	key := weburl.Key(p.URL)
	if key == "" {
		return fmt.Errorf("cannot index non-HTTP URL %q", p.URL)
	}
	tx, err := x.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := deletePages(tx, `SELECT id FROM pages WHERE key = ?`, key); err != nil {
		return err
	}
	res, err := tx.Exec(`INSERT INTO pages (key, url, host, title, markdown, added_at) VALUES (?, ?, ?, ?, ?, ?)`,
		key, p.URL, weburl.Host(p.URL), p.Title, p.Markdown, time.Now().UnixNano())
	if err != nil {
		return err
	}
	pageID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chunks (heading, content, page_id, pos) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range ChunkMarkdown(p.Markdown) {
		if _, err := stmt.Exec(c.Heading, c.Text, pageID, c.Offset); err != nil {
			return err
		}
	}

	evict := `SELECT id FROM pages ORDER BY added_at DESC LIMIT -1 OFFSET ?`
	if err := deletePages(tx, evict, x.maxPages); err != nil {
		return err
	}
	return tx.Commit()
}

func deletePages(tx *sql.Tx, selectIDs string, args ...any) error {
	where := `IN (` + selectIDs + `)`
	if _, err := tx.Exec(`DELETE FROM chunks WHERE page_id `+where, args...); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM pages WHERE id `+where, args...)
	return err
}

// Get returns a page indexed less than maxAge ago.
func (x *SQLiteIndex) Get(rawURL string, maxAge time.Duration) (*Page, bool) {
	var p Page
	var addedAt int64
	err := x.db.QueryRow(`SELECT url, title, markdown, added_at FROM pages WHERE key = ?`, weburl.Key(rawURL)).
		Scan(&p.URL, &p.Title, &p.Markdown, &addedAt)
	if err != nil || time.Since(time.Unix(0, addedAt)) > maxAge {
		return nil, false
	}
	return &p, true
}

// CountHost returns how many pages of host are indexed.
func (x *SQLiteIndex) CountHost(host string) int {
	var n int
	_ = x.db.QueryRow(`SELECT COUNT(*) FROM pages WHERE host = ?`, strings.TrimPrefix(strings.ToLower(host), "www.")).Scan(&n)
	return n
}

// Search returns the best matching chunks, ranked by BM25 (headings weigh double).
func (x *SQLiteIndex) Search(q Query) ([]Hit, error) {
	match := matchExpression(q.Text)
	if match == "" {
		return nil, fmt.Errorf("query %q contains no searchable words", q.Text)
	}
	if q.Limit <= 0 {
		q.Limit = 8
	}

	sqlStr := `
		SELECT p.url, p.title, c.heading, c.content, c.pos
		FROM chunks c JOIN pages p ON p.id = c.page_id
		WHERE chunks MATCH ?`
	args := []any{match}
	if q.URL != "" {
		sqlStr += ` AND p.key = ?`
		args = append(args, weburl.Key(q.URL))
	}
	if q.Host != "" {
		sqlStr += ` AND p.host = ?`
		args = append(args, strings.TrimPrefix(strings.ToLower(q.Host), "www."))
	}
	sqlStr += ` ORDER BY bm25(chunks, 2.0, 1.0) LIMIT ?`
	args = append(args, q.Limit)

	rows, err := x.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.URL, &h.Title, &h.Heading, &h.Text, &h.Offset); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

func (x *SQLiteIndex) Close() error { return x.db.Close() }

// matchExpression builds an FTS5 query that matches any of the words (longer
// words also as prefix, which helps with German compounds); BM25 then ranks
// chunks containing more of them higher.
func matchExpression(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := make(map[string]bool)
	var terms []string
	for _, w := range words {
		if seen[w] {
			continue
		}
		seen[w] = true
		term := `"` + w + `"`
		if len([]rune(w)) >= 4 {
			term += "*"
		}
		// "335c" should also find "Art. 335 c", which is how many law texts
		// and extractors render article numbers.
		if split := splitLettersDigits(w); split != w {
			term = "(" + term + ` OR "` + split + `")`
		}
		terms = append(terms, term)
	}
	return strings.Join(terms, " OR ")
}

// splitLettersDigits inserts a space wherever letters and digits meet,
// e.g. "335c" -> "335 c".
func splitLettersDigits(w string) string {
	var sb strings.Builder
	prev := rune(0)
	for i, r := range w {
		if i > 0 && unicode.IsDigit(r) != unicode.IsDigit(prev) {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
		prev = r
	}
	return sb.String()
}

// ErrUnavailable is returned by Discard.
var ErrUnavailable = errors.New("page index is unavailable")

// Discard is an Index that stores nothing; used if SQLite cannot start.
type Discard struct{}

func (Discard) Add(Page) error                          { return nil }
func (Discard) Get(string, time.Duration) (*Page, bool) { return nil, false }
func (Discard) Search(Query) ([]Hit, error)             { return nil, ErrUnavailable }
func (Discard) CountHost(string) int                    { return 0 }
func (Discard) Close() error                            { return nil }
