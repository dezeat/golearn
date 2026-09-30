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

// Package app holds Forge's use cases: the logic that composes ports without
// knowing which adapter implements them.
package app

import (
	"context"
	"fmt"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coreapp "github.com/dezeat/golearn/internal/app"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// packImporter is the core's import use case, narrowed to the one method this
// needs. Depending on the interface rather than the concrete service is what
// lets the acceptance path be tested without a database, and it states plainly
// that Forge uses the standard import and nothing else of it.
type packImporter interface {
	ImportPack(pack *coredomain.Pack, sourceName string) (*coreapp.ImportResult, error)
}

// DraftAcceptor implements the "Add to library" action.
type DraftAcceptor struct {
	importer packImporter
	drafts   ports.DraftStore
}

// NewDraftAcceptor wires the acceptance path.
func NewDraftAcceptor(importer packImporter, drafts ports.DraftStore) *DraftAcceptor {
	return &DraftAcceptor{importer: importer, drafts: drafts}
}

// Accept imports a draft's pack through the standard atomic import path and
// then deletes the draft.
//
// The order is deliberate and so is its failure mode. Importing first means a
// crash between the two steps leaves the draft in place with its content
// already in the library — the user resolves it again, and D-007's hash dedup
// turns the second import into zero inserts. Deleting first would mean the
// same crash loses the pack entirely, with nothing to retry from.
//
// A failed import leaves the draft untouched, so nothing is lost and the
// failure is reported rather than swallowed.
func (a *DraftAcceptor) Accept(ctx context.Context, draft domain.Draft) (ports.Imported, error) {
	pack := draft.Pack
	result, err := a.importer.ImportPack(&pack, draftSourceName(draft))
	if err != nil {
		return ports.Imported{}, fmt.Errorf("accept draft %d: %w", draft.ID, err)
	}

	if err := a.drafts.DeleteDraft(ctx, draft.ID); err != nil {
		// The library already holds the content, so this is not a failed
		// acceptance — it is an unresolved draft. Say so, rather than
		// reporting an import failure that did not happen.
		return ports.Imported{Inserted: result.Inserted, Duplicates: result.Duplicates},
			fmt.Errorf("draft %d was imported but could not be removed; resolving it again is safe: %w", draft.ID, err)
	}

	return ports.Imported{Inserted: result.Inserted, Duplicates: result.Duplicates}, nil
}

// draftSourceName gives validation errors a location a user can act on. It
// names the draft, never a file path, because a draft has never been a file.
func draftSourceName(draft domain.Draft) string {
	return fmt.Sprintf("forge draft %d", draft.ID)
}
