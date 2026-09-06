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

package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/pipeline"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

var fixedNow = func() time.Time { return time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC) }

func testEvidence() []domain.Evidence {
	return []domain.Evidence{{
		ID: "s1", URL: "https://example.test/goroutines", Title: "Goroutines",
		RetrievedAt: fixedNow(),
		Content:     domain.Untrusted("A goroutine is started with the go keyword."),
	}}
}

// goodQuestion passes every gate when the scripted verifier and critic agree.
func goodQuestion(prompt string) map[string]any {
	return map[string]any{
		"prompt":          prompt,
		"choices":         []string{"go", "run", "spawn", "thread"},
		"correct_choices": []int{0},
		"explanation":     "The go keyword starts a goroutine.",
		"tags":            []string{"concurrency"},
		"citations":       []string{"s1"},
	}
}

func batch(questions ...map[string]any) map[string]any {
	return map[string]any{"questions": questions}
}

func passingVerify() map[string]any {
	return map[string]any{"correct_choice_ids": []string{"a"}, "reasoning": "the go keyword"}
}

func failingVerify() map[string]any {
	return map[string]any{"correct_choice_ids": []string{"b"}, "reasoning": "disagrees"}
}

func passingCritique() map[string]any {
	return map[string]any{
		"grounded": true, "distractors_plausible": true,
		"single_defensible_answer": true, "problem": "",
	}
}

func testSpec(count int) domain.GenerationSpec {
	return domain.GenerationSpec{
		Topic: "Go concurrency", Count: count,
		Difficulty: coredomain.DifficultyEasy, Language: "en",
	}
}

type harness struct {
	provider *fakeProvider
	research *fakeResearch
	gate     *fakeGate
	runs     *fakeRunStore
	drafts   *fakeDraftStore
	pipeline *pipeline.Pipeline
}

func newHarness(t *testing.T, provider *fakeProvider) *harness {
	t.Helper()
	h := &harness{
		provider: provider,
		research: &fakeResearch{evidence: testEvidence()},
		gate:     &fakeGate{},
		runs:     &fakeRunStore{},
		drafts:   &fakeDraftStore{},
	}
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Research: h.research, Gate: h.gate,
		Runs: h.runs, Drafts: h.drafts, Now: fixedNow, ForgeVersion: "test",
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
	h.pipeline = p
	return h
}

func TestACompleteRunProducesADraftWithProvenance(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Which keyword starts a goroutine?"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	result, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Accepted != 1 {
		t.Errorf("accepted = %d, want 1", result.Accepted)
	}
	if len(h.drafts.saved) != 1 {
		t.Fatalf("want exactly one saved draft, got %d", len(h.drafts.saved))
	}

	pack := h.drafts.saved[0].Pack
	if pack.PackVersion != coredomain.PackVersionGenerated {
		t.Errorf("pack version = %q", pack.PackVersion)
	}
	if pack.Provenance == nil || pack.Provenance.Model.Model != "test-model" {
		t.Errorf("provenance did not record the model: %+v", pack.Provenance)
	}
	if pack.GenerationSpec == nil || pack.GenerationSpec.Topic != "Go concurrency" {
		t.Errorf("generation spec was not carried: %+v", pack.GenerationSpec)
	}
	if len(pack.Provenance.Sources) != 1 || pack.Provenance.Sources[0].ID != "s1" {
		t.Errorf("source refs were not carried: %+v", pack.Provenance.Sources)
	}
	q := pack.Questions[0]
	if q.Confidence == nil || *q.Confidence >= coredomain.ManualConfidence {
		t.Errorf("a generated question must carry confidence below the manual default, got %v", q.Confidence)
	}
	if !strings.HasPrefix(q.Source, "llm:") {
		t.Errorf("source = %q, want an llm: prefix", q.Source)
	}
	if h.runs.finishedAs[0] != domain.RunSucceeded {
		t.Errorf("run status = %q", h.runs.finishedAs[0])
	}
}

