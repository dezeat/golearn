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

import "fmt"

// ProfileID identifies one of the four V1 provider profiles (FORGE.md 6.1).
type ProfileID string

// The V1 profiles. The user flow is Provider -> Model; no vendor or model is
// ever globally forced.
const (
	ProfileOpenAI     ProfileID = "openai"
	ProfileAnthropic  ProfileID = "anthropic"
	ProfileOpenRouter ProfileID = "openrouter"
	ProfileOllama     ProfileID = "ollama"
)

// Profile describes a provider's auth and endpoint conventions.
//
// It is data, not behavior, so the secret resolver and the provider adapters
// agree on the same facts without depending on each other. A profile that
// disagreed with its adapter about which environment variable carries the key
// would produce the worst error message in the product: "no credential" for a
// key the user had already set.
type Profile struct {
	ID          ProfileID
	DisplayName string

	// CredentialEnvVar is the environment variable carrying the API key, or
	// empty when the profile needs no credential at all.
	CredentialEnvVar string

	// EndpointEnvVar overrides the endpoint, where the provider has an
	// established convention for it.
	EndpointEnvVar string

	// DefaultEndpoint is the base URL used when nothing overrides it.
	DefaultEndpoint string

	// Embeds reports whether the provider exposes an embeddings API at all.
	//
	// This is a fact about the vendor, not about configuration: Anthropic
	// ships no embeddings endpoint, so no key and no model makes one appear
	// (D-018). It exists so the surface that lists profiles can say so before
	// a user picks one and discovers it mid-run. The load-bearing form of the
	// same fact is structural — an adapter without embeddings does not
	// implement ports.Embedder — and a test holds the two in agreement.
	Embeds bool
}

// NeedsCredential reports whether the profile requires an API key.
func (p Profile) NeedsCredential() bool { return p.CredentialEnvVar != "" }

// profiles is the V1 registry, in the order FORGE.md 6.1 lists them.
var profiles = []Profile{
	{
		ID: ProfileOpenAI, DisplayName: "OpenAI",
		CredentialEnvVar: "OPENAI_API_KEY",
		DefaultEndpoint:  "https://api.openai.com/v1",
		Embeds:           true,
	},
	{
		ID: ProfileAnthropic, DisplayName: "Anthropic",
		CredentialEnvVar: "ANTHROPIC_API_KEY",
		DefaultEndpoint:  "https://api.anthropic.com/v1",
		// Anthropic ships no embeddings API. See D-018.
		Embeds: false,
	},
	{
		ID: ProfileOpenRouter, DisplayName: "OpenRouter",
		CredentialEnvVar: "OPENROUTER_API_KEY",
		DefaultEndpoint:  "https://openrouter.ai/api/v1",
		Embeds:           true,
	},
	{
		ID: ProfileOllama, DisplayName: "Ollama (local)",
		// No credential: Ollama's "login" is a reachability check, not key
		// validation (FORGE.md 6.1). Conflating the two is what produces an
		// "invalid API key" message for a service that is simply not running.
		CredentialEnvVar: "",
		EndpointEnvVar:   "OLLAMA_HOST",
		DefaultEndpoint:  "http://localhost:11434",
		Embeds:           true,
	},
}

// Profiles returns the V1 provider profiles in presentation order.
func Profiles() []Profile {
	out := make([]Profile, len(profiles))
	copy(out, profiles)
	return out
}

// ProfileByID returns the named profile.
func ProfileByID(id ProfileID) (Profile, error) {
	for _, p := range profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("unknown provider profile %q", id)
}
