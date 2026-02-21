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

package tui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dezeat/golearn/internal/adapters/localconfig"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// TestReviewState_ExplanationToggle verifies that the showExplanations
// flag toggles correctly in review mode.
func TestReviewState_ExplanationToggle(t *testing.T) {
	m := model{
		screen:    screenReview,
		submitted: true,
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Prompt:           "Test question",
				CorrectChoiceIDs: []string{"A"},
				Rationale: domain.Rationale{
					Correct: "A is correct because...",
					PerChoice: map[string]string{
						"A": "Correct. A is the right answer.",
						"B": "Incorrect. B is wrong because...",
					},
				},
			},
			ShuffledChoices: []domain.Choice{
				{ID: "A", Text: "Option A"},
				{ID: "B", Text: "Option B"},
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
	m.setDisplayLabelMapping(m.currentQuestion.ShuffledChoices)
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

func TestQuestionView_UsesDisplayLabelsFromShuffledOrder(t *testing.T) {
	m := model{
		screen:         screenQuestion,
		questionNum:    1,
		totalQuestions: 1,
		choiceCursor:   0,
		width:          80,
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Type:   domain.SingleSelect,
				Prompt: "Pick one",
			},
			ShuffledChoices: []domain.Choice{
				{ID: "C", Text: "First shown"},
				{ID: "A", Text: "Second shown"},
				{ID: "B", Text: "Third shown"},
			},
		},
		selected: map[string]bool{},
	}

	m.setDisplayLabelMapping(m.currentQuestion.ShuffledChoices)
	view := m.viewQuestion()

	if !containsSubstring(view, "A) First shown") {
		t.Fatalf("expected first rendered choice to use display label A; got:\n%s", view)
	}
	if !containsSubstring(view, "B) Second shown") {
		t.Fatalf("expected second rendered choice to use display label B; got:\n%s", view)
	}
	if !containsSubstring(view, "C) Third shown") {
		t.Fatalf("expected third rendered choice to use display label C; got:\n%s", view)
	}
	if containsSubstring(view, "C) First shown") {
		t.Fatalf("did not expect internal ID label rendering after shuffle; got:\n%s", view)
	}
}

func TestReviewView_ShuffledLabelsUseInternalExplanationLookup(t *testing.T) {
	m := model{
		screen:           screenReview,
		submitted:        true,
		showExplanations: true,
		width:            80,
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Prompt:           "Pick one",
				CorrectChoiceIDs: []string{"A"},
				Rationale: domain.Rationale{
					Correct: "because of root cause",
					PerChoice: map[string]string{
						"C": "rationale for C",
						"A": "rationale for A",
						"B": "rationale for B",
					},
				},
			},
			ShuffledChoices: []domain.Choice{
				{ID: "C", Text: "First shown"},
				{ID: "A", Text: "Second shown"},
				{ID: "B", Text: "Third shown"},
			},
		},
		selected: map[string]bool{"A": true},
	}

	m.setDisplayLabelMapping(m.currentQuestion.ShuffledChoices)
	view := m.viewReview()

	if !containsSubstring(view, "A) First shown") {
		t.Fatalf("expected display labels to follow shuffled order; got:\n%s", view)
	}
	if !containsSubstring(view, "rationale for C") {
		t.Fatalf("expected explanation lookup by internal choice ID after shuffle; got:\n%s", view)
	}
	// FormatChoiceExplanation applies Correct:/Incorrect: dynamically.
	if !containsSubstring(view, "Incorrect: rationale for C") {
		t.Fatalf("expected Incorrect: prefix for wrong choice; got:\n%s", view)
	}
	if !containsSubstring(view, "Correct: rationale for A") {
		t.Fatalf("expected Correct: prefix for right choice; got:\n%s", view)
	}
}

