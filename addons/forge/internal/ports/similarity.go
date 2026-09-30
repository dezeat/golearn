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

// Neighbor is one library question found near a probe vector.
//
// It addresses exactly one corpus. An earlier form of this type documented
// QuestionID as "a core question id, or a draft-local index, depending on
// which method you called" — two id spaces in one int64, distinguished only by
// a doc comment. That is resolved here rather than described: the index holds
// library vectors only, so the id is always a [domain.LibraryQuestionID], and
// intra-pack collisions never travel through this type at all.
//
// The reason intra-pack comparison stays out of the index is not tidiness. A
// candidate may be repaired, replaced or rejected and never reach the library,
// so writing its vector here to compare it against its siblings would persist
// embeddings for questions that do not exist — and every later search would
// score against content no user can ever practice. Candidates are already in
// memory during a run, and comparing them to each other is arithmetic over a
// slice, so the app layer does it there and keeps its own index-free type for
// the result.
type Neighbor struct {
	// QuestionID identifies the stored library question.
	QuestionID domain.LibraryQuestionID
	// Score is cosine similarity in [-1, 1]. Higher is more similar.
	Score float64
}

// SimilarityIndex stores and searches embedding vectors for library questions.
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
// "Refuse" means the whole search fails: returning the neighbors that did
// compare and dropping the ones that did not is the dangerous form, because a
// duplicate that silently fell out of the result set reads exactly like no
// duplicate at all.
type SimilarityIndex interface {
	// Put stores or replaces the vector for a library question.
	Put(ctx context.Context, questionID domain.LibraryQuestionID, model domain.ModelIdentity, v domain.Vector) error

	// Missing returns the subset of ids that have no vector for this model,
	// preserving the input order.
	//
	// It exists so the gate embeds only what it must. Without it every run
	// re-embeds the whole topic corpus, and since embeddings come from the
	// configured provider (D-018), that is a per-run network and token cost
	// proportional to the library rather than to the pack being generated.
	Missing(ctx context.Context, model domain.ModelIdentity, questionIDs []domain.LibraryQuestionID) ([]domain.LibraryQuestionID, error)

	// Nearest returns up to limit stored neighbors of probe, ordered by
	// descending score. The model identity scopes the search: only vectors
	// from the same embedding model are candidates.
	Nearest(ctx context.Context, model domain.ModelIdentity, probe domain.Vector, limit int) ([]Neighbor, error)

	// Count reports how many vectors are stored for a model, so the pipeline
	// can report an empty corpus as an empty corpus rather than as "nothing
	// was too similar".
	Count(ctx context.Context, model domain.ModelIdentity) (int, error)
}
