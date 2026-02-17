package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) terminalWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m model) centerLine(s string) string {
	w := m.terminalWidth()
	lw := lipgloss.Width(s)
	if lw >= w {
		return s
	}
	left := (w - lw) / 2
	return strings.Repeat(" ", left) + s
}

func (m model) writeCenteredLine(b *strings.Builder, s string) {
	b.WriteString(m.centerLine(s))
	b.WriteString("\n")
}

func (m model) writeFooter(b *strings.Builder, footer string) {
	b.WriteString("\n")
	m.writeCenteredLine(b, styleDim.Render(footer))
}
