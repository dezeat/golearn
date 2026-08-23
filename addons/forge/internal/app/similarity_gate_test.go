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

package app_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// No network call happens anywhere in this file: every embedding comes from
// the scripted embedder below, which is arithmetic. That is what keeps
// `make check` offline while still exercising the gate's decisions.

func gateModel() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: "fixture", Model: "scripted-angles"}
}

// scriptedEmbedder places each text on the unit circle at a chosen angle, so
// the cosine between any two texts is exactly cos(a-b) and a test can dial a
// score instead of hoping for one. Texts with no scripted angle are spread far
// apart, which makes "unrelated" the default rather than something to arrange.
type scriptedEmbedder struct {
	angles map[string]float64
	calls  [][]string
	fail   error
	short  bool
}

func (e *scriptedEmbedder) EmbeddingIdentity() domain.ModelIdentity { return gateModel() }

func (e *scriptedEmbedder) Embed(_ context.Context, texts []string) ([]domain.Vector, error) {
	e.calls = append(e.calls, append([]string(nil), texts...))
	if e.fail != nil {
		return nil, e.fail
	}
	out := make([]domain.Vector, 0, len(texts))
	for _, t := range texts {
		angle, ok := e.angles[t]
		if !ok {
			// Deterministic, and far from every scripted angle.
			angle = math.Pi/2 + float64(len(t)%7)*0.01
		}
		out = append(out, domain.Vector{float32(math.Cos(angle)), float32(math.Sin(angle))})
	}
	if e.short && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out, nil
}

// angleFor returns the angle whose cosine against angle 0 is the wanted score.
func angleFor(score float64) float64 { return math.Acos(score) }

// memoryIndex is the SimilarityIndex a test can inspect. It mirrors the real
// adapter's contract rather than its storage.
type memoryIndex struct {
	vectors map[domain.ModelIdentity]map[domain.LibraryQuestionID]domain.Vector
	puts    int
}

func newMemoryIndex() *memoryIndex {
	return &memoryIndex{vectors: map[domain.ModelIdentity]map[domain.LibraryQuestionID]domain.Vector{}}
}

func (m *memoryIndex) Put(_ context.Context, id domain.LibraryQuestionID, model domain.ModelIdentity, v domain.Vector) error {
	if m.vectors[model] == nil {
		m.vectors[model] = map[domain.LibraryQuestionID]domain.Vector{}
	}
	m.vectors[model][id] = v
	m.puts++
	return nil
}

