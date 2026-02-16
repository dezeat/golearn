package app

import (
	"math/rand"
	"testing"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// makeQuestion is a test helper to build a minimal question with an ID.
func makeQuestion(id int64) domain.Question {
	return domain.Question{
		ID:               id,
		TopicID:          1,
		Type:             domain.SingleSelect,
		Prompt:           "question",
		Choices:          []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}},
		CorrectChoiceIDs: []string{"A"},
	}
}

func TestSelectQuestions_UnseenFirst(t *testing.T) {
	// All questions are unseen — stats map is empty.
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
		makeQuestion(3),
		makeQuestion(4),
		makeQuestion(5),
	}
	stats := map[int64]ports.QuestionStats{}
	rng := rand.New(rand.NewSource(42))

	selected := SelectQuestions(questions, stats, 3, rng)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(selected))
	}

	// All selected should come from the original set.
	ids := make(map[int64]bool)
	for _, q := range selected {
		if ids[q.ID] {
			t.Errorf("duplicate question ID %d in selection", q.ID)
		}
		ids[q.ID] = true
	}
}

func TestSelectQuestions_WeakPrioritised(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1), // seen, all correct (rest bucket)
		makeQuestion(2), // unseen
		makeQuestion(3), // seen, high wrong rate (weak bucket)
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 5, Wrong: 0}, // 0% wrong
		3: {QuestionID: 3, Attempts: 5, Wrong: 4}, // 80% wrong
	}
	rng := rand.New(rand.NewSource(42))

	selected := SelectQuestions(questions, stats, 3, rng)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(selected))
	}

	// First should be unseen (Q2), then weak (Q3), then rest (Q1).
	if selected[0].ID != 2 {
		t.Errorf("expected unseen question (ID=2) first, got ID=%d", selected[0].ID)
	}
	if selected[1].ID != 3 {
		t.Errorf("expected weak question (ID=3) second, got ID=%d", selected[1].ID)
	}
	if selected[2].ID != 1 {
		t.Errorf("expected remaining question (ID=1) third, got ID=%d", selected[2].ID)
	}
}

func TestSelectQuestions_WeakSortedByWrongRate(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1), // 20% wrong
		makeQuestion(2), // 80% wrong — should come first among weak
		makeQuestion(3), // 50% wrong
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 10, Wrong: 2},
		2: {QuestionID: 2, Attempts: 10, Wrong: 8},
		3: {QuestionID: 3, Attempts: 10, Wrong: 5},
	}
	rng := rand.New(rand.NewSource(42))

	selected := SelectQuestions(questions, stats, 3, rng)
	if len(selected) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(selected))
	}

	// Weak bucket sorted by wrong rate desc: Q2 (80%), Q3 (50%), Q1 (20%).
	if selected[0].ID != 2 {
		t.Errorf("expected Q2 (highest wrong rate) first, got ID=%d", selected[0].ID)
	}
	if selected[1].ID != 3 {
		t.Errorf("expected Q3 (second highest) second, got ID=%d", selected[1].ID)
	}
	if selected[2].ID != 1 {
		t.Errorf("expected Q1 (lowest wrong rate) third, got ID=%d", selected[2].ID)
	}
}

func TestSelectQuestions_CappedAtN(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
		makeQuestion(3),
		makeQuestion(4),
		makeQuestion(5),
	}
	stats := map[int64]ports.QuestionStats{}
	rng := rand.New(rand.NewSource(42))

	selected := SelectQuestions(questions, stats, 2, rng)
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(selected))
	}
}

func TestSelectQuestions_DeterministicWithSeed(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
		makeQuestion(3),
		makeQuestion(4),
		makeQuestion(5),
	}
	stats := map[int64]ports.QuestionStats{}

	// Two runs with the same seed should produce the same order.
	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))

	s1 := SelectQuestions(questions, stats, 5, rng1)
	s2 := SelectQuestions(questions, stats, 5, rng2)

	for i := range s1 {
		if s1[i].ID != s2[i].ID {
			t.Errorf("position %d: ID %d vs %d — not deterministic", i, s1[i].ID, s2[i].ID)
		}
	}
}

func TestSelectQuestions_EmptyInput(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	if got := SelectQuestions(nil, nil, 5, rng); len(got) != 0 {
		t.Errorf("expected empty result for nil questions, got %d", len(got))
	}
	if got := SelectQuestions([]domain.Question{makeQuestion(1)}, nil, 0, rng); len(got) != 0 {
		t.Errorf("expected empty result for n=0, got %d", len(got))
	}
}
