package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dezeat/golearn/internal/app"
)

var modeOptions = []app.SelectionMode{
	app.ModeBalanced,
	app.ModeByDifficulty,
	app.ModeWeakest,
}

var difficultyOptions = []string{"easy", "medium", "hard"}

var weakestOptions = []app.WeakestSubMode{
	app.WeakestByQuestion,
	app.WeakestByTag,
}

// viewSessionConfig renders the session configuration screen.
func (m model) viewSessionConfig() string {
	var b strings.Builder

	b.WriteString("golearn — Session Config\n")
	b.WriteString("════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Topic:     %s\n", m.selectedTopic.Topic.Name))
	b.WriteString(fmt.Sprintf("  Available: %d questions\n\n", m.selectedTopic.QuestionCount))

	// Field 0: Questions count.
	qCursor := "  "
	if m.sessionConfigField == 0 {
		qCursor = "▸ "
	}
	b.WriteString(fmt.Sprintf("%sQuestions: ◀ %d ▶\n", qCursor, m.questionCount))

	// Field 1: Mode.
	mCursor := "  "
	if m.sessionConfigField == 1 {
		mCursor = "▸ "
	}
	b.WriteString(fmt.Sprintf("%sMode:      ◀ %s ▶\n", mCursor, app.ModeDisplayName(m.sessionMode)))

	// Field 2: Sub-option (only shown when needed).
	if m.sessionMode == app.ModeByDifficulty {
		dCursor := "  "
		if m.sessionConfigField == 2 {
			dCursor = "▸ "
		}
		b.WriteString(fmt.Sprintf("%sDifficulty: ◀ %s ▶\n", dCursor, m.sessionDifficulty))
	} else if m.sessionMode == app.ModeWeakest {
		wCursor := "  "
		if m.sessionConfigField == 2 {
			wCursor = "▸ "
		}
		subLabel := "by Questions"
		if m.sessionWeakestSub == app.WeakestByTag {
			subLabel = "by Tag"
		}
		b.WriteString(fmt.Sprintf("%sWeakest:    ◀ %s ▶\n", wCursor, subLabel))
	}

	b.WriteString("\n")
	m.writeFooter(&b, footerSessionConfig)
	return b.String()
}

// --- Session Config Update Handler ---

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
