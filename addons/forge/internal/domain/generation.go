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

package domain

import (
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// The pack-format types are defined once, in the core domain, and aliased here.
//
// They describe a *file format*, and the offline binary must parse a pack Forge
// produced (D-017), so the core owns the definitions. Redeclaring them here
// would create two structs for one wire format, and the first field added to
// only one of them would produce a pack that round-trips differently depending
// on which binary read it.
//
// These are true aliases, not conversions: Forge code reads naturally as
// domain.GenerationSpec while remaining the same type the core marshals.
type (
	// Style is the pack-level intent selector; see [coredomain.Style].
	Style = coredomain.Style

	// GenerationSpec records the content-shaping inputs to a run.
	GenerationSpec = coredomain.GenerationSpec

	// ModelIdentity names the provider and model that served a request.
	ModelIdentity = coredomain.ModelIdentity

	// SourceRef is the compact pointer to grounding evidence that a pack carries.
	SourceRef = coredomain.SourceRef

	// Provenance records how a pack came to exist.
	Provenance = coredomain.Provenance
)

const (
	// PackVersion is the pack schema version Forge emits (D-017).
	PackVersion = coredomain.PackVersionGenerated

	// StyleUnset is the absence of a style selection, and is always valid.
	StyleUnset = Style("")

	// GeneratedConfidence marks a question as machine-produced, strictly below
	// the hand-authored default.
	GeneratedConfidence = coredomain.GeneratedConfidence
)

// Candidate is one generated question before it has been accepted into a pack.
// It is the unit the validation, verification, critique and similarity stages
// operate on, and it exists only inside a run — nothing here reaches a pack.
type Candidate struct {
	Question coredomain.PackQuestion

	// Citations are the evidence ids the generator claimed support for this
	// question. Grounding fidelity is checked against these.
	Citations []string

	// Vector is the embedding of the candidate's canonical representation,
	// populated only when the bound profile has an embedding capability. A nil
	// vector is the normal state under a provider that exposes none — Anthropic
	// ships no embeddings API (D-018) — and the similarity gate's fallback
	// policy, not a panic, decides what happens next.
	Vector Vector
}
