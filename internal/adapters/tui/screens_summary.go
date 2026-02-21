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
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
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
			return m, nil
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
	return m, nil
}
