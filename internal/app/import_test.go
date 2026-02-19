package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/pack"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
)

func TestImportService_SingleFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "import.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	reader := pack.NewReader()
	svc := app.NewImportService(reader, topicRepo, questionRepo)

	result, err := svc.Import("../../examples/mvp-basics.yaml")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.FilesProcessed != 1 {
		t.Errorf("FilesProcessed = %d, want 1", result.FilesProcessed)
	}
	if result.Inserted < 1 {
		t.Errorf("Inserted = %d, want > 0", result.Inserted)
	}
	if result.Invalid != 0 {
		t.Errorf("Invalid = %d, want 0", result.Invalid)
	}
}

func TestImportService_DuplicateSkipped(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dup.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	reader := pack.NewReader()
	svc := app.NewImportService(reader, topicRepo, questionRepo)

	// First import.
	r1, err := svc.Import("../../examples/mvp-basics.yaml")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	inserted := r1.Inserted

	// Second import — should all be duplicates.
	r2, err := svc.Import("../../examples/mvp-basics.yaml")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if r2.Inserted != 0 {
		t.Errorf("second Inserted = %d, want 0", r2.Inserted)
	}
	if r2.Duplicates != inserted {
		t.Errorf("second Duplicates = %d, want %d", r2.Duplicates, inserted)
	}
}

func TestImportService_Directory(t *testing.T) {
	dir := t.TempDir()
	content := `pack_version: "0.1.0"
topic:
  slug: "dir-test"
  name: "Dir Test"
questions:
  - type: "single_select"
    prompt: "What is 2+2?"
    choices:
      - { id: "A", text: "3" }
      - { id: "B", text: "4" }
    correct_choice_ids: ["B"]
    tags: ["math"]
    difficulty: easy
`
	if err := os.WriteFile(filepath.Join(dir, "pack1.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("not a pack"), 0o644); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(t.TempDir(), "dir.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	reader := pack.NewReader()
	svc := app.NewImportService(reader, topicRepo, questionRepo)

	result, err := svc.Import(dir)
	if err != nil {
		t.Fatalf("Import dir: %v", err)
	}
	if result.FilesProcessed != 1 {
		t.Errorf("FilesProcessed = %d, want 1", result.FilesProcessed)
	}
	if result.Inserted != 1 {
		t.Errorf("Inserted = %d, want 1", result.Inserted)
	}
}

func TestImportService_MissingPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	reader := pack.NewReader()
	svc := app.NewImportService(reader, topicRepo, questionRepo)

	_, err = svc.Import("/tmp/nonexistent-golearn-import-test")
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}
