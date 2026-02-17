package sqlite_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/domain"
)

// setupStatsDB creates a temp DB and returns it along with repos and a cleanup function.
func setupStatsDB(t *testing.T) (*sql.DB, *sqlite.TopicRepo, *sqlite.QuestionRepo, *sqlite.SessionRepo, *sqlite.AttemptRepo, *sqlite.UserRepo, *sqlite.StatsRepo) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stats_test.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db,
		sqlite.NewTopicRepo(db),
		sqlite.NewQuestionRepo(db),
		sqlite.NewSessionRepo(db),
		sqlite.NewAttemptRepo(db),
		sqlite.NewUserRepo(db),
		sqlite.NewStatsRepo(db)
}

// insertQuestions inserts questions with given hashes and returns their IDs.
func insertQuestions(t *testing.T, qRepo *sqlite.QuestionRepo, topicID int64, count int, opts ...func(i int, q *domain.Question)) []int64 {
	t.Helper()
	var questions []domain.Question
	for i := 0; i < count; i++ {
		q := domain.Question{
			TopicID:          topicID,
			Type:             domain.SingleSelect,
			Prompt:           fmt.Sprintf("Question %d?", i+1),
			Choices:          []domain.Choice{{ID: "A", Text: "Yes"}, {ID: "B", Text: "No"}},
			CorrectChoiceIDs: []string{"A"},
			Confidence:       1.0,
			Hash:             fmt.Sprintf("stats_hash_%d_%d", topicID, i),
		}
		for _, opt := range opts {
			opt(i, &q)
		}
		questions = append(questions, q)
	}
	_, err := qRepo.InsertMany(questions)
	if err != nil {
		t.Fatalf("insert questions: %v", err)
	}

	stored, err := qRepo.ListByTopic(topicID)
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}
	ids := make([]int64, len(stored))
	for i, q := range stored {
		ids[i] = q.ID
	}
	return ids
}

// recordAttempts records attempts for a user in a session.
func recordAttempts(t *testing.T, attemptRepo *sqlite.AttemptRepo, userID, sessionID int64, questionIDs []int64, correct []bool, latencyMs []int) {
	t.Helper()
	for i, qID := range questionIDs {
		a := &domain.Attempt{
			UserID:            userID,
			SessionID:         sessionID,
			QuestionID:        qID,
			SelectedChoiceIDs: []string{"A"},
			Correct:           correct[i],
			Skipped:           false,
			LatencyMs:         latencyMs[i],
			CreatedAt:         time.Now().UTC(),
		}
		if err := attemptRepo.Record(a); err != nil {
			t.Fatalf("record attempt: %v", err)
		}
	}
}

