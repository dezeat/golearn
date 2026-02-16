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
