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

package secrets_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/secrets"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

var _ ports.SecretResolver = (*secrets.Resolver)(nil)

const (
	envValue      = "env-CANARY-not-a-real-key"
	keychainValue = "keychain-CANARY-not-a-real-key"
)

type fakeKeychain struct {
	values map[domain.ProfileID]string
	err    error
	calls  int
}

func (f *fakeKeychain) Get(_ context.Context, profile domain.ProfileID) (value string, found bool, err error) {
	f.calls++
	if f.err != nil {
		return "", false, f.err
	}
	v, ok := f.values[profile]
	return v, ok, nil
}

func envWith(pairs map[string]string) (lookup func(string) (value string, ok bool)) {
	return func(key string) (string, bool) {
		v, ok := pairs[key]
		return v, ok
	}
}

// D-019's falsifier, and the reason the keychain seam ships without an
// implementation: proving "the environment wins" requires a keychain holding a
// DIFFERENT value to lose to. Without one this test would assert nothing —
// the environment would win by being the only source present, which is not the
// precedence rule, just an absence.
func TestEnvironmentWinsOverAPopulatedKeychain(t *testing.T) {
	keychain := &fakeKeychain{values: map[domain.ProfileID]string{
		domain.ProfileOpenAI: keychainValue,
	}}
	resolver := secrets.New(
		secrets.WithEnvLookup(envWith(map[string]string{"OPENAI_API_KEY": envValue})),
		secrets.WithKeychain(keychain),
	)

	got, err := resolver.Resolve(context.Background(), string(domain.ProfileOpenAI))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Reveal() != envValue {
		t.Errorf("resolved the wrong source: got the keychain value, want the environment's")
	}
	if got.Origin() != domain.OriginEnvironment {
		t.Errorf("origin = %q, want %q", got.Origin(), domain.OriginEnvironment)
	}

	origin, err := resolver.Describe(context.Background(), string(domain.ProfileOpenAI))
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if origin != domain.OriginEnvironment {
		t.Errorf("Describe disagrees with Resolve: %q vs %q", origin, domain.OriginEnvironment)
	}
}

// The other half of the precedence rule: with nothing in the environment, the
// keychain supplies the value and says so.
func TestKeychainSuppliesTheCredentialWhenTheEnvironmentIsEmpty(t *testing.T) {
	keychain := &fakeKeychain{values: map[domain.ProfileID]string{
		domain.ProfileAnthropic: keychainValue,
	}}
	resolver := secrets.New(
		secrets.WithEnvLookup(envWith(nil)),
		secrets.WithKeychain(keychain),
	)

	got, err := resolver.Resolve(context.Background(), string(domain.ProfileAnthropic))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Reveal() != keychainValue || got.Origin() != domain.OriginKeychain {
		t.Errorf("want the keychain value and origin, got origin %q", got.Origin())
	}
}

// An empty or whitespace-only variable is not a credential. Treating one as
// set sends an empty Authorization header and produces a 401 the user cannot
// explain, instead of the actionable "set X" they need.
func TestBlankEnvironmentValuesAreNotCredentials(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t\n"} {
		resolver := secrets.New(secrets.WithEnvLookup(envWith(map[string]string{
			"OPENAI_API_KEY": blank,
		})))
		_, err := resolver.Resolve(context.Background(), string(domain.ProfileOpenAI))
		if !errors.Is(err, domain.ErrCredentialMissing) {
			t.Errorf("blank value %q must not count as a credential, got %v", blank, err)
		}
	}
}

// Local Ollama needs no credential. Reporting that as a missing one would send
// a user hunting for a key that does not exist for this provider.
func TestOllamaResolvesToNoCredentialWithoutError(t *testing.T) {
	resolver := secrets.New(secrets.WithEnvLookup(envWith(nil)))

	got, err := resolver.Resolve(context.Background(), string(domain.ProfileOllama))
	if err != nil {
		t.Fatalf("a profile needing no credential must not error: %v", err)
	}
	if !got.IsZero() {
		t.Error("want an empty secret")
	}
	if got.Origin() != domain.OriginNone {
		t.Errorf("origin = %q, want %q", got.Origin(), domain.OriginNone)
	}
}

