package tui

import (
	"fmt"
	"strings"
)

// viewSessionConfig renders the session configuration screen.
func (m model) viewSessionConfig() string {
	var b strings.Builder

	b.WriteString("golearn — Session Config\n")
	b.WriteString("════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Topic:     %s\n", m.selectedTopic.Topic.Name))
	b.WriteString(fmt.Sprintf("  Available: %d questions\n", m.selectedTopic.QuestionCount))
	b.WriteString(fmt.Sprintf("  Mode:      Practice\n\n"))

	b.WriteString(fmt.Sprintf("  Questions: ◀ %d ▶\n\n", m.questionCount))

	m.writeFooter(&b, footerSessionConfig)
	return b.String()
}
