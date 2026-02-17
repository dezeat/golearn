package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/domain"
)

// viewQuestion renders the question answering screen.
func (m model) viewQuestion() string {
	var b strings.Builder

	header := fmt.Sprintf("golearn — Question %d/%d", m.questionNum, m.totalQuestions)
	b.WriteString(styleHeader.Render(header) + "\n")
	b.WriteString("════════════════════════\n")

	// Show mode context.
	if m.sessionModeLabel != "" {
		b.WriteString(styleDim.Render("  "+truncate(m.sessionModeLabel, m.contentWidth(2))) + "\n")
	}
	if m.sessionModeNote != "" {
		b.WriteString(styleDim.Render("  "+truncate(m.sessionModeNote, m.contentWidth(2))) + "\n")
	}
	b.WriteString("\n")

	if m.currentQuestion == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}

	q := m.currentQuestion.Question
	if q == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}
	cw := m.contentWidth(2)

	// Show type hint.
	typeHint := "Select one"
	if q.Type == "multi_select" {
		typeHint = "Select all that apply"
	}
	b.WriteString(fmt.Sprintf("  [%s]\n\n", typeHint))

	// Show intro if present.
	if q.Intro != "" {
		b.WriteString(wrapAndIndent(q.Intro, cw, "  ") + "\n\n")
	}

	// Show prompt.
	b.WriteString(styleBold.Render(wrapAndIndent(q.Prompt, cw, "  ")) + "\n\n")

	// Show choices with wrapping.
	const choicePad = 10 // visual: "  ▸ ● A) "
	choiceCW := m.contentWidth(choicePad)
	contIndent := "          " // 10 spaces

	for i, c := range m.currentQuestion.ShuffledChoices {
		cursor := "  "
		if i == m.choiceCursor {
			cursor = "▸ "
		}

		// Show selection state.
		marker := "○"
		if m.selected[c.ID] {
			marker = "●"
		}

		choiceText := fmt.Sprintf("%s) %s", m.displayLabelForChoiceID(c.ID), c.Text)
		wrappedLines := strings.Split(wrapText(choiceText, choiceCW), "\n")

		b.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, marker, wrappedLines[0]))
		for _, l := range wrappedLines[1:] {
			b.WriteString(contIndent + l + "\n")
		}
	}

	m.writeFooter(&b, footerQuiz)
	return b.String()
}

// viewReview renders the quiz-show review screen after answer submission.
// Shows full question + choices with color-coded feedback.
func (m model) viewReview() string {
	var b strings.Builder

	header := fmt.Sprintf("golearn — Question %d/%d — Review", m.questionNum, m.totalQuestions)
	b.WriteString(styleHeader.Render(header) + "\n")
	b.WriteString("════════════════════════\n")

	// Show mode context.
	if m.sessionModeLabel != "" {
		b.WriteString(styleDim.Render("  "+truncate(m.sessionModeLabel, m.contentWidth(2))) + "\n")
	}
	b.WriteString("\n")

	if m.currentQuestion == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}

	q := m.currentQuestion.Question
	if q == nil {
		b.WriteString("  No question loaded.\n")
		return b.String()
	}
	cw := m.contentWidth(2)

	// Show prompt.
	if q.Intro != "" {
		b.WriteString(wrapAndIndent(q.Intro, cw, "  ") + "\n\n")
	}
	b.WriteString(styleBold.Render(wrapAndIndent(q.Prompt, cw, "  ")) + "\n\n")

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

	// Show choices with visual feedback and wrapping.
	const choicePad = 8 // visual: "  ▸ ✔ " prefix
	choiceCW := m.contentWidth(choicePad)
	contIndent := "        " // 8 spaces

	for _, c := range m.currentQuestion.ShuffledChoices {
		isCorrect := correctSet[c.ID]
		isSelected := m.selected[c.ID]

		var marker string
		var choiceStyle *lipgloss.Style
		switch {
		case isCorrect && isSelected:
			marker = styleCorrect.Render("✔")
			s := styleCorrect
			choiceStyle = &s
		case isCorrect && !isSelected:
			marker = styleCorrect.Render("✔")
			s := styleCorrect
			choiceStyle = &s
		case !isCorrect && isSelected:
			marker = styleIncorrect.Render("✘")
			s := styleIncorrect
			choiceStyle = &s
		default:
			marker = " "
			choiceStyle = nil
		}

		// Add selection indicator.
		selectIndicator := " "
		if isSelected {
			selectIndicator = styleSelected.Render("▸")
		}

		choiceText := fmt.Sprintf("%s) %s", m.displayLabelForChoiceID(c.ID), c.Text)
		wrappedLines := strings.Split(wrapText(choiceText, choiceCW), "\n")

		for li, l := range wrappedLines {
			if choiceStyle != nil {
				l = choiceStyle.Render(l)
			}
			if li == 0 {
				b.WriteString(fmt.Sprintf("  %s %s %s\n", selectIndicator, marker, l))
			} else {
				b.WriteString(contIndent + l + "\n")
			}
		}

		// Show per-choice explanation if toggled.
		if m.showExplanations && q.Rationale.PerChoice != nil {
			if explanation, ok := q.Rationale.PerChoice[c.ID]; ok {
				expCW := m.contentWidth(7)
				formatted := domain.FormatChoiceExplanation(c.ID, explanation, q.CorrectChoiceIDs)
				b.WriteString(styleExplain.Render(wrapAndIndent(formatted, expCW, "       ")) + "\n")
			}
		}
	}

	// Show correct explanation if available and explanations toggled.
	if m.showExplanations && q.Rationale.Correct != "" {
		b.WriteString("\n" + styleBold.Render("  Explanation:") + "\n")
		b.WriteString(styleExplain.Render(wrapAndIndent(sanitizeExplanation(q.Rationale.Correct), cw, "  ")) + "\n")
	}

	// Controls.
	m.writeFooter(&b, footerReview)

	return b.String()
}

