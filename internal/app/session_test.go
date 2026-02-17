package app_test

import (
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
)

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
	t.Cleanup(func() { db.Close() })

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
		q := engine.GetNextQuestion()
		if q == nil {
			t.Fatalf("expected question at position %d, got nil", i)
		}
		if seen[q.ID] {
			t.Errorf("duplicate question ID %d", q.ID)
		}
		seen[q.ID] = true
	}

	// After all questions, GetNextQuestion returns nil.
	if q := engine.GetNextQuestion(); q != nil {
		t.Errorf("expected nil after all questions, got ID=%d", q.ID)
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
		q := engine.GetNextQuestion()
		if q == nil {
			break
		}

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

	q := engine.GetNextQuestion()
	if q == nil {
		t.Fatal("expected a question")
	}

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
		q := engine.GetNextQuestion()
		if q == nil {
			break
		}
		switch {
		case q.Prompt == "Q1?":
			// Wrong answer.
			engine.RecordAttempt(q.ID, []string{"B"}, false, 50)
		case q.Prompt == "Q2?":
			// Correct answer.
			engine.RecordAttempt(q.ID, []string{"A", "C"}, false, 50)
		default:
			// Skip.
			engine.RecordAttempt(q.ID, nil, true, 50)
		}
	}
	engine.EndSession()

	// Session 2: selection should prioritise weak questions.
	// We use a fresh engine with the same repos so stats are visible.
	rng2 := rand.New(rand.NewSource(42))
	engine2 := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(userID), rng2)

	if _, err := engine2.StartSession("test-topic", 3, "practice"); err != nil {
		t.Fatalf("StartSession 2: %v", err)
	}

	// The first questions should be Q1 (wrong) or Q3 (skip=wrong in stats),
	// since they are in the "weak" bucket, while Q2 (all correct) goes to rest.
	first := engine2.GetNextQuestion()
	second := engine2.GetNextQuestion()
	if first == nil || second == nil {
		t.Fatal("expected at least 2 questions")
	}

	// Q2 should NOT be in the first two positions (it's the "rest" bucket).
	for _, q := range []*domain.Question{first, second} {
		if q.Prompt == "Q2?" {
			// Q2 was correct, so it should be in the "rest" bucket, served last.
			// This is acceptable only if all weak ones have been served already.
			// With 3 questions total, weak bucket has Q1 + Q3, so Q2 should be third.
			t.Errorf("expected weak questions before Q2, but got Q2 at early position")
		}
	}

	engine2.EndSession()
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
	qLocal := localEngine.GetNextQuestion()
	if qLocal == nil {
		t.Fatal("expected local question")
	}
	if _, err := localEngine.RecordAttempt(qLocal.ID, []string{"B"}, false, 10); err != nil {
		t.Fatalf("local record attempt: %v", err)
	}
	_ = localEngine.EndSession()

	if _, err := aliceEngine.StartSession("test-topic", 1, "practice"); err != nil {
		t.Fatalf("start alice session: %v", err)
	}
	qAlice := aliceEngine.GetNextQuestion()
	if qAlice == nil {
		t.Fatal("expected alice question")
	}
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
