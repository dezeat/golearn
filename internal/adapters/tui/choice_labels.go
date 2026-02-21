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

func sanitizeExplanation(raw string) string {
	return domain.StripExplanationPrefix(raw)
}
