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

package sqlite_test

import (
	"errors"
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
	defer func() { _ = db.Close() }()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("db file was not created")
	}
}

func TestTopicRepo_UpsertAndList(t *testing.T) {
	db, err := sqlite.Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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

func TestUserRepo_CreateListGet(t *testing.T) {
	db, err := sqlite.Open(tempDB(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := sqlite.NewUserRepo(db)

	seeded, found, err := repo.GetByHandle("local")
	if err != nil {
		t.Fatalf("GetByHandle(local): %v", err)
	}
	if !found || seeded == nil {
		t.Fatal("expected seeded local user")
	}

	created, err := repo.Create("Alice", "Alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero user ID")
	}
	if created.Handle != "alice" {
		t.Fatalf("expected normalized lowercase handle, got %q", created.Handle)
	}

	byHandle, found, err := repo.GetByHandle("alice")
	if err != nil {
		t.Fatalf("GetByHandle(alice): %v", err)
	}
	if !found || byHandle == nil {
		t.Fatal("expected to find alice by handle")
	}
	if byHandle.ID != created.ID {
		t.Fatalf("expected same ID by handle, got %d vs %d", byHandle.ID, created.ID)
	}

	byID, found, err := repo.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !found || byID == nil {
		t.Fatal("expected to find alice by ID")
	}
	if byID.Handle != "alice" {
		t.Fatalf("expected handle alice, got %q", byID.Handle)
	}

	users, err := repo.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) < 2 {
		t.Fatalf("expected at least 2 users (local + alice), got %d", len(users))
	}
}

func TestUserRepo_UniqueHandleEnforced(t *testing.T) {
	tests := []struct {
		name         string
		firstHandle  string
		secondHandle string
	}{
		{name: "same exact handle", firstHandle: "bob", secondHandle: "bob"},
		{name: "case-insensitive duplicate", firstHandle: "Alice", secondHandle: "alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := sqlite.Open(tempDB(t))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = db.Close() }()

			repo := sqlite.NewUserRepo(db)
			if _, err := repo.Create(tt.firstHandle, "First"); err != nil {
				t.Fatalf("Create first user: %v", err)
			}

			if _, err := repo.Create(tt.secondHandle, "Second"); !errors.Is(err, domain.ErrHandleTaken) {
				t.Fatalf("expected ErrHandleTaken, got %v", err)
			}
		})
	}
}
