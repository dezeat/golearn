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

package app

import (
	"context"
	"fmt"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// maxGateAttempts bounds the ladder: one repair, then one replacement, then
// the candidate is rejected. FORGE.md 7 requires the gate to resolve a
// too-similar candidate "within bounded attempts", and an unbounded loop
// against a model that keeps producing the same question is how a run spends
// a budget with nothing to show.
const maxGateAttempts = 2

// nearestLimit caps how many library neighbors a probe retrieves. The gate
// only needs to know whether anything crossed the threshold and what the worst
// offender was, so a handful is enough to diagnose with and nothing is gained
// by ranking the whole corpus.
const nearestLimit = 5

// Decision is the gate's verdict on one candidate.
type Decision string

const (
	// DecisionAccepted means the candidate went into the pack as generated.
	DecisionAccepted Decision = "accepted"
	// DecisionRepaired means it was reworded and then passed.
	DecisionRepaired Decision = "repaired"
	// DecisionReplaced means a fresh candidate took its place and passed.
	DecisionReplaced Decision = "replaced"
	// DecisionRejected means it left the pack. Existing library content is
	// never the thing that gives way — FORGE.md 7 forbids modifying it — so
	// the candidate is always the side that loses.
	DecisionRejected Decision = "rejected"
)

// Reason names why the gate acted.
type Reason string

const (
	// ReasonNone marks a candidate nothing was wrong with.
	ReasonNone Reason = ""
	// ReasonExactDuplicate marks a candidate whose comparison representation
	// is byte-identical to something already present.
	ReasonExactDuplicate Reason = "exact-duplicate"
	// ReasonLibraryNearDuplicate marks a candidate too close to stored content.
	ReasonLibraryNearDuplicate Reason = "library-near-duplicate"
	// ReasonPackNearDuplicate marks two candidates in one pack that collide.
	ReasonPackNearDuplicate Reason = "pack-near-duplicate"
	// ReasonBudgetExhausted marks a candidate still too similar after the
	// bounded attempts ran out.
	ReasonBudgetExhausted Reason = "budget-exhausted"
)

// PackNeighbor is another candidate in the same pack, addressed by position.
//
// It is a separate type from [ports.Neighbor] because it addresses a separate
// id space — a position in a slice that exists only during this run, versus a
// row the core owns. Sharing one type between them is what made the original
// port ambiguous.
type PackNeighbor struct {
	CandidateIndex int
	Score          float64
}

// Finding is what the gate observed about one candidate.
type Finding struct {
	CandidateIndex int
	Reason         Reason
	// Library holds the stored questions that crossed the threshold.
	Library []ports.Neighbor
	// Pack holds the same-pack candidates that crossed it.
	Pack []PackNeighbor
	// TopScore is the highest similarity seen, whichever corpus it came from.
	TopScore float64
}

// Clean reports whether the candidate may ship as it stands.
func (f Finding) Clean() bool { return f.Reason == ReasonNone }

// Report is the gate's read-only diagnostic view.
type Report struct {
	Model domain.ModelIdentity
	// CorpusSize is how many stored vectors the candidates were compared
	// against. The gate reports it so an empty corpus reads as "there was
	// nothing to compare against" rather than as "nothing was too similar" —
	// two very different statements that produce identical findings.
	CorpusSize int
	Threshold  float64
	Findings   []Finding
}

// CandidateDecision records how one candidate was resolved.
type CandidateDecision struct {
	// OriginalIndex is the candidate's position in the input, which stays
	// meaningful after repairs and replacements have changed the content.
	OriginalIndex int
	Decision      Decision
	Reason        Reason
	Attempts      int
	TopScore      float64
}

// Outcome is the result of driving the ladder.
type Outcome struct {
	Accepted   []domain.Candidate
	Decisions  []CandidateDecision
	CorpusSize int
}

// Reviser produces a better candidate for one the gate found too similar.
//
// The gate decides *that* a candidate must change and the reviser decides
// *how*: repair reworks the question it is given, replacement discards it and
// generates a different one. Keeping the two apart is what lets the gate bound
// the attempts without knowing anything about generation.
type Reviser interface {
	Repair(ctx context.Context, c domain.Candidate, f Finding) (domain.Candidate, error)
	Replace(ctx context.Context, c domain.Candidate, f Finding) (domain.Candidate, error)
}

// Gate is the near-duplicate gate: a background pipeline stage, not a review
// chore (FORGE.md 7).
type Gate struct {
	embedder ports.Embedder
	index    ports.SimilarityIndex
	library  ports.LibraryReader
	calib    domain.Calibration
}

// NewGate wires the gate.
//
// A nil embedder is the explicit encoding of "this provider profile has no
// embedding capability". D-018 makes [ports.Embedder] a separate optional
// interface precisely because Anthropic ships no embeddings API, so the
// composition root's type assertion either succeeds or does not, and the
// failure is handed here as nil rather than discovered mid-run.
func NewGate(embedder ports.Embedder, index ports.SimilarityIndex, library ports.LibraryReader, calib domain.Calibration) *Gate {
	return &Gate{embedder: embedder, index: index, library: library, calib: calib}
}

// Preflight reports whether the gate can run at all, before a run spends a
// single generation.
//
// D-018 requires the no-capability case to be checked before a strategy is
// chosen rather than discovered mid-run, and both refusals here are of that
// kind: they are properties of the configured profile, knowable at startup.
func (g *Gate) Preflight() error {
	if g.embedder == nil {
		return fmt.Errorf(
			"similarity gate: %w, so no candidate can be compared for near-duplication",
			domain.ErrNoEmbeddingCapability)
	}

	// A threshold calibrated for another model is exactly as meaningless as a
	// vector from another model, and far easier to miss: the numbers still
	// compare, they just no longer mean anything. Refusing here is the same
	// refusal the index makes on a mixed-dimension search.
	if identity := g.embedder.EmbeddingIdentity(); identity != g.calib.Model {
		return fmt.Errorf(
			"%w: the gate is calibrated for %s but the profile embeds with %s",
			domain.ErrUncalibratedEmbeddingModel, g.calib.Model, identity)
	}
	return nil
}

// Screen classifies candidates without changing anything.
func (g *Gate) Screen(ctx context.Context, topicID int64, candidates []domain.Candidate) (Report, error) {
	if err := g.Preflight(); err != nil {
		return Report{}, err
	}
	model := g.embedder.EmbeddingIdentity()

	if err := g.backfillLibrary(ctx, topicID); err != nil {
		return Report{}, err
	}
	corpus, err := g.index.Count(ctx, model)
	if err != nil {
		return Report{}, fmt.Errorf("similarity gate: %w", err)
	}

	vectors, texts, err := g.embedCandidates(ctx, candidates)
	if err != nil {
		return Report{}, err
	}

	report := Report{Model: model, CorpusSize: corpus, Threshold: g.calib.NearDuplicate}
	var seen []peer
	for i := range candidates {
		finding, err := g.inspect(ctx, i, vectors[i], texts[i], seen)
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, finding)
		seen = append(seen, peer{originalIndex: i, vector: vectors[i], text: texts[i]})
	}
	return report, nil
}

