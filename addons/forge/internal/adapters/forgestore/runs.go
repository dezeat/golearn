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
	"fmt"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// StartRun records a run beginning and returns its id.
func (s *Store) StartRun(ctx context.Context, run domain.Run) (int64, error) {
	specJSON, err := json.Marshal(run.Spec)
	if err != nil {
		return 0, fmt.Errorf("encode generation spec: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO forge_runs
			(spec_json, provider, model, verifier_provider, verifier_model, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(specJSON), run.Model.Provider, run.Model.Model,
		run.Verifier.Provider, run.Verifier.Model,
		string(domain.RunRunning), run.StartedAt.UTC())
	if err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("run id: %w", err)
	}
	return id, nil
}

// FinishRun records the terminal state of a run.
//
// The parameter list is fixed rather than an open map of columns, because the
// fields a run may accumulate are exactly the ones FORGE.md 8 permits. An
// open-ended update is how raw prompts and request mechanics end up persisted
// despite the rule forbidding them.
func (s *Store) FinishRun(ctx context.Context, id int64, status domain.RunStatus, sources []domain.SourceRef, cost domain.Cost, diagnostic string) error {
	if status == domain.RunRunning {
		return fmt.Errorf("finish run %d: %q is not a terminal status", id, status)
	}
	if sources == nil {
		sources = []domain.SourceRef{}
	}
	sourcesJSON, err := json.Marshal(sources)
	if err != nil {
		return fmt.Errorf("encode source refs: %w", err)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE forge_runs
		SET status = ?, finished_at = ?, sources_json = ?,
		    input_tokens = ?, output_tokens = ?, attempts = ?, diagnostic = ?
		WHERE id = ?`,
		string(status), time.Now().UTC(), string(sourcesJSON),
		cost.InputTokens, cost.OutputTokens, cost.Attempts, diagnostic, id)
	if err != nil {
		return fmt.Errorf("finish run %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("finish run %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("finish run %d: no such run", id)
	}
	return nil
}

// RecentRuns returns the most recent runs, newest first.
func (s *Store) RecentRuns(ctx context.Context, limit int) ([]domain.Run, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, spec_json, provider, model, verifier_provider, verifier_model,
		       status, started_at, finished_at, sources_json,
		       input_tokens, output_tokens, attempts, diagnostic
		FROM forge_runs
		ORDER BY started_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []domain.Run
	for rows.Next() {
		var (
			run          domain.Run
			specJSON     string
			sourcesJSON  string
			status       string
			finishedAt   sql.NullTime
			verifierProv string
			verifierName string
		)
		if err := rows.Scan(&run.ID, &specJSON, &run.Model.Provider, &run.Model.Model,
			&verifierProv, &verifierName, &status, &run.StartedAt, &finishedAt,
			&sourcesJSON, &run.Cost.InputTokens, &run.Cost.OutputTokens,
			&run.Cost.Attempts, &run.Diagnostic); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if err := json.Unmarshal([]byte(specJSON), &run.Spec); err != nil {
			return nil, fmt.Errorf("decode generation spec for run %d: %w", run.ID, err)
		}
		if err := json.Unmarshal([]byte(sourcesJSON), &run.Sources); err != nil {
			return nil, fmt.Errorf("decode source refs for run %d: %w", run.ID, err)
		}
		run.Status = domain.RunStatus(status)
		run.Verifier = domain.ModelIdentity{Provider: verifierProv, Model: verifierName}
		if finishedAt.Valid {
			finished := finishedAt.Time
			run.FinishedAt = &finished
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runs: %w", err)
	}
	return runs, nil
}
