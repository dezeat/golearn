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
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// Redact was added because its own test caught a real leak: provider error
// bodies were being echoed verbatim into Forge's diagnostics, and a provider
// or proxy that reflects the submitted key back would put a credential into
// the text users paste into bug reports.
func TestRedactRemovesCredentialShapesWhoeverIssuedThem(t *testing.T) {
	// These deterministic, low-entropy stand-ins preserve credential shapes
	// without creating secret-shaped values for historical scanners.
	cases := []struct {
		name   string
		in     string
		canary string
	}{
		{"openai style", `{"error":"invalid key sk-aaaaaaaaaaaaaaaa"}`, "sk-aaaaaaaaaaaaaaaa"},
		// Anthropic keys are covered by the same sk- pattern rather than a
		// separate one; the case stays because the coverage claim is what
		// matters, not which pattern happens to satisfy it.
		{"anthropic style", `rejected sk-ant-aaaaaaaaaaaaaaaa`, "sk-ant-aaaaaaaaaaaaaaaa"},
		{"bearer header echo", `Authorization: Bearer aaaaaaaaaaaaaaaa`, "aaaaaaaaaaaaaaaa"},
		{"labeled key", `api_key=aaaaaaaa`, "aaaaaaaa"},
		{"labeled token", `token: aaaaaaaa`, "aaaaaaaa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Redact(tc.in)
			if strings.Contains(got, tc.canary) {
				t.Errorf("credential survived redaction: %q", got)
			}
			if !strings.Contains(got, domain.RedactedPlaceholder) {
				t.Errorf("redaction should leave a visible marker so the reader knows text was removed, got %q", got)
			}
		})
	}
}

// Over-redaction has a cost too: an error message with its substance removed
// is not diagnosable. Ordinary provider prose must survive intact.
func TestRedactLeavesOrdinaryDiagnosticTextIntact(t *testing.T) {
	for _, ordinary := range []string{
		"model qwen3:4b is not installed",
		"rate limit exceeded, retry after 30s",
		"upstream connect error or disconnect/reset before headers",
		"invalid request: max_tokens must be positive",
	} {
		if got := domain.Redact(ordinary); got != ordinary {
			t.Errorf("redaction damaged a diagnostic message:\n in:  %q\n out: %q", ordinary, got)
		}
	}
}
