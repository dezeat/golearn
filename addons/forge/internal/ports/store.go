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

package ports

import (
	"context"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// RunStore persists the minimal run history FORGE.md 8 permits.
//
// The interface is narrow on purpose. There is no UpdateRun taking arbitrary
// fields, because the fields a run may accumulate are exactly the ones named
// in domain.Run — and an open-ended update is how raw prompts and retry
// mechanics end up persisted despite the rule forbidding it.
type RunStore interface {
	// StartRun records a run beginning and returns its id.
	StartRun(ctx context.Context, run domain.Run) (int64, error)

	// FinishRun records the terminal state: status, finish time, source
	// references, cost summary and one concise diagnostic.
	FinishRun(ctx context.Context, id int64, status domain.RunStatus, sources []domain.SourceRef, cost domain.Cost, diagnostic string) error

	// RecentRuns returns the most recent runs, newest first, for the run-info
	// surface that makes a failure diagnosable.
	RecentRuns(ctx context.Context, limit int) ([]domain.Run, error)
}

// DraftStore persists preview-ready packs that have not entered the library.
//
// The no-junk rule (FORGE.md 3.3) rests on two properties this interface must
// guarantee: a draft is written atomically or not at all, so a crash mid-save
// cannot leave a partial pack; and a draft is never library content, so
// nothing in the core's practice path can see it.
type DraftStore interface {
	// SaveDraft writes a validated draft atomically and returns its id.
	//
	// Implementations must reject a draft whose pack is not a valid generated
	// pack, and must reject one whose run has not succeeded. The no-junk rule
	// is only checkable if the store refuses to hold junk in the first place.
	SaveDraft(ctx context.Context, draft domain.Draft) (int64, error)

	// ListDrafts returns all unresolved drafts, oldest first — the order the
	// draft screen resolves them in.
	ListDrafts(ctx context.Context) ([]domain.Draft, error)

	// GetDraft returns one draft, or domain.ErrDraftNotFound.
	GetDraft(ctx context.Context, id int64) (domain.Draft, error)

	// DeleteDraft removes a draft. It is the terminal step of both "Add to
	// library" and "Discard", and is idempotent so a repeated resolve after a
	// crash is not an error.
	DeleteDraft(ctx context.Context, id int64) error
}

// DraftImporter moves an accepted draft into the library.
//
// It exists as its own interface because the acceptance path is not a store
// operation: FORGE.md 3.2 requires "Add to library" to run the *standard*
// atomic import (D-004), the same all-or-nothing path a hand-authored file
// takes. Modeling it as a DraftStore method would invite an implementation
// that inserted rows directly and quietly bypassed the validation every
// imported question is subject to.
type DraftImporter interface {
	// Accept imports the draft's pack through the standard import path and
	// then deletes the draft. A failed import leaves the draft in place.
	Accept(ctx context.Context, draft domain.Draft) (Imported, error)
}

// Imported reports what an accepted draft contributed to the library.
type Imported struct {
	Inserted   int
	Duplicates int
}

// LibraryReader reads existing library content for the similarity gate.
//
// Read-only by construction. FORGE.md 7 forbids modifying existing library
// content automatically, and the cheapest way to keep that promise is to give
// the gate no interface through which it could.
type LibraryReader interface {
	// QuestionsByTopic returns the stored questions for a topic, which is the
	// corpus a candidate is compared against.
	QuestionsByTopic(ctx context.Context, topicID int64) ([]coredomain.Question, error)
}
