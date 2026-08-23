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

package domain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// A credential that can reach a log line, an error or a bug report through
// ordinary formatting is a leak waiting for the first person who prints a
// config struct. Cover every verb, including the two that bypass Stringer.
func TestSecretNeverRendersItsValueUnderAnyVerb(t *testing.T) {
	const canary = "sk-test-CANARY-not-a-real-key"
	s := domain.NewSecret(canary, domain.OriginEnvironment)

	for _, verb := range []string{"%v", "%s", "%q", "%+v", "%#v", "%d", "%x"} {
		if rendered := fmt.Sprintf(verb, s); strings.Contains(rendered, canary) {
			t.Errorf("verb %s leaked the credential: %s", verb, rendered)
		}
	}
	// Wrapping is the realistic path: a struct printed whole, an error built
	// with %v. Both must stay clean.
	wrapper := struct {
		Profile string
		Key     domain.Secret
	}{Profile: "openai", Key: s}
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		if rendered := fmt.Sprintf(verb, wrapper); strings.Contains(rendered, canary) {
			t.Errorf("verb %s leaked through an enclosing struct: %s", verb, rendered)
		}
	}
	if got := s.Reveal(); got != canary {
		t.Errorf("Reveal must return the credential verbatim, got %q", got)
	}
}

// "Resolved nothing, legitimately" and "failed to resolve" must not look
// alike: local Ollama needs no credential, and reporting that as a missing one
// would send a user hunting for a key that does not exist.
func TestSecretDistinguishesAbsenceFromOrigin(t *testing.T) {
	none := domain.NewSecret("", domain.OriginNone)
	if !none.IsZero() {
		t.Error("a profile that needs no credential must report an empty secret")
	}
	if none.Origin() != domain.OriginNone {
		t.Errorf("origin = %q, want %q", none.Origin(), domain.OriginNone)
	}

	fromEnv := domain.NewSecret("value", domain.OriginEnvironment)
	if fromEnv.IsZero() {
		t.Error("a resolved credential must not report as absent")
	}
	if fromEnv.Origin() != domain.OriginEnvironment {
		t.Errorf("origin = %q, want %q", fromEnv.Origin(), domain.OriginEnvironment)
	}
}
