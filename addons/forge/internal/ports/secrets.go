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

// SecretResolver supplies a provider credential at runtime.
//
// Resolution order is environment first, then any desktop keychain (D-019).
// That inverts the emphasis of FORGE.md 6.2, which leads with the keychain and
// treats the environment as an automation override; the inversion is a
// deliberate, recorded deviation rather than drift, and D-019 carries the
// reasoning. The interface itself is order-agnostic — precedence belongs to
// the resolver implementation, which is where it can be tested.
//
// Nothing here persists anything. A credential is read at the moment it is
// needed and never written to SQLite, packs, drafts, logs or diagnostics.
type SecretResolver interface {
	// Resolve returns the credential for a profile.
	//
	// A profile that needs no credential — local Ollama — resolves to a zero
	// Secret with domain.OriginNone and no error, which is why the origin is
	// carried on the value rather than returned separately: "resolved
	// nothing, legitimately" and "failed to resolve" must not look alike.
	//
	// A profile that requires one and has none returns
	// domain.ErrCredentialMissing. The error names the profile and the
	// environment variable that would supply it; it never names, echoes or
	// partially reveals a value.
	Resolve(ctx context.Context, profile string) (domain.Secret, error)

	// Describe reports where a credential would come from, without resolving
	// or revealing it. This is what lets a user answer "which source is
	// supplying my key?" from the config surface — the question FORGE.md 6.2
	// requires an answer to, and the one most likely to grow a leak.
	Describe(ctx context.Context, profile string) (domain.SecretOrigin, error)
}
