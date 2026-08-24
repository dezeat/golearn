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
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// maxGateAttempts bounds the ladder: one repair, then one replacement, then
// the candidate is rejected. FORGE.md 7 requires the gate to resolve a
// too-similar candidate "within bounded attempts", and an unbounded loop
// against a model that keeps producing the same question is how a run spends
// a budget with nothing to show.
const maxGateAttempts = 2

// nearestLimit caps how many library neighbors the recall stage retrieves, and
// therefore how many judgements one candidate can cost.
//
// Under D-023 this is a *budget*, not a decision: the neighbors it returns are
// classified by the judge, and a neighbor admitted needlessly costs one
// provider call while a neighbor missed costs a shipped duplicate. There is
// deliberately no score cut alongside it — a cut would be a per-model number of
// exactly the kind A-22 and A-24 showed cannot be carried between models, and
// re-introducing one here would smuggle back the threshold D-023 removed.
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
	// ReasonExactDuplicate marks a candidate whose comparison representation is
	// byte-identical to another candidate in the same pack.
	//
	// It is intra-pack only, and deliberately so. A candidate identical to a
	// STORED question is caught by score instead: it reaches 1.0, lands above
	// the reject threshold, and is replaced. The asymmetry is the remedy, not
	// the detection — two siblings can be resolved by dropping one, whereas
	// nothing can be dropped on the library side (FORGE.md 7), so the
	// candidate has to be regenerated either way and the short circuit would
	// save nothing.
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
	// TopScore is the highest retrieval similarity seen, whichever corpus it
	// came from. Under D-023 it is *diagnostic only* — it reports how close
	// the recall stage thought things were and decides nothing. The verdict is
	// Relation.
	TopScore float64
	// Relation is the judge's classification of the closest offender, empty
	// when nothing was found to be a duplicate.
	Relation domain.Relation
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
	// Judge names the model that made the decisions, so a diagnostic says what
	// screened the pack rather than only that something did.
	Judge coredomain.ModelIdentity
	// RecallLimit is how many neighbors each candidate was compared against.
	// It bounds cost and is not a threshold (D-023).
	RecallLimit int
	Findings    []Finding
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
	judge    ports.DuplicateJudge
}

// NewGate wires the cascade: an embedding index that narrows the corpus, and a
// judge that decides (D-023).
//
// It no longer looks up a calibration. Two embedding models were measured
// against the labeled fixture set and both failed the criteria committed
// before them, for the same structural reason — roughly half the compared text
// is answer options, so a legitimate non-duplicate sharing an option set is
// lexically near-identical while a real duplicate phrased differently shares
// almost nothing (FORGE-EXPERIMENTS A-22, A-24). No threshold separates those,
// so the gate stopped asking for one.
//
// D-022's principle survives intact and is in fact strengthened: there is no
// threshold picked by feel because there is no scoring threshold at all. The
// empty calibration table and its guards stay in the tree as the evidence for
// why this design exists.
//
// A nil embedder is the explicit encoding of "this provider profile has no
// embedding capability" (D-018): the composition root's type assertion either
// succeeds or does not, and the failure is handed here rather than discovered
// mid-run. A nil judge is the same kind of fact — without it nothing decides,
// and reporting every retrieved neighbor as a duplicate would be worse than
// refusing.
func NewGate(
	embedder ports.Embedder,
	index ports.SimilarityIndex,
	library ports.LibraryReader,
	judge ports.DuplicateJudge,
) (*Gate, error) {
	g := &Gate{embedder: embedder, index: index, library: library, judge: judge}
	if err := g.Preflight(); err != nil {
		return nil, err
	}
	return g, nil
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
	if g.judge == nil {
		return fmt.Errorf(
			"similarity gate: %w, so retrieved neighbors could be narrowed but never classified",
			domain.ErrNoJudgeCapability)
	}
	return nil
}

