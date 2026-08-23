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

const critiqueSchema = `{
  "type": "object",
  "properties": {
    "grounded": {"type": "boolean"},
    "distractors_plausible": {"type": "boolean"},
    "single_defensible_answer": {"type": "boolean"},
    "problem": {"type": "string"}
  },
  "required": ["grounded", "distractors_plausible", "single_defensible_answer", "problem"]
}`

// critiqueSystemPrompt asks for named defects rather than a score.
//
// A numeric quality score would need a threshold, and a threshold picked by
// feel is exactly what the repo's testing rules forbid. Three boolean
// properties are each independently checkable and each map to a defect a
// reader could confirm by looking at the question.
const critiqueSystemPrompt = `You are reviewing one multiple-choice question against the evidence it was written from.

Judge only these, strictly:
- grounded: every claim in the question and its answer is supported by the supplied evidence.
- distractors_plausible: the wrong choices are plausible to someone who has not learned the material, and are definitely wrong.
- single_defensible_answer: exactly the marked choices are defensible; no unmarked choice is also arguably correct.

If any is false, state the single most serious problem in one sentence. If all are true, leave problem empty.
Reply with JSON only.`

type critiqueReply struct {
	Grounded               bool   `json:"grounded"`
	DistractorsPlausible   bool   `json:"distractors_plausible"`
	SingleDefensibleAnswer bool   `json:"single_defensible_answer"`
	Problem                string `json:"problem"`
}

// ungroundedCritiqueSystemPrompt drops the grounding question entirely rather
// than asking it with nothing to answer from.
const ungroundedCritiqueSystemPrompt = `You are reviewing one multiple-choice question.

Judge only these, strictly:
- distractors_plausible: the wrong choices are plausible to someone who has not learned the material, and are definitely wrong.
- single_defensible_answer: exactly the marked choices are defensible; no unmarked choice is also arguably correct.

If either is false, state the single most serious problem in one sentence. If both are true, leave problem empty.
Reply with JSON only.`

const ungroundedCritiqueSchema = `{
  "type": "object",
  "properties": {
    "distractors_plausible": {"type": "boolean"},
    "single_defensible_answer": {"type": "boolean"},
    "problem": {"type": "string"}
  },
  "required": ["distractors_plausible", "single_defensible_answer", "problem"]
}`

// critique runs the source-grounding and quality gate.
//
// With no evidence supplied, the grounding question is not asked and its
// answer is not read. The live lane found this the hard way: the critic was
// handed an empty evidence set, correctly reported that nothing supported the
// question, and every candidate was rejected — a pipeline that could not
// succeed while research was unwired, presenting as a model quality problem.
//
// Not asking is the honest form. Asking and ignoring the answer would be a
// gate whose verdict nobody acts on, and asking with nothing to judge against
// invites the model to invent one. The absence of grounding is recorded at the
// run level, which is where it belongs.
func (p *Pipeline) critique(ctx context.Context, _ domain.GenerationSpec, evidence []domain.Evidence,
	candidate domain.Candidate) (ok bool, detail string, cost domain.Cost, err error) {

	call, cancel := context.WithTimeout(ctx, p.budgets.PerCallTimeout)
	defer cancel()

	cited := citedEvidence(evidence, candidate.Citations)
	grounded := len(cited) > 0

	system, schema := critiqueSystemPrompt, critiqueSchema
	if !grounded {
		system, schema = ungroundedCritiqueSystemPrompt, ungroundedCritiqueSchema
	}

	var reply critiqueReply
	if err := p.deps.Provider.Generate(call, ports.Request{
		System:   system,
		User:     renderQuestionWithKey(candidate.Question),
		Evidence: cited,
		Schema:   []byte(schema),
	}, &reply); err != nil {
		return false, "", domain.Cost{Attempts: 1}, fmt.Errorf("critique: %w", err)
	}

	cost = domain.Cost{Attempts: 1}
	switch {
	case grounded && !reply.Grounded:
		return false, firstNonEmpty(reply.Problem, "not supported by the evidence"), cost, nil
	case !reply.SingleDefensibleAnswer:
		return false, firstNonEmpty(reply.Problem, "more than one choice is defensible"), cost, nil
	case !reply.DistractorsPlausible:
		return false, firstNonEmpty(reply.Problem, "the wrong choices are not plausible"), cost, nil
	}
	return true, "", cost, nil
}

// citedEvidence narrows the evidence to what the candidate actually cited, so
// the critic judges grounding against the sources claimed rather than against
// everything retrieved. Passing all of it would let a question be called
// grounded on the strength of a source it never used.
func citedEvidence(evidence []domain.Evidence, citations []string) []domain.Evidence {
	if len(citations) == 0 {
		return evidence
	}
	cited := make(map[string]bool, len(citations))
	for _, c := range citations {
		cited[strings.TrimSpace(c)] = true
	}
	out := make([]domain.Evidence, 0, len(citations))
	for _, e := range evidence {
		if cited[e.ID] {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return evidence
	}
	return out
}

func renderQuestionWithKey(q coredomain.PackQuestion) string {
	correct := make(map[string]bool, len(q.CorrectChoiceIDs))
	for _, id := range q.CorrectChoiceIDs {
		correct[strings.TrimSpace(id)] = true
	}
	var b strings.Builder
	b.WriteString(q.Prompt + "\n")
	for _, c := range q.Choices {
		marker := " "
		if correct[c.ID] {
			marker = "*"
		}
		fmt.Fprintf(&b, "%s %s. %s\n", marker, c.ID, c.Text)
	}
	if q.Rationale != nil && q.Rationale.Correct != "" {
		fmt.Fprintf(&b, "\nExplanation given: %s\n", q.Rationale.Correct)
	}
	b.WriteString("\n(* marks the choices the question claims are correct.)")
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unspecified"
}

const repairSystemPrompt = `You are fixing one multiple-choice question that failed review.

Make the smallest change that resolves the stated problem. Keep the question on
the same point unless the problem is that the point itself is unsupported.
Every claim must remain supported by the supplied evidence. Reply with JSON only.`

// repair spends the single permitted repair attempt.
//
// It returns nil when the repair produced nothing usable, which the caller
// treats as a discard rather than as an error: a failed repair is an ordinary
// outcome of a bounded budget, not a fault in the pipeline.
func (p *Pipeline) repair(ctx context.Context, spec domain.GenerationSpec, evidence []domain.Evidence,
	candidate domain.Candidate, problem string, cost *domain.Cost) (*domain.Candidate, error) {

	call, cancel := context.WithTimeout(ctx, p.budgets.PerCallTimeout)
	defer cancel()

	var batch generatedBatch
	err := p.deps.Provider.Generate(call, ports.Request{
		System: repairSystemPrompt,
		User: fmt.Sprintf("Problem to fix: %s\n\nThe question:\n%s\n\nReturn exactly one corrected question.",
			problem, renderQuestionWithKey(candidate.Question)),
		Evidence: citedEvidence(evidence, candidate.Citations),
		Schema:   []byte(candidateSchema),
	}, &batch)
	*cost = addCost(*cost, domain.Cost{Attempts: 1})
	if err != nil {
		// A provider failure during repair aborts the run rather than silently
		// discarding the candidate: the two are different facts, and only one
		// of them is about the candidate.
		return nil, fmt.Errorf("repair: %w", err)
	}
	if len(batch.Questions) == 0 {
		return nil, nil
	}

	fixed := toCandidate(spec, batch.Questions[0])
	if len(fixed.Citations) == 0 {
		// Keep the original citations rather than letting a repair silently
		// drop grounding.
		fixed.Citations = candidate.Citations
	}
	return &fixed, nil
}
