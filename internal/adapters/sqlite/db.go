// Package sqlite implements the persistence layer using SQLite.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // CGo-free SQLite driver
)

// DefaultDBPath returns ~/.golearn/golearn.db.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".golearn", "golearn.db")
}

// Open opens (or creates) a SQLite database, runs migrations, and enables WAL.
func Open(dbPath string) (*sql.DB, error) {
	// Ensure the directory exists.
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// Enable WAL mode to avoid "database is locked" with concurrent readers.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Enable foreign keys.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// migrate applies schema migrations in order.
func migrate(db *sql.DB) error {
	// Use a simple version-tracking table.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for i, m := range migrations {
		version := i + 1
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}
	return nil
}

// migrations are applied in order. Each entry is a single SQL statement or batch.
var migrations = []string{
	// v1: core schema
	`CREATE TABLE IF NOT EXISTS topics (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS questions (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id                INTEGER NOT NULL REFERENCES topics(id),
		type                    TEXT NOT NULL,
		intro                   TEXT NOT NULL DEFAULT '',
		prompt                  TEXT NOT NULL,
		choices_json            TEXT NOT NULL,
		correct_choice_ids_json TEXT NOT NULL,
		tags_json               TEXT NOT NULL DEFAULT '[]',
		difficulty              INTEGER NOT NULL DEFAULT 0,
		rationale_correct       TEXT NOT NULL DEFAULT '',
		rationale_per_choice_json TEXT NOT NULL DEFAULT '{}',
		source                  TEXT NOT NULL DEFAULT '',
		source_ref              TEXT NOT NULL DEFAULT '',
		confidence              REAL NOT NULL DEFAULT 1.0,
		hash                    TEXT NOT NULL UNIQUE,
		created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_questions_topic_id ON questions(topic_id);

	CREATE TABLE IF NOT EXISTS sessions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		topic_id    INTEGER NOT NULL REFERENCES topics(id),
		mode        TEXT NOT NULL DEFAULT 'practice',
		requested_n INTEGER NOT NULL DEFAULT 10,
		started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		ended_at    DATETIME
	);

	CREATE TABLE IF NOT EXISTS attempts (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id              INTEGER NOT NULL REFERENCES sessions(id),
		question_id             INTEGER NOT NULL REFERENCES questions(id),
		selected_choice_ids_json TEXT NOT NULL DEFAULT '[]',
		correct                 INTEGER NOT NULL DEFAULT 0,
		skipped                 INTEGER NOT NULL DEFAULT 0,
		latency_ms              INTEGER NOT NULL DEFAULT 0,
		created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_attempts_session_id ON attempts(session_id);
	CREATE INDEX IF NOT EXISTS idx_attempts_question_id ON attempts(question_id);`,
}
