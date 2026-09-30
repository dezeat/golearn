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

package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	forgeapp "github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coreapp "github.com/dezeat/golearn/internal/app"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

type fakeImporter struct {
	calls  []string
	result *coreapp.ImportResult
	err    error
}

func (f *fakeImporter) ImportPack(pack *coredomain.Pack, sourceName string) (*coreapp.ImportResult, error) {
	f.calls = append(f.calls, sourceName)
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &coreapp.ImportResult{FilesProcessed: 1, Inserted: len(pack.Questions)}, nil
}

type fakeDraftStore struct {
	deleted   []int64
	deleteErr error
}

func (f *fakeDraftStore) SaveDraft(context.Context, domain.Draft) (int64, error) { return 0, nil }
func (f *fakeDraftStore) ListDrafts(context.Context) ([]domain.Draft, error)     { return nil, nil }
func (f *fakeDraftStore) GetDraft(context.Context, int64) (domain.Draft, error) {
	return domain.Draft{}, domain.ErrDraftNotFound
}
func (f *fakeDraftStore) DeleteDraft(_ context.Context, id int64) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, id)
	return nil
}

func draftWithQuestions(n int) domain.Draft {
	questions := make([]coredomain.PackQuestion, n)
	for i := range questions {
		questions[i] = coredomain.PackQuestion{Type: coredomain.SingleSelect, Prompt: "q"}
	}
	return domain.Draft{
		ID:    7,
		RunID: 1,
		Pack: coredomain.Pack{
			PackVersion: coredomain.PackVersionGenerated,
			Topic:       coredomain.PackTopic{Slug: "go", Name: "Go"},
			Questions:   questions,
		},
	}
}

// FORGE.md 3.2 requires "Add to library" to run the standard atomic import
// (D-004), not a private insert path. If it did not, generated questions would
// escape the validation every hand-authored one is subject to.
func TestAcceptImportsThroughTheStandardPathThenRemovesTheDraft(t *testing.T) {
	importer := &fakeImporter{}
	drafts := &fakeDraftStore{}
	acceptor := forgeapp.NewDraftAcceptor(importer, drafts)

	imported, err := acceptor.Accept(context.Background(), draftWithQuestions(3))
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if len(importer.calls) != 1 {
		t.Fatalf("want exactly one import call, got %d", len(importer.calls))
	}
	if imported.Inserted != 3 {
		t.Errorf("inserted = %d, want 3", imported.Inserted)
	}
	if len(drafts.deleted) != 1 || drafts.deleted[0] != 7 {
		t.Errorf("the draft must be removed after a successful import, deleted = %v", drafts.deleted)
	}
}

// A failed import must leave the draft in place. Losing the pack because the
// library rejected it would destroy the only copy of the generated content.
func TestAFailedImportLeavesTheDraftIntact(t *testing.T) {
	importer := &fakeImporter{err: errors.New("validation failed")}
	drafts := &fakeDraftStore{}
	acceptor := forgeapp.NewDraftAcceptor(importer, drafts)

	if _, err := acceptor.Accept(context.Background(), draftWithQuestions(1)); err == nil {
		t.Fatal("want the import failure reported")
	}
	if len(drafts.deleted) != 0 {
		t.Errorf("a failed import must not delete the draft, deleted = %v", drafts.deleted)
	}
}

// The crash window between import and delete. The content is already in the
// library, so this is an unresolved draft rather than a failed acceptance —
// and the message has to say so, or a user re-runs a generation they already
// have.
func TestAnUnremovableDraftIsReportedAsUnresolvedNotAsAFailedImport(t *testing.T) {
	importer := &fakeImporter{}
	drafts := &fakeDraftStore{deleteErr: errors.New("database is locked")}
	acceptor := forgeapp.NewDraftAcceptor(importer, drafts)

	imported, err := acceptor.Accept(context.Background(), draftWithQuestions(2))
	if err == nil {
		t.Fatal("want the unresolved draft reported")
	}
	if !strings.Contains(err.Error(), "imported") {
		t.Errorf("the message must say the import succeeded, got: %v", err)
	}
	if imported.Inserted != 2 {
		t.Errorf("the import result must survive the delete failure, got %+v", imported)
	}
}

// Validation errors have to point at something a user can act on. A draft has
// never been a file, so naming a path would send them looking for one.
func TestImportErrorsNameTheDraftNotAFilePath(t *testing.T) {
	importer := &fakeImporter{}
	acceptor := forgeapp.NewDraftAcceptor(importer, &fakeDraftStore{})

	if _, err := acceptor.Accept(context.Background(), draftWithQuestions(1)); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if !strings.Contains(importer.calls[0], "draft") {
		t.Errorf("import source name = %q, want it to name the draft", importer.calls[0])
	}
	if strings.Contains(importer.calls[0], "/") || strings.HasSuffix(importer.calls[0], ".yaml") {
		t.Errorf("import source name looks like a file path: %q", importer.calls[0])
	}
}
