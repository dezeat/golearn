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

// Package evaluation runs the repo-shipped, offline Forge contract matrix.
package evaluation_test

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/adapters/searxng"
	forgeapp "github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/pipeline"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coreapp "github.com/dezeat/golearn/internal/app"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// Fixtures are the external oracle for this package. The contract tests never
// generate their expected answers from the implementation under test.
//
//go:embed testdata/*.json
var fixtures embed.FS

const fixtureNow = "2026-08-24T01:00:00Z"

func loadFixture(t *testing.T, name string, out any) {
	t.Helper()
	b, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("decode fixture %q: %v", name, err)
	}
}

type matrixFixture struct {
	SchemaVersion string   `json:"schema_version"`
	RubricVersion string   `json:"rubric_version"`
	Axes          []string `json:"axes"`
	Capabilities  []struct {
		ID      string `json:"id"`
		Section string `json:"section"`
	} `json:"capabilities"`
	Cases []struct {
		ID         string `json:"id"`
		Capability string `json:"capability"`
		Fixture    string `json:"fixture"`
		Contract   string `json:"contract"`
		Falsifier  string `json:"falsifier"`
	} `json:"cases"`
}

// TestEvaluationMatrixIsCompleteAndVersioned is the release gate for the
// matrix itself. A new FORGE capability cannot silently ship without a
// fixture-backed contract, and a policy entry must state how it could fail.
func TestEvaluationMatrixIsCompleteAndVersioned(t *testing.T) {
	var matrix matrixFixture
	loadFixture(t, "matrix.json", &matrix)

	if matrix.SchemaVersion != "forge-evaluation-0.1.0" {
		t.Fatalf("matrix schema version = %q, want forge-evaluation-0.1.0", matrix.SchemaVersion)
	}
	if matrix.RubricVersion != "forge-rubric-0.1.0" {
		t.Fatalf("rubric version = %q, want forge-rubric-0.1.0", matrix.RubricVersion)
	}

	wantAxes := []string{
		"topic", "description", "count", "difficulty", "style/mode",
		"answer format", "source quality", "provider profile", "failure", "near-duplicate",
	}
	if !reflect.DeepEqual(matrix.Axes, wantAxes) {
		t.Fatalf("matrix axes = %v, want %v", matrix.Axes, wantAxes)
	}

	wantCapabilities := []string{
		"inputs", "grounding", "validation", "verification", "repair",
		"near-duplicate", "draft-lifecycle", "run-info", "provider-profiles",
		"pack-compatibility", "evaluation-harness",
	}
	seenCapabilities := map[string]int{}
	for _, capability := range matrix.Capabilities {
		seenCapabilities[capability.ID]++
		if capability.Section == "" {
			t.Errorf("capability %q has no FORGE.md section", capability.ID)
		}
	}
	for _, want := range wantCapabilities {
		if seenCapabilities[want] != 1 {
			t.Errorf("capability %q appears %d times; want exactly once", want, seenCapabilities[want])
		}
	}
	if len(seenCapabilities) != len(wantCapabilities) {
		t.Errorf("matrix has %d capabilities, want %d", len(seenCapabilities), len(wantCapabilities))
	}

	seenCases := map[string]bool{}
	caseCapabilities := map[string]bool{}
	for _, c := range matrix.Cases {
		if seenCases[c.ID] {
			t.Errorf("duplicate evaluation case %q", c.ID)
		}
		seenCases[c.ID] = true
		caseCapabilities[c.Capability] = true
		if c.Fixture == "" || c.Contract == "" || c.Falsifier == "" {
			t.Errorf("case %q must name a fixture, contract and falsifier", c.ID)
		}
		if _, err := fixtures.ReadFile("testdata/" + c.Fixture); err != nil {
			t.Errorf("case %q references missing fixture %q: %v", c.ID, c.Fixture, err)
		}
	}
	for _, capability := range wantCapabilities {
		if !caseCapabilities[capability] {
			t.Errorf("capability %q has no fixture-backed case", capability)
		}
	}
}

type fixtureEvidence struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Content     string    `json:"content"`
}

