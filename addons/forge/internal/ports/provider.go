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

// Request is one bounded call to a provider.
//
// It carries no retry policy, no budget and no fallback model. Forge owns stop
// conditions and budgets (FORGE.md 4); an adapter that retried on its own
// would take a decision the pipeline is accountable for and make the run's
// cost summary a lie.
type Request struct {
	// System is operator-authored instruction. Retrieved material never
	// belongs here — it goes in Evidence, fenced.
	System string
	// User is the operator-authored task for this call.
	User string
	// Evidence is the untrusted grounding material. The adapter renders it as
	// quoted data; it is never concatenated into System.
	Evidence []domain.Evidence
	// Schema is the JSON Schema the reply must satisfy. Providers that support
	// native structured output use it directly; the rest receive it as an
	// instruction and the reply is validated on arrival either way.
	Schema []byte
	// MaxOutputTokens bounds the reply. Zero means the adapter's documented
	// default for the model.
	MaxOutputTokens int
	// Temperature is a pointer so that "unset" is distinguishable from an
	// explicit zero, which is a meaningful and commonly requested value.
	Temperature *float64
}

// Provider is the chat-capable half of a provider profile: the contract all
// four V1 profiles satisfy.
//
// Every call is independent. There is no session and no conversation handle,
// which is what makes FORGE.md 4's "verify in a fresh context on the same
// provider/model" a structural property of the port rather than a discipline
// the pipeline has to remember — a verifier call literally cannot see the
// generator's context.
//
// Cancellation is through ctx and must release the underlying request rather
// than abandon it, because a hung cancel is a resource defect the live lane
// measures directly.
type Provider interface {
	// Identity returns the provider and model this instance is bound to, for
	// provenance and diagnostics. It never includes the endpoint.
	Identity() domain.ModelIdentity

	// Generate performs one request and decodes the structured reply into out,
	// which must be a non-nil pointer.
	//
	// A reply that does not parse against the requested schema is reported as
	// domain.ErrStructuredOutput, wrapped with provider context. The adapter
	// does not repair it: the single permitted repair is the pipeline's to
	// spend, and an adapter that quietly retried would consume that budget
	// invisibly.
	Generate(ctx context.Context, req Request, out any) error
}

// Embedder is implemented only by provider profiles that actually expose an
// embeddings endpoint.
//
// It is deliberately not part of [Provider]. Anthropic ships no embeddings
// API, so its adapter does not implement this interface at all — the absence
// is a compile-time fact rather than a runtime error, and a test asserting
// that the Anthropic adapter does not satisfy Embedder fails the moment
// someone bolts on a stub that pretends otherwise.
//
// A provider that does implement it may still fail per-model: an Ollama
// endpoint answers for an embedding model and refuses for a chat-only one, and
// that is domain.ErrModelNotAvailable, not a missing capability.
type Embedder interface {
	// EmbeddingIdentity names the embedding model, which is generally not the
	// chat model. Vectors from different models are not comparable, so this
	// identity is stored alongside every vector.
	EmbeddingIdentity() domain.ModelIdentity

	// Embed returns one vector per input text, in the same order.
	Embed(ctx context.Context, texts []string) ([]domain.Vector, error)
}

// ProbeResult reports whether a profile is usable, without spending a
// generation.
type ProbeResult struct {
	// Reachable is whether the endpoint answered at all.
	Reachable bool
	// Authenticated is whether the credential was accepted. For Ollama there
	// is no credential to accept, so a reachable endpoint reports true here —
	// FORGE.md 6.1 requires reachability and key validation to stay
	// distinguishable, and conflating them is what produces the "invalid API
	// key" message for a service that is simply not running.
	Authenticated bool
	// Models is the installed or available model list where the provider
	// exposes one. Ollama answers this and reachability in a single call;
	// key-based providers may leave it empty.
	Models []string
	// Detail is a short, disclosure-safe explanation for a user. It never
	// contains a credential, and never the endpoint for a non-local provider.
	Detail string
}

// ProviderProbe answers "can I use this profile?" before a run starts.
//
// Kept separate from [Provider] because the answer is a property of the
// configured profile rather than of a generation, and because the Ollama
// profile satisfies it through a plain reachability call that has no
// equivalent on the key-based providers.
type ProviderProbe interface {
	Probe(ctx context.Context) (ProbeResult, error)
}
