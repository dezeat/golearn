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

// Package app — export_pack.go implements the export use case.
//
// Exports all questions for a given topic from the database back
// to the canonical pack file format (YAML or JSON). Questions are
// sorted deterministically by created_at ASC, then by hash for
// tie-breaking, ensuring byte-stable output for the same data.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// ExportService handles the export use case.
type ExportService struct {
	topics    ports.TopicRepository
	questions ports.QuestionRepository
}

// NewExportService creates an export service with the given dependencies.
func NewExportService(topics ports.TopicRepository, questions ports.QuestionRepository) *ExportService {
	return &ExportService{topics: topics, questions: questions}
}

// Export writes the canonical pack file for the given topic slug.
// format must be "yaml" or "json". outPath is the destination file.
func (s *ExportService) Export(topicSlug, outPath, format string) error {
	if format == "" {
		format = "yaml"
	}
	format = strings.ToLower(format)
	if format != "yaml" && format != "json" {
		return fmt.Errorf("unsupported export format %q (must be yaml or json)", format)
	}

	data, err := s.ExportToBytes(topicSlug, format)
	if err != nil {
		return err
	}

	// Ensure the output directory exists.
	dir := filepath.Dir(outPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("write export file %s: %w", outPath, err)
	}

	return nil
}

// ExportToBytes returns the serialized pack as bytes (for testing).
func (s *ExportService) ExportToBytes(topicSlug, format string) ([]byte, error) {
	if format == "" {
		format = "yaml"
	}
	format = strings.ToLower(format)

	topic, err := s.findTopic(topicSlug)
	if err != nil {
		return nil, err
	}

	questions, err := s.questions.ListByTopic(topic.ID)
	if err != nil {
		return nil, fmt.Errorf("list questions for topic %q: %w", topicSlug, err)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("topic %q has no questions to export", topicSlug)
	}

	sort.Slice(questions, func(i, j int) bool {
		if questions[i].CreatedAt.Equal(questions[j].CreatedAt) {
			return questions[i].Hash < questions[j].Hash
		}
		return questions[i].CreatedAt.Before(questions[j].CreatedAt)
	})

	pack := domain.Pack{
		PackVersion: "0.1.0",
		Topic: domain.PackTopic{
			Slug: topic.Slug,
			Name: topic.Name,
		},
		Questions: make([]domain.PackQuestion, 0, len(questions)),
	}

	for i := range questions {
		q := questions[i]
		pq := domain.PackQuestion{
			Type:             q.Type,
			Prompt:           q.Prompt,
			Choices:          q.Choices,
			CorrectChoiceIDs: q.CorrectChoiceIDs,
		}
		if q.Intro != "" {
			pq.Intro = q.Intro
		}
		if len(q.Tags) > 0 {
			pq.Tags = q.Tags
		}
		if q.Difficulty != "" {
			pq.Difficulty = q.Difficulty
		}
		if q.Source != "" && q.Source != "manual:file" {
			pq.Source = q.Source
		}
		if q.SourceRef != "" {
			pq.SourceRef = q.SourceRef
		}
		if q.Rationale.Correct != "" || len(q.Rationale.PerChoice) > 0 {
			rat := q.Rationale
			pq.Rationale = &rat
		}
		if q.Confidence != 1.0 {
			conf := q.Confidence
			pq.Confidence = &conf
		}
		pack.Questions = append(pack.Questions, pq)
	}

	switch format {
	case "yaml":
		return yaml.Marshal(&pack)
	case "json":
		data, err := json.MarshalIndent(&pack, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

// findTopic resolves a topic by slug from the repository.
func (s *ExportService) findTopic(slug string) (*domain.Topic, error) {
	topic, err := s.topics.GetBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("get topic %q: %w", slug, err)
	}
	if topic == nil {
		return nil, fmt.Errorf("topic %q not found", slug)
	}
	return topic, nil
}