// viewReviewBrowse renders the read-only review of wrong answers.
// No new attempts are recorded — just browse with explanations.
func (m model) viewReviewBrowse() string {
	var b strings.Builder

	if len(m.reviewQueue) == 0 {
		b.WriteString("  No wrong answers to review.\n")
		return b.String()
	}

	wa := m.reviewQueue[m.reviewCursor]
	q := &wa.Question
	cw := m.contentWidth(2)

	header := fmt.Sprintf("golearn — Review (%d/%d)", m.reviewCursor+1, len(m.reviewQueue))
	b.WriteString(styleHeader.Render(header) + "\n")
	b.WriteString("════════════════════════\n\n")

	// Type hint.
	typeHint := "Single select"
	if q.Type == "multi_select" {
		typeHint = "Multi select"
	}
	b.WriteString(fmt.Sprintf("  [%s]\n\n", typeHint))

	// Intro.
	if q.Intro != "" {
		b.WriteString(wrapAndIndent(q.Intro, cw, "  ") + "\n\n")
	}

	// Prompt.
	b.WriteString(styleBold.Render(wrapAndIndent(q.Prompt, cw, "  ")) + "\n\n")

	// Build lookup sets.
	correctSet := make(map[string]bool, len(q.CorrectChoiceIDs))
	for _, id := range q.CorrectChoiceIDs {
		correctSet[id] = true
	}
	selectedSet := make(map[string]bool, len(wa.SelectedIDs))
	for _, id := range wa.SelectedIDs {
		selectedSet[id] = true
	}

	// Show choices with feedback.
	const choicePad = 8
	choiceCW := m.contentWidth(choicePad)
	contIndent := "        "

	for _, c := range q.Choices {
		isCorrect := correctSet[c.ID]
		wasSelected := selectedSet[c.ID]

		var marker string
		var choiceStyle *lipgloss.Style
		switch {
		case isCorrect:
			marker = styleCorrect.Render("✔")
			s := styleCorrect
			choiceStyle = &s
		case wasSelected:
			marker = styleIncorrect.Render("✘")
			s := styleIncorrect
			choiceStyle = &s
		default:
			marker = " "
			choiceStyle = nil
		}

		indicator := " "
		if wasSelected {
			indicator = styleSelected.Render("▸")
		}

		choiceText := fmt.Sprintf("%s) %s", m.displayLabelForChoiceID(c.ID), c.Text)
		wrappedLines := strings.Split(wrapText(choiceText, choiceCW), "\n")

		for li, l := range wrappedLines {
			if choiceStyle != nil {
				l = choiceStyle.Render(l)
			}
			if li == 0 {
				b.WriteString(fmt.Sprintf("  %s %s %s\n", indicator, marker, l))
			} else {
				b.WriteString(contIndent + l + "\n")
			}
		}

		// Per-choice explanation.
		if m.showExplanations && q.Rationale.PerChoice != nil {
			if explanation, ok := q.Rationale.PerChoice[c.ID]; ok {
				expCW := m.contentWidth(7)
				formatted := domain.FormatChoiceExplanation(c.ID, explanation, q.CorrectChoiceIDs)
				b.WriteString(styleExplain.Render(wrapAndIndent(formatted, expCW, "       ")) + "\n")
			}
		}
	}

	// Correct explanation.
	if m.showExplanations && q.Rationale.Correct != "" {
		b.WriteString("\n" + styleBold.Render("  Explanation:") + "\n")
		b.WriteString(styleExplain.Render(wrapAndIndent(sanitizeExplanation(q.Rationale.Correct), cw, "  ")) + "\n")
	}

	// Controls.
	m.writeFooter(&b, footerReview)

	return b.String()
}
