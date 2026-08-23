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
	"sort"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// assessment is the verdict on one candidate after the full chain.
type assessment struct {
	Candidate domain.Candidate
	Accepted  bool
	Repaired  bool
	Reason    string
}

// assess runs stages 5 through 8 on one candidate.
//
// The repair budget is spent at most once across the whole chain, not once per
// stage — D-016 permits "exactly one targeted repair and re-verification", and
// granting one per gate would quietly turn one repair into four.
func (p *Pipeline) assess(ctx context.Context, spec domain.GenerationSpec, evidence []domain.Evidence,
	candidate domain.Candidate, accepted []domain.Candidate) (assessment, domain.Cost, error) {

	var cost domain.Cost
	repairsLeft := p.budgets.RepairsPerCandidate
	repaired := false

	for {
		reason, err := p.screen(ctx, spec, evidence, candidate, accepted, &cost)
		if err != nil {
			return assessment{}, cost, err
		}
		if reason == "" {
			return assessment{Candidate: candidate, Accepted: true, Repaired: repaired}, cost, nil
		}
		if repairsLeft == 0 {
			return assessment{Accepted: false, Repaired: repaired, Reason: reason}, cost, nil
		}

		// Named `fixed`, not `repaired`: `repaired` is the outer flag, and
		// shadowing it here meant a successful repair was never recorded on
		// the assessment. Caught by TestARepairedCandidateIsAcceptedAfterReVerification.
		fixed, err := p.repair(ctx, spec, evidence, candidate, reason, &cost)
		if err != nil {
			return assessment{}, cost, err
		}
		repaired = true
		if fixed == nil {
			// The repair produced nothing usable. Discarding is correct: the
			// budget is spent, and a second attempt would be the second repair
			// D-016 does not permit.
			return assessment{Accepted: false, Repaired: true, Reason: reason}, cost, nil
		}
		candidate = *fixed
		repairsLeft--
	}
}

// screen runs the gates in cost order: the free deterministic one first, then
// the two that each cost a model call, then the similarity gate. A candidate
// that fails schema validation must never reach a provider — that would be
// paying to verify something already known to be invalid.
func (p *Pipeline) screen(ctx context.Context, spec domain.GenerationSpec, evidence []domain.Evidence,
	candidate domain.Candidate, accepted []domain.Candidate, cost *domain.Cost) (reason string, err error) {

	// Stage 5: deterministic schema and domain validation — the same rules a
	// hand-authored pack faces.
	if errs := coredomain.ValidateQuestion(&candidate.Question, 0, ""); len(errs) > 0 {
		return fmt.Sprintf("failed validation: %s", errs[0].Message), nil
	}
	if reason := p.checkCitations(candidate, evidence); reason != "" {
		return reason, nil
	}

	// Stage 6: verification in a fresh context on the same provider and model.
	ok, detail, verifyCost, err := p.verify(ctx, candidate)
	*cost = addCost(*cost, verifyCost)
	if err != nil {
		return "", err
	}
	if !ok {
		return fmt.Sprintf("answer key rejected on verification: %s", detail), nil
	}

	// Stage 7a: critique for grounding and quality.
	ok, detail, critiqueCost, err := p.critique(ctx, spec, evidence, candidate)
	*cost = addCost(*cost, critiqueCost)
	if err != nil {
		return "", err
	}
	if !ok {
		return fmt.Sprintf("rejected by critique: %s", detail), nil
	}

	// Stage 7b: the near-duplicate gate.
	if p.deps.Gate != nil {
		verdicts, err := p.deps.Gate.Screen(ctx, TopicSlug(spec.Topic), append(accepted, candidate))
		if err != nil {
			return "", fmt.Errorf("similarity gate: %w", err)
		}
		if len(verdicts) > 0 {
			last := verdicts[len(verdicts)-1]
			if last.TooSimilar {
				return fmt.Sprintf("too similar to existing content: %s", last.Reason), nil
			}
		}
	}

	return "", nil
}

