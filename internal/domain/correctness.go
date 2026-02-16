// Package domain — correctness.go centralises answer evaluation logic.
// Correctness is an exact, order-insensitive set match between
// the user's selected choice IDs and the question's correct_choice_ids.
package domain

import "sort"

// EvaluateCorrectness returns true when selectedIDs exactly match
// correctIDs (order-insensitive). A skipped attempt is never correct.
func EvaluateCorrectness(selectedIDs, correctIDs []string) bool {
	if len(selectedIDs) != len(correctIDs) {
		return false
	}
	// Sort copies so callers' slices are not mutated.
	s := make([]string, len(selectedIDs))
	copy(s, selectedIDs)
	sort.Strings(s)

	c := make([]string, len(correctIDs))
	copy(c, correctIDs)
	sort.Strings(c)

	for i := range s {
		if s[i] != c[i] {
			return false
		}
	}
	return true
}
