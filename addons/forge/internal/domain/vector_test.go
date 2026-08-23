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
	"math"
	"strings"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

const cosineTolerance = 1e-9

// Expectations come from the definition of cosine similarity, not from running
// the implementation: identical direction is 1, orthogonal is 0, opposed is -1,
// and scaling a vector cannot change its direction.
func TestCosineMatchesItsGeometricDefinition(t *testing.T) {
	cases := []struct {
		name string
		a, b domain.Vector
		want float64
	}{
		{"identical vectors are maximally similar", domain.Vector{1, 2, 3}, domain.Vector{1, 2, 3}, 1},
		{"orthogonal vectors are unrelated", domain.Vector{1, 0}, domain.Vector{0, 1}, 0},
		{"opposed vectors are maximally dissimilar", domain.Vector{1, 2}, domain.Vector{-1, -2}, -1},
		{"magnitude does not affect direction", domain.Vector{1, 1}, domain.Vector{7, 7}, 1},
		{"45 degrees apart", domain.Vector{1, 0}, domain.Vector{1, 1}, math.Sqrt2 / 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.Cosine(tc.a, tc.b)
			if err != nil {
				t.Fatalf("Cosine: %v", err)
			}
			if math.Abs(got-tc.want) > cosineTolerance {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A dimension mismatch means the two vectors came from different embedding
// models and are not comparable at all. Returning 0 would report that as
// "unrelated", which is a legitimate-looking similarity verdict and would let
// the gate pass a duplicate through on the strength of a bug.
func TestCosineRefusesIncomparableInputRatherThanScoringIt(t *testing.T) {
	cases := []struct {
		name string
		a, b domain.Vector
		want string
	}{
		{"different dimensions", domain.Vector{1, 2, 3}, domain.Vector{1, 2}, "dimension mismatch"},
		{"empty", domain.Vector{}, domain.Vector{}, "empty"},
		{"zero magnitude", domain.Vector{0, 0}, domain.Vector{1, 1}, "zero-magnitude"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.Cosine(tc.a, tc.b)
			if err == nil {
				t.Fatalf("want an error, got score %v", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error must name the cause %q, got: %v", tc.want, err)
			}
		})
	}
}

// A database file is portable, so the encoding must be too. The expected bytes
// are IEEE-754 single precision, little-endian, read off the standard rather
// than off this implementation: 1.0 is 0x3F800000, -2.0 is 0xC0000000.
func TestVectorBlobEncodingIsLittleEndianIEEE754(t *testing.T) {
	got := domain.MarshalVector(domain.Vector{1, -2})
	want := []byte{0x00, 0x00, 0x80, 0x3F, 0x00, 0x00, 0x00, 0xC0}
	if fmt.Sprintf("%x", got) != fmt.Sprintf("%x", want) {
		t.Errorf("got % x, want % x", got, want)
	}
}

func TestVectorSurvivesABlobRoundTrip(t *testing.T) {
	original := domain.Vector{0, 1, -1, 0.5, float32(math.Pi)}
	back, err := domain.UnmarshalVector(domain.MarshalVector(original))
	if err != nil {
		t.Fatalf("UnmarshalVector: %v", err)
	}
	if back.Dim() != original.Dim() {
		t.Fatalf("dimension changed: %d -> %d", original.Dim(), back.Dim())
	}
	for i := range original {
		if back[i] != original[i] {
			t.Errorf("element %d: %v -> %v", i, original[i], back[i])
		}
	}
}

func TestTruncatedVectorBlobIsRejected(t *testing.T) {
	if _, err := domain.UnmarshalVector([]byte{1, 2, 3}); err == nil {
		t.Fatal("a blob that is not a whole number of float32 must be rejected")
	}
}

func question(prompt string, choices []coredomain.Choice, correct, tags []string) *coredomain.PackQuestion {
	return &coredomain.PackQuestion{
		Type:             coredomain.SingleSelect,
		Prompt:           prompt,
		Choices:          choices,
		CorrectChoiceIDs: correct,
		Tags:             tags,
	}
}

// FORGE.md 7 excludes introductions and explanations from the comparison
// representation: they share source language across questions drawn from the
// same evidence, so including them drives exactly the false positives the
// gate's quality target rules out.
func TestCanonicalTextIgnoresIntroAndRationale(t *testing.T) {
	choices := []coredomain.Choice{{ID: "a", Text: "yes"}, {ID: "b", Text: "no"}}
	plain := question("Is it?", choices, []string{"a"}, nil)

	decorated := question("Is it?", choices, []string{"a"}, nil)
	decorated.Intro = "A long shared preamble copied from the same source page."
	decorated.Rationale = &coredomain.Rationale{Correct: "Because of the thing."}

	if domain.CanonicalText(plain) != domain.CanonicalText(decorated) {
		t.Error("intro and rationale must not affect the comparison representation")
	}
}

// Choice and tag order is presentation, exactly as it is for D-007's hash. Two
// candidates differing only in ordering are the same question.
func TestCanonicalTextIsInvariantToPresentationOrder(t *testing.T) {
	forward := question("Which?",
		[]coredomain.Choice{{ID: "a", Text: "alpha"}, {ID: "b", Text: "beta"}},
		[]string{"a"}, []string{"go", "sql"})
	reversed := question("Which?",
		[]coredomain.Choice{{ID: "b", Text: "beta"}, {ID: "a", Text: "alpha"}},
		[]string{"a"}, []string{"sql", "go"})

	if domain.CanonicalText(forward) != domain.CanonicalText(reversed) {
		t.Errorf("order changed the representation:\n%q\n%q",
			domain.CanonicalText(forward), domain.CanonicalText(reversed))
	}
}

// The correct answer is content, not presentation. Two questions offering the
// same options but disagreeing on the answer are different questions, and a
// gate that could not tell them apart would reject a correction as a duplicate.
func TestCanonicalTextDistinguishesTheCorrectAnswer(t *testing.T) {
	choices := []coredomain.Choice{{ID: "a", Text: "alpha"}, {ID: "b", Text: "beta"}}
	first := question("Which?", choices, []string{"a"}, nil)
	second := question("Which?", choices, []string{"b"}, nil)

	if domain.CanonicalText(first) == domain.CanonicalText(second) {
		t.Error("a different correct answer must produce a different representation")
	}
}

// Choice ids are arbitrary labels chosen by whoever authored the pack, so the
// same question written with a different id scheme must still compare equal.
func TestCanonicalTextIgnoresChoiceIdSchemes(t *testing.T) {
	letters := question("Which?",
		[]coredomain.Choice{{ID: "a", Text: "alpha"}, {ID: "b", Text: "beta"}},
		[]string{"a"}, nil)
	numbers := question("Which?",
		[]coredomain.Choice{{ID: "1", Text: "alpha"}, {ID: "2", Text: "beta"}},
		[]string{"1"}, nil)

	if domain.CanonicalText(letters) != domain.CanonicalText(numbers) {
		t.Error("choice id scheme must not affect the comparison representation")
	}
}