func TestStatsRepo_MultiUserIsolation(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	// Create two users.
	alice, err := userRepo.Create("alice", "Alice")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := userRepo.Create("bob", "Bob")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// Create a topic + questions.
	topic, err := topicRepo.UpsertBySlug("stats-test", "Stats Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}
	qIDs := insertQuestions(t, qRepo, topic.ID, 5)

	// Alice: session with 5 attempts, 4 correct.
	aliceSession := &domain.Session{UserID: alice.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 5, StartedAt: time.Now().UTC()}
	aliceSessID, err := sessionRepo.Create(aliceSession)
	if err != nil {
		t.Fatalf("create alice session: %v", err)
	}
	recordAttempts(t, attemptRepo, alice.ID, aliceSessID, qIDs,
		[]bool{true, true, true, true, false},
		[]int{1000, 2000, 1500, 1000, 3000})
	_ = sessionRepo.Finish(aliceSessID)

	// Bob: session with 5 attempts, 2 correct.
	bobSession := &domain.Session{UserID: bob.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 5, StartedAt: time.Now().UTC()}
	bobSessID, err := sessionRepo.Create(bobSession)
	if err != nil {
		t.Fatalf("create bob session: %v", err)
	}
	recordAttempts(t, attemptRepo, bob.ID, bobSessID, qIDs,
		[]bool{true, false, true, false, false},
		[]int{500, 1000, 2000, 500, 1000})
	_ = sessionRepo.Finish(bobSessID)

	// Verify Alice's global stats.
	aliceGlobal, err := statsRepo.GlobalStats(alice.ID)
	if err != nil {
		t.Fatalf("alice global stats: %v", err)
	}
	if aliceGlobal.TotalAnswered != 5 {
		t.Errorf("alice answered: got %d, want 5", aliceGlobal.TotalAnswered)
	}
	if aliceGlobal.AccuracyPct != 80 {
		t.Errorf("alice accuracy: got %.1f%%, want 80%%", aliceGlobal.AccuracyPct)
	}

	// Verify Bob's global stats.
	bobGlobal, err := statsRepo.GlobalStats(bob.ID)
	if err != nil {
		t.Fatalf("bob global stats: %v", err)
	}
	if bobGlobal.TotalAnswered != 5 {
		t.Errorf("bob answered: got %d, want 5", bobGlobal.TotalAnswered)
	}
	if bobGlobal.AccuracyPct != 40 {
		t.Errorf("bob accuracy: got %.1f%%, want 40%%", bobGlobal.AccuracyPct)
	}

	// Verify topic summaries are different.
	aliceTopic, err := statsRepo.TopicSummary(alice.ID, topic.ID)
	if err != nil {
		t.Fatalf("alice topic summary: %v", err)
	}
	bobTopic, err := statsRepo.TopicSummary(bob.ID, topic.ID)
	if err != nil {
		t.Fatalf("bob topic summary: %v", err)
	}
	if aliceTopic.AccuracyPct == bobTopic.AccuracyPct {
		t.Error("alice and bob should have different accuracy")
	}
}

func TestStatsRepo_DifficultyBucketing(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("duser", "Difficulty User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("diff-test", "Difficulty Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	// Insert questions with different difficulties: easy, easy, medium, hard, hard, unset.
	qIDs := insertQuestions(t, qRepo, topic.ID, 6, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("diff_hash_%d", i)
		difficulties := []domain.Difficulty{
			domain.DifficultyEasy, domain.DifficultyEasy,
			domain.DifficultyMedium,
			domain.DifficultyHard, domain.DifficultyHard,
			domain.DifficultyUnset,
		}
		q.Difficulty = difficulties[i]
	})

	sess := &domain.Session{UserID: user.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 6, StartedAt: time.Now().UTC()}
	sessID, err := sessionRepo.Create(sess)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recordAttempts(t, attemptRepo, user.ID, sessID, qIDs,
		[]bool{true, true, true, false, false, true},
		[]int{1000, 1000, 2000, 3000, 3000, 500})
	_ = sessionRepo.Finish(sessID)

	diffs, err := statsRepo.DifficultyStats(user.ID, topic.ID)
	if err != nil {
		t.Fatalf("difficulty stats: %v", err)
	}

	// Expect: Easy (2 questions), Medium (1), Hard (2), Unrated (1).
	bucketMap := map[string]int{}
	for _, d := range diffs {
		bucketMap[d.Bucket] = d.AttemptsAnswered
	}

	if bucketMap["Easy"] != 2 {
		t.Errorf("Easy attempts: got %d, want 2", bucketMap["Easy"])
	}
	if bucketMap["Medium"] != 1 {
		t.Errorf("Medium attempts: got %d, want 1", bucketMap["Medium"])
	}
	if bucketMap["Hard"] != 2 {
		t.Errorf("Hard attempts: got %d, want 2", bucketMap["Hard"])
	}
	if bucketMap["Unrated"] != 1 {
		t.Errorf("Unrated attempts: got %d, want 1", bucketMap["Unrated"])
	}

	// Verify accuracy: Easy = 100%, Medium = 100%, Hard = 0%, Unrated = 100%.
	for _, d := range diffs {
		switch d.Bucket {
		case "Easy":
			if d.AccuracyPct != 100 {
				t.Errorf("Easy accuracy: got %.0f%%, want 100%%", d.AccuracyPct)
			}
		case "Medium":
			if d.AccuracyPct != 100 {
				t.Errorf("Medium accuracy: got %.0f%%, want 100%%", d.AccuracyPct)
			}
		case "Hard":
			if d.AccuracyPct != 0 {
				t.Errorf("Hard accuracy: got %.0f%%, want 0%%", d.AccuracyPct)
			}
		case "Unrated":
			if d.AccuracyPct != 100 {
				t.Errorf("Unrated accuracy: got %.0f%%, want 100%%", d.AccuracyPct)
			}
		}
	}
}

func TestStatsRepo_SessionTrend(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("trenduser", "Trend User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("trend-test", "Trend Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	qIDs := insertQuestions(t, qRepo, topic.ID, 3, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("trend_hash_%d", i)
	})

	// Create 3 sessions with different accuracies.
	// Session 1: 1/3 correct = 33.3%
	// Session 2: 2/3 correct = 66.7%
	// Session 3: 3/3 correct = 100%
	expectedAccuracies := []float64{100.0 / 3, 200.0 / 3, 100}
	correctPatterns := [][]bool{
		{true, false, false},
		{true, true, false},
		{true, true, true},
	}

	for i := 0; i < 3; i++ {
		baseTime := time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC)
		sess := &domain.Session{
			UserID:     user.ID,
			TopicID:    topic.ID,
			Mode:       "practice",
			RequestedN: 3,
			StartedAt:  baseTime,
		}
		sessID, err := sessionRepo.Create(sess)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		recordAttempts(t, attemptRepo, user.ID, sessID, qIDs,
			correctPatterns[i],
			[]int{1000, 1000, 1000})
		_ = sessionRepo.Finish(sessID)
	}

	trend, err := statsRepo.SessionTrend(user.ID, topic.ID, 10)
	if err != nil {
		t.Fatalf("session trend: %v", err)
	}

	if len(trend) != 3 {
		t.Fatalf("trend length: got %d, want 3", len(trend))
	}

	for i, expected := range expectedAccuracies {
		if diff := trend[i] - expected; diff > 0.2 || diff < -0.2 {
			t.Errorf("trend[%d]: got %.1f, want ~%.1f", i, trend[i], expected)
		}
	}

	// Trend should be ascending.
	for i := 1; i < len(trend); i++ {
		if trend[i] < trend[i-1] {
			t.Errorf("trend not ascending: trend[%d]=%.1f < trend[%d]=%.1f", i, trend[i], i-1, trend[i-1])
		}
	}
}

