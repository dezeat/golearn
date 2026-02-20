package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// startReviewSession sets up the review browse queue from wrong answers.
// No new quiz session is created — just a read-only browse through mistakes.
func (m *model) startReviewSession(returnTo screen) {
	m.reviewQueue = make([]wrongAnswer, len(m.wrongAnswers))
	copy(m.reviewQueue, m.wrongAnswers)
	m.reviewCursor = 0
	if len(m.reviewQueue) > 0 {
		m.setDisplayLabelMapping(m.reviewQueue[0].Question.Choices)
	} else {
		m.displayLabelByChoiceID = nil
		m.choiceIDByDisplayLabel = nil
	}
	m.reviewReturnScreen = returnTo
	m.showExplanations = false
	m.screen = screenReviewBrowse
}

// summaryOptions returns the menu items for the summary screen.
func (m model) summaryOptions() []string {
	var opts []string
	if len(m.wrongAnswers) > 0 {
		opts = append(opts, "Review incorrect questions")
	}
	opts = append(opts, "View stats for this pack")
	return opts
}

// --- Summary Update Handler ---

func (m model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch {
		case isBackKey(key):
			if err := m.reloadTopicsForCurrentUser(); err != nil {
				m.lastError = fmt.Sprintf("reload topics: %v", err)
			}
			m.homeMenuCursor = 0
			m.screen = screenHomeMenu
		case isUpNav(key):
			if m.summaryCursor > 0 {
				m.summaryCursor--
			}
		case isDownNav(key):
			if m.summaryCursor < len(m.summaryOptions())-1 {
				m.summaryCursor++
			}
		case isEnterKey(key):
			options := m.summaryOptions()
			if m.summaryCursor >= len(options) {
				break
			}
			switch options[m.summaryCursor] {
			case "Review incorrect questions":
				if len(m.wrongAnswers) > 0 {
					m.startReviewSession(screenSummary)
				}
			case "View stats for this pack":
				m.loadPackDetailStats(m.selectedTopic.Topic.ID)
				m.screen = screenStatsPackDetail
			}
		case isReviewKey(key):
			if len(m.wrongAnswers) > 0 {
				m.startReviewSession(screenSummary)
			}
		}
	}
	return m, nil
}
