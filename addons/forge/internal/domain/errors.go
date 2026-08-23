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

import "errors"

// The Forge error taxonomy. Each sentinel names a condition the pipeline must
// be able to distinguish and report, rather than a message it happens to
// print. Adapters wrap these with provider-specific context using %w.
var (
	// ErrNoEmbeddingCapability reports that the bound provider profile does
	// not expose an embeddings API at all.
	//
	// This is the typed form of a product fact, not a transient failure:
	// Anthropic ships no embeddings endpoint, so an Anthropic-backed run
	// cannot produce vectors no matter how it is configured. Modeling it as
	// an error value rather than a runtime surprise is what lets the
	// similarity gate fail clear and say which capability is missing and why.
	// The complementary compile-time fact is that such an adapter does not
	// implement ports.Embedder at all.
	ErrNoEmbeddingCapability = errors.New("provider profile exposes no embedding capability")

	// ErrProviderUnreachable reports that the endpoint did not answer. For
	// local Ollama this is the common case — the service is simply not
	// running — and it must never be reported as an authentication failure.
	ErrProviderUnreachable = errors.New("provider endpoint unreachable")

	// ErrCredentialMissing reports that no credential was resolved for a
	// profile that requires one. It never carries the value it looked for.
	ErrCredentialMissing = errors.New("no credential resolved for provider profile")

	// ErrCredentialRejected reports that a credential was supplied and the
	// provider refused it. Distinct from ErrCredentialMissing: one means "you
	// have not configured a key", the other "the key you configured is not
	// accepted", and sending a user to look for a missing key they already set
	// is the most frustrating error this surface can produce.
	ErrCredentialRejected = errors.New("provider rejected the credential")

	// ErrRateLimited reports a provider-side rate limit. It is separated from
	// other failures because it is the one worth waiting on rather than
	// reconfiguring.
	ErrRateLimited = errors.New("provider rate limit reached")

	// ErrModelNotAvailable reports that the requested model is not present at
	// the resolved endpoint. Distinct from ErrProviderUnreachable: the service
	// answered, and the answer was "not that model".
	ErrModelNotAvailable = errors.New("model not available at provider")

	// ErrStructuredOutput reports a reply that did not parse against the
	// requested schema. It is the failure D-016's pipeline is most sensitive
	// to, because pack-level acceptance replaced per-question human review.
	ErrStructuredOutput = errors.New("provider reply did not parse as the requested structure")

	// ErrResearchResponse reports that the research endpoint answered, but the
	// answer could not be used: a refusing or failing status code, a body that
	// did not parse, or a body past the configured size bound.
	//
	// It is deliberately distinct from ErrProviderUnreachable, which means the
	// endpoint never answered at all. The pipeline acts on the difference: an
	// unreachable endpoint may be worth retrying later, while an unusable
	// answer is nearly always a configuration fault the operator must fix.
	// ErrStructuredOutput is the wrong sentinel to reuse here — it names a
	// model reply that failed its schema, and the pack-acceptance reasoning in
	// its comment does not apply to a search endpoint.
	ErrResearchResponse = errors.New("research endpoint returned an unusable response")

	// ErrInsufficientEvidence reports that research completed without enough
	// admissible evidence to ground generation. FORGE.md 5 forbids lowering
	// the quality bar to reach the requested count, so this fails the run
	// rather than degrading it.
	ErrInsufficientEvidence = errors.New("insufficient evidence to ground generation")

	// ErrBudgetExhausted reports that a bounded budget — attempts, time, or
	// tokens — ran out before the run could complete.
	ErrBudgetExhausted = errors.New("run budget exhausted")

	// ErrDraftNotFound reports a draft that no longer exists, typically
	// because it was resolved by another window or process.
	ErrDraftNotFound = errors.New("draft not found")
)
