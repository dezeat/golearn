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
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
)

// D-020's argument for brute force rests entirely on the corpus staying small
// enough that a linear scan is irrelevant to the user. That argument is only as
// good as the benchmark testing it, so these are written to LOOK FOR the size
// at which it stops holding rather than to confirm that it does.
//
// What is held constant, and what is not:
//
//   - Dimensionality is fixed at 768 (benchDim), the width of the embedding
//     models the V1 profiles would use. A model at 1536 doubles both the bytes
//     and the arithmetic.
//   - The probe count is fixed at one pack's worth (benchPackSize). This is the
//     number that matters and the one a single-scan benchmark hides: the gate
//     scans the corpus once PER CANDIDATE, so the user-visible cost is the pack
//     size times the scan, not the scan.
//   - The page cache is WARM. testing.B repeats the same query, so from the
//     second iteration the corpus is in SQLite's cache and the OS's. A user's
//     first pack after launch is cold, and BenchmarkColdNearestScan measures
//     that separately because the difference is the disk, not the arithmetic.
//   - Vector CONTENT is random and the probe is unrelated to the corpus. Cosine
//     is exhaustive regardless of the values, so content does not change the
//     cost — but it does mean these numbers say nothing about result quality.
//   - One process, one goroutine, no concurrent TUI reader. WAL makes a
//     concurrent reader cheap but not free.
//
// Run: go test ./internal/adapters/forgestore/ -bench . -benchtime 1x -run '^$'
const (
	benchDim      = 768
	benchPackSize = 20
)

func benchCorpusSizes() []int { return []int{100, 1000, 2000, 5000, 10000, 50000, 100000} }

