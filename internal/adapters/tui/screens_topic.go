package tui

import (
	"fmt"
	"strings"
)

// viewTopicSelect renders the topic selection screen.
func (m model) viewTopicSelect() string {
	var b strings.Builder

	b.WriteString("golearn — Select a Topic\n")
	b.WriteString("════════════════════════\n\n")

	if len(m.topics) == 0 {
		b.WriteString("  No topics found.\n")
		b.WriteString("  Import a pack first: golearn import <path>\n")
		return b.String()
	}

	for i, ti := range m.topics {
		cursor := "  "
		if i == m.topicCursor {
			cursor = "▸ "
		}

		// Build the info line.
		info := fmt.Sprintf("%d questions", ti.QuestionCount)
		if ti.TotalAttempts > 0 {
			pct := float64(ti.TotalCorrect) / float64(ti.TotalAttempts) * 100
			info += fmt.Sprintf(" · %.0f%% accuracy", pct)
		}

		b.WriteString(fmt.Sprintf("%s%-20s  %s\n", cursor, ti.Topic.Name, info))
	}

	b.WriteString("\n  ↑/↓ or j/k to navigate · enter to select · q to quit\n")
	return b.String()
}
