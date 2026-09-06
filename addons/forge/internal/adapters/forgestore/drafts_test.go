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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

func TestRunRoundTripsThroughItsLifecycle(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)

	started := time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC)
	id, err := store.StartRun(ctx, domain.Run{
		Spec:      testSpec(),
		Model:     domain.ModelIdentity{Provider: "ollama", Model: "qwen3:4b"},
		Verifier:  domain.ModelIdentity{Provider: "ollama", Model: "qwen3:4b"},
		StartedAt: started,
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	runs, err := store.RecentRuns(ctx, 10)
	if err != nil {
		t.Fatalf("RecentRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(runs))
	}
	if runs[0].Status != domain.RunRunning {
		t.Errorf("a started run must be running, got %q", runs[0].Status)
	}
	if runs[0].FinishedAt != nil {
		t.Error("a running run has no finish time")
	}
	if runs[0].Spec.Topic != "Go concurrency" {
		t.Errorf("spec did not round-trip: %+v", runs[0].Spec)
	}

	sources := []domain.SourceRef{{ID: "s1", URL: "https://example.test/a", Title: "A"}}
	if err := store.FinishRun(ctx, id, domain.RunSucceeded, sources,
		domain.Cost{InputTokens: 900, OutputTokens: 400, Attempts: 2}, "2 candidates repaired"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	runs, err = store.RecentRuns(ctx, 10)
	if err != nil {
		t.Fatalf("RecentRuns: %v", err)
	}
	got := runs[0]
	if got.Status != domain.RunSucceeded {
		t.Errorf("status = %q", got.Status)
	}
	if got.FinishedAt == nil {
		t.Error("a finished run must record when it finished")
	}
	if len(got.Sources) != 1 || got.Sources[0].ID != "s1" {
		t.Errorf("source refs did not round-trip: %+v", got.Sources)
	}
	if got.Cost.Attempts != 2 || got.Cost.InputTokens != 900 {
		t.Errorf("cost summary did not round-trip: %+v", got.Cost)
	}
	if got.Verifier.Model != "qwen3:4b" {
		t.Errorf("verifier identity did not round-trip: %+v", got.Verifier)
	}
}

// "Running" is not an outcome. Recording it as one would leave a run that
// looks finished but never was, which is exactly the state the run history
// exists to make diagnosable.
func TestFinishingARunRequiresATerminalStatus(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	id := succeededRun(t, store)

	err := store.FinishRun(ctx, id, domain.RunRunning, nil, domain.Cost{}, "")
	if err == nil {
		t.Fatal("finishing a run as 'running' must be refused")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Errorf("the refusal should say why, got: %v", err)
	}
}

func TestFinishingAnUnknownRunIsReportedNotIgnored(t *testing.T) {
	store, _ := newStore(t)
	err := store.FinishRun(context.Background(), 9999, domain.RunFailed, nil, domain.Cost{}, "x")
	if err == nil {
		t.Fatal("finishing a run that does not exist must be an error, not a silent no-op")
	}
}

func TestDraftSurvivesTheProcessThatWroteIt(t *testing.T) {
	ctx := context.Background()
	path, db := coreDB(t)
	store, err := forgestore.New(ctx, db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	runID := succeededRun(t, store)

	id, err := store.SaveDraft(ctx, domain.Draft{
		RunID:     runID,
		Pack:      testPack(),
		CreatedAt: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	_ = db.Close()

	// A crash is indistinguishable from this: the process is gone, the file
	// remains. The draft must still be there and still be complete.
	reopened, err := reopen(t, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	draft, err := reopened.GetDraft(ctx, id)
	if err != nil {
		t.Fatalf("GetDraft after reopen: %v", err)
	}
	if draft.QuestionCount() != 1 {
		t.Errorf("want 1 question, got %d", draft.QuestionCount())
	}
	prov, ok := draft.Provenance()
	if !ok {
		t.Fatal("provenance did not survive")
	}
	if prov.Model.String() != "ollama/qwen3:4b" {
		t.Errorf("model identity = %q", prov.Model)
	}
	spec, ok := draft.Spec()
	if !ok || spec.Topic != "Go concurrency" {
		t.Errorf("generation spec did not survive: %+v", spec)
	}
}

// The no-junk rule is only checkable if the store refuses to hold junk. A
// draft for a run that has not succeeded is precisely the artifact FORGE.md 8
// says cancellation must never leave behind.
func TestOnlyASucceededRunMayProduceADraft(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)

	for _, status := range []domain.RunStatus{domain.RunRunning, domain.RunFailed, domain.RunCanceled} {
		t.Run(string(status), func(t *testing.T) {
			runID, err := store.StartRun(ctx, domain.Run{Spec: testSpec(), StartedAt: time.Now()})
			if err != nil {
				t.Fatalf("StartRun: %v", err)
			}
			if status != domain.RunRunning {
				if err := store.FinishRun(ctx, runID, status, nil, domain.Cost{}, "x"); err != nil {
					t.Fatalf("FinishRun: %v", err)
				}
			}
			if _, err := store.SaveDraft(ctx, domain.Draft{RunID: runID, Pack: testPack()}); err == nil {
				t.Fatalf("a %q run must not be able to produce a draft", status)
			}
		})
	}
}

func TestDraftMustBeAValidGeneratedPack(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	runID := succeededRun(t, store)

	cases := []struct {
		name string
		want string
		mut  func(p *coredomain.Pack)
	}{
		{"invalid question", "not valid", func(p *coredomain.Pack) {
			p.Questions[0].CorrectChoiceIDs = []string{"nonexistent"}
		}},
		{"no questions", "not valid", func(p *coredomain.Pack) { p.Questions = nil }},
		{"older schema version", "schema", func(p *coredomain.Pack) { p.PackVersion = "0.1.0" }},
		{"missing provenance", "provenance", func(p *coredomain.Pack) { p.Provenance = nil }},
		{"missing generation spec", "generation spec", func(p *coredomain.Pack) { p.GenerationSpec = nil }},
		{"question omits confidence", "confidence", func(p *coredomain.Pack) { p.Questions[0].Confidence = nil }},
		{"question claims hand-authored confidence", "confidence", func(p *coredomain.Pack) {
			manual := coredomain.ManualConfidence
			p.Questions[0].Confidence = &manual
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack := testPack()
			tc.mut(&pack)
			_, err := store.SaveDraft(ctx, domain.Draft{RunID: runID, Pack: pack})
			if err == nil {
				t.Fatal("want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal should mention %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestDraftsListOldestFirst(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	runID := succeededRun(t, store)

	base := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	for i, offset := range []time.Duration{2 * time.Hour, 0, time.Hour} {
		pack := testPack()
		pack.Questions[0].Prompt = "Question variant " + string(rune('A'+i))
		if _, err := store.SaveDraft(ctx, domain.Draft{
			RunID: runID, Pack: pack, CreatedAt: base.Add(offset),
		}); err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
	}

	drafts, err := store.ListDrafts(ctx)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(drafts) != 3 {
		t.Fatalf("want 3 drafts, got %d", len(drafts))
	}
	for i := 1; i < len(drafts); i++ {
		if drafts[i].CreatedAt.Before(drafts[i-1].CreatedAt) {
			t.Errorf("drafts are not oldest-first: %v then %v",
				drafts[i-1].CreatedAt, drafts[i].CreatedAt)
		}
	}
}

// Both resolutions end in a delete, and a crash can land between the import
// and the delete. Repeating the resolve must complete the lifecycle, not fail
// it.
func TestDeletingADraftIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store, _ := newStore(t)
	runID := succeededRun(t, store)

	id, err := store.SaveDraft(ctx, domain.Draft{RunID: runID, Pack: testPack()})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := store.DeleteDraft(ctx, id); err != nil {
			t.Fatalf("DeleteDraft call %d: %v", i+1, err)
		}
	}
	if _, err := store.GetDraft(ctx, id); !errors.Is(err, domain.ErrDraftNotFound) {
		t.Errorf("want ErrDraftNotFound, got %v", err)
	}
}

// A draft is not library content until the import path accepts it (#121
// acceptance). Nothing the practice engine reads may see it.
func TestADraftIsNotVisibleAsLibraryContent(t *testing.T) {
	ctx := context.Background()
	store, db := newStore(t)
	runID := succeededRun(t, store)

	if _, err := store.SaveDraft(ctx, domain.Draft{RunID: runID, Pack: testPack()}); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	for _, table := range []string{"questions", "topics"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("saving a draft put %d row(s) into the library table %q", n, table)
		}
	}
}

func reopen(t *testing.T, path string) (*forgestore.Store, error) {
	t.Helper()
	db, err := openCore(t, path)
	if err != nil {
		return nil, err
	}
	return forgestore.New(context.Background(), db)
}
