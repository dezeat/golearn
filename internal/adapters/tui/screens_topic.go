package tui

import (
	"fmt"
	"strings"
)

// viewIntro renders the ASCII splash screen shown once at startup.
func (m model) viewIntro() string {
	var b strings.Builder

	b.WriteString("\n\n")
	b.WriteString(styleHeader.Render("   ██████╗  ██████╗ ██╗     ███████╗ █████╗ ██████╗ ███╗   ██╗") + "\n")
	b.WriteString(styleHeader.Render("  ██╔════╝ ██╔═══██╗██║     ██╔════╝██╔══██╗██╔══██╗████╗  ██║") + "\n")
	b.WriteString(styleHeader.Render("  ██║  ███╗██║   ██║██║     █████╗  ███████║██████╔╝██╔██╗ ██║") + "\n")
	b.WriteString(styleHeader.Render("  ██║   ██║██║   ██║██║     ██╔══╝  ██╔══██║██╔══██╗██║╚██╗██║") + "\n")
	b.WriteString(styleHeader.Render("  ╚██████╔╝╚██████╔╝███████╗███████╗██║  ██║██║  ██║██║ ╚████║") + "\n")
	b.WriteString(styleHeader.Render("   ╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝") + "\n")
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  adaptive certification practice") + "\n")
	b.WriteString("\n\n")
	b.WriteString("  Press any key to continue\n")

	return b.String()
}

// viewTopicSelect renders the topic selection screen with a fixed-column
// layout that adapts to terminal width without horizontal overflow.
func (m model) viewTopicSelect() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Select a Topic") + "\n")
	b.WriteString("════════════════════════\n\n")

	if len(m.topics) == 0 {
		b.WriteString("  No topics found.\n")
		b.WriteString("  Import a pack first: golearn import <path>\n")
		return b.String()
	}

	// Fixed-column layout: cursor(2) + name(nameW) + gap(2) + questions(14) + gap(2) + accuracy(5)
	w := m.width
	if w == 0 {
		w = 80
	}
	const cursorW = 2
	const qsColW = 14 // "999 questions"
	const accColW = 5 // "100%" or "  —"
	const gaps = 4    // two gaps of 2 chars
	nameW := w - cursorW - gaps - qsColW - accColW
	if nameW < 10 {
		nameW = 10
	}

	for i, ti := range m.topics {
		cursor := "  "
		if i == m.topicCursor {
			cursor = "▸ "
		}

		name := ti.Topic.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}

		qsStr := fmt.Sprintf("%d questions", ti.QuestionCount)
		accStr := "—"
		if ti.TotalAttempts > 0 {
			pct := float64(ti.TotalCorrect) / float64(ti.TotalAttempts) * 100
			accStr = fmt.Sprintf("%.0f%%", pct)
		}

		b.WriteString(fmt.Sprintf("%s%-*s  %-*s  %*s\n", cursor, nameW, name, qsColW, qsStr, accColW, accStr))
	}

	b.WriteString("\n  ↑/↓ or j/k to navigate · enter to select · q to quit\n")
	return b.String()
}
