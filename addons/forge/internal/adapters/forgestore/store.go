// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package forgestore persists Forge's run history and drafts in the golearn
// database.
//
// Forge extends the *existing* database rather than opening a second one
// (FORGE.md 8), which raises the question of who owns schema versioning. It
// keeps its own registry, forge_schema_migrations, and never appends to the
// core's.
//
// That is not tidiness. The core's applied-check is per version number
// (WHERE version = ?), so a version another module already claimed reads as
// "already applied" — and the core's own next migration is then silently
// skipped, with no error and no way to notice. The measurement, including the
// failing case, is docs/design/FORGE-EXPERIMENTS.md A-7.
//
// The complementary rule: no Forge migration may alter a core table. Additive
// tables leave a Forge-extended database *compatible* rather than newer, so the
// offline binary keeps opening it and a user who tried Forge once is not locked
// out of the product D-015 promises is unchanged. TestForgeMigrationsNeverTouchCoreTables
// is what holds an implementer to it.
package forgestore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Store is the Forge persistence adapter. It borrows an already-open database
// handle rather than opening its own: the core's Open enforces WAL, foreign
// keys and the D-014 schema guard, and a second opener would either duplicate
// those or quietly skip them.
type Store struct {
	db *sql.DB
}

// New prepares Forge's tables in an already-open golearn database.
func New(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("forgestore: nil database handle")
	}
	if err := migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("forgestore: %w", err)
	}
	return &Store{db: db}, nil
}

// migrationsTable is Forge's own registry. See the package comment for why it
// is not the core's.
const migrationsTable = "forge_schema_migrations"

// migrations are applied in order, each exactly once, and each strictly
// additive: new tables and new indexes only, never an ALTER of a core table.
var migrations = []string{
	// v1: run history and drafts
	`CREATE TABLE IF NOT EXISTS forge_runs (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		spec_json         TEXT NOT NULL,
		provider          TEXT NOT NULL DEFAULT '',
		model             TEXT NOT NULL DEFAULT '',
		verifier_provider TEXT NOT NULL DEFAULT '',
		verifier_model    TEXT NOT NULL DEFAULT '',
		status            TEXT NOT NULL,
		started_at        DATETIME NOT NULL,
		finished_at       DATETIME,
		sources_json      TEXT NOT NULL DEFAULT '[]',
		input_tokens      INTEGER NOT NULL DEFAULT 0,
		output_tokens     INTEGER NOT NULL DEFAULT 0,
		attempts          INTEGER NOT NULL DEFAULT 0,
		diagnostic        TEXT NOT NULL DEFAULT ''
	);

	CREATE INDEX IF NOT EXISTS idx_forge_runs_started ON forge_runs(started_at);

	CREATE TABLE IF NOT EXISTS forge_drafts (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id     INTEGER NOT NULL REFERENCES forge_runs(id),
		pack_json  TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_forge_drafts_created ON forge_drafts(created_at);`,

	// v2: embedding vectors for the near-duplicate gate (D-020)
	//
	// question_id references a core questions(id) but carries no FOREIGN KEY,
	// and that is the load-bearing part. The core opens the database with
	// foreign_keys=ON, so a child row here would make a core-side delete of a
	// question fail — the offline binary would break because the user once ran
	// Forge, which is precisely what "additive" is supposed to prevent. The
	// price is that a deleted question can leave a vector behind; a stale
	// vector costs bytes, an unopenable library costs the product.
	//
	// dim is stored beside the BLOB rather than derived from its length so a
	// mixed-model corpus is detectable by a query instead of by decoding every
	// row.
	`CREATE TABLE IF NOT EXISTS forge_embeddings (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		question_id INTEGER NOT NULL,
		provider    TEXT NOT NULL,
		model       TEXT NOT NULL,
		dim         INTEGER NOT NULL,
		vector      BLOB NOT NULL,
		UNIQUE (question_id, provider, model)
	);

	CREATE INDEX IF NOT EXISTS idx_forge_embeddings_model ON forge_embeddings(provider, model);`,
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+migrationsTable+` (
		version INTEGER PRIMARY KEY
	)`); err != nil {
		return fmt.Errorf("create %s: %w", migrationsTable, err)
	}

	for i, stmt := range migrations {
		version := i + 1
		if err := applyMigration(ctx, db, version, stmt); err != nil {
			return err
		}
	}
	return nil
}

// migrationAttempts bounds the retry on write contention.
const migrationAttempts = 5

// applyMigration applies one migration, retrying on write contention.
//
// The applied-check runs INSIDE the transaction that applies the migration:
// checking outside leaves a window where two processes both see "not applied"
// and both try to record it, and one then fails on the primary key against a
// perfectly valid schema.
//
// The transaction starts with BEGIN IMMEDIATE, acquiring the write lock before
// the applied-check reads the registry. Competing openers therefore wait for
// the current writer or retry on a fresh connection instead of upgrading a
// stale read snapshot to a write and receiving SQLITE_BUSY_SNAPSHOT.
func applyMigration(ctx context.Context, db *sql.DB, version int, stmt string) error {
	var lastErr error
	for attempt := 0; attempt < migrationAttempts; attempt++ {
		done, err := tryMigration(ctx, db, version, stmt)
		if done {
			return nil
		}
		if err == nil {
			return nil
		}
		if !isContention(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("migration %d: still contended after %d attempts: %w",
		version, migrationAttempts, lastErr)
}

// isContention reports whether an error is SQLite telling us to try again.
// Matched on message text because the driver does not export a typed error for
// it; the alternative is treating a retryable condition as fatal.
func isContention(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") || strings.Contains(text, "database table is locked")
}

func tryMigration(ctx context.Context, db *sql.DB, version int, stmt string) (done bool, err error) {
	// A deferred transaction reads the migration registry before it asks for a
	// write lock. If another opener commits in between, SQLite cannot upgrade
	// that stale snapshot and returns SQLITE_BUSY immediately; busy_timeout is
	// intentionally not consulted for that case. Pin a connection and acquire
	// the write lock before reading so every retry starts from a current snapshot.
	conn, err := db.Conn(ctx)
	if err != nil {
		return false, fmt.Errorf("get connection for migration %d: %w", version, err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, fmt.Errorf("begin migration %d: %w", version, err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	var applied int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+migrationsTable+` WHERE version = ?`, version,
	).Scan(&applied); err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}
	if applied > 0 {
		return true, nil
	}

	// Statement and version bump land together, so a crash between them cannot
	// leave a migration applied but unrecorded — which would run it a second
	// time on the next open.
	if _, err := conn.ExecContext(ctx, stmt); err != nil {
		return false, fmt.Errorf("migration %d: %w", version, err)
	}
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO `+migrationsTable+` (version) VALUES (?)`, version,
	); err != nil {
		return false, fmt.Errorf("record migration %d: %w", version, err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, fmt.Errorf("commit migration %d: %w", version, err)
	}
	committed = true
	return true, nil
}
