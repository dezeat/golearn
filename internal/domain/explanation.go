// Package domain — explanation.go provides helpers for rendering
// per-choice explanation text with correctness prefixes.
//
// Packs store content-only explanations (no "Correct:" / "Incorrect:" prefix).
// The UI applies the prefix dynamically based on correct_choice_ids.
package domain

import "strings"

// StripExplanationPrefix removes a leading "Correct:", "Incorrect:", "✅", or "❌"
// prefix from explanation text. This ensures backward compatibility with packs
// that were authored before the content-only convention.
func StripExplanationPrefix(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	prefixes := []string{"correct:", "incorrect:", "✅", "❌"}
	lower := strings.ToLower(s)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			s = strings.TrimSpace(s[len(p):])
			break
		}
	}
	return s
}

// FormatChoiceExplanation returns the explanation text for a choice
// with the appropriate "Correct: " or "Incorrect: " prefix applied.
// correctIDs is the set of correct choice IDs for the question.
func FormatChoiceExplanation(choiceID string, text string, correctIDs []string) string {
	stripped := StripExplanationPrefix(text)
	if stripped == "" {
		return ""
	}
	isCorrect := false
	for _, cid := range correctIDs {
		if cid == choiceID {
			isCorrect = true
			break
		}
	}
	if isCorrect {
		return "Correct: " + stripped
	}
	return "Incorrect: " + stripped
}