// D-016's fail-clear rule, and the single most important behavior here: a
// pipeline that cannot meet the requested count must say so, not hand back a
// smaller pack that looks complete. Nothing downstream can tell the difference
// by looking at the pack.
func TestAShortfallFailsClearlyRatherThanEmittingASmallerPack(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"), goodQuestion("Q2"))).
		reply(stageVerify, failingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, err := h.pipeline.Generate(context.Background(), testSpec(3))
	if !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("want ErrShortfall, got %v", err)
	}
	if !strings.Contains(err.Error(), "of 3") {
		t.Errorf("the error must name the shortfall, got: %v", err)
	}
	if len(h.drafts.saved) != 0 {
		t.Errorf("a failed run must save no draft, got %d", len(h.drafts.saved))
	}
	if h.runs.finishedAs[0] != domain.RunFailed {
		t.Errorf("run status = %q, want failed", h.runs.finishedAs[0])
	}
}

// D-016 permits "exactly one targeted repair and re-verification". The bound
// is per candidate and spans the whole chain — one per gate would quietly turn
// one repair into four.
//
// The assertion is repairs <= candidates assessed. An earlier version compared
// repairs to *generation rounds*, which is only the bound when exactly one
// candidate is produced per round — true of a one-question fixture and of
// nothing else. An independent review probe ran it with two candidates per
// round and it failed against unmodified code, reporting six repairs for six
// candidates: exactly the invariant it was named for. The assertion was a
// coincidence of the fixture, and it would have passed equally under the
// weaker rule "one repair per round".
func TestAtMostOneRepairIsSpentPerCandidate(t *testing.T) {
	for _, count := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			questions := make([]map[string]any, 0, count)
			for i := 0; i < count; i++ {
				questions = append(questions, goodQuestion(fmt.Sprintf("Q%d", i)))
			}
			provider := newFakeProvider().
				reply(stageGenerate, batch(questions...)).
				reply(stageVerify, failingVerify()).
				reply(stageCritique, passingCritique()).
				reply(stageRepair, batch(goodQuestion("repaired")))
			h := newHarness(t, provider)

			_, _ = h.pipeline.Generate(context.Background(), testSpec(count))

			repairs := provider.countFor(stageRepair)
			// Every generated candidate is assessed, so the number assessed is
			// the number the generator produced across all rounds.
			assessed := 0
			for _, c := range provider.callsFor(stageGenerate) {
				_ = c
				assessed += count
			}
			if repairs > assessed {
				t.Errorf("%d repairs for %d candidates assessed: the per-candidate bound was exceeded",
					repairs, assessed)
			}
			if repairs == 0 {
				t.Error("no repair was attempted, so the bound is not being exercised")
			}
		})
	}
}

func TestARepairedCandidateIsAcceptedAfterReVerification(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, failingVerify(), passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch(goodQuestion("Q1 repaired")))
	h := newHarness(t, provider)

	result, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Repaired != 1 {
		t.Errorf("repaired = %d, want 1", result.Repaired)
	}
	if provider.countFor(stageVerify) < 2 {
		t.Error("a repair must be followed by re-verification")
	}
}

// The verifier must re-derive the answer, not review a proposed one. A
// verifier shown the key tends to agree with it, which would make the stage a
// confirmation rather than a check.
func TestTheVerifierIsNeverShownTheProposedAnswerKey(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Which keyword starts a goroutine?"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	calls := provider.callsFor(stageVerify)
	if len(calls) == 0 {
		t.Fatal("the verification stage never ran")
	}
	sent := calls[0].Request.User
	if strings.Contains(sent, "*") {
		t.Errorf("the verifier was shown the answer-key marker:\n%s", sent)
	}
	if strings.Contains(sent, "The go keyword starts a goroutine") {
		t.Errorf("the verifier was shown the explanation:\n%s", sent)
	}
}

// Fresh context is structural: the port has no session handle. Asserting it on
// the request is what proves the property rather than assuming it.
func TestVerificationCarriesNoneOfTheGeneratorsContext(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	generate := provider.callsFor(stageGenerate)[0]
	verify := provider.callsFor(stageVerify)[0]
	if verify.Request.System == generate.Request.System {
		t.Error("the verifier reused the generator's system prompt")
	}
	if len(verify.Request.Evidence) != 0 {
		t.Error("the verifier must answer from the question alone, not the generator's evidence")
	}
	if strings.Contains(verify.Request.User, generate.Request.User) {
		t.Error("the generator's instruction leaked into the verification call")
	}
}

