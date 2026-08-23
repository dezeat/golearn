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
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// LibraryQuestionID is the id of a question that already lives in the library.
//
// It is a defined type rather than a bare int64 because the similarity gate
// handles two id spaces that are both integers and mean entirely different
// things: this one, which addresses a row the core owns and FORGE.md 7 forbids
// modifying, and a candidate's position within the pack being generated, which
// addresses something that does not exist yet and may never. Carrying both as
// int64 makes substituting one for the other a silent bug — the gate would
// report "too similar to library question 3" while meaning the fourth
// candidate. A defined type makes that substitution a compile error.
type LibraryQuestionID int64

// AsPackQuestion projects a stored question into the pack shape.
//
// The similarity gate compares stored library content with not-yet-stored
// candidates, and the comparison representation ([CanonicalText]) is defined
// over the pack shape. Projecting rather than writing a second canonicalizer
// is what keeps a library question and an identical candidate producing
// byte-identical text: two canonicalizers would drift, and the drift would
// show up as every library score sitting slightly below every candidate
// score — uniform false negatives, with nothing failing.
//
// Only the fields [CanonicalText] reads are carried. Fields it ignores are
// omitted deliberately rather than copied for symmetry, so that a future field
// added to the comparison has to be added here too and cannot arrive
// half-projected.
func AsPackQuestion(q coredomain.Question) coredomain.PackQuestion {
	return coredomain.PackQuestion{
		Type:             q.Type,
		Prompt:           q.Prompt,
		Choices:          q.Choices,
		CorrectChoiceIDs: q.CorrectChoiceIDs,
		Tags:             q.Tags,
		Difficulty:       q.Difficulty,
	}
}