func (e fixtureEvidence) evidence() domain.Evidence {
	return domain.Evidence{
		ID: e.ID, URL: e.URL, Title: e.Title, RetrievedAt: e.RetrievedAt,
		Content: domain.Untrusted(e.Content),
	}
}

type providerFixture struct {
	Identity domain.ModelIdentity `json:"identity"`
	Generate json.RawMessage      `json:"generate"`
	Verify   []json.RawMessage    `json:"verify"`
	Critique []json.RawMessage    `json:"critique"`
	Repair   json.RawMessage      `json:"repair"`
}

type pipelineFixture struct {
	Spec     domain.GenerationSpec `json:"spec"`
	Evidence []fixtureEvidence     `json:"evidence"`
	Provider providerFixture       `json:"provider"`
	Gate     *struct {
		TooSimilar bool   `json:"too_similar"`
		Reason     string `json:"reason"`
	} `json:"gate,omitempty"`
}

type generationExpected struct {
	Accepted         int      `json:"accepted"`
	Repaired         int      `json:"repaired"`
	Rounds           int      `json:"rounds"`
	ProviderCalls    int      `json:"provider_calls"`
	PackVersion      string   `json:"pack_version"`
	TopicSlug        string   `json:"topic_slug"`
	Source           string   `json:"source"`
	Confidence       float64  `json:"confidence"`
	ForgeVersion     string   `json:"forge_version"`
	SourceID         string   `json:"source_id"`
	FirstChoiceText  string   `json:"first_choice_text"`
	SecondType       string   `json:"second_type"`
	SecondCorrectIDs []string `json:"second_correct_ids"`
}

type generationFixture struct {
	pipelineFixture
	Expected generationExpected `json:"expected"`
}

type fixtureStage string

const (
	fixtureGenerate fixtureStage = "generate"
	fixtureVerify   fixtureStage = "verify"
	fixtureCritique fixtureStage = "critique"
	fixtureRepair   fixtureStage = "repair"
)

type fixtureProvider struct {
	mu       sync.Mutex
	identity domain.ModelIdentity
	replies  map[fixtureStage][]json.RawMessage
	calls    []ports.Request
}

func newFixtureProvider(f providerFixture) *fixtureProvider {
	replies := map[fixtureStage][]json.RawMessage{
		fixtureGenerate: {f.Generate},
		fixtureVerify:   append([]json.RawMessage(nil), f.Verify...),
		fixtureCritique: append([]json.RawMessage(nil), f.Critique...),
	}
	if len(f.Repair) > 0 {
		replies[fixtureRepair] = []json.RawMessage{f.Repair}
	}
	return &fixtureProvider{identity: f.Identity, replies: replies}
}

func classifyFixtureRequest(system string) fixtureStage {
	switch {
	case strings.Contains(system, "You are answering a multiple-choice question"):
		return fixtureVerify
	case strings.Contains(system, "You are reviewing one multiple-choice question"):
		return fixtureCritique
	case strings.Contains(system, "You are fixing one multiple-choice question"):
		return fixtureRepair
	default:
		return fixtureGenerate
	}
}

func (p *fixtureProvider) Identity() domain.ModelIdentity { return p.identity }

func (p *fixtureProvider) Generate(ctx context.Context, req ports.Request, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	p.calls = append(p.calls, req)
	stage := classifyFixtureRequest(req.System)
	queue := p.replies[stage]
	if len(queue) == 0 || len(queue[0]) == 0 {
		if stage == fixtureRepair {
			return json.Unmarshal([]byte(`{"questions":[]}`), out)
		}
		return fmt.Errorf("fixture provider has no %s reply", stage)
	}
	payload := queue[0]
	if len(queue) > 1 {
		p.replies[stage] = queue[1:]
	}
	return json.Unmarshal(payload, out)
}

