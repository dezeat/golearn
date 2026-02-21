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

package domain

import (
	"fmt"
	"strings"
)

// ValidationError captures a single validation failure with context.
type ValidationError struct {
	File    string // source file path (if known)
	Index   int    // question index within the pack (0-based)
	Field   string // field name that failed
	Message string // human-readable description
}

// Error returns the formatted validation error string.
func (e ValidationError) Error() string {
	loc := fmt.Sprintf("question[%d]", e.Index)
	if e.File != "" {
		loc = fmt.Sprintf("%s: %s", e.File, loc)
	}
	return fmt.Sprintf("%s.%s: %s", loc, e.Field, e.Message)
}

// ValidatePack checks the pack-level fields and delegates to per-question validation.
// filePath is used only for error context.
func ValidatePack(p *Pack, filePath string) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(p.PackVersion) == "" {
		errs = append(errs, ValidationError{File: filePath, Index: -1, Field: "pack_version", Message: "required"})
	}
	if strings.TrimSpace(p.Topic.Slug) == "" {
		errs = append(errs, ValidationError{File: filePath, Index: -1, Field: "topic.slug", Message: "required"})
	}
	if strings.TrimSpace(p.Topic.Name) == "" {
		errs = append(errs, ValidationError{File: filePath, Index: -1, Field: "topic.name", Message: "required"})
	}
	if len(p.Questions) == 0 {
		errs = append(errs, ValidationError{File: filePath, Index: -1, Field: "questions", Message: "must contain at least one question"})
	}

	for i := range p.Questions {
		errs = append(errs, ValidateQuestion(&p.Questions[i], i, filePath)...)
	}
	return errs
}

// ValidateQuestion checks a single question against all canonical rules.
func ValidateQuestion(q *PackQuestion, index int, filePath string) []ValidationError {
	var errs []ValidationError
	ve := func(field, msg string) {
		errs = append(errs, ValidationError{File: filePath, Index: index, Field: field, Message: msg})
	}

	// Rule 1: supported type
	switch q.Type {
	case SingleSelect, MultiSelect:
		// ok
	default:
		ve("type", fmt.Sprintf("unsupported type %q (must be single_select or multi_select)", q.Type))
	}

	// Rule 7: non-empty prompt
	if strings.TrimSpace(q.Prompt) == "" {
		ve("prompt", "must be non-empty")
	}

	// Rule 2: at least 2 choices
	if len(q.Choices) < 2 {
		ve("choices", fmt.Sprintf("must have >= 2 choices, got %d", len(q.Choices)))
	}

	// Rule 3: unique choice IDs
	choiceIDs := make(map[string]bool, len(q.Choices))
	for _, c := range q.Choices {
		if c.ID == "" {
			ve("choices.id", "choice ID must be non-empty")
		}
		if choiceIDs[c.ID] {
			ve("choices.id", fmt.Sprintf("duplicate choice ID %q", c.ID))
		}
		choiceIDs[c.ID] = true
	}

	// Rule 4: correct_choice_ids non-empty
	if len(q.CorrectChoiceIDs) == 0 {
		ve("correct_choice_ids", "must be non-empty")
	}

	// Rule 5: single_select requires exactly one correct ID
	if q.Type == SingleSelect && len(q.CorrectChoiceIDs) != 1 {
		ve("correct_choice_ids", fmt.Sprintf("single_select requires exactly 1 correct ID, got %d", len(q.CorrectChoiceIDs)))
	}

	// Rule 6: correct IDs must reference existing choices
	for _, cid := range q.CorrectChoiceIDs {
		if !choiceIDs[cid] {
			ve("correct_choice_ids", fmt.Sprintf("references non-existent choice ID %q", cid))
		}
	}

	// Rule 8: difficulty must be easy|medium|hard if provided
	if q.Difficulty != DifficultyUnset && !ValidDifficulties[q.Difficulty] {
		ve("difficulty", fmt.Sprintf("must be easy, medium, or hard; got %q", q.Difficulty))
	}

	return errs
}
