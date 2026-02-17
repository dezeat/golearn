package tui

import (
	"fmt"
	"strings"
)

// viewSummary renders the session summary screen with review option.
func (m model) viewSummary() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Session Summary") + "\n")
	b.WriteString("═════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Topic:          %s\n", m.selectedTopic.Topic.Name))
	b.WriteString(fmt.Sprintf("  Total answered: %d\n", m.answered))
	b.WriteString(fmt.Sprintf("  Correct:        %d\n", m.correctCount))

	if m.answered > 0 {
		pct := float64(m.correctCount) / float64(m.answered) * 100
		b.WriteString(fmt.Sprintf("  Accuracy:       %.1f%%\n", pct))

		if m.totalLatency > 0 {
			avgMs := m.totalLatency / m.answered
			if avgMs >= 1000 {
				b.WriteString(fmt.Sprintf("  Avg response:   %.1fs\n", float64(avgMs)/1000))
			} else {
				b.WriteString(fmt.Sprintf("  Avg response:   %dms\n", avgMs))
			}
		}
	} else {
		b.WriteString("  Accuracy:       —\n")
	}

	wrongCount := len(m.wrongAnswers)
	if wrongCount > 0 {
		b.WriteString(fmt.Sprintf("\n  %s\n",
			styleIncorrect.Render(fmt.Sprintf("  %d question(s) answered incorrectly", wrongCount))))
		b.WriteString(fmt.Sprintf("  %s\n",
			styleBold.Render("  Press r to review wrong questions")))
	}

	b.WriteString("\n  enter or b for topics · r review wrong · q to quit\n")
	return b.String()
}

// startReviewSession sets up the review browse queue from wrong answers.
// No new quiz session is created — just a read-only browse through mistakes.
func (m *model) startReviewSession() {
	m.reviewQueue = make([]wrongAnswer, len(m.wrongAnswers))
	copy(m.reviewQueue, m.wrongAnswers)
	m.reviewCursor = 0
	m.showExplanations = false
	m.screen = screenReviewBrowse
}
