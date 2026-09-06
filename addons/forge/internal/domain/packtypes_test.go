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
	"testing"

	forgedomain "github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// The pack-format types must be *the same types* the core marshals, not
// look-alikes. Go does not allow a structurally identical named type to be
// passed where another is expected, so these calls compile only while the
// Forge names are true aliases — which is the property that stops the two
// definitions drifting into a pack that round-trips differently per binary.
//
// The assertion is made through function parameters rather than typed variable
// declarations because the two are equivalent to the compiler, and the
// declaration form reads to a linter as a redundant type annotation it should
// suggest deleting — deleting exactly the thing under test.
func asCoreSpec(s coredomain.GenerationSpec) string    { return s.Topic }
func asCoreProvenance(p coredomain.Provenance) string  { return p.ForgeVersion }
func asCoreModel(m coredomain.ModelIdentity) string    { return m.String() }
func asCoreSourceRef(r coredomain.SourceRef) string    { return r.ID }
func asCoreStyle(st coredomain.Style) coredomain.Style { return st }

func TestForgePackTypesAreTheCoreTypesNotCopies(t *testing.T) {
	if got := asCoreSpec(forgedomain.GenerationSpec{Topic: "go", Count: 1}); got != "go" {
		t.Errorf("spec.Topic = %q", got)
	}
	if got := asCoreProvenance(forgedomain.Provenance{ForgeVersion: "0.3.0"}); got != "0.3.0" {
		t.Errorf("provenance.ForgeVersion = %q", got)
	}
	if got := asCoreModel(forgedomain.ModelIdentity{Provider: "ollama", Model: "qwen3:4b"}); got != "ollama/qwen3:4b" {
		t.Errorf("model identity = %q", got)
	}
	if got := asCoreSourceRef(forgedomain.SourceRef{ID: "s1"}); got != "s1" {
		t.Errorf("source ref id = %q", got)
	}
	if got := asCoreStyle(forgedomain.Style("exam")); got != "exam" {
		t.Errorf("style = %q", got)
	}
}

// Forge emits the newer schema; the two constants must not drift apart.
func TestForgeEmitsTheGeneratedSchemaVersion(t *testing.T) {
	if forgedomain.PackVersion != coredomain.PackVersionGenerated {
		t.Errorf("Forge emits %q but the core's generated schema is %q",
			forgedomain.PackVersion, coredomain.PackVersionGenerated)
	}
	if forgedomain.PackVersion == coredomain.CurrentPackVersion {
		t.Error("Forge must emit the newer schema, not the core's export version")
	}
	if msg := coredomain.ValidatePackVersion(forgedomain.PackVersion); msg != "" {
		t.Errorf("the offline binary must be able to read what Forge emits: %s", msg)
	}
}
