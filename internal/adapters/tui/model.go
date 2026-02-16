package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dezeat/golearn/internal/app"
)

// Init implements tea.Model. It returns no initial command.
func (m model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It routes messages to the appropriate
// screen handler based on the current screen state.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit: ctrl+c always exits.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	switch m.screen {
	case screenTopicSelect:
		return m.updateTopicSelect(msg)
	case screenSessionConfig:
		return m.updateSessionConfig(msg)
	case screenQuestion:
		return m.updateQuestion(msg)
	case screenFeedback:
		return m.updateFeedback(msg)
	case screenSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

// View implements tea.Model.
func (m model) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case screenTopicSelect:
		return m.viewTopicSelect()
	case screenSessionConfig:
		return m.viewSessionConfig()
	case screenQuestion:
		return m.viewQuestion()
	case screenFeedback:
		return m.viewFeedback()
	case screenSummary:
		return m.viewSummary()
	}

	return ""
}

// --- Topic Select ---

func (m model) updateTopicSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.topicCursor > 0 {
				m.topicCursor--
			}
		case "down", "j":
			if m.topicCursor < len(m.topics)-1 {
				m.topicCursor++
			}
		case "enter":
			if len(m.topics) > 0 {
				m.selectedTopic = m.topics[m.topicCursor]
				// Cap default question count to available questions.
				if m.questionCount > m.selectedTopic.QuestionCount {
					m.questionCount = m.selectedTopic.QuestionCount
				}
				m.screen = screenSessionConfig
			}
		}
	}
	return m, nil
}

// --- Session Config ---

func (m model) updateSessionConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.screen = screenTopicSelect
		case "up", "k":
			if m.questionCount < m.selectedTopic.QuestionCount {
				m.questionCount++
			}
		case "down", "j":
			if m.questionCount > 1 {
				m.questionCount--
			}
		case "enter":
			// Start the session via the engine.
			engine := app.NewSessionEngine(
				m.topicRepo, m.questionRepo,
				m.sessionRepo, m.attemptRepo,
				nil, // time-seeded rng
			)
			_, err := engine.StartSession(
				m.selectedTopic.Topic.Slug,
				m.questionCount,
				"practice",
			)
			if err != nil {
				// On error, go back to topic select.
				m.screen = screenTopicSelect
				return m, nil
			}

			m.engine = engine
			m.totalQuestions = engine.QueueLength()
			m.questionNum = 0
			m.answered = 0
			m.correctCount = 0

			// Advance to the first question.
			m.advanceQuestion()
		}
	}
	return m, nil
}

// --- Question ---

func (m model) updateQuestion(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			// Quit session early → go to summary.
			_ = m.engine.EndSession()
			m.screen = screenSummary
		case "up", "k":
			if m.choiceCursor > 0 {
				m.choiceCursor--
			}
		case "down", "j":
			if m.currentQuestion != nil && m.choiceCursor < len(m.currentQuestion.Choices)-1 {
				m.choiceCursor++
			}
		case " ":
			// Toggle selection (for multi_select, or single_select).
			if m.currentQuestion != nil {
				choiceID := m.currentQuestion.Choices[m.choiceCursor].ID
				if m.currentQuestion.Type == "single_select" {
					// Single select: clear others, select this one.
					m.selected = map[string]bool{choiceID: true}
				} else {
					// Multi select: toggle.
					if m.selected[choiceID] {
						delete(m.selected, choiceID)
					} else {
						m.selected[choiceID] = true
					}
				}
			}
		case "s":
			// Skip this question.
			if m.currentQuestion != nil {
				_, _ = m.engine.RecordAttempt(m.currentQuestion.ID, nil, true, 0)
				m.answered++
				m.lastSkipped = true
				m.lastCorrect = false
				m.submitted = true
				m.screen = screenFeedback
			}
		case "enter":
			if m.currentQuestion != nil && len(m.selected) > 0 {
				// Submit answer.
				selectedIDs := make([]string, 0, len(m.selected))
				for id := range m.selected {
					selectedIDs = append(selectedIDs, id)
				}

				latencyMs := int(time.Since(m.questionStarted()).Milliseconds())
				correct, _ := m.engine.RecordAttempt(
					m.currentQuestion.ID, selectedIDs, false, latencyMs,
				)

				m.answered++
				m.lastCorrect = correct
				m.lastSkipped = false
				m.submitted = true
				if correct {
					m.correctCount++
				}
				m.screen = screenFeedback
			}
		}
	}
	return m, nil
}

// questionStartedAt is used for latency tracking; we use a simple approach.
var questionStartedAt time.Time

func (m *model) advanceQuestion() {
	q := m.engine.GetNextQuestion()
	if q == nil {
		_ = m.engine.EndSession()
		m.screen = screenSummary
		return
	}
	m.currentQuestion = q
	m.questionNum++
	m.choiceCursor = 0
	m.selected = make(map[string]bool)
	m.submitted = false
	m.screen = screenQuestion
	questionStartedAt = time.Now()
}

func (m model) questionStarted() time.Time {
	return questionStartedAt
}

// --- Feedback ---

func (m model) updateFeedback(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			// Advance to next question.
			m.advanceQuestion()
		case "q":
			_ = m.engine.EndSession()
			m.screen = screenSummary
		}
	}
	return m, nil
}

// --- Summary ---

func (m model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter", "b":
			// Back to topic select.
			m.screen = screenTopicSelect
		}
	}
	return m, nil
}
