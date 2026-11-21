// Package store keeps cestafet's shared state in a local SQLite database.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is a handle on the cestafet database.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	id          TEXT PRIMARY KEY,
	claude_pid  INTEGER NOT NULL DEFAULT 0,
	profile     TEXT NOT NULL,
	project     TEXT NOT NULL,
	repo        TEXT NOT NULL DEFAULT '',
	cwd         TEXT NOT NULL,
	branch      TEXT NOT NULL DEFAULT '',
	summary     TEXT NOT NULL DEFAULT '',
	role        TEXT NOT NULL DEFAULT 'member',
	status      TEXT NOT NULL DEFAULT 'live',
	started_at  INTEGER NOT NULL,
	last_seen   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_project ON sessions(project, status);
CREATE INDEX IF NOT EXISTS sessions_pid ON sessions(claude_pid, status);

CREATE TABLE IF NOT EXISTS prompts (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id  TEXT NOT NULL,
	text        TEXT NOT NULL,
	at          INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS prompts_session ON prompts(session_id, id);

CREATE TABLE IF NOT EXISTS notices (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	project       TEXT NOT NULL,
	from_session  TEXT NOT NULL,
	from_profile  TEXT NOT NULL,
	from_role     TEXT NOT NULL,
	origin        TEXT NOT NULL,
	prompt_id     INTEGER,
	body          TEXT NOT NULL,
	created_at    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS notices_project ON notices(project, created_at);

CREATE TABLE IF NOT EXISTS deliveries (
	notice_id     INTEGER NOT NULL,
	to_session    TEXT NOT NULL,
	delivered_at  INTEGER,
	acked_at      INTEGER,
	reply         TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (notice_id, to_session)
);
CREATE INDEX IF NOT EXISTS deliveries_pending ON deliveries(to_session, delivered_at);
`

// DefaultPath returns the database location: $XDG_DATA_HOME/cestafet/bus.db,
// falling back to ~/.local/share/cestafet/bus.db.
func DefaultPath() (string, error) {
	if p := os.Getenv("CESTAFET_DB"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "cestafet", "bus.db"), nil
}

// Open opens (and creates if needed) the database at path. The directory is
// created with mode 0700 and the file with mode 0600.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	_ = f.Close()

	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

// OpenDefault opens the database at DefaultPath.
func OpenDefault() (*Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(p)
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// SetClock replaces the time source; used by tests.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) unix() int64 { return s.now().Unix() }

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")
