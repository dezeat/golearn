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
	case screenProfileMenu:
		return m.updateProfileMenu(msg)
	case screenProfileLogin:
		return m.updateProfileLogin(msg)
	case screenProfileRegister:
		return m.updateProfileRegister(msg)
	case screenTopicSelect:
		return m.updateTopicSelect(msg)
	case screenSessionConfig:
		return m.updateSessionConfig(msg)
	case screenQuestion:
		return m.updateQuestion(msg)
	case screenReview:
		return m.updateReview(msg)
	case screenReviewBrowse:
		return m.updateReviewBrowse(msg)
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
	case screenProfileMenu:
		return m.viewProfileMenu()
	case screenProfileLogin:
		return m.viewProfileLogin()
	case screenProfileRegister:
		return m.viewProfileRegister()
	case screenTopicSelect:
		return m.viewTopicSelect()
	case screenSessionConfig:
		return m.viewSessionConfig()
	case screenQuestion:
		return m.viewQuestion()
	case screenReview:
		return m.viewReview()
	case screenReviewBrowse:
		return m.viewReviewBrowse()
	case screenSummary:
		return m.viewSummary()
	}

	return ""
}

// --- Profile Menu ---

func (m model) updateProfileMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.profileMenuCursor > 0 {
				m.profileMenuCursor--
			}
		case "down", "j":
			if m.profileMenuCursor < len(m.profileMenuOptions())-1 {
				m.profileMenuCursor++
			}
		case "enter":
			options := m.profileMenuOptions()
			if len(options) == 0 {
				return m, nil
			}

			selected := options[m.profileMenuCursor]
			switch {
			case selected == "Login":
				profiles, err := m.userRepo.List()
				if err != nil {
					m.profileError = "Failed to load profiles"
					return m, nil
				}
				m.profiles = profiles
				m.profileLoginCursor = 0
				m.profileError = ""
				m.screen = screenProfileLogin
			case selected == "Register":
				m.registerHandle = ""
				m.registerDisplayName = ""
				m.registerField = 0
				m.profileError = ""
				m.screen = screenProfileRegister
			case selected == "Quit":
				m.quitting = true
				return m, tea.Quit
			default: // Continue
				if m.currentUser == nil {
					m.profileError = "No active profile"
					return m, nil
				}
				if err := m.reloadTopicsForCurrentUser(); err != nil {
					m.profileError = "Failed to load topics"
					return m, nil
				}
				m.screen = screenTopicSelect
			}
		}
	}
	return m, nil
}

func (m model) updateProfileLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.screen = screenProfileMenu
		case "up", "k":
			if m.profileLoginCursor > 0 {
				m.profileLoginCursor--
			}
		case "down", "j":
			if m.profileLoginCursor < len(m.profiles)-1 {
				m.profileLoginCursor++
			}
		case "enter":
			if len(m.profiles) == 0 {
				m.profileError = "No profiles available"
				return m, nil
			}
			selected := m.profiles[m.profileLoginCursor]
			if err := m.setCurrentUser(&selected, true); err != nil {
				m.profileError = "Failed to save profile"
				return m, nil
			}
			if err := m.reloadTopicsForCurrentUser(); err != nil {
				m.profileError = "Failed to load topics"
				return m, nil
			}
			m.profileError = ""
			m.screen = screenTopicSelect
		}
	}
	return m, nil
}

