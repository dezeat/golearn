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

// Package pipeline is Forge's bounded generation workflow (D-016, FORGE.md 4).
//
// It is an explicit, finite Go workflow — not an agent, not a framework, and
// not a dynamic loop (#80). Every stage is a named function called in a fixed
// order, and the pipeline owns the decisions an autonomous loop would take for
// itself: stop conditions, budgets, cancellation, source policy, and the
// acceptance gate.
//
// # Why the stage chain is not optional
//
// D-016 replaced the per-question human review gate with this chain. Its
// Consequences say that removing a stage reopens that decision rather than
// descoping it — so when time is short the parameter that gives is the
// question COUNT, never the stages. A three-question pack through the complete
// chain is a real result; a twenty-question pack through half of it is a
// liability, because nothing downstream can tell the difference by looking at
// the pack.
//
// The order, and what each stage is for:
//
//  1. validate the request — a spec that cannot succeed fails before spending
//     a single call;
//  2. research — evidence records from the Research port (FORGE.md 5);
//  3. generate — candidates grounded in that evidence;
//  4. validate deterministically — the same schema and domain rules a
//     hand-authored pack faces (D-004);
//  5. verify — answer keys re-checked in a FRESH CONTEXT on the same
//     provider and model;
//  6. critique — grounding and quality, against the evidence actually
//     supplied;
//  7. similarity — the near-duplicate gate (FORGE.md 7);
//  8. repair — exactly one targeted attempt per candidate, then re-verify;
//  9. fail clear — if the requested count cannot be met, say so. Never emit a
//     silently degraded pack.
//
// # Fresh context is structural, not a discipline
//
// ports.Provider has no session and no conversation handle, so a verifier call
// literally cannot see the generator's context. Step 5 is therefore a property
// of the port rather than something this package has to remember.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// SimilarityGate screens candidates for near-duplicates.
//
// Declared here rather than imported so the pipeline depends on the behavior
// it needs and not on a particular gate implementation — the Go convention of
// accepting an interface at the point of use. The similarity story owns the
// implementation; this is the shape the pipeline consumes.
type SimilarityGate interface {
	// Screen returns one verdict per candidate, in the same order.
	Screen(ctx context.Context, topicSlug string, candidates []domain.Candidate) ([]GateVerdict, error)
}

// GateVerdict is the similarity gate's judgement on one candidate.
type GateVerdict struct {
	// TooSimilar is true when the candidate duplicates existing library
	// content or another candidate in the same pack.
	TooSimilar bool
	// Reason is a short human-readable explanation, surfaced only in
	// diagnostics — similarity is a background gate, not a review chore.
	Reason string
}

// Budgets bound a run. Forge owns these, not the provider and not the model.
type Budgets struct {
	// MaxCandidateRounds caps how many times the pipeline will ask for more
	// candidates to replace rejected ones.
	MaxCandidateRounds int
	// RepairsPerCandidate is fixed at one by D-016 and is not configurable;
	// the field exists to make the bound visible at the call site rather than
	// buried in a loop.
	RepairsPerCandidate int
	// PerCallTimeout bounds a single provider call.
	PerCallTimeout time.Duration
	// EvidencePerRun caps how many evidence records are gathered.
	EvidencePerRun int
	// MaxBytesPerSource bounds extracted content per source.
	MaxBytesPerSource int
}

// DefaultBudgets are deliberately small.
//
// The reference host answers a single structured request in roughly a minute
// (FORGE-EXPERIMENTS B-2.1), and the chain makes several calls per question,
// so a generous round count would turn a demo into an unattended overnight
// job. Raising these is a deployment decision, not a default.
func DefaultBudgets() Budgets {
	return Budgets{
		MaxCandidateRounds:  3,
		RepairsPerCandidate: 1, // D-016: exactly one
		// Generous, because the bound that matters is the caller's context and
		// a tight default here would surface as "endpoint unreachable" on
		// hardware that was working correctly — the least diagnosable failure
		// available. The reference host answers a small structured request in
		// about a minute and a larger one in several (B-2.1).
		PerCallTimeout:    10 * time.Minute,
		EvidencePerRun:    6,
		MaxBytesPerSource: 4000,
	}
}

