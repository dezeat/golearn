package domain_test

import (
	"testing"

	"github.com/dezeat/golearn/internal/domain"
)

func TestComputeQuestionHash_Stability(t *testing.T) {
	q := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Intro:  "Some context.",
		Prompt: "What is Go?",
		Choices: []domain.Choice{
			{ID: "A", Text: "A language"},
			{ID: "B", Text: "A game"},
		},
		CorrectChoiceIDs: []string{"A"},
	}

	hash1 := domain.ComputeQuestionHash("go-basics", q)
	hash2 := domain.ComputeQuestionHash("go-basics", q)

	if hash1 != hash2 {
		t.Errorf("hash not stable: %s != %s", hash1, hash2)
	}
	if len(hash1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got len=%d", len(hash1))
	}
}

func TestComputeQuestionHash_CorrectIDOrderIndependent(t *testing.T) {
	q1 := &domain.PackQuestion{
		Type:   domain.MultiSelect,
		Prompt: "Pick two",
		Choices: []domain.Choice{
			{ID: "A", Text: "X"},
			{ID: "B", Text: "Y"},
			{ID: "C", Text: "Z"},
		},
		CorrectChoiceIDs: []string{"A", "C"},
	}
	q2 := &domain.PackQuestion{
		Type:   domain.MultiSelect,
		Prompt: "Pick two",
		Choices: []domain.Choice{
			{ID: "A", Text: "X"},
			{ID: "B", Text: "Y"},
			{ID: "C", Text: "Z"},
		},
		CorrectChoiceIDs: []string{"C", "A"},
	}

	hash1 := domain.ComputeQuestionHash("topic", q1)
	hash2 := domain.ComputeQuestionHash("topic", q2)

	if hash1 != hash2 {
		t.Errorf("hash should be identical regardless of correct_choice_ids order: %s != %s", hash1, hash2)
	}
}

func TestComputeQuestionHash_DifferentPromptDifferentHash(t *testing.T) {
	q1 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "Question A",
		Choices: []domain.Choice{
			{ID: "A", Text: "Yes"},
			{ID: "B", Text: "No"},
		},
		CorrectChoiceIDs: []string{"A"},
	}
	q2 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "Question B",
		Choices: []domain.Choice{
			{ID: "A", Text: "Yes"},
			{ID: "B", Text: "No"},
		},
		CorrectChoiceIDs: []string{"A"},
	}

	hash1 := domain.ComputeQuestionHash("topic", q1)
	hash2 := domain.ComputeQuestionHash("topic", q2)

	if hash1 == hash2 {
		t.Error("different prompts should produce different hashes")
	}
}

func TestComputeQuestionHash_DifferentTopicDifferentHash(t *testing.T) {
	q := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "Same question",
		Choices: []domain.Choice{
			{ID: "A", Text: "Yes"},
			{ID: "B", Text: "No"},
		},
		CorrectChoiceIDs: []string{"A"},
	}

	hash1 := domain.ComputeQuestionHash("topic-1", q)
	hash2 := domain.ComputeQuestionHash("topic-2", q)

	if hash1 == hash2 {
		t.Error("different topic slugs should produce different hashes")
	}
}

func TestComputeQuestionHash_DifficultyChangesDifferentHash(t *testing.T) {
	q1 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "Same question",
		Choices: []domain.Choice{
			{ID: "1", Text: "Yes"},
			{ID: "2", Text: "No"},
		},
		CorrectChoiceIDs: []string{"1"},
		Difficulty:       domain.DifficultyEasy,
	}
	q2 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "Same question",
		Choices: []domain.Choice{
			{ID: "1", Text: "Yes"},
			{ID: "2", Text: "No"},
		},
		CorrectChoiceIDs: []string{"1"},
		Difficulty:       domain.DifficultyHard,
	}

	hash1 := domain.ComputeQuestionHash("topic", q1)
	hash2 := domain.ComputeQuestionHash("topic", q2)

	if hash1 == hash2 {
		t.Error("different difficulty should produce different hashes")
	}
}

func TestComputeQuestionHash_WhitespaceNormalized(t *testing.T) {
	q1 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "  What is Go?  ",
		Intro:  "  context  ",
		Choices: []domain.Choice{
			{ID: "A", Text: "  A language  "},
			{ID: "B", Text: "A game"},
		},
		CorrectChoiceIDs: []string{"A"},
	}
	q2 := &domain.PackQuestion{
		Type:   domain.SingleSelect,
		Prompt: "What is Go?",
		Intro:  "context",
		Choices: []domain.Choice{
			{ID: "A", Text: "A language"},
			{ID: "B", Text: "A game"},
		},
		CorrectChoiceIDs: []string{"A"},
	}

	hash1 := domain.ComputeQuestionHash("go", q1)
	hash2 := domain.ComputeQuestionHash("go", q2)

	if hash1 != hash2 {
		t.Errorf("whitespace-only differences should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"  hello  ", "hello"},
		{"line1\r\nline2", "line1\nline2"},
		{"line1\rline2", "line1\nline2"},
		{"\n\n  spaced  \n\n", "spaced"},
	}
	for _, tt := range tests {
		got := domain.NormalizeText(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