func (m *memoryIndex) Missing(_ context.Context, model domain.ModelIdentity, ids []domain.LibraryQuestionID) ([]domain.LibraryQuestionID, error) {
	var missing []domain.LibraryQuestionID
	for _, id := range ids {
		if _, ok := m.vectors[model][id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

func (m *memoryIndex) Nearest(_ context.Context, model domain.ModelIdentity, probe domain.Vector, limit int) ([]ports.Neighbor, error) {
	if limit < 1 {
		return nil, fmt.Errorf("memoryIndex: limit %d", limit)
	}
	var out []ports.Neighbor
	for id, v := range m.vectors[model] {
		score, err := domain.Cosine(probe, v)
		if err != nil {
			return nil, err
		}
		out = append(out, ports.Neighbor{QuestionID: id, Score: score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].QuestionID < out[j].QuestionID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memoryIndex) Count(_ context.Context, model domain.ModelIdentity) (int, error) {
	return len(m.vectors[model]), nil
}

// stubLibrary is read-only, exactly like the port. It records what it handed
// out so a test can prove the gate gave it back unchanged.
type stubLibrary struct {
	questions []coredomain.Question
	reads     int
}

func (l *stubLibrary) QuestionsByTopic(_ context.Context, _ int64) ([]coredomain.Question, error) {
	l.reads++
	return l.questions, nil
}

// scriptedReviser hands back whatever the test queued, so the ladder's
// behavior can be driven from a stubbornly-similar model to a cooperative one.
type scriptedReviser struct {
	repairs  []domain.Candidate
	replaces []domain.Candidate
	repaired int
	replaced int
}

func (r *scriptedReviser) Repair(_ context.Context, c domain.Candidate, _ app.Finding) (domain.Candidate, error) {
	r.repaired++
	if len(r.repairs) == 0 {
		return c, nil
	}
	out := r.repairs[0]
	r.repairs = r.repairs[1:]
	return out, nil
}

func (r *scriptedReviser) Replace(_ context.Context, c domain.Candidate, _ app.Finding) (domain.Candidate, error) {
	r.replaced++
	if len(r.replaces) == 0 {
		return c, nil
	}
	out := r.replaces[0]
	r.replaces = r.replaces[1:]
	return out, nil
}

func candidate(prompt string) domain.Candidate {
	return domain.Candidate{Question: coredomain.PackQuestion{
		Type:             coredomain.SingleSelect,
		Prompt:           prompt,
		Choices:          []coredomain.Choice{{ID: "a", Text: "yes"}, {ID: "b", Text: "no"}},
		CorrectChoiceIDs: []string{"a"},
	}}
}

func libraryQuestion(id int64, prompt string) coredomain.Question {
	return coredomain.Question{
		ID:               id,
		Type:             coredomain.SingleSelect,
		Prompt:           prompt,
		Choices:          []coredomain.Choice{{ID: "a", Text: "yes"}, {ID: "b", Text: "no"}},
		CorrectChoiceIDs: []string{"a"},
	}
}

func canonicalOf(prompt string) string {
	q := candidate(prompt).Question
	return domain.CanonicalText(&q)
}

// testCalibration is a threshold pair chosen for the tests, not a measured
// one. It is deliberately not in the production table.
func testCalibration() domain.Calibration {
	return domain.Calibration{
		Model:         gateModel(),
		Version:       "test",
		NearDuplicate: 0.80,
		Reject:        0.95,
		Fixture:       true,
	}
}

func TestACandidateUnlikeAnythingIsAccepted(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("stored"): 0,
		canonicalOf("fresh"):  angleFor(0.10),
	}}
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	gate := app.NewGate(embedder, newMemoryIndex(), library, testCalibration())

	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("fresh")}, &scriptedReviser{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(out.Accepted) != 1 {
		t.Fatalf("want the candidate accepted, got %d accepted", len(out.Accepted))
	}
	if out.Decisions[0].Decision != app.DecisionAccepted {
		t.Errorf("want accepted, got %s", out.Decisions[0].Decision)
	}
	if out.Decisions[0].Attempts != 0 {
		t.Errorf("a clean candidate must spend no attempts, got %d", out.Decisions[0].Attempts)
	}
}

func TestACandidateTooCloseToLibraryContentIsRepairedThenAccepted(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("stored"):    0,
		canonicalOf("too close"): angleFor(0.90),
		canonicalOf("reworded"):  angleFor(0.20),
	}}
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	reviser := &scriptedReviser{repairs: []domain.Candidate{candidate("reworded")}}
	gate := app.NewGate(embedder, newMemoryIndex(), library, testCalibration())

	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("too close")}, reviser)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Decisions[0].Decision != app.DecisionRepaired {
		t.Errorf("want repaired, got %s", out.Decisions[0].Decision)
	}
	if out.Decisions[0].Reason != app.ReasonLibraryNearDuplicate {
		t.Errorf("want a library near-duplicate reason, got %q", out.Decisions[0].Reason)
	}
	if reviser.repaired != 1 || reviser.replaced != 0 {
		t.Errorf("want exactly one repair and no replacement, got %d and %d", reviser.repaired, reviser.replaced)
	}
	if len(out.Accepted) != 1 {
		t.Errorf("the repaired candidate must reach the pack, got %d accepted", len(out.Accepted))
	}
}

