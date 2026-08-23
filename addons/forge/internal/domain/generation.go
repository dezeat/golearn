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
	"time"

	coredomain "github.com/dezeat/golearn/internal/domain"
)

// PackVersion is the schema version Forge emits (D-017). 0.1.x stays
// importable; the importer carries the compatibility policy.
const PackVersion = "0.2.0"

// Style is the pack-level intent selector.
//
// The vocabulary is intentionally not enumerated. FORGE.md 12 sequences the
// intent enum behind spike #105, which is itself sequenced behind a working
// pipeline — so fixing values here would both pre-empt the spike and create
// the circular dependency #121 explicitly forbids. Forge therefore treats an
// unknown or missing style as valid and content-neutral, which is the
// backwards-compatible behavior #121 requires.
type Style string

// StyleUnset is the absence of a style selection, and is always valid.
const StyleUnset Style = ""

// GenerationSpec is every user-visible, content-shaping input to a run
// (D-017). It answers "what was requested?" and is what makes a run
// inspectable, filterable, and partially reproducible.
//
// It carries inputs only. Effort presets, retry counters, budgets and request
// mechanics are deliberately absent: FORGE.md 8 forbids persisting them, and
// they do not shape content in a way a reader of the pack needs to know.
type GenerationSpec struct {
	Topic       string
	Description string
	Count       int
	Difficulty  coredomain.Difficulty
	Style       Style
	Language    string
}

// ModelIdentity names the provider and model that served a request. It is
// safe to disclose and is recorded in provenance; the endpoint that served it
// is not part of it, because a deployment address is operator information.
type ModelIdentity struct {
	Provider string
	Model    string
}

// String renders "provider/model", the form used in provenance and diagnostics.
func (m ModelIdentity) String() string {
	if m.Model == "" {
		return m.Provider
	}
	return m.Provider + "/" + m.Model
}

// Provenance is the durable record of how a pack came to exist (D-017):
// generation time, provider/model identity, and source references.
//
// Categorically excluded, per D-017 and FORGE.md 8: secrets, raw prompts, raw
// model or tool output, retry and repair counters, and provider request
// mechanics.
type Provenance struct {
	GeneratedAt time.Time
	Model       ModelIdentity
	Verifier    ModelIdentity
	Sources     []SourceRef
	// ForgeVersion identifies the binary that produced the pack, so a defect
	// traced to a generation vintage can be scoped.
	ForgeVersion string
}

// GeneratedConfidence is the per-question confidence Forge assigns.
//
// FORGE.md 9 keeps hand-authored content at the manual default of 1.0 and
// requires generated questions to sit strictly below it. The value is a
// marker of provenance class, not a calibrated probability — the finer
// assurance taxonomy (Ideas #99) is deferred.
const GeneratedConfidence = 0.9

// Candidate is one generated question before it has been accepted into a
// pack. It is the unit the validation, verification, critique and similarity
// stages operate on.
type Candidate struct {
	Question coredomain.PackQuestion
	// Citations are the evidence ids the generator claimed support for this
	// question. Grounding fidelity is checked against these.
	Citations []string
	// Vector is the embedding of the candidate's canonical representation,
	// populated only when the bound profile has an embedding capability. A nil
	// vector is the normal state under a provider that exposes none, and the
	// similarity gate's fallback policy — not a panic — decides what happens.
	Vector Vector
}
