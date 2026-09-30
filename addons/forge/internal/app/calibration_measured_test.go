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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// The A-22 measurement against a real embedding model, replayed offline.
//
// A-16 showed the deterministic lexical stand-in cannot calibrate the gate and
// left open whether a real model could. It was measured, it could not, and the
// calibration table therefore stays empty (D-022). These tests exist so that
// negative result is executable rather than a sentence in a document: the
// numbers are replayed from testdata, the committed selection rule is applied
// to them again, and the refusal is asserted at the point a caller would meet
// it.
//
// Nothing here reaches the network. The scores were measured once, are
// bit-reproducible, and are stored as scores.

// measuredModel is the model A-22 measured. It is written out rather than read
// from the fixture so that editing the fixture's provenance cannot quietly
// re-point these assertions at a different model.
// measuredRun is one completed live calibration run.
//
// Every field is asserted somewhere below. The expectations are written out
// per run rather than derived from the score file, because a guard that reads
// its expectation from the data it is checking cannot fail.
type measuredRun struct {
	// File is the stored score table under testdata.
	File string
	// Model is the embedding model the run measured, and the identity whose
	// refusal is asserted. Written out rather than read from the file's
	// provenance, so repointing the file at another model is caught.
	Model domain.ModelIdentity
	// Dimension is the vector width the run measured at.
	Dimension int
	// DuplicatesAboveEveryNegative is how many of the labeled duplicates
	// outranked every non-duplicate. It is the number that separates the two
	// runs' failure modes and is asserted exactly.
	DuplicatesAboveEveryNegative int
	// SeparatesByRank records whether every remaining duplicate that did
	// outrank the negatives still lost on the margin (A-22) or whether the
	// model failed to separate at all (A-24).
	SeparatesByRank bool
	// Entry names the experiment log entry this run is recorded in.
	Entry string
}

// measuredRuns are the live calibration runs that have been performed.
//
// Both failed the committed criteria, so the calibration table is still empty.
// They are kept together because the comparison is the finding: two
// independent models, different architectures and widths, fail at the same
// recall for opposite reasons.
var measuredRuns = []measuredRun{
	{
		File:                         "similarity_scores_nomic_embed_text.json",
		Model:                        domain.ModelIdentity{Provider: "ollama", Model: "nomic-embed-text"},
		Dimension:                    768,
		DuplicatesAboveEveryNegative: 6,
		SeparatesByRank:              true,
		Entry:                        "A-22",
	},
	{
		File:                         "similarity_scores_bge_m3.json",
		Model:                        domain.ModelIdentity{Provider: "ollama", Model: "bge-m3"},
		Dimension:                    1024,
		DuplicatesAboveEveryNegative: 5,
		SeparatesByRank:              false,
		Entry:                        "A-24",
	},
}

type measuredScores struct {
	Provenance struct {
		Model          domain.ModelIdentity `json:"model"`
		Dimension      int                  `json:"dimension"`
		FixtureFile    string               `json:"fixture_file"`
		FixtureSHA256  string               `json:"fixture_sha256"`
		CanonicalTexts []string             `json:"canonical_text_sha256"`
	} `json:"provenance"`
	Pairs []struct {
		Name      string  `json:"name"`
		Relation  string  `json:"relation"`
		Duplicate bool    `json:"duplicate"`
		Score     float64 `json:"score"`
	} `json:"pairs"`
}

func loadMeasuredScores(t *testing.T, file string) measuredScores {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read measured scores: %v", err)
	}
	var m measuredScores
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse measured scores: %v", err)
	}
	if len(m.Pairs) == 0 {
		t.Fatal("the measured score set is empty")
	}
	return m
}

func measuredAsScoredPairs(m measuredScores) []domain.ScoredPair {
	out := make([]domain.ScoredPair, 0, len(m.Pairs))
	for _, p := range m.Pairs {
		out = append(out, domain.ScoredPair{Name: p.Name, Positive: p.Duplicate, Score: p.Score})
	}
	return out
}

