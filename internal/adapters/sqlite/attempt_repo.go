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
		`INSERT INTO attempts (user_id, session_id, question_id, selected_choice_ids_json, correct, skipped, latency_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.UserID, a.SessionID, a.QuestionID, string(selectedJSON), correct, skipped, a.LatencyMs, createdAt,
	)
	if err != nil {
		return fmt.Errorf("insert attempt: %w", err)
	}
	return nil
}

// StatsByTopic returns per-question attempt statistics for all questions
// belonging to a topic. The map is keyed by question ID.
// Wrong count = total attempts - correct attempts (skips count as wrong).
func (r *AttemptRepo) StatsByTopic(userID, topicID int64) (map[int64]ports.QuestionStats, error) {
	rows, err := r.db.Query(`
		SELECT a.question_id,
		       COUNT(*) AS attempts,
		       COUNT(*) - SUM(a.correct) AS wrong
		FROM attempts a
		JOIN questions q ON q.id = a.question_id
		WHERE a.user_id = ? AND q.topic_id = ?
		GROUP BY a.question_id`, userID, topicID)
	if err != nil {
		return nil, fmt.Errorf("stats by user %d topic %d: %w", userID, topicID, err)
	}
	defer func() { _ = rows.Close() }()

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