// TestReviewState_CorrectFeedback verifies correct answer visual feedback.
func TestReviewState_CorrectFeedback(t *testing.T) {
	m := model{
		screen:      screenReview,
		submitted:   true,
		lastCorrect: true,
		lastSkipped: false,
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Prompt:           "Test correct",
				CorrectChoiceIDs: []string{"A"},
			},
			ShuffledChoices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
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
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Prompt:           "Test incorrect",
				CorrectChoiceIDs: []string{"A"},
			},
			ShuffledChoices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
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
		currentQuestion: &app.SessionQuestion{
			Question: &domain.Question{
				Prompt:           "Test skipped",
				CorrectChoiceIDs: []string{"A"},
			},
			ShuffledChoices: []domain.Choice{
				{ID: "A", Text: "Right"},
				{ID: "B", Text: "Wrong"},
			},
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
	m := model{screen: screenProfileMenu, hasValidCurrentUser: true, currentUser: &domain.User{Handle: "local", DisplayName: "Local"}}
	view := m.viewProfileMenu()
	if view == "" {
		t.Fatal("viewProfileMenu returned empty string")
	}
	if !containsSubstring(view, "adaptive certification practice") {
		t.Error("expected tagline in intro screen")
	}
	if !containsSubstring(view, "Continue") {
		t.Error("expected Continue option in profile menu")
	}
}

func TestProfileLoginSetsCurrentUserID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tui.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)
	userRepo := sqlite.NewUserRepo(db)
	configStore := localconfig.NewStore(filepath.Join(t.TempDir(), "config.json"))

	alice, err := userRepo.Create("alice", "Alice")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}

	ctx := app.NewUserContext(0)
	statsRepo := sqlite.NewStatsRepo(db)
	m := newModel(topicRepo, questionRepo, sessionRepo, attemptRepo, userRepo, statsRepo, configStore, ctx)
	m.screen = screenProfileLogin
	m.profiles = []domain.User{*alice}
	m.profileLoginCursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm := updated.(model)

	if ctx.CurrentUserID() != alice.ID {
		t.Fatalf("expected current user id %d, got %d", alice.ID, ctx.CurrentUserID())
	}
	if nm.screen != screenHomeMenu {
		t.Fatalf("expected transition to home menu, got screen %d", nm.screen)
	}
}

