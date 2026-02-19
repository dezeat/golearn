package tui

import (
	"strings"

	"github.com/dezeat/golearn/internal/domain"
)

func (m *model) setDisplayLabelMapping(choices []domain.Choice) {
	m.displayLabelByChoiceID = make(map[string]string, len(choices))
	m.choiceIDByDisplayLabel = make(map[string]string, len(choices))

	for i, c := range choices {
		label := domain.DisplayLabelForIndex(i)
		m.displayLabelByChoiceID[c.ID] = label
		m.choiceIDByDisplayLabel[label] = c.ID
	}
}

func (m model) displayLabelForChoiceID(choiceID string) string {
	if m.displayLabelByChoiceID == nil {
		return choiceID
	}
	if label, ok := m.displayLabelByChoiceID[choiceID]; ok {
		return label
	}
	return choiceID
}

func (m model) choiceIDForDisplayLabel(label string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(label))
	if normalized == "" || m.choiceIDByDisplayLabel == nil {
		return "", false
	}
	choiceID, ok := m.choiceIDByDisplayLabel[normalized]
	return choiceID, ok
}

func sanitizeExplanation(raw string) string {
	return domain.StripExplanationPrefix(raw)
}
