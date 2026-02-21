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
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

	m.writeFooter(&b, footerMenuRoot)
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

	m.writeFooter(&b, footerRegister)

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

// --- Profile Update Handlers ---

func (m model) updateProfileMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isQuitKey(key):
		m.quitting = true
		return m, tea.Quit
	case isUpNav(key):
		if m.profileMenuCursor > 0 {
			m.profileMenuCursor--
		}
	case isDownNav(key):
		if m.profileMenuCursor < len(m.profileMenuOptions())-1 {
			m.profileMenuCursor++
		}
	case isEnterKey(key):
		options := m.profileMenuOptions()
		if len(options) == 0 {
			return m, nil
		}

		selected := options[m.profileMenuCursor]
		switch selected {
		case "Login":
			profiles, err := m.userRepo.List()
			if err != nil {
				m.profileError = "Failed to load profiles"
				return m, nil
			}
			m.profiles = profiles
			m.profileLoginCursor = 0
			m.profileError = ""
			m.screen = screenProfileLogin
		case "Register":
			m.registerHandle = ""
			m.registerDisplayName = ""
			m.registerField = 0
			m.profileError = ""
			m.screen = screenProfileRegister
		default: // Continue
			if m.currentUser == nil {
				m.profileError = "No active profile"
				return m, nil
			}
			if err := m.reloadTopicsForCurrentUser(); err != nil {
				m.profileError = "Failed to load topics"
				return m, nil
			}
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		}
	}
	return m, nil
}

func (m model) updateProfileLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isBackKey(key):
		m.screen = screenProfileMenu
	case isUpNav(key):
		if m.profileLoginCursor > 0 {
			m.profileLoginCursor--
		}
	case isDownNav(key):
		if m.profileLoginCursor < len(m.profiles)-1 {
			m.profileLoginCursor++
		}
	case isEnterKey(key):
		if len(m.profiles) == 0 {
			m.profileError = "No profiles available"
			return m, nil
		}
		selected := m.profiles[m.profileLoginCursor]
		if err := m.setCurrentUser(&selected, true); err != nil {
			m.profileError = "Failed to save profile"
			return m, nil
		}
		if err := m.reloadTopicsForCurrentUser(); err != nil {
			m.profileError = "Failed to load topics"
			return m, nil
		}
		m.profileError = ""
		m.homeMenuCursor = 0
		m.screen = screenHomeMenu
	}
	return m, nil
}

func (m model) updateProfileRegister(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch key {
	case keyBack:
		m.screen = screenProfileMenu
		return m, nil
	case "tab":
		m.registerField = (m.registerField + 1) % 2
		return m, nil
	case "backspace":
		if m.registerField == 0 {
			if len(m.registerHandle) > 0 {
				m.registerHandle = m.registerHandle[:len(m.registerHandle)-1]
			}
		} else if len(m.registerDisplayName) > 0 {
			m.registerDisplayName = m.registerDisplayName[:len(m.registerDisplayName)-1]
		}
		return m, nil
	case keyEnter:
		m.profileError = ""
		if m.registerField == 0 {
			handle := m.registerHandle
			if !isValidHandle(handle) {
				m.profileError = "Handle must use: a-z 0-9 - _"
				return m, nil
			}
			existing, found, err := m.userRepo.GetByHandle(handle)
			if err != nil {
				m.profileError = "Failed to check handle"
				return m, nil
			}
			if found && existing != nil {
				m.profileError = "Handle already exists"
				return m, nil
			}
			m.registerField = 1
			return m, nil
		}

		u, err := m.userRepo.Create(m.registerHandle, m.registerDisplayName)
		if err != nil {
			m.profileError = "Failed to create profile"
			return m, nil
		}
		if err := m.setCurrentUser(u, true); err != nil {
			m.profileError = "Failed to save profile"
			return m, nil
		}
		profiles, err := m.userRepo.List()
		if err == nil {
			m.profiles = profiles
		}
		if err := m.reloadTopicsForCurrentUser(); err != nil {
			m.profileError = "Failed to load topics"
			return m, nil
		}
		m.homeMenuCursor = 0
		m.screen = screenHomeMenu
		return m, nil
	}

	if len(keyMsg.Runes) == 0 {
		return m, nil
	}
	r := keyMsg.Runes[0]
	if m.registerField == 0 {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			m.registerHandle += string(r)
		}
	} else if r >= 32 && r <= 126 {
		m.registerDisplayName += string(r)
	}
	return m, nil
}

// --- Topic Select Update Handler ---

func (m model) updateTopicSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isBackKey(key):
		m.homeMenuCursor = 0
		m.screen = screenHomeMenu
	case isUpNav(key):
		if m.topicCursor > 0 {
			m.topicCursor--
		}
	case isDownNav(key):
		if m.topicCursor < len(m.topics)-1 {
			m.topicCursor++
		}
	case isEnterKey(key):
		if len(m.topics) > 0 {
			m.selectedTopic = m.topics[m.topicCursor]
			// Cap default question count to available questions.
			if m.questionCount > m.selectedTopic.QuestionCount {
				m.questionCount = m.selectedTopic.QuestionCount
			}
			m.screen = screenSessionConfig
		}
	}
	return m, nil
}
