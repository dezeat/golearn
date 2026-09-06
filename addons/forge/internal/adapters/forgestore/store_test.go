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

package forgestore_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

func coreDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "golearn.db")
	db, err := coresqlite.Open(path)
	if err != nil {
		t.Fatalf("core open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return path, db
}

func newStore(t *testing.T) (*forgestore.Store, *sql.DB) {
	t.Helper()
	_, db := coreDB(t)
	store, err := forgestore.New(context.Background(), db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}
	return store, db
}

func testSpec() domain.GenerationSpec {
	return domain.GenerationSpec{
		Topic: "Go concurrency", Count: 2,
		Difficulty: coredomain.DifficultyEasy, Language: "en",
	}
}

func testPack() coredomain.Pack {
	return coredomain.Pack{
		PackVersion: coredomain.PackVersionGenerated,
		Topic:       coredomain.PackTopic{Slug: "go-concurrency", Name: "Go Concurrency"},
		GenerationSpec: func() *coredomain.GenerationSpec {
			s := testSpec()
			return &s
		}(),
		Provenance: &coredomain.Provenance{
			GeneratedAt:  time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC),
			Model:        coredomain.ModelIdentity{Provider: "ollama", Model: "qwen3:4b"},
			ForgeVersion: "0.3.0",
		},
		Questions: []coredomain.PackQuestion{{
			Type:             coredomain.SingleSelect,
			Prompt:           "Which keyword starts a goroutine?",
			Choices:          []coredomain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
			CorrectChoiceIDs: []string{"a"},
			Difficulty:       coredomain.DifficultyEasy,
			Source:           "llm:ollama",
			Confidence:       generatedConfidence(),
		}},
	}
}

// succeededRun creates a run and drives it to succeeded, which is the only
// state from which a draft may be saved.
func succeededRun(t *testing.T, store *forgestore.Store) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := store.StartRun(ctx, domain.Run{
		Spec:      testSpec(),
		Model:     domain.ModelIdentity{Provider: "ollama", Model: "qwen3:4b"},
		StartedAt: time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.FinishRun(ctx, id, domain.RunSucceeded, nil, domain.Cost{Attempts: 1}, "ok"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	return id
}

func columnsOf(t *testing.T, db *sql.DB, table string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT name, type FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	cols := map[string]string{}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = typ
	}
	return cols
}

// The rule the whole compatibility story rests on. Additive Forge tables leave
// a Forge-extended database openable by the offline binary; an ALTER of a core
// table would advance the core's schema behind its back, and a user who tried
// Forge once would be locked out of the product D-015 promises is unchanged.
//
// A doc comment cannot enforce this. This test can.
func TestForgeMigrationsNeverTouchCoreTables(t *testing.T) {
	coreTables := []string{"users", "topics", "questions", "sessions", "attempts"}

	_, untouched := coreDB(t)
	before := map[string]map[string]string{}
	for _, table := range coreTables {
		before[table] = columnsOf(t, untouched, table)
	}

	_, forged := coreDB(t)
	if _, err := forgestore.New(context.Background(), forged); err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}

	for _, table := range coreTables {
		after := columnsOf(t, forged, table)
		if len(after) != len(before[table]) {
			t.Errorf("Forge changed the column count of core table %q: %d -> %d",
				table, len(before[table]), len(after))
			continue
		}
		for name, typ := range before[table] {
			if after[name] != typ {
				t.Errorf("Forge altered core table %q column %q: %q -> %q",
					table, name, typ, after[name])
			}
		}
	}
}

// A-7's other half: Forge must not append to the core's migration counter,
// because the core's applied-check is per version number and a claimed version
// silently skips the core's own next migration.
func TestForgeKeepsItsOwnMigrationRegistry(t *testing.T) {
	_, db := coreDB(t)
	var coreBefore int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&coreBefore); err != nil {
		t.Fatalf("count core migrations: %v", err)
	}
	if _, err := forgestore.New(context.Background(), db); err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}

	var coreAfter int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&coreAfter); err != nil {
		t.Fatalf("count core migrations: %v", err)
	}
	if coreAfter != coreBefore {
		t.Errorf("Forge wrote %d row(s) into the core's migration registry", coreAfter-coreBefore)
	}

	var forgeVersions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM forge_schema_migrations`).Scan(&forgeVersions); err != nil {
		t.Fatalf("Forge must track its own migrations: %v", err)
	}
	if forgeVersions == 0 {
		t.Error("Forge recorded no migrations of its own")
	}
}

// The invariant is that reopening applies nothing a second time, which is not
// the same as a particular registry size. Pinning the literal count made this
// test fail the moment a migration was added — reporting a schema change as an
// idempotency defect, which is the wrong alarm.
func TestMigrationsAreIdempotentAcrossReopens(t *testing.T) {
	_, db := coreDB(t)
	if _, err := forgestore.New(context.Background(), db); err != nil {
		t.Fatalf("first open: %v", err)
	}
	afterFirst := countForgeMigrations(t, db)
	if afterFirst == 0 {
		t.Fatal("the first open recorded no migrations at all")
	}

	for i := 0; i < 2; i++ {
		if _, err := forgestore.New(context.Background(), db); err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
	}

	if got := countForgeMigrations(t, db); got != afterFirst {
		t.Errorf("reopening re-applied migrations: %d recorded after one open, %d after three", afterFirst, got)
	}
}

func countForgeMigrations(t *testing.T, db *sql.DB) int {
	t.Helper()
	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM forge_schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("count forge migrations: %v", err)
	}
	return versions
}

func openCore(t *testing.T, path string) (*sql.DB, error) {
	t.Helper()
	db, err := coresqlite.Open(path)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, nil
}

// generatedConfidence returns a pointer to the generated-content marker, which
// the pack schema carries as an optional field.
func generatedConfidence() *float64 {
	c := coredomain.GeneratedConfidence
	return &c
}

// Two golearn windows opening at once is an ordinary thing for a local tool.
// With the applied-check outside the transaction that applies the migration,
// both openers see "not applied", both try to record it, and one fails on the
// primary key — on a database whose schema is perfectly valid.
func TestConcurrentOpensDoNotCollideOnMigrations(t *testing.T) {
	_, db := coreDB(t)

	const openers = 8
	errs := make(chan error, openers)
	start := make(chan struct{})
	for i := 0; i < openers; i++ {
		go func() {
			<-start
			_, err := forgestore.New(context.Background(), db)
			errs <- err
		}()
	}
	close(start)

	for i := 0; i < openers; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent open %d failed: %v", i, err)
		}
	}

	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM forge_schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("count: %v", err)
	}
	if versions != len(forgestore.MigrationCount()) {
		t.Errorf("want %d recorded migrations, got %d", len(forgestore.MigrationCount()), versions)
	}
}
