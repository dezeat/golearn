package domain

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// NormalizeText trims whitespace and normalises line endings to \n.
func NormalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// NormalizePack normalises all string fields in a pack in-place.
func NormalizePack(p *Pack) {
	p.Topic.Slug = strings.TrimSpace(p.Topic.Slug)
	p.Topic.Name = strings.TrimSpace(p.Topic.Name)
	for i := range p.Questions {
		NormalizePackQuestion(&p.Questions[i])
	}
}

// NormalizePackQuestion normalises a single pack question in-place.
func NormalizePackQuestion(q *PackQuestion) {
	q.Intro = NormalizeText(q.Intro)
	q.Prompt = NormalizeText(q.Prompt)
	for i := range q.Choices {
		q.Choices[i].ID = strings.TrimSpace(q.Choices[i].ID)
		q.Choices[i].Text = NormalizeText(q.Choices[i].Text)
	}
	for i := range q.CorrectChoiceIDs {
		q.CorrectChoiceIDs[i] = strings.TrimSpace(q.CorrectChoiceIDs[i])
	}
	for i := range q.Tags {
		q.Tags[i] = strings.TrimSpace(q.Tags[i])
	}
}

// ComputeQuestionHash produces a stable SHA-256 hex digest for deduplication.
// Fields are joined with the null byte separator (\x00).
// correct_choice_ids are sorted before hashing so order doesn't matter.
func ComputeQuestionHash(topicSlug string, q *PackQuestion) string {
	sep := "\x00"
	h := sha256.New()

	// topic slug
	h.Write([]byte(topicSlug))
	h.Write([]byte(sep))

	// type
	h.Write([]byte(string(q.Type)))
	h.Write([]byte(sep))

	// intro (normalised)
	h.Write([]byte(NormalizeText(q.Intro)))
	h.Write([]byte(sep))

	// prompt (normalised)
	h.Write([]byte(NormalizeText(q.Prompt)))
	h.Write([]byte(sep))

	// choices in order: id + text
	for _, c := range q.Choices {
		h.Write([]byte(strings.TrimSpace(c.ID)))
		h.Write([]byte(NormalizeText(c.Text)))
		h.Write([]byte(sep))
	}

	// correct_choice_ids sorted
	sorted := make([]string, len(q.CorrectChoiceIDs))
	copy(sorted, q.CorrectChoiceIDs)
	sort.Strings(sorted)
	h.Write([]byte(strings.Join(sorted, ",")))
	h.Write([]byte(sep))

	// difficulty (included so changing it creates a new hash)
	h.Write([]byte(string(q.Difficulty)))

	return fmt.Sprintf("%x", h.Sum(nil))
}
