package domain_test

import (
	"testing"

	"github.com/dezeat/golearn/internal/domain"
)

func TestStripExplanationPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no prefix", "Delta Lake uses WAL.", "Delta Lake uses WAL."},
		{"correct prefix", "Correct: Delta Lake uses WAL.", "Delta Lake uses WAL."},
		{"incorrect prefix", "Incorrect: This is wrong.", "This is wrong."},
		{"correct mixed case", "CORRECT: upper case prefix.", "upper case prefix."},
		{"emoji correct", "✅ This is right.", "This is right."},
		{"emoji incorrect", "❌ This is wrong.", "This is wrong."},
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},
		{"prefix with extra space", "Correct:   extra spaces.", "extra spaces."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.StripExplanationPrefix(tt.input)
			if got != tt.want {
				t.Errorf("StripExplanationPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatChoiceExplanation(t *testing.T) {
	correctIDs := []string{"1", "3"}

	tests := []struct {
		name     string
		choiceID string
		text     string
		want     string
	}{
		{"correct choice", "1", "This is the right answer.", "Correct: This is the right answer."},
		{"incorrect choice", "2", "This is wrong because X.", "Incorrect: This is wrong because X."},
		{"correct choice with old prefix", "3", "Correct: already prefixed.", "Correct: already prefixed."},
		{"incorrect choice with old prefix", "4", "Incorrect: was prefixed.", "Incorrect: was prefixed."},
		{"empty text", "1", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.FormatChoiceExplanation(tt.choiceID, tt.text, correctIDs)
			if got != tt.want {
				t.Errorf("FormatChoiceExplanation(%q, %q, ...) = %q, want %q",
					tt.choiceID, tt.text, got, tt.want)
			}
		})
	}
}
