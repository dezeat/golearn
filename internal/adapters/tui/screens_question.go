package tui

import (
	"fmt"
	"strings"
)

// viewQuestion renders the question answering screen.
func (m model) viewQuestion() string {
	var b strings.Builder

	header := fmt.Sprintf("golearn — Question %d/%d", m.questionNum, m.totalQuestions)
	if m.reviewMode {
		header += " (Review)"
	}
	b.WriteString(styleHeader.Render(header) + "\n")
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
	b.WriteString(fmt.Sprintf("  %s\n\n", styleBold.Render(q.Prompt)))

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

// viewReview renders the quiz-show review screen after answer submission.
// Shows full question + choices with color-coded feedback.
func (m model) viewReview() string {
	var b strings.Builder

	header := fmt.Sprintf("golearn — Question %d/%d — Review", m.questionNum, m.totalQuestions)
	b.WriteString(styleHeader.Render(header) + "\n")
	b.WriteString("════════════════════════\n\n")

	if m.currentQuestion == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}

	q := m.currentQuestion

	// Show prompt.
	if q.Intro != "" {
		b.WriteString(fmt.Sprintf("  %s\n\n", q.Intro))
	}
	b.WriteString(fmt.Sprintf("  %s\n\n", styleBold.Render(q.Prompt)))

	if m.lastSkipped {
		b.WriteString("  ⏭  " + styleDim.Render("Skipped") + "\n\n")
	} else if m.lastCorrect {
		b.WriteString("  " + styleCorrect.Render("✔ Correct!") + "\n\n")
	} else {
		b.WriteString("  " + styleIncorrect.Render("✘ Incorrect") + "\n\n")
	}

	// Build a set of correct choice IDs for lookup.
	correctSet := make(map[string]bool, len(q.CorrectChoiceIDs))
	for _, id := range q.CorrectChoiceIDs {
		correctSet[id] = true
	}

	// Show choices with visual feedback.
	for _, c := range q.Choices {
		isCorrect := correctSet[c.ID]
		isSelected := m.selected[c.ID]

		var marker, line string
		switch {
		case isCorrect && isSelected:
			// Correct and user selected it — green ✔
			marker = styleCorrect.Render("✔")
			line = styleCorrect.Render(fmt.Sprintf("%s) %s", c.ID, c.Text))
		case isCorrect && !isSelected:
			// Correct but user didn't select — green (missed)
			marker = styleCorrect.Render("✔")
			line = styleCorrect.Render(fmt.Sprintf("%s) %s", c.ID, c.Text))
		case !isCorrect && isSelected:
			// Wrong and user selected it — red ✘
			marker = styleIncorrect.Render("✘")
			line = styleIncorrect.Render(fmt.Sprintf("%s) %s", c.ID, c.Text))
		default:
			// Wrong and user didn't select — neutral
			marker = "  "
			line = fmt.Sprintf("%s) %s", c.ID, c.Text)
		}

		// Add selection indicator.
		selectIndicator := " "
		if isSelected {
			selectIndicator = styleSelected.Render("▸")
		}

		b.WriteString(fmt.Sprintf("  %s %s %s\n", selectIndicator, marker, line))

		// Show per-choice explanation if toggled.
		if m.showExplanations && q.Rationale.PerChoice != nil {
			if explanation, ok := q.Rationale.PerChoice[c.ID]; ok {
				b.WriteString(fmt.Sprintf("       %s\n", styleExplain.Render(explanation)))
			}
		}
	}

	// Show correct explanation if available and explanations toggled.
	if m.showExplanations && q.Rationale.Correct != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", styleBold.Render("Explanation:")))
		b.WriteString(fmt.Sprintf("  %s\n", styleExplain.Render(q.Rationale.Correct)))
	}

	// Controls.
	b.WriteString("\n")
	explainHint := "e show explanations"
	if m.showExplanations {
		explainHint = "e hide explanations"
	}
	b.WriteString(fmt.Sprintf("  %s · enter next · q end session\n", explainHint))

	return b.String()
}
