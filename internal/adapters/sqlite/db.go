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

	if err := ensureCompatibleSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ensure compatible schema: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func ensureCompatibleSchema(db *sql.DB) error {
	needsReset, err := schemaNeedsReset(db)
	if err != nil {
		return err
	}
	if !needsReset {
		return nil
	}

	if err := resetSchema(db); err != nil {
		return fmt.Errorf("reset schema: %w", err)
	}
	return nil
}

func schemaNeedsReset(db *sql.DB) (bool, error) {
	usersExists, err := tableExists(db, "users")
	if err != nil {
		return false, fmt.Errorf("check users table: %w", err)
	}
	if !usersExists {
		hasLegacy, err := hasAnyLegacyTables(db)
		if err != nil {
			return false, err
		}
		if hasLegacy {
			return true, nil
		}
		return false, nil
	}

	hasSessionUserID, err := tableHasColumn(db, "sessions", "user_id")
	if err != nil {
		return false, fmt.Errorf("check sessions.user_id: %w", err)
	}
	hasAttemptUserID, err := tableHasColumn(db, "attempts", "user_id")
	if err != nil {
		return false, fmt.Errorf("check attempts.user_id: %w", err)
	}

	return !hasSessionUserID || !hasAttemptUserID, nil
}

func hasAnyLegacyTables(db *sql.DB) (bool, error) {
	tables := []string{"schema_migrations", "topics", "questions", "sessions", "attempts"}
	for _, table := range tables {
		exists, err := tableExists(db, table)
		if err != nil {
			return false, fmt.Errorf("check table %s: %w", table, err)
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func resetSchema(db *sql.DB) error {
	_, err := db.Exec(`
		DROP TABLE IF EXISTS attempts;
		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS questions;
		DROP TABLE IF EXISTS topics;
		DROP TABLE IF EXISTS users;
		DROP TABLE IF EXISTS schema_migrations;
	`)
	if err != nil {
		return err
	}
	return nil
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
	`CREATE TABLE IF NOT EXISTS users (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		handle      TEXT NOT NULL UNIQUE,
		display_name TEXT,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	INSERT OR IGNORE INTO users (handle, display_name, created_at)
	VALUES ('local', 'Local', CURRENT_TIMESTAMP);

	CREATE TABLE IF NOT EXISTS topics (
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
		difficulty              TEXT NOT NULL DEFAULT '',
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
		user_id     INTEGER NOT NULL REFERENCES users(id),
		topic_id    INTEGER NOT NULL REFERENCES topics(id),
		mode        TEXT NOT NULL DEFAULT 'practice',
		requested_n INTEGER NOT NULL DEFAULT 10,
		started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		ended_at    DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_user_topic_started
	ON sessions(user_id, topic_id, started_at);

	CREATE TABLE IF NOT EXISTS attempts (
		id                      INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id                 INTEGER NOT NULL REFERENCES users(id),
		session_id              INTEGER NOT NULL REFERENCES sessions(id),
		question_id             INTEGER NOT NULL REFERENCES questions(id),
		selected_choice_ids_json TEXT NOT NULL DEFAULT '[]',
		correct                 INTEGER NOT NULL DEFAULT 0,
		skipped                 INTEGER NOT NULL DEFAULT 0,
		latency_ms              INTEGER NOT NULL DEFAULT 0,
		created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_attempts_user_question_created
	ON attempts(user_id, question_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_attempts_user_session
	ON attempts(user_id, session_id);`,
}
