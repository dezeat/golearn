package sqlite_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/domain"
)

func tempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.db")
}

func TestOpen_CreatesDB(t *testing.T) {
	dbPath := tempDB(t)
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("db file was not created")
	}
}

func TestTopicRepo_UpsertAndList(t *testing.T) {
	db, err := sqlite.Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	repo := sqlite.NewTopicRepo(db)

	// First insert.
	topic, err := repo.UpsertBySlug("go-basics", "Go Basics")
	if err != nil {
		t.Fatalf("UpsertBySlug: %v", err)
	}
	if topic.Slug != "go-basics" || topic.Name != "Go Basics" || topic.ID == 0 {
		t.Errorf("unexpected topic: %+v", topic)
	}

	// Upsert same slug updates name.
	topic2, err := repo.UpsertBySlug("go-basics", "Go Basics v2")
	if err != nil {
		t.Fatalf("UpsertBySlug (update): %v", err)
	}
	if topic2.ID != topic.ID {
		t.Errorf("expected same ID, got %d vs %d", topic.ID, topic2.ID)
	}
	if topic2.Name != "Go Basics v2" {
		t.Errorf("expected updated name, got %q", topic2.Name)
	}

	// List.
	topics, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topics))
	}
}

func TestQuestionRepo_InsertAndDedupe(t *testing.T) {
	db, err := sqlite.Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	topic, err := topicRepo.UpsertBySlug("test-topic", "Test Topic")
	if err != nil {
		t.Fatalf("UpsertBySlug: %v", err)
	}

	qRepo := sqlite.NewQuestionRepo(db)

	questions := []domain.Question{
		{
			TopicID:          topic.ID,
			Type:             domain.SingleSelect,
			Prompt:           "What is 1+1?",
			Choices:          []domain.Choice{{ID: "A", Text: "1"}, {ID: "B", Text: "2"}},
			CorrectChoiceIDs: []string{"B"},
			Confidence:       1.0,
			Hash:             "hash_unique_1",
		},
		{
			TopicID:          topic.ID,
			Type:             domain.SingleSelect,
			Prompt:           "What is 2+2?",
			Choices:          []domain.Choice{{ID: "A", Text: "3"}, {ID: "B", Text: "4"}},
			CorrectChoiceIDs: []string{"B"},
			Confidence:       1.0,
			Hash:             "hash_unique_2",
		},
	}

	// First insert: both should be inserted.
	result, err := qRepo.InsertMany(questions)
	if err != nil {
		t.Fatalf("InsertMany: %v", err)
	}
	if result.Inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", result.Inserted)
	}
	if result.Duplicates != 0 {
		t.Errorf("expected 0 duplicates, got %d", result.Duplicates)
	}

	// Second insert: both should be duplicates.
	result2, err := qRepo.InsertMany(questions)
	if err != nil {
		t.Fatalf("InsertMany (re-import): %v", err)
	}
	if result2.Inserted != 0 {
		t.Errorf("expected 0 inserted on re-import, got %d", result2.Inserted)
	}
	if result2.Duplicates != 2 {
		t.Errorf("expected 2 duplicates on re-import, got %d", result2.Duplicates)
	}

	// ListByTopic should still return 2.
	stored, err := qRepo.ListByTopic(topic.ID)
	if err != nil {
		t.Fatalf("ListByTopic: %v", err)
	}
	if len(stored) != 2 {
		t.Errorf("expected 2 stored questions, got %d", len(stored))
	}
}

func TestQuestionRepo_ListByTopic_Empty(t *testing.T) {
	db, err := sqlite.Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	topic, err := topicRepo.UpsertBySlug("empty", "Empty")
	if err != nil {
		t.Fatalf("UpsertBySlug: %v", err)
	}

	qRepo := sqlite.NewQuestionRepo(db)
	questions, err := qRepo.ListByTopic(topic.ID)
	if err != nil {
		t.Fatalf("ListByTopic: %v", err)
	}
	if len(questions) != 0 {
		t.Errorf("expected 0 questions, got %d", len(questions))
	}
}
