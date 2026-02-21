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

package app_test

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
)

func choiceIDs(choices []domain.Choice) []string {
	ids := make([]string, len(choices))
	for i := range choices {
		ids[i] = choices[i].ID
	}
	return ids
}

// setupTestDB opens a temp SQLite DB, seeds a topic + questions, and returns
// all the repos plus the topic used for the test.
func setupTestDB(t *testing.T) (
	*sqlite.UserRepo,
	*sqlite.TopicRepo,
	*sqlite.QuestionRepo,
	*sqlite.SessionRepo,
	*sqlite.AttemptRepo,
	*domain.Topic,
	int64,
) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)
	userRepo := sqlite.NewUserRepo(db)
	localUser, found, err := userRepo.GetByHandle("local")
	if err != nil {
		t.Fatalf("GetByHandle(local): %v", err)
	}
	if !found || localUser == nil {
		t.Fatal("expected seeded local user")
	}

	topic, err := topicRepo.UpsertBySlug("test-topic", "Test Topic")
	if err != nil {
		t.Fatalf("UpsertBySlug: %v", err)
	}

	questions := []domain.Question{
		{
			TopicID:          topic.ID,
			Type:             domain.SingleSelect,
			Prompt:           "Q1?",
			Choices:          []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}},
			CorrectChoiceIDs: []string{"A"},
			Confidence:       1.0,
			Hash:             "hash_1",
		},
		{
			TopicID:          topic.ID,
			Type:             domain.MultiSelect,
			Prompt:           "Q2?",
			Choices:          []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}, {ID: "C", Text: "c"}},
			CorrectChoiceIDs: []string{"A", "C"},
			Confidence:       1.0,
			Hash:             "hash_2",
		},
		{
			TopicID:          topic.ID,
			Type:             domain.SingleSelect,
			Prompt:           "Q3?",
			Choices:          []domain.Choice{{ID: "A", Text: "a"}, {ID: "B", Text: "b"}},
			CorrectChoiceIDs: []string{"B"},
			Confidence:       1.0,
			Hash:             "hash_3",
		},
	}

	if _, err := questionRepo.InsertMany(questions); err != nil {
		t.Fatalf("InsertMany: %v", err)
	}

	return userRepo, topicRepo, questionRepo, sessionRepo, attemptRepo, topic, localUser.ID
}

func TestSessionLifecycle(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)

	// Start session.
	sessionID, err := engine.StartSession("test-topic", 3, "practice")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if sessionID == 0 {
		t.Fatal("expected non-zero session ID")
	}
	if engine.QueueLength() != 3 {
		t.Errorf("expected 3 questions in queue, got %d", engine.QueueLength())
	}

	// Iterate all questions.
	seen := make(map[int64]bool)
	for i := 0; i < 3; i++ {
		sq := engine.GetNextSessionQuestion()
		if sq == nil {
			t.Fatalf("expected question at position %d, got nil", i)
		}
		q := sq.Question
		if seen[q.ID] {
			t.Errorf("duplicate question ID %d", q.ID)
		}
		seen[q.ID] = true
	}

	// After all questions, GetNextSessionQuestion returns nil.
	if sq := engine.GetNextSessionQuestion(); sq != nil {
		t.Errorf("expected nil after all questions, got ID=%d", sq.Question.ID)
	}

	// End session should not error.
	if err := engine.EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

func TestRecordAttempt_CorrectnessEvaluation(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)

	if _, err := engine.StartSession("test-topic", 3, "practice"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	// Answer each question and check correctness.
	for {
		sq := engine.GetNextSessionQuestion()
		if sq == nil {
			break
		}
		q := sq.Question

		// Correct answer.
		correct, err := engine.RecordAttempt(q.ID, q.CorrectChoiceIDs, false, 100)
		if err != nil {
			t.Fatalf("RecordAttempt correct: %v", err)
		}
		if !correct {
			t.Errorf("expected correct=true for question %d with matching IDs", q.ID)
		}
	}

	if err := engine.EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

func TestRecordAttempt_SkippedIsNotCorrect(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)

	if _, err := engine.StartSession("test-topic", 1, "practice"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	sq := engine.GetNextSessionQuestion()
	if sq == nil {
		t.Fatal("expected a question")
	}
	q := sq.Question

	correct, err := engine.RecordAttempt(q.ID, nil, true, 0)
	if err != nil {
		t.Fatalf("RecordAttempt skipped: %v", err)
	}
	if correct {
		t.Error("expected skipped attempt to be not correct")
	}

	if err := engine.EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}

func TestStartSession_TopicNotFound(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)

	_, err := engine.StartSession("nonexistent", 5, "practice")
	if err == nil {
		t.Fatal("expected error for nonexistent topic")
	}
}