// Apply drives the bounded repair/replace/reject ladder.
//
// Candidates are resolved in order and compared against the library plus the
// peers already accepted, so the first of two colliding siblings keeps its
// place and the second is the one asked to change. That is deterministic,
// which matters: the same pack screened twice must resolve the same way.
func (g *Gate) Apply(ctx context.Context, topicID int64, candidates []domain.Candidate, reviser Reviser) (Outcome, error) {
	if err := g.Preflight(); err != nil {
		return Outcome{}, err
	}
	if reviser == nil {
		return Outcome{}, fmt.Errorf("similarity gate: a bounded ladder needs a reviser to produce the replacement")
	}
	model := g.embedder.EmbeddingIdentity()

	if err := g.backfillLibrary(ctx, topicID); err != nil {
		return Outcome{}, err
	}
	corpus, err := g.index.Count(ctx, model)
	if err != nil {
		return Outcome{}, fmt.Errorf("similarity gate: %w", err)
	}

	outcome := Outcome{CorpusSize: corpus}
	// Accepted candidates are compared in memory and never written to the
	// index: a candidate still being decided may yet be rejected, and a
	// rejected candidate must leave no vector behind in a store whose whole
	// contents are meant to be practicable content.
	var accepted []peer

	for i := range candidates {
		candidate := candidates[i]
		decision := CandidateDecision{OriginalIndex: i, Decision: DecisionAccepted}

		// The loop bound is what guarantees termination, rather than any
		// comparison inside it. That ordering is deliberate: when the exit
		// depended on the budget check, weakening that check did not produce a
		// wrong decision, it produced a process that never returned. A
		// structural bound degrades into a wrong answer a test can see, and
		// leaves rejection — not acceptance — as the outcome of falling
		// through.
		resolved := false
		for attempt := 0; attempt <= maxGateAttempts && !resolved; attempt++ {
			vector, text, err := g.embedOne(ctx, candidate)
			if err != nil {
				return Outcome{}, err
			}
			finding, err := g.inspect(ctx, i, vector, text, accepted)
			if err != nil {
				return Outcome{}, err
			}
			decision.TopScore = finding.TopScore

			if finding.Clean() {
				outcome.Accepted = append(outcome.Accepted, candidate)
				accepted = append(accepted, peer{originalIndex: i, vector: vector, text: text})
				resolved = true
				continue
			}

			// A candidate whose comparison representation is byte-identical to
			// something already present cannot be reworded into a different
			// question, so the repair budget is not spent on it. D-007's hash
			// still governs import; this only avoids buying a provider call to
			// learn what is already certain.
			if finding.Reason == ReasonExactDuplicate {
				decision.Decision, decision.Reason = DecisionRejected, ReasonExactDuplicate
				resolved = true
				continue
			}

			if attempt == maxGateAttempts {
				// No budget left to spend on it; the loop ends here and the
				// candidate falls through to the rejection below.
				break
			}

			decision.Attempts++
			decision.Reason = finding.Reason
			// Above the reject threshold the two questions are the same
			// question, so rewording cannot separate them and the repair
			// attempt would be spent to learn that. The last attempt is always
			// a replacement for the same reason.
			if finding.TopScore >= g.calib.Reject || decision.Attempts == maxGateAttempts {
				decision.Decision = DecisionReplaced
				candidate, err = reviser.Replace(ctx, candidate, finding)
			} else {
				decision.Decision = DecisionRepaired
				candidate, err = reviser.Repair(ctx, candidate, finding)
			}
			if err != nil {
				return Outcome{}, fmt.Errorf("similarity gate: revising candidate %d: %w", i, err)
			}
		}

		// Unresolved means the ladder ran out with the candidate still too
		// similar. It leaves the pack: existing library content is never the
		// side that gives way (FORGE.md 7).
		if !resolved {
			decision.Decision, decision.Reason = DecisionRejected, ReasonBudgetExhausted
		}

		outcome.Decisions = append(outcome.Decisions, decision)
	}
	return outcome, nil
}

