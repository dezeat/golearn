package tui

import (
	"fmt"
	"strings"

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