// The falsifier for the bounded ladder. A model that keeps returning the same
// too-similar question must end in a rejection; falling through to acceptance
// on exhaustion would make the whole ladder decorative, and every other test
// here would still pass.
func TestACandidateStillTooSimilarWhenTheBudgetRunsOutIsRejected(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("stored"):   0,
		canonicalOf("stubborn"): angleFor(0.90),
	}}
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	// The reviser hands back the same candidate every time.
	reviser := &scriptedReviser{}
	gate := app.NewGate(embedder, newMemoryIndex(), library, testCalibration())

	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("stubborn")}, reviser)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(out.Accepted) != 0 {
		t.Fatalf("a candidate that never stopped being a duplicate must not reach the pack, got %d accepted", len(out.Accepted))
	}
	if out.Decisions[0].Decision != app.DecisionRejected {
		t.Errorf("want rejected, got %s", out.Decisions[0].Decision)
	}
	if out.Decisions[0].Reason != app.ReasonBudgetExhausted {
		t.Errorf("want the budget named as the reason, got %q", out.Decisions[0].Reason)
	}
	if out.Decisions[0].Attempts != 2 {
		t.Errorf("the ladder is bounded at 2 attempts, got %d", out.Decisions[0].Attempts)
	}
}

// Above the reject threshold the two questions are the same question, so
// rewording cannot separate them and spending the repair attempt would buy a
// provider call to learn what is already certain.
func TestANearIdenticalCandidateIsReplacedRatherThanRepaired(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("stored"):      0,
		canonicalOf("almost same"): angleFor(0.97),
		canonicalOf("different"):   angleFor(0.10),
	}}
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	reviser := &scriptedReviser{replaces: []domain.Candidate{candidate("different")}}
	gate := app.NewGate(embedder, newMemoryIndex(), library, testCalibration())

	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("almost same")}, reviser)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if reviser.repaired != 0 {
		t.Errorf("a candidate above the reject threshold must not spend a repair, got %d repairs", reviser.repaired)
	}
	if reviser.replaced != 1 {
		t.Errorf("want one replacement, got %d", reviser.replaced)
	}
	if out.Decisions[0].Decision != app.DecisionReplaced {
		t.Errorf("want replaced, got %s", out.Decisions[0].Decision)
	}
}

// Two candidates in one pack that collide: the first keeps its place and the
// second is the one asked to change. Which one gives way must be deterministic,
// or the same pack screened twice resolves differently.
func TestTwoCollidingCandidatesInOnePackKeepTheFirst(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("first"):  0,
		canonicalOf("second"): angleFor(0.88),
		canonicalOf("fixed"):  angleFor(0.10),
	}}
	reviser := &scriptedReviser{repairs: []domain.Candidate{candidate("fixed")}}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, testCalibration())

	out, err := gate.Apply(context.Background(), 1,
		[]domain.Candidate{candidate("first"), candidate("second")}, reviser)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Decisions[0].Decision != app.DecisionAccepted {
		t.Errorf("the first candidate must keep its place, got %s", out.Decisions[0].Decision)
	}
	if out.Decisions[1].Decision != app.DecisionRepaired {
		t.Errorf("the second candidate is the one that changes, got %s", out.Decisions[1].Decision)
	}
	if out.Decisions[1].Reason != app.ReasonPackNearDuplicate {
		t.Errorf("want an intra-pack reason, got %q", out.Decisions[1].Reason)
	}
}

// A verbatim repeat cannot be reworded into a different question, so it costs
// no repair budget. D-007's hash still governs import; this only avoids buying
// a provider call to learn what is already certain.
func TestAVerbatimDuplicateInOnePackSpendsNoRepairBudget(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{canonicalOf("same"): 0}}
	reviser := &scriptedReviser{}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, testCalibration())

	out, err := gate.Apply(context.Background(), 1,
		[]domain.Candidate{candidate("same"), candidate("same")}, reviser)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.Decisions[1].Decision != app.DecisionRejected {
		t.Errorf("want the repeat rejected, got %s", out.Decisions[1].Decision)
	}
	if out.Decisions[1].Reason != app.ReasonExactDuplicate {
		t.Errorf("want the exact-duplicate reason, got %q", out.Decisions[1].Reason)
	}
	if out.Decisions[1].Attempts != 0 || reviser.repaired != 0 || reviser.replaced != 0 {
		t.Errorf("a verbatim repeat must spend no budget: %d attempts, %d repairs, %d replacements",
			out.Decisions[1].Attempts, reviser.repaired, reviser.replaced)
	}
}

