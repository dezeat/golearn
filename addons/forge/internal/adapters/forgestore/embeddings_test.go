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
	"math"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
)

func embeddingModel() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: "ollama", Model: "nomic-embed-text"}
}

func otherEmbeddingModel() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: "openai", Model: "text-embedding-3-small"}
}

// unit returns a deterministic unit vector pointing mostly along axis i, so
// two calls with different i are similar but distinguishable and the expected
// cosine can be reasoned about by hand rather than read off the implementation.
func unit(dim, i int) domain.Vector {
	v := make(domain.Vector, dim)
	for j := range v {
		v[j] = 0.1
	}
	v[i%dim] = 1
	return v
}

func TestAStoredVectorIsItsOwnNearestNeighbor(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	probe := unit(8, 0)
	if err := store.Put(ctx, 1, embeddingModel(), probe); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, 2, embeddingModel(), unit(8, 4)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Nearest(ctx, embeddingModel(), probe, 2)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 neighbors, got %d", len(got))
	}
	if got[0].QuestionID != 1 {
		t.Errorf("the identical vector must rank first, got question %d", got[0].QuestionID)
	}
	if math.Abs(got[0].Score-1.0) > 1e-9 {
		t.Errorf("a vector compared with itself must score 1.0, got %v", got[0].Score)
	}
	if got[1].Score >= got[0].Score {
		t.Errorf("neighbors must be ordered by descending score, got %v then %v", got[0].Score, got[1].Score)
	}
}

// D-020: the BLOB is the storage format, so a vector must survive the database
// rather than only the encoder. A round trip through SQLite is the assertion
// the domain-level encoding test cannot make.
func TestAVectorSurvivesTheDatabaseUnchanged(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	// Values chosen to exercise sign, fractional precision and exact zero.
	original := domain.Vector{-1.5, 0, 0.125, 3.25, -0.0625}
	if err := store.Put(ctx, 7, embeddingModel(), original); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Nearest(ctx, embeddingModel(), original, 1)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 neighbor, got %d", len(got))
	}
	// A lossy round trip moves the self-similarity off 1.0.
	if math.Abs(got[0].Score-1.0) > 1e-9 {
		t.Errorf("vector did not survive the database intact: self-similarity %v", got[0].Score)
	}
}

// The hard rule of D-020. A stored vector whose dimensionality does not match
// the probe came from a different embedding model wearing the same name — a
// re-pulled Ollama tag is the realistic case. Scoring what happens to compare
// and dropping the rest is the dangerous form: a duplicate that silently fell
// out of the result set reads exactly like no duplicate at all.
func TestASearchRefusesAStoredVectorOfAnotherDimension(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Written directly, because Put refuses this state at the write side. This
	// is the database an older Forge left behind, not one this code can make.
	insertRawVector(t, db, 2, embeddingModel(), unit(16, 0))

	got, err := store.Nearest(ctx, embeddingModel(), unit(8, 0), 5)
	if err == nil {
		t.Fatalf("a mixed-dimension corpus must fail the search, got %d neighbors and no error", len(got))
	}
	if got != nil {
		t.Errorf("a refused search must return no partial result, got %d neighbors", len(got))
	}
	for _, want := range []string{"8", "16"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name both dimensions so the mismatch is diagnosable; %q lacks %q", err, want)
		}
	}
}

// The write-side half of the same invariant: the corrupt state above is
// unreachable through the port, so it can only arrive from an older writer.
func TestPutRefusesAVectorOfADifferentDimensionForTheSameModel(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, 2, embeddingModel(), unit(16, 0)); err == nil {
		t.Fatal("storing a 16-dim vector under a model already holding 8-dim vectors must be refused")
	}
}

// Vectors from a different model are not candidates at all — they are excluded
// by the search rather than scored against it.
func TestASearchNeverScoresVectorsFromAnotherModel(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// A different model, and a different dimensionality — which is the normal
	// case between real embedding models. If the scope were ignored this would
	// surface as the dimension refusal above rather than as a wrong score.
	if err := store.Put(ctx, 2, otherEmbeddingModel(), unit(16, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Nearest(ctx, embeddingModel(), unit(8, 0), 5)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(got) != 1 || got[0].QuestionID != 1 {
		t.Errorf("only the searched model's vectors are candidates, got %v", got)
	}
}

func TestCountIsScopedToOneEmbeddingModel(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	// An empty corpus must report as empty, so the gate can say "nothing to
	// compare against" rather than "nothing was too similar".
	n, err := store.Count(ctx, embeddingModel())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Fatalf("an empty corpus must count 0, got %d", n)
	}

	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, 2, embeddingModel(), unit(8, 1)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, 3, otherEmbeddingModel(), unit(16, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	n, err = store.Count(ctx, embeddingModel())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 2 {
		t.Errorf("Count must exclude other models, want 2, got %d", n)
	}
}

func TestPutReplacesAVectorRatherThanAccumulating(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 3)); err != nil {
		t.Fatalf("re-Put: %v", err)
	}

	n, err := store.Count(ctx, embeddingModel())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("re-storing a question's vector must replace it, got %d rows", n)
	}

	got, err := store.Nearest(ctx, embeddingModel(), unit(8, 3), 1)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if math.Abs(got[0].Score-1.0) > 1e-9 {
		t.Errorf("the replacement vector must be the one stored, self-similarity %v", got[0].Score)
	}
}