func (p *fixtureProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

type fixtureResearch struct {
	evidence []domain.Evidence
	queries  []ports.Query
}

func (r *fixtureResearch) Gather(_ context.Context, query ports.Query) ([]domain.Evidence, error) {
	r.queries = append(r.queries, query)
	return append([]domain.Evidence(nil), r.evidence...), nil
}

type fixtureGate struct {
	tooSimilar bool
	reason     string
	calls      int
}

func (g *fixtureGate) Screen(_ context.Context, _ string, candidates []domain.Candidate) ([]pipeline.GateVerdict, error) {
	g.calls++
	verdicts := make([]pipeline.GateVerdict, len(candidates))
	if g.tooSimilar && len(verdicts) > 0 {
		verdicts[len(verdicts)-1] = pipeline.GateVerdict{TooSimilar: true, Reason: g.reason}
	}
	return verdicts, nil
}

type finishedFixtureRun struct {
	status     domain.RunStatus
	sources    []domain.SourceRef
	cost       domain.Cost
	diagnostic string
}

type fixtureRunStore struct {
	started  []domain.Run
	finished []finishedFixtureRun
}

func (s *fixtureRunStore) StartRun(_ context.Context, run domain.Run) (int64, error) {
	s.started = append(s.started, run)
	return int64(len(s.started)), nil
}

func (s *fixtureRunStore) FinishRun(_ context.Context, _ int64, status domain.RunStatus,
	sources []domain.SourceRef, cost domain.Cost, diagnostic string) error {
	s.finished = append(s.finished, finishedFixtureRun{
		status: status, sources: sources, cost: cost, diagnostic: diagnostic,
	})
	return nil
}

func (s *fixtureRunStore) RecentRuns(context.Context, int) ([]domain.Run, error) { return nil, nil }

type fixtureDraftStore struct {
	saved   []domain.Draft
	deleted []int64
}

func (s *fixtureDraftStore) SaveDraft(_ context.Context, draft domain.Draft) (int64, error) {
	s.saved = append(s.saved, draft)
	return int64(len(s.saved)), nil
}

func (s *fixtureDraftStore) ListDrafts(context.Context) ([]domain.Draft, error) { return s.saved, nil }
func (s *fixtureDraftStore) GetDraft(context.Context, int64) (domain.Draft, error) {
	return domain.Draft{}, domain.ErrDraftNotFound
}
func (s *fixtureDraftStore) DeleteDraft(_ context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func newFixturePipeline(t *testing.T, f pipelineFixture, gate *fixtureGate) (*pipeline.Pipeline, *fixtureProvider, *fixtureResearch, *fixtureRunStore, *fixtureDraftStore) {
	t.Helper()
	provider := newFixtureProvider(f.Provider)
	research := &fixtureResearch{}
	for _, evidence := range f.Evidence {
		research.evidence = append(research.evidence, evidence.evidence())
	}
	runs := &fixtureRunStore{}
	drafts := &fixtureDraftStore{}
	p, err := pipeline.New(pipeline.Deps{
		Provider: provider, Research: research, Gate: gate,
		Runs: runs, Drafts: drafts,
		Now:          func() time.Time { return time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC) },
		ForgeVersion: "fixture-127",
	}, pipeline.DefaultBudgets())
	if err != nil {
		t.Fatalf("pipeline.New: %v", err)
	}
	return p, provider, research, runs, drafts
}

// TestGenerationFixtureCoversTheRichRequestAndTrustChain exercises the
// provider, research, validation, verification, bounded repair, provenance,
// cost and run-info contracts in one deterministic fixture-backed path.
func TestGenerationFixtureCoversTheRichRequestAndTrustChain(t *testing.T) {
	var fixture generationFixture
	loadFixture(t, "generation.json", &fixture)
	p, provider, research, runs, drafts := newFixturePipeline(t, fixture.pipelineFixture, &fixtureGate{})

	result, err := p.Generate(context.Background(), fixture.Spec)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	want := fixture.Expected
	if result.Accepted != want.Accepted || result.Repaired != want.Repaired || result.Rounds != want.Rounds {
		t.Fatalf("result = accepted %d, repaired %d, rounds %d; want %d, %d, %d",
			result.Accepted, result.Repaired, result.Rounds, want.Accepted, want.Repaired, want.Rounds)
	}
	if provider.callCount() != want.ProviderCalls {
		t.Errorf("provider calls = %d, want %d", provider.callCount(), want.ProviderCalls)
	}
	if len(research.queries) != 1 || research.queries[0].Terms != "Go concurrency Focus on cancellation and channel ownership." {
		t.Errorf("research query = %+v, want the topic plus description", research.queries)
	}
	if len(drafts.saved) != 1 || len(runs.finished) != 1 {
		t.Fatalf("drafts = %d, finished runs = %d; want one of each", len(drafts.saved), len(runs.finished))
	}
	if runs.finished[0].status != domain.RunSucceeded {
		t.Errorf("run status = %q, want succeeded", runs.finished[0].status)
	}
	if runs.finished[0].cost.Attempts != want.ProviderCalls {
		t.Errorf("recorded attempts = %d, want %d", runs.finished[0].cost.Attempts, want.ProviderCalls)
	}

	pack := drafts.saved[0].Pack
	if pack.PackVersion != want.PackVersion || pack.Topic.Slug != want.TopicSlug {
		t.Errorf("pack identity = %q/%q, want %q/%q", pack.PackVersion, pack.Topic.Slug, want.PackVersion, want.TopicSlug)
	}
	if !reflect.DeepEqual(pack.GenerationSpec, &fixture.Spec) {
		t.Errorf("generation spec was not carried exactly: got %+v, want %+v", pack.GenerationSpec, fixture.Spec)
	}
	if pack.Provenance == nil {
		t.Fatal("generated pack has no provenance")
	}
	if pack.Provenance.Model != fixture.Provider.Identity || pack.Provenance.Verifier != fixture.Provider.Identity {
		t.Errorf("provenance identities = %s/%s, want %s/%s", pack.Provenance.Model, pack.Provenance.Verifier, fixture.Provider.Identity, fixture.Provider.Identity)
	}
	if pack.Provenance.ForgeVersion != want.ForgeVersion || len(pack.Provenance.Sources) != 1 || pack.Provenance.Sources[0].ID != want.SourceID {
		t.Errorf("provenance = %+v, want fixture source and Forge version", pack.Provenance)
	}
	if len(pack.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(pack.Questions))
	}
	if pack.Questions[0].Choices[0].Text != want.FirstChoiceText {
		t.Errorf("model presentation leaked into choice text: %q", pack.Questions[0].Choices[0].Text)
	}
	if string(pack.Questions[1].Type) != want.SecondType || !reflect.DeepEqual(pack.Questions[1].CorrectChoiceIDs, want.SecondCorrectIDs) {
		t.Errorf("second answer format = %s/%v, want %s/%v", pack.Questions[1].Type, pack.Questions[1].CorrectChoiceIDs, want.SecondType, want.SecondCorrectIDs)
	}
	for i, question := range pack.Questions {
		if question.Confidence == nil || *question.Confidence != want.Confidence {
			t.Errorf("question %d confidence = %v, want %v", i, question.Confidence, want.Confidence)
		}
		if question.Source != want.Source || question.SourceRef != want.SourceID {
			t.Errorf("question %d provenance = %q/%q, want %q/%q", i, question.Source, question.SourceRef, want.Source, want.SourceID)
		}
	}
	if strings.Contains(runs.finished[0].diagnostic, fixture.Evidence[0].Content) {
		t.Error("run diagnostic persisted raw research content")
	}
}

