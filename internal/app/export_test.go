package app_test

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/pack"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
)

// TestExportRoundtrip verifies that exporting a topic and re-importing
// it produces zero duplicates — proving the export format is canonical
// and hash-compatible with the import pipeline.
func TestExportRoundtrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "roundtrip.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	// Import the example pack.
	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)

	result, err := importSvc.Import("../../packs/go-basics.yaml")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Inserted != 15 {
		t.Fatalf("expected 15 inserted, got %d", result.Inserted)
	}

	// Export to a temp file.
	exportPath := filepath.Join(t.TempDir(), "exported.yaml")
	exportSvc := app.NewExportService(topicRepo, questionRepo)
	if err := exportSvc.Export("go-basics", exportPath, "yaml"); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Re-import the exported file — should produce zero new inserts.
	result2, err := importSvc.Import(exportPath)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if result2.Inserted != 0 {
		t.Errorf("expected 0 inserts on re-import, got %d", result2.Inserted)
	}
	if result2.Duplicates != 15 {
		t.Errorf("expected 15 duplicates on re-import, got %d", result2.Duplicates)
	}
}

// TestExportDeterministic verifies that two consecutive exports of
// the same data produce identical file content.
func TestExportDeterministic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "deterministic.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)
	if _, err := importSvc.Import("../../packs/go-basics.yaml"); err != nil {
		t.Fatalf("import: %v", err)
	}

	exportSvc := app.NewExportService(topicRepo, questionRepo)

	// Export twice to different paths.
	path1 := filepath.Join(t.TempDir(), "export1.yaml")
	path2 := filepath.Join(t.TempDir(), "export2.yaml")
	if err := exportSvc.Export("go-basics", path1, "yaml"); err != nil {
		t.Fatalf("export1: %v", err)
	}
	if err := exportSvc.Export("go-basics", path2, "yaml"); err != nil {
		t.Fatalf("export2: %v", err)
	}

	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("read export1: %v", err)
	}
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("read export2: %v", err)
	}

	if string(data1) != string(data2) {
		t.Error("exports are not byte-identical")
	}
}

// TestExportJSON verifies that JSON export also works and is reimportable.
func TestExportJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "json.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)
	if _, err := importSvc.Import("../../packs/go-basics.yaml"); err != nil {
		t.Fatalf("import: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "exported.json")
	exportSvc := app.NewExportService(topicRepo, questionRepo)
	if err := exportSvc.Export("go-basics", exportPath, "json"); err != nil {
		t.Fatalf("export json: %v", err)
	}

	// Re-import JSON.
	result, err := importSvc.Import(exportPath)
	if err != nil {
		t.Fatalf("re-import json: %v", err)
	}
	if result.Inserted != 0 {
		t.Errorf("expected 0 inserts on json re-import, got %d", result.Inserted)
	}
}

// TestIntegration_ImportAndSession imports a pack, runs a full session
// via the session engine (non-interactively), and verifies stats.
func TestIntegration_ImportAndSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)
	userRepo := sqlite.NewUserRepo(db)
	localUser, found, err := userRepo.GetByHandle("local")
	if err != nil {
		t.Fatalf("get local user: %v", err)
	}
	if !found || localUser == nil {
		t.Fatal("expected seeded local user")
	}

	// Import the example pack.
	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)
	result, err := importSvc.Import("../../packs/go-basics.yaml")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Inserted != 15 {
		t.Fatalf("expected 15 inserted, got %d", result.Inserted)
	}

	// Start a session with 5 questions.
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, app.NewUserContext(localUser.ID), rng)
	sessionID, err := engine.StartSession("go-basics", 5, "practice")
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if sessionID == 0 {
		t.Fatal("expected non-zero session ID")
	}
	if engine.QueueLength() != 5 {
		t.Fatalf("expected queue length 5, got %d", engine.QueueLength())
	}

	// Answer all questions — submit the correct answers.
	answered := 0
	correctCount := 0
	for {
		sq := engine.GetNextSessionQuestion()
		if sq == nil {
			break
		}
		q := sq.Question

		// Submit the correct answer.
		correct, err := engine.RecordAttempt(q.ID, q.CorrectChoiceIDs, false, 100)
		if err != nil {
			t.Fatalf("record attempt: %v", err)
		}
		answered++
		if correct {
			correctCount++
		}
	}

	if answered != 5 {
		t.Errorf("expected 5 answered, got %d", answered)
	}
	if correctCount != 5 {
		t.Errorf("expected 5 correct (all correct answers submitted), got %d", correctCount)
	}

	if err := engine.EndSession(); err != nil {
		t.Fatalf("end session: %v", err)
	}

	// Verify stats are recorded.
	topics, err := topicRepo.List()
	if err != nil {
		t.Fatalf("list topics: %v", err)
	}
	var topicID int64
	for _, tp := range topics {
		if tp.Slug == "go-basics" {
			topicID = tp.ID
			break
		}
	}

	stats, err := attemptRepo.StatsByTopic(localUser.ID, topicID)
	if err != nil {
		t.Fatalf("stats by topic: %v", err)
	}
	if len(stats) != 5 {
		t.Errorf("expected stats for 5 questions, got %d", len(stats))
	}
	for qid, s := range stats {
		if s.Attempts != 1 {
			t.Errorf("question %d: expected 1 attempt, got %d", qid, s.Attempts)
		}
		if s.Wrong != 0 {
			t.Errorf("question %d: expected 0 wrong, got %d", qid, s.Wrong)
		}
	}
}