// Screen classifies candidates without changing anything.
//
// It is a diagnostic view, not a dry run of [Apply], and the two do not always
// agree: Screen compares each candidate against every earlier one, while Apply
// compares only against the peers it actually accepted. So Screen can report an
// intra-pack collision with a candidate Apply would have rejected anyway. That
// is the right bias for a diagnostic — it shows what is in the batch — but it
// means a Screen finding is not a prediction of an Apply decision.
func (g *Gate) Screen(ctx context.Context, topicID int64, candidates []domain.Candidate) (Report, error) {
	if err := g.Preflight(); err != nil {
		return Report{}, err
	}
	model := g.embedder.EmbeddingIdentity()

	libraryText, err := g.backfillLibrary(ctx, topicID)
	if err != nil {
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

	report := Report{
		Model:       model,
		CorpusSize:  corpus,
		Judge:       g.judge.JudgeIdentity(),
		RecallLimit: nearestLimit,
	}
	var seen []peer
	for i := range candidates {
		finding, err := g.inspect(ctx, i, vectors[i], texts[i], candidates[i].Question, seen, libraryText)
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, finding)
		seen = append(seen, peer{
			originalIndex: i, vector: vectors[i], text: texts[i], question: candidates[i].Question,
		})
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

	libraryText, err := g.backfillLibrary(ctx, topicID)
	if err != nil {
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
			finding, err := g.inspect(ctx, i, vector, text, candidate.Question, accepted, libraryText)
			if err != nil {
				return Outcome{}, err
			}
			decision.TopScore = finding.TopScore

			if finding.Clean() {
				outcome.Accepted = append(outcome.Accepted, candidate)
				accepted = append(accepted, peer{
					originalIndex: i, vector: vector, text: text, question: candidate.Question,
				})
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
			// Escalation is by attempt alone. It used to also fire when the
			// retrieval score crossed a reject threshold, and that threshold
			// no longer exists (D-023). Nor could the judge's own label
			// replace it: A-25 measured exact six-way label accuracy at 0.538
			// while the duplicate/not verdict was perfect, so routing on
			// "identical versus paraphrase" would rest the ladder on the
			// weakest part of the measurement. Repair once, then replace.
			if decision.Attempts == maxGateAttempts {
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
	question      coredomain.PackQuestion
}

// inspect narrows with the index, then decides with the judge.
//
// The split is the whole design (D-023). The index answers "which handful of
// stored questions are worth looking at", which is arithmetic over a corpus;
// the judge answers "is this the same question", which is a provider call over
// a pair. Neither can do the other's job: two embedding models failed the
// second task on the same pair (A-22, A-24), and a judge cannot scan a corpus.
//
// Retrieval order is preserved, so the first duplicate the judge confirms is
// the nearest one — which makes the reported offender the most useful one to
// show a user without ranking the rest.
func (g *Gate) inspect(
	ctx context.Context,
	index int,
	vector domain.Vector,
	text string,
	candidate coredomain.PackQuestion,
	peers []peer,
	libraryText map[domain.LibraryQuestionID]coredomain.PackQuestion,
) (Finding, error) {
	finding := Finding{CandidateIndex: index}

	neighbors, err := g.index.Nearest(ctx, g.embedder.EmbeddingIdentity(), vector, nearestLimit)
	if err != nil {
		return Finding{}, fmt.Errorf("similarity gate: searching the library for candidate %d: %w", index, err)
	}
	for _, n := range neighbors {
		if n.Score > finding.TopScore {
			finding.TopScore = n.Score
		}
		stored, ok := libraryText[n.QuestionID]
		if !ok {
			// The index holds a vector for a question the topic read did not
			// return. Skipping is right rather than fatal: the corpus can
			// legitimately shrink between runs, and a vector outliving its
			// question is the store's business, not the gate's.
			continue
		}
		relation, err := g.judge.Judge(ctx, candidate, stored)
		if err != nil {
			return Finding{}, fmt.Errorf("similarity gate: judging candidate %d against library question %d: %w",
				index, int64(n.QuestionID), err)
		}
		if !relation.IsDuplicate() {
			continue
		}
		finding.Library = append(finding.Library, n)
		if finding.Reason == ReasonNone {
			finding.Reason, finding.Relation = ReasonLibraryNearDuplicate, relation
		}
	}

	for _, p := range peers {
		if p.originalIndex == index {
			continue
		}
		// A byte-identical comparison representation needs no provider call to
		// settle, and buying one would spend a judgement to learn what string
		// equality already proved.
		if p.text == text {
			score, err := domain.Cosine(vector, p.vector)
			if err != nil {
				return Finding{}, fmt.Errorf("similarity gate: comparing candidates %d and %d: %w", index, p.originalIndex, err)
			}
			finding.Reason, finding.Relation = ReasonExactDuplicate, domain.RelationIdentical
			finding.TopScore = score
			finding.Pack = append(finding.Pack, PackNeighbor{CandidateIndex: p.originalIndex, Score: score})
			return finding, nil
		}

		score, err := domain.Cosine(vector, p.vector)
		if err != nil {
			return Finding{}, fmt.Errorf("similarity gate: comparing candidates %d and %d: %w", index, p.originalIndex, err)
		}
		if score > finding.TopScore {
			finding.TopScore = score
		}
		relation, err := g.judge.Judge(ctx, candidate, p.question)
		if err != nil {
			return Finding{}, fmt.Errorf("similarity gate: judging candidates %d and %d: %w",
				index, p.originalIndex, err)
		}
		if !relation.IsDuplicate() {
			continue
		}
		finding.Pack = append(finding.Pack, PackNeighbor{CandidateIndex: p.originalIndex, Score: score})
		if finding.Reason == ReasonNone {
			finding.Reason, finding.Relation = ReasonPackNearDuplicate, relation
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
func (g *Gate) backfillLibrary(ctx context.Context, topicID int64) (map[domain.LibraryQuestionID]coredomain.PackQuestion, error) {
	questions, err := g.library.QuestionsByTopic(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("similarity gate: reading topic %d: %w", topicID, err)
	}
	if len(questions) == 0 {
		return nil, nil
	}

	model := g.embedder.EmbeddingIdentity()
	// The projected questions are kept and handed back rather than discarded
	// after embedding: the judge needs the stored question's text, and reading
	// the topic a second time to get what this pass already holds would double
	// the database work for nothing.
	byID := make(map[domain.LibraryQuestionID]coredomain.PackQuestion, len(questions))
	canonical := make(map[domain.LibraryQuestionID]string, len(questions))
	ids := make([]domain.LibraryQuestionID, 0, len(questions))
	for _, q := range questions {
		id := domain.LibraryQuestionID(q.ID)
		projected := domain.AsPackQuestion(q)
		byID[id] = projected
		canonical[id] = domain.CanonicalText(&projected)
		ids = append(ids, id)
	}

	missing, err := g.index.Missing(ctx, model, ids)
	if err != nil {
		return nil, fmt.Errorf("similarity gate: %w", err)
	}
	if len(missing) == 0 {
		return byID, nil
	}

	texts := make([]string, len(missing))
	for i, id := range missing {
		texts[i] = canonical[id]
	}
	vectors, err := g.embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	for i, id := range missing {
		if err := g.index.Put(ctx, id, model, vectors[i]); err != nil {
			return nil, fmt.Errorf("similarity gate: %w", err)
		}
	}
	return byID, nil
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