// A candidate failing deterministic validation must never reach a provider.
// Paying to verify something already known invalid is waste the budget cannot
// afford at roughly a minute per call.
func TestAStructurallyInvalidCandidateNeverReachesTheProvider(t *testing.T) {
	invalid := goodQuestion("Q1")
	// An answer position outside the choice list. It is dropped rather than
	// clamped, leaving no correct answer, which deterministic validation then
	// rejects — clamping would silently mark a different choice correct.
	invalid["correct_choices"] = []int{99}

	provider := newFakeProvider().
		reply(stageGenerate, batch(invalid)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, _ = h.pipeline.Generate(context.Background(), testSpec(1))

	if provider.countFor(stageVerify) != 0 {
		t.Errorf("verification ran %d time(s) on a candidate that failed schema validation",
			provider.countFor(stageVerify))
	}
	if provider.countFor(stageCritique) != 0 {
		t.Errorf("critique ran %d time(s) on a candidate that failed schema validation",
			provider.countFor(stageCritique))
	}
}

// A citation naming evidence that was never supplied is the clearest signal a
// question was invented rather than grounded, and it costs nothing to detect.
func TestACandidateCitingUnsuppliedEvidenceIsRejectedWithoutAModelCall(t *testing.T) {
	fabricated := goodQuestion("Q1")
	fabricated["citations"] = []string{"s99"}

	provider := newFakeProvider().
		reply(stageGenerate, batch(fabricated)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("want a shortfall, got %v", err)
	}
	if provider.countFor(stageVerify) != 0 {
		t.Error("a fabricated citation must be caught before spending a verification call")
	}
}

func TestAnUncitedCandidateIsRejected(t *testing.T) {
	uncited := goodQuestion("Q1")
	uncited["citations"] = []string{}

	provider := newFakeProvider().
		reply(stageGenerate, batch(uncited)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("want a shortfall, got %v", err)
	}
}

// FORGE.md 5 forbids silently degrading to ungrounded generation.
func TestNoEvidenceFailsClearlyRatherThanGeneratingUngrounded(t *testing.T) {
	provider := newFakeProvider().reply(stageGenerate, batch(goodQuestion("Q1")))
	h := newHarness(t, provider)
	h.research.evidence = nil

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, domain.ErrInsufficientEvidence) {
		t.Fatalf("want ErrInsufficientEvidence, got %v", err)
	}
	if provider.countFor(stageGenerate) != 0 {
		t.Error("generation ran despite there being no evidence to ground it in")
	}
	if len(h.drafts.saved) != 0 {
		t.Error("a run without grounding must save no draft")
	}
}

func TestATooSimilarCandidateIsRejectedByTheGate(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)
	h.gate.tooSimilar = []bool{true}

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("want a shortfall, got %v", err)
	}
	if h.gate.calls == 0 {
		t.Error("the similarity gate never ran")
	}
}

// A cancel is the user's own signal. Recording it as a failure would make
// Ctrl-C look like a defect, and the run history is the only place a user can
// tell the two apart afterwards.
func TestCancellationRecordsACanceledRunNotAFailedOne(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.pipeline.Generate(ctx, testSpec(1))
	if err == nil {
		t.Fatal("want a cancellation error")
	}
	if len(h.runs.finishedAs) == 0 {
		t.Fatal("a canceled run must still be recorded")
	}
	if h.runs.finishedAs[0] != domain.RunCanceled {
		t.Errorf("run status = %q, want %q", h.runs.finishedAs[0], domain.RunCanceled)
	}
	if len(h.drafts.saved) != 0 {
		t.Error("cancellation must leave no draft")
	}
}

// D-016 fixes the repair budget at exactly one. Accepting another value would
// let a caller quietly reopen an accepted decision through configuration.
func TestARepairBudgetOtherThanOneIsRefused(t *testing.T) {
	budgets := pipeline.DefaultBudgets()
	budgets.RepairsPerCandidate = 3

	_, err := pipeline.New(pipeline.Deps{
		Provider: newFakeProvider(), Research: &fakeResearch{}, Gate: &fakeGate{},
		Runs: &fakeRunStore{}, Drafts: &fakeDraftStore{},
	}, budgets)
	if err == nil {
		t.Fatal("a repair budget other than 1 must be refused")
	}
	if !strings.Contains(err.Error(), "D-016") {
		t.Errorf("the refusal should cite the decision it protects, got: %v", err)
	}
}

