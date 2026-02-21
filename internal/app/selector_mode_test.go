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

package app

import (
	"math/rand"
	"testing"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

func makeQuestionWithDifficulty(id int64, diff domain.Difficulty) domain.Question {
	q := makeQuestion(id)
	q.Difficulty = diff
	return q
}

func makeQuestionWithTag(id int64, tags []string) domain.Question {
	q := makeQuestion(id)
	q.Tags = tags
	return q
}

// ─── Difficulty mode tests ───

func TestSelectByDifficulty_FiltersCorrectly(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithDifficulty(1, domain.DifficultyEasy),
		makeQuestionWithDifficulty(2, domain.DifficultyMedium),
		makeQuestionWithDifficulty(3, domain.DifficultyHard),
		makeQuestionWithDifficulty(4, domain.DifficultyEasy),
		makeQuestionWithDifficulty(5, domain.DifficultyHard),
	}
	stats := map[int64]ports.QuestionStats{}
	rng := rand.New(rand.NewSource(42))

	selected, note := SelectByDifficulty(questions, stats, 5, "easy", rng)
	if len(selected) != 2 {
		t.Fatalf("expected 2 easy questions, got %d", len(selected))
	}
	if note == "" {
		t.Fatal("expected note about reduced N, got empty")
	}

	// All selected should be easy.
	for _, q := range selected {
		if q.Difficulty != domain.DifficultyEasy {
			t.Errorf("expected easy difficulty, got %q", q.Difficulty)
		}
	}
}

func TestSelectByDifficulty_ReducesN(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithDifficulty(1, domain.DifficultyHard),
		makeQuestionWithDifficulty(2, domain.DifficultyEasy),
	}
	stats := map[int64]ports.QuestionStats{}
	rng := rand.New(rand.NewSource(42))

	selected, note := SelectByDifficulty(questions, stats, 10, "hard", rng)
	if len(selected) != 1 {
		t.Fatalf("expected 1 hard question, got %d", len(selected))
	}
	if note == "" {
		t.Error("expected a note about reduced count")
	}
}

func TestSelectByDifficulty_NoBucket(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithDifficulty(1, domain.DifficultyEasy),
	}
	stats := map[int64]ports.QuestionStats{}
	rng := rand.New(rand.NewSource(42))

	selected, note := SelectByDifficulty(questions, stats, 5, "hard", rng)
	if len(selected) != 0 {
		t.Fatalf("expected 0 questions for empty bucket, got %d", len(selected))
	}
	if note == "" {
		t.Error("expected a note about no questions")
	}
}

func TestSelectByDifficulty_AppliesBalancedWithinBucket(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithDifficulty(1, domain.DifficultyMedium), // unseen
		makeQuestionWithDifficulty(2, domain.DifficultyMedium), // seen+wrong (weak)
		makeQuestionWithDifficulty(3, domain.DifficultyMedium), // seen+correct (rest)
	}
	stats := map[int64]ports.QuestionStats{
		2: {QuestionID: 2, Attempts: 5, Wrong: 4},
		3: {QuestionID: 3, Attempts: 5, Wrong: 0},
	}
	rng := rand.New(rand.NewSource(42))

	selected, _ := SelectByDifficulty(questions, stats, 3, "medium", rng)
	if len(selected) != 3 {
		t.Fatalf("expected 3, got %d", len(selected))
	}
	// Unseen (Q1) should be first, weak (Q2) second, rest (Q3) third.
	if selected[0].ID != 1 {
		t.Errorf("expected unseen Q1 first, got ID=%d", selected[0].ID)
	}
	if selected[1].ID != 2 {
		t.Errorf("expected weak Q2 second, got ID=%d", selected[1].ID)
	}
	if selected[2].ID != 3 {
		t.Errorf("expected rest Q3 third, got ID=%d", selected[2].ID)
	}
}

func TestSelectByDifficulty_Deterministic(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithDifficulty(1, domain.DifficultyEasy),
		makeQuestionWithDifficulty(2, domain.DifficultyEasy),
		makeQuestionWithDifficulty(3, domain.DifficultyEasy),
	}
	stats := map[int64]ports.QuestionStats{}

	rng1 := rand.New(rand.NewSource(99))
	rng2 := rand.New(rand.NewSource(99))

	s1, _ := SelectByDifficulty(questions, stats, 3, "easy", rng1)
	s2, _ := SelectByDifficulty(questions, stats, 3, "easy", rng2)

	for i := range s1 {
		if s1[i].ID != s2[i].ID {
			t.Errorf("position %d: ID %d vs %d — not deterministic", i, s1[i].ID, s2[i].ID)
		}
	}
}

// ─── Weakest by Questions tests ───

func TestSelectWeakestByQuestions_OrderByWrongRate(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
		makeQuestion(3),
		makeQuestion(4),
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 10, Wrong: 2}, // 20%
		2: {QuestionID: 2, Attempts: 10, Wrong: 8}, // 80%
		3: {QuestionID: 3, Attempts: 10, Wrong: 5}, // 50%
		4: {QuestionID: 4, Attempts: 10, Wrong: 0}, // 0% (no wrong)
	}
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByQuestions(questions, stats, 3, 3, rng)
	if len(result.Questions) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(result.Questions))
	}

	// Q2 (80%), Q3 (50%), Q1 (20%) — sorted by wrong rate desc.
	if result.Questions[0].ID != 2 {
		t.Errorf("expected Q2 (highest wrong rate) first, got ID=%d", result.Questions[0].ID)
	}
	if result.Questions[1].ID != 3 {
		t.Errorf("expected Q3 second, got ID=%d", result.Questions[1].ID)
	}
	if result.Questions[2].ID != 1 {
		t.Errorf("expected Q1 third, got ID=%d", result.Questions[2].ID)
	}
}