func TestAttemptStats_AffectSelection(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)

	// Session 1: answer Q1 wrong, Q2 correct, Q3 skip.
	if _, err := engine.StartSession("test-topic", 3, "practice"); err != nil {
		t.Fatalf("StartSession 1: %v", err)
	}
	for {
		sq := engine.GetNextSessionQuestion()
		if sq == nil {
			break
		}
		q := sq.Question
		switch q.Prompt {
		case "Q1?":
			// Wrong answer.
			if _, err := engine.RecordAttempt(q.ID, []string{"B"}, false, 50); err != nil {
				t.Fatalf("RecordAttempt wrong: %v", err)
			}
		case "Q2?":
			// Correct answer.
			if _, err := engine.RecordAttempt(q.ID, []string{"A", "C"}, false, 50); err != nil {
				t.Fatalf("RecordAttempt correct: %v", err)
			}
		default:
			// Skip.
			if _, err := engine.RecordAttempt(q.ID, nil, true, 50); err != nil {
				t.Fatalf("RecordAttempt skip: %v", err)
			}
		}
	}
	if err := engine.EndSession(); err != nil {
		t.Fatalf("EndSession 1: %v", err)
	}

	// Session 2: selection should prioritise weak questions.
	// We use a fresh engine with the same repos so stats are visible.
	rng2 := rand.New(rand.NewSource(42))
	engine2 := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng2)

	if _, err := engine2.StartSession("test-topic", 3, "practice"); err != nil {
		t.Fatalf("StartSession 2: %v", err)
	}

	// The first questions should be Q1 (wrong) or Q3 (skip=wrong in stats),
	// since they are in the "weak" bucket, while Q2 (all correct) goes to rest.
	firstSQ := engine2.GetNextSessionQuestion()
	secondSQ := engine2.GetNextSessionQuestion()
	if firstSQ == nil || secondSQ == nil {
		t.Fatal("expected at least 2 questions")
	}

	// Q2 should NOT be in the first two positions (it's the "rest" bucket).
	for _, q := range []*domain.Question{firstSQ.Question, secondSQ.Question} {
		if q.Prompt == "Q2?" {
			// Q2 was correct, so it should be in the "rest" bucket, served last.
			// This is acceptable only if all weak ones have been served already.
			// With 3 questions total, weak bucket has Q1 + Q3, so Q2 should be third.
			t.Errorf("expected weak questions before Q2, but got Q2 at early position")
		}
	}

	if err := engine2.EndSession(); err != nil {
		t.Fatalf("EndSession 2: %v", err)
	}
}

func TestAttemptStats_UserScoped(t *testing.T) {
	userRepo, topicRepo, questionRepo, sessionRepo, attemptRepo, topic, localUserID := setupTestDB(t)

	secondUser, err := userRepo.Create("alice", "Alice")
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	localEngine := app.NewSessionEngine(
		topicRepo, questionRepo, sessionRepo, attemptRepo,
		app.NewUserContext(localUserID), rand.New(rand.NewSource(1)),
	)
	aliceEngine := app.NewSessionEngine(
		topicRepo, questionRepo, sessionRepo, attemptRepo,
		app.NewUserContext(secondUser.ID), rand.New(rand.NewSource(1)),
	)

	if _, err := localEngine.StartSession("test-topic", 1, "practice"); err != nil {
		t.Fatalf("start local session: %v", err)
	}
	sqLocal := localEngine.GetNextSessionQuestion()
	if sqLocal == nil {
		t.Fatal("expected local question")
	}
	qLocal := sqLocal.Question
	if _, err := localEngine.RecordAttempt(qLocal.ID, []string{"B"}, false, 10); err != nil {
		t.Fatalf("local record attempt: %v", err)
	}
	_ = localEngine.EndSession()

	if _, err := aliceEngine.StartSession("test-topic", 1, "practice"); err != nil {
		t.Fatalf("start alice session: %v", err)
	}
	sqAlice := aliceEngine.GetNextSessionQuestion()
	if sqAlice == nil {
		t.Fatal("expected alice question")
	}
	qAlice := sqAlice.Question
	if _, err := aliceEngine.RecordAttempt(qAlice.ID, qAlice.CorrectChoiceIDs, false, 10); err != nil {
		t.Fatalf("alice record attempt: %v", err)
	}
	_ = aliceEngine.EndSession()

	localStats, err := attemptRepo.StatsByTopic(localUserID, topic.ID)
	if err != nil {
		t.Fatalf("local stats: %v", err)
	}
	aliceStats, err := attemptRepo.StatsByTopic(secondUser.ID, topic.ID)
	if err != nil {
		t.Fatalf("alice stats: %v", err)
	}

	if len(localStats) != 1 {
		t.Fatalf("expected local stats for 1 question, got %d", len(localStats))
	}
	if len(aliceStats) != 1 {
		t.Fatalf("expected alice stats for 1 question, got %d", len(aliceStats))
	}

	for _, s := range localStats {
		if s.Wrong != 1 {
			t.Fatalf("expected local wrong=1, got %d", s.Wrong)
		}
	}
	for _, s := range aliceStats {
		if s.Wrong != 0 {
			t.Fatalf("expected alice wrong=0, got %d", s.Wrong)
		}
	}
}