// A rejected candidate must leave nothing behind. Writing candidate vectors to
// the index to compare them with each other would persist embeddings for
// questions no user can ever practice, and every later search would score
// against them.
func TestARejectedCandidateLeavesNoVectorInTheIndex(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{
		canonicalOf("stored"):   0,
		canonicalOf("stubborn"): angleFor(0.90),
	}}
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	index := newMemoryIndex()
	gate := app.NewGate(embedder, index, library, testCalibration())

	if _, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("stubborn")}, &scriptedReviser{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// One vector, and it is the library's.
	stored := index.vectors[gateModel()]
	if len(stored) != 1 {
		t.Fatalf("the index must hold only the library's vector, got %d", len(stored))
	}
	if _, ok := stored[domain.LibraryQuestionID(1)]; !ok {
		t.Error("the stored vector is not the library question's, so a candidate's vector was persisted")
	}
}

// The library is read-only by construction: ports.LibraryReader offers no way
// to write it. This asserts the other half — that the gate did not reorder or
// edit what it read on the way past.
func TestTheGateLeavesLibraryContentExactlyAsItFoundIt(t *testing.T) {
	original := []coredomain.Question{libraryQuestion(1, "stored"), libraryQuestion(2, "also stored")}
	snapshot := append([]coredomain.Question(nil), original...)
	library := &stubLibrary{questions: original}

	embedder := &scriptedEmbedder{angles: map[string]float64{canonicalOf("fresh"): angleFor(0.10)}}
	gate := app.NewGate(embedder, newMemoryIndex(), library, testCalibration())

	if _, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("fresh")}, &scriptedReviser{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for i := range snapshot {
		if library.questions[i].ID != snapshot[i].ID || library.questions[i].Prompt != snapshot[i].Prompt {
			t.Errorf("library question %d changed: %+v became %+v", i, snapshot[i], library.questions[i])
		}
	}
}

// The D-018 falsifier. It has to exercise the gate's DECISION, not a type
// assertion: the candidate below is a blatant duplicate of library content, so
// a policy of "no embedder, wave everything through" would accept it. Asserting
// only the error type would leave that policy untested.
func TestWithoutAnEmbeddingCapabilityTheGateRefusesRatherThanAcceptingEverything(t *testing.T) {
	library := &stubLibrary{questions: []coredomain.Question{libraryQuestion(1, "stored")}}
	gate := app.NewGate(nil, newMemoryIndex(), library, testCalibration())

	// Byte-identical to the stored question: the most obvious duplicate there is.
	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("stored")}, &scriptedReviser{})

	if !errors.Is(err, domain.ErrNoEmbeddingCapability) {
		t.Fatalf("want ErrNoEmbeddingCapability, got %v", err)
	}
	if len(out.Accepted) != 0 {
		t.Errorf("a profile with no embedding capability must not wave a duplicate through; %d accepted", len(out.Accepted))
	}
	if len(out.Decisions) != 0 {
		t.Errorf("no candidate was screened, so no decision may be reported; got %d", len(out.Decisions))
	}
}

// The same refusal must be available before a run spends anything, which is
// what D-018 means by checking the capability before a strategy is chosen.
func TestPreflightReportsAMissingEmbeddingCapabilityBeforeAnyGeneration(t *testing.T) {
	gate := app.NewGate(nil, newMemoryIndex(), &stubLibrary{}, testCalibration())
	if err := gate.Preflight(); !errors.Is(err, domain.ErrNoEmbeddingCapability) {
		t.Fatalf("want ErrNoEmbeddingCapability, got %v", err)
	}
}