// Deps are the pipeline's collaborators. Research and Gate may be nil, and the
// pipeline degrades in a stated way rather than silently — see [New].
type Deps struct {
	Provider ports.Provider
	Research ports.Research
	Gate     SimilarityGate
	Runs     ports.RunStore
	Drafts   ports.DraftStore

	// Now is injected so a run's timestamps are deterministic under test.
	Now func() time.Time
	// ForgeVersion is recorded in provenance so a defect traced to a
	// generation vintage can be scoped.
	ForgeVersion string

	// AllowUngrounded permits a run with no Research adapter wired.
	//
	// It defaults to false and must be set deliberately, because FORGE.md 5
	// requires grounding in V1 and forbids silently degrading to ungrounded
	// generation. Without this, a missing adapter would produce a pack that
	// looks exactly like a grounded one — same schema, same provenance block,
	// an empty source list nobody reads — which is precisely the outcome
	// D-016 rules out. Setting it records the fact in the run diagnostic; it
	// does not make the pack grounded.
	AllowUngrounded bool
}

// Pipeline runs the bounded generation workflow.
type Pipeline struct {
	deps    Deps
	budgets Budgets
}

// New builds a pipeline.
//
// Research and Gate are permitted to be nil because their stories can land
// independently, but the pipeline records their absence in the run diagnostic
// rather than pretending the stage ran. A run that skipped grounding must not
// be indistinguishable from one that was grounded.
func New(deps Deps, budgets Budgets) (*Pipeline, error) {
	if deps.Provider == nil {
		return nil, errors.New("pipeline: a provider is required")
	}
	if deps.Runs == nil || deps.Drafts == nil {
		return nil, errors.New("pipeline: a run store and a draft store are required")
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if budgets.RepairsPerCandidate != 1 {
		// D-016 fixes this at exactly one. Accepting another value here would
		// let a caller quietly reopen a decision.
		return nil, fmt.Errorf("pipeline: repairs per candidate is fixed at 1 by D-016, got %d",
			budgets.RepairsPerCandidate)
	}
	if deps.Research == nil && !deps.AllowUngrounded {
		return nil, fmt.Errorf(
			"%w: no research adapter is wired, and FORGE.md 5 forbids degrading to ungrounded generation; set AllowUngrounded to accept an explicitly ungrounded run",
			domain.ErrInsufficientEvidence)
	}
	return &Pipeline{deps: deps, budgets: budgets}, nil
}

// Result reports what a completed run produced.
type Result struct {
	Draft    domain.Draft
	RunID    int64
	Accepted int
	Rejected int
	Repaired int
	Rounds   int
}

// ErrShortfall reports that the pipeline could not produce the requested
// number of valid questions within its budget.
//
// This is the fail-clear outcome D-016 requires, and it is deliberately an
// error rather than a partial success: a caller that treated a short pack as
// success would ship exactly the silently degraded pack the decision forbids.
var ErrShortfall = errors.New("could not produce the requested number of valid questions")

// Generate runs the full chain and returns a saved draft.
func (p *Pipeline) Generate(ctx context.Context, spec domain.GenerationSpec) (Result, error) {
	if err := ValidateSpec(spec); err != nil {
		// Fail before spending a call: a spec that cannot succeed should cost
		// nothing, and a run record for it would be noise.
		return Result{}, err
	}

	identity := p.deps.Provider.Identity()
	runID, err := p.deps.Runs.StartRun(ctx, domain.Run{
		Spec: spec, Model: identity, Verifier: identity,
		StartedAt: p.deps.Now(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("start run: %w", err)
	}

	result, sources, cost, err := p.run(ctx, spec, runID)
	if err != nil {
		p.finish(ctx, runID, statusFor(ctx, err), sources, cost, diagnose(err))
		return Result{}, err
	}

	if finishErr := p.deps.Runs.FinishRun(ctx, runID, domain.RunSucceeded, sources, cost,
		p.successDiagnostic(result)); finishErr != nil {
		return Result{}, fmt.Errorf("finish run: %w", finishErr)
	}

	// The draft is saved only after the run is marked succeeded, because the
	// store refuses a draft from a run that has not succeeded — the no-junk
	// rule enforced at the boundary rather than trusted here.
	draftID, err := p.deps.Drafts.SaveDraft(ctx, domain.Draft{
		RunID: runID, Pack: result.Draft.Pack, CreatedAt: p.deps.Now(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("save draft: %w", err)
	}
	result.Draft.ID = draftID
	result.RunID = runID
	return result, nil
}

// successDiagnostic summarizes a completed run.
//
// Any stage that did not run is named. A run diagnostic that reads the same
// whether or not the pack was grounded and screened is the one artifact that
// could let an unscreened pack pass for a screened one later, when nobody
// remembers how it was produced.
func (p *Pipeline) successDiagnostic(result Result) string {
	summary := fmt.Sprintf("%d accepted, %d rejected, %d repaired, %d round(s)",
		result.Accepted, result.Rejected, result.Repaired, result.Rounds)

	var skipped []string
	if p.deps.Research == nil {
		skipped = append(skipped, "grounding")
	}
	if p.deps.Gate == nil {
		skipped = append(skipped, "similarity")
	}
	if len(skipped) > 0 {
		summary += "; NOT RUN: " + strings.Join(skipped, ", ")
	}
	return summary
}

// finish records a terminal run state, swallowing its own error.
//
// The caller is already returning a failure; replacing it with a bookkeeping
// error would hide what actually went wrong, and there is no recovery from a
// store that cannot record a failure.
func (p *Pipeline) finish(ctx context.Context, runID int64, status domain.RunStatus,
	sources []domain.SourceRef, cost domain.Cost, diagnostic string) {
	// A canceled context cannot record anything, so the write uses a detached
	// one: losing the diagnostic for every canceled run would make cancels
	// the least diagnosable failure in the product.
	writeCtx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
	}
	_ = p.deps.Runs.FinishRun(writeCtx, runID, status, sources, cost, diagnostic)
}

// statusFor separates a cancellation from a failure. Recording a cancel as a
// failure would make a user's own Ctrl-C look like a defect in the pipeline.
func statusFor(ctx context.Context, err error) domain.RunStatus {
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return domain.RunCanceled
	}
	return domain.RunFailed
}

// diagnose renders one concise clause for the run record.
//
// FORGE.md 8 forbids persisting raw model output and request mechanics, so
// this is the error's own text, redacted, and nothing else.
func diagnose(err error) string {
	const maxDiagnostic = 300
	text := domain.Redact(err.Error())
	if len(text) > maxDiagnostic {
		text = text[:maxDiagnostic] + "…"
	}
	return text
}

// packFrom assembles the final pack from accepted candidates.
func (p *Pipeline) packFrom(spec domain.GenerationSpec, accepted []domain.Candidate,
	sources []domain.SourceRef, identity domain.ModelIdentity) coredomain.Pack {
	questions := make([]coredomain.PackQuestion, 0, len(accepted))
	for _, c := range accepted {
		q := c.Question
		confidence := coredomain.GeneratedConfidence
		q.Confidence = &confidence
		q.Source = "llm:" + identity.Provider
		if q.SourceRef == "" && len(c.Citations) > 0 {
			q.SourceRef = c.Citations[0]
		}
		questions = append(questions, q)
	}
	specCopy := spec
	return coredomain.Pack{
		PackVersion:    coredomain.PackVersionGenerated,
		Topic:          coredomain.PackTopic{Slug: TopicSlug(spec.Topic), Name: spec.Topic},
		GenerationSpec: &specCopy,
		Provenance: &coredomain.Provenance{
			GeneratedAt:  p.deps.Now(),
			Model:        identity,
			Verifier:     identity,
			Sources:      sources,
			ForgeVersion: p.deps.ForgeVersion,
		},
		Questions: questions,
	}
}