// TestImport_RationalePack validates that a pack with full rationale
// imports cleanly and has the expected number of questions.
func TestImport_RationalePack(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rationale.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)

	result, err := importSvc.Import("testdata/rationale-pack.yaml")
	if err != nil {
		t.Fatalf("import rationale pack: %v", err)
	}
	if result.Inserted != 3 {
		t.Errorf("expected 3 inserted, got %d", result.Inserted)
	}
	if result.Duplicates != 0 {
		t.Errorf("expected 0 duplicates, got %d", result.Duplicates)
	}
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	// Verify questions have rationale data.
	topic, err := topicRepo.GetBySlug("rationale-test")
	if err != nil {
		t.Fatalf("get topic: %v", err)
	}
	questions, err := questionRepo.ListByTopic(topic.ID)
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}

	for _, q := range questions {
		if q.Rationale.Correct == "" {
			t.Errorf("question %q missing rationale.correct", q.Prompt[:40])
		}
		if len(q.Rationale.PerChoice) == 0 {
			t.Errorf("question %q missing rationale.per_choice", q.Prompt[:40])
		}
		// Every choice should have a per_choice explanation.
		for _, c := range q.Choices {
			if _, ok := q.Rationale.PerChoice[c.ID]; !ok {
				t.Errorf("question %q missing per_choice explanation for %s", q.Prompt[:40], c.ID)
			}
		}
	}

	// Re-import should produce all duplicates.
	result2, err := importSvc.Import("testdata/rationale-pack.yaml")
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if result2.Inserted != 0 {
		t.Errorf("re-import: expected 0 inserts, got %d", result2.Inserted)
	}
	if result2.Duplicates != 3 {
		t.Errorf("re-import: expected 3 duplicates, got %d", result2.Duplicates)
	}
}

// TestExportRoundtrip_WithRationale verifies that rationale survives export/reimport.
func TestExportRoundtrip_WithRationale(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rationale_roundtrip.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)

	if _, err := importSvc.Import("testdata/rationale-pack.yaml"); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Export.
	exportPath := filepath.Join(t.TempDir(), "exported_rationale.yaml")
	exportSvc := app.NewExportService(topicRepo, questionRepo)
	if err := exportSvc.Export("rationale-test", exportPath, "yaml"); err != nil {
		t.Fatalf("export: %v", err)
	}

	// Import into a fresh DB to verify rationale survived.
	dbPath2 := filepath.Join(t.TempDir(), "rationale_roundtrip2.db")
	db2, err := sqlite.Open(dbPath2)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()

	topicRepo2 := sqlite.NewTopicRepo(db2)
	questionRepo2 := sqlite.NewQuestionRepo(db2)
	importSvc2 := app.NewImportService(reader, topicRepo2, questionRepo2)

	result, err := importSvc2.Import(exportPath)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if result.Inserted != 3 {
		t.Errorf("expected 3 inserted from exported file, got %d", result.Inserted)
	}

	// Verify rationale preserved.
	topic, err := topicRepo2.GetBySlug("rationale-test")
	if err != nil {
		t.Fatalf("get topic: %v", err)
	}
	questions, err := questionRepo2.ListByTopic(topic.ID)
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}

	for _, q := range questions {
		if q.Rationale.Correct == "" {
			t.Errorf("exported question %q lost rationale.correct", q.Prompt[:40])
		}
		if len(q.Rationale.PerChoice) == 0 {
			t.Errorf("exported question %q lost rationale.per_choice", q.Prompt[:40])
		}
	}
}
