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
          "choices": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": {"type": "string"},
                "text": {"type": "string"}
              },
              "required": ["id", "text"]
            }
          },
          "correct_choice_ids": {"type": "array", "items": {"type": "string"}},
          "explanation": {"type": "string"},
          "tags": {"type": "array", "items": {"type": "string"}},
          "citations": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["prompt", "choices", "correct_choice_ids", "explanation", "citations"]
      }
    }
  },
  "required": ["questions"]
}`

type generatedQuestion struct {
	Prompt           string              `json:"prompt"`
	Choices          []coredomain.Choice `json:"choices"`
	CorrectChoiceIDs []string            `json:"correct_choice_ids"`
	Explanation      string              `json:"explanation"`
	Tags             []string            `json:"tags"`
	Citations        []string            `json:"citations"`
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
- Exactly one correct choice per question unless told otherwise.
- Distractors must be plausible and wrong, never obviously absurd and never partially correct.
- The explanation states why the correct answer is correct. It contains no prefix such as "Correct:" and no emoji.
- Write content only. Do not address the reader or refer to "the evidence" in the question text.
- Reply with JSON only.`

// generateCandidates asks for the questions still missing from the pack.
func (p *Pipeline) generateCandidates(ctx context.Context, spec domain.GenerationSpec,
	evidence []domain.Evidence, want int, already []domain.Candidate) ([]domain.Candidate, domain.Cost, error) {

	// Ask for a little more than is missing, because some candidates will be
	// rejected downstream and a second round costs a full model call. The
	// surplus is capped so a large shortfall cannot inflate one request into
	// something the model answers badly.
	const maxOverAsk = 3
	ask := want + min(want, maxOverAsk)

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

// toCandidate maps a generated question onto the pack format.
func toCandidate(spec domain.GenerationSpec, q generatedQuestion) domain.Candidate {
	questionType := coredomain.SingleSelect
	if len(q.CorrectChoiceIDs) > 1 {
		questionType = coredomain.MultiSelect
	}
	pq := coredomain.PackQuestion{
		Type:             questionType,
		Prompt:           q.Prompt,
		Choices:          q.Choices,
		CorrectChoiceIDs: q.CorrectChoiceIDs,
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
