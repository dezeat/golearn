// Package app — selector_difficulty.go implements the By Difficulty selection mode.
//
// Filters candidate questions to the chosen difficulty bucket, then applies
// the Balanced selector logic (unseen → weak → random fill) within that set.
package app

import (
	"math/rand"
	"strconv"

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
	return "Only " + strconv.Itoa(available) + " questions available for difficulty '" + difficulty + "'. Running " + strconv.Itoa(available) + " questions."
}