func TestStatsRepo_WeakQuestions(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("weakuser", "Weak User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("weak-test", "Weak Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	qIDs := insertQuestions(t, qRepo, topic.ID, 3, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("weak_hash_%d", i)
	})

	// Create 4 sessions so each question has ≥3 attempts.
	// Q1: always correct (0% wrong)
	// Q2: 1/4 correct (75% wrong)
	// Q3: 2/4 correct (50% wrong)
	patternsPerSession := [][]bool{
		{true, false, true},
		{true, false, false},
		{true, true, true},
		{true, false, false},
	}
	for i := 0; i < 4; i++ {
		baseTime := time.Date(2026, 2, 1+i, 0, 0, 0, 0, time.UTC)
		sess := &domain.Session{
			UserID:     user.ID,
			TopicID:    topic.ID,
			Mode:       "practice",
			RequestedN: 3,
			StartedAt:  baseTime,
		}
		sessID, err := sessionRepo.Create(sess)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		recordAttempts(t, attemptRepo, user.ID, sessID, qIDs,
			patternsPerSession[i],
			[]int{1000, 1000, 1000})
		_ = sessionRepo.Finish(sessID)
	}

	// minAttempts=3, limit=10
	weak, err := statsRepo.WeakQuestions(user.ID, topic.ID, 3, 10)
	if err != nil {
		t.Fatalf("weak questions: %v", err)
	}

	// Q1 has 0% wrong rate — should NOT appear.
	// Q2 has 75% wrong rate, Q3 has 50% wrong rate.
	// Should be ordered by wrong_rate descending: Q2, Q3.
	if len(weak) != 2 {
		t.Fatalf("weak count: got %d, want 2", len(weak))
	}

	// First should be Q2 (75% wrong).
	if weak[0].QuestionID != qIDs[1] {
		t.Errorf("weakest question: got qID %d, want %d", weak[0].QuestionID, qIDs[1])
	}
	if weak[0].WrongRate < 0.7 || weak[0].WrongRate > 0.8 {
		t.Errorf("wrong rate for Q2: got %.2f, want ~0.75", weak[0].WrongRate)
	}

	// Second should be Q3 (50% wrong).
	if weak[1].QuestionID != qIDs[2] {
		t.Errorf("second weakest: got qID %d, want %d", weak[1].QuestionID, qIDs[2])
	}

	// With minAttempts=5, should return nothing.
	weak5, err := statsRepo.WeakQuestions(user.ID, topic.ID, 5, 10)
	if err != nil {
		t.Fatalf("weak questions min5: %v", err)
	}
	if len(weak5) != 0 {
		t.Errorf("expected 0 weak questions with minAttempts=5, got %d", len(weak5))
	}
}