// The measurement is only evidence about the labeled set if it measured that
// set. A fixture pair added, removed or relabeled after the run would leave
// the scores describing questions that no longer exist, and every conclusion
// below would be about the wrong data.
func TestTheMeasuredScoresStillDescribeTheCommittedFixtureSet(t *testing.T) {
	pairs := loadFixturePairs(t)
	relations := relationsOf(pairs)
	labels := map[string]bool{}
	for _, p := range pairs {
		labels[p.Name] = p.Duplicate
	}

	for _, run := range measuredRuns {
		t.Run(run.Model.Model, func(t *testing.T) {
			m := loadMeasuredScores(t, run.File)

			if len(m.Pairs) != len(pairs) {
				t.Fatalf("%s measured %d pairs but the fixture set now holds %d; the measurement no longer covers it",
					run.Entry, len(m.Pairs), len(pairs))
			}
			for _, p := range m.Pairs {
				want, ok := labels[p.Name]
				if !ok {
					t.Errorf("measured pair %q is no longer in the fixture set", p.Name)
					continue
				}
				if want != p.Duplicate {
					t.Errorf("pair %q: measured under duplicate=%v, fixture now says %v — a label moved after the run",
						p.Name, p.Duplicate, want)
				}
				if got := relations[p.Name]; got != p.Relation {
					t.Errorf("pair %q: measured as relation %q, fixture now says %q", p.Name, p.Relation, got)
				}
			}

			// Tie the scores to the model whose refusal is asserted elsewhere.
			// Without this the two halves drift apart: the scores could be
			// replaced with another model's and every guard would still pass,
			// while "this model is still refused" would be a claim about a
			// model nothing in this file measured.
			if m.Provenance.Model != run.Model {
				t.Errorf("the score file names %s, but this run asserts the refusal of %s",
					m.Provenance.Model, run.Model)
			}
			if m.Provenance.Dimension != run.Dimension {
				t.Errorf("scores measured at %d dimensions, the run expects %d",
					m.Provenance.Dimension, run.Dimension)
			}
			if want := 2 * len(m.Pairs); len(m.Provenance.CanonicalTexts) != want {
				t.Errorf("provenance carries %d canonical-text digests for %d pairs, want %d",
					len(m.Provenance.CanonicalTexts), len(m.Pairs), want)
			}

			raw, err := os.ReadFile(filepath.Join("testdata", m.Provenance.FixtureFile))
			if err != nil {
				t.Fatalf("read the fixture the scores name: %v", err)
			}
			sum := sha256.Sum256(raw)
			if got := hex.EncodeToString(sum[:]); got != m.Provenance.FixtureSHA256 {
				t.Errorf("the fixture file changed since it was measured:\n  measured against %s\n  now             %s",
					m.Provenance.FixtureSHA256, got)
			}
		})
	}
}

// The load-bearing negative result: a real embedding model, measured on the
// committed set under the committed criteria, does not yield a threshold.
//
// If this ever passes, the right response is to read A-22 and re-measure — not
// to seed the table from these stored numbers, which describe one model on one
// day and are kept for reproducibility, not as a source of thresholds.
func TestTheMeasuredModelStillFailsTheCommittedCriteria(t *testing.T) {
	for _, run := range measuredRuns {
		t.Run(run.Model.Model, func(t *testing.T) {
			m := loadMeasuredScores(t, run.File)
			result, err := domain.Calibrate(measuredAsScoredPairs(m), domain.V1CalibrationCriteria())
			if err == nil {
				t.Fatalf("the measured scores now calibrate cleanly at threshold %.4f (recall %.4f); "+
					"%s recorded a failure, so either the scores or the criteria changed and D-022 must be revisited deliberately",
					result.Threshold, result.Recall, run.Entry)
			}

			// Pin *why* it failed. Asserting only "it errors" would still pass
			// if the scores were replaced with noise, or if the criteria were
			// tightened until anything fails — neither of which is the finding.
			if result.FalsePositives != 0 {
				t.Errorf("%s failed on recall with a clean false-positive count; got %d false positives instead",
					run.Entry, result.FalsePositives)
			}
			if result.Recall >= domain.V1CalibrationCriteria().MinRecall {
				t.Errorf("recall %.4f now meets the floor; %s's failure was a recall failure", result.Recall, run.Entry)
			}
		})
	}
}

