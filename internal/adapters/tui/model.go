package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
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
	case screenHomeMenu:
		return m.updateHomeMenu(msg)
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
	case screenStatsMenu:
		return m.updateStatsMenu(msg)
	case screenStatsGlobal:
		return m.updateStatsGlobal(msg)
	case screenStatsPackList:
		return m.updateStatsPackList(msg)
	case screenStatsPackDetail:
		return m.updateStatsPackDetail(msg)
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
	case screenHomeMenu:
		return m.viewHomeMenu()
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
	case screenStatsMenu:
		return m.viewStatsMenu()
	case screenStatsGlobal:
		return m.viewStatsGlobal()
	case screenStatsPackList:
		return m.viewStatsPackList()
	case screenStatsPackDetail:
		return m.viewStatsPackDetail()
	}

	return ""
}

// --- Profile Menu ---

func (m model) updateProfileMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isQuitKey(key):
			m.quitting = true
			return m, tea.Quit
		case isUpNav(key):
			if m.profileMenuCursor > 0 {
				m.profileMenuCursor--
			}
		case isDownNav(key):
			if m.profileMenuCursor < len(m.profileMenuOptions())-1 {
				m.profileMenuCursor++
			}
		case isEnterKey(key):
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
				m.homeMenuCursor = 0
				m.screen = screenHomeMenu
			}
		}
	}
	return m, nil
}

func (m model) updateProfileLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			m.screen = screenProfileMenu
		case isUpNav(key):
			if m.profileLoginCursor > 0 {
				m.profileLoginCursor--
			}
		case isDownNav(key):
			if m.profileLoginCursor < len(m.profiles)-1 {
				m.profileLoginCursor++
			}
		case isEnterKey(key):
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
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		}
	}
	return m, nil
}

func (m model) updateProfileRegister(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case keyBack:
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
		case keyEnter:
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
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
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
		key := msg.String()
		switch {
		case isBackKey(key):
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		case isUpNav(key):
			if m.topicCursor > 0 {
				m.topicCursor--
			}
		case isDownNav(key):
			if m.topicCursor < len(m.topics)-1 {
				m.topicCursor++
			}
		case isEnterKey(key):
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
		key := msg.String()
		switch {
		case isBackKey(key):
			m.screen = screenTopicSelect
		case isUpNav(key):
			if m.sessionConfigField > 0 {
				m.sessionConfigField--
			}
		case isDownNav(key):
			maxField := 1 // questions, mode
			if m.sessionMode == app.ModeByDifficulty || m.sessionMode == app.ModeWeakest {
				maxField = 2 // + sub-option
			}
			if m.sessionConfigField < maxField {
				m.sessionConfigField++
			}
		case isAdjustUp(key):
			m.adjustSessionConfigField(1)
		case isAdjustDown(key):
			m.adjustSessionConfigField(-1)
		case isEnterKey(key):
			m.startConfiguredSession()
		}
	}
	return m, nil
}

// adjustSessionConfigField adjusts the value of the currently focused field.
// dir is +1 for right/up, -1 for left/down.
func (m *model) adjustSessionConfigField(dir int) {
	switch m.sessionConfigField {
	case 0: // question count
		m.questionCount += dir
		if m.questionCount < 1 {
			m.questionCount = 1
		}
		if m.questionCount > m.selectedTopic.QuestionCount {
			m.questionCount = m.selectedTopic.QuestionCount
		}
	case 1: // mode
		m.sessionModeCursor += dir
		if m.sessionModeCursor < 0 {
			m.sessionModeCursor = len(modeOptions) - 1
		}
		if m.sessionModeCursor >= len(modeOptions) {
			m.sessionModeCursor = 0
		}
		m.sessionMode = modeOptions[m.sessionModeCursor]
		// Reset sub-field cursor when mode changes.
		m.sessionConfigField = 1
	case 2: // sub-option
		if m.sessionMode == app.ModeByDifficulty {
			m.sessionDiffCursor += dir
			if m.sessionDiffCursor < 0 {
				m.sessionDiffCursor = len(difficultyOptions) - 1
			}
			if m.sessionDiffCursor >= len(difficultyOptions) {
				m.sessionDiffCursor = 0
			}
			m.sessionDifficulty = difficultyOptions[m.sessionDiffCursor]
		} else if m.sessionMode == app.ModeWeakest {
			m.sessionWeakCursor += dir
			if m.sessionWeakCursor < 0 {
				m.sessionWeakCursor = len(weakestOptions) - 1
			}
			if m.sessionWeakCursor >= len(weakestOptions) {
				m.sessionWeakCursor = 0
			}
			m.sessionWeakestSub = weakestOptions[m.sessionWeakCursor]
		}
	}
}

