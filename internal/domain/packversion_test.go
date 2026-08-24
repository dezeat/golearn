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
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"

	"github.com/dezeat/golearn/internal/domain"
)

// D-017 evolves the pack schema to 0.2.0 while keeping 0.1.x importable. The
// compatibility policy is therefore "same major, minor at or below the one
// this binary knows" — not "exactly this minor", which is what the pre-0.2.0
// rule enforced and which would have made every Forge pack unreadable by the
// offline binary.
func TestPackVersionCompatibilityAcceptsOlderMinorsAndRefusesNewer(t *testing.T) {
	cases := []struct {
		version   string
		acceptRef bool
		reason    string
	}{
		{"0.1.0", true, "the original schema stays importable"},
		{"0.1.7", true, "a patch of an older minor stays importable"},
		{"0.2.0", true, "the schema Forge emits"},
		{"0.2.3", true, "a patch of the current minor"},
		{"0.3.0", false, "a newer minor carries fields this binary cannot honor"},
		{"1.0.0", false, "a newer major is a different contract"},
		{"0.2", false, "must be a three-component semantic version"},
		{"", false, "empty is not a version"},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			msg := domain.ValidatePackVersion(tc.version)
			accepted := msg == ""
			if accepted != tc.acceptRef {
				t.Fatalf("ValidatePackVersion(%q) accepted=%v, want %v (%s); message: %q",
					tc.version, accepted, tc.acceptRef, tc.reason, msg)
			}
			if !accepted && !strings.Contains(msg, tc.version) && tc.version != "" {
				t.Errorf("refusal should name the offending version, got %q", msg)
			}
		})
	}
}

// The single most expensive thing D-017 could have broken — and the first
// version of this test could not have caught it.
//
// It built two packs and hashed a question from each, but ComputeQuestionHash
// takes a topic slug and a *PackQuestion, not a Pack. Both arguments were
// identical, so it asserted hash(x) == hash(x): a tautology that no change to
// the recipe could fail, because a change making the hash depend on pack
// metadata would alter the signature and fail to compile instead.
//
// What can actually regress is the recipe's *inputs*. So the guard is now a
// pinned digest from an external oracle — computed by hand from the documented
// recipe in docs/architecture.md, not by running the implementation — plus an
// explicit check that the per-question provenance fields D-017 excludes really
// are excluded.
func TestTheContentHashRecipeIsFrozen(t *testing.T) {
	question := domain.PackQuestion{
		Type:             domain.SingleSelect,
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []domain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
		CorrectChoiceIDs: []string{"a"},
		Difficulty:       domain.DifficultyEasy,
	}

	// SHA-256 over the documented field order, null-byte separated:
	//   "go" \x00 "single_select" \x00 "" \x00 prompt \x00
	//   "a"+"go" "b"+"run" \x00 "a" \x00 "easy"
	want := independentQuestionHash("go", &question)

	if got := domain.ComputeQuestionHash("go", &question); got != want {
		t.Errorf("the frozen hash recipe changed:\n got  %s\n want %s\n"+
			"This constant lives in every database's UNIQUE constraint; changing it "+
			"silently breaks dedup for every existing user (D-007).", got, want)
	}
}

// independentQuestionHash re-implements the D-007 recipe from its written
// specification, so the pinned value comes from the documented contract rather
// than from the code under test. A test whose expectation is produced by the
// implementation cannot detect the implementation changing.
func independentQuestionHash(topicSlug string, q *domain.PackQuestion) string {
	const sep = "\x00"
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
		}
	}
	write(topicSlug, sep, string(q.Type), sep, q.Intro, sep, q.Prompt, sep)
	for _, c := range q.Choices {
		write(c.ID, c.Text, sep)
	}
	sorted := append([]string(nil), q.CorrectChoiceIDs...)
	sort.Strings(sorted)
	write(strings.Join(sorted, ","), sep, string(q.Difficulty))
	return hex.EncodeToString(h.Sum(nil))
}