func TestAnInvalidSpecIsRefusedBeforeAnyCallOrRunRecord(t *testing.T) {
	cases := map[string]domain.GenerationSpec{
		"no topic":          {Topic: "  ", Count: 1},
		"zero count":        {Topic: "Go", Count: 0},
		"count above limit": {Topic: "Go", Count: pipeline.MaxCount + 1},
		"bad difficulty":    {Topic: "Go", Count: 1, Difficulty: coredomain.Difficulty("trivial")},
		"unsluggable topic": {Topic: "!!!", Count: 1},
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			provider := newFakeProvider()
			h := newHarness(t, provider)

			if _, err := h.pipeline.Generate(context.Background(), spec); err == nil {
				t.Fatal("want the spec refused")
			}
			if len(provider.calls) != 0 {
				t.Errorf("an invalid spec spent %d provider call(s)", len(provider.calls))
			}
			if len(h.runs.started) != 0 {
				t.Error("an invalid spec must not create a run record")
			}
		})
	}
}

func TestAnUnknownStyleIsAccepted(t *testing.T) {
	spec := testSpec(1)
	spec.Style = coredomain.Style("a-style-nobody-has-defined-yet")
	if err := pipeline.ValidateSpec(spec); err != nil {
		t.Errorf("an unknown style must be accepted: %v", err)
	}
}

// The slug feeds the topic upsert and the D-007 content hash, so drift would
// grow a duplicate topic per run and break dedup across them.
func TestTopicSlugIsStableAndKebabCase(t *testing.T) {
	cases := map[string]string{
		"Go concurrency":       "go-concurrency",
		"  Go   Concurrency  ": "go-concurrency",
		"Go/Concurrency!":      "go-concurrency",
		"C++ basics":           "c-basics",
	}
	for topic, want := range cases {
		if got := pipeline.TopicSlug(topic); got != want {
			t.Errorf("TopicSlug(%q) = %q, want %q", topic, got, want)
		}
	}
}

// A provider failure is not a shortfall. Reporting one as the other would send
// a user to rewrite their topic when the endpoint was down.
func TestAProviderFailureIsReportedAsItselfNotAsAShortfall(t *testing.T) {
	provider := newFakeProvider().fail(stageGenerate, domain.ErrProviderUnreachable)
	h := newHarness(t, provider)

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if errors.Is(err, pipeline.ErrShortfall) {
		t.Error("a provider outage must not be reported as a shortfall")
	}
	if !errors.Is(err, domain.ErrProviderUnreachable) {
		t.Errorf("want ErrProviderUnreachable, got %v", err)
	}
}

// FORGE.md 8 categorically forbids persisting raw model or tool output, and a
// provider's error body is exactly that. Shape-based redaction is not a
// sufficient answer: it catches things shaped like credentials, and a proxy
// echoing an arbitrary token is not shaped like anything.
//
// So the diagnostic classifies rather than quotes.
func TestRunDiagnosticsClassifyRatherThanQuoteProviderOutput(t *testing.T) {
	const leaked = "ordinary-secret-not-shaped-like-a-key"
	provider := newFakeProvider().
		fail(stageGenerate, fmt.Errorf("%w: provider said: %s",
			domain.ErrProviderUnreachable, leaked))
	h := newHarness(t, provider)

	_, _ = h.pipeline.Generate(context.Background(), testSpec(1))

	if len(h.runs.diagnostic) == 0 {
		t.Fatal("no diagnostic recorded")
	}
	recorded := h.runs.diagnostic[0]
	if strings.Contains(recorded, leaked) {
		t.Errorf("the run diagnostic quoted provider output: %s", recorded)
	}
	if !strings.Contains(recorded, "unreachable") {
		t.Errorf("the diagnostic must still classify the failure, got: %s", recorded)
	}
}

