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

// Package secrets resolves provider credentials at runtime.
//
// Resolution order is **environment first, then any OS keychain** (D-019).
// That inverts the emphasis of FORGE.md 6.2, which leads with the keychain and
// treats the environment as an automation override. The inversion is a
// recorded deviation, not drift: golearn is developed, tested and run largely
// in containers, headless Linux, CI and SSH sessions, none of which have a
// Secret Service — a keychain-first policy would make the fallback the common
// path and the primary path the exception.
//
// **No keychain implementation ships.** Every candidate library is a new
// dependency, and the library choice is spike #106's open deliverable. What
// ships is the seam: a Keychain interface, the precedence rule written against
// the full order, and tests that exercise it with a fake. When #106 reports,
// an implementation drops in without the precedence rule being retrofitted —
// and the rule is already proven, because the falsifier for "environment
// wins" needs a keychain that holds a *different* value to lose to.
//
// Nothing here persists anything. A credential is read when it is needed and
// never written to SQLite, packs, drafts, logs or diagnostics.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// Keychain is the seam for an OS credential store.
//
// Found reports whether an entry existed, separately from err: "the keychain
// works and holds nothing for this profile" is an ordinary outcome, while "the
// keychain could not be reached" is a fault. Collapsing them into a nil/err
// pair would make an unavailable keychain indistinguishable from an empty one,
// and on a headless box that is the difference between a clear message and a
// mystery.
type Keychain interface {
	Get(ctx context.Context, profile domain.ProfileID) (value string, found bool, err error)
}

// Resolver implements ports.SecretResolver.
type Resolver struct {
	lookupEnv func(string) (string, bool)
	keychain  Keychain
}

// Option configures a Resolver.
type Option func(*Resolver)

// WithKeychain supplies an OS keychain to consult after the environment.
// No implementation ships today; this is how #106's will arrive, and how the
// precedence rule is tested with a fake in the meantime.
func WithKeychain(k Keychain) Option {
	return func(r *Resolver) { r.keychain = k }
}

// WithEnvLookup replaces the environment lookup, so precedence can be tested
// without mutating the process environment of a parallel test run.
func WithEnvLookup(lookup func(string) (string, bool)) Option {
	return func(r *Resolver) { r.lookupEnv = lookup }
}

// New builds a resolver. With no options it reads the process environment and
// consults no keychain, which is the shipped configuration.
func New(opts ...Option) *Resolver {
	r := &Resolver{lookupEnv: os.LookupEnv}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve returns the credential for a profile.
//
// A profile that needs no credential — local Ollama — resolves to a zero
// Secret with OriginNone and no error. That is why the origin travels on the
// value rather than as a separate return: "resolved nothing, legitimately" and
// "failed to resolve" must not look alike to a caller, and Ollama hits the
// first case on every run.
func (r *Resolver) Resolve(ctx context.Context, profile string) (domain.Secret, error) {
	p, err := domain.ProfileByID(domain.ProfileID(profile))
	if err != nil {
		return domain.Secret{}, err
	}
	if !p.NeedsCredential() {
		return domain.NewSecret("", domain.OriginNone), nil
	}

	if value, ok := r.lookupEnv(p.CredentialEnvVar); ok && strings.TrimSpace(value) != "" {
		return domain.NewSecret(value, domain.OriginEnvironment), nil
	}

	if r.keychain != nil {
		value, found, err := r.keychain.Get(ctx, p.ID)
		if err != nil {
			// Report the fault rather than reporting a missing credential:
			// telling a user to set a key they already stored is the worst
			// available message.
			return domain.Secret{}, fmt.Errorf("read keychain for %s: %w", p.DisplayName, err)
		}
		if found && strings.TrimSpace(value) != "" {
			return domain.NewSecret(value, domain.OriginKeychain), nil
		}
	}

	// Names the variable that would supply it, never a value and never a
	// fragment of one.
	return domain.Secret{}, fmt.Errorf(
		"%w: set %s to use %s", domain.ErrCredentialMissing, p.CredentialEnvVar, p.DisplayName)
}

// Describe reports where a credential would come from without resolving or
// revealing it — the answer to "which source is supplying my key?" that the
// config surface needs and that is the most likely place for a leak to appear.
func (r *Resolver) Describe(ctx context.Context, profile string) (domain.SecretOrigin, error) {
	p, err := domain.ProfileByID(domain.ProfileID(profile))
	if err != nil {
		return domain.OriginNone, err
	}
	if !p.NeedsCredential() {
		return domain.OriginNone, nil
	}
	if value, ok := r.lookupEnv(p.CredentialEnvVar); ok && strings.TrimSpace(value) != "" {
		return domain.OriginEnvironment, nil
	}
	if r.keychain != nil {
		_, found, err := r.keychain.Get(ctx, p.ID)
		if err != nil {
			return domain.OriginNone, fmt.Errorf("read keychain for %s: %w", p.DisplayName, err)
		}
		if found {
			return domain.OriginKeychain, nil
		}
	}
	return domain.OriginNone, nil
}
