package sqlite

import (
	"database/sql"
	"fmt"

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
	const minForWeak = 5
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

// TopicSummaries returns stats for all topics for a user.
func (r *StatsRepo) TopicSummaries(userID int64) ([]ports.TopicSummary, error) {
	rows, err := r.db.Query(`SELECT id FROM topics ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan topic id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summaries := make([]ports.TopicSummary, 0, len(ids))
	for _, id := range ids {
		ts, err := r.TopicSummary(userID, id)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, *ts)
	}
	return summaries, nil
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
	defer rows.Close()

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
			ds.AvgLatencySeconds = ds.AvgLatencySeconds / float64(ds.AttemptsAnswered)
		}
		result = append(result, *ds)
	}
	return result, nil
}

// TagStats returns per-tag stats for a user+topic, filtering by minAttempts.
func (r *StatsRepo) TagStats(userID, topicID int64, minAttempts int) ([]ports.TagStat, error) {
	// Tags are stored as JSON arrays. We need to join tags with attempts.
	// Since SQLite doesn't have native JSON array unnest in all versions,
	// we load question IDs + tags in Go and aggregate.
	type qTag struct {
		questionID int64
		tag        string
	}

	rows, err := r.db.Query(`
		SELECT id, tags_json FROM questions WHERE topic_id = ?`, topicID)
	if err != nil {
		return nil, fmt.Errorf("load question tags: %w", err)
	}
	defer rows.Close()

	// Build tag -> []questionID mapping.
	tagQuestions := map[string][]int64{}
	for rows.Next() {
		var qID int64
		var tagsJSON string
		if err := rows.Scan(&qID, &tagsJSON); err != nil {
			return nil, fmt.Errorf("scan question tag: %w", err)
		}
		tags := parseJSONStringArray(tagsJSON)
		for _, tag := range tags {
			if tag != "" {
				tagQuestions[tag] = append(tagQuestions[tag], qID)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(tagQuestions) == 0 {
		return nil, nil
	}

	// For each tag, query attempts for its questions.
	var result []ports.TagStat
	for tag, qIDs := range tagQuestions {
		var answered, correct int
		for _, qID := range qIDs {
			var a, c int
			err := r.db.QueryRow(`
				SELECT COUNT(*), COALESCE(SUM(correct), 0)
				FROM attempts
				WHERE user_id = ? AND question_id = ? AND skipped = 0`,
				userID, qID).Scan(&a, &c)
			if err != nil {
				return nil, fmt.Errorf("tag stat query: %w", err)
			}
			answered += a
			correct += c
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
	sortTagStats(result)
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
	defer rows.Close()

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
func (r *StatsRepo) SessionTrend(userID, topicID int64, limitN int) ([]float64, error) {
	rows, err := r.db.Query(`
		SELECT s.id, s.started_at
		FROM sessions s
		WHERE s.user_id = ? AND s.topic_id = ? AND s.ended_at IS NOT NULL
		ORDER BY s.started_at DESC
		LIMIT ?`, userID, topicID, limitN)
	if err != nil {
		return nil, fmt.Errorf("session trend query: %w", err)
	}
	defer rows.Close()

	type sessionRef struct {
		id int64
	}
	var refs []sessionRef
	for rows.Next() {
		var sr sessionRef
		var startedAt string
		if err := rows.Scan(&sr.id, &startedAt); err != nil {
			return nil, fmt.Errorf("scan session ref: %w", err)
		}
		refs = append(refs, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to get chronological order.
	for i, j := 0, len(refs)-1; i < j; i, j = i+1, j-1 {
		refs[i], refs[j] = refs[j], refs[i]
	}

	// Compute accuracy per session.
	trend := make([]float64, 0, len(refs))
	for _, sr := range refs {
		var answered, correct int
		err := r.db.QueryRow(`
			SELECT COUNT(*), COALESCE(SUM(correct), 0)
			FROM attempts
			WHERE user_id = ? AND session_id = ? AND skipped = 0`,
			userID, sr.id).Scan(&answered, &correct)
		if err != nil {
			return nil, fmt.Errorf("session accuracy: %w", err)
		}
		if answered > 0 {
			trend = append(trend, float64(correct)/float64(answered)*100)
		}
	}
	return trend, nil
}

// SessionTrendGlobal returns accuracy per session across all topics for last N sessions.
func (r *StatsRepo) SessionTrendGlobal(userID int64, limitN int) ([]float64, error) {
	rows, err := r.db.Query(`
		SELECT s.id
		FROM sessions s
		WHERE s.user_id = ? AND s.ended_at IS NOT NULL
		ORDER BY s.started_at DESC
		LIMIT ?`, userID, limitN)
	if err != nil {
		return nil, fmt.Errorf("global trend query: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse to chronological order.
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}

	trend := make([]float64, 0, len(ids))
	for _, id := range ids {
		var answered, correct int
		err := r.db.QueryRow(`
			SELECT COUNT(*), COALESCE(SUM(correct), 0)
			FROM attempts
			WHERE user_id = ? AND session_id = ? AND skipped = 0`,
			userID, id).Scan(&answered, &correct)
		if err != nil {
			return nil, fmt.Errorf("session accuracy: %w", err)
		}
		if answered > 0 {
			trend = append(trend, float64(correct)/float64(answered)*100)
		}
	}
	return trend, nil
}

// parseJSONStringArray is a simple JSON string array parser.
func parseJSONStringArray(s string) []string {
	if s == "" || s == "[]" || s == "null" {
		return nil
	}
	var result []string
	// Simple parsing: strip brackets, split by comma, strip quotes.
	s = trimBrackets(s)
	if s == "" {
		return nil
	}
	for _, item := range splitJSON(s) {
		item = trimQuotes(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func trimBrackets(s string) string {
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1]
	}
	return s
}

func trimQuotes(s string) string {
	s = trimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func splitJSON(s string) []string {
	var parts []string
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func sortTagStats(stats []ports.TagStat) {
	// Sort by accuracy ascending (weakest first).
	for i := 1; i < len(stats); i++ {
		for j := i; j > 0 && stats[j].AccuracyPct < stats[j-1].AccuracyPct; j-- {
			stats[j], stats[j-1] = stats[j-1], stats[j]
		}
	}
}