// A shortfall diagnostic is built by the pipeline from its own counters, so it
// quotes nothing and must keep its numbers — they are the whole diagnostic
// value of a failed run.
func TestAShortfallDiagnosticKeepsItsCounts(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, failingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, _ = h.pipeline.Generate(context.Background(), testSpec(2))

	recorded := h.runs.diagnostic[0]
	if !strings.Contains(recorded, "of 2") {
		t.Errorf("the shortfall diagnostic lost its counts: %s", recorded)
	}
}

// FORGE.md 5 requires grounding in V1 and forbids silently degrading to
// ungrounded generation. Without a research adapter, a pack would look exactly
// like a grounded one — same schema, same provenance block, an empty source
// list nobody reads — so running ungrounded has to be a deliberate act.
func TestAnUngroundedRunMustBeOptedIntoExplicitly(t *testing.T) {
	deps := pipeline.Deps{
		Provider: newFakeProvider(), Runs: &fakeRunStore{}, Drafts: &fakeDraftStore{},
		Now: fixedNow,
	}

	_, err := pipeline.New(deps, pipeline.DefaultBudgets())
	if err == nil {
		t.Fatal("a pipeline with no research adapter must refuse to be built")
	}
	if !errors.Is(err, domain.ErrInsufficientEvidence) {
		t.Errorf("want ErrInsufficientEvidence, got %v", err)
	}

	deps.AllowUngrounded = true
	if _, err := pipeline.New(deps, pipeline.DefaultBudgets()); err != nil {
		t.Errorf("an explicitly ungrounded pipeline must be permitted: %v", err)
	}
}

// Opting in makes the run ungrounded, not grounded. The run record has to say
// so, because it is the one artifact that could later let an unscreened,
// ungrounded pack pass for a checked one when nobody remembers how it was
// produced.
func TestASkippedStageIsNamedInTheRunDiagnostic(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	runs := &fakeRunStore{}

	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Runs: runs, Drafts: &fakeDraftStore{},
		Now: fixedNow, ForgeVersion: "test", AllowUngrounded: true,
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}

	if _, err := p.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(runs.diagnostic) == 0 {
		t.Fatal("no diagnostic recorded")
	}
	diagnostic := runs.diagnostic[0]
	for _, stage := range []string{"grounding", "similarity"} {
		if !strings.Contains(diagnostic, stage) {
			t.Errorf("the diagnostic must name the skipped %s stage, got: %s", stage, diagnostic)
		}
	}
	if !strings.Contains(diagnostic, "NOT RUN") {
		t.Errorf("the diagnostic must be unmistakable about what did not run, got: %s", diagnostic)
	}
}

// A fully wired run must not carry the warning, or the signal would mean
// nothing.
func TestAFullyWiredRunNamesNoSkippedStages(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(h.runs.diagnostic[0], "NOT RUN") {
		t.Errorf("a fully wired run must not report skipped stages: %s", h.runs.diagnostic[0])
	}
}

// Choice ids feed the D-007 content hash, so the same question generated twice
// must produce the same ids or dedup would stop recognizing it.
func TestChoiceIdsAreAssignedDeterministicallyByPosition(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	q := h.drafts.saved[0].Pack.Questions[0]

	want := []string{"a", "b", "c", "d"}
	for i, c := range q.Choices {
		if c.ID != want[i] {
			t.Errorf("choice %d has id %q, want %q", i, c.ID, want[i])
		}
	}
	if len(q.CorrectChoiceIDs) != 1 || q.CorrectChoiceIDs[0] != "a" {
		t.Errorf("answer position 0 should resolve to choice id %q, got %v", "a", q.CorrectChoiceIDs)
	}
}

// An answer position outside the choice list must be dropped, never clamped.
// Clamping would silently mark a different choice correct, which is the single
// worst defect a practice tool can ship — and it would look like a model
// quality problem rather than a mapping bug.
func TestAnOutOfRangeAnswerPositionIsDroppedNotClamped(t *testing.T) {
	broken := goodQuestion("Q1")
	broken["correct_choices"] = []int{99}

	provider := newFakeProvider().
		reply(stageGenerate, batch(broken)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("want the candidate rejected, got %v", err)
	}
	if len(h.drafts.saved) != 0 {
		t.Error("a question with no resolvable answer must never reach a draft")
	}
	if provider.countFor(stageVerify) != 0 {
		t.Error("an unresolvable answer must be caught by deterministic validation, before a model call")
	}
}