// D-017's actual claim: the per-question provenance fields are not hash
// inputs, so the same question dedups identically whether it arrived in a
// 0.1.x pack or a generated 0.2.0 one.
func TestPerQuestionProvenanceIsNotAHashInput(t *testing.T) {
	base := domain.PackQuestion{
		Type:             domain.SingleSelect,
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []domain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
		CorrectChoiceIDs: []string{"a"},
		Difficulty:       domain.DifficultyEasy,
	}
	bare := domain.ComputeQuestionHash("go", &base)

	confidence := domain.GeneratedConfidence
	generated := base
	generated.Source = "llm:ollama"
	generated.SourceRef = "s1"
	generated.Confidence = &confidence
	generated.Tags = []string{"concurrency"}
	generated.Rationale = &domain.Rationale{Correct: "The go keyword does."}

	if got := domain.ComputeQuestionHash("go", &generated); got != bare {
		t.Errorf("provenance leaked into the content hash:\n bare      %s\n generated %s\n"+
			"Dedup would then differ between a 0.1.x import and a 0.2.0 one.", bare, got)
	}
}

// FORGE.md 9 keeps hand-authored content at the manual default of 1.0 and
// requires generated questions to sit strictly below it, so that provenance
// class is visible in the data rather than only in a pack header.
func TestGeneratedConfidenceIsStrictlyBelowTheManualDefault(t *testing.T) {
	if domain.GeneratedConfidence >= domain.ManualConfidence {
		t.Errorf("generated confidence %v must be strictly below the manual default %v",
			domain.GeneratedConfidence, domain.ManualConfidence)
	}
	if domain.GeneratedConfidence <= 0 {
		t.Errorf("generated confidence %v must be positive", domain.GeneratedConfidence)
	}
}

// A 0.2.0 pack carrying no generation metadata is legal: the fields are
// optional additions, and a hand-authored file that simply declares the newer
// version must not be rejected for omitting them.
func TestGenerationMetadataIsOptionalAtSchema020(t *testing.T) {
	p := domain.Pack{
		PackVersion: domain.PackVersionGenerated,
		Topic:       domain.PackTopic{Slug: "go", Name: "Go"},
		Questions: []domain.PackQuestion{{
			Type:             domain.SingleSelect,
			Prompt:           "Is Go compiled?",
			Choices:          []domain.Choice{{ID: "a", Text: "yes"}, {ID: "b", Text: "no"}},
			CorrectChoiceIDs: []string{"a"},
		}},
	}
	if errs := domain.ValidatePack(&p, "t.yaml"); len(errs) != 0 {
		t.Errorf("0.2.0 without generation metadata must validate, got %v", errs)
	}
}

// Missing or unknown style stays backwards-compatible while #105 refines the
// intent enum. #121 requires this explicitly so the two do not become a
// circular dependency.
func TestUnknownStyleIsAcceptedRatherThanValidated(t *testing.T) {
	for _, style := range []string{"", "exam", "quiz-show", "a-style-nobody-has-defined-yet"} {
		p := domain.Pack{
			PackVersion: domain.PackVersionGenerated,
			Topic:       domain.PackTopic{Slug: "go", Name: "Go"},
			Questions: []domain.PackQuestion{{
				Type:             domain.SingleSelect,
				Prompt:           "Is Go compiled?",
				Choices:          []domain.Choice{{ID: "a", Text: "yes"}, {ID: "b", Text: "no"}},
				CorrectChoiceIDs: []string{"a"},
			}},
			GenerationSpec: &domain.GenerationSpec{Topic: "Go", Count: 1, Style: domain.Style(style)},
		}
		if errs := domain.ValidatePack(&p, "t.yaml"); len(errs) != 0 {
			t.Errorf("style %q must be accepted, got %v", style, errs)
		}
	}
}
