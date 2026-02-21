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

package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/dezeat/golearn/internal/domain"
)

// TopicRepo implements ports.TopicRepository using SQLite.
type TopicRepo struct {
	db *sql.DB
}

// NewTopicRepo creates a TopicRepo.
func NewTopicRepo(db *sql.DB) *TopicRepo {
	return &TopicRepo{db: db}
}

// UpsertBySlug inserts a topic if the slug doesn't exist, otherwise returns the existing one.
// The name is updated if the topic already exists.
func (r *TopicRepo) UpsertBySlug(slug, name string) (*domain.Topic, error) {
	_, err := r.db.Exec(
		`INSERT INTO topics (slug, name) VALUES (?, ?)
		 ON CONFLICT(slug) DO UPDATE SET name = excluded.name`,
		slug, name,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert topic %q: %w", slug, err)
	}

	var t domain.Topic
	err = r.db.QueryRow("SELECT id, slug, name FROM topics WHERE slug = ?", slug).
		Scan(&t.ID, &t.Slug, &t.Name)
	if err != nil {
		return nil, fmt.Errorf("select topic %q: %w", slug, err)
	}
	return &t, nil
}

// GetBySlug returns a single topic by slug, or nil if not found.
func (r *TopicRepo) GetBySlug(slug string) (*domain.Topic, error) {
	var t domain.Topic
	err := r.db.QueryRow("SELECT id, slug, name FROM topics WHERE slug = ?", slug).
		Scan(&t.ID, &t.Slug, &t.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get topic by slug %q: %w", slug, err)
	}
	return &t, nil
}

// List returns all topics ordered by slug.
func (r *TopicRepo) List() ([]domain.Topic, error) {
	rows, err := r.db.Query("SELECT id, slug, name FROM topics ORDER BY slug")
	if err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var topics []domain.Topic
	for rows.Next() {
		var t domain.Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name); err != nil {
			return nil, fmt.Errorf("scan topic: %w", err)
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}
