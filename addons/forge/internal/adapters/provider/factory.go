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

package provider

import (
	"context"
	"fmt"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// Open builds the provider client for a profile, resolving its credential
// through the supplied resolver.
//
// This is the composition point where "which provider" stops being a string
// and becomes a typed capability. A caller that needs embeddings asks the
// returned value for them with [EmbedderFor] and gets a definite yes or no —
// it never discovers mid-run that the provider it was handed cannot embed.
func Open(ctx context.Context, resolver ports.SecretResolver, id domain.ProfileID, model string, opts ...ClientOption) (ports.Provider, error) {
	profile, err := domain.ProfileByID(id)
	if err != nil {
		return nil, err
	}

	// Ollama needs no credential, so it is built before any resolution is
	// attempted rather than resolving to an empty secret and hoping nothing
	// downstream treats that as a failure.
	if !profile.NeedsCredential() {
		return NewOllama(profile, model, opts...), nil
	}

	secret, err := resolver.Resolve(ctx, string(id))
	if err != nil {
		return nil, err
	}

	switch id {
	case domain.ProfileAnthropic:
		return NewAnthropic(profile, model, secret, opts...), nil
	case domain.ProfileOpenAI, domain.ProfileOpenRouter:
		return NewOpenAICompatible(profile, model, secret, opts...), nil
	default:
		return nil, fmt.Errorf("no adapter for provider profile %q", id)
	}
}

// EmbedderFor reports whether a provider can produce embeddings, and returns
// the capability when it can.
//
// The type assertion is the whole mechanism, and that is the point of D-018: a
// profile without an embeddings API does not implement the interface, so the
// answer is structural rather than a flag that could disagree with reality.
// Callers that need vectors check this BEFORE choosing a strategy, which is
// what turns "Anthropic has no embeddings" from a mid-run surprise into a
// decision made up front.
func EmbedderFor(p ports.Provider) (ports.Embedder, bool) {
	e, ok := p.(ports.Embedder)
	return e, ok
}

// RequireEmbedder returns the embedding capability or the typed error naming
// its absence, for callers that cannot proceed without one.
func RequireEmbedder(p ports.Provider) (ports.Embedder, error) {
	if e, ok := EmbedderFor(p); ok {
		return e, nil
	}
	identity := p.Identity()
	profile, err := domain.ProfileByID(domain.ProfileID(identity.Provider))
	if err != nil {
		return nil, fmt.Errorf("%w: provider %q", domain.ErrNoEmbeddingCapability, identity.Provider)
	}
	return nil, embeddingUnsupported(profile)
}
