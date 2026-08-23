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
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/pipeline"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// stage names the role a provider call is playing, recovered from the
// operator-authored system prompt. The fake routes on it so a test can script
// each stage independently and assert on what each one was actually sent.
type stage string

const (
	stageGenerate stage = "generate"
	stageVerify   stage = "verify"
	stageCritique stage = "critique"
	stageRepair   stage = "repair"
)

func classify(system string) stage {
	switch {
	case strings.Contains(system, "You are answering a multiple-choice question"):
		return stageVerify
	case strings.Contains(system, "You are reviewing one multiple-choice question"):
		return stageCritique
	case strings.Contains(system, "You are fixing one multiple-choice question"):
		return stageRepair
	default:
		return stageGenerate
	}
}

// call records one provider interaction for later assertion.
type call struct {
	Stage   stage
	Request ports.Request
}

// fakeProvider is a scripted provider. Each stage has a queue of replies;
// exhausting a queue repeats the last one, so a test only scripts what it
// cares about.
type fakeProvider struct {
	mu       sync.Mutex
	identity domain.ModelIdentity
	replies  map[stage][]any
	errs     map[stage]error
	calls    []call

	// observeDeadline records how much time each call was actually given, so a
	// test can assert the configured budget reached the provider rather than
	// only that it was configured.
	observeDeadline bool
	deadlines       []time.Duration
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{
		identity: domain.ModelIdentity{Provider: "ollama", Model: "test-model"},
		replies:  map[stage][]any{},
		errs:     map[stage]error{},
	}
}

func (f *fakeProvider) Identity() domain.ModelIdentity { return f.identity }

func (f *fakeProvider) reply(s stage, values ...any) *fakeProvider {
	f.replies[s] = append(f.replies[s], values...)
	return f
}

func (f *fakeProvider) fail(s stage, err error) *fakeProvider {
	f.errs[s] = err
	return f
}

func (f *fakeProvider) Generate(ctx context.Context, req ports.Request, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	s := classify(req.System)
	f.calls = append(f.calls, call{Stage: s, Request: req})
	if f.observeDeadline {
		if deadline, ok := ctx.Deadline(); ok {
			f.deadlines = append(f.deadlines, time.Until(deadline))
		} else {
			f.deadlines = append(f.deadlines, 0)
		}
	}
	if err := f.errs[s]; err != nil {
		return err
	}

	queue := f.replies[s]
	if len(queue) == 0 {
		return nil
	}
	value := queue[0]
	if len(queue) > 1 {
		f.replies[s] = queue[1:]
	}
	// Round-trip through JSON so the fake exercises the same decoding the real
	// adapters do, rather than assigning a Go value the model could not have
	// produced.
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

func (f *fakeProvider) callsFor(s stage) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Stage == s {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeProvider) countFor(s stage) int { return len(f.callsFor(s)) }

// fakeResearch returns a fixed evidence set.
type fakeResearch struct {
	evidence []domain.Evidence
	err      error
	queries  []ports.Query
}

func (f *fakeResearch) Gather(_ context.Context, q ports.Query) ([]domain.Evidence, error) {
	f.queries = append(f.queries, q)
	return f.evidence, f.err
}

// fakeGate scripts the similarity verdict for the candidate under test, which
// the pipeline always passes last.
type fakeGate struct {
	tooSimilar []bool
	calls      int
	err        error
	batchSizes []int
}

func (f *fakeGate) Screen(_ context.Context, _ string, candidates []domain.Candidate) ([]pipeline.GateVerdict, error) {
	f.calls++
	f.batchSizes = append(f.batchSizes, len(candidates))
	if f.err != nil {
		return nil, f.err
	}
	verdicts := make([]pipeline.GateVerdict, len(candidates))
	tooSimilar := false
	if len(f.tooSimilar) > 0 {
		tooSimilar = f.tooSimilar[0]
		if len(f.tooSimilar) > 1 {
			f.tooSimilar = f.tooSimilar[1:]
		}
	}
	if len(verdicts) > 0 && tooSimilar {
		verdicts[len(verdicts)-1] = pipeline.GateVerdict{
			TooSimilar: true, Reason: "near-duplicate of an existing question",
		}
	}
	return verdicts, nil
}

// fakeRunStore records the run lifecycle.
type fakeRunStore struct {
	started    []domain.Run
	finishedAs []domain.RunStatus
	diagnostic []string
	sources    [][]domain.SourceRef
	costs      []domain.Cost
	nextID     int64
}

func (f *fakeRunStore) StartRun(_ context.Context, run domain.Run) (int64, error) {
	f.started = append(f.started, run)
	f.nextID++
	return f.nextID, nil
}

func (f *fakeRunStore) FinishRun(_ context.Context, _ int64, status domain.RunStatus,
	sources []domain.SourceRef, cost domain.Cost, diagnostic string) error {
	f.finishedAs = append(f.finishedAs, status)
	f.diagnostic = append(f.diagnostic, diagnostic)
	f.sources = append(f.sources, sources)
	f.costs = append(f.costs, cost)
	return nil
}

func (f *fakeRunStore) RecentRuns(context.Context, int) ([]domain.Run, error) { return nil, nil }

// fakeDraftStore records what, if anything, was saved.
type fakeDraftStore struct {
	saved []domain.Draft
	err   error
}

func (f *fakeDraftStore) SaveDraft(_ context.Context, d domain.Draft) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.saved = append(f.saved, d)
	return int64(len(f.saved)), nil
}

func (f *fakeDraftStore) ListDrafts(context.Context) ([]domain.Draft, error) { return f.saved, nil }
func (f *fakeDraftStore) GetDraft(context.Context, int64) (domain.Draft, error) {
	return domain.Draft{}, domain.ErrDraftNotFound
}
func (f *fakeDraftStore) DeleteDraft(context.Context, int64) error { return nil }