func TestStatsRepo_TopicSummary_Coverage(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("covuser", "Coverage User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("cov-test", "Coverage Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	// 5 questions, attempt only 3.
	qIDs := insertQuestions(t, qRepo, topic.ID, 5, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("cov_hash_%d", i)
	})

	sess := &domain.Session{UserID: user.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 3, StartedAt: time.Now().UTC()}
	sessID, err := sessionRepo.Create(sess)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recordAttempts(t, attemptRepo, user.ID, sessID, qIDs[:3],
		[]bool{true, true, false},
		[]int{1000, 2000, 1500})
	_ = sessionRepo.Finish(sessID)

	ts, err := statsRepo.TopicSummary(user.ID, topic.ID)
	if err != nil {
		t.Fatalf("topic summary: %v", err)
	}

	if ts.TotalQuestions != 5 {
		t.Errorf("total questions: got %d, want 5", ts.TotalQuestions)
	}
	if ts.SeenQuestions != 3 {
		t.Errorf("seen questions: got %d, want 3", ts.SeenQuestions)
	}
	if ts.CoveragePct != 60 {
		t.Errorf("coverage: got %.0f, want 60", ts.CoveragePct)
	}
	if ts.AttemptsAnswered != 3 {
		t.Errorf("answered: got %d, want 3", ts.AttemptsAnswered)
	}
}

func TestStatsRepo_GlobalStats_Skipped(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("skipuser", "Skip User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("skip-test", "Skip Test")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	qIDs := insertQuestions(t, qRepo, topic.ID, 3, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("skip_hash_%d", i)
	})

	sess := &domain.Session{UserID: user.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 3, StartedAt: time.Now().UTC()}
	sessID, err := sessionRepo.Create(sess)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// 2 answered (1 correct, 1 wrong), 1 skipped.
	for i, qID := range qIDs {
		a := &domain.Attempt{
			UserID:            user.ID,
			SessionID:         sessID,
			QuestionID:        qID,
			SelectedChoiceIDs: []string{"A"},
			Correct:           i == 0,
			Skipped:           i == 2,
			LatencyMs:         1000,
			CreatedAt:         time.Now().UTC(),
		}
		if err := attemptRepo.Record(a); err != nil {
			t.Fatalf("record attempt: %v", err)
		}
	}
	_ = sessionRepo.Finish(sessID)

	gs, err := statsRepo.GlobalStats(user.ID)
	if err != nil {
		t.Fatalf("global stats: %v", err)
	}

	if gs.TotalAnswered != 2 {
		t.Errorf("answered: got %d, want 2", gs.TotalAnswered)
	}
	if gs.TotalSkipped != 1 {
		t.Errorf("skipped: got %d, want 1", gs.TotalSkipped)
	}
	// Accuracy = 1/2 = 50% (excludes skipped).
	if gs.AccuracyPct != 50 {
		t.Errorf("accuracy: got %.0f, want 50", gs.AccuracyPct)
	}
}

func TestStatsRepo_StrongestTopic(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("stronguser", "Strong User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Two topics: "Alpha" (100% accuracy, 5 attempts) and "Beta" (40% accuracy, 5 attempts).
	topicA, err := topicRepo.UpsertBySlug("alpha", "Alpha")
	if err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}
	topicB, err := topicRepo.UpsertBySlug("beta", "Beta")
	if err != nil {
		t.Fatalf("upsert beta: %v", err)
	}

	qA := insertQuestions(t, qRepo, topicA.ID, 5, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("strong_a_%d", i)
	})
	qB := insertQuestions(t, qRepo, topicB.ID, 5, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("strong_b_%d", i)
	})

	// Alpha: 5/5 correct = 100%.
	sessA := &domain.Session{UserID: user.ID, TopicID: topicA.ID, Mode: "practice", RequestedN: 5, StartedAt: time.Now().UTC()}
	sessAID, err := sessionRepo.Create(sessA)
	if err != nil {
		t.Fatalf("create alpha session: %v", err)
	}
	recordAttempts(t, attemptRepo, user.ID, sessAID, qA,
		[]bool{true, true, true, true, true},
		[]int{1000, 1000, 1000, 1000, 1000})
	_ = sessionRepo.Finish(sessAID)

	// Beta: 2/5 correct = 40%.
	sessB := &domain.Session{UserID: user.ID, TopicID: topicB.ID, Mode: "practice", RequestedN: 5, StartedAt: time.Now().UTC()}
	sessBID, err := sessionRepo.Create(sessB)
	if err != nil {
		t.Fatalf("create beta session: %v", err)
	}
	recordAttempts(t, attemptRepo, user.ID, sessBID, qB,
		[]bool{true, true, false, false, false},
		[]int{1000, 1000, 1000, 1000, 1000})
	_ = sessionRepo.Finish(sessBID)

	gs, err := statsRepo.GlobalStats(user.ID)
	if err != nil {
		t.Fatalf("global stats: %v", err)
	}

	if gs.StrongestTopic != "Alpha" {
		t.Errorf("strongest: got %q, want %q", gs.StrongestTopic, "Alpha")
	}
	if gs.WeakestTopic != "Beta" {
		t.Errorf("weakest: got %q, want %q", gs.WeakestTopic, "Beta")
	}
}

