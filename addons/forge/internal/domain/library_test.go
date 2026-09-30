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
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// The gate compares stored library content against candidates that are still
// pack questions. If the projection drops a field the comparison reads, every
// library canonical text is systematically shorter than the candidate's, every
// score is depressed, and the gate produces uniform false negatives with a
// fully green suite. Byte-identical is the only assertion that catches it.
func TestALibraryQuestionAndAnIdenticalCandidateCanonicalizeIdentically(t *testing.T) {
	candidate := coredomain.PackQuestion{
		Type:             coredomain.SingleSelect,
		Intro:            "Read the following carefully.",
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []coredomain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
		CorrectChoiceIDs: []string{"a"},
		Tags:             []string{"concurrency", "keywords"},
		Difficulty:       coredomain.DifficultyEasy,
	}

	// The same content as the library stores it: extra bookkeeping fields, and
	// the rationale/intro the comparison representation excludes.
	stored := coredomain.Question{
		ID:               42,
		TopicID:          7,
		Type:             candidate.Type,
		Intro:            candidate.Intro,
		Prompt:           candidate.Prompt,
		Choices:          candidate.Choices,
		CorrectChoiceIDs: candidate.CorrectChoiceIDs,
		Tags:             candidate.Tags,
		Difficulty:       candidate.Difficulty,
		Rationale:        coredomain.Rationale{Correct: "go launches a goroutine"},
		Source:           "manual",
		Confidence:       1.0,
		Hash:             "deadbeef",
		CreatedAt:        time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC),
	}

	projected := domain.AsPackQuestion(stored)
	want := domain.CanonicalText(&candidate)
	got := domain.CanonicalText(&projected)

	if got != want {
		t.Errorf("a library question and an identical candidate must canonicalize identically\nlibrary:   %q\ncandidate: %q", got, want)
	}
}

// Each field the comparison reads gets its own case, so a projection that
// drops exactly one of them names which one rather than failing as a blob.
func TestTheProjectionCarriesEveryFieldTheComparisonReads(t *testing.T) {
	base := coredomain.Question{
		Type:             coredomain.SingleSelect,
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []coredomain.Choice{{ID: "a", Text: "go"}, {ID: "b", Text: "run"}},
		CorrectChoiceIDs: []string{"a"},
		Tags:             []string{"concurrency"},
	}
	baseline := domain.CanonicalText(ptr(domain.AsPackQuestion(base)))

	tests := map[string]func(q *coredomain.Question){
		"prompt":  func(q *coredomain.Question) { q.Prompt = "Which keyword starts a thread?" },
		"choices": func(q *coredomain.Question) { q.Choices[1] = coredomain.Choice{ID: "b", Text: "spawn"} },
		"correct": func(q *coredomain.Question) { q.CorrectChoiceIDs = []string{"b"} },
		"tags":    func(q *coredomain.Question) { q.Tags = []string{"scheduling"} },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changed := base
			changed.Choices = append([]coredomain.Choice(nil), base.Choices...)
			mutate(&changed)

			if got := domain.CanonicalText(ptr(domain.AsPackQuestion(changed))); got == baseline {
				t.Errorf("changing %s left the canonical text identical, so the projection drops it", name)
			}
		})
	}
}

// The comparison representation excludes intro and rationale (FORGE.md 7), so
// the projection must not smuggle them back in through a stored question.
func TestTheProjectionDoesNotReintroduceExcludedFields(t *testing.T) {
	bare := coredomain.Question{
		Type:             coredomain.SingleSelect,
		Prompt:           "Which keyword starts a goroutine?",
		Choices:          []coredomain.Choice{{ID: "a", Text: "go"}},
		CorrectChoiceIDs: []string{"a"},
	}
	decorated := bare
	decorated.Intro = "A long shared preamble every question in this pack repeats."
	decorated.Rationale = coredomain.Rationale{Correct: "because the spec says so"}

	if domain.CanonicalText(ptr(domain.AsPackQuestion(bare))) != domain.CanonicalText(ptr(domain.AsPackQuestion(decorated))) {
		t.Error("intro or rationale reached the comparison representation; FORGE.md 7 excludes both because they share source language and drive false positives")
	}
}

func ptr(q coredomain.PackQuestion) *coredomain.PackQuestion { return &q }
