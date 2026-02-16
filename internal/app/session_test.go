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
	*sqlite.TopicRepo,
	*sqlite.QuestionRepo,
	*sqlite.SessionRepo,
	*sqlite.AttemptRepo,
	*domain.Topic,
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

	return topicRepo, questionRepo, sessionRepo, attemptRepo, topic
}

func TestSessionLifecycle(t *testing.T) {
	topicRepo, questionRepo, sessionRepo, attemptRepo, _ := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)

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
	topicRepo, questionRepo, sessionRepo, attemptRepo, _ := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)

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
	topicRepo, questionRepo, sessionRepo, attemptRepo, _ := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)

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
	topicRepo, questionRepo, sessionRepo, attemptRepo, _ := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)

	_, err := engine.StartSession("nonexistent", 5, "practice")
	if err == nil {
		t.Fatal("expected error for nonexistent topic")
	}
}

func TestAttemptStats_AffectSelection(t *testing.T) {
	topicRepo, questionRepo, sessionRepo, attemptRepo, _ := setupTestDB(t)
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)

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
	engine2 := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng2)

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
