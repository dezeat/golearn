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
) (selected []domain.Question, note string) {
	if n <= 0 || len(questions) == 0 {
		return nil, ""
	}

	// Filter to the chosen difficulty bucket.
	filtered := make([]domain.Question, 0, len(questions))
	for i := range questions {
		q := questions[i]
		if string(q.Difficulty) == difficulty {
			filtered = append(filtered, q)
		}
	}

	if len(filtered) == 0 {
		return nil, "No questions available for difficulty '" + difficulty + "'."
	}
	if len(filtered) < n {
		note = formatDifficultyNote(len(filtered), difficulty)
		n = len(filtered)
	}

	selected = SelectQuestions(filtered, stats, n, rng)
	return selected, note
}

func formatDifficultyNote(available int, difficulty string) string {
	if available == 1 {
		return "Only 1 question available for difficulty '" + difficulty + "'. Running 1 question."
	}
	return "Only " + strconv.Itoa(available) + " questions available for difficulty '" + difficulty + "'. Running " + strconv.Itoa(available) + " questions."
}