// A duplicated answer position must not produce a duplicated answer id, which
// would turn a single-select question into a bogus multi-select.
func TestARepeatedAnswerPositionIsCountedOnce(t *testing.T) {
	repeated := goodQuestion("Q1")
	repeated["correct_choices"] = []int{0, 0}

	provider := newFakeProvider().
		reply(stageGenerate, batch(repeated)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	q := h.drafts.saved[0].Pack.Questions[0]
	if len(q.CorrectChoiceIDs) != 1 {
		t.Errorf("want one answer id, got %v", q.CorrectChoiceIDs)
	}
	if q.Type != coredomain.SingleSelect {
		t.Errorf("a repeated position must not promote the question to %q", q.Type)
	}
}

// The per-call budget must actually reach the provider call. A budget that is
// configured but never applied is invisible until a run hangs — and then it
// looks like a provider problem, because the pipeline is the one component
// nobody suspects of not enforcing its own limit.
func TestThePerCallBudgetIsAppliedToEveryProviderCall(t *testing.T) {
	provider := newFakeProvider()
	// Report back the deadline each call was given, so the test asserts on
	// what the provider actually received rather than on what was configured.
	provider.observeDeadline = true
	provider.reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())

	budgets := pipeline.DefaultBudgets()
	budgets.PerCallTimeout = 90 * time.Second

	h := &harness{
		provider: provider,
		research: &fakeResearch{evidence: testEvidence()},
		gate:     &fakeGate{},
		runs:     &fakeRunStore{},
		drafts:   &fakeDraftStore{},
	}
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Research: h.research, Gate: h.gate,
		Runs: h.runs, Drafts: h.drafts, Now: fixedNow, ForgeVersion: "test",
	}, budgets)
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
	if _, err := p.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(provider.deadlines) == 0 {
		t.Fatal("no provider call was made")
	}
	for i, remaining := range provider.deadlines {
		if remaining <= 0 {
			t.Errorf("call %d received no deadline at all", i)
			continue
		}
		// Generous bounds: the assertion is "the configured budget reached the
		// call", not a timing measurement. A default of ten minutes would fail
		// this; an unset deadline fails above.
		if remaining > budgets.PerCallTimeout {
			t.Errorf("call %d received %v, more than the configured budget %v",
				i, remaining, budgets.PerCallTimeout)
		}
		if remaining < budgets.PerCallTimeout/2 {
			t.Errorf("call %d received only %v of the configured %v",
				i, remaining, budgets.PerCallTimeout)
		}
	}
}

// The live lane found this and no unit test could have, because every unit
// test supplied evidence: with research unwired the critic was asked whether
// the question was supported by evidence, given none, correctly answered no,
// and rejected every candidate. The pipeline could not succeed, and it
// presented as a model quality problem.
//
// With nothing to judge against, the grounding question is not asked and its
// answer is not read.
func TestAnUngroundedRunDoesNotAskTheCriticAboutGrounding(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		// A critic that reports grounded=false, exactly as one would when
		// handed no evidence. It must not cause a rejection here.
		reply(stageCritique, map[string]any{
			"grounded": false, "distractors_plausible": true,
			"single_defensible_answer": true, "problem": "",
		})

	runs := &fakeRunStore{}
	drafts := &fakeDraftStore{}
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Runs: runs, Drafts: drafts,
		Now: fixedNow, ForgeVersion: "test", AllowUngrounded: true,
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}

	if _, err := p.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("an ungrounded run must not be blocked by the grounding verdict: %v", err)
	}
	if len(drafts.saved) != 1 {
		t.Fatalf("want one draft, got %d", len(drafts.saved))
	}

	critiques := provider.callsFor(stageCritique)
	if len(critiques) == 0 {
		t.Fatal("the critique stage did not run")
	}
	if strings.Contains(critiques[0].Request.System, "grounded:") {
		t.Errorf("the critic was asked about grounding with no evidence to judge against:\n%s",
			critiques[0].Request.System)
	}
	if len(critiques[0].Request.Evidence) != 0 {
		t.Error("no evidence should have been sent")
	}
}

