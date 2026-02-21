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

import "testing"

func TestEvaluateCorrectness_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		selected []string
		correct  []string
		want     bool
	}{
		{
			name:     "single correct",
			selected: []string{"A"},
			correct:  []string{"A"},
			want:     true,
		},
		{
			name:     "multi correct same order",
			selected: []string{"A", "C"},
			correct:  []string{"A", "C"},
			want:     true,
		},
		{
			name:     "multi correct different order",
			selected: []string{"C", "A"},
			correct:  []string{"A", "C"},
			want:     true,
		},
		{
			name:     "wrong answer",
			selected: []string{"B"},
			correct:  []string{"A"},
			want:     false,
		},
		{
			name:     "subset not correct",
			selected: []string{"A"},
			correct:  []string{"A", "C"},
			want:     false,
		},
		{
			name:     "superset not correct",
			selected: []string{"A", "B", "C"},
			correct:  []string{"A", "C"},
			want:     false,
		},
		{
			name:     "empty selected vs non-empty correct",
			selected: []string{},
			correct:  []string{"A"},
			want:     false,
		},
		{
			name:     "nil selected vs non-empty correct",
			selected: nil,
			correct:  []string{"A"},
			want:     false,
		},
		{
			name:     "both empty",
			selected: []string{},
			correct:  []string{},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCorrectness(tt.selected, tt.correct)
			if got != tt.want {
				t.Errorf("EvaluateCorrectness(%v, %v) = %v, want %v",
					tt.selected, tt.correct, got, tt.want)
			}
		})
	}
}
