// Package ports defines the interfaces that adapters must implement.
package ports

import "github.com/dezeat/golearn/internal/domain"

// TopicRepository persists and retrieves topics.
type TopicRepository interface {
	// UpsertBySlug creates a topic or returns the existing one matching the slug.
	UpsertBySlug(slug, name string) (*domain.Topic, error)

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