type validationFixture struct {
	Cases []struct {
		ID            string                  `json:"id"`
		Valid         bool                    `json:"valid"`
		ErrorContains string                  `json:"error_contains"`
		Question      coredomain.PackQuestion `json:"question"`
	} `json:"cases"`
}

func TestValidationFixtureCoversSingleAndMultiSelect(t *testing.T) {
	var fixture validationFixture
	loadFixture(t, "validation.json", &fixture)
	for _, tc := range fixture.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			errs := coredomain.ValidateQuestion(&tc.Question, 0, "fixture")
			if tc.Valid && len(errs) != 0 {
				t.Fatalf("valid fixture rejected: %v", errs)
			}
			if !tc.Valid {
				if len(errs) == 0 {
					t.Fatal("invalid fixture was accepted")
				}
				if !strings.Contains(errs[0].Message, tc.ErrorContains) {
					t.Errorf("first validation error = %q, want %q", errs[0].Message, tc.ErrorContains)
				}
			}
		})
	}
}

type researchFixture struct {
	Results  []map[string]any `json:"results"`
	Expected struct {
		Count        int    `json:"count"`
		FirstID      string `json:"first_id"`
		FirstTitle   string `json:"first_title"`
		FirstURL     string `json:"first_url"`
		FirstContent string `json:"first_content"`
	} `json:"expected"`
}