func TestMissingReportsOnlyTheIdsWithoutAVector(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	if err := store.Put(ctx, 2, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Present, but under a different model — so still missing for this one.
	if err := store.Put(ctx, 4, otherEmbeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := store.Missing(ctx, embeddingModel(), []domain.LibraryQuestionID{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("Missing: %v", err)
	}
	want := []domain.LibraryQuestionID{1, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("want missing %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Missing must preserve input order: want %v, got %v", want, got)
		}
	}
}

func TestNearestHonorsTheRequestedLimit(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		if err := store.Put(ctx, domain.LibraryQuestionID(i), embeddingModel(), unit(8, i)); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	got, err := store.Nearest(ctx, embeddingModel(), unit(8, 1), 2)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 neighbors, got %d", len(got))
	}
}

// A search that asks for nothing is a caller bug, and returning an empty
// result would let it read as "no duplicates found".
func TestNearestRefusesANonPositiveLimit(t *testing.T) {
	store, _ := newStore(t)
	if _, err := store.Nearest(context.Background(), embeddingModel(), unit(8, 0), 0); err == nil {
		t.Error("a limit of 0 must be refused, not answered with an empty neighbor list")
	}
}

func TestPutRefusesAnEmptyVector(t *testing.T) {
	store, _ := newStore(t)
	if err := store.Put(context.Background(), 1, embeddingModel(), domain.Vector{}); err == nil {
		t.Error("an empty vector carries no similarity information and must be refused")
	}
}

// The falsifier for "additive really means additive". The core opens the
// database with foreign_keys=ON, so a foreign key from a Forge table to
// questions(id) would make a core-side delete fail — the offline binary would
// break because a user once ran Forge. TestForgeMigrationsNeverTouchCoreTables
// cannot see this: it compares pragma_table_info, and an inbound foreign key
// changes no column.
func TestStoredEmbeddingsDoNotBlockACoreSideDelete(t *testing.T) {
	store, db := newStore(t)
	ctx := context.Background()

	questionID := seedCoreQuestion(t, db)
	if err := store.Put(ctx, domain.LibraryQuestionID(questionID), embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM questions WHERE id = ?`, questionID); err != nil {
		t.Fatalf("a core-side delete must still succeed with embeddings stored; Forge added a constraint the core cannot see: %v", err)
	}
}

// Vectors must survive the process, not just the handle.
func TestStoredVectorsSurviveAReopen(t *testing.T) {
	path, db := coreDB(t)
	ctx := context.Background()

	store := openForgeStore(t, db)
	if err := store.Put(ctx, 1, embeddingModel(), unit(8, 0)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened := reopenCoreDB(t, path)
	store = openForgeStore(t, reopened)
	n, err := store.Count(ctx, embeddingModel())
	if err != nil {
		t.Fatalf("Count after reopen: %v", err)
	}
	if n != 1 {
		t.Errorf("want 1 vector after reopen, got %d", n)
	}
}

func openForgeStore(t *testing.T, db *sql.DB) *forgestore.Store {
	t.Helper()
	store, err := forgestore.New(context.Background(), db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}
	return store
}

func reopenCoreDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := coresqlite.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// seedCoreQuestion inserts a question the way the core owns it, so the
// core-side delete under test deletes a real row rather than nothing.
func seedCoreQuestion(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO topics (slug, name) VALUES ('go-concurrency', 'Go Concurrency')`)
	if err != nil {
		t.Fatalf("seed topic: %v", err)
	}
	topicID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("topic id: %v", err)
	}
	res, err = db.Exec(
		`INSERT INTO questions (topic_id, type, prompt, choices_json, correct_choice_ids_json, hash)
		 VALUES (?, 'single', 'Which keyword starts a goroutine?', '[]', '[]', 'seed-hash')`,
		topicID,
	)
	if err != nil {
		t.Fatalf("seed question: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("question id: %v", err)
	}
	return id
}

func insertRawVector(t *testing.T, db *sql.DB, questionID int64, model domain.ModelIdentity, v domain.Vector) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO forge_embeddings (question_id, provider, model, dim, vector) VALUES (?, ?, ?, ?, ?)`,
		questionID, model.Provider, model.Model, len(v), domain.MarshalVector(v),
	); err != nil {
		t.Fatalf("seed raw vector: %v", err)
	}
}
