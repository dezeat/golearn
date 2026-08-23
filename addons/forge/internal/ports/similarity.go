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
)

// Neighbor is one stored question found near a probe vector.
type Neighbor struct {
	// QuestionID identifies the stored question. It is a core question id when
	// the neighbor is library content, or a draft-local index when it is
	// another candidate from the same pack — the two corpora are searched
	// separately because the gate's remedies differ. FORGE.md 7 permits
	// repairing or replacing a candidate and forbids touching library content.
	QuestionID int64
	// Score is cosine similarity in [-1, 1]. Higher is more similar.
	Score float64
}

// SimilarityIndex stores and searches embedding vectors.
//
// It exchanges vectors and never providers. The index must not know how a
// vector was produced and the embedding source must not know what it will be
// compared against; the pipeline holds both. Handing this interface a
// [Embedder] would fuse the two seams and make either one unswappable — and
// the whole reason the backend sits behind a port is that D-012 makes today's
// answer (BLOB vectors, cosine in Go) a measured choice rather than a
// permanent one.
//
// Vectors from different embedding models are not comparable. Implementations
// must record the model identity alongside each vector and must refuse a
// search that mixes them, rather than returning a plausible-looking score.
type SimilarityIndex interface {
	// Put stores or replaces the vector for a question.
	Put(ctx context.Context, questionID int64, model domain.ModelIdentity, v domain.Vector) error

	// Nearest returns up to limit stored neighbors of probe, ordered by
	// descending score. The model identity scopes the search: only vectors
	// from the same embedding model are candidates.
	Nearest(ctx context.Context, model domain.ModelIdentity, probe domain.Vector, limit int) ([]Neighbor, error)

	// Count reports how many vectors are stored for a model, so the pipeline
	// can report an empty corpus as an empty corpus rather than as "nothing
	// was too similar".
	Count(ctx context.Context, model domain.ModelIdentity) (int, error)
}
