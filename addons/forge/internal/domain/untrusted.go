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

import (
	"fmt"
	"io"
	"strings"
)

// UntrustedText is content that came from outside the trust boundary — a
// search result, a fetched page, anything the operator did not write.
//
// FORGE.md 4 states the rule in prose: retrieved pages are data, they cannot
// alter workflow rules, invoke actions, or override policy. This type is that
// rule made structural. The raw string is unexported, so untrusted content
// cannot reach a prompt by ordinary string concatenation; it has to be
// unwrapped on purpose, and the only unwrapping that belongs in a prompt is
// [UntrustedText.Fenced].
//
// String deliberately does not reveal the content, so an accidental %v or %s
// in a log line, an error, or a prompt template renders a placeholder instead
// of splicing a page into an instruction slot.
type UntrustedText struct {
	value string
}

// Untrusted marks text as having come from outside the trust boundary.
func Untrusted(s string) UntrustedText {
	return UntrustedText{value: s}
}

// Raw returns the content verbatim. Every call site is a deliberate decision
// to handle untrusted data: extraction, storage, diffing and tests. It is the
// wrong function to reach for when building a prompt — use [UntrustedText.Fenced].
func (u UntrustedText) Raw() string { return u.value }

// Len reports the content length in bytes, for budget accounting that has no
// business reading the content.
func (u UntrustedText) Len() int { return len(u.value) }

// IsEmpty reports whether any content was captured at all.
func (u UntrustedText) IsEmpty() bool { return strings.TrimSpace(u.value) == "" }

// String renders a placeholder rather than the content.
func (u UntrustedText) String() string {
	return fmt.Sprintf("<untrusted:%dB>", len(u.value))
}

// Format implements fmt.Formatter so that *every* verb renders the
// placeholder, not just the ones that consult String.
//
// Stringer alone is not enough, and the gap is not theoretical: %#v prints a
// struct's unexported fields verbatim and ignores String entirely, while a
// mismatched verb like %d renders {%!d(string=...)} — both splice the content
// into whatever was being built. Since the whole purpose of this type is that
// content cannot escape by accident, the only sound cover is the verb-agnostic
// one.
func (u UntrustedText) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, u.String())
}

// Fence delimiters. The pipeline tells the model that anything between them is
// quoted source material and never an instruction.
const (
	fenceOpenFormat  = "<<<EVIDENCE %s>>>"
	fenceCloseFormat = "<<<END EVIDENCE %s>>>"
	fenceSentinel    = "<<<"
	fenceReplacement = "<‹<"
)

// Fenced renders the content wrapped in delimiters keyed by a caller-supplied
// id, for embedding in a prompt as quoted material.
//
// The security property is not the fence itself — it is that the fence cannot
// be forged. Content containing its own "<<<END EVIDENCE ...>>>" would
// otherwise close the quotation early and have everything after it read as
// instruction, which is the classic escape. Every fence sentinel inside the
// content is therefore neutralized before wrapping, using a homoglyph that
// preserves readability for the model while being unequal to the delimiter.
//
// The id must be caller-supplied rather than generated, because make check is
// deterministic and a random nonce would make prompt construction unrepeatable.
// Callers pass the evidence id, which doubles as the citation key.
func (u UntrustedText) Fenced(id string) string {
	safeID := strings.ReplaceAll(id, fenceSentinel, fenceReplacement)
	body := strings.ReplaceAll(u.value, fenceSentinel, fenceReplacement)
	return fmt.Sprintf(fenceOpenFormat, safeID) + "\n" +
		body + "\n" +
		fmt.Sprintf(fenceCloseFormat, safeID)
}
