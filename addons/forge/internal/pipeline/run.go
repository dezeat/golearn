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

// run executes the stage chain. It is separated from Generate so the run
// record's terminal state is written in exactly one place regardless of which
// stage failed.
func (p *Pipeline) run(ctx context.Context, spec domain.GenerationSpec, _ int64) (
	result Result, sources []domain.SourceRef, cost domain.Cost, err error) {

	identity := p.deps.Provider.Identity()

	// Stage 2-3: research and evidence records.
	evidence, err := p.gatherEvidence(ctx, spec)
	if err != nil {
		return Result{}, nil, cost, err
	}
	for _, e := range evidence {
		sources = append(sources, e.Ref())
	}

	var accepted []domain.Candidate
	seenPrompts := map[string]bool{}

	for round := 0; round < p.budgets.MaxCandidateRounds && len(accepted) < spec.Count; round++ {
		result.Rounds = round + 1
		missing := spec.Count - len(accepted)

		// Stage 4: generate candidates grounded in the evidence.
		candidates, genCost, err := p.generateCandidates(ctx, spec, evidence, missing, accepted)
		cost = addCost(cost, genCost)
		if err != nil {
			return Result{}, sources, cost, err
		}

		for i := range candidates {
			candidate := candidates[i]

			// Stage 5: deterministic validation, then verification, critique
			// and similarity. One bounded repair is permitted across the
			// whole chain, which is why the attempt is threaded rather than
			// re-granted per stage.
			outcome, attemptCost, err := p.assess(ctx, spec, evidence, candidate, accepted)
			cost = addCost(cost, attemptCost)
			if err != nil {
				return Result{}, sources, cost, err
			}
			if outcome.Repaired {
				result.Repaired++
			}
			if !outcome.Accepted {
				result.Rejected++
				continue
			}

			// An identical prompt inside one pack is a duplicate the exact
			// hash would also catch on import, but catching it here keeps the
			// pack itself honest about its own count.
			key := strings.ToLower(coredomain.NormalizeText(outcome.Candidate.Question.Prompt))
			if seenPrompts[key] {
				result.Rejected++
				continue
			}
			seenPrompts[key] = true

			accepted = append(accepted, outcome.Candidate)
			if len(accepted) == spec.Count {
				break
			}
		}
	}

	result.Accepted = len(accepted)
	if len(accepted) < spec.Count {
		// Fail clear. D-016 forbids emitting a silently degraded pack, so a
		// shortfall is an error carrying the numbers rather than a smaller
		// pack that looks complete.
		return Result{}, sources, cost, fmt.Errorf(
			"%w: produced %d of %d after %d round(s); %d candidate(s) were rejected by validation, verification, critique or the similarity gate",
			ErrShortfall, len(accepted), spec.Count, result.Rounds, result.Rejected)
	}

	result.Draft = domain.Draft{Pack: p.packFrom(spec, accepted, sources, identity)}
	return result, sources, cost, nil
}

// gatherEvidence runs the research stage.
//
// FORGE.md 5 requires grounding in V1 and forbids silently degrading to
// ungrounded generation. With no Research adapter wired, this returns nothing
// and the caller records that in the run diagnostic — the absence is visible
// rather than invisible.
func (p *Pipeline) gatherEvidence(ctx context.Context, spec domain.GenerationSpec) ([]domain.Evidence, error) {
	if p.deps.Research == nil {
		return nil, nil
	}
	query := ports.Query{
		Terms:             researchTerms(spec),
		MaxResults:        p.budgets.EvidencePerRun,
		MaxBytesPerSource: p.budgets.MaxBytesPerSource,
		Timeout:           p.budgets.PerCallTimeout,
		Language:          spec.Language,
	}
	evidence, err := p.deps.Research.Gather(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("research: %w", err)
	}
	if len(evidence) == 0 {
		// Bounded retry then fail clear, rather than generating ungrounded.
		return nil, fmt.Errorf("%w: the research adapter returned no usable sources for %q",
			domain.ErrInsufficientEvidence, spec.Topic)
	}
	return evidence, nil
}

// researchTerms plans the query. Query planning is the pipeline's, never the
// adapter's (FORGE.md 5), so it lives here where the source policy can see
// every candidate query.
func researchTerms(spec domain.GenerationSpec) string {
	terms := spec.Topic
	if d := strings.TrimSpace(spec.Description); d != "" {
		terms += " " + d
	}
	return terms
}

func addCost(a, b domain.Cost) domain.Cost {
	return domain.Cost{
		InputTokens:  a.InputTokens + b.InputTokens,
		OutputTokens: a.OutputTokens + b.OutputTokens,
		Attempts:     a.Attempts + b.Attempts,
	}
}
