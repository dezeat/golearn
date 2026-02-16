package tui

import (
	"fmt"
	"strings"
)

// viewSummary renders the session summary screen.
func (m model) viewSummary() string {
	var b strings.Builder

	b.WriteString("golearn — Session Summary\n")
	b.WriteString("═════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Topic:          %s\n", m.selectedTopic.Topic.Name))
	b.WriteString(fmt.Sprintf("  Total answered: %d\n", m.answered))
	b.WriteString(fmt.Sprintf("  Correct:        %d\n", m.correctCount))

	if m.answered > 0 {
		pct := float64(m.correctCount) / float64(m.answered) * 100
		b.WriteString(fmt.Sprintf("  Accuracy:       %.1f%%\n", pct))
	} else {
		b.WriteString("  Accuracy:       —\n")
	}

	b.WriteString("\n  Press enter or b for topics · q to quit\n")
	return b.String()
}
