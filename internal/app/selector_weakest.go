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

// Package app — selector_weakest.go implements the Weakest selection mode.
//
// Two sub-modes:
//   - Weakest by Tag: find the tag with lowest accuracy, select questions
//     from that tag using Balanced logic within the tag set.
//   - Weakest by Questions: rank all questions by wrong rate and select
//     the weakest ones directly. Falls back to Balanced fill if not enough.
package app

import (
	"math/rand"
	"sort"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// DefaultMinAttempts is the minimum number of attempts a question must
// have before it is considered for weakest-question selection.
const DefaultMinAttempts = 3

// DefaultMinTagAttempts is the minimum number of attempts required for
// a tag to be included in tag-based stats and weakest-tag selection.
const DefaultMinTagAttempts = 5

// WeakestResult holds the output of a weakest selection.
type WeakestResult struct {
	Questions []domain.Question
	Note      string // informational note (e.g., fallback notice)
	Tag       string // the resolved weakest tag, if tag mode was used
}

// SelectWeakestByTag selects questions from the weakest tag.
// tagStats must already be filtered by minAttempts.
// If no valid tag is found, falls back to SelectWeakestByQuestions.
func SelectWeakestByTag(
	questions []domain.Question,
	stats map[int64]ports.QuestionStats,
	tagStats []ports.TagStat,
	n int,
	rng *rand.Rand,
) WeakestResult {
	if n <= 0 || len(questions) == 0 {
		return WeakestResult{}
	}

	// Find the weakest tag: lowest accuracy_pct, tie-break by higher attempts.
	weakestTag := findWeakestTag(tagStats)
	if weakestTag == "" {
		// Fallback to weakest by questions.
		result := SelectWeakestByQuestions(questions, stats, n, 3, rng)
		if result.Note == "" {
			result.Note = "Not enough tagged history — using weakest questions instead."
		} else {
			result.Note = "Not enough tagged history — using weakest questions instead. " + result.Note
		}
		return result
	}

	// Filter questions to those containing the weakest tag.
	tagSet := filterQuestionsByTag(questions, weakestTag)
	if len(tagSet) == 0 {
		result := SelectWeakestByQuestions(questions, stats, n, 3, rng)
		result.Note = "No questions found for tag '" + weakestTag + "' — using weakest questions instead."
		return result
	}

	// Apply Balanced selection within the tag set.
	if len(tagSet) < n {
		n = len(tagSet)
	}
	selected := SelectQuestions(tagSet, stats, n, rng)
	return WeakestResult{
		Questions: selected,
		Tag:       weakestTag,
	}
}

// SelectWeakestByQuestions selects questions ranked by wrong rate.
// Candidates must have attempts >= minAttempts.
// If not enough weak questions exist, fills remainder with Balanced selection.
func SelectWeakestByQuestions(
	questions []domain.Question,
	stats map[int64]ports.QuestionStats,
	n int,
	minAttempts int,
	rng *rand.Rand,
) WeakestResult {
	if n <= 0 || len(questions) == 0 {
		return WeakestResult{}
	}

	// Build weak candidates: questions with enough attempts and wrong > 0.
	type weakCandidate struct {
		question  domain.Question
		wrongRate float64
		attempts  int
		lastWrong int // we use Wrong as proxy since we don't have timestamps
	}

	var candidates []weakCandidate
	for i := range questions {
		q := questions[i]
		s, ok := stats[q.ID]
		if !ok || s.Attempts < minAttempts || s.Wrong == 0 {
			continue
		}
		wr := float64(s.Wrong) / float64(s.Attempts)
		candidates = append(candidates, weakCandidate{
			question:  q,
			wrongRate: wr,
			attempts:  s.Attempts,
			lastWrong: s.Wrong,
		})
	}

	// Sort: highest wrong_rate first, tie-break: more attempts first.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].wrongRate != candidates[j].wrongRate {
			return candidates[i].wrongRate > candidates[j].wrongRate
		}
		return candidates[i].attempts > candidates[j].attempts
	})

	// Take up to n weak candidates.
	selected := make([]domain.Question, 0, n)
	usedIDs := make(map[int64]bool)
	for i := range candidates {
		c := candidates[i]
		if len(selected) >= n {
			break
		}
		selected = append(selected, c.question)
		usedIDs[c.question.ID] = true
	}

	note := ""
	if len(selected) < n {
		// Fill remainder with Balanced selection from unused questions.
		remaining := make([]domain.Question, 0, len(questions)-len(selected))
		for i := range questions {
			q := questions[i]
			if !usedIDs[q.ID] {
				remaining = append(remaining, q)
			}
		}
		fill := SelectQuestions(remaining, stats, n-len(selected), rng)
		selected = append(selected, fill...)
		if len(fill) > 0 {
			note = "Not enough weak questions — filled remaining with balanced selection."
		}
	}

	return WeakestResult{
		Questions: selected,
		Note:      note,
	}
}

// findWeakestTag returns the tag with the lowest accuracy among those
// that pass the minimum attempts threshold. Tie-break: higher attempts.
func findWeakestTag(tagStats []ports.TagStat) string {
	if len(tagStats) == 0 {
		return ""
	}

	// tagStats are already sorted by accuracy ascending from the repo,
	// but we'll do our own deterministic selection.
	best := tagStats[0]
	for _, ts := range tagStats[1:] {
		if ts.AccuracyPct < best.AccuracyPct {
			best = ts
		} else if ts.AccuracyPct == best.AccuracyPct && ts.AttemptsAnswered > best.AttemptsAnswered {
			best = ts
		}
	}
	return best.Tag
}

// filterQuestionsByTag returns questions that contain the given tag.
func filterQuestionsByTag(questions []domain.Question, tag string) []domain.Question {
	var result []domain.Question
	for i := range questions {
		q := questions[i]
		for _, t := range q.Tags {
			if t == tag {
				result = append(result, q)
				break
			}
		}
	}
	return result
}