// peer is one already-settled candidate a later one is compared against.
//
// It carries the original index rather than its position among the settled
// ones, so a finding always addresses the candidate the caller handed in even
// after earlier candidates have been rejected and the two numberings diverge.
type peer struct {
	originalIndex int
	vector        domain.Vector
	text          string
}

// inspect scores one candidate against the library and the given peers.
func (g *Gate) inspect(
	ctx context.Context,
	index int,
	vector domain.Vector,
	text string,
	peers []peer,
) (Finding, error) {
	finding := Finding{CandidateIndex: index}

	neighbors, err := g.index.Nearest(ctx, g.embedder.EmbeddingIdentity(), vector, nearestLimit)
	if err != nil {
		return Finding{}, fmt.Errorf("similarity gate: searching the library for candidate %d: %w", index, err)
	}
	for _, n := range neighbors {
		if n.Score >= g.calib.NearDuplicate {
			finding.Library = append(finding.Library, n)
			if n.Score > finding.TopScore {
				finding.TopScore = n.Score
				finding.Reason = ReasonLibraryNearDuplicate
			}
		}
	}

	for _, p := range peers {
		if p.originalIndex == index {
			continue
		}
		score, err := domain.Cosine(vector, p.vector)
		if err != nil {
			return Finding{}, fmt.Errorf("similarity gate: comparing candidates %d and %d: %w", index, p.originalIndex, err)
		}
		if p.text == text {
			finding.Reason = ReasonExactDuplicate
			finding.TopScore = score
			finding.Pack = append(finding.Pack, PackNeighbor{CandidateIndex: p.originalIndex, Score: score})
			return finding, nil
		}
		if score >= g.calib.NearDuplicate {
			finding.Pack = append(finding.Pack, PackNeighbor{CandidateIndex: p.originalIndex, Score: score})
			if score > finding.TopScore {
				finding.TopScore = score
				finding.Reason = ReasonPackNearDuplicate
			}
		}
	}
	return finding, nil
}

