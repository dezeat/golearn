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
	"fmt"
	"time"

	"github.com/dezeat/golearn/internal/domain"
)

// SessionRepo implements ports.SessionRepository using SQLite.
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo creates a SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session row and returns the auto-generated ID.
func (r *SessionRepo) Create(s *domain.Session) (int64, error) {
	startedAt := s.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	modeParams := s.ModeParamsJSON
	if modeParams == "" {
		modeParams = "{}"
	}

	res, err := r.db.Exec(
		`INSERT INTO sessions (user_id, topic_id, mode, mode_params_json, requested_n, started_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.UserID, s.TopicID, s.Mode, modeParams, s.RequestedN, startedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("session last insert id: %w", err)
	}
	return id, nil
}

// GetByID returns a session by ID, or nil if not found.
func (r *SessionRepo) GetByID(id int64) (*domain.Session, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, topic_id, mode, mode_params_json, requested_n, started_at, ended_at
		 FROM sessions WHERE id = ?`,
		id,
	)

	var s domain.Session
	var endedAt sql.NullTime
	if err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.TopicID,
		&s.Mode,
		&s.ModeParamsJSON,
		&s.RequestedN,
		&s.StartedAt,
		&endedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get session %d: %w", id, err)
	}
	if endedAt.Valid {
		t := endedAt.Time
		s.EndedAt = &t
	}

	return &s, nil
}

// Finish sets ended_at to now for the given session.
func (r *SessionRepo) Finish(id int64) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET ended_at = ? WHERE id = ?`,
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("finish session %d: %w", id, err)
	}
	return nil
}
