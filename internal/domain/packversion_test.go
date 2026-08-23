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
	"time"

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

// The single most expensive thing D-017 could have broken. The hash recipe is
// frozen (D-007) and lives in a UNIQUE constraint in every existing database,
// so if pack-level generation metadata fed the hash, the same question would
// dedup differently depending on which schema version carried it — and a
// re-import of a 0.2.0 export of a 0.1.0 pack would silently duplicate
// everything.
func TestGenerationMetadataNeverChangesTheContentHash(t *testing.T) {
	question := domain.PackQuestion{
		Type:             domain.SingleSelect,
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []domain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
		CorrectChoiceIDs: []string{"a"},
		Difficulty:       domain.DifficultyEasy,
	}

	bare := domain.Pack{
		PackVersion: "0.1.0",
		Topic:       domain.PackTopic{Slug: "go", Name: "Go"},
		Questions:   []domain.PackQuestion{question},
	}

	generated := domain.Pack{
		PackVersion: domain.PackVersionGenerated,
		Topic:       domain.PackTopic{Slug: "go", Name: "Go"},
		Questions:   []domain.PackQuestion{question},
		GenerationSpec: &domain.GenerationSpec{
			Topic:       "Go concurrency",
			Description: "goroutines and channels",
			Count:       10,
			Difficulty:  domain.DifficultyEasy,
			Style:       "exam",
			Language:    "en",
		},
		Provenance: &domain.Provenance{
			GeneratedAt:  time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC),
			Model:        domain.ModelIdentity{Provider: "ollama", Model: "qwen3:8b"},
			Verifier:     domain.ModelIdentity{Provider: "ollama", Model: "qwen3:8b"},
			Sources:      []domain.SourceRef{{ID: "s1", URL: "https://example.test/a", Title: "A"}},
			ForgeVersion: "0.3.0",
		},
	}

	bareHash := domain.ComputeQuestionHash(bare.Topic.Slug, &bare.Questions[0])
	generatedHash := domain.ComputeQuestionHash(generated.Topic.Slug, &generated.Questions[0])

	if bareHash != generatedHash {
		t.Errorf("pack-level metadata leaked into the content hash:\n 0.1.0: %s\n 0.2.0: %s",
			bareHash, generatedHash)
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
