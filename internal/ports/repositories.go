// Package ports defines the interfaces that adapters must implement.
package ports

import "github.com/dezeat/golearn/internal/domain"

// UserRepository persists and retrieves local profiles.
type UserRepository interface {
	// Create inserts a user profile.
	Create(handle, displayName string) (*domain.User, error)

	// GetByHandle returns a user by handle.
	GetByHandle(handle string) (*domain.User, bool, error)

	// List returns all users ordered by handle.
	List() ([]domain.User, error)

	// GetByID returns a user by ID.
	GetByID(id int64) (*domain.User, bool, error)
}

// TopicRepository persists and retrieves topics.
type TopicRepository interface {
	// UpsertBySlug creates a topic or returns the existing one matching the slug.
	UpsertBySlug(slug, name string) (*domain.Topic, error)

	// GetBySlug returns a single topic by slug, or nil if not found.
	GetBySlug(slug string) (*domain.Topic, error)

	// List returns all topics ordered by slug.
	List() ([]domain.Topic, error)
}

// InsertResult reports the outcome of a batch question insert.
type InsertResult struct {
	Inserted   int
	Duplicates int
}

// QuestionRepository persists and retrieves questions.
type QuestionRepository interface {
	// InsertMany inserts questions, skipping those whose hash already exists.
	InsertMany(questions []domain.Question) (*InsertResult, error)

	// ListByTopic returns all questions for a given topic ID.
	ListByTopic(topicID int64) ([]domain.Question, error)
}

// SessionRepository persists practice sessions.
type SessionRepository interface {
	// Create inserts a new session and returns its ID.
	Create(session *domain.Session) (int64, error)

	// Finish marks a session as ended (sets ended_at).
	Finish(id int64) error
}

// QuestionStats holds aggregated attempt statistics for one question.
type QuestionStats struct {
	QuestionID int64
	Attempts   int
	Wrong      int
}

// AttemptRepository persists and queries attempts within sessions.
type AttemptRepository interface {
	// Record inserts a single attempt.
	Record(attempt *domain.Attempt) error

	// StatsByTopic returns per-question attempt stats for all
	// questions in a topic, keyed by question ID.
	StatsByTopic(userID, topicID int64) (map[int64]QuestionStats, error)
}
