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
)

// SecretOrigin names where a resolved credential came from. A user must be
// able to ask which source supplied the active key without the answer printing
// the key — "environment" is diagnosable, the value is not.
type SecretOrigin string

// The resolution sources, in the precedence order Forge applies. See D-019 for
// why the environment leads rather than the keychain.
const (
	// OriginEnvironment is a credential taken from the process environment.
	OriginEnvironment SecretOrigin = "environment"
	// OriginKeychain is a credential taken from an OS keychain. No adapter
	// supplies this yet; the constant exists so the precedence rule is written
	// and tested against the full order rather than retrofitted later.
	OriginKeychain SecretOrigin = "keychain"
	// OriginNone means no credential was resolved.
	OriginNone SecretOrigin = "none"
)

// Secret is a credential that refuses to print itself.
//
// The unexported field plus the String and GoString methods mean a credential
// cannot reach a log line, an error, a diagnostic or a bug report through
// ordinary formatting — %s, %v, %#v and fmt.Println all render the redaction.
// Reading the value has to be deliberate, and the only place that should is
// the code putting it in an Authorization header.
//
// This is defense in depth, not the only defense: internal/config carries a
// shape-based redaction guard for output that never goes through this type.
type Secret struct {
	value  string
	origin SecretOrigin
}

// NewSecret wraps a credential value with the origin that supplied it.
func NewSecret(value string, origin SecretOrigin) Secret {
	return Secret{value: value, origin: origin}
}

// Reveal returns the credential. Every call site is a deliberate decision to
// handle a secret in the clear.
func (s Secret) Reveal() string { return s.value }

// Origin reports which source supplied the credential.
func (s Secret) Origin() SecretOrigin { return s.origin }

// IsZero reports whether any credential was resolved at all.
func (s Secret) IsZero() bool { return s.value == "" }

// redacted is what a Secret renders as under every formatting verb.
const redacted = "[redacted]"

// String renders the redaction instead of the credential.
func (s Secret) String() string { return redacted }

// GoString covers %#v, which ignores String and would otherwise print the
// struct's unexported fields verbatim.
func (s Secret) GoString() string { return redacted }

// Format implements fmt.Formatter so that every verb renders the redaction.
// Stringer and GoStringer together still leave a mismatched verb such as %d
// printing {%!d(string=sk-...)}, which is a credential in a log line by any
// other name.
func (s Secret) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, redacted)
}
