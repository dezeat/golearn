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

// Package domain defines the core types for golearn.
// All types here are persistence-agnostic.
package domain

import "time"

// QuestionType enumerates supported MCQ types.
type QuestionType string

const (
	// SingleSelect allows exactly one correct answer.
	SingleSelect QuestionType = "single_select"
	// MultiSelect allows one or more correct answers.
	MultiSelect QuestionType = "multi_select"
)

// Difficulty represents the difficulty level of a question.
type Difficulty string

const (
	// DifficultyEasy marks a question as easy.
	DifficultyEasy Difficulty = "easy"
	// DifficultyMedium marks a question as medium.
	DifficultyMedium Difficulty = "medium"
	// DifficultyHard marks a question as hard.
	DifficultyHard Difficulty = "hard"
	// DifficultyUnset means the pack author omitted difficulty.
	DifficultyUnset Difficulty = "" // optional; packs may omit difficulty
)

// ValidDifficulties is the set of allowed non-empty difficulty values.
var ValidDifficulties = map[Difficulty]bool{
	DifficultyEasy:   true,
	DifficultyMedium: true,
	DifficultyHard:   true,
}

// Choice represents one option in a multiple-choice question.
type Choice struct {
	ID   string `json:"id"   yaml:"id"`
	Text string `json:"text" yaml:"text"`
}

// Rationale holds optional explanation text (reserved for future use).
type Rationale struct {
	Correct   string            `json:"correct,omitempty"   yaml:"correct,omitempty"`
	PerChoice map[string]string `json:"per_choice,omitempty" yaml:"per_choice,omitempty"`
}

// Topic groups questions under a named category.
type Topic struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// User represents a local profile.
type User struct {
	ID          int64     `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Question is the canonical MCQ domain model.
type Question struct {
	ID               int64        `json:"id"`
	TopicID          int64        `json:"topic_id"`
	Type             QuestionType `json:"type"`
	Intro            string       `json:"intro,omitempty"`
	Prompt           string       `json:"prompt"`
	Choices          []Choice     `json:"choices"`
	CorrectChoiceIDs []string     `json:"correct_choice_ids"`
	Tags             []string     `json:"tags,omitempty"`
	Difficulty       Difficulty   `json:"difficulty,omitempty"`
	Rationale        Rationale    `json:"rationale,omitempty"`
	Source           string       `json:"source,omitempty"`
	SourceRef        string       `json:"source_ref,omitempty"`
	Confidence       float64      `json:"confidence"`
	Hash             string       `json:"hash"`
	CreatedAt        time.Time    `json:"created_at"`
}

// Session tracks a practice or exam run (stubbed for future use).
type Session struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	TopicID        int64      `json:"topic_id"`
	Mode           string     `json:"mode"`
	ModeParamsJSON string     `json:"mode_params_json,omitempty"`
	RequestedN     int        `json:"requested_n"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
}

// Attempt records a single answer within a session (stubbed for future use).
type Attempt struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	SessionID         int64     `json:"session_id"`
	QuestionID        int64     `json:"question_id"`
	SelectedChoiceIDs []string  `json:"selected_choice_ids"`
	Correct           bool      `json:"correct"`
	Skipped           bool      `json:"skipped"`
	LatencyMs         int       `json:"latency_ms"`
	CreatedAt         time.Time `json:"created_at"`
}

// Pack is the in-memory representation of a question pack file.
type Pack struct {
	PackVersion string         `json:"pack_version" yaml:"pack_version"`
	Topic       PackTopic      `json:"topic"        yaml:"topic"`
	Questions   []PackQuestion `json:"questions" yaml:"questions"`
}

const (
	// CurrentPackVersion is the pack schema version emitted by export.
	CurrentPackVersion = "0.1.0"

	// SupportedPackMajor is the accepted pack schema major version for import.
	SupportedPackMajor = 0
	// SupportedPackMinor is the accepted pack schema minor version for import.
	SupportedPackMinor = 1
)

// PackTopic identifies the topic inside a pack file.
type PackTopic struct {
	Slug string `json:"slug" yaml:"slug"`
	Name string `json:"name" yaml:"name"`
}

// PackQuestion is the per-question schema inside a pack file.
type PackQuestion struct {
	Type             QuestionType `json:"type"               yaml:"type"`
	Intro            string       `json:"intro,omitempty"    yaml:"intro,omitempty"`
	Prompt           string       `json:"prompt"             yaml:"prompt"`
	Choices          []Choice     `json:"choices"            yaml:"choices"`
	CorrectChoiceIDs []string     `json:"correct_choice_ids" yaml:"correct_choice_ids"`
	Tags             []string     `json:"tags,omitempty"     yaml:"tags,omitempty"`
	Difficulty       Difficulty   `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Rationale        *Rationale   `json:"rationale,omitempty"  yaml:"rationale,omitempty"`
	Source           string       `json:"source,omitempty"   yaml:"source,omitempty"`
	SourceRef        string       `json:"source_ref,omitempty" yaml:"source_ref,omitempty"`
	Confidence       *float64     `json:"confidence,omitempty" yaml:"confidence,omitempty"`
}
