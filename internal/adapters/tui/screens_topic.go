package tui

import (
	"fmt"
	"strings"
)

// viewProfileMenu renders the startup profile menu with ASCII logo.
func (m model) viewProfileMenu() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleHeader.Render("   ██████╗  ██████╗ ██╗     ███████╗ █████╗ ██████╗ ███╗   ██╗") + "\n")
	b.WriteString(styleHeader.Render("  ██╔════╝ ██╔═══██╗██║     ██╔════╝██╔══██╗██╔══██╗████╗  ██║") + "\n")
	b.WriteString(styleHeader.Render("  ██║  ███╗██║   ██║██║     █████╗  ███████║██████╔╝██╔██╗ ██║") + "\n")
	b.WriteString(styleHeader.Render("  ██║   ██║██║   ██║██║     ██╔══╝  ██╔══██║██╔══██╗██║╚██╗██║") + "\n")
	b.WriteString(styleHeader.Render("  ╚██████╔╝╚██████╔╝███████╗███████╗██║  ██║██║  ██║██║ ╚████║") + "\n")
	b.WriteString(styleHeader.Render("   ╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝") + "\n")
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  golearn — adaptive certification practice") + "\n\n")

	if m.currentUser != nil {
		b.WriteString(fmt.Sprintf("  Current profile: %s\n\n", displayProfile(*m.currentUser)))
	}

	options := m.profileMenuOptions()
	for i, opt := range options {
		cursor := "  "
		if i == m.profileMenuCursor {
			cursor = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, opt))
	}

	if m.profileError != "" {
		b.WriteString("\n")
		b.WriteString(styleIncorrect.Render("  " + m.profileError + "\n"))
	}

	b.WriteString("\n  ↑/↓ or j/k to navigate · enter to select · q to quit\n")
	return b.String()
}

// viewProfileLogin renders existing profile selection.
func (m model) viewProfileLogin() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Login") + "\n")
	b.WriteString("═══════════════\n\n")

	if len(m.profiles) == 0 {
		b.WriteString("  No profiles found. Press esc to go back.\n")
		return b.String()
	}

	for i, p := range m.profiles {
		cursor := "  "
		if i == m.profileLoginCursor {
			cursor = "▸ "
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, displayProfile(p)))
	}

	if m.profileError != "" {
		b.WriteString("\n")
		b.WriteString(styleIncorrect.Render("  " + m.profileError + "\n"))
	}

	b.WriteString("\n  ↑/↓ or j/k to navigate · enter to login · esc to back\n")
	return b.String()
}

// viewProfileRegister renders profile creation without passwords.
func (m model) viewProfileRegister() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Register") + "\n")
	b.WriteString("══════════════════\n\n")
	b.WriteString("  Create a local profile (no passwords).\n\n")

	handlePrefix := "  "
	namePrefix := "  "
	if m.registerField == 0 {
		handlePrefix = "▸ "
	} else {
		namePrefix = "▸ "
	}

	b.WriteString(fmt.Sprintf("%sHandle:      %s\n", handlePrefix, m.registerHandle))
	b.WriteString(fmt.Sprintf("%sDisplay name: %s\n", namePrefix, m.registerDisplayName))
	b.WriteString("\n  Handle rules: a-z 0-9 - _\n")

	if m.profileError != "" {
		b.WriteString("\n")
		b.WriteString(styleIncorrect.Render("  " + m.profileError + "\n"))
	}

	b.WriteString("\n  type to edit · tab switch field · enter next/create · esc back\n")

	return b.String()
}

// viewTopicSelect renders the topic selection screen with a fixed-column
// layout that adapts to terminal width without horizontal overflow.
func (m model) viewTopicSelect() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Select a Topic") + "\n")
	b.WriteString("════════════════════════\n\n")
	if m.currentUser != nil {
		b.WriteString(fmt.Sprintf("  Profile: %s\n\n", displayProfile(*m.currentUser)))
	}

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
