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

// SourceCategory classifies a source for the source-authority policy.
//
// The vocabulary is deliberately empty apart from the zero value. FORGE.md 12
// assigns the categories, their ranking, exclusions and exceptions to the
// research-adapter spike (#120), and that spike has not reported. Enumerating
// plausible-sounding categories here would author the policy by fiat under the
// cover of a type definition, and every downstream test would then encode a
// guess as a specification.
type SourceCategory string

// SourceCategoryUnclassified is the state of every source until #120's
// source-authority policy lands and something can classify it.
const SourceCategoryUnclassified SourceCategory = ""

// SourceQuality carries the policy verdict on one source. It is a seam, not a
// policy: Forge owns the judgement, the adapter never makes it.
type SourceQuality struct {
	Category SourceCategory
	// Admissible records whether the source-authority policy accepted this
	// source as grounding material. With #120 unreported, nothing may set this
	// true on policy grounds; the field exists so the pipeline is written
	// against the decision rather than around it.
	Admissible bool
	// Reason states why in one human-readable clause, for the failure surface
	// FORGE.md 3.2 requires when a run cannot be grounded.
	Reason string
}

// Evidence is one standardized, attributable record of retrieved material.
//
// FORGE.md 5 fixes the shape: source identity, URL, title/publisher where
// available, retrieval time, relevant content, and quality/policy metadata.
// The generator, verifier and critic consume these records and nothing else —
// no stage browses independently, which is what keeps the trust boundary in
// one place.
type Evidence struct {
	// ID is the citation key and the fence id. It must be stable for a given
	// run so prompts are reproducible and a citation can be resolved back to
	// its record.
	ID string
	// URL is the canonical location the content came from.
	URL string
	// Title and Publisher are recorded where the source exposes them, and
	// left empty rather than guessed where it does not.
	Title     string
	Publisher string
	// RetrievedAt is injected, never read from the clock inside a constructor,
	// so that fixture-backed tests are deterministic.
	RetrievedAt time.Time
	// Content is the extracted, relevant portion — bounded by the pipeline's
	// budget, never a whole copied page, which FORGE.md 8 forbids persisting.
	Content UntrustedText
	// Quality carries the source-authority verdict.
	Quality SourceQuality
}

// Ref reduces an Evidence record to what a pack may carry.
//
// The reduction is the point: FORGE.md 8 keeps raw pages and full traces out
// of persistence, so a shipped pack carries the pointer and never the content.
// [SourceRef] is the core pack-format type, aliased in this package.
func (e Evidence) Ref() SourceRef {
	return SourceRef{ID: e.ID, URL: e.URL, Title: e.Title}
}
