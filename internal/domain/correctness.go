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

// Package domain — correctness.go centralises answer evaluation logic.
// Correctness is an exact, order-insensitive set match between
// the user's selected choice IDs and the question's correct_choice_ids.
package domain

import "sort"

// EvaluateCorrectness returns true when selectedIDs exactly match
// correctIDs (order-insensitive). A skipped attempt is never correct.
func EvaluateCorrectness(selectedIDs, correctIDs []string) bool {
	if len(selectedIDs) != len(correctIDs) {
		return false
	}
	// Sort copies so callers' slices are not mutated.
	s := make([]string, len(selectedIDs))
	copy(s, selectedIDs)
	sort.Strings(s)

	c := make([]string, len(correctIDs))
	copy(c, correctIDs)
	sort.Strings(c)

	for i := range s {
		if s[i] != c[i] {
			return false
		}
	}
	return true
}
