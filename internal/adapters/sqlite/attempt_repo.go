package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// AttemptRepo implements ports.AttemptRepository using SQLite.
type AttemptRepo struct {
	db *sql.DB
}

// NewAttemptRepo creates an AttemptRepo.
func NewAttemptRepo(db *sql.DB) *AttemptRepo {
	return &AttemptRepo{db: db}
}

// Record inserts a single attempt row.
func (r *AttemptRepo) Record(a *domain.Attempt) error {
	selectedJSON, err := json.Marshal(a.SelectedChoiceIDs)
	if err != nil {
		return fmt.Errorf("marshal selected_choice_ids: %w", err)
	}

	createdAt := a.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	correct := 0
	if a.Correct {
		correct = 1
	}
	skipped := 0
	if a.Skipped {
		skipped = 1
	}

	_, err = r.db.Exec(
		`INSERT INTO attempts (session_id, question_id, selected_choice_ids_json, correct, skipped, latency_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.SessionID, a.QuestionID, string(selectedJSON), correct, skipped, a.LatencyMs, createdAt,
	)
	if err != nil {
		return fmt.Errorf("insert attempt: %w", err)
	}
	return nil
}

// StatsByTopic returns per-question attempt statistics for all questions
// belonging to a topic. The map is keyed by question ID.
// Wrong count = total attempts - correct attempts (skips count as wrong).
func (r *AttemptRepo) StatsByTopic(topicID int64) (map[int64]ports.QuestionStats, error) {
	rows, err := r.db.Query(`
		SELECT a.question_id,
		       COUNT(*) AS attempts,
		       COUNT(*) - SUM(a.correct) AS wrong
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE q.topic_id = ?
		GROUP BY a.question_id`, topicID)
	if err != nil {
		return nil, fmt.Errorf("stats by topic %d: %w", topicID, err)
	}
	defer rows.Close()

	stats := make(map[int64]ports.QuestionStats)
	for rows.Next() {
		var s ports.QuestionStats
		if err := rows.Scan(&s.QuestionID, &s.Attempts, &s.Wrong); err != nil {
			return nil, fmt.Errorf("scan attempt stats: %w", err)
		}
		stats[s.QuestionID] = s
	}
	return stats, rows.Err()
}
