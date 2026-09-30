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

package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// seedDraft puts a realistic finished draft in a fresh database, the way a
// completed run would.
func seedDraft(t *testing.T, dbPath string) int64 {
	t.Helper()
	ctx := context.Background()
	db, err := coresqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	store, err := forgestore.New(ctx, db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}
	runID, err := store.StartRun(ctx, domain.Run{
		Spec:      domain.GenerationSpec{Topic: "Go concurrency", Count: 1},
		Model:     domain.ModelIdentity{Provider: "ollama", Model: "test-model"},
		StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := store.FinishRun(ctx, runID, domain.RunSucceeded, nil, domain.Cost{Attempts: 1}, "ok"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	confidence := coredomain.GeneratedConfidence
	draftID, err := store.SaveDraft(ctx, domain.Draft{
		RunID: runID,
		Pack: coredomain.Pack{
			PackVersion:    coredomain.PackVersionGenerated,
			Topic:          coredomain.PackTopic{Slug: "go-concurrency", Name: "Go concurrency"},
			GenerationSpec: &coredomain.GenerationSpec{Topic: "Go concurrency", Count: 1},
			Provenance: &coredomain.Provenance{
				GeneratedAt:  time.Now(),
				Model:        coredomain.ModelIdentity{Provider: "ollama", Model: "test-model"},
				ForgeVersion: "test",
			},
			Questions: []coredomain.PackQuestion{{
				Type:             coredomain.SingleSelect,
				Prompt:           "Which keyword starts a goroutine?",
				Choices:          []coredomain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
				CorrectChoiceIDs: []string{"a"},
				Source:           "llm:ollama",
				Confidence:       &confidence,
			}},
		},
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	return draftID
}

func libraryQuestionCount(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// The demo path, through the actual command surface rather than through the
// store: list, show, add, and confirm the draft became library content and is
// no longer a draft.
//
// The unit tests cover the store and the acceptance use case; this covers the
// configuration that ships, which is the one the pipeline's ungrounded-critique
// defect proved nothing else was covering.
func TestTheDraftCommandCompletesTheAcceptanceLoop(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "golearn.db")
	draftID := seedDraft(t, dbPath)
	id := strconv.FormatInt(draftID, 10)

	var out, errOut bytes.Buffer
	if code := runDrafts([]string{"list", "--db", dbPath}, &out, &errOut); code != 0 {
		t.Fatalf("drafts list exited %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Go concurrency") {
		t.Errorf("list did not name the draft's topic:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "ollama/test-model") {
		t.Errorf("list did not name the model that produced it:\n%s", out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := runDrafts([]string{"show", id, "--db", dbPath}, &out, &errOut); code != 0 {
		t.Fatalf("drafts show exited %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Which keyword starts a goroutine?") {
		t.Errorf("show did not render the question:\n%s", out.String())
	}

	if n := libraryQuestionCount(t, dbPath); n != 0 {
		t.Fatalf("the draft was library content before acceptance: %d questions", n)
	}

	out.Reset()
	errOut.Reset()
	if code := runDrafts([]string{"add", id, "--db", dbPath}, &out, &errOut); code != 0 {
		t.Fatalf("drafts add exited %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "1 inserted") {
		t.Errorf("add did not report the insert:\n%s", out.String())
	}
	if n := libraryQuestionCount(t, dbPath); n != 1 {
		t.Errorf("want 1 library question after acceptance, got %d", n)
	}

	out.Reset()
	errOut.Reset()
	if code := runDrafts([]string{"list", "--db", dbPath}, &out, &errOut); code != 0 {
		t.Fatalf("drafts list exited %d", code)
	}
	if !strings.Contains(out.String(), "No unresolved drafts") {
		t.Errorf("an accepted draft must be gone:\n%s", out.String())
	}
}

// Discard removes the draft and adds nothing to the library.
func TestDiscardingADraftAddsNothingToTheLibrary(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "golearn.db")
	draftID := seedDraft(t, dbPath)

	var out, errOut bytes.Buffer
	code := runDrafts([]string{"discard", strconv.FormatInt(draftID, 10), "--db", dbPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("drafts discard exited %d: %s", code, errOut.String())
	}
	if n := libraryQuestionCount(t, dbPath); n != 0 {
		t.Errorf("discard put %d question(s) into the library", n)
	}

	out.Reset()
	_ = runDrafts([]string{"list", "--db", dbPath}, &out, &errOut)
	if !strings.Contains(out.String(), "No unresolved drafts") {
		t.Errorf("the discarded draft is still listed:\n%s", out.String())
	}
}

// An id that does not exist must fail rather than exiting 0 having done
// nothing — the rule the whole CLI is held to.
func TestResolvingAnUnknownDraftFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "golearn.db")
	seedDraft(t, dbPath)

	for _, action := range []string{"show", "add"} {
		var out, errOut bytes.Buffer
		if code := runDrafts([]string{action, "9999", "--db", dbPath}, &out, &errOut); code == 0 {
			t.Errorf("drafts %s on an unknown id exited 0", action)
		}
	}
}
