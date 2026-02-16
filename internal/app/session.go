// Package app — session.go implements the session lifecycle use cases:
// StartSession, GetNextQuestion, RecordAttempt, EndSession.
//
// The SessionEngine holds in-memory state for the current session's
// selected question queue and cursor position.
package app

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// SessionEngine manages the lifecycle of a practice session.
type SessionEngine struct {
	topics    ports.TopicRepository
	questions ports.QuestionRepository
	sessions  ports.SessionRepository
	attempts  ports.AttemptRepository

	// In-memory state for the active session.
	sessionID int64
	queue     []domain.Question // ordered list of selected questions
	cursor    int               // index of the next question to serve
	rng       *rand.Rand
}

// NewSessionEngine creates a session engine with the given dependencies.
// If rng is nil, a time-seeded source is used.
func NewSessionEngine(
	topics ports.TopicRepository,
	questions ports.QuestionRepository,
	sessions ports.SessionRepository,
	attempts ports.AttemptRepository,
	rng *rand.Rand,
) *SessionEngine {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &SessionEngine{
		topics:    topics,
		questions: questions,
		sessions:  sessions,
		attempts:  attempts,
		rng:       rng,
	}
}

// StartSession validates the topic, selects questions, persists a session
// row, and returns the session ID. mode should be "practice".
func (e *SessionEngine) StartSession(topicSlug string, n int, mode string) (int64, error) {
	if mode == "" {
		mode = "practice"
	}

	// Resolve topic by slug.
	topics, err := e.topics.List()
	if err != nil {
		return 0, fmt.Errorf("list topics: %w", err)
	}
	var topic *domain.Topic
	for i := range topics {
		if topics[i].Slug == topicSlug {
			topic = &topics[i]
			break
		}
	}
	if topic == nil {
		return 0, fmt.Errorf("topic %q not found", topicSlug)
	}

	// Load all questions for the topic.
	allQuestions, err := e.questions.ListByTopic(topic.ID)
	if err != nil {
		return 0, fmt.Errorf("list questions for topic %q: %w", topicSlug, err)
	}
	if len(allQuestions) == 0 {
		return 0, fmt.Errorf("topic %q has no questions", topicSlug)
	}

	// Load attempt stats to inform selection policy.
	stats, err := e.attempts.StatsByTopic(topic.ID)
	if err != nil {
		return 0, fmt.Errorf("load attempt stats: %w", err)
	}

	// Select questions using the prioritisation policy.
	e.queue = SelectQuestions(allQuestions, stats, n, e.rng)
	e.cursor = 0

	// Persist the session row.
	sess := &domain.Session{
		TopicID:    topic.ID,
		Mode:       mode,
		RequestedN: n,
		StartedAt:  time.Now().UTC(),
	}
	id, err := e.sessions.Create(sess)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}
	e.sessionID = id
	return id, nil
}

// GetNextQuestion returns the next question in the queue, or nil when
// all questions have been served.
func (e *SessionEngine) GetNextQuestion() *domain.Question {
	if e.cursor >= len(e.queue) {
		return nil
	}
	q := &e.queue[e.cursor]
	e.cursor++
	return q
}

// RecordAttempt evaluates correctness and persists the attempt.
func (e *SessionEngine) RecordAttempt(
	questionID int64,
	selectedChoiceIDs []string,
	skipped bool,
	latencyMs int,
) (bool, error) {
	// Find the question to check correctness against.
	var correctIDs []string
	for i := range e.queue {
		if e.queue[i].ID == questionID {
			correctIDs = e.queue[i].CorrectChoiceIDs
			break
		}
	}

	correct := false
	if !skipped && correctIDs != nil {
		correct = domain.EvaluateCorrectness(selectedChoiceIDs, correctIDs)
	}

	attempt := &domain.Attempt{
		SessionID:         e.sessionID,
		QuestionID:        questionID,
		SelectedChoiceIDs: selectedChoiceIDs,
		Correct:           correct,
		Skipped:           skipped,
		LatencyMs:         latencyMs,
	}
	if err := e.attempts.Record(attempt); err != nil {
		return false, fmt.Errorf("record attempt: %w", err)
	}
	return correct, nil
}

// EndSession marks the session as finished.
func (e *SessionEngine) EndSession() error {
	if e.sessionID == 0 {
		return nil
	}
	return e.sessions.Finish(e.sessionID)
}

// QueueLength returns the total number of questions selected for this session.
func (e *SessionEngine) QueueLength() int {
	return len(e.queue)
}