// The distinction the whole exercise turns on: the model separated the classes
// by rank and failed only the margin.
//
// A-22 records that six of seven duplicates score above every non-duplicate,
// and that the weakest of them clears the highest non-duplicate by ~0.004 —
// about a fifth of the committed 0.02. That is why a threshold of 0.85 would
// have looked respectable and is still refused. This test fails if either half
// of that statement stops being true, because both halves are load-bearing:
// the ranking is what shows the fixture set is separable in principle, and the
// gap is what shows this model does not separate it robustly.
func TestTheTwoMeasuredModelsFailForOppositeReasons(t *testing.T) {
	for _, run := range measuredRuns {
		t.Run(run.Model.Model, func(t *testing.T) {
			m := loadMeasuredScores(t, run.File)

			highestNegative := math.Inf(-1)
			for _, p := range m.Pairs {
				if !p.Duplicate && p.Score > highestNegative {
					highestNegative = p.Score
				}
			}

			var above int
			smallestGap := math.Inf(1)
			for _, p := range m.Pairs {
				if !p.Duplicate || p.Score <= highestNegative {
					continue
				}
				above++
				if gap := p.Score - highestNegative; gap < smallestGap {
					smallestGap = gap
				}
			}

			if above != run.DuplicatesAboveEveryNegative {
				t.Errorf("%s measured %d of 7 duplicates above every non-duplicate, got %d",
					run.Entry, run.DuplicatesAboveEveryNegative, above)
			}

			margin := domain.V1CalibrationCriteria().MinMargin
			if run.SeparatesByRank {
				// A-22: the model ordered the classes correctly and lost on
				// the margin alone. Both halves are load-bearing — the
				// ordering shows the fixture set is separable in principle,
				// the gap shows this model does not separate it robustly.
				if smallestGap <= 0 {
					t.Fatal("no duplicate outranks the highest non-duplicate; the 'separable by rank' finding is gone")
				}
				if smallestGap >= margin {
					t.Errorf("the narrowest winning gap is %.6f, at or above the committed margin %.4f — "+
						"the measured set would now calibrate, which contradicts %s", smallestGap, margin, run.Entry)
				}
				return
			}

			// A-24: this model cleared the margin comfortably for the pairs it
			// did rank correctly, and still failed — because it never lifted
			// the two disjoint-vocabulary duplicates above the negatives at
			// all. A wider margin would not have saved it, which is what makes
			// it a different failure from A-22 rather than a worse one.
			if smallestGap < margin {
				t.Errorf("%s's failure was a separation failure, not a margin failure, "+
					"but the narrowest winning gap %.6f is below the committed margin %.4f",
					run.Entry, smallestGap, margin)
			}
		})
	}
}

// The reject rung fails differently, and the difference is not cosmetic.
//
// With only verbatim-identical pairs counted as duplicates, the highest
// non-duplicate is a paraphrase at ~0.99, so the committed procedure cuts
// above 1.0 — outside the range cosine can return. No reject threshold exists
// for this model on this set at any recall, which the lexical baseline never
// showed because its paraphrases scored far lower.
func TestNoRejectThresholdIsDerivableForTheMeasuredModel(t *testing.T) {
	for _, run := range measuredRuns {
		t.Run(run.Model.Model, func(t *testing.T) {
			m := loadMeasuredScores(t, run.File)

			scored := make([]domain.ScoredPair, 0, len(m.Pairs))
			for _, p := range m.Pairs {
				scored = append(scored, domain.ScoredPair{
					Name: p.Name, Positive: p.Relation == "identical", Score: p.Score,
				})
			}

			result, err := domain.Calibrate(scored, domain.V1CalibrationCriteria())
			if err == nil {
				t.Fatalf("a reject threshold %.4f now derives; %s recorded that none does", result.Threshold, run.Entry)
			}
			if result.Threshold <= 1.0 {
				t.Errorf("%s's reject derivation failed by cutting above the maximum cosine of 1.0; got %.4f, which is a different failure",
					run.Entry, result.Threshold)
			}
		})
	}
}

// The refusal, asserted where a caller meets it.
//
// The measurement did not produce a threshold, so the table must still hold no
// entry for the model that was measured. Seeding it from the stored scores
// would give a derivation that failed the authority of one that passed, which
// is the single thing D-022 exists to prevent.
func TestTheMeasuredModelIsStillRefusedAsUncalibrated(t *testing.T) {
	for _, run := range measuredRuns {
		t.Run(run.Model.Model, func(t *testing.T) {
			if _, err := domain.CalibrationFor(run.Model); !errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
				t.Errorf("%s now has a calibration entry, but the run that measured it failed the committed criteria: %v",
					run.Model, err)
			}

			// Ollama resolves a bare model name to its ":latest" tag, so the
			// tagged form names the same artifact and must be refused for the
			// same reason. An entry added for one spelling and not the other
			// would let the refusal be walked around by typing the model name
			// differently.
			tagged := domain.ModelIdentity{Provider: run.Model.Provider, Model: run.Model.Model + ":latest"}
			if _, err := domain.CalibrationFor(tagged); !errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
				t.Errorf("%s has a calibration entry while %s does not; the same artifact must not be calibrated under one spelling only: %v",
					tagged, run.Model, err)
			}
		})
	}
}
