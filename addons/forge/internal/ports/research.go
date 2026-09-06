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
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// Query is one bounded research request.
//
// Forge plans the queries; the adapter executes them. FORGE.md 5 draws that
// line deliberately: the adapter knows the provider's API, auth and wire
// format and nothing else, while query planning, source policy, evidence
// assembly, budgets and citations stay in the pipeline. An adapter that
// decided what to search for would make the source-authority policy
// unenforceable, because the policy would no longer see every candidate.
type Query struct {
	// Terms is the search expression, planned by the pipeline.
	Terms string
	// MaxResults bounds how many results to consider.
	MaxResults int
	// MaxBytesPerSource bounds extracted content per source, so one long page
	// cannot consume a whole run's evidence budget.
	MaxBytesPerSource int
	// Timeout bounds the whole call. Cancellation still travels through ctx;
	// this is the budget, not the mechanism.
	Timeout time.Duration
	// Language is the preferred content language, empty for no preference.
	Language string
}

// Research retrieves grounding material.
//
// The contract has three properties the pipeline depends on:
//
//   - Every returned record's content is [domain.UntrustedText]. The adapter
//     may not return a plain string, which is what stops retrieved text from
//     reaching a prompt by ordinary concatenation.
//   - The adapter classifies nothing. It leaves
//     domain.SourceQuality at its zero value; admissibility is the pipeline's
//     judgement under the source-authority policy, and #120 owns that policy.
//   - Returning zero records is a legitimate, non-error outcome. "Nothing
//     found" and "the search failed" are different facts, and only the
//     pipeline can decide whether the former is fatal for a given run —
//     FORGE.md 5 requires bounded retry before failing clear.
type Research interface {
	// Gather executes one query and returns the evidence it yielded.
	Gather(ctx context.Context, q Query) ([]domain.Evidence, error)
}