// startConfiguredSession builds a SessionConfig from the current UI state
// and starts the session via the engine.
func (m *model) startConfiguredSession() {
	cfg := app.SessionConfig{
		TopicSlug:  m.selectedTopic.Topic.Slug,
		N:          m.questionCount,
		Mode:       m.sessionMode,
		Difficulty: m.sessionDifficulty,
		WeakestSub: m.sessionWeakestSub,
	}

	engine := app.NewSessionEngine(
		m.topicRepo, m.questionRepo,
		m.sessionRepo, m.attemptRepo,
		m.userCtx,
		nil, // time-seeded rng
	).WithStatsRepo(m.statsRepo)

	_, err := engine.StartSessionWithConfig(cfg)
	if err != nil {
		// On error, go back to topic select.
		m.screen = screenTopicSelect
		return
	}

	m.engine = engine
	m.totalQuestions = engine.QueueLength()
	m.sessionModeLabel = app.ModeLabel(engine.ActiveMode(), engine.ActiveModeParams())
	m.sessionModeNote = engine.ModeNote()
	m.questionNum = 0
	m.answered = 0
	m.correctCount = 0
	m.totalLatency = 0
	m.wrongAnswers = nil

	// Advance to the first question.
	m.advanceQuestion()
}

// --- Question ---

