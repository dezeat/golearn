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
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) terminalWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m *model) centerLine(s string) string {
	w := m.terminalWidth()
	lw := lipgloss.Width(s)
	if lw >= w {
		return s
	}
	left := (w - lw) / 2
	return strings.Repeat(" ", left) + s
}

func (m *model) writeCenteredLine(b *strings.Builder, s string) {
	b.WriteString(m.centerLine(s))
	b.WriteString("\n")
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if lw := lipgloss.Width(line); lw > width {
			width = lw
		}
	}
	return width
}

func padRightVisible(s string, width int) string {
	pad := width - lipgloss.Width(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func (m *model) writeCenteredLeftLine(b *strings.Builder, width int, s string) {
	m.writeCenteredLine(b, padRightVisible(s, width))
}

func (m *model) centeredBlockWidth(maxWidth, totalPadding, minWidth int) int {
	w := m.terminalWidth()
	if minWidth < 1 {
		minWidth = 1
	}

	blockW := w - totalPadding
	if blockW > maxWidth {
		blockW = maxWidth
	}
	if blockW < minWidth {
		if w < minWidth {
			return w
		}
		return minWidth
	}
	return blockW
}

func (m model) writeFooter(b *strings.Builder, footer string) {
	if m.lastError != "" {
		b.WriteString("\n")
		m.writeCenteredLine(b, styleIncorrect.Render("Error: "+m.lastError))
	}
	b.WriteString("\n")
	m.writeCenteredLine(b, styleDim.Render(footer))
}
