package tui

import (
	"fmt"
	"strings"
)

// viewProfileMenu renders the startup profile menu with ASCII logo.
func (m model) viewProfileMenu() string {
	var b strings.Builder

	b.WriteString("\n")
	m.writeCenteredLine(&b, styleHeader.Render("   ██████╗  ██████╗ ██╗     ███████╗ █████╗ ██████╗ ███╗   ██╗"))
	m.writeCenteredLine(&b, styleHeader.Render("  ██╔════╝ ██╔═══██╗██║     ██╔════╝██╔══██╗██╔══██╗████╗  ██║"))
	m.writeCenteredLine(&b, styleHeader.Render("  ██║  ███╗██║   ██║██║     █████╗  ███████║██████╔╝██╔██╗ ██║"))
	m.writeCenteredLine(&b, styleHeader.Render("  ██║   ██║██║   ██║██║     ██╔══╝  ██╔══██║██╔══██╗██║╚██╗██║"))
	m.writeCenteredLine(&b, styleHeader.Render("  ╚██████╔╝╚██████╔╝███████╗███████╗██║  ██║██║  ██║██║ ╚████║"))
	m.writeCenteredLine(&b, styleHeader.Render("   ╚═════╝  ╚═════╝ ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝"))
	b.WriteString("\n")
	m.writeCenteredLine(&b, styleDim.Render("golearn — adaptive certification practice"))
	b.WriteString("\n")

	options := m.profileMenuOptions()
	for i, opt := range options {
		cursor := "  "
		if i == m.profileMenuCursor {
			cursor = "▸ "
		}
		m.writeCenteredLine(&b, fmt.Sprintf("%s%s", cursor, opt))
	}

	if m.profileError != "" {
		b.WriteString("\n")
		m.writeCenteredLine(&b, styleIncorrect.Render(m.profileError))
	}

	m.writeFooter(&b, footerMenuTop)
	return b.String()
}

// viewProfileLogin renders existing profile selection.
func (m model) viewProfileLogin() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Login"))
	m.writeCenteredLine(&b, "═══════════════")
	b.WriteString("\n")

	if len(m.profiles) == 0 {
		m.writeCenteredLine(&b, "No profiles found.")
		m.writeFooter(&b, footerMenuSub)
		return b.String()
	}

	for i, p := range m.profiles {
		cursor := "  "
		if i == m.profileLoginCursor {
			cursor = "▸ "
		}
		m.writeCenteredLine(&b, fmt.Sprintf("%s%s", cursor, displayProfile(p)))
	}

	if m.profileError != "" {
		b.WriteString("\n")
		m.writeCenteredLine(&b, styleIncorrect.Render(m.profileError))
	}

	m.writeFooter(&b, footerMenuSub)
	return b.String()
}

// viewProfileRegister renders profile creation without passwords.
func (m model) viewProfileRegister() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Register"))
	m.writeCenteredLine(&b, "══════════════════")
	b.WriteString("\n")
	m.writeCenteredLine(&b, "Create a local profile (no passwords).")
	b.WriteString("\n")

	handlePrefix := "  "
	namePrefix := "  "
	if m.registerField == 0 {
		handlePrefix = "▸ "
	} else {
		namePrefix = "▸ "
	}

	m.writeCenteredLine(&b, fmt.Sprintf("%sHandle:      %s", handlePrefix, m.registerHandle))
	m.writeCenteredLine(&b, fmt.Sprintf("%sDisplay name: %s", namePrefix, m.registerDisplayName))
	b.WriteString("\n")
	m.writeCenteredLine(&b, "Handle rules: a-z 0-9 - _")

	if m.profileError != "" {
		b.WriteString("\n")
		m.writeCenteredLine(&b, styleIncorrect.Render(m.profileError))
	}

	m.writeFooter(&b, footerMenuSub)

	return b.String()
}

// viewTopicSelect renders the topic selection screen with a fixed-column
// layout that adapts to terminal width without horizontal overflow.
func (m model) viewTopicSelect() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Select a Topic"))
	m.writeCenteredLine(&b, "════════════════════════")
	b.WriteString("\n")

	if len(m.topics) == 0 {
		m.writeCenteredLine(&b, "No topics found.")
		m.writeCenteredLine(&b, "Import a pack first: golearn import <path>")
		m.writeFooter(&b, footerMenuSub)
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

		m.writeCenteredLine(&b, fmt.Sprintf("%s%-*s  %-*s  %*s", cursor, nameW, name, qsColW, qsStr, accColW, accStr))
	}

	m.writeFooter(&b, footerMenuSub)
	return b.String()
}
