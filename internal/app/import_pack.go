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

// Package app contains application use cases for golearn.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// ImportResult summarises the outcome of an import operation.
type ImportResult struct {
	FilesProcessed int
	Inserted       int
	Duplicates     int
	Invalid        int
	Errors         []string
}

// ImportService handles the import use case.
type ImportService struct {
	reader    ports.PackReader
	topics    ports.TopicRepository
	questions ports.QuestionRepository
}

// NewImportService creates an import service with the given dependencies.
func NewImportService(reader ports.PackReader, topics ports.TopicRepository, questions ports.QuestionRepository) *ImportService {
	return &ImportService{reader: reader, topics: topics, questions: questions}
}

// Import loads packs from a file or directory, validates, normalises, and persists.
func (s *ImportService) Import(path string) (*ImportResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat path %s: %w", path, err)
	}

	result := &ImportResult{}

	if info.IsDir() {
		return s.importDirectory(path, result)
	}
	return s.importFile(path, result)
}

func (s *ImportService) importDirectory(dir string, result *ImportResult) (*ImportResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(entry.Name())
		if !strings.HasSuffix(ext, ".yaml") && !strings.HasSuffix(ext, ".yml") && !strings.HasSuffix(ext, ".json") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		if _, err := s.importFile(fullPath, result); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}
	return result, nil
}

func (s *ImportService) importFile(path string, result *ImportResult) (*ImportResult, error) {
	result.FilesProcessed++

	pack, err := s.reader.ReadPack(path)
	if err != nil {
		return result, fmt.Errorf("read pack %s: %w", path, err)
	}

	// Normalise before validation and hashing.
	domain.NormalizePack(pack)

	// Validate.
	validationErrors := domain.ValidatePack(pack, path)
	if len(validationErrors) > 0 {
		for _, ve := range validationErrors {
			result.Errors = append(result.Errors, ve.Error())
		}
		result.Invalid += len(validationErrors)
		return result, fmt.Errorf("validation failed for %s (%d errors)", path, len(validationErrors))
	}

	// Upsert topic.
	topic, err := s.topics.UpsertBySlug(pack.Topic.Slug, pack.Topic.Name)
	if err != nil {
		return result, fmt.Errorf("upsert topic %q: %w", pack.Topic.Slug, err)
	}

	// Convert pack questions to domain questions.
	questions := make([]domain.Question, 0, len(pack.Questions))
	for i := range pack.Questions {
		pq := &pack.Questions[i]
		confidence := 1.0
		if pq.Confidence != nil {
			confidence = *pq.Confidence
		}
		source := pq.Source
		if source == "" {
			source = "manual:file"
		}

		var rationale domain.Rationale
		if pq.Rationale != nil {
			rationale = *pq.Rationale
		}

		q := domain.Question{
			TopicID:          topic.ID,
			Type:             pq.Type,
			Intro:            pq.Intro,
			Prompt:           pq.Prompt,
			Choices:          pq.Choices,
			CorrectChoiceIDs: pq.CorrectChoiceIDs,
			Tags:             pq.Tags,
			Difficulty:       pq.Difficulty,
			Rationale:        rationale,
			Source:           source,
			SourceRef:        pq.SourceRef,
			Confidence:       confidence,
			Hash:             domain.ComputeQuestionHash(pack.Topic.Slug, pq),
		}
		questions = append(questions, q)
	}

	// Insert questions (duplicates skipped by hash constraint).
	insertResult, err := s.questions.InsertMany(questions)
	if err != nil {
		return result, fmt.Errorf("insert questions: %w", err)
	}

	result.Inserted += insertResult.Inserted
	result.Duplicates += insertResult.Duplicates
	return result, nil
}
