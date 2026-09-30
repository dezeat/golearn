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
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	forgeapp "github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	corepack "github.com/dezeat/golearn/internal/adapters/pack"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coreapp "github.com/dezeat/golearn/internal/app"
)

// The adapter must satisfy the frozen contracts, checked at compile time so a
// signature drift is a build failure rather than a wiring surprise in the
// composition root.
var (
	_ ports.RunStore      = (*forgestore.Store)(nil)
	_ ports.DraftStore    = (*forgestore.Store)(nil)
	_ ports.DraftImporter = (*forgeapp.DraftAcceptor)(nil)
)

// The whole #121 lifecycle against real SQLite and the real import service:
// run -> draft -> accept -> library, with the draft gone and the questions
// practicable afterwards. The unit tests use fakes; this proves the seams
// actually meet.
func TestDraftBecomesLibraryContentOnlyThroughTheImportPath(t *testing.T) {
	ctx := context.Background()
	path, db := coreDB(t)
	store, err := forgestore.New(ctx, db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}

	runID := succeededRun(t, store)
	draftID, err := store.SaveDraft(ctx, domain.Draft{
		RunID: runID, Pack: testPack(), CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}

	var libraryQuestions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions`).Scan(&libraryQuestions); err != nil {
		t.Fatalf("count: %v", err)
	}
	if libraryQuestions != 0 {
		t.Fatalf("the draft leaked into the library before acceptance: %d questions", libraryQuestions)
	}

	importer := coreapp.NewImportService(
		corepack.NewReader(),
		coresqlite.NewTopicRepo(db),
		coresqlite.NewQuestionRepo(db),
	)
	acceptor := forgeapp.NewDraftAcceptor(importer, store)

	draft, err := store.GetDraft(ctx, draftID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}
	imported, err := acceptor.Accept(ctx, draft)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if imported.Inserted != 1 {
		t.Errorf("inserted = %d, want 1", imported.Inserted)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM questions`).Scan(&libraryQuestions); err != nil {
		t.Fatalf("count: %v", err)
	}
	if libraryQuestions != 1 {
		t.Errorf("want 1 library question after acceptance, got %d", libraryQuestions)
	}
	remaining, err := store.ListDrafts(ctx)
	if err != nil {
		t.Fatalf("ListDrafts: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("an accepted draft must be gone, %d remain", len(remaining))
	}

	// The generated question must carry its provenance markers into the
	// library, or a user could not tell generated content from hand-authored.
	var source string
	var confidence float64
	if err := db.QueryRow(`SELECT source, confidence FROM questions LIMIT 1`).Scan(&source, &confidence); err != nil {
		t.Fatalf("read question: %v", err)
	}
	_ = path
	if confidence >= 1.0 {
		t.Errorf("a generated question must carry confidence below the manual default, got %v", confidence)
	}
}

// Re-accepting after a crash between import and delete must not duplicate the
// content. D-007's hash dedup is what makes the import-then-delete order safe,
// so this is the test that licenses that ordering.
func TestReacceptingADraftDuplicatesNothing(t *testing.T) {
	ctx := context.Background()
	_, db := coreDB(t)
	store, err := forgestore.New(ctx, db)
	if err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}
	runID := succeededRun(t, store)
	draftID, err := store.SaveDraft(ctx, domain.Draft{RunID: runID, Pack: testPack()})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	draft, err := store.GetDraft(ctx, draftID)
	if err != nil {
		t.Fatalf("GetDraft: %v", err)
	}

	importer := coreapp.NewImportService(
		corepack.NewReader(),
		coresqlite.NewTopicRepo(db),
		coresqlite.NewQuestionRepo(db),
	)
	acceptor := forgeapp.NewDraftAcceptor(importer, store)

	first, err := acceptor.Accept(ctx, draft)
	if err != nil {
		t.Fatalf("first Accept: %v", err)
	}
	second, err := acceptor.Accept(ctx, draft)
	if err != nil {
		t.Fatalf("second Accept: %v", err)
	}

	if first.Inserted != 1 {
		t.Errorf("first accept inserted %d, want 1", first.Inserted)
	}
	if second.Inserted != 0 || second.Duplicates != 1 {
		t.Errorf("re-accepting must insert nothing and count a duplicate, got %+v", second)
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM questions`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 1 {
		t.Errorf("want 1 question after two accepts, got %d", total)
	}
}