func TestSelectWeakestByQuestions_FillsWithBalanced(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
		makeQuestion(3),
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 5, Wrong: 3}, // only 1 weak
	}
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByQuestions(questions, stats, 3, 3, rng)
	if len(result.Questions) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(result.Questions))
	}

	// First should be Q1 (weak), remaining filled with balanced.
	if result.Questions[0].ID != 1 {
		t.Errorf("expected Q1 (weak) first, got ID=%d", result.Questions[0].ID)
	}
	if result.Note == "" {
		t.Error("expected a note about balanced fill")
	}
}

func TestSelectWeakestByQuestions_MinAttemptsFilter(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 2, Wrong: 2}, // below minAttempts=3
		2: {QuestionID: 2, Attempts: 5, Wrong: 3}, // meets threshold
	}
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByQuestions(questions, stats, 2, 3, rng)
	if len(result.Questions) != 2 {
		t.Fatalf("expected 2, got %d", len(result.Questions))
	}

	// Q2 is the only weak candidate; Q1 should be filled by balanced.
	if result.Questions[0].ID != 2 {
		t.Errorf("expected Q2 (meets threshold) first, got ID=%d", result.Questions[0].ID)
	}
}

// ─── Weakest by Tag tests ───

func TestSelectWeakestByTag_ChoosesLowestAccuracy(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithTag(1, []string{"streaming"}),
		makeQuestionWithTag(2, []string{"streaming"}),
		makeQuestionWithTag(3, []string{"delta"}),
		makeQuestionWithTag(4, []string{"delta"}),
	}
	stats := map[int64]ports.QuestionStats{}
	tagStats := []ports.TagStat{
		{Tag: "streaming", AttemptsAnswered: 10, AccuracyPct: 40},
		{Tag: "delta", AttemptsAnswered: 10, AccuracyPct: 80},
	}
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByTag(questions, stats, tagStats, 2, rng)
	if result.Tag != "streaming" {
		t.Errorf("expected weakest tag 'streaming', got %q", result.Tag)
	}
	if len(result.Questions) != 2 {
		t.Fatalf("expected 2 questions from streaming tag, got %d", len(result.Questions))
	}

	// All selected should have the streaming tag.
	for _, q := range result.Questions {
		hasTag := false
		for _, tag := range q.Tags {
			if tag == "streaming" {
				hasTag = true
			}
		}
		if !hasTag {
			t.Errorf("question ID=%d does not have streaming tag", q.ID)
		}
	}
}

func TestSelectWeakestByTag_FallbackToQuestions(t *testing.T) {
	questions := []domain.Question{
		makeQuestion(1),
		makeQuestion(2),
	}
	stats := map[int64]ports.QuestionStats{
		1: {QuestionID: 1, Attempts: 10, Wrong: 8},
	}
	// No tag stats available.
	var tagStats []ports.TagStat
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByTag(questions, stats, tagStats, 2, rng)
	if result.Tag != "" {
		t.Errorf("expected empty tag (fallback), got %q", result.Tag)
	}
	if len(result.Questions) != 2 {
		t.Fatalf("expected 2 questions (fallback), got %d", len(result.Questions))
	}
	if result.Note == "" {
		t.Error("expected fallback note")
	}
}

func TestSelectWeakestByTag_TieBreakByAttempts(t *testing.T) {
	questions := []domain.Question{
		makeQuestionWithTag(1, []string{"alpha"}),
		makeQuestionWithTag(2, []string{"beta"}),
	}
	stats := map[int64]ports.QuestionStats{}
	tagStats := []ports.TagStat{
		{Tag: "alpha", AttemptsAnswered: 5, AccuracyPct: 50},
		{Tag: "beta", AttemptsAnswered: 10, AccuracyPct: 50}, // tie: higher attempts wins
	}
	rng := rand.New(rand.NewSource(42))

	result := SelectWeakestByTag(questions, stats, tagStats, 1, rng)
	if result.Tag != "beta" {
		t.Errorf("expected 'beta' (higher attempts tie-break), got %q", result.Tag)
	}
}

// ─── Mode label tests ───

func TestModeLabel(t *testing.T) {
	tests := []struct {
		mode   SelectionMode
		params ModeParams
		want   string
	}{
		{ModeBalanced, ModeParams{}, "Mode: Balanced"},
		{ModeByDifficulty, ModeParams{Difficulty: "hard"}, "Mode: Difficulty (hard)"},
		{ModeWeakest, ModeParams{WeakestTag: "streaming"}, "Mode: Weakest (tag: streaming)"},
		{ModeWeakest, ModeParams{}, "Mode: Weakest (questions)"},
	}
	for _, tc := range tests {
		got := ModeLabel(tc.mode, tc.params)
		if got != tc.want {
			t.Errorf("ModeLabel(%q, %+v) = %q, want %q", tc.mode, tc.params, got, tc.want)
		}
	}
}