// checkCitations enforces grounding deterministically, before spending a model
// call on it.
//
// FORGE.md 5 requires evidence-grounded generation and forbids silently
// degrading to ungrounded output. A citation naming an evidence id that was
// never supplied is the clearest possible signal that a question was invented
// rather than grounded, and it costs nothing to detect.
func (p *Pipeline) checkCitations(candidate domain.Candidate, evidence []domain.Evidence) string {
	if len(evidence) == 0 {
		// Nothing was supplied, so nothing can be cited. The absence of
		// grounding is recorded at the run level rather than blamed on the
		// candidate.
		return ""
	}
	known := make(map[string]bool, len(evidence))
	for _, e := range evidence {
		known[e.ID] = true
	}
	if len(candidate.Citations) == 0 {
		return "no evidence was cited"
	}
	for _, c := range candidate.Citations {
		if !known[strings.TrimSpace(c)] {
			return fmt.Sprintf("cited evidence %q was never supplied", c)
		}
	}
	return ""
}

const verifierSchema = `{
  "type": "object",
  "properties": {
    "correct_choice_ids": {"type": "array", "items": {"type": "string"}},
    "reasoning": {"type": "string"}
  },
  "required": ["correct_choice_ids", "reasoning"]
}`

// verifierSystemPrompt deliberately does not reveal the proposed answer. A
// verifier shown the key tends to agree with it, which would make this stage
// a confirmation rather than a check — the same tautology the repo's testing
// rules forbid.
const verifierSystemPrompt = `You are answering a multiple-choice question.

Choose the correct answer or answers from the choices given. Reply with the
choice ids only, plus one sentence of reasoning. Reply with JSON only.`

type verifierReply struct {
	CorrectChoiceIDs []string `json:"correct_choice_ids"`
	Reasoning        string   `json:"reasoning"`
}

// verify re-derives the answer key in a fresh context.
//
// "Fresh context" is structural rather than a discipline: ports.Provider has
// no session handle, so this call cannot see the generator's context. The
// verifier is asked to ANSWER the question, not to review the proposed key,
// and agreement is then checked here in Go.
func (p *Pipeline) verify(ctx context.Context, candidate domain.Candidate) (ok bool, detail string, cost domain.Cost, err error) {
	call, cancel := context.WithTimeout(ctx, p.budgets.PerCallTimeout)
	defer cancel()

	var reply verifierReply
	if err := p.deps.Provider.Generate(call, ports.Request{
		System: verifierSystemPrompt,
		User:   renderQuestionWithoutKey(candidate.Question),
		Schema: []byte(verifierSchema),
	}, &reply); err != nil {
		return false, "", domain.Cost{Attempts: 1}, fmt.Errorf("verify: %w", err)
	}

	if sameChoiceSet(reply.CorrectChoiceIDs, candidate.Question.CorrectChoiceIDs) {
		return true, "", domain.Cost{Attempts: 1}, nil
	}
	return false, fmt.Sprintf("independent answer was %s, the question claims %s",
			strings.Join(sorted(reply.CorrectChoiceIDs), ","),
			strings.Join(sorted(candidate.Question.CorrectChoiceIDs), ",")),
		domain.Cost{Attempts: 1}, nil
}

// renderQuestionWithoutKey shows the question and its choices and nothing else.
func renderQuestionWithoutKey(q coredomain.PackQuestion) string {
	var b strings.Builder
	if q.Intro != "" {
		b.WriteString(q.Intro + "\n\n")
	}
	b.WriteString(q.Prompt + "\n")
	for _, c := range q.Choices {
		fmt.Fprintf(&b, "%s. %s\n", c.ID, c.Text)
	}
	return b.String()
}

func sorted(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strings.TrimSpace(id))
	}
	sort.Strings(out)
	return out
}

// sameChoiceSet compares answer keys as sets, because choice order is
// presentation (D-008) and a verifier listing the same ids in another order
// agrees.
func sameChoiceSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa, sb := sorted(a), sorted(b)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
