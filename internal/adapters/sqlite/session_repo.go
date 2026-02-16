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

	res, err := r.db.Exec(
		`INSERT INTO sessions (topic_id, mode, requested_n, started_at)
		 VALUES (?, ?, ?, ?)`,
		s.TopicID, s.Mode, s.RequestedN, startedAt,
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
