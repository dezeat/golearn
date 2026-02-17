package tui

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
