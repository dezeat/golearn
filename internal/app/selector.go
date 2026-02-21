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

// Package app — selector.go implements the question selection policy.
//
// Strategy (MVP):
//  1. Unseen questions (zero prior attempts) — shuffled
//  2. Weak questions (highest wrong rate) — sorted descending
//  3. Random fill from remaining — shuffled
//
// No duplicates within a session. Capped at n.
// Uses a supplied random source for test determinism.
package app

import (
	"math/rand"
	"sort"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// SelectQuestions picks up to n questions for a session, prioritizing
// unseen and weak questions. rng controls shuffle order (use a seeded
// *rand.Rand for deterministic tests).
func SelectQuestions(
	questions []domain.Question,
	stats map[int64]ports.QuestionStats,
	n int,
	rng *rand.Rand,
) []domain.Question {
	if n <= 0 || len(questions) == 0 {
		return nil
	}

	// Partition questions into buckets.
	var unseen, weak, rest []domain.Question
	for i := range questions {
		q := questions[i]
		s, found := stats[q.ID]
		switch {
		case !found || s.Attempts == 0:
			unseen = append(unseen, q)
		case s.Wrong > 0:
			weak = append(weak, q)
		default:
			rest = append(rest, q)
		}
	}

	// Shuffle unseen so we don't always start with the same ones.
	rng.Shuffle(len(unseen), func(i, j int) {
		unseen[i], unseen[j] = unseen[j], unseen[i]
	})

	// Sort weak by wrong rate descending (highest error rate first).
	sort.Slice(weak, func(i, j int) bool {
		si := stats[weak[i].ID]
		sj := stats[weak[j].ID]
		rateI := float64(si.Wrong) / float64(si.Attempts)
		rateJ := float64(sj.Wrong) / float64(sj.Attempts)
		return rateI > rateJ
	})

	// Shuffle the rest for variety.
	rng.Shuffle(len(rest), func(i, j int) {
		rest[i], rest[j] = rest[j], rest[i]
	})

	// Concatenate buckets: unseen → weak → rest.
	selected := make([]domain.Question, 0, n)
	for _, bucket := range [][]domain.Question{unseen, weak, rest} {
		for i := range bucket {
			q := bucket[i]
			if len(selected) >= n {
				return selected
			}
			selected = append(selected, q)
		}
	}
	return selected
}