// The message has to name the variable that would fix it, and must never name,
// echo or partially reveal a value.
func TestMissingCredentialNamesTheVariableAndNoValue(t *testing.T) {
	resolver := secrets.New(secrets.WithEnvLookup(envWith(nil)))

	_, err := resolver.Resolve(context.Background(), string(domain.ProfileOpenRouter))
	if !errors.Is(err, domain.ErrCredentialMissing) {
		t.Fatalf("want ErrCredentialMissing, got %v", err)
	}
	if !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Errorf("the message must name the variable, got: %v", err)
	}
}

// A keychain that cannot be reached is a fault, not an absence. Reporting it
// as "no credential" tells a user to set a key they have already stored.
func TestAnUnreachableKeychainIsAFaultNotAMissingCredential(t *testing.T) {
	keychain := &fakeKeychain{err: errors.New("no Secret Service on this bus")}
	resolver := secrets.New(
		secrets.WithEnvLookup(envWith(nil)),
		secrets.WithKeychain(keychain),
	)

	_, err := resolver.Resolve(context.Background(), string(domain.ProfileOpenAI))
	if err == nil {
		t.Fatal("want the keychain fault reported")
	}
	if errors.Is(err, domain.ErrCredentialMissing) {
		t.Error("a broken keychain must not be reported as a missing credential")
	}
}

// The shipped configuration consults no keychain at all — #106 owns the
// library choice, and adopting one silently would be exactly the policy
// invention the epic forbids.
func TestTheShippedResolverConsultsNoKeychain(t *testing.T) {
	keychain := &fakeKeychain{values: map[domain.ProfileID]string{domain.ProfileOpenAI: keychainValue}}
	shipped := secrets.New(secrets.WithEnvLookup(envWith(nil)))

	_, err := shipped.Resolve(context.Background(), string(domain.ProfileOpenAI))
	if !errors.Is(err, domain.ErrCredentialMissing) {
		t.Errorf("without a keychain option, resolution must fall through to missing, got %v", err)
	}
	if keychain.calls != 0 {
		t.Errorf("the shipped resolver must not consult a keychain, %d calls", keychain.calls)
	}
}

// Every error and description from this package is user-facing. None may carry
// a credential.
func TestNoResolverOutputEverCarriesACredential(t *testing.T) {
	keychain := &fakeKeychain{values: map[domain.ProfileID]string{domain.ProfileOpenAI: keychainValue}}
	resolver := secrets.New(
		secrets.WithEnvLookup(envWith(map[string]string{"ANTHROPIC_API_KEY": envValue})),
		secrets.WithKeychain(keychain),
	)

	for _, profile := range []domain.ProfileID{
		domain.ProfileOpenAI, domain.ProfileAnthropic, domain.ProfileOpenRouter, domain.ProfileOllama, "bogus",
	} {
		secret, err := resolver.Resolve(context.Background(), string(profile))
		rendered := ""
		if err != nil {
			rendered = err.Error()
		}
		origin, describeErr := resolver.Describe(context.Background(), string(profile))
		if describeErr != nil {
			rendered += " " + describeErr.Error()
		}
		rendered += " " + string(origin) + " " + secret.String()

		for _, canary := range []string{envValue, keychainValue} {
			if strings.Contains(rendered, canary) {
				t.Errorf("profile %q leaked a credential in user-facing output: %s", profile, rendered)
			}
		}
	}
}

func TestUnknownProfileIsRejected(t *testing.T) {
	resolver := secrets.New(secrets.WithEnvLookup(envWith(nil)))
	if _, err := resolver.Resolve(context.Background(), "not-a-provider"); err == nil {
		t.Fatal("an unknown profile must be rejected")
	}
}
