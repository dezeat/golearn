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

// GlobalStats holds aggregated metrics across all topics for a user.
type GlobalStats struct {
	TotalAnswered      int
	TotalSkipped       int
	AccuracyPct        float64 // excludes skipped
	AvgLatencySeconds  float64 // excludes skipped
	TotalTimeSeconds   float64 // sum of all latency
	MostPracticedTopic string  // topic name, by attempt count
	WeakestTopic       string  // topic name, by lowest accuracy (min attempts threshold)
}

// TopicSummary holds per-topic stats for a user.
type TopicSummary struct {
	TopicID           int64
	TopicName         string
	TotalQuestions    int
	SeenQuestions     int
	CoveragePct       float64
	AttemptsAnswered  int
	AttemptsSkipped   int
	AccuracyPct       float64
	AvgLatencySeconds float64
	LastPracticedAt   string // formatted datetime or empty
}

// DifficultyBucket labels for stats breakdown.
const (
	DifficultyEasy    = "Easy"
	DifficultyMedium  = "Medium"
	DifficultyHard    = "Hard"
	DifficultyUnrated = "Unrated"
)

// DifficultyStat holds stats for one difficulty bucket.
type DifficultyStat struct {
	Bucket            string
	AttemptsAnswered  int
	AccuracyPct       float64
	AvgLatencySeconds float64
}

// TagStat holds stats for one question tag.
type TagStat struct {
	Tag              string
	AttemptsAnswered int
	AccuracyPct      float64
}

// QuestionWeakStat holds weak-question stats for ranking.
type QuestionWeakStat struct {
	QuestionID       int64
	PromptPreview    string
	AttemptsAnswered int
	WrongRate        float64
	LastAttemptAt    string
}

// StatsRepository provides user-scoped aggregated statistics.
type StatsRepository interface {
	// GlobalStats returns overall metrics for a user.
	GlobalStats(userID int64) (*GlobalStats, error)

	// TopicSummary returns per-topic stats for a user.
	TopicSummary(userID, topicID int64) (*TopicSummary, error)

	// TopicSummaries returns stats for all topics for a user.
	TopicSummaries(userID int64) ([]TopicSummary, error)

	// DifficultyStats returns per-difficulty-bucket stats for a user+topic.
	DifficultyStats(userID, topicID int64) ([]DifficultyStat, error)

	// TagStats returns per-tag stats for a user+topic (minAttempts filter).
	TagStats(userID, topicID int64, minAttempts int) ([]TagStat, error)

	// WeakQuestions returns worst-performing questions for a user+topic.
	WeakQuestions(userID, topicID int64, minAttempts, limit int) ([]QuestionWeakStat, error)

	// SessionTrend returns accuracy per session for the last N sessions.
	SessionTrend(userID, topicID int64, limitN int) ([]float64, error)

	// SessionTrendGlobal returns accuracy per session across all topics for last N sessions.
	SessionTrendGlobal(userID int64, limitN int) ([]float64, error)
}
