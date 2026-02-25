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

// Package app — session.go implements the session lifecycle use cases:
// StartSession, GetNextSessionQuestion, RecordAttempt, EndSession.
//
// The SessionEngine holds in-memory state for the current session's
// selected question queue and cursor position.
package app

import (
	"encoding/json"
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

type sessionQuestionSnapshot struct {
	QuestionID int64    `json:"question_id"`
	ChoiceIDs  []string `json:"choice_ids"`
}

type persistedSessionState struct {
	ModeParams
	ModeNote string                    `json:"mode_note,omitempty"`
	Queue    []sessionQuestionSnapshot `json:"queue,omitempty"`
}

type sessionRuntime struct {
	queue      []SessionQuestion
	cursor     int
	activeMode SelectionMode
	modeParams ModeParams
	modeNote   string
}

// SessionEngine manages the lifecycle of a practice session.
type SessionEngine struct {
	topics    ports.TopicRepository
	questions ports.QuestionRepository
	sessions  ports.SessionRepository
	attempts  ports.AttemptRepository
	stats     ports.StatsRepository // optional; needed for weakest-by-tag
	userCtx   CurrentUserProvider

	rng *rand.Rand

	activeSessionID int64
	states          map[int64]*sessionRuntime
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
		states:    make(map[int64]*sessionRuntime),
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
	case ModeRandom:
		selected = SelectRandomQuestions(allQuestions, cfg.N, e.rng)

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

	queue := make([]SessionQuestion, 0, len(selected))
	for i := range selected {
		q := &selected[i]
		shuffled := make([]domain.Choice, len(q.Choices))
		copy(shuffled, q.Choices)
		e.rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		queue = append(queue, SessionQuestion{
			Question:        q,
			ShuffledChoices: shuffled,
		})
	}

	persisted := persistedSessionState{
		ModeParams: modeParams,
		ModeNote:   modeNote,
		Queue:      make([]sessionQuestionSnapshot, 0, len(queue)),
	}
	for i := range queue {
		choiceIDs := make([]string, 0, len(queue[i].ShuffledChoices))
		for j := range queue[i].ShuffledChoices {
			choiceIDs = append(choiceIDs, queue[i].ShuffledChoices[j].ID)
		}
		persisted.Queue = append(persisted.Queue, sessionQuestionSnapshot{
			QuestionID: queue[i].Question.ID,
			ChoiceIDs:  choiceIDs,
		})
	}
	modeParamsJSON := "{}"
	if b, err := json.Marshal(persisted); err == nil {
		modeParamsJSON = string(b)
	}

	// Persist the session row.
	sess := &domain.Session{
		UserID:         e.userCtx.CurrentUserID(),
		TopicID:        topic.ID,
		Mode:           string(mode),
		ModeParamsJSON: modeParamsJSON,
		RequestedN:     cfg.N,
		StartedAt:      time.Now().UTC(),
	}
	id, err := e.sessions.Create(sess)
	if err != nil {
		return 0, fmt.Errorf("create session: %w", err)
	}
	e.states[id] = &sessionRuntime{
		queue:      queue,
		cursor:     0,
		activeMode: mode,
		modeParams: modeParams,
		modeNote:   modeNote,
	}
	e.activeSessionID = id
	return id, nil
}

// LoadSession loads a persisted session into engine memory and sets it active.
// It enables resuming sessions across engine instances.
func (e *SessionEngine) LoadSession(sessionID int64) error {
	runtime, err := e.loadSessionRuntime(sessionID)
	if err != nil {
		return err
	}
	e.states[sessionID] = runtime
	e.activeSessionID = sessionID
	return nil
}

// GetNextSessionQuestionForSession returns next question for a specific session.
func (e *SessionEngine) GetNextSessionQuestionForSession(sessionID int64) (*SessionQuestion, error) {
	runtime, err := e.ensureState(sessionID)
	if err != nil {
		return nil, err
	}
	if runtime.cursor >= len(runtime.queue) {
		return nil, nil
	}
	q := &runtime.queue[runtime.cursor]
	runtime.cursor++
	return q, nil
}

// GetNextSessionQuestion returns the next session question in the queue,
// or nil when all questions have been served.
func (e *SessionEngine) GetNextSessionQuestion() *SessionQuestion {
	if e.activeSessionID == 0 {
		return nil
	}
	q, err := e.GetNextSessionQuestionForSession(e.activeSessionID)
	if err != nil {
		return nil
	}
	return q
}

// RecordAttemptForSession evaluates correctness and persists an attempt for a specific session.
func (e *SessionEngine) RecordAttemptForSession(
	sessionID int64,
	questionID int64,
	selectedChoiceIDs []string,
	skipped bool,
	latencyMs int,
) (bool, error) {
	runtime, err := e.ensureState(sessionID)
	if err != nil {
		return false, err
	}

	var correctIDs []string
	for i := range runtime.queue {
		if runtime.queue[i].Question != nil && runtime.queue[i].Question.ID == questionID {
			correctIDs = runtime.queue[i].Question.CorrectChoiceIDs
			break
		}
	}

	correct := false
	if !skipped && correctIDs != nil {
		correct = domain.EvaluateCorrectness(selectedChoiceIDs, correctIDs)
	}

	attempt := &domain.Attempt{
		UserID:            e.userCtx.CurrentUserID(),
		SessionID:         sessionID,
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

// RecordAttempt evaluates correctness and persists the attempt.
func (e *SessionEngine) RecordAttempt(
	questionID int64,
	selectedChoiceIDs []string,
	skipped bool,
	latencyMs int,
) (bool, error) {
	if e.activeSessionID == 0 {
		return false, fmt.Errorf("no active session")
	}
	return e.RecordAttemptForSession(e.activeSessionID, questionID, selectedChoiceIDs, skipped, latencyMs)
}

// EndSessionByID marks a specific session as finished and unloads in-memory state.
func (e *SessionEngine) EndSessionByID(sessionID int64) error {
	if sessionID == 0 {
		return nil
	}
	if err := e.sessions.Finish(sessionID); err != nil {
		return err
	}
	delete(e.states, sessionID)
	if e.activeSessionID == sessionID {
		e.activeSessionID = 0
	}
	return nil
}

// EndSession marks the session as finished.
func (e *SessionEngine) EndSession() error {
	return e.EndSessionByID(e.activeSessionID)
}

// QueueLengthForSession returns question count for a specific session.
func (e *SessionEngine) QueueLengthForSession(sessionID int64) (int, error) {
	runtime, err := e.ensureState(sessionID)
	if err != nil {
		return 0, err
	}
	return len(runtime.queue), nil
}

// QueueLength returns the total number of questions selected for this session.
func (e *SessionEngine) QueueLength() int {
	if e.activeSessionID == 0 {
		return 0
	}
	runtime, ok := e.states[e.activeSessionID]
	if !ok || runtime == nil {
		return 0
	}
	return len(runtime.queue)
}

// ActiveMode returns the resolved selection mode for the current session.
func (e *SessionEngine) ActiveMode() SelectionMode {
	runtime, ok := e.states[e.activeSessionID]
	if !ok || runtime == nil {
		return ModeBalanced
	}
	return runtime.activeMode
}

// ActiveModeParams returns the resolved mode parameters for the current session.
func (e *SessionEngine) ActiveModeParams() ModeParams {
	runtime, ok := e.states[e.activeSessionID]
	if !ok || runtime == nil {
		return ModeParams{}
	}
	return runtime.modeParams
}

// ModeNote returns any informational note from the selector (e.g., reduced N).
func (e *SessionEngine) ModeNote() string {
	runtime, ok := e.states[e.activeSessionID]
	if !ok || runtime == nil {
		return ""
	}
	return runtime.modeNote
}

func (e *SessionEngine) ensureState(sessionID int64) (*sessionRuntime, error) {
	if sessionID == 0 {
		return nil, fmt.Errorf("session ID is required")
	}
	if runtime, ok := e.states[sessionID]; ok && runtime != nil {
		return runtime, nil
	}
	runtime, err := e.loadSessionRuntime(sessionID)
	if err != nil {
		return nil, err
	}
	e.states[sessionID] = runtime
	return runtime, nil
}

func (e *SessionEngine) loadSessionRuntime(sessionID int64) (*sessionRuntime, error) {
	session, err := e.sessions.GetByID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load session %d: %w", sessionID, err)
	}
	if session == nil {
		return nil, fmt.Errorf("session %d not found", sessionID)
	}

	var persisted persistedSessionState
	if err := json.Unmarshal([]byte(session.ModeParamsJSON), &persisted); err != nil {
		persisted = persistedSessionState{}
	}
	if len(persisted.Queue) == 0 {
		return nil, fmt.Errorf("session %d has no persisted queue", sessionID)
	}

	questions, err := e.questions.ListByTopic(session.TopicID)
	if err != nil {
		return nil, fmt.Errorf("load questions for session %d topic %d: %w", sessionID, session.TopicID, err)
	}
	questionByID := make(map[int64]domain.Question, len(questions))
	for i := range questions {
		questionByID[questions[i].ID] = questions[i]
	}

	queue := make([]SessionQuestion, 0, len(persisted.Queue))
	for i := range persisted.Queue {
		snap := persisted.Queue[i]
		q, ok := questionByID[snap.QuestionID]
		if !ok {
			continue
		}
		choiceByID := make(map[string]domain.Choice, len(q.Choices))
		for j := range q.Choices {
			choiceByID[q.Choices[j].ID] = q.Choices[j]
		}

		shuffled := make([]domain.Choice, 0, len(q.Choices))
		for j := range snap.ChoiceIDs {
			if c, found := choiceByID[snap.ChoiceIDs[j]]; found {
				shuffled = append(shuffled, c)
			}
		}
		if len(shuffled) != len(q.Choices) {
			shuffled = make([]domain.Choice, len(q.Choices))
			copy(shuffled, q.Choices)
		}

		qq := q
		queue = append(queue, SessionQuestion{
			Question:        &qq,
			ShuffledChoices: shuffled,
		})
	}

	if len(queue) == 0 {
		return nil, fmt.Errorf("session %d has an empty restored queue", sessionID)
	}

	attemptsCount, err := e.attempts.CountBySession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("count attempts for session %d: %w", sessionID, err)
	}
	if attemptsCount > len(queue) {
		attemptsCount = len(queue)
	}

	mode := SelectionMode(session.Mode)
	if !ValidModes[mode] {
		mode = ModeBalanced
	}

	return &sessionRuntime{
		queue:      queue,
		cursor:     attemptsCount,
		activeMode: mode,
		modeParams: persisted.ModeParams,
		modeNote:   persisted.ModeNote,
	}, nil
}