func (m model) updateQuestion(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			// Cancel session and return to previous config screen.
			_ = m.engine.EndSession()
			m.screen = screenSessionConfig
		case isUpNav(key):
			if m.choiceCursor > 0 {
				m.choiceCursor--
			}
		case isDownNav(key):
			if m.currentQuestion != nil && m.choiceCursor < len(m.currentQuestion.ShuffledChoices)-1 {
				m.choiceCursor++
			}
		case isToggleKey(key):
			// Toggle selection (for multi_select, or single_select).
			if m.currentQuestion != nil && m.currentQuestion.Question != nil {
				choiceID := m.currentQuestion.ShuffledChoices[m.choiceCursor].ID
				if m.currentQuestion.Question.Type == "single_select" {
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
		case isSkipKey(key):
			// Skip this question.
			if m.currentQuestion != nil && m.currentQuestion.Question != nil {
				_, _ = m.engine.RecordAttempt(m.currentQuestion.Question.ID, nil, true, 0)
				m.answered++
				m.lastSkipped = true
				m.lastCorrect = false
				m.submitted = true
				m.showExplanations = false
				m.screen = screenReview
			}
		case isEnterKey(key):
			if m.currentQuestion != nil && m.currentQuestion.Question != nil && len(m.selected) > 0 {
				// Submit answer.
				selectedIDs := make([]string, 0, len(m.selected))
				for id := range m.selected {
					selectedIDs = append(selectedIDs, id)
				}

				latencyMs := int(time.Since(m.questionStarted()).Milliseconds())
				correct, _ := m.engine.RecordAttempt(
					m.currentQuestion.Question.ID, selectedIDs, false, latencyMs,
				)
				m.lastCorrect = correct

				m.answered++
				m.totalLatency += latencyMs
				m.lastSkipped = false
				m.submitted = true
				if m.lastCorrect {
					m.correctCount++
				} else {
					q := *m.currentQuestion.Question
					q.Choices = make([]domain.Choice, len(m.currentQuestion.ShuffledChoices))
					copy(q.Choices, m.currentQuestion.ShuffledChoices)
					// Track wrong answers for review browse.
					m.wrongAnswers = append(m.wrongAnswers, wrongAnswer{
						Question:    q,
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
	q := m.engine.GetNextSessionQuestion()
	if q == nil {
		_ = m.engine.EndSession()
		m.screen = screenSummary
		return
	}
	m.currentQuestion = q
	m.setDisplayLabelMapping(q.ShuffledChoices)
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
		key := msg.String()
		switch {
		case isExplainKey(key):
			// Toggle explanations.
			m.showExplanations = !m.showExplanations
		case isEnterKey(key):
			// Advance to next question.
			m.advanceQuestion()
		case isBackKey(key):
			_ = m.engine.EndSession()
			m.screen = screenSessionConfig
		}
	}
	return m, nil
}

// --- Review Browse (read-only review of wrong answers) ---

func (m model) updateReviewBrowse(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isEnterKey(key):
			if m.reviewCursor < len(m.reviewQueue)-1 {
				m.reviewCursor++
				m.showExplanations = false
				m.setDisplayLabelMapping(m.reviewQueue[m.reviewCursor].Question.Choices)
			} else {
				m.screen = m.reviewReturnScreen
			}
		case isExplainKey(key):
			m.showExplanations = !m.showExplanations
		case isBackKey(key):
			m.screen = m.reviewReturnScreen
		}
	}
	return m, nil
}

// --- Summary ---

func (m model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			_ = m.reloadTopicsForCurrentUser()
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		case isUpNav(key):
			if m.summaryCursor > 0 {
				m.summaryCursor--
			}
		case isDownNav(key):
			maxCursor := 2 // stats, home
			if len(m.wrongAnswers) > 0 {
				maxCursor = 3 // review, stats, home
			}
			if m.summaryCursor < maxCursor {
				m.summaryCursor++
			}
		case isEnterKey(key):
			options := m.summaryOptions()
			if m.summaryCursor >= len(options) {
				break
			}
			switch options[m.summaryCursor] {
			case "Review incorrect questions":
				if len(m.wrongAnswers) > 0 {
					m.startReviewSession(screenSummary)
				}
			case "View stats for this pack":
				m.loadPackDetailStats(m.selectedTopic.Topic.ID)
				m.screen = screenStatsPackDetail
			case "Back to Home":
				_ = m.reloadTopicsForCurrentUser()
				m.homeMenuCursor = 0
				m.screen = screenHomeMenu
			}
		case isReviewKey(key):
			if len(m.wrongAnswers) > 0 {
				m.startReviewSession(screenSummary)
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

// summaryOptions returns the menu items for the summary screen.
func (m model) summaryOptions() []string {
	var opts []string
	if len(m.wrongAnswers) > 0 {
		opts = append(opts, "Review incorrect questions")
	}
	opts = append(opts, "View stats for this pack", "Back to Home")
	return opts
}

// --- Home Menu ---

func (m model) homeMenuOptions() []string {
	opts := []string{"Start Practice", "Review Wrong Answers", "Stats", "Switch Profile", "Quit"}
	return opts
}

func (m model) updateHomeMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isQuitKey(key):
			m.quitting = true
			return m, tea.Quit
		case isBackKey(key):
			m.profileMenuCursor = 0
			m.screen = screenProfileMenu
		case isUpNav(key):
			if m.homeMenuCursor > 0 {
				m.homeMenuCursor--
			}
		case isDownNav(key):
			if m.homeMenuCursor < len(m.homeMenuOptions())-1 {
				m.homeMenuCursor++
			}
		case isEnterKey(key):
			options := m.homeMenuOptions()
			if m.homeMenuCursor >= len(options) {
				return m, nil
			}
			switch options[m.homeMenuCursor] {
			case "Start Practice":
				m.screen = screenTopicSelect
			case "Review Wrong Answers":
				if len(m.wrongAnswers) > 0 {
					m.startReviewSession(screenHomeMenu)
				}
			case "Stats":
				m.statsMenuCursor = 0
				m.screen = screenStatsMenu
			case "Switch Profile":
				m.profileMenuCursor = 0
				m.screen = screenProfileMenu
			case "Quit":
				m.quitting = true
				return m, tea.Quit
			}
		case isReviewKey(key):
			if len(m.wrongAnswers) > 0 {
				m.startReviewSession(screenHomeMenu)
			}
		}
	}
	return m, nil
}

// --- Stats screens ---

func (m model) statsMenuOptions() []string {
	return []string{"Global Stats", "Stats by Pack", "Back"}
}

func (m model) updateStatsMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		case isUpNav(key):
			if m.statsMenuCursor > 0 {
				m.statsMenuCursor--
			}
		case isDownNav(key):
			opts := m.statsMenuOptions()
			if m.statsMenuCursor < len(opts)-1 {
				m.statsMenuCursor++
			}
		case isEnterKey(key):
			opts := m.statsMenuOptions()
			if m.statsMenuCursor >= len(opts) {
				return m, nil
			}
			switch opts[m.statsMenuCursor] {
			case "Global Stats":
				m.loadGlobalStats()
				m.screen = screenStatsGlobal
			case "Stats by Pack":
				m.loadPackListStats()
				m.screen = screenStatsPackList
			case "Back":
				m.homeMenuCursor = 0
				m.screen = screenHomeMenu
			}
		}
	}
	return m, nil
}

