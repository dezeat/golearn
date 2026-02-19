package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Init implements tea.Model. It returns no initial command.
func (m model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It routes messages to the appropriate
// screen handler based on the current screen state.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit: ctrl+c always exits.
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		// Clear transient error on any key press.
		m.lastError = ""
	}

	switch m.screen {
	case screenProfileMenu:
		return m.updateProfileMenu(msg)
	case screenProfileLogin:
		return m.updateProfileLogin(msg)
	case screenProfileRegister:
		return m.updateProfileRegister(msg)
	case screenHomeMenu:
		return m.updateHomeMenu(msg)
	case screenTopicSelect:
		return m.updateTopicSelect(msg)
	case screenSessionConfig:
		return m.updateSessionConfig(msg)
	case screenQuestion:
		return m.updateQuestion(msg)
	case screenReview:
		return m.updateReview(msg)
	case screenReviewBrowse:
		return m.updateReviewBrowse(msg)
	case screenSummary:
		return m.updateSummary(msg)
	case screenStatsMenu:
		return m.updateStatsMenu(msg)
	case screenStatsGlobal:
		return m.updateStatsGlobal(msg)
	case screenStatsPackList:
		return m.updateStatsPackList(msg)
	case screenStatsPackDetail:
		return m.updateStatsPackDetail(msg)
	}

	return m, nil
}

// View implements tea.Model.
func (m model) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case screenProfileMenu:
		return m.viewProfileMenu()
	case screenProfileLogin:
		return m.viewProfileLogin()
	case screenProfileRegister:
		return m.viewProfileRegister()
	case screenHomeMenu:
		return m.viewHomeMenu()
	case screenTopicSelect:
		return m.viewTopicSelect()
	case screenSessionConfig:
		return m.viewSessionConfig()
	case screenQuestion:
		return m.viewQuestion()
	case screenReview:
		return m.viewReview()
	case screenReviewBrowse:
		return m.viewReviewBrowse()
	case screenSummary:
		return m.viewSummary()
	case screenStatsMenu:
		return m.viewStatsMenu()
	case screenStatsGlobal:
		return m.viewStatsGlobal()
	case screenStatsPackList:
		return m.viewStatsPackList()
	case screenStatsPackDetail:
		return m.viewStatsPackDetail()
	}

	return ""
}

// contentWidth returns the available character width for content,
// subtracting padding from the terminal width. Falls back to 80
// columns when no WindowSizeMsg has been received yet.
func (m model) contentWidth(padding int) int {
	w := m.width
	if w == 0 {
		w = 80
	}
	cw := w - padding
	if cw < 20 {
		cw = 20
	}
	return cw
}