// backfillLibrary embeds the topic's stored questions that have no vector yet.
//
// Only the missing ones: embeddings come from the configured provider (D-018),
// so re-embedding the whole topic on every run would spend network and tokens
// proportional to the library rather than to the pack being generated. The
// library is read here and never written — ports.LibraryReader offers no way
// to write it, which is how FORGE.md 7's "existing content is never modified"
// stays true by construction rather than by discipline.
func (g *Gate) backfillLibrary(ctx context.Context, topicID int64) error {
	questions, err := g.library.QuestionsByTopic(ctx, topicID)
	if err != nil {
		return fmt.Errorf("similarity gate: reading topic %d: %w", topicID, err)
	}
	if len(questions) == 0 {
		return nil
	}

	model := g.embedder.EmbeddingIdentity()
	byID := make(map[domain.LibraryQuestionID]string, len(questions))
	ids := make([]domain.LibraryQuestionID, 0, len(questions))
	for _, q := range questions {
		id := domain.LibraryQuestionID(q.ID)
		projected := domain.AsPackQuestion(q)
		byID[id] = domain.CanonicalText(&projected)
		ids = append(ids, id)
	}

	missing, err := g.index.Missing(ctx, model, ids)
	if err != nil {
		return fmt.Errorf("similarity gate: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	texts := make([]string, len(missing))
	for i, id := range missing {
		texts[i] = byID[id]
	}
	vectors, err := g.embed(ctx, texts)
	if err != nil {
		return err
	}
	for i, id := range missing {
		if err := g.index.Put(ctx, id, model, vectors[i]); err != nil {
			return fmt.Errorf("similarity gate: %w", err)
		}
	}
	return nil
}

func (g *Gate) embedCandidates(ctx context.Context, candidates []domain.Candidate) ([]domain.Vector, []string, error) {
	texts := make([]string, len(candidates))
	for i := range candidates {
		texts[i] = domain.CanonicalText(&candidates[i].Question)
	}
	vectors, err := g.embed(ctx, texts)
	if err != nil {
		return nil, nil, err
	}
	return vectors, texts, nil
}

func (g *Gate) embedOne(ctx context.Context, c domain.Candidate) (domain.Vector, string, error) {
	text := domain.CanonicalText(&c.Question)
	vectors, err := g.embed(ctx, []string{text})
	if err != nil {
		return nil, "", err
	}
	return vectors[0], text, nil
}

// embed calls the provider and checks the shape of what came back, because a
// short or misaligned reply would otherwise pair a candidate with another
// candidate's vector — a wrong answer rather than an error.
func (g *Gate) embed(ctx context.Context, texts []string) ([]domain.Vector, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vectors, err := g.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("similarity gate: embedding %d text(s): %w", len(texts), err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf(
			"similarity gate: embedder returned %d vectors for %d texts; the pairing is positional, so a short reply would silently misattribute them",
			len(vectors), len(texts))
	}
	return vectors, nil
}