func TestStatsRepo_StrongestTopic_BelowThreshold(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	user, err := userRepo.Create("threshuser", "Threshold User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	topic, err := topicRepo.UpsertBySlug("small-topic", "Small Topic")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	// Only 3 attempts — below the min 5 threshold.
	qIDs := insertQuestions(t, qRepo, topic.ID, 3, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("thresh_hash_%d", i)
	})

	sess := &domain.Session{UserID: user.ID, TopicID: topic.ID, Mode: "practice", RequestedN: 3, StartedAt: time.Now().UTC()}
	sessID, err := sessionRepo.Create(sess)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	recordAttempts(t, attemptRepo, user.ID, sessID, qIDs,
		[]bool{true, true, true},
		[]int{1000, 1000, 1000})
	_ = sessionRepo.Finish(sessID)

	gs, err := statsRepo.GlobalStats(user.ID)
	if err != nil {
		t.Fatalf("global stats: %v", err)
	}

	// Below threshold: both strongest and weakest should be empty.
	if gs.StrongestTopic != "" {
		t.Errorf("strongest should be empty below threshold, got %q", gs.StrongestTopic)
	}
	if gs.WeakestTopic != "" {
		t.Errorf("weakest should be empty below threshold, got %q", gs.WeakestTopic)
	}
}

func TestStatsRepo_StrongestTopic_MultiUser(t *testing.T) {
	_, topicRepo, qRepo, sessionRepo, attemptRepo, userRepo, statsRepo := setupStatsDB(t)

	alice, err := userRepo.Create("alice2", "Alice2")
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := userRepo.Create("bob2", "Bob2")
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	topicX, err := topicRepo.UpsertBySlug("topic-x", "Topic X")
	if err != nil {
		t.Fatalf("upsert x: %v", err)
	}
	topicY, err := topicRepo.UpsertBySlug("topic-y", "Topic Y")
	if err != nil {
		t.Fatalf("upsert y: %v", err)
	}

	qX := insertQuestions(t, qRepo, topicX.ID, 5, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("mu_x_%d", i)
	})
	qY := insertQuestions(t, qRepo, topicY.ID, 5, func(i int, q *domain.Question) {
		q.Hash = fmt.Sprintf("mu_y_%d", i)
	})

	// Alice: 100% on X, 40% on Y → strongest=X, weakest=Y
	for _, tc := range []struct {
		user    *domain.User
		topic   *domain.Topic
		qIDs    []int64
		correct []bool
	}{
		{alice, topicX, qX, []bool{true, true, true, true, true}},
		{alice, topicY, qY, []bool{true, true, false, false, false}},
		// Bob: 20% on X, 80% on Y → strongest=Y, weakest=X
		{bob, topicX, qX, []bool{true, false, false, false, false}},
		{bob, topicY, qY, []bool{true, true, true, true, false}},
	} {
		sess := &domain.Session{UserID: tc.user.ID, TopicID: tc.topic.ID, Mode: "practice", RequestedN: 5, StartedAt: time.Now().UTC()}
		sessID, err := sessionRepo.Create(sess)
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		recordAttempts(t, attemptRepo, tc.user.ID, sessID, tc.qIDs,
			tc.correct,
			[]int{1000, 1000, 1000, 1000, 1000})
		_ = sessionRepo.Finish(sessID)
	}

	aliceGS, err := statsRepo.GlobalStats(alice.ID)
	if err != nil {
		t.Fatalf("alice global: %v", err)
	}
	if aliceGS.StrongestTopic != "Topic X" {
		t.Errorf("alice strongest: got %q, want %q", aliceGS.StrongestTopic, "Topic X")
	}
	if aliceGS.WeakestTopic != "Topic Y" {
		t.Errorf("alice weakest: got %q, want %q", aliceGS.WeakestTopic, "Topic Y")
	}

	bobGS, err := statsRepo.GlobalStats(bob.ID)
	if err != nil {
		t.Fatalf("bob global: %v", err)
	}
	if bobGS.StrongestTopic != "Topic Y" {
		t.Errorf("bob strongest: got %q, want %q", bobGS.StrongestTopic, "Topic Y")
	}
	if bobGS.WeakestTopic != "Topic X" {
		t.Errorf("bob weakest: got %q, want %q", bobGS.WeakestTopic, "Topic X")
	}
}