// TestReviewSessionSetup verifies startReviewSession initializes correctly.
func TestReviewSessionSetup(t *testing.T) {
	wrongAs := []wrongAnswer{
		{Question: domain.Question{ID: 1, Prompt: "Q1"}, SelectedIDs: []string{"B"}},
		{Question: domain.Question{ID: 2, Prompt: "Q2"}, SelectedIDs: []string{"A"}},
	}
	m := model{
		wrongAnswers: wrongAs,
		selected:     make(map[string]bool),
	}

	m.startReviewSession(screenSummary)

	if m.screen != screenReviewBrowse {
		t.Errorf("expected screenReviewBrowse, got %d", m.screen)
	}
	if len(m.reviewQueue) != 2 {
		t.Errorf("expected 2 review items, got %d", len(m.reviewQueue))
	}
	if m.reviewCursor != 0 {
		t.Errorf("expected reviewCursor 0, got %d", m.reviewCursor)
	}
	if m.reviewReturnScreen != screenSummary {
		t.Errorf("expected reviewReturnScreen summary, got %d", m.reviewReturnScreen)
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
	if !containsSubstring(view, "Toggle Explanation") {
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

// --- Home Menu tests ---

func TestHomeMenuView(t *testing.T) {
	m := model{
		screen:      screenHomeMenu,
		currentUser: &domain.User{Handle: "alice", DisplayName: "Alice"},
		width:       80,
	}

	view := m.viewHomeMenu()
	if !containsSubstring(view, "Home") {
		t.Error("expected Home in header")
	}
	if !containsSubstring(view, "Start Practice") {
		t.Error("expected Start Practice option")
	}
	if !containsSubstring(view, "Stats") {
		t.Error("expected Stats option")
	}
}

func TestHomeMenuNavigation(t *testing.T) {
	m := model{
		screen:         screenHomeMenu,
		currentUser:    &domain.User{Handle: "test"},
		homeMenuCursor: 0,
		selected:       make(map[string]bool),
	}

	// Down key should move cursor.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	nm := updated.(model)
	if nm.homeMenuCursor != 1 {
		t.Errorf("cursor after down: got %d, want 1", nm.homeMenuCursor)
	}
}

// --- Stats Sparkline tests ---

func TestSparkline(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{"empty", nil, "—"},
		{"all zeros", []float64{0, 0, 0}, "▁▁▁"},
		{"all 100", []float64{100, 100, 100}, "███"},
		{"ascending", []float64{0, 50, 100}, "▁▄█"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sparkline(tt.values)
			if got != tt.want {
				t.Errorf("sparkline(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestTrendDelta(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   string
	}{
		{"single", []float64{50}, ""},
		{"increase", []float64{40, 80}, " ↑40%"},
		{"decrease", []float64{80, 60}, " ↓20%"},
		{"same", []float64{50, 50}, " →0%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trendDelta(tt.values)
			if got != tt.want {
				t.Errorf("trendDelta(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello world", 5) != "hell…" {
		t.Errorf("expected 'hell…', got %q", truncate("hello world", 5))
	}
	if truncate("hi", 5) != "hi" {
		t.Errorf("expected 'hi', got %q", truncate("hi", 5))
	}
}

func TestFormatStatsTimestamp(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "sqlite format", raw: "2026-02-17 14:33:59", want: "2026-02-17 14:33"},
		{name: "rfc3339", raw: "2026-02-17T14:33:59Z", want: "2026-02-17 14:33"},
		{name: "already minute precision", raw: "2026-02-17 14:33", want: "2026-02-17 14:33"},
		{name: "fallback truncate", raw: "2026-02-17 14:33:59.123 whatever", want: "2026-02-17 14:33"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatsTimestamp(tt.raw)
			if got != tt.want {
				t.Fatalf("formatStatsTimestamp(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// --- Global Stats View test ---

func TestStatsGlobalViewEmpty(t *testing.T) {
	m := model{
		screen:      screenStatsGlobal,
		currentUser: &domain.User{Handle: "test"},
		statsGlobal: nil,
		width:       80,
	}
	view := m.viewStatsGlobal()
	if !containsSubstring(view, "No stats data") {
		t.Error("expected empty state message for global stats")
	}
}

// --- Summary Screen Updated ---

func TestSummaryViewHasStatsOption(t *testing.T) {
	m := model{
		screen: screenSummary,
		width:  80,
		selectedTopic: topicInfo{
			Topic: domain.Topic{Name: "Test Topic"},
		},
		answered:      5,
		correctCount:  3,
		totalLatency:  5000,
		wrongAnswers:  nil,
		summaryCursor: 0,
	}

	view := m.viewSummary()
	if !containsSubstring(view, "View stats for this pack") {
		t.Error("expected stats option in summary view")
	}
}

func TestSortPacksByAttempts(t *testing.T) {
	packs := []ports.TopicSummary{
		{TopicName: "Zero", AttemptsAnswered: 0},
		{TopicName: "Medium", AttemptsAnswered: 10},
		{TopicName: "High", AttemptsAnswered: 50},
		{TopicName: "Low", AttemptsAnswered: 3},
	}

	sortPacksByAttempts(packs)

	expected := []struct {
		name     string
		attempts int
	}{
		{"High", 50},
		{"Medium", 10},
		{"Low", 3},
		{"Zero", 0},
	}

	for i, want := range expected {
		if packs[i].TopicName != want.name {
			t.Errorf("packs[%d]: got name %q, want %q", i, packs[i].TopicName, want.name)
		}
		if packs[i].AttemptsAnswered != want.attempts {
			t.Errorf("packs[%d]: got attempts %d, want %d", i, packs[i].AttemptsAnswered, want.attempts)
		}
	}
}

func TestSortPacksByAttempts_Empty(t *testing.T) {
	var packs []ports.TopicSummary
	sortPacksByAttempts(packs) // should not panic
	if len(packs) != 0 {
		t.Errorf("expected empty, got %d", len(packs))
	}
}

func TestSortPacksByAttempts_AlreadySorted(t *testing.T) {
	packs := []ports.TopicSummary{
		{TopicName: "A", AttemptsAnswered: 30},
		{TopicName: "B", AttemptsAnswered: 20},
		{TopicName: "C", AttemptsAnswered: 10},
	}

	sortPacksByAttempts(packs)

	if packs[0].TopicName != "A" || packs[1].TopicName != "B" || packs[2].TopicName != "C" {
		t.Errorf("already sorted list should remain unchanged")
	}
}
