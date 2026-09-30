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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	forgeapp "github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/pipeline"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// gateAdapter adapts the similarity gate to the shape the pipeline consumes.
//
// The pipeline declares the narrow interface it needs and this maps the gate
// onto it — the Go convention of accepting an interface at the point of use,
// with the translation living in the composition root rather than in either
// package.
//
// It calls Screen, not Apply. Apply carries its own repair/replace ladder, and
// the pipeline already owns the single repair D-016 permits; running both
// would give one candidate two repair budgets, which is the decision reopened
// by accident. Screen reports, the pipeline decides.
type gateAdapter struct {
	gate    *forgeapp.Gate
	topicID int64
}

func (a gateAdapter) Screen(ctx context.Context, _ string, candidates []domain.Candidate) ([]pipeline.GateVerdict, error) {
	report, err := a.gate.Screen(ctx, a.topicID, candidates)
	if err != nil {
		return nil, err
	}
	verdicts := make([]pipeline.GateVerdict, len(candidates))
	for _, finding := range report.Findings {
		if finding.CandidateIndex < 0 || finding.CandidateIndex >= len(verdicts) {
			continue
		}
		if finding.Clean() {
			continue
		}
		verdicts[finding.CandidateIndex] = pipeline.GateVerdict{
			TooSimilar: true,
			Reason:     fmt.Sprintf("%s (score %.3f)", finding.Reason, finding.TopScore),
		}
	}
	return verdicts, nil
}

// buildGate wires the similarity gate when the configured profile can support
// one, and explains itself when it cannot.
//
// Two independent reasons it may be unavailable, and they are different facts:
//
//   - the provider exposes no embeddings API at all (D-018) — Anthropic;
//   - the embedding model has no committed calibration (D-022), which is the
//     current state of every model, because calibrating one requires measuring
//     a real provider.
//
// Neither is silently tolerated. The caller reports the reason and records it
// in the run diagnostic, because a pack that was never screened for
// near-duplicates must not be indistinguishable from one that passed.
func buildGate(llm ports.Provider, index ports.SimilarityIndex, library ports.LibraryReader, topicID int64) (gate pipeline.SimilarityGate, unavailable string) {
	embedder, err := provider.RequireEmbedder(llm)
	if err != nil {
		return nil, fmt.Sprintf("similarity gate not run: %v", err)
	}

	built, err := forgeapp.NewGate(embedder, index, library)
	if err != nil {
		if errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
			return nil, fmt.Sprintf(
				"similarity gate not run: %s has no committed calibration, and an uncalibrated threshold would be a number picked by feel (D-022)",
				embedder.EmbeddingIdentity())
		}
		return nil, fmt.Sprintf("similarity gate not run: %v", err)
	}
	return gateAdapter{gate: built, topicID: topicID}, ""
}

// libraryReader adapts the core's question repository to the Forge port.
//
// The shapes differ in exactly two ways: the port threads a context, and it
// names the method for what the gate wants rather than for how the core
// stores it. Adapting here rather than widening either side keeps the core
// unaware that Forge exists, which is the one-way rule D-015 fixes.
type libraryReader struct {
	questions interface {
		ListByTopic(topicID int64) ([]coredomain.Question, error)
	}
}

func (l libraryReader) QuestionsByTopic(ctx context.Context, topicID int64) ([]coredomain.Question, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.questions.ListByTopic(topicID)
}
