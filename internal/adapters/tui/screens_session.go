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
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/app"
)

var modeOptions = []app.SelectionMode{
	app.ModeBalanced,
	app.ModeRandom,
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

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Session Config"))
	m.writeCenteredLine(&b, "════════════════════════")
	b.WriteString("\n")

	const metaLabelW = 10
	const fieldLabelW = 10
	const labelToColonPad = 2

	mid := m.terminalWidth() / 2
	rightAvail := m.terminalWidth() - (mid + 2)
	if rightAvail < 1 {
		rightAvail = 1
	}

	valueCandidates := []string{
		fmt.Sprintf("◀ %d ▶", m.selectedTopic.QuestionCount),
		"—",
		"◀ by Questions ▶",
		"◀ by Tag ▶",
	}
	for _, mode := range modeOptions {
		valueCandidates = append(valueCandidates, fmt.Sprintf("◀ %s ▶", app.ModeDisplayName(mode)))
	}
	for _, diff := range difficultyOptions {
		valueCandidates = append(valueCandidates, fmt.Sprintf("◀ %s ▶", diff))
	}
	valueW := maxLineWidth(valueCandidates)
	if valueW > rightAvail {
		valueW = rightAvail
	}
	if valueW < 1 {
		valueW = 1
	}

	renderLine := func(cursor string, labelW int, label string, value string, dim bool) string {
		left := fmt.Sprintf("%s%*s", cursor, labelW, label)
		prefix := mid - lipgloss.Width(left) - labelToColonPad
		if prefix < 0 {
			prefix = 0
		}
		value = truncate(value, valueW)
		line := strings.Repeat(" ", prefix) + left + strings.Repeat(" ", labelToColonPad) + ": " + padRightVisible(value, valueW)
		if dim {
			return styleDim.Render(line)
		}
		return line
	}

	topicName := truncate(m.selectedTopic.Topic.Name, valueW)
	availableText := truncate(fmt.Sprintf("%d questions", m.selectedTopic.QuestionCount), valueW)

	lines := []string{
		renderLine("  ", metaLabelW, "Topic", topicName, false),
		renderLine("  ", metaLabelW, "Available", availableText, false),
		"",
	}

	renderFieldLine := func(active bool, dim bool, label string, value string) string {
		cursor := "  "
		if active {
			cursor = "▸ "
		}
		return renderLine(cursor, fieldLabelW, label, value, dim)
	}

	// Field 0: Questions count.
	lines = append(lines, renderFieldLine(m.sessionConfigField == 0, false, "Questions", fmt.Sprintf("◀ %d ▶", m.questionCount)))

	// Field 1: Mode.
	lines = append(lines, renderFieldLine(m.sessionConfigField == 1, false, "Mode", fmt.Sprintf("◀ %s ▶", app.ModeDisplayName(m.sessionMode))))

	// Field 2: Sub-option (always rendered to keep layout fixed).
	switch m.sessionMode {
	case app.ModeByDifficulty:
		lines = append(lines, renderFieldLine(m.sessionConfigField == 2, false, "Difficulty", fmt.Sprintf("◀ %s ▶", m.sessionDifficulty)))
	case app.ModeWeakest:
		subLabel := "by Questions"
		if m.sessionWeakestSub == app.WeakestByTag {
			subLabel = "by Tag"
		}
		lines = append(lines, renderFieldLine(m.sessionConfigField == 2, false, "Weakest", fmt.Sprintf("◀ %s ▶", subLabel)))
	default:
		lines = append(lines, renderFieldLine(false, true, "Option", "—"))
	}

	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	m.writeFooter(&b, footerSessionConfig)
	return b.String()
}

// --- Session Config Update Handler ---

func (m model) updateSessionConfig(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
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
		switch m.sessionMode {
		case app.ModeByDifficulty:
			m.sessionDiffCursor += dir
			if m.sessionDiffCursor < 0 {
				m.sessionDiffCursor = len(difficultyOptions) - 1
			}
			if m.sessionDiffCursor >= len(difficultyOptions) {
				m.sessionDiffCursor = 0
			}
			m.sessionDifficulty = difficultyOptions[m.sessionDiffCursor]
		case app.ModeWeakest:
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
