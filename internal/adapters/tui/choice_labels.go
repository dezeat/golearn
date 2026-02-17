package tui

import (
	"strings"

	"github.com/dezeat/golearn/internal/domain"
)

func (m *model) setDisplayLabelMapping(choices []domain.Choice) {
	m.displayLabelByChoiceID = make(map[string]string, len(choices))
	m.choiceIDByDisplayLabel = make(map[string]string, len(choices))

	for i, c := range choices {
		label := displayLabelForIndex(i)
		m.displayLabelByChoiceID[c.ID] = label
		m.choiceIDByDisplayLabel[label] = c.ID
	}
}

func displayLabelForIndex(index int) string {
	if index < 0 {
		return ""
	}

	value := index + 1
	label := ""
	for value > 0 {
		value--
		label = string(rune('A'+(value%26))) + label
		value /= 26
	}
	return label
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
	explanation := strings.TrimSpace(raw)
	if explanation == "" {
		return ""
	}

	prefixes := []string{"correct:", "incorrect:", "✅", "❌"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(explanation), prefix) {
			explanation = strings.TrimSpace(explanation[len(prefix):])
			break
		}
	}
	return explanation
}
