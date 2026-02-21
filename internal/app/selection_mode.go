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

// Package app — selection_mode.go defines the question selection modes.
//
// Modes control how questions are chosen for a practice session:
//   - Balanced:      unseen → weak → random fill (default, formerly "practice")
//   - ByDifficulty:  filter to a chosen difficulty bucket, then apply Balanced logic
//   - Weakest:       focus on weakest tag or weakest questions
package app

import "encoding/json"

// SelectionMode enumerates supported question selection strategies.
type SelectionMode string

const (
	// ModeBalanced selects questions using unseen → weak → random fill.
	ModeBalanced SelectionMode = "balanced"
	// ModeByDifficulty filters to a chosen difficulty, then applies Balanced.
	ModeByDifficulty SelectionMode = "by_difficulty"
	// ModeWeakest focuses on the user's weakest tag or questions.
	ModeWeakest SelectionMode = "weakest"
)

// ValidModes lists all accepted selection modes.
var ValidModes = map[SelectionMode]bool{
	ModeBalanced:     true,
	ModeByDifficulty: true,
	ModeWeakest:      true,
}

// ModeDisplayName returns the user-facing label for a mode.
func ModeDisplayName(m SelectionMode) string {
	switch m {
	case ModeBalanced:
		return "Balanced"
	case ModeByDifficulty:
		return "By Difficulty"
	case ModeWeakest:
		return "Weakest"
	default:
		return string(m)
	}
}

// WeakestSubMode distinguishes between tag-based and question-based weakest.
type WeakestSubMode string

const (
	// WeakestByTag selects questions from the weakest tag.
	WeakestByTag WeakestSubMode = "by_tag"
	// WeakestByQuestion selects individual questions with the highest wrong rate.
	WeakestByQuestion WeakestSubMode = "by_question"
)

// SessionConfig holds all parameters needed to start a session.
type SessionConfig struct {
	TopicSlug string
	N         int
	Mode      SelectionMode

	// ByDifficulty params — only used when Mode == ModeByDifficulty.
	Difficulty string // "easy", "medium", or "hard"

	// Weakest params — only used when Mode == ModeWeakest.
	WeakestSub WeakestSubMode
}

// ModeParams is the serialisable form of mode-specific parameters,
// persisted as JSON in the sessions table.
type ModeParams struct {
	Difficulty string         `json:"difficulty,omitempty"`
	WeakestSub WeakestSubMode `json:"weakest_sub,omitempty"`
	WeakestTag string         `json:"weakest_tag,omitempty"` // resolved tag name
}

// ModeParamsJSON encodes mode params to a JSON string.
func ModeParamsJSON(p ModeParams) string {
	b, _ := json.Marshal(p)
	return string(b)
}

// ModeLabel builds a one-line context string for display in the quiz header.
func ModeLabel(mode SelectionMode, params ModeParams) string {
	switch mode {
	case ModeByDifficulty:
		return "Mode: Difficulty (" + params.Difficulty + ")"
	case ModeWeakest:
		if params.WeakestTag != "" {
			return "Mode: Weakest (tag: " + params.WeakestTag + ")"
		}
		return "Mode: Weakest (questions)"
	default:
		return "Mode: Balanced"
	}
}
