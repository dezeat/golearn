// Package app — selector_difficulty.go implements the By Difficulty selection mode.
//
// Filters candidate questions to the chosen difficulty bucket, then applies
// the Balanced selector logic (unseen → weak → random fill) within that set.
package app

import (
	"math/rand"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// SelectByDifficulty picks up to n questions matching the given difficulty,
// then applies Balanced selection within that filtered set.
// Returns the selected questions and a note string (non-empty if N was reduced).
func SelectByDifficulty(
	questions []domain.Question,
	stats map[int64]ports.QuestionStats,
	n int,
	difficulty string,
	rng *rand.Rand,
) ([]domain.Question, string) {
	if n <= 0 || len(questions) == 0 {
		return nil, ""
	}

	// Filter to the chosen difficulty bucket.
	filtered := make([]domain.Question, 0, len(questions))
	for _, q := range questions {
		if string(q.Difficulty) == difficulty {
			filtered = append(filtered, q)
		}
	}

	note := ""
	if len(filtered) == 0 {
		return nil, "No questions available for difficulty '" + difficulty + "'."
	}
	if len(filtered) < n {
		note = formatDifficultyNote(len(filtered), difficulty)
		n = len(filtered)
	}

	selected := SelectQuestions(filtered, stats, n, rng)
	return selected, note
}

func formatDifficultyNote(available int, difficulty string) string {
	if available == 1 {
		return "Only 1 question available for difficulty '" + difficulty + "'. Running 1 question."
	}
	return "Only " + itoa(available) + " questions available for difficulty '" + difficulty + "'. Running " + itoa(available) + " questions."
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if negative {
		digits = append(digits, '-')
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
