// Package app — session.go implements the session lifecycle use cases:
// StartSession, GetNextSessionQuestion, RecordAttempt, EndSession.
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

// SessionQuestion holds session-scoped display state for one question.
// It preserves a pointer to the original question while allowing
// per-session shuffled presentation of choices.
type SessionQuestion struct {
	Question        *domain.Question
	ShuffledChoices []domain.Choice
}

// SessionEngine manages the lifecycle of a practice session.
type SessionEngine struct {
	topics    ports.TopicRepository
	questions ports.QuestionRepository
	sessions  ports.SessionRepository
	attempts  ports.AttemptRepository
	stats     ports.StatsRepository // optional; needed for weakest-by-tag
	userCtx   CurrentUserProvider

	// In-memory state for the active session.
	sessionID int64
	queue     []SessionQuestion // ordered list of selected questions
	cursor    int               // index of the next question to serve
	rng       *rand.Rand

	// Mode context for display.
	activeMode       SelectionMode
	activeModeParams ModeParams
	modeNote         string // informational note from selector
}

// NewSessionEngine creates a session engine with the given dependencies.
// If rng is nil, a time-seeded source is used.
func NewSessionEngine(
	topics ports.TopicRepository,
	questions ports.QuestionRepository,
	sessions ports.SessionRepository,
	attempts ports.AttemptRepository,
	userCtx CurrentUserProvider,
	rng *rand.Rand,
) *SessionEngine {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	if userCtx == nil {
		userCtx = NewUserContext(0)
	}
	return &SessionEngine{
		topics:    topics,
		questions: questions,
		sessions:  sessions,
		attempts:  attempts,
		userCtx:   userCtx,
		rng:       rng,
	}
}

// WithStatsRepo sets an optional stats repository for weakest-by-tag selection.
func (e *SessionEngine) WithStatsRepo(sr ports.StatsRepository) *SessionEngine {
	e.stats = sr
	return e
}

// StartSession validates the topic, selects questions, persists a session
// row, and returns the session ID. mode should be "balanced" (or legacy "practice").
// This method uses Balanced mode for backward compatibility.
func (e *SessionEngine) StartSession(topicSlug string, n int, mode string) (int64, error) {
	if mode == "" || mode == "practice" {
		mode = string(ModeBalanced)
	}
	return e.StartSessionWithConfig(SessionConfig{
		TopicSlug: topicSlug,
		N:         n,
		Mode:      SelectionMode(mode),
	})
}

// StartSessionWithConfig validates the topic, selects questions using the
// specified mode, persists a session row, and returns the session ID.
func (e *SessionEngine) StartSessionWithConfig(cfg SessionConfig) (int64, error) {
	mode := cfg.Mode
	if mode == "" || mode == "practice" {
		mode = ModeBalanced
	}
	if e.userCtx.CurrentUserID() <= 0 {
		return 0, fmt.Errorf("current user is not set")
	}

	// Resolve topic by slug.
	topic, err := e.topics.GetBySlug(cfg.TopicSlug)
	if err != nil {
		return 0, fmt.Errorf("get topic %q: %w", cfg.TopicSlug, err)
	}
	if topic == nil {
		return 0, fmt.Errorf("topic %q not found", cfg.TopicSlug)
	}

	// Load all questions for the topic.
	allQuestions, err := e.questions.ListByTopic(topic.ID)
	if err != nil {
		return 0, fmt.Errorf("list questions for topic %q: %w", cfg.TopicSlug, err)
	}
	if len(allQuestions) == 0 {
		return 0, fmt.Errorf("topic %q has no questions", cfg.TopicSlug)
	}

	// Load attempt stats to inform selection policy.
	stats, err := e.attempts.StatsByTopic(e.userCtx.CurrentUserID(), topic.ID)
	if err != nil {
		return 0, fmt.Errorf("load attempt stats: %w", err)
	}

	// Select questions using the configured mode.
	var selected []domain.Question
	modeParams := ModeParams{}
	modeNote := ""

	switch mode {
	case ModeByDifficulty:
		modeParams.Difficulty = cfg.Difficulty
		selected, modeNote = SelectByDifficulty(allQuestions, stats, cfg.N, cfg.Difficulty, e.rng)

	case ModeWeakest:
		modeParams.WeakestSub = cfg.WeakestSub
		if cfg.WeakestSub == WeakestByTag {
			// Load tag stats for weakest-by-tag selection.
			var tagStats []ports.TagStat
			if e.stats != nil {
				tagStats, _ = e.stats.TagStats(e.userCtx.CurrentUserID(), topic.ID, DefaultMinTagAttempts)
			}
			result := SelectWeakestByTag(allQuestions, stats, tagStats, cfg.N, e.rng)
			selected = result.Questions
			modeNote = result.Note
			modeParams.WeakestTag = result.Tag
		} else {
			result := SelectWeakestByQuestions(allQuestions, stats, cfg.N, DefaultMinAttempts, e.rng)
			selected = result.Questions
			modeNote = result.Note
		}

	default: // ModeBalanced
		mode = ModeBalanced
		selected = SelectQuestions(allQuestions, stats, cfg.N, e.rng)
	}

	if len(selected) == 0 {
		return 0, fmt.Errorf("no questions selected for topic %q with mode %s", cfg.TopicSlug, mode)
	}

	e.activeMode = mode
	e.activeModeParams = modeParams
	e.modeNote = modeNote

	e.queue = make([]SessionQuestion, 0, len(selected))
	for i := range selected {
		q := &selected[i]
		shuffled := make([]domain.Choice, len(q.Choices))
		copy(shuffled, q.Choices)
		e.rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		e.queue = append(e.queue, SessionQuestion{
			Question:        q,
			ShuffledChoices: shuffled,
		})
	}
	e.cursor = 0

	// Persist the session row.
	sess := &domain.Session{
		UserID:         e.userCtx.CurrentUserID(),
		TopicID:        topic.ID,
		Mode:           string(mode),
		ModeParamsJSON: ModeParamsJSON(modeParams),
		RequestedN:     cfg.N,
		StartedAt:      time.Now().UTC(),
	}
	id, err := e.sessions.Create(sess)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}
	e.sessionID = id
	return id, nil
}

// GetNextSessionQuestion returns the next session question in the queue,
// or nil when all questions have been served.
func (e *SessionEngine) GetNextSessionQuestion() *SessionQuestion {
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
		if e.queue[i].Question != nil && e.queue[i].Question.ID == questionID {
			correctIDs = e.queue[i].Question.CorrectChoiceIDs
			break
		}
	}

	correct := false
	if !skipped && correctIDs != nil {
		correct = domain.EvaluateCorrectness(selectedChoiceIDs, correctIDs)
	}

	attempt := &domain.Attempt{
		UserID:            e.userCtx.CurrentUserID(),
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

// ActiveMode returns the resolved selection mode for the current session.
func (e *SessionEngine) ActiveMode() SelectionMode {
	return e.activeMode
}

// ActiveModeParams returns the resolved mode parameters for the current session.
func (e *SessionEngine) ActiveModeParams() ModeParams {
	return e.activeModeParams
}

// ModeNote returns any informational note from the selector (e.g., reduced N).
func (e *SessionEngine) ModeNote() string {
	return e.modeNote
}
