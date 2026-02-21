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

	"github.com/dezeat/golearn/internal/domain"
)

func TestValidateQuestion_ValidSingleSelect(t *testing.T) {
	q := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "What is 1+1?",
		Choices: []domain.Choice{
			{ID: "A", Text: "1"},
			{ID: "B", Text: "2"},
		},
		CorrectChoiceIDs: []string{"B"},
	}
	errs := domain.ValidateQuestion(q, 0, "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateQuestion_ValidMultiSelect(t *testing.T) {
	q := &domain.PackQuestion{
		Type:   domain.MultiSelect,
		Prompt: "Select all primes",
		Choices: []domain.Choice{
			{ID: "A", Text: "2"},
			{ID: "B", Text: "3"},
			{ID: "C", Text: "4"},
		},
		CorrectChoiceIDs: []string{"A", "B"},
	}
	errs := domain.ValidateQuestion(q, 0, "test.yaml")
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateQuestion_Failures(t *testing.T) {
	tests := []struct {
		name      string
		q         domain.PackQuestion
		wantField string
	}{
		{
			name:      "unsupported type",
			q:         domain.PackQuestion{Type: "essay", Prompt: "x", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}}, CorrectChoiceIDs: []string{"A"}},
			wantField: "type",
		},
		{
			name:      "empty prompt",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "  ", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}}, CorrectChoiceIDs: []string{"A"}},
			wantField: "prompt",
		},
		{
			name:      "too few choices",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "q?", Choices: []domain.Choice{{ID: "A", Text: "a"}}, CorrectChoiceIDs: []string{"A"}},
			wantField: "choices",
		},
		{
			name:      "duplicate choice IDs",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "q?", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "A", Text: "b"}}, CorrectChoiceIDs: []string{"A"}},
			wantField: "choices.id",
		},
		{
			name:      "empty correct_choice_ids",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "q?", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}}, CorrectChoiceIDs: []string{}},
			wantField: "correct_choice_ids",
		},
		{
			name:      "single_select with multiple correct",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "q?", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}}, CorrectChoiceIDs: []string{"A", "B"}},
			wantField: "correct_choice_ids",
		},
		{
			name:      "correct ID references non-existent choice",
			q:         domain.PackQuestion{Type: domain.SingleSelect, Prompt: "q?", Choices: []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}}, CorrectChoiceIDs: []string{"C"}},
			wantField: "correct_choice_ids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := domain.ValidateQuestion(&tt.q, 0, "test.yaml")
			if len(errs) == 0 {
				t.Fatal("expected validation errors, got none")
			}
			found := false
			for _, e := range errs {
				if e.Field == tt.wantField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field %q, got %v", tt.wantField, errs)
			}
		})
	}
}

func TestValidatePack_EmptyPack(t *testing.T) {
	p := &domain.Pack{}
	errs := domain.ValidatePack(p, "empty.yaml")
	if len(errs) == 0 {
		t.Fatal("expected errors for empty pack")
	}
	// Should flag pack_version, topic.slug, topic.name, and questions.
	fields := make(map[string]bool)
	for _, e := range errs {
		fields[e.Field] = true
	}
	for _, f := range []string{"pack_version", "topic.slug", "topic.name", "questions"} {
		if !fields[f] {
			t.Errorf("expected error on field %q", f)
		}
	}
}

func TestValidationError_Format(t *testing.T) {
	e := domain.ValidationError{
		File:    "packs/go.yaml",
		Index:   2,
		Field:   "prompt",
		Message: "must be non-empty",
	}
	got := e.Error()
	want := "packs/go.yaml: question[2].prompt: must be non-empty"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateQuestion_DifficultyEnum(t *testing.T) {
	base := domain.PackQuestion{
		Type:             domain.SingleSelect,
		Prompt:           "Question?",
		Choices:          []domain.Choice{{ID: "1", Text: "A"}, {ID: "2", Text: "B"}},
		CorrectChoiceIDs: []string{"1"},
	}

	// Valid values should pass.
	for _, d := range []domain.Difficulty{domain.DifficultyEasy, domain.DifficultyMedium, domain.DifficultyHard, domain.DifficultyUnset} {
		q := base
		q.Difficulty = d
		errs := domain.ValidateQuestion(&q, 0, "test.yaml")
		if len(errs) != 0 {
			t.Errorf("difficulty %q should be valid, got errors: %v", d, errs)
		}
	}

	// Invalid values should fail.
	for _, d := range []domain.Difficulty{"1", "2", "5", "expert", "EASY"} {
		q := base
		q.Difficulty = d
		errs := domain.ValidateQuestion(&q, 0, "test.yaml")
		found := false
		for _, e := range errs {
			if e.Field == "difficulty" {
				found = true
			}
		}
		if !found {
			t.Errorf("difficulty %q should be rejected, got no difficulty error", d)
		}
	}
}