func TestResearchFixtureProducesUnclassifiedEvidence(t *testing.T) {
	var fixture researchFixture
	loadFixture(t, "research.json", &fixture)
	body, err := fixtures.ReadFile("testdata/research.json")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	now, err := time.Parse(time.RFC3339, fixtureNow)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := searxng.New(searxng.Config{
		BaseURL: server.URL, MaxResponseBytes: 32 * 1024, MaxAttempts: 1,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("searxng.New: %v", err)
	}
	evidence, err := adapter.Gather(context.Background(), ports.Query{
		Terms: "Go concurrency cancellation", MaxResults: 2, MaxBytesPerSource: 4000,
	})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(evidence) != fixture.Expected.Count {
		t.Fatalf("evidence count = %d, want %d", len(evidence), fixture.Expected.Count)
	}
	first := evidence[0]
	if first.ID != fixture.Expected.FirstID || first.Title != fixture.Expected.FirstTitle || first.URL != fixture.Expected.FirstURL || first.Content.Raw() != fixture.Expected.FirstContent {
		t.Errorf("first evidence = %+v, want the fixture fields", first)
	}
	if first.RetrievedAt != now {
		t.Errorf("retrieved_at = %s, want injected %s", first.RetrievedAt, now)
	}
	if first.Quality.Category != domain.SourceCategoryUnclassified || first.Quality.Admissible {
		t.Errorf("source quality = %+v, want unclassified and inadmissible until #120", first.Quality)
	}
}

type providerFixtureFile struct {
	TestKey  string                     `json:"test_key"`
	Profiles map[string]json.RawMessage `json:"profiles"`
}

type fixtureAnswer struct {
	Answer string `json:"answer"`
}

func TestProviderFixtureCoversAllV1Profiles(t *testing.T) {
	var fixture providerFixtureFile
	loadFixture(t, "provider.json", &fixture)
	wantProfiles := []domain.ProfileID{
		domain.ProfileOpenAI, domain.ProfileAnthropic, domain.ProfileOpenRouter, domain.ProfileOllama,
	}
	for _, id := range wantProfiles {
		t.Run(string(id), func(t *testing.T) {
			reply, ok := fixture.Profiles[string(id)]
			if !ok {
				t.Fatalf("fixture has no response for %s", id)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(reply)
			}))
			t.Cleanup(server.Close)

			profile, err := domain.ProfileByID(id)
			if err != nil {
				t.Fatal(err)
			}
			secret := domain.NewSecret(fixture.TestKey, domain.OriginEnvironment)
			var client ports.Provider
			switch id {
			case domain.ProfileOpenAI:
				client = provider.NewOpenAICompatible(profile, "fixture-model", secret, provider.WithEndpoint(server.URL))
			case domain.ProfileOpenRouter:
				client = provider.NewOpenAICompatible(profile, "fixture-model", secret, provider.WithEndpoint(server.URL))
			case domain.ProfileAnthropic:
				client = provider.NewAnthropic(profile, "fixture-model", secret, provider.WithEndpoint(server.URL))
			case domain.ProfileOllama:
				client = provider.NewOllama(profile, "fixture-model", provider.WithEndpoint(server.URL))
			}
			var answer fixtureAnswer
			if err := client.Generate(context.Background(), ports.Request{User: "fixture", Schema: []byte(`{"type":"object"}`)}, &answer); err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if answer.Answer != "42" || client.Identity().Provider != string(id) {
				t.Errorf("answer/identity = %q/%s, want 42/%s", answer.Answer, client.Identity(), id)
			}
			if strings.Contains(client.Identity().String(), server.URL) {
				t.Error("provider identity disclosed its endpoint")
			}
		})
	}
}