func (m model) updateProfileRegister(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.screen = screenProfileMenu
			return m, nil
		case "tab":
			m.registerField = (m.registerField + 1) % 2
			return m, nil
		case "backspace":
			if m.registerField == 0 {
				if len(m.registerHandle) > 0 {
					m.registerHandle = m.registerHandle[:len(m.registerHandle)-1]
				}
			} else if len(m.registerDisplayName) > 0 {
				m.registerDisplayName = m.registerDisplayName[:len(m.registerDisplayName)-1]
			}
			return m, nil
		case "enter":
			m.profileError = ""
			if m.registerField == 0 {
				handle := m.registerHandle
				if !isValidHandle(handle) {
					m.profileError = "Handle must use: a-z 0-9 - _"
					return m, nil
				}
				existing, found, err := m.userRepo.GetByHandle(handle)
				if err != nil {
					m.profileError = "Failed to check handle"
					return m, nil
				}
				if found && existing != nil {
					m.profileError = "Handle already exists"
					return m, nil
				}
				m.registerField = 1
				return m, nil
			}

			u, err := m.userRepo.Create(m.registerHandle, m.registerDisplayName)
			if err != nil {
				m.profileError = "Failed to create profile"
				return m, nil
			}
			if err := m.setCurrentUser(u, true); err != nil {
				m.profileError = "Failed to save profile"
				return m, nil
			}
			profiles, err := m.userRepo.List()
			if err == nil {
				m.profiles = profiles
			}
			if err := m.reloadTopicsForCurrentUser(); err != nil {
				m.profileError = "Failed to load topics"
				return m, nil
			}
			m.screen = screenTopicSelect
			return m, nil
		}

		if len(msg.Runes) == 0 {
			return m, nil
		}
		r := msg.Runes[0]
		if m.registerField == 0 {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				m.registerHandle += string(r)
			}
		} else if r >= 32 && r <= 126 {
			m.registerDisplayName += string(r)
		}
	}
	return m, nil
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
		case "right", "k":
			if m.questionCount < m.selectedTopic.QuestionCount {
				m.questionCount++
			}
		case "left", "j":
			if m.questionCount > 1 {
				m.questionCount--
			}
		case "enter":
			// Start the session via the engine.
			engine := app.NewSessionEngine(
				m.topicRepo, m.questionRepo,
				m.sessionRepo, m.attemptRepo,
				m.userCtx,
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
			m.totalLatency = 0
			m.wrongAnswers = nil

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
				m.showExplanations = false
				m.screen = screenReview
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
				m.lastCorrect = correct

				m.answered++
				m.totalLatency += latencyMs
				m.lastSkipped = false
				m.submitted = true
				if m.lastCorrect {
					m.correctCount++
				} else {
					// Track wrong answers for review browse.
					m.wrongAnswers = append(m.wrongAnswers, wrongAnswer{
						Question:    *m.currentQuestion,
						SelectedIDs: selectedIDs,
					})
				}
				m.showExplanations = false
				m.screen = screenReview
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
	m.showExplanations = false
	m.screen = screenQuestion
	questionStartedAt = time.Now()
}

func (m model) questionStarted() time.Time {
	return questionStartedAt
}

// --- Review (quiz-show feedback) ---

func (m model) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			// Toggle explanations.
			m.showExplanations = !m.showExplanations
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

// --- Review Browse (read-only review of wrong answers) ---

func (m model) updateReviewBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "n", "right":
			if m.reviewCursor < len(m.reviewQueue)-1 {
				m.reviewCursor++
				m.showExplanations = false
			} else {
				// End of review, back to summary.
				m.screen = screenSummary
			}
		case "p", "left":
			if m.reviewCursor > 0 {
				m.reviewCursor--
				m.showExplanations = false
			}
		case "e":
			m.showExplanations = !m.showExplanations
		case "q", "esc":
			m.screen = screenTopicSelect
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
			_ = m.reloadTopicsForCurrentUser()
			m.screen = screenTopicSelect
		case "r":
			// Review mode: browse wrong answers (read-only).
			if len(m.wrongAnswers) > 0 {
				m.startReviewSession()
			}
		}
	}
	return m, nil
}

// contentWidth returns the available character width for content,
// subtracting padding from the terminal width. Falls back to 80
// columns when no WindowSizeMsg has been received yet.
func (m model) contentWidth(padding int) int {
	w := m.width
	if w == 0 {
		w = 80
	}
	cw := w - padding
	if cw < 20 {
		cw = 20
	}
	return cw
}
