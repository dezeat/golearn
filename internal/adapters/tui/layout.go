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

func (m model) writeFooter(b *strings.Builder, footer string) {
	if m.lastError != "" {
		b.WriteString("\n")
		m.writeCenteredLine(b, styleIncorrect.Render("Error: "+m.lastError))
	}
	b.WriteString("\n")
	m.writeCenteredLine(b, styleDim.Render(footer))
}