// A threshold calibrated for another model still compares — it just no longer
// means anything. That is the failure a plausible-looking score produces, and
// it is refused for the same reason a mixed-dimension search is.
func TestAThresholdCalibratedForAnotherModelIsRefused(t *testing.T) {
	embedder := &scriptedEmbedder{}
	wrong := testCalibration()
	wrong.Model = domain.ModelIdentity{Provider: "openai", Model: "text-embedding-3-small"}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, wrong)

	err := gate.Preflight()
	if !errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
		t.Fatalf("want ErrUncalibratedEmbeddingModel, got %v", err)
	}

	out, applyErr := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("anything")}, &scriptedReviser{})
	if !errors.Is(applyErr, domain.ErrUncalibratedEmbeddingModel) {
		t.Errorf("Apply must refuse too, got %v", applyErr)
	}
	if len(out.Accepted) != 0 {
		t.Errorf("nothing may be accepted under an uncalibrated threshold, got %d", len(out.Accepted))
	}
}

// An empty corpus and a corpus with no matches produce identical findings, so
// the count is the only thing that tells them apart.
func TestAnEmptyLibraryIsReportedAsAnEmptyCorpus(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{canonicalOf("fresh"): 0}}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, testCalibration())

	report, err := gate.Screen(context.Background(), 1, []domain.Candidate{candidate("fresh")})
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if report.CorpusSize != 0 {
		t.Errorf("an empty library must report an empty corpus, got %d", report.CorpusSize)
	}
	if !report.Findings[0].Clean() {
		t.Error("nothing to compare against cannot produce a collision")
	}
}

// Embeddings are bought from the configured provider, so re-embedding the
// whole topic on every run would cost network and tokens proportional to the
// library rather than to the pack being generated.
func TestOnlyLibraryQuestionsWithoutAVectorAreEmbedded(t *testing.T) {
	library := &stubLibrary{questions: []coredomain.Question{
		libraryQuestion(1, "already indexed"),
		libraryQuestion(2, "new"),
	}}
	index := newMemoryIndex()
	embedder := &scriptedEmbedder{angles: map[string]float64{canonicalOf("fresh"): 0}}

	// Question 1 already has a vector from an earlier run.
	if err := index.Put(context.Background(), 1, gateModel(), domain.Vector{1, 0}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	index.puts = 0

	gate := app.NewGate(embedder, index, library, testCalibration())
	if _, err := gate.Screen(context.Background(), 1, []domain.Candidate{candidate("fresh")}); err != nil {
		t.Fatalf("Screen: %v", err)
	}

	if index.puts != 1 {
		t.Errorf("only the un-indexed question should be embedded and stored, got %d puts", index.puts)
	}
	backfill := embedder.calls[0]
	if len(backfill) != 1 || backfill[0] != canonicalOf("new") {
		t.Errorf("the backfill must embed only the missing question, got %v", backfill)
	}
}

// The pairing between texts and vectors is positional, so a short reply would
// silently give a candidate another candidate's vector — a wrong answer rather
// than an error.
func TestAShortEmbedderReplyIsRefusedRatherThanMisattributed(t *testing.T) {
	embedder := &scriptedEmbedder{angles: map[string]float64{}, short: true}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, testCalibration())

	_, err := gate.Screen(context.Background(), 1, []domain.Candidate{candidate("a"), candidate("b")})
	if err == nil {
		t.Fatal("an embedder returning fewer vectors than texts must be refused")
	}
}

func TestAnEmbedderFailureStopsTheGateRatherThanPassingCandidates(t *testing.T) {
	embedder := &scriptedEmbedder{fail: errors.New("endpoint down")}
	gate := app.NewGate(embedder, newMemoryIndex(), &stubLibrary{}, testCalibration())

	out, err := gate.Apply(context.Background(), 1, []domain.Candidate{candidate("a")}, &scriptedReviser{})
	if err == nil {
		t.Fatal("an embedding failure must stop the gate")
	}
	if len(out.Accepted) != 0 {
		t.Errorf("nothing may be accepted when the gate could not score, got %d", len(out.Accepted))
	}
}
