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

package forgestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// draftSource is the reference name used when a draft's pack is validated, so
// a rejection reads as coming from a draft rather than from a file that does
// not exist.
const draftSource = "<forge draft>"

// SaveDraft writes a validated draft and returns its id.
//
// It refuses three things the no-junk rule depends on, rather than trusting the
// caller: a pack that is not valid under the standard rules, a pack that is not
// a generated pack (no provenance, wrong schema version), and a draft whose run
// has not succeeded. FORGE.md 8 says no draft is created during active
// generation and a canceled run leaves at most diagnostics — a store that
// accepts a draft for a running run makes that unenforceable, and the rule
// would then hold only as long as every caller remembered it.
//
// The write is a single row insert, so it is atomic without a transaction: a
// crash mid-write leaves the row absent, never half-present. The pack is stored
// whole, as the JSON encoding of the same structure the importer reads.
func (s *Store) SaveDraft(ctx context.Context, draft domain.Draft) (int64, error) {
	pack := draft.Pack
	coredomain.NormalizePack(&pack)

	if errs := coredomain.ValidatePack(&pack, draftSource); len(errs) > 0 {
		return 0, fmt.Errorf("save draft: pack is not valid (%d errors, first: %s)", len(errs), errs[0])
	}
	if pack.PackVersion != coredomain.PackVersionGenerated {
		return 0, fmt.Errorf("save draft: pack declares schema %q, want %q",
			pack.PackVersion, coredomain.PackVersionGenerated)
	}
	if pack.Provenance == nil {
		return 0, errors.New("save draft: a generated pack must carry provenance")
	}
	if pack.GenerationSpec == nil {
		return 0, errors.New("save draft: a generated pack must carry a generation spec")
	}
	// D-017 requires generated questions to sit strictly below the
	// hand-authored default, so provenance class is visible in the library
	// data and not only in a pack header. Enforcing it here rather than
	// trusting the generator is what makes it hold: the importer's default is
	// 1.0, so a question that simply omits confidence enters the library
	// looking hand-authored.
	for i := range pack.Questions {
		c := pack.Questions[i].Confidence
		if c == nil || *c >= coredomain.ManualConfidence {
			return 0, fmt.Errorf(
				"save draft: question %d must carry confidence below the hand-authored default %v, or it enters the library indistinguishable from manual content",
				i, coredomain.ManualConfidence)
		}
	}

	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM forge_runs WHERE id = ?`, draft.RunID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("save draft: no such run %d", draft.RunID)
	}
	if err != nil {
		return 0, fmt.Errorf("save draft: read run %d: %w", draft.RunID, err)
	}
	if domain.RunStatus(status) != domain.RunSucceeded {
		return 0, fmt.Errorf("save draft: run %d is %q, and only a succeeded run may produce a draft", draft.RunID, status)
	}

	packJSON, err := json.Marshal(pack)
	if err != nil {
		return 0, fmt.Errorf("save draft: encode pack: %w", err)
	}
	createdAt := draft.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO forge_drafts (run_id, pack_json, created_at) VALUES (?, ?, ?)`,
		draft.RunID, string(packJSON), createdAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("save draft: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("save draft: id: %w", err)
	}
	return id, nil
}

// ListDrafts returns all unresolved drafts, oldest first — the order the draft
// screen resolves them in, so the oldest junk is dealt with first.
func (s *Store) ListDrafts(ctx context.Context) ([]domain.Draft, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, pack_json, created_at FROM forge_drafts ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var drafts []domain.Draft
	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drafts: %w", err)
	}
	return drafts, nil
}

// GetDraft returns one draft, or domain.ErrDraftNotFound.
func (s *Store) GetDraft(ctx context.Context, id int64) (domain.Draft, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, pack_json, created_at FROM forge_drafts WHERE id = ?`, id)
	draft, err := scanDraft(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Draft{}, fmt.Errorf("draft %d: %w", id, domain.ErrDraftNotFound)
	}
	return draft, err
}

// DeleteDraft removes a draft.
//
// Idempotent by contract: it is the terminal step of both "Add to library" and
// "Discard", and a repeated resolve after a crash — where the import committed
// but the delete did not — must complete the lifecycle rather than fail it.
func (s *Store) DeleteDraft(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM forge_drafts WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete draft %d: %w", id, err)
	}
	return nil
}

// scanner covers both *sql.Row and *sql.Rows so one decode path serves the
// single-row and the iterating query.
type scanner interface {
	Scan(dest ...any) error
}

func scanDraft(sc scanner) (domain.Draft, error) {
	var (
		draft    domain.Draft
		packJSON string
	)
	if err := sc.Scan(&draft.ID, &draft.RunID, &packJSON, &draft.CreatedAt); err != nil {
		return domain.Draft{}, err
	}
	if err := json.Unmarshal([]byte(packJSON), &draft.Pack); err != nil {
		return domain.Draft{}, fmt.Errorf("decode pack for draft %d: %w", draft.ID, err)
	}
	return draft, nil
}
