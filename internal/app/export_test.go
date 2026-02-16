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

	result, err := importSvc.Import("../../examples/mvp-basics.yaml")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Inserted != 10 {
		t.Fatalf("expected 10 inserted, got %d", result.Inserted)
	}

	// Export to a temp file.
	exportPath := filepath.Join(t.TempDir(), "exported.yaml")
	exportSvc := app.NewExportService(topicRepo, questionRepo)
	if err := exportSvc.Export("mvp-basics", exportPath, "yaml"); err != nil {
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
	if result2.Duplicates != 10 {
		t.Errorf("expected 10 duplicates on re-import, got %d", result2.Duplicates)
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
	if _, err := importSvc.Import("../../examples/mvp-basics.yaml"); err != nil {
		t.Fatalf("import: %v", err)
	}

	exportSvc := app.NewExportService(topicRepo, questionRepo)

	// Export twice to different paths.
	path1 := filepath.Join(t.TempDir(), "export1.yaml")
	path2 := filepath.Join(t.TempDir(), "export2.yaml")
	if err := exportSvc.Export("mvp-basics", path1, "yaml"); err != nil {
		t.Fatalf("export1: %v", err)
	}
	if err := exportSvc.Export("mvp-basics", path2, "yaml"); err != nil {
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
	if _, err := importSvc.Import("../../examples/mvp-basics.yaml"); err != nil {
		t.Fatalf("import: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "exported.json")
	exportSvc := app.NewExportService(topicRepo, questionRepo)
	if err := exportSvc.Export("mvp-basics", exportPath, "json"); err != nil {
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

	// Import the example pack.
	reader := pack.NewReader()
	importSvc := app.NewImportService(reader, topicRepo, questionRepo)
	result, err := importSvc.Import("../../examples/mvp-basics.yaml")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Inserted != 10 {
		t.Fatalf("expected 10 inserted, got %d", result.Inserted)
	}

	// Start a session with 5 questions.
	rng := rand.New(rand.NewSource(42))
	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, rng)
	sessionID, err := engine.StartSession("mvp-basics", 5, "practice")
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
		q := engine.GetNextQuestion()
		if q == nil {
			break
		}

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
		if tp.Slug == "mvp-basics" {
			topicID = tp.ID
			break
		}
	}

	stats, err := attemptRepo.StatsByTopic(topicID)
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