// The complementary case: when evidence IS supplied, an ungrounded verdict
// must still reject. Dropping the question entirely would remove the gate.
func TestAGroundedRunStillRejectsAnUngroundedQuestion(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, map[string]any{
			"grounded": false, "distractors_plausible": true,
			"single_defensible_answer": true, "problem": "invented a fact",
		}).
		reply(stageRepair, batch())
	h := newHarness(t, provider)

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, pipeline.ErrShortfall) {
		t.Fatalf("an ungrounded question must be rejected when evidence was supplied, got %v", err)
	}
	critiques := provider.callsFor(stageCritique)
	if !strings.Contains(critiques[0].Request.System, "grounded:") {
		t.Error("the critic must be asked about grounding when evidence exists")
	}
}

// The candidate schema requires a citations field, so an ungrounded run still
// gets one — populated with whatever the model invents. Copying that into a
// question's source_ref would make an ungrounded question look grounded at
// exactly the level a reader inspects, which is worse than carrying no
// reference at all.
func TestAnUngroundedPackCarriesNoFabricatedSourceReferences(t *testing.T) {
	fabricated := goodQuestion("Q1")
	fabricated["citations"] = []string{"s1", "https://example.test/invented"}

	provider := newFakeProvider().
		reply(stageGenerate, batch(fabricated)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, map[string]any{
			"distractors_plausible": true, "single_defensible_answer": true, "problem": "",
		})

	drafts := &fakeDraftStore{}
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Runs: &fakeRunStore{}, Drafts: drafts,
		Now: fixedNow, ForgeVersion: "test", AllowUngrounded: true,
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
	if _, err := p.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	pack := drafts.saved[0].Pack
	if got := pack.Questions[0].SourceRef; got != "" {
		t.Errorf("an ungrounded question carried a source reference %q", got)
	}
	if len(pack.Provenance.Sources) != 0 {
		t.Errorf("an ungrounded pack must carry no sources, got %v", pack.Provenance.Sources)
	}
}

// A grounded run must still record the reference, or the fix would have
// removed the attribution rather than the fabrication.
func TestAGroundedPackCarriesTheCitationItActuallyUsed(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	pack := h.drafts.saved[0].Pack
	if got := pack.Questions[0].SourceRef; got != "s1" {
		t.Errorf("source_ref = %q, want the evidence id the question cited", got)
	}
}

// The run is marked succeeded before the draft is saved, because the store
// refuses a draft from a run that has not succeeded. That ordering leaves a
// window: a failed save would leave a run row saying "succeeded" with no draft
// attached, so the run history reports a success the user cannot act on and
// nothing reconciles it.
func TestAFailedDraftSaveCorrectsTheRunRatherThanLeavingItSucceeded(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())

	runs := &fakeRunStore{}
	drafts := &fakeDraftStore{err: errors.New("database is locked")}
	h := newHarness(t, provider)
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Research: h.research, Gate: h.gate,
		Runs: runs, Drafts: drafts, Now: fixedNow, ForgeVersion: "test",
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}

	if _, err := p.Generate(context.Background(), testSpec(1)); err == nil {
		t.Fatal("a failed draft save must be reported")
	}

	if len(runs.finishedAs) == 0 {
		t.Fatal("no terminal state recorded")
	}
	final := runs.finishedAs[len(runs.finishedAs)-1]
	if final == domain.RunSucceeded {
		t.Error("the run still reports succeeded with no draft attached")
	}
	if final != domain.RunFailed {
		t.Errorf("final status = %q, want %q", final, domain.RunFailed)
	}
	if !strings.Contains(runs.diagnostic[len(runs.diagnostic)-1], "could not be saved") {
		t.Errorf("the diagnostic should say what happened, got: %s",
			runs.diagnostic[len(runs.diagnostic)-1])
	}
}

