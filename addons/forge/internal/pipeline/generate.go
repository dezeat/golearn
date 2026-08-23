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

package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// candidateSchema is the structured-output contract for generation.
//
// It is deliberately not the pack schema. The model is asked for the smallest
// shape that carries a question plus its citations; mapping that onto the pack
// format is this package's job, so a change to the pack schema does not
// silently change what the model is asked for.
const candidateSchema = `{
  "type": "object",
  "properties": {
    "questions": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "prompt": {"type": "string"},
          "choices": {"type": "array", "items": {"type": "string"}},
          "correct_choices": {"type": "array", "items": {"type": "integer"}},
          "explanation": {"type": "string"},
          "tags": {"type": "array", "items": {"type": "string"}},
          "citations": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["prompt", "choices", "correct_choices", "explanation", "citations"]
      }
    }
  },
  "required": ["questions"]
}`

// generatedQuestion asks for choices as plain strings and the answer as
// positions, not as objects carrying ids the model invents.
//
// Choice ids are arbitrary internal labels — display labels are computed from
// display order at render time (D-008) — so having the model produce them adds
// a degree of freedom that can only go wrong: an id repeated, an answer
// referencing an id that was never emitted. Positions cannot dangle, and the
// pipeline assigns ids deterministically.
//
// It is also markedly cheaper. On the CPU-only reference host, generation is
// the longest call in the chain and its cost scales with tokens produced;
// dropping a nesting level and an id per choice removes roughly a third of the
// output for the same content (B-2.2).
type generatedQuestion struct {
	Prompt         string   `json:"prompt"`
	Choices        []string `json:"choices"`
	CorrectChoices []int    `json:"correct_choices"`
	Explanation    string   `json:"explanation"`
	Tags           []string `json:"tags"`
	Citations      []string `json:"citations"`
}

type generatedBatch struct {
	Questions []generatedQuestion `json:"questions"`
}

// generatorSystemPrompt is operator-authored instruction. Retrieved material
// never appears here — it travels as fenced evidence in its own turn, which is
// the prompt-injection boundary FORGE.md 4 requires.
const generatorSystemPrompt = `You write multiple-choice practice questions for a learning tool.

Rules:
- Every question must be answerable from the supplied evidence. Do not use outside knowledge.
- Cite the evidence id you used for each question.
- choices is a list of answer texts. correct_choices holds the ZERO-BASED positions of the correct ones.
- Exactly one correct position per question unless told otherwise.
- Distractors must be plausible and wrong, never obviously absurd and never partially correct.
- The explanation states why the correct answer is correct. It contains no prefix such as "Correct:" and no emoji.
- Write content only. Do not address the reader or refer to "the evidence" in the question text.
- Reply with JSON only.`

// generateCandidates asks for the questions still missing from the pack.
func (p *Pipeline) generateCandidates(ctx context.Context, spec domain.GenerationSpec,
	evidence []domain.Evidence, want int, already []domain.Candidate) ([]domain.Candidate, domain.Cost, error) {

	// Ask for exactly what is missing.
	//
	// Over-asking to absorb downstream rejections is the obvious optimization,
	// and measurement says it is wrong here. Generation is the longest call in
	// the chain and its cost scales with the tokens produced, so asking for
	// spares inflates the single most expensive call on every round — on the
	// reference host a four-question batch approaches the per-call budget
	// while a two-question batch sits comfortably inside it (B-2.1). The round
	// budget already covers the rejection case, and unlike an over-ask it only
	// spends time when rejections actually happen.
	ask := want

	call, cancel := context.WithTimeout(ctx, p.budgets.PerCallTimeout)
	defer cancel()

	var batch generatedBatch
	err := p.deps.Provider.Generate(call, ports.Request{
		System:   generatorSystemPrompt,
		User:     generatorUserPrompt(spec, ask, already),
		Evidence: evidence,
		Schema:   []byte(candidateSchema),
	}, &batch)
	cost := domain.Cost{Attempts: 1}
	if err != nil {
		return nil, cost, fmt.Errorf("generate candidates: %w", err)
	}

	candidates := make([]domain.Candidate, 0, len(batch.Questions))
	for _, q := range batch.Questions {
		candidates = append(candidates, toCandidate(spec, q))
	}
	return candidates, cost, nil
}

// generatorUserPrompt states the request, and lists the prompts already
// accepted so the model does not spend a round re-proposing them.
func generatorUserPrompt(spec domain.GenerationSpec, want int, already []domain.Candidate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Write %d multiple-choice questions about: %s\n", want, spec.Topic)
	if d := strings.TrimSpace(spec.Description); d != "" {
		fmt.Fprintf(&b, "Focus: %s\n", d)
	}
	if spec.Difficulty != coredomain.DifficultyUnset {
		fmt.Fprintf(&b, "Difficulty: %s\n", spec.Difficulty)
	}
	if spec.Language != "" {
		fmt.Fprintf(&b, "Language: %s\n", spec.Language)
	}
	if len(already) > 0 {
		b.WriteString("\nThese questions already exist. Write different ones, on different points:\n")
		for _, c := range already {
			fmt.Fprintf(&b, "- %s\n", c.Question.Prompt)
		}
	}
	return b.String()
}

// choiceID is the deterministic label for the nth choice: a, b, c, …
//
// Deterministic because the id feeds the D-007 content hash, so the same
// question generated twice must produce the same ids or dedup would stop
// recognizing it.
func choiceID(index int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	if index < 0 || index >= len(alphabet) {
		return fmt.Sprintf("c%d", index)
	}
	return string(alphabet[index])
}

// toCandidate maps a generated question onto the pack format, assigning choice
// ids and resolving answer positions.
//
// An out-of-range answer position is dropped rather than clamped: clamping
// would silently mark a different choice correct, which is the single worst
// defect a practice tool can ship. A question left with no correct answer
// fails deterministic validation immediately afterwards, which is the visible
// outcome.
func toCandidate(spec domain.GenerationSpec, q generatedQuestion) domain.Candidate {
	choices := make([]coredomain.Choice, 0, len(q.Choices))
	for i, text := range q.Choices {
		choices = append(choices, coredomain.Choice{ID: choiceID(i), Text: text})
	}

	correct := make([]string, 0, len(q.CorrectChoices))
	seen := map[int]bool{}
	for _, position := range q.CorrectChoices {
		if position < 0 || position >= len(choices) || seen[position] {
			continue
		}
		seen[position] = true
		correct = append(correct, choices[position].ID)
	}

	questionType := coredomain.SingleSelect
	if len(correct) > 1 {
		questionType = coredomain.MultiSelect
	}
	pq := coredomain.PackQuestion{
		Type:             questionType,
		Prompt:           q.Prompt,
		Choices:          choices,
		CorrectChoiceIDs: correct,
		Tags:             q.Tags,
		Difficulty:       spec.Difficulty,
	}
	if strings.TrimSpace(q.Explanation) != "" {
		// Explanations are content-only; the correctness prefix is presentation
		// added at render time (D-008).
		pq.Rationale = &coredomain.Rationale{Correct: q.Explanation}
	}
	coredomain.NormalizePackQuestion(&pq)
	return domain.Candidate{Question: pq, Citations: q.Citations}
}