func TestSessionQuestion_ShuffleDeterministicWithSeed(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, _, userID := setupTestDB(t)

	buildOrders := func(seed int64) map[int64][]string {
		rng := rand.New(rand.NewSource(seed))
		engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng)
		if _, err := engine.StartSession("test-topic", 3, "practice"); err != nil {
			t.Fatalf("StartSession: %v", err)
		}
		orders := map[int64][]string{}
		for {
			sq := engine.GetNextSessionQuestion()
			if sq == nil || sq.Question == nil {
				break
			}
			orders[sq.Question.ID] = choiceIDs(sq.ShuffledChoices)
		}
		_ = engine.EndSession()
		return orders
	}

	seedA := buildOrders(99)
	seedB := buildOrders(99)
	if !reflect.DeepEqual(seedA, seedB) {
		t.Fatalf("expected identical shuffled orders for same seed; got %#v vs %#v", seedA, seedB)
	}
}

func TestSessionQuestion_ShuffleAndCorrectness_MultiSelect(t *testing.T) {
	_, topicRepo, questionRepo, sessionRepo, attemptRepo, topic, userID := setupTestDB(t)

	baseQuestions, err := questionRepo.ListByTopic(topic.ID)
	if err != nil {
		t.Fatalf("ListByTopic: %v", err)
	}
	originalByID := map[int64][]string{}
	for i := range baseQuestions {
		originalByID[baseQuestions[i].ID] = choiceIDs(baseQuestions[i].Choices)
	}

	engine := app.NewSessionEngine(
		topicRepo,
		questionRepo,
		sessionRepo,
		attemptRepo,
		app.NewUserContext(userID),
		rand.New(rand.NewSource(42)),
	)
	if _, err := engine.StartSession("test-topic", 3, "practice"); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	seenDifferentOrder := false
	seenMultiSelect := false

	for {
		sq := engine.GetNextSessionQuestion()
		if sq == nil || sq.Question == nil {
			break
		}
		qid := sq.Question.ID
		if !reflect.DeepEqual(choiceIDs(sq.ShuffledChoices), originalByID[qid]) {
			seenDifferentOrder = true
		}

		if sq.Question.Type == domain.MultiSelect {
			seenMultiSelect = true
			selected := make([]string, 0, len(sq.Question.CorrectChoiceIDs))
			correctSet := map[string]bool{}
			for _, id := range sq.Question.CorrectChoiceIDs {
				correctSet[id] = true
			}
			for _, c := range sq.ShuffledChoices {
				if correctSet[c.ID] {
					selected = append(selected, c.ID)
				}
			}
			correct, err := engine.RecordAttempt(sq.Question.ID, selected, false, 10)
			if err != nil {
				t.Fatalf("RecordAttempt multi-select: %v", err)
			}
			if !correct {
				t.Fatal("expected multi-select answer to remain correct with shuffled display order")
			}
			continue
		}

		correct, err := engine.RecordAttempt(sq.Question.ID, sq.Question.CorrectChoiceIDs, false, 10)
		if err != nil {
			t.Fatalf("RecordAttempt single-select: %v", err)
		}
		if !correct {
			t.Fatal("expected single-select answer to remain correct")
		}
	}

	if !seenDifferentOrder {
		t.Fatal("expected at least one question to have a different shuffled choice order")
	}
	if !seenMultiSelect {
		t.Fatal("expected at least one multi-select question in session")
	}

	if err := engine.EndSession(); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
}
