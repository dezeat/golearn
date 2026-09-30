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
	"fmt"
	"sort"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// The adapter is the whole of D-020's backend: BLOB vectors in the database
// the core already owns, cosine computed in Go, no index and no second
// storage system. The linear scan is deliberate — D-012 makes performance an
// explicit non-goal at this scale, and the corpus size at which that stops
// being true is measured rather than assumed (FORGE-EXPERIMENTS Part C).
var _ ports.SimilarityIndex = (*Store)(nil)

// Put stores or replaces the vector for a library question.
func (s *Store) Put(ctx context.Context, questionID domain.LibraryQuestionID, model domain.ModelIdentity, v domain.Vector) error {
	if len(v) == 0 {
		return fmt.Errorf("forgestore: refusing to store an empty vector for question %d", questionID)
	}
	if err := s.guardDimension(ctx, model, len(v)); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO forge_embeddings (question_id, provider, model, dim, vector)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(question_id, provider, model)
		 DO UPDATE SET dim = excluded.dim, vector = excluded.vector`,
		int64(questionID), model.Provider, model.Model, len(v), domain.MarshalVector(v),
	); err != nil {
		return fmt.Errorf("forgestore: store vector for question %d: %w", questionID, err)
	}
	return nil
}

// guardDimension refuses a vector whose width disagrees with what this model
// has already produced.
//
// Same name, different width means the name stopped identifying the model —
// re-pulling an Ollama tag is the realistic way that happens. Letting the
// write through would leave a corpus that can only be read by skipping rows,
// and a skipped row is a duplicate that was never reported. Refusing here
// keeps that state unreachable through the port; the read side still guards
// it, because a database written by an older Forge is not this code's to
// promise.
func (s *Store) guardDimension(ctx context.Context, model domain.ModelIdentity, dim int) error {
	var stored int
	err := s.db.QueryRowContext(ctx,
		`SELECT dim FROM forge_embeddings WHERE provider = ? AND model = ? LIMIT 1`,
		model.Provider, model.Model,
	).Scan(&stored)
	switch {
	case err == sql.ErrNoRows:
		return nil
	case err != nil:
		return fmt.Errorf("forgestore: read stored dimension for %s: %w", model, err)
	case stored != dim:
		return fmt.Errorf(
			"forgestore: %s already holds %d-dimension vectors and this one is %d; "+
				"the model behind that name changed, so the stored vectors are no longer comparable — "+
				"clear the embeddings for this model before re-indexing",
			model, stored, dim)
	}
	return nil
}

// Missing returns the subset of ids with no vector for this model, in the
// order they were given.
func (s *Store) Missing(ctx context.Context, model domain.ModelIdentity, questionIDs []domain.LibraryQuestionID) ([]domain.LibraryQuestionID, error) {
	if len(questionIDs) == 0 {
		return nil, nil
	}

	present, err := s.storedIDs(ctx, model)
	if err != nil {
		return nil, err
	}

	missing := make([]domain.LibraryQuestionID, 0, len(questionIDs))
	for _, id := range questionIDs {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

// storedIDs reads the model's ids in one query rather than building an IN
// clause per call: SQLite caps host parameters, and a topic corpus large
// enough to matter would otherwise have to be chunked.
func (s *Store) storedIDs(ctx context.Context, model domain.ModelIdentity) (map[domain.LibraryQuestionID]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT question_id FROM forge_embeddings WHERE provider = ? AND model = ?`,
		model.Provider, model.Model,
	)
	if err != nil {
		return nil, fmt.Errorf("forgestore: list stored vectors for %s: %w", model, err)
	}
	defer func() { _ = rows.Close() }()

	present := map[domain.LibraryQuestionID]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("forgestore: scan stored question id: %w", err)
		}
		present[domain.LibraryQuestionID(id)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forgestore: read stored question ids: %w", err)
	}
	return present, nil
}

// Nearest scores every vector stored for the model and returns the best.
//
// A failure to compare any single stored vector fails the whole search. The
// tempting alternative — skip it and score the rest — turns a corrupted or
// mixed corpus into a shorter result list, and a duplicate missing from the
// list is indistinguishable from no duplicate at all. D-020 requires the
// refusal for exactly this reason.
func (s *Store) Nearest(ctx context.Context, model domain.ModelIdentity, probe domain.Vector, limit int) ([]ports.Neighbor, error) {
	if limit < 1 {
		return nil, fmt.Errorf("forgestore: a nearest-neighbor search needs a limit of at least 1, got %d", limit)
	}
	if len(probe) == 0 {
		return nil, fmt.Errorf("forgestore: refusing to search with an empty probe vector")
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT question_id, vector FROM forge_embeddings WHERE provider = ? AND model = ?`,
		model.Provider, model.Model,
	)
	if err != nil {
		return nil, fmt.Errorf("forgestore: search %s: %w", model, err)
	}
	defer func() { _ = rows.Close() }()

	var found []ports.Neighbor
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, fmt.Errorf("forgestore: scan stored vector: %w", err)
		}
		stored, err := domain.UnmarshalVector(blob)
		if err != nil {
			return nil, fmt.Errorf("forgestore: stored vector for question %d: %w", id, err)
		}
		if len(stored) != len(probe) {
			return nil, fmt.Errorf(
				"forgestore: question %d holds a %d-dimension vector under %s but the probe is %d-dimension; "+
					"vectors from different embedding models are not comparable, so this search is refused rather than scored",
				id, len(stored), model, len(probe))
		}
		score, err := domain.Cosine(probe, stored)
		if err != nil {
			return nil, fmt.Errorf("forgestore: score question %d: %w", id, err)
		}
		found = append(found, ports.Neighbor{QuestionID: domain.LibraryQuestionID(id), Score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("forgestore: read stored vectors: %w", err)
	}

	// Ties break on the question id so a repeated search returns a repeated
	// answer; two identical questions in the library score identically, and
	// row order out of SQLite is not a promise.
	sort.Slice(found, func(i, j int) bool {
		if found[i].Score != found[j].Score {
			return found[i].Score > found[j].Score
		}
		return found[i].QuestionID < found[j].QuestionID
	})
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

// Count reports how many vectors are stored for a model.
func (s *Store) Count(ctx context.Context, model domain.ModelIdentity) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM forge_embeddings WHERE provider = ? AND model = ?`,
		model.Provider, model.Model,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("forgestore: count vectors for %s: %w", model, err)
	}
	return n, nil
}
