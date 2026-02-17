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
	wrongAs := []wrongAnswer{
		{Question: domain.Question{ID: 1, Prompt: "Q1"}, SelectedIDs: []string{"B"}},
		{Question: domain.Question{ID: 2, Prompt: "Q2"}, SelectedIDs: []string{"A"}},
	}
	m := model{
		wrongAnswers: wrongAs,
		selected:     make(map[string]bool),
	}

	m.startReviewSession()

	if m.screen != screenReviewBrowse {
		t.Errorf("expected screenReviewBrowse, got %d", m.screen)
	}
	if len(m.reviewQueue) != 2 {
		t.Errorf("expected 2 review items, got %d", len(m.reviewQueue))
	}
	if m.reviewCursor != 0 {
		t.Errorf("expected reviewCursor 0, got %d", m.reviewCursor)
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

// --- Wrap helper tests ---

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"no wrap needed", "hello world", 80, "hello world"},
		{"exact width", "hello world", 11, "hello world"},
		{"wraps at word boundary", "hello world foo bar", 11, "hello world\nfoo bar"},
		{"multiple wraps", "ab cd ef gh", 5, "ab cd\nef gh"},
		{"preserves newlines", "hello\nworld", 80, "hello\nworld"},
		{"breaks long word", "abcdefghij", 5, "abcde\nfghij"},
		{"zero width passthrough", "hello", 0, "hello"},
		{"negative width passthrough", "hello", -1, "hello"},
		{"empty string", "", 80, ""},
		{"preserves multi newlines", "a\n\nb", 80, "a\n\nb"},
		{"single word fits", "hello", 5, "hello"},
		{"single word too long", "hello", 3, "hel\nlo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}

func TestWrapText_NoLineExceedsWidth(t *testing.T) {
	input := "This is a fairly long sentence that should be wrapped nicely at word boundaries without exceeding the specified width"
	width := 30
	result := wrapText(input, width)
	for i, line := range splitLines(result) {
		if len(line) > width {
			t.Errorf("line %d length %d exceeds width %d: %q", i, len(line), width, line)
		}
	}
}

func TestWrapAndIndent(t *testing.T) {
	result := wrapAndIndent("hello world foo bar", 11, "  ")
	want := "  hello world\n  foo bar"
	if result != want {
		t.Errorf("wrapAndIndent got %q, want %q", result, want)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

// --- Review Browse tests ---

func TestReviewBrowseView(t *testing.T) {
	m := model{
		screen: screenReviewBrowse,
		width:  80,
		reviewQueue: []wrongAnswer{
			{
				Question: domain.Question{
					Prompt: "What is the answer?",
					Choices: []domain.Choice{
						{ID: "A", Text: "Right"},
						{ID: "B", Text: "Wrong"},
					},
					CorrectChoiceIDs: []string{"A"},
					Rationale: domain.Rationale{
						Correct: "A is correct because...",
					},
				},
				SelectedIDs: []string{"B"},
			},
		},
		reviewCursor:     0,
		showExplanations: false,
	}

	view := m.viewReviewBrowse()
	if !containsSubstring(view, "Review (1/1)") {
		t.Error("expected review header with count")
	}
	if !containsSubstring(view, "What is the answer?") {
		t.Error("expected prompt in review browse")
	}
	if !containsSubstring(view, "enter next") {
		t.Error("expected browse controls")
	}
}

func TestContentWidth(t *testing.T) {
	m := model{width: 80}
	if cw := m.contentWidth(2); cw != 78 {
		t.Errorf("expected 78, got %d", cw)
	}

	m.width = 0 // no WindowSizeMsg yet
	if cw := m.contentWidth(2); cw != 78 {
		t.Errorf("expected 78 (default 80-2), got %d", cw)
	}

	m.width = 25
	if cw := m.contentWidth(10); cw != 20 {
		t.Errorf("expected 20 (min), got %d", cw)
	}
}
