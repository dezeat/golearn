// Package tui implements the Bubble Tea terminal UI for golearn.
//
// The TUI provides an interactive experience for practising MCQs:
//   - ASCII intro splash screen
//   - Topic selection screen
//   - Session configuration screen
//   - Question answering screen with quiz-show feedback
//   - Session summary screen with review option
package tui

import (
	"database/sql"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// Styles used across TUI screens.
var (
	styleCorrect   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	styleIncorrect = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	styleSelected  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue
	styleExplain   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // dim
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")) // cyan
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Run launches the TUI application. It owns the DB connection and
// creates all necessary repos and services.
func Run(db *sql.DB) error {
	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)

	m := newModel(topicRepo, questionRepo, sessionRepo, attemptRepo)

	// Load topics at startup.
	topics, err := topicRepo.List()
	if err != nil {
		return fmt.Errorf("load topics: %w", err)
	}

	// Load question counts per topic for the topic list.
	topicInfos := make([]topicInfo, 0, len(topics))
	for _, t := range topics {
		qs, err := questionRepo.ListByTopic(t.ID)
		if err != nil {
			return fmt.Errorf("list questions for topic %q: %w", t.Slug, err)
		}
		stats, err := attemptRepo.StatsByTopic(t.ID)
		if err != nil {
			return fmt.Errorf("load stats for topic %q: %w", t.Slug, err)
		}
		totalAttempts := 0
		totalCorrect := 0
		for _, s := range stats {
			totalAttempts += s.Attempts
			totalCorrect += s.Attempts - s.Wrong
		}
		ti := topicInfo{
			Topic:         t,
			QuestionCount: len(qs),
			TotalAttempts: totalAttempts,
			TotalCorrect:  totalCorrect,
		}
		topicInfos = append(topicInfos, ti)
	}

	m.topics = topicInfos

	if len(topicInfos) == 0 {
		return fmt.Errorf("no topics found — import a question pack first: golearn import <path>")
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// topicInfo extends a Topic with display metadata.
type topicInfo struct {
	Topic         domain.Topic
	QuestionCount int
	TotalAttempts int
	TotalCorrect  int
}

// screen represents which screen is currently active.
type screen int

const (
	screenIntro         screen = iota // ASCII splash screen
	screenTopicSelect                 // topic selection
	screenSessionConfig               // session configuration
	screenQuestion                    // answering a question
	screenReview                      // quiz-show review mode (replaces screenFeedback)
	screenSummary                     // session summary
)

// model is the root Bubble Tea model that holds all TUI state.
type model struct {
	// Dependencies
	topicRepo    ports.TopicRepository
	questionRepo ports.QuestionRepository
	sessionRepo  ports.SessionRepository
	attemptRepo  ports.AttemptRepository

	// Current screen
	screen screen

	// Topic select state
	topics      []topicInfo
	topicCursor int

	// Session config state
	selectedTopic topicInfo
	questionCount int // number of questions for the session

	// Session engine
	engine *app.SessionEngine

	// Question screen state
	currentQuestion *domain.Question
	questionNum     int // 1-based index
	totalQuestions  int
	choiceCursor    int             // which choice is highlighted
	selected        map[string]bool // toggled choices for multi_select
	submitted       bool            // whether answer has been submitted
	lastCorrect     bool            // result of last submission
	lastSkipped     bool

	// Review mode state
	showExplanations bool // toggled by 'e' in review mode

	// Summary state
	answered     int
	correctCount int
	totalLatency int // cumulative latency in milliseconds

	// Track wrong questions for review
	wrongQuestions []domain.Question

	// Review session mode: only replay wrong questions
	reviewMode   bool
	reviewQueue  []domain.Question
	reviewCursor int

	// Window size
	width  int
	height int

	// Quit flag
	quitting bool
}

func newModel(
	topicRepo ports.TopicRepository,
	questionRepo ports.QuestionRepository,
	sessionRepo ports.SessionRepository,
	attemptRepo ports.AttemptRepository,
) model {
	return model{
		topicRepo:     topicRepo,
		questionRepo:  questionRepo,
		sessionRepo:   sessionRepo,
		attemptRepo:   attemptRepo,
		screen:        screenIntro,
		questionCount: 10,
		selected:      make(map[string]bool),
	}
}
