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

package sqlite_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/sqlite"

	_ "modernc.org/sqlite"
)

// rawDB opens a database file without going through Open, so a test can put
// it into a shape that predates — or postdates — what this binary ships.
func rawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func rawExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func countRows(t *testing.T, path, table string) int {
	t.Helper()
	db := rawDB(t, path)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// D-014's promise is that no open can destroy user data. This is the case that
// actually broke it: a database written before migration tracking existed, so
// the old startup path recognized nothing and dropped every table.
func TestPopulatedLegacyDatabaseSurvivesAnOpenAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	seed := rawDB(t, path)
	rawExec(t, seed, `
		CREATE TABLE topics (id INTEGER PRIMARY KEY, slug TEXT, name TEXT);
		INSERT INTO topics (slug, name) VALUES ('go', 'Go');
		INSERT INTO topics (slug, name) VALUES ('sql', 'SQL');
	`)
	_ = seed.Close()

	db, err := sqlite.Open(path)
	if db != nil {
		_ = db.Close()
	}
	if err == nil {
		t.Fatal("opening an untracked legacy database must refuse, not proceed")
	}
	if !errors.Is(err, sqlite.ErrIncompatibleSchema) {
		t.Errorf("want ErrIncompatibleSchema, got %v", err)
	}
	if got := countRows(t, path, "topics"); got != 2 {
		t.Errorf("user data destroyed: want 2 topics, got %d", got)
	}
}

// D-014 names the one consented destructive path; a refusal that does not point
// at it leaves the user stuck with a database they cannot open or discard.
func TestSchemaRefusalNamesTheConsentedRecoveryPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	seed := rawDB(t, path)
	rawExec(t, seed, `CREATE TABLE questions (id INTEGER PRIMARY KEY, prompt TEXT);`)
	_ = seed.Close()

	db, err := sqlite.Open(path)
	if db != nil {
		_ = db.Close()
	}
	if err == nil {
		t.Fatal("want a refusal")
	}
	if !strings.Contains(err.Error(), "golearn db reset --yes") {
		t.Errorf("refusal must name the recovery command, got: %v", err)
	}
}

// The gap D-014 records as "there is no schema-newer-than-expected branch at
// all". A database written by a later binary must not be silently opened and
// half-migrated by this one.
func TestNewerSchemaRefusesToOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newer.db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = db.Close()

	future := rawDB(t, path)
	rawExec(t, future, `INSERT INTO schema_migrations (version) VALUES (9999)`)
	rawExec(t, future, `INSERT INTO topics (slug, name) VALUES ('go', 'Go')`)
	_ = future.Close()

	db2, err := sqlite.Open(path)
	if db2 != nil {
		_ = db2.Close()
	}
	if err == nil {
		t.Fatal("a database from a newer binary must be refused, not opened")
	}
	if !errors.Is(err, sqlite.ErrNewerSchema) {
		t.Errorf("want ErrNewerSchema, got %v", err)
	}
	if got := countRows(t, path, "topics"); got != 1 {
		t.Errorf("user data destroyed: want 1 topic, got %d", got)
	}
}

// A tracked database whose recorded version predates a column the code reads is
// migrated forward where possible and refused where not — never reset. The old
// path reset it.
func TestTrackedDatabaseMissingARequiredColumnIsRefusedNotReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.db")
	seed := rawDB(t, path)
	rawExec(t, seed, `
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY);
		INSERT INTO schema_migrations (version) VALUES (1);
		CREATE TABLE users (id INTEGER PRIMARY KEY, handle TEXT);
		CREATE TABLE topics (id INTEGER PRIMARY KEY, slug TEXT, name TEXT);
		INSERT INTO topics (slug, name) VALUES ('go', 'Go');
		CREATE TABLE sessions (id INTEGER PRIMARY KEY, topic_id INTEGER);
		CREATE TABLE attempts (id INTEGER PRIMARY KEY, session_id INTEGER);
	`)
	_ = seed.Close()

	db, err := sqlite.Open(path)
	if db != nil {
		_ = db.Close()
	}
	if err == nil {
		t.Fatal("a tracked database missing sessions.user_id must be refused")
	}
	if !errors.Is(err, sqlite.ErrIncompatibleSchema) {
		t.Errorf("want ErrIncompatibleSchema, got %v", err)
	}
	if got := countRows(t, path, "topics"); got != 1 {
		t.Errorf("user data destroyed: want 1 topic, got %d", got)
	}
}

// The guards must not make the ordinary paths stricter: a fresh database and a
// reopened one both still work.
func TestFreshAndReopenedDatabasesStillOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.db")
	for i, label := range []string{"fresh", "reopen"} {
		db, err := sqlite.Open(path)
		if err != nil {
			t.Fatalf("%s open: %v", label, err)
		}
		if i == 0 {
			if _, err := db.Exec(`INSERT INTO topics (slug, name) VALUES ('go','Go')`); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
		_ = db.Close()
	}
	if got := countRows(t, path, "topics"); got != 1 {
		t.Errorf("want 1 topic across reopen, got %d", got)
	}
}

// Forge extends the same database with its own additive tables and its own
// migration registry. That is a compatible schema for the core, not a newer
// one: the offline binary must keep working for a user who has used Forge.
func TestForgeExtendedDatabaseStaysOpenableByTheCore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forged.db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO topics (slug, name) VALUES ('go','Go')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = db.Close()

	forge := rawDB(t, path)
	rawExec(t, forge, `
		CREATE TABLE forge_schema_migrations (version INTEGER PRIMARY KEY);
		INSERT INTO forge_schema_migrations (version) VALUES (1);
		CREATE TABLE forge_runs (id INTEGER PRIMARY KEY, status TEXT NOT NULL);
	`)
	_ = forge.Close()

	db2, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("core must still open a Forge-extended database: %v", err)
	}
	_ = db2.Close()
	if got := countRows(t, path, "topics"); got != 1 {
		t.Errorf("want 1 topic, got %d", got)
	}
	if got := countRows(t, path, "forge_runs"); got != 0 {
		t.Errorf("core must not disturb Forge tables, got %d rows", got)
	}
}
