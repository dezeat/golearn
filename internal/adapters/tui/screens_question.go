package tui

import (
	"fmt"
	"strings"
)

// viewQuestion renders the question answering screen.
func (m model) viewQuestion() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("golearn — Question %d/%d\n", m.questionNum, m.totalQuestions))
	b.WriteString("════════════════════════\n\n")

	if m.currentQuestion == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}

	q := m.currentQuestion

	// Show type hint.
	typeHint := "Select one"
	if q.Type == "multi_select" {
		typeHint = "Select all that apply"
	}
	b.WriteString(fmt.Sprintf("  [%s]\n\n", typeHint))

	// Show intro if present.
	if q.Intro != "" {
		b.WriteString(fmt.Sprintf("  %s\n\n", q.Intro))
	}

	// Show prompt.
	b.WriteString(fmt.Sprintf("  %s\n\n", q.Prompt))

	// Show choices.
	for i, c := range q.Choices {
		cursor := "  "
		if i == m.choiceCursor {
			cursor = "▸ "
		}

		// Show selection state.
		marker := "○"
		if m.selected[c.ID] {
			marker = "●"
		}

		b.WriteString(fmt.Sprintf("  %s%s %s) %s\n", cursor, marker, c.ID, c.Text))
	}

	b.WriteString("\n  ↑/↓ navigate · space toggle · enter submit · s skip · q quit\n")
	return b.String()
}

// viewFeedback renders the brief feedback shown after submitting an answer.
func (m model) viewFeedback() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("golearn — Question %d/%d\n", m.questionNum, m.totalQuestions))
	b.WriteString("════════════════════════\n\n")

	if m.currentQuestion != nil {
		b.WriteString(fmt.Sprintf("  %s\n\n", m.currentQuestion.Prompt))
	}

	if m.lastSkipped {
		b.WriteString("  ⏭  Skipped\n")
	} else if m.lastCorrect {
		b.WriteString("  ✓  Correct!\n")
	} else {
		b.WriteString("  ✗  Incorrect\n")
		if m.currentQuestion != nil {
			b.WriteString(fmt.Sprintf("     Correct answer: %s\n",
				strings.Join(m.currentQuestion.CorrectChoiceIDs, ", ")))
		}
	}

	b.WriteString("\n  Press enter to continue · q to end session\n")
	return b.String()
}
