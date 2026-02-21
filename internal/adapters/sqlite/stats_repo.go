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

package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dezeat/golearn/internal/ports"
)

// StatsRepo implements ports.StatsRepository using SQLite aggregation queries.
type StatsRepo struct {
	db *sql.DB
}

// NewStatsRepo creates a StatsRepo.
func NewStatsRepo(db *sql.DB) *StatsRepo {
	return &StatsRepo{db: db}
}

// GlobalStats returns user-scoped aggregate metrics across all topics.
func (r *StatsRepo) GlobalStats(userID int64) (*ports.GlobalStats, error) {
	gs := &ports.GlobalStats{}

	// Total answered (not skipped) and total skipped.
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN skipped = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN skipped = 1 THEN 1 ELSE 0 END), 0)
		FROM attempts WHERE user_id = ?`, userID).Scan(&gs.TotalAnswered, &gs.TotalSkipped)
	if err != nil {
		return nil, fmt.Errorf("global counts: %w", err)
	}

	// Accuracy among answered (non-skipped).
	if gs.TotalAnswered > 0 {
		var totalCorrect int
		err = r.db.QueryRow(`
			SELECT COALESCE(SUM(correct), 0)
			FROM attempts WHERE user_id = ? AND skipped = 0`, userID).Scan(&totalCorrect)
		if err != nil {
			return nil, fmt.Errorf("global accuracy: %w", err)
		}
		gs.AccuracyPct = float64(totalCorrect) / float64(gs.TotalAnswered) * 100
	}

	// Average latency and total time (non-skipped only).
	if gs.TotalAnswered > 0 {
		var totalLatencyMs int64
		err = r.db.QueryRow(`
			SELECT COALESCE(SUM(latency_ms), 0)
			FROM attempts WHERE user_id = ? AND skipped = 0`, userID).Scan(&totalLatencyMs)
		if err != nil {
			return nil, fmt.Errorf("global latency: %w", err)
		}
		gs.TotalTimeSeconds = float64(totalLatencyMs) / 1000.0
		gs.AvgLatencySeconds = gs.TotalTimeSeconds / float64(gs.TotalAnswered)
	}

	// Most practiced topic (by total non-skipped attempts).
	err = r.db.QueryRow(`
		SELECT t.name FROM attempts a
		JOIN questions q ON q.id = a.question_id
		JOIN topics t ON t.id = q.topic_id
		WHERE a.user_id = ? AND a.skipped = 0
		GROUP BY t.id
		ORDER BY COUNT(*) DESC LIMIT 1`, userID).Scan(&gs.MostPracticedTopic)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("most practiced: %w", err)
	}

	// Weakest topic (lowest accuracy, minimum 5 answered attempts).
	const minForWeak = 5 // minimum attempts to rank as weakest/strongest
	err = r.db.QueryRow(`
		SELECT t.name FROM attempts a
		JOIN questions q ON q.id = a.question_id
		JOIN topics t ON t.id = q.topic_id
		WHERE a.user_id = ? AND a.skipped = 0
		GROUP BY t.id
		HAVING COUNT(*) >= ?
		ORDER BY (CAST(SUM(a.correct) AS REAL) / COUNT(*)) ASC
		LIMIT 1`, userID, minForWeak).Scan(&gs.WeakestTopic)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("weakest topic: %w", err)
	}

	// Strongest topic (highest accuracy, minimum 5 answered attempts).
	err = r.db.QueryRow(`
		SELECT t.name FROM attempts a
		JOIN questions q ON q.id = a.question_id
		JOIN topics t ON t.id = q.topic_id
		WHERE a.user_id = ? AND a.skipped = 0
		GROUP BY t.id
		HAVING COUNT(*) >= ?
		ORDER BY (CAST(SUM(a.correct) AS REAL) / COUNT(*)) DESC
		LIMIT 1`, userID, minForWeak).Scan(&gs.StrongestTopic)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("strongest topic: %w", err)
	}

	return gs, nil
}

// TopicSummary returns per-topic stats for a user.
func (r *StatsRepo) TopicSummary(userID, topicID int64) (*ports.TopicSummary, error) {
	ts := &ports.TopicSummary{TopicID: topicID}

	// Topic name and total questions.
	err := r.db.QueryRow(`SELECT name FROM topics WHERE id = ?`, topicID).Scan(&ts.TopicName)
	if err != nil {
		return nil, fmt.Errorf("topic name: %w", err)
	}

	err = r.db.QueryRow(`SELECT COUNT(*) FROM questions WHERE topic_id = ?`, topicID).Scan(&ts.TotalQuestions)
	if err != nil {
		return nil, fmt.Errorf("total questions: %w", err)
	}

	// Seen questions (distinct question_id attempted by user).
	err = r.db.QueryRow(`
		SELECT COUNT(DISTINCT a.question_id)
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ?`, userID, topicID).Scan(&ts.SeenQuestions)
	if err != nil {
		return nil, fmt.Errorf("seen questions: %w", err)
	}
	if ts.TotalQuestions > 0 {
		ts.CoveragePct = float64(ts.SeenQuestions) / float64(ts.TotalQuestions) * 100
	}

	// Answered and skipped.
	err = r.db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN a.skipped = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN a.skipped = 1 THEN 1 ELSE 0 END), 0)
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ?`, userID, topicID).Scan(&ts.AttemptsAnswered, &ts.AttemptsSkipped)
	if err != nil {
		return nil, fmt.Errorf("topic answered/skipped: %w", err)
	}

	// Accuracy.
	if ts.AttemptsAnswered > 0 {
		var correct int
		err = r.db.QueryRow(`
			SELECT COALESCE(SUM(a.correct), 0)
			FROM attempts a
			JOIN questions q ON q.id = a.question_id
			WHERE a.user_id = ? AND q.topic_id = ? AND a.skipped = 0`, userID, topicID).Scan(&correct)
		if err != nil {
			return nil, fmt.Errorf("topic accuracy: %w", err)
		}
		ts.AccuracyPct = float64(correct) / float64(ts.AttemptsAnswered) * 100
	}

	// Avg latency.
	if ts.AttemptsAnswered > 0 {
		var totalLatencyMs int64
		err = r.db.QueryRow(`
			SELECT COALESCE(SUM(a.latency_ms), 0)
			FROM attempts a
			JOIN questions q ON q.id = a.question_id
			WHERE a.user_id = ? AND q.topic_id = ? AND a.skipped = 0`, userID, topicID).Scan(&totalLatencyMs)
		if err != nil {
			return nil, fmt.Errorf("topic latency: %w", err)
		}
		ts.AvgLatencySeconds = float64(totalLatencyMs) / 1000.0 / float64(ts.AttemptsAnswered)
	}

	// Last practiced at.
	var lastAt sql.NullString
	err = r.db.QueryRow(`
		SELECT MAX(a.created_at)
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ?`, userID, topicID).Scan(&lastAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("last practiced: %w", err)
	}
	if lastAt.Valid {
		ts.LastPracticedAt = lastAt.String
	}

	return ts, nil
}