func (m *model) loadGlobalStats() {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	gs, err := m.statsRepo.GlobalStats(uid)
	if err != nil {
		m.statsError = err.Error()
		m.statsGlobal = nil
		return
	}
	m.statsGlobal = gs
	m.statsError = ""

	// Load trend for global or most practiced topic.
	trend, _ := m.statsRepo.SessionTrendGlobal(uid, 10)
	m.statsGlobalTrend = trend
}

func (m *model) loadPackListStats() {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	packs, err := m.statsRepo.TopicSummaries(uid)
	if err != nil {
		m.statsError = err.Error()
		m.statsPacks = nil
		return
	}

	// Sort by attempts descending for the current user.
	sortPacksByAttempts(packs)

	m.statsPacks = packs
	m.statsError = ""
	m.statsPackCursor = 0
}

func (m *model) loadPackDetailStats(topicID int64) {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	ts, err := m.statsRepo.TopicSummary(uid, topicID)
	if err != nil {
		m.statsError = err.Error()
		m.statsDetail = nil
		return
	}
	m.statsDetail = ts
	m.statsError = ""

	m.statsDifficulty, _ = m.statsRepo.DifficultyStats(uid, topicID)

	weakTags, _ := m.statsRepo.TagStats(uid, topicID, 5)
	var weak, strong []ports.TagStat
	for _, t := range weakTags {
		if t.AccuracyPct < 70 {
			weak = append(weak, t)
		} else {
			strong = append(strong, t)
		}
	}
	m.statsWeakTags = weak
	m.statsStrongTags = strong

	m.statsWeakQs, _ = m.statsRepo.WeakQuestions(uid, topicID, 3, 10)
	m.statsDetailTrend, _ = m.statsRepo.SessionTrend(uid, topicID, 10)
}

func (m model) updateStatsGlobal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			m.statsMenuCursor = 0
			m.screen = screenStatsMenu
		}
	}
	return m, nil
}

func (m model) updateStatsPackList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			m.statsMenuCursor = 0
			m.screen = screenStatsMenu
		case isUpNav(key):
			if m.statsPackCursor > 0 {
				m.statsPackCursor--
			}
		case isDownNav(key):
			if m.statsPackCursor < len(m.statsPacks)-1 {
				m.statsPackCursor++
			}
		case isEnterKey(key):
			if len(m.statsPacks) > 0 {
				sel := m.statsPacks[m.statsPackCursor]
				m.loadPackDetailStats(sel.TopicID)
				m.screen = screenStatsPackDetail
			}
		}
	}
	return m, nil
}

func (m model) updateStatsPackDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			m.loadPackListStats()
			m.screen = screenStatsPackList
		}
	}
	return m, nil
}

// sortPacksByAttempts sorts packs by AttemptsAnswered descending.
func sortPacksByAttempts(packs []ports.TopicSummary) {
	for i := 1; i < len(packs); i++ {
		for j := i; j > 0 && packs[j].AttemptsAnswered > packs[j-1].AttemptsAnswered; j-- {
			packs[j], packs[j-1] = packs[j-1], packs[j]
		}
	}
}