// D-008 makes presentation a render-time concern: labels are computed from
// display order and the stored text is content. A model asked for choices
// prints them the way it has seen them printed, and the repair stage makes it
// worse — the question is rendered to the model WITH a correctness marker and
// the model copies that back.
//
// The first live run produced "* b. To enable...", which then rendered as
// "* B) * b. To enable...". The label was applied twice and the answer was
// marked twice, in stored data.
func TestModelSuppliedChoiceLabelsAreStrippedFromStoredText(t *testing.T) {
	labeled := goodQuestion("Q1")
	labeled["choices"] = []string{
		"a. go",
		"B) run",
		"* c. spawn",
		"4. thread",
	}

	provider := newFakeProvider().
		reply(stageGenerate, batch(labeled)).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := []string{"go", "run", "spawn", "thread"}
	for i, c := range h.drafts.saved[0].Pack.Questions[0].Choices {
		if c.Text != want[i] {
			t.Errorf("choice %d text = %q, want %q — presentation was stored as content", i, c.Text, want[i])
		}
	}
}

// Stripping must be conservative. Removing real content would be far worse
// than leaving a stray prefix, because the question would silently change
// meaning rather than merely look untidy.
func TestChoiceTextThatMerelyResemblesALabelIsKept(t *testing.T) {
	cases := map[string]string{
		"a. go":              "go",
		"go":                 "go",
		"Go. The keyword":    "Go. The keyword",
		"3.14 approximately": "3.14 approximately",
		"x := 5":             "x := 5",
		// The marker goes; the label does not, because "b" is not the label
		// position 0 would carry. A mismatched label is either a model error
		// or real content, and stripping it would be a guess.
		"* b. channels":          "b. channels",
		"- buffered channels":    "buffered channels",
		"i.e. a lightweight one": "i.e. a lightweight one",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			labeled := goodQuestion("Q1")
			labeled["choices"] = []string{in, "other one", "other two", "other three"}

			provider := newFakeProvider().
				reply(stageGenerate, batch(labeled)).
				reply(stageVerify, passingVerify()).
				reply(stageCritique, passingCritique())
			h := newHarness(t, provider)

			if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got := h.drafts.saved[0].Pack.Questions[0].Choices[0].Text; got != want {
				t.Errorf("%q became %q, want %q", in, got, want)
			}
		})
	}
}

// FORGE.md 5 requires a bounded retry before failing clear, and one attempt is
// not a retry. An earlier version claimed "bounded retry then fail clear" in a
// comment and returned on the first empty response — the comment described the
// specification and the code did not implement it, with nothing testing the
// difference.
func TestATransientlyEmptyResearchResultIsRetried(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, passingVerify()).
		reply(stageCritique, passingCritique())
	h := newHarness(t, provider)
	// Empty first, usable second.
	h.research.perAttempt = [][]domain.Evidence{nil, testEvidence()}

	if _, err := h.pipeline.Generate(context.Background(), testSpec(1)); err != nil {
		t.Fatalf("a transient empty result must be retried, not fatal: %v", err)
	}
	if len(h.research.queries) < 2 {
		t.Errorf("research was attempted %d time(s); the retry did not happen", len(h.research.queries))
	}
}

// The retry is bounded: persistent emptiness still fails clear rather than
// looping.
func TestPersistentlyEmptyResearchFailsClearWithinTheBound(t *testing.T) {
	provider := newFakeProvider()
	h := newHarness(t, provider)
	h.research.evidence = nil

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, domain.ErrInsufficientEvidence) {
		t.Fatalf("want ErrInsufficientEvidence, got %v", err)
	}
	if len(h.research.queries) > 3 {
		t.Errorf("research was attempted %d times; the retry is not bounded", len(h.research.queries))
	}
	if len(provider.calls) != 0 {
		t.Error("generation ran despite there being no evidence")
	}
}

// A record with no extractable content is not evidence, however well-formed.
// Counting it would let a candidate cite a source that says nothing.
func TestEvidenceWithNoContentDoesNotCountAsGrounding(t *testing.T) {
	provider := newFakeProvider()
	h := newHarness(t, provider)
	h.research.evidence = []domain.Evidence{{
		ID: "s1", URL: "https://example.test/empty", Title: "Empty",
		Content: domain.Untrusted("   "),
	}}

	_, err := h.pipeline.Generate(context.Background(), testSpec(1))
	if !errors.Is(err, domain.ErrInsufficientEvidence) {
		t.Fatalf("empty content must not count as grounding, got %v", err)
	}
}