func TestFailureFixtureFailsClearWithoutADraft(t *testing.T) {
	var fixture struct {
		pipelineFixture
		Expected struct {
			ErrorContains      string `json:"error_contains"`
			RunStatus          string `json:"run_status"`
			DraftCount         int    `json:"draft_count"`
			DiagnosticContains string `json:"diagnostic_contains"`
		} `json:"expected"`
	}
	loadFixture(t, "failure.json", &fixture)
	p, _, _, runs, drafts := newFixturePipeline(t, fixture.pipelineFixture, &fixtureGate{})
	_, err := p.Generate(context.Background(), fixture.Spec)
	if !errors.Is(err, pipeline.ErrShortfall) || !strings.Contains(err.Error(), fixture.Expected.ErrorContains) {
		t.Fatalf("error = %v, want a clear shortfall containing %q", err, fixture.Expected.ErrorContains)
	}
	if len(drafts.saved) != fixture.Expected.DraftCount {
		t.Errorf("drafts saved = %d, want %d", len(drafts.saved), fixture.Expected.DraftCount)
	}
	if len(runs.finished) != 1 || string(runs.finished[0].status) != fixture.Expected.RunStatus {
		t.Fatalf("finished runs = %+v, want status %q", runs.finished, fixture.Expected.RunStatus)
	}
	if !strings.Contains(runs.finished[0].diagnostic, fixture.Expected.DiagnosticContains) {
		t.Errorf("diagnostic = %q, want %q", runs.finished[0].diagnostic, fixture.Expected.DiagnosticContains)
	}
}

func TestNearDuplicateFixtureFailsClosed(t *testing.T) {
	var fixture struct {
		pipelineFixture
		Expected struct {
			ErrorContains string `json:"error_contains"`
			DraftCount    int    `json:"draft_count"`
		} `json:"expected"`
	}
	loadFixture(t, "near-duplicate.json", &fixture)
	gate := &fixtureGate{tooSimilar: fixture.Gate.TooSimilar, reason: fixture.Gate.Reason}
	p, _, _, _, drafts := newFixturePipeline(t, fixture.pipelineFixture, gate)
	_, err := p.Generate(context.Background(), fixture.Spec)
	if !errors.Is(err, pipeline.ErrShortfall) || !strings.Contains(err.Error(), fixture.Expected.ErrorContains) {
		t.Fatalf("error = %v, want near-duplicate shortfall containing %q", err, fixture.Expected.ErrorContains)
	}
	if gate.calls == 0 {
		t.Fatal("near-duplicate fixture never reached the similarity gate")
	}
	if len(drafts.saved) != fixture.Expected.DraftCount {
		t.Errorf("drafts saved = %d, want %d", len(drafts.saved), fixture.Expected.DraftCount)
	}
}

type reviewFixture struct {
	DraftID  int64           `json:"draft_id"`
	RunID    int64           `json:"run_id"`
	Pack     coredomain.Pack `json:"pack"`
	Expected struct {
		Inserted             int    `json:"inserted"`
		Deleted              bool   `json:"deleted"`
		ImportSourceContains string `json:"import_source_contains"`
	} `json:"expected"`
}

type fixtureImporter struct {
	calls []string
	pack  *coredomain.Pack
}

func (i *fixtureImporter) ImportPack(pack *coredomain.Pack, sourceName string) (*coreapp.ImportResult, error) {
	i.calls = append(i.calls, sourceName)
	i.pack = pack
	return &coreapp.ImportResult{Inserted: len(pack.Questions)}, nil
}

