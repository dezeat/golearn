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
		"prompt": prompt,
		"choices": []map[string]string{
			{"id": "a", "text": "go"},
			{"id": "b", "text": "run"},
			{"id": "c", "text": "spawn"},
			{"id": "d", "text": "thread"},
		},
		"correct_choice_ids": []string{"a"},
		"explanation":        "The go keyword starts a goroutine.",
		"tags":               []string{"concurrency"},
		"citations":          []string{"s1"},
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
func TestExactlyOneRepairIsSpentPerCandidate(t *testing.T) {
	provider := newFakeProvider().
		reply(stageGenerate, batch(goodQuestion("Q1"))).
		reply(stageVerify, failingVerify()).
		reply(stageCritique, passingCritique()).
		reply(stageRepair, batch(goodQuestion("Q1 repaired")))
	h := newHarness(t, provider)

	_, _ = h.pipeline.Generate(context.Background(), testSpec(1))

	repairs := provider.countFor(stageRepair)
	generations := provider.countFor(stageGenerate)
	if repairs != generations {
		t.Errorf("want exactly one repair per generated candidate: %d repairs across %d generation rounds",
			repairs, generations)
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
	invalid["correct_choice_ids"] = []string{"does-not-exist"}

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

// Run diagnostics are user-facing and must never carry a credential.
func TestRunDiagnosticsAreRedacted(t *testing.T) {
	provider := newFakeProvider().
		fail(stageGenerate, errors.New("upstream said: api_key=sk-aaaaaaaaaaaaaaaa"))
	h := newHarness(t, provider)

	_, _ = h.pipeline.Generate(context.Background(), testSpec(1))

	if len(h.runs.diagnostic) == 0 {
		t.Fatal("no diagnostic recorded")
	}
	if strings.Contains(h.runs.diagnostic[0], "sk-aaaaaaaaaaaaaaaa") {
		t.Errorf("the run diagnostic leaked a credential: %s", h.runs.diagnostic[0])
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