// seedCorpus builds a database holding n vectors for one model. It writes in
// one transaction because the benchmark is about reading, and a per-row commit
// would spend the whole run in fsync.
func seedCorpus(tb testing.TB, n int) (*forgestore.Store, *sql.DB, string) {
	tb.Helper()
	path := filepath.Join(tb.TempDir(), "golearn.db")
	db, err := coresqlite.Open(path)
	if err != nil {
		tb.Fatalf("open: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })

	store, err := forgestore.New(context.Background(), db)
	if err != nil {
		tb.Fatalf("forgestore.New: %v", err)
	}

	// Seeded explicitly: the corpus must be identical from run to run, or the
	// benchmark measures a different database each time.
	rng := rand.New(rand.NewSource(1))
	tx, err := db.Begin()
	if err != nil {
		tb.Fatalf("begin: %v", err)
	}
	stmt, err := tx.Prepare(
		`INSERT INTO forge_embeddings (question_id, provider, model, dim, vector) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		tb.Fatalf("prepare: %v", err)
	}
	model := embeddingModel()
	for i := 0; i < n; i++ {
		if _, err := stmt.Exec(int64(i+1), model.Provider, model.Model, benchDim,
			domain.MarshalVector(randomVector(rng, benchDim))); err != nil {
			tb.Fatalf("seed %d: %v", i, err)
		}
	}
	if err := stmt.Close(); err != nil {
		tb.Fatalf("close stmt: %v", err)
	}
	if err := tx.Commit(); err != nil {
		tb.Fatalf("commit: %v", err)
	}
	return store, db, path
}

func randomVector(rng *rand.Rand, dim int) domain.Vector {
	v := make(domain.Vector, dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	return v
}

// BenchmarkNearestScan measures one scan: the comfortable number, reported for
// completeness and not the one to cite.
func BenchmarkNearestScan(b *testing.B) {
	for _, n := range benchCorpusSizes() {
		b.Run(fmt.Sprintf("corpus=%d", n), func(b *testing.B) {
			store, _, _ := seedCorpus(b, n)
			probe := randomVector(rand.New(rand.NewSource(2)), benchDim)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := store.Nearest(ctx, embeddingModel(), probe, 5); err != nil {
					b.Fatalf("Nearest: %v", err)
				}
			}
		})
	}
}

// BenchmarkPackScreeningPass is the number that matters: the gate scans the
// corpus once per candidate, so a pack of 20 pays 20 scans before a user sees
// anything. Reporting the single-scan figure alone would understate the wait
// by that factor.
func BenchmarkPackScreeningPass(b *testing.B) {
	for _, n := range benchCorpusSizes() {
		b.Run(fmt.Sprintf("corpus=%d", n), func(b *testing.B) {
			store, _, _ := seedCorpus(b, n)
			rng := rand.New(rand.NewSource(3))
			probes := make([]domain.Vector, benchPackSize)
			for i := range probes {
				probes[i] = randomVector(rng, benchDim)
			}
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, probe := range probes {
					if _, err := store.Nearest(ctx, embeddingModel(), probe, 5); err != nil {
						b.Fatalf("Nearest: %v", err)
					}
				}
			}
		})
	}
}

// BenchmarkColdPackScreeningPass reopens the database each iteration so the
// scan starts without SQLite's page cache. It cannot drop the OS cache, so it
// is a floor on the cold cost rather than the true worst case — stated because
// a benchmark that quietly measured only the warm path is exactly the kind of
// comfortable number this file exists to avoid.
func BenchmarkColdPackScreeningPass(b *testing.B) {
	for _, n := range benchCorpusSizes() {
		b.Run(fmt.Sprintf("corpus=%d", n), func(b *testing.B) {
			_, db, path := seedCorpus(b, n)
			if err := db.Close(); err != nil {
				b.Fatalf("close: %v", err)
			}
			rng := rand.New(rand.NewSource(4))
			probes := make([]domain.Vector, benchPackSize)
			for i := range probes {
				probes[i] = randomVector(rng, benchDim)
			}
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				fresh, err := coresqlite.Open(path)
				if err != nil {
					b.Fatalf("reopen: %v", err)
				}
				store, err := forgestore.New(ctx, fresh)
				if err != nil {
					b.Fatalf("forgestore.New: %v", err)
				}
				b.StartTimer()

				for _, probe := range probes {
					if _, err := store.Nearest(ctx, embeddingModel(), probe, 5); err != nil {
						b.Fatalf("Nearest: %v", err)
					}
				}

				b.StopTimer()
				if err := fresh.Close(); err != nil {
					b.Fatalf("close: %v", err)
				}
				b.StartTimer()
			}
		})
	}
}

// TestEmbeddingFootprintPerThousandQuestions measures what a user's database
// actually grows by, which is the claim D-020 makes about storage. It is a
// test rather than a benchmark because the answer is deterministic: the same
// vectors always occupy the same bytes.
func TestEmbeddingFootprintPerThousandQuestions(t *testing.T) {
	const questions = 1000

	path := filepath.Join(t.TempDir(), "golearn.db")
	db, err := coresqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	store, err := forgestore.New(context.Background(), db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}

	// Measured after a checkpoint, because with WAL the pages live in the -wal
	// file until one happens and the main file would understate the growth.
	checkpoint(t, db)
	before := databaseBytes(t, path)

	rng := rand.New(rand.NewSource(5))
	ctx := context.Background()
	for i := 0; i < questions; i++ {
		if err := store.Put(ctx, domain.LibraryQuestionID(i+1), embeddingModel(), randomVector(rng, benchDim)); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}
	checkpoint(t, db)
	after := databaseBytes(t, path)

	grown := after - before
	payload := int64(questions * benchDim * 4)

	t.Logf("embedding footprint: %d questions at %d dims grew the database by %d bytes (%.2f MB); raw float32 payload is %d bytes; overhead factor %.2fx",
		questions, benchDim, grown, float64(grown)/(1024*1024), payload, float64(grown)/float64(payload))

	// The regression bound, locked from the observation rather than guessed.
	// The floor catches a change that silently stops storing full vectors; the
	// ceiling catches one that doubles the width — float64 instead of float32
	// being the exact mistake D-020 rules out.
	if grown < payload {
		t.Errorf("stored %d bytes for a %d-byte payload: the vectors are not being stored at full width", grown, payload)
	}
	if grown > payload*3/2 {
		t.Errorf("stored %d bytes for a %d-byte payload (%.2fx): the per-vector overhead regressed",
			grown, payload, float64(grown)/float64(payload))
	}
}

func checkpoint(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
}

func databaseBytes(t *testing.T, path string) int64 {
	t.Helper()
	var total int64
	for _, suffix := range []string{"", "-wal"} {
		info, err := os.Stat(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("stat %s: %v", path+suffix, err)
		}
		total += info.Size()
	}
	return total
}