func TestReviewFixtureAcceptsThroughTheImportPort(t *testing.T) {
	var fixture reviewFixture
	loadFixture(t, "review-flow.json", &fixture)
	importer := &fixtureImporter{}
	drafts := &fixtureDraftStore{}
	accepted, err := forgeapp.NewDraftAcceptor(importer, drafts).Accept(context.Background(), domain.Draft{
		ID: fixture.DraftID, RunID: fixture.RunID, Pack: fixture.Pack,
	})
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if accepted.Inserted != fixture.Expected.Inserted || len(importer.calls) != 1 {
		t.Fatalf("accepted/import calls = %+v/%v, want %d/1", accepted, importer.calls, fixture.Expected.Inserted)
	}
	if !strings.Contains(importer.calls[0], fixture.Expected.ImportSourceContains) {
		t.Errorf("import source = %q, want %q", importer.calls[0], fixture.Expected.ImportSourceContains)
	}
	if fixture.Expected.Deleted && !reflect.DeepEqual(drafts.deleted, []int64{fixture.DraftID}) {
		t.Errorf("deleted drafts = %v, want %d", drafts.deleted, fixture.DraftID)
	}
	if importer.pack == nil || importer.pack.Provenance == nil || importer.pack.GenerationSpec == nil {
		t.Fatal("accepted fixture lost generation metadata")
	}
}

type migrationFixture struct {
	ForgeRegistry       string   `json:"forge_registry"`
	ForgeTables         []string `json:"forge_tables"`
	CoreRegistry        string   `json:"core_registry"`
	CoreTablesUntouched []string `json:"core_tables_untouched"`
	PackVersions        struct {
		Accepted []string `json:"accepted"`
		Refused  []string `json:"refused"`
	} `json:"pack_versions"`
}

func schemaSQL(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&sqlText); err != nil {
		t.Fatalf("schema for %s: %v", table, err)
	}
	return sqlText
}

func TestMigrationFixtureKeepsForgeAndCoreRegistriesSeparate(t *testing.T) {
	var fixture migrationFixture
	loadFixture(t, "migration.json", &fixture)
	db, err := coresqlite.Open(t.TempDir() + "/fixture.db")
	if err != nil {
		t.Fatalf("core Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	before := make(map[string]string, len(fixture.CoreTablesUntouched))
	for _, table := range fixture.CoreTablesUntouched {
		before[table] = schemaSQL(t, db, table)
	}
	var beforeCoreMigrations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + fixture.CoreRegistry).Scan(&beforeCoreMigrations); err != nil {
		t.Fatalf("count core migrations before Forge: %v", err)
	}
	if _, err := forgestore.New(context.Background(), db); err != nil {
		t.Fatalf("forgestore.New: %v", err)
	}

	var forgeMigrations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + fixture.ForgeRegistry).Scan(&forgeMigrations); err != nil {
		t.Fatalf("count Forge migrations: %v", err)
	}
	if forgeMigrations == 0 {
		t.Fatal("Forge migration registry is empty")
	}
	var afterCoreMigrations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + fixture.CoreRegistry).Scan(&afterCoreMigrations); err != nil {
		t.Fatalf("count core migrations after Forge: %v", err)
	}
	if afterCoreMigrations != beforeCoreMigrations {
		t.Errorf("core migration rows changed from %d to %d", beforeCoreMigrations, afterCoreMigrations)
	}
	for _, table := range fixture.CoreTablesUntouched {
		if got := schemaSQL(t, db, table); got != before[table] {
			t.Errorf("core table %s changed:\nbefore %s\nafter %s", table, before[table], got)
		}
	}
	for _, table := range fixture.ForgeTables {
		if got := schemaSQL(t, db, table); got == "" {
			t.Errorf("Forge table %s was not created", table)
		}
	}
	for _, version := range fixture.PackVersions.Accepted {
		if msg := coredomain.ValidatePackVersion(version); msg != "" {
			t.Errorf("fixture pack version %s was refused: %s", version, msg)
		}
	}
	for _, version := range fixture.PackVersions.Refused {
		if msg := coredomain.ValidatePackVersion(version); msg == "" {
			t.Errorf("fixture pack version %s was accepted", version)
		}
	}
}