// TopicSummaries returns stats for all topics for a user using a single
// aggregate query instead of per-topic lookups (eliminates N+1 pattern).
func (r *StatsRepo) TopicSummaries(userID int64) ([]ports.TopicSummary, error) {
	rows, err := r.db.Query(`
		SELECT
			t.id,
			t.name,
			(SELECT COUNT(*) FROM questions WHERE topic_id = t.id) AS total_questions,
			COALESCE((
				SELECT COUNT(DISTINCT a.question_id)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id
			), 0) AS seen_questions,
			COALESCE((
				SELECT SUM(CASE WHEN a.skipped = 0 THEN 1 ELSE 0 END)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id
			), 0) AS answered,
			COALESCE((
				SELECT SUM(CASE WHEN a.skipped = 1 THEN 1 ELSE 0 END)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id
			), 0) AS skipped,
			COALESCE((
				SELECT SUM(a.correct)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id AND a.skipped = 0
			), 0) AS correct,
			COALESCE((
				SELECT SUM(a.latency_ms)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id AND a.skipped = 0
			), 0) AS total_latency_ms,
			(
				SELECT MAX(a.created_at)
				FROM attempts a
				JOIN questions q ON q.id = a.question_id
				WHERE a.user_id = ? AND q.topic_id = t.id
			) AS last_practiced
		FROM topics t
		ORDER BY t.slug`,
		userID, userID, userID, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("topic summaries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var summaries []ports.TopicSummary
	for rows.Next() {
		var ts ports.TopicSummary
		var correct, totalLatencyMs int64
		var lastAt sql.NullString
		if err := rows.Scan(
			&ts.TopicID, &ts.TopicName, &ts.TotalQuestions,
			&ts.SeenQuestions, &ts.AttemptsAnswered, &ts.AttemptsSkipped,
			&correct, &totalLatencyMs, &lastAt,
		); err != nil {
			return nil, fmt.Errorf("scan topic summary: %w", err)
		}
		if ts.TotalQuestions > 0 {
			ts.CoveragePct = float64(ts.SeenQuestions) / float64(ts.TotalQuestions) * 100
		}
		if ts.AttemptsAnswered > 0 {
			ts.AccuracyPct = float64(correct) / float64(ts.AttemptsAnswered) * 100
			ts.AvgLatencySeconds = float64(totalLatencyMs) / 1000.0 / float64(ts.AttemptsAnswered)
		}
		if lastAt.Valid {
			ts.LastPracticedAt = lastAt.String
		}
		summaries = append(summaries, ts)
	}
	return summaries, rows.Err()
}

// difficultyBucket maps a question's string difficulty to a display label.
func difficultyBucket(d string) string {
	switch d {
	case "easy":
		return ports.DifficultyEasy
	case "medium":
		return ports.DifficultyMedium
	case "hard":
		return ports.DifficultyHard
	default:
		return ports.DifficultyUnrated
	}
}

// DifficultyStats returns per-difficulty-bucket stats for a user+topic.
func (r *StatsRepo) DifficultyStats(userID, topicID int64) ([]ports.DifficultyStat, error) {
	rows, err := r.db.Query(`
		SELECT
			q.difficulty,
			COUNT(*) AS answered,
			COALESCE(SUM(a.correct), 0) AS correct_count,
			COALESCE(SUM(a.latency_ms), 0) AS total_latency_ms
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ? AND a.skipped = 0
		GROUP BY q.difficulty
		ORDER BY q.difficulty`, userID, topicID)
	if err != nil {
		return nil, fmt.Errorf("difficulty stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Accumulate into buckets.
	buckets := map[string]*ports.DifficultyStat{}
	for rows.Next() {
		var diff string
		var answered, correctCount int
		var totalLatencyMs int64
		if err := rows.Scan(&diff, &answered, &correctCount, &totalLatencyMs); err != nil {
			return nil, fmt.Errorf("scan difficulty: %w", err)
		}

		bucket := difficultyBucket(diff)
		ds, ok := buckets[bucket]
		if !ok {
			ds = &ports.DifficultyStat{Bucket: bucket}
			buckets[bucket] = ds
		}
		ds.AttemptsAnswered += answered
		// We'll recompute accuracy below after accumulation.
		ds.AccuracyPct += float64(correctCount)                  // temp: store raw correct count
		ds.AvgLatencySeconds += float64(totalLatencyMs) / 1000.0 // temp: store total seconds
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Finalize percentages.
	order := []string{ports.DifficultyEasy, ports.DifficultyMedium, ports.DifficultyHard, ports.DifficultyUnrated}
	var result []ports.DifficultyStat
	for _, name := range order {
		ds, ok := buckets[name]
		if !ok {
			continue
		}
		correctCount := ds.AccuracyPct // was stored as raw correct count
		if ds.AttemptsAnswered > 0 {
			ds.AccuracyPct = correctCount / float64(ds.AttemptsAnswered) * 100
			ds.AvgLatencySeconds /= float64(ds.AttemptsAnswered)
		}
		result = append(result, *ds)
	}
	return result, nil
}

// TagStats returns per-tag stats for a user+topic, filtering by minAttempts.
func (r *StatsRepo) TagStats(userID, topicID int64, minAttempts int) ([]ports.TagStat, error) {
	// Tags are stored as JSON arrays. Since SQLite doesn't have native
	// JSON array unnest in all versions, we load question tags in Go,
	// then fetch per-question attempt stats in a single aggregate query.

	// Step 1: load question → tags mapping.
	tagRows, err := r.db.Query(`
		SELECT id, tags_json FROM questions WHERE topic_id = ?`, topicID)
	if err != nil {
		return nil, fmt.Errorf("load question tags: %w", err)
	}
	defer func() { _ = tagRows.Close() }()

	tagQuestions := map[string][]int64{}
	for tagRows.Next() {
		var qID int64
		var tagsJSON string
		if err := tagRows.Scan(&qID, &tagsJSON); err != nil {
			return nil, fmt.Errorf("scan question tag: %w", err)
		}
		for _, tag := range parseJSONStringArray(tagsJSON) {
			if tag != "" {
				tagQuestions[tag] = append(tagQuestions[tag], qID)
			}
		}
	}
	if err := tagRows.Err(); err != nil {
		return nil, err
	}
	if len(tagQuestions) == 0 {
		return nil, nil
	}

	// Step 2: fetch per-question attempt stats in a single query.
	type qStat struct {
		answered int
		correct  int
	}
	statsByQ := map[int64]qStat{}

	statRows, err := r.db.Query(`
		SELECT a.question_id, COUNT(*), COALESCE(SUM(a.correct), 0)
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ? AND a.skipped = 0
		GROUP BY a.question_id`, userID, topicID)
	if err != nil {
		return nil, fmt.Errorf("tag stats batch query: %w", err)
	}
	defer func() { _ = statRows.Close() }()

	for statRows.Next() {
		var qID int64
		var a, c int
		if err := statRows.Scan(&qID, &a, &c); err != nil {
			return nil, fmt.Errorf("scan tag stat: %w", err)
		}
		statsByQ[qID] = qStat{answered: a, correct: c}
	}
	if err := statRows.Err(); err != nil {
		return nil, err
	}

	// Step 3: aggregate per tag.
	var result []ports.TagStat
	for tag, qIDs := range tagQuestions {
		var answered, correct int
		for _, qID := range qIDs {
			s := statsByQ[qID]
			answered += s.answered
			correct += s.correct
		}
		if answered < minAttempts {
			continue
		}
		ts := ports.TagStat{
			Tag:              tag,
			AttemptsAnswered: answered,
		}
		if answered > 0 {
			ts.AccuracyPct = float64(correct) / float64(answered) * 100
		}
		result = append(result, ts)
	}

	// Sort by accuracy ascending (weakest first).
	sort.Slice(result, func(i, j int) bool {
		return result[i].AccuracyPct < result[j].AccuracyPct
	})
	return result, nil
}

// WeakQuestions returns the worst-performing questions for a user+topic.
func (r *StatsRepo) WeakQuestions(userID, topicID int64, minAttempts, limit int) ([]ports.QuestionWeakStat, error) {
	rows, err := r.db.Query(`
		SELECT
			a.question_id,
			q.prompt,
			COUNT(*) AS answered,
			CAST(COUNT(*) - SUM(a.correct) AS REAL) / COUNT(*) AS wrong_rate,
			MAX(a.created_at) AS last_at
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ? AND a.skipped = 0
		GROUP BY a.question_id
		HAVING COUNT(*) >= ? AND (COUNT(*) - SUM(a.correct)) > 0
		ORDER BY wrong_rate DESC, answered DESC
		LIMIT ?`, userID, topicID, minAttempts, limit)
	if err != nil {
		return nil, fmt.Errorf("weak questions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []ports.QuestionWeakStat
	for rows.Next() {
		var ws ports.QuestionWeakStat
		if err := rows.Scan(&ws.QuestionID, &ws.PromptPreview, &ws.AttemptsAnswered, &ws.WrongRate, &ws.LastAttemptAt); err != nil {
			return nil, fmt.Errorf("scan weak question: %w", err)
		}
		result = append(result, ws)
	}
	return result, rows.Err()
}

// SessionTrend returns accuracy per session for the last N sessions of a topic.
// Uses a single aggregate query with GROUP BY instead of per-session lookups.
func (r *StatsRepo) SessionTrend(userID, topicID int64, limitN int) ([]float64, error) {
	rows, err := r.db.Query(`
		SELECT
			COUNT(*) AS answered,
			COALESCE(SUM(a.correct), 0) AS correct
		FROM attempts a
		JOIN sessions s ON s.id = a.session_id
		WHERE a.user_id = ? AND s.topic_id = ? AND s.ended_at IS NOT NULL AND a.skipped = 0
		  AND s.id IN (
			SELECT s2.id FROM sessions s2
			WHERE s2.user_id = ? AND s2.topic_id = ? AND s2.ended_at IS NOT NULL
			ORDER BY s2.started_at DESC LIMIT ?
		  )
		GROUP BY s.id
		ORDER BY s.started_at ASC`, userID, topicID, userID, topicID, limitN)
	if err != nil {
		return nil, fmt.Errorf("session trend query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var trend []float64
	for rows.Next() {
		var answered, correct int
		if err := rows.Scan(&answered, &correct); err != nil {
			return nil, fmt.Errorf("scan session trend: %w", err)
		}
		if answered > 0 {
			trend = append(trend, float64(correct)/float64(answered)*100)
		}
	}
	return trend, rows.Err()
}

// SessionTrendGlobal returns accuracy per session across all topics for last N sessions.
// Uses a single aggregate query with GROUP BY instead of per-session lookups.
func (r *StatsRepo) SessionTrendGlobal(userID int64, limitN int) ([]float64, error) {
	rows, err := r.db.Query(`
		SELECT
			COUNT(*) AS answered,
			COALESCE(SUM(a.correct), 0) AS correct
		FROM attempts a
		JOIN sessions s ON s.id = a.session_id
		WHERE a.user_id = ? AND s.ended_at IS NOT NULL AND a.skipped = 0
		  AND s.id IN (
			SELECT s2.id FROM sessions s2
			WHERE s2.user_id = ? AND s2.ended_at IS NOT NULL
			ORDER BY s2.started_at DESC LIMIT ?
		  )
		GROUP BY s.id
		ORDER BY s.started_at ASC`, userID, userID, limitN)
	if err != nil {
		return nil, fmt.Errorf("global trend query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var trend []float64
	for rows.Next() {
		var answered, correct int
		if err := rows.Scan(&answered, &correct); err != nil {
			return nil, fmt.Errorf("scan global trend: %w", err)
		}
		if answered > 0 {
			trend = append(trend, float64(correct)/float64(answered)*100)
		}
	}
	return trend, rows.Err()
}

// parseJSONStringArray unmarshals a JSON-encoded string array.
func parseJSONStringArray(s string) []string {
	if s == "" || s == "[]" || s == "null" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil
	}
	return result
}
