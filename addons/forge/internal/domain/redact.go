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

import "regexp"

// credentialShapes match text that looks like a credential regardless of which
// provider issued it.
//
// Shape rather than name is the whole point: a guard that knows only today's
// provider names stops working the moment a new provider is added, which is
// exactly when it is needed most. The same reasoning already governs the
// redaction guard over Forge's config output.
// A separate sk-ant- pattern was removed after mutation testing showed it
// could not fail: every Anthropic-style key is already matched by the sk-
// pattern below, so it asserted nothing while implying coverage it did not
// add. A redundant guard is worse than a missing one, because it reads as
// protection that was never there.
var credentialShapes = []*regexp.Regexp{
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{16,}`),            // OpenAI- and Anthropic-style
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._-]{16,}`), // Authorization header
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*"?[A-Za-z0-9._\-]{8,}"?`),
}

// RedactedPlaceholder is what a redacted credential is replaced with.
const RedactedPlaceholder = "[redacted]"

// Redact removes credential-shaped substrings from text that is about to be
// shown to a user or written to a diagnostic.
//
// It exists because of a leak found by its own test: a provider's error body
// was being echoed verbatim into the error Forge reports, and a provider or a
// misconfigured proxy that reflects the submitted key back in that body would
// put the credential straight into a diagnostic — the exact place users paste
// into bug reports.
//
// This is defense in depth, not the primary defense. [Secret] refuses to
// render its value under any formatting verb, which stops the credential
// Forge holds from leaking. Redact covers the other direction: text arriving
// from outside that happens to contain one.
func Redact(s string) string {
	for _, re := range credentialShapes {
		s = re.ReplaceAllString(s, RedactedPlaceholder)
	}
	return s
}
