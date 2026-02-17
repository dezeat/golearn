package tui

import (
	"testing"

	"github.com/dezeat/golearn/internal/domain"
)

// TestReviewState_ExplanationToggle verifies that the showExplanations
// flag toggles correctly in review mode.
func TestReviewState_ExplanationToggle(t *testing.T) {
	m := model{
		screen:    screenReview,
		submitted: true,
		currentQuestion: &domain.Question{
			Prompt: "Test question",
			Choices: []domain.Choice{
				{ID: "A", Text: "Option A"},
				{ID: "B", Text: "Option B"},
			},
			CorrectChoiceIDs: []string{"A"},
			Rationale: domain.Rationale{
				Correct: "A is correct because...",
				PerChoice: map[string]string{
					"A": "Correct. A is the right answer.",
					"B": "Incorrect. B is wrong because...",
				},
			},
		},
		selected:         map[string]bool{"A": true},
		showExplanations: false,
	}

	// Initially explanations are hidden.
	if m.showExplanations {
		t.Error("expected showExplanations to be false initially")
	}

	// Verify view renders without explanations.
	view := m.viewReview()
	if view == "" {
		t.Fatal("viewReview returned empty string")
	}

	// Toggle explanations on.
	m.showExplanations = true
	viewWithExplanations := m.viewReview()
	if viewWithExplanations == view {
		t.Error("expected view to change when explanations are toggled on")
	}

	// Verify explanations content appears.
	if !containsSubstring(viewWithExplanations, "A is correct because") {
		t.Error("expected rationale.correct text in view with explanations")
	}
	if !containsSubstring(viewWithExplanations, "Correct. A is the right answer.") {
		t.Error("expected per_choice explanation for A in view")
	}

	// Toggle back off.
	m.showExplanations = false
	viewOff := m.viewReview()
	if containsSubstring(viewOff, "A is correct because") {
		t.Error("expected rationale.correct text to be hidden when explanations off")
	}
}

// TestReviewState_CorrectFeedback verifies correct answer visual feedback.
func TestReviewState_CorrectFeedback(t *testing.T) {
	m := model{
		screen:      screenReview,
		submitted:   true,
		lastCorrect: true,
		lastSkipped: false,
		currentQuestion: &domain.Question{
			Prompt: "Test correct",
			Choices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
			CorrectChoiceIDs: []string{"A"},
		},
		selected: map[string]bool{"A": true},
	}

	view := m.viewReview()
	if !containsSubstring(view, "Correct") {
		t.Error("expected 'Correct' in review view for correct answer")
	}
}

// TestReviewState_IncorrectFeedback verifies incorrect answer visual feedback.
func TestReviewState_IncorrectFeedback(t *testing.T) {
	m := model{
		screen:      screenReview,
		submitted:   true,
		lastCorrect: false,
		lastSkipped: false,
		currentQuestion: &domain.Question{
			Prompt: "Test incorrect",
			Choices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
			CorrectChoiceIDs: []string{"A"},
		},
		selected: map[string]bool{"B": true},
	}

	view := m.viewReview()
	if !containsSubstring(view, "Incorrect") {
		t.Error("expected 'Incorrect' in review view for wrong answer")
	}
}

// TestReviewState_SkippedFeedback verifies skipped answer visual feedback.
func TestReviewState_SkippedFeedback(t *testing.T) {
	m := model{
		screen:      screenReview,
		submitted:   true,
		lastCorrect: false,
		lastSkipped: true,
		currentQuestion: &domain.Question{
			Prompt: "Test skipped",
			Choices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
			CorrectChoiceIDs: []string{"A"},
		},
		selected: map[string]bool{},
	}

	view := m.viewReview()
	if !containsSubstring(view, "Skipped") {
		t.Error("expected 'Skipped' in review view for skipped answer")
	}
}

// TestIntroScreen verifies the ASCII intro renders.
func TestIntroScreen(t *testing.T) {
	m := model{screen: screenIntro}
	view := m.viewIntro()
	if view == "" {
		t.Fatal("viewIntro returned empty string")
	}
	// The ASCII art contains "GO" as part of the block letters.
	if !containsSubstring(view, "adaptive certification practice") {
		t.Error("expected tagline in intro screen")
	}
	if !containsSubstring(view, "Press any key") {
		t.Error("expected 'Press any key' prompt in intro screen")
	}
}

// TestReviewSessionSetup verifies startReviewSession initialises correctly.
func TestReviewSessionSetup(t *testing.T) {
	wrongQs := []domain.Question{
		{ID: 1, Prompt: "Q1"},
		{ID: 2, Prompt: "Q2"},
	}
	m := model{
		wrongQuestions: wrongQs,
		selected:       make(map[string]bool),
	}

	m.startReviewSession()

	if !m.reviewMode {
		t.Error("expected reviewMode to be true")
	}
	if m.totalQuestions != 2 {
		t.Errorf("expected 2 total questions, got %d", m.totalQuestions)
	}
	if m.questionNum != 1 {
		t.Errorf("expected questionNum 1, got %d", m.questionNum)
	}
	if m.screen != screenQuestion {
		t.Errorf("expected screenQuestion, got %d", m.screen)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
