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

import "time"

// Pack schema 0.2.0 (D-017) adds pack-level metadata describing how a pack was
// produced. The fields are optional additions: a 0.1.x pack has none, a
// hand-authored 0.2.0 pack may have none, and the practice engine reads none of
// them. They exist so a generated pack can say what was asked for and where the
// content came from, and so that a shared pack carries that with it.
//
// These types live in the core domain rather than in Forge because they are the
// *pack format*, and the offline binary must be able to parse a pack Forge
// produced. That is not the core knowing about Forge — the core knows about a
// file format, and D-015's one-way rule is untouched.

// Style is the pack-level intent selector.
//
// It is deliberately an open string with no validated vocabulary. FORGE.md 12
// sequences the intent enum behind spike #105, which is itself sequenced behind
// a working pipeline; fixing values here would pre-empt the spike and create
// the circular dependency #121 forbids. An unknown or missing style is
// therefore valid and content-neutral, which is the backwards-compatible
// behavior #121 requires.
type Style string

// GenerationSpec records every user-visible, content-shaping input to a
// generation run (D-017). It answers "what was requested?" and is what makes a
// generated pack inspectable, filterable and partially reproducible.
//
// Inputs only. Effort presets, retry counters, budgets and request mechanics
// are absent by contract, not by omission: they do not shape content, and
// FORGE.md 8 forbids persisting them.
type GenerationSpec struct {
	Topic       string     `json:"topic"                 yaml:"topic"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Count       int        `json:"count"                 yaml:"count"`
	Difficulty  Difficulty `json:"difficulty,omitempty"  yaml:"difficulty,omitempty"`
	Style       Style      `json:"style,omitempty"       yaml:"style,omitempty"`
	Language    string     `json:"language,omitempty"    yaml:"language,omitempty"`
}

// ModelIdentity names the provider and model that served a request.
//
// It carries no endpoint. A model identifier is safe to disclose; the
// deployment that served it is operator information, and a pack is a file
// people share.
type ModelIdentity struct {
	Provider string `json:"provider" yaml:"provider"`
	Model    string `json:"model"    yaml:"model"`
}

// String renders "provider/model", the form used in provenance and diagnostics.
func (m ModelIdentity) String() string {
	if m.Model == "" {
		return m.Provider
	}
	return m.Provider + "/" + m.Model
}

// SourceRef is the compact, durable pointer to one piece of grounding
// evidence. Full evidence records and copied pages stay local (FORGE.md 8); a
// shipped pack carries only this.
type SourceRef struct {
	ID    string `json:"id"              yaml:"id"`
	URL   string `json:"url"             yaml:"url"`
	Title string `json:"title,omitempty" yaml:"title,omitempty"`
}

// Provenance records how a pack came to exist (D-017): generation time,
// provider/model identity, and source references.
//
// Categorically excluded: secrets, raw prompts, raw model or tool output,
// retry and repair counters, and provider request mechanics.
type Provenance struct {
	GeneratedAt  time.Time     `json:"generated_at"            yaml:"generated_at"`
	Model        ModelIdentity `json:"model"                   yaml:"model"`
	Verifier     ModelIdentity `json:"verifier,omitempty"      yaml:"verifier,omitempty"`
	Sources      []SourceRef   `json:"sources,omitempty"       yaml:"sources,omitempty"`
	ForgeVersion string        `json:"forge_version,omitempty" yaml:"forge_version,omitempty"`
}

// Confidence defaults. FORGE.md 9 keeps hand-authored content at 1.0 and
// requires generated content to sit strictly below it, so provenance class is
// visible in the data and not only in a pack header.
const (
	// ManualConfidence is the value a pack question carries when its author
	// stated none.
	ManualConfidence = 1.0

	// GeneratedConfidence marks a question as machine-produced. It is a
	// provenance marker, not a calibrated probability — the finer assurance
	// taxonomy (Ideas #99) is deferred.
	GeneratedConfidence = 0.9
)
