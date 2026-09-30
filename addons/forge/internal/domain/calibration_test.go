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

package domain_test

import (
	"errors"
	"math"
	"testing"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
)

// Expected values here are arithmetic stated in the test, not values read back
// from the implementation: with a highest negative of 0.55 and a required
// margin of 0.02, the threshold is 0.57 and every positive at or above it is
// caught. That is the external oracle AGENTS.md asks for.
func TestTheThresholdSitsJustAboveTheHighestNonDuplicate(t *testing.T) {
	pairs := []domain.ScoredPair{
		{Name: "unrelated", Score: 0.10},
		{Name: "same topic", Score: 0.42},
		{Name: "shared stem", Score: 0.55},
		{Name: "paraphrase", Positive: true, Score: 0.58},
		{Name: "reordered", Positive: true, Score: 0.62},
		{Name: "same fact", Positive: true, Score: 0.90},
		{Name: "verbatim", Positive: true, Score: 0.99},
	}

	got, err := domain.Calibrate(pairs, domain.V1CalibrationCriteria())
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if math.Abs(got.Threshold-0.57) > 1e-9 {
		t.Errorf("threshold: want 0.57 (0.55 + the 0.02 margin), got %v", got.Threshold)
	}
	if got.FalsePositives != 0 {
		t.Errorf("the criteria forbid false positives, got %d", got.FalsePositives)
	}
	if math.Abs(got.Recall-1.0) > 1e-9 {
		t.Errorf("every positive sits above 0.57, so recall is 1.0, got %v", got.Recall)
	}
}

// The criteria favor few false positives (FORGE.md 7), so a fixture set whose
// classes overlap must fail the derivation rather than settle for the least
// bad cut. A threshold that cannot separate the set is not a threshold.
func TestCalibrationFailsWhenTheClassesOverlapTooMuch(t *testing.T) {
	pairs := []domain.ScoredPair{
		{Name: "unrelated", Score: 0.10},
		{Name: "hard negative", Score: 0.80},
		{Name: "weak paraphrase", Positive: true, Score: 0.50},
		{Name: "moderate", Positive: true, Score: 0.60},
		{Name: "clear", Positive: true, Score: 0.70},
		{Name: "verbatim", Positive: true, Score: 0.95},
	}

	// A no-false-positive threshold lands at 0.82 and catches only 1 of 4
	// positives — recall 0.25, far below the committed floor.
	if _, err := domain.Calibrate(pairs, domain.V1CalibrationCriteria()); err == nil {
		t.Fatal("a set the threshold cannot separate must fail calibration, not produce a number")
	}
}

func TestCalibrationRefusesASetMissingAClass(t *testing.T) {
	tests := map[string][]domain.ScoredPair{
		"no positives": {{Name: "a", Score: 0.1}, {Name: "b", Score: 0.2}},
		"no negatives": {{Name: "a", Positive: true, Score: 0.9}},
		"empty":        {},
	}
	for name, pairs := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.Calibrate(pairs, domain.V1CalibrationCriteria()); err == nil {
				t.Error("a calibration set needs both classes; deriving a threshold from one is meaningless")
			}
		})
	}
}

// Rounding up is what buys the false-positive margin, so it must survive the
// binary representation of a sum like 0.55 + 0.02, which is fractionally above
// 0.57 in float64 and would otherwise round to 0.58.
func TestTheThresholdRoundsUpWithoutInventingAFloatingPointStep(t *testing.T) {
	pairs := []domain.ScoredPair{
		{Name: "negative", Score: 0.55},
		{Name: "positive", Positive: true, Score: 0.99},
	}
	got, err := domain.Calibrate(pairs, domain.V1CalibrationCriteria())
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if math.Abs(got.Threshold-0.57) > 1e-9 {
		t.Errorf("0.55 + 0.02 must land on 0.57, not a step above it; got %v", got.Threshold)
	}
}

// A threshold from another embedding model is exactly as meaningless as a
// vector from another embedding model (D-020). The table is the only source of
// a committed number, and a model absent from it is refused rather than scored
// with a default that would look authoritative.
func TestAnUncalibratedModelIsRefusedRatherThanGivenADefault(t *testing.T) {
	_, err := domain.CalibrationFor(domain.ModelIdentity{Provider: "ollama", Model: "nomic-embed-text"})
	if !errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
		t.Fatalf("want ErrUncalibratedEmbeddingModel, got %v", err)
	}
}

// The fixture embedder's number is a self-consistency baseline for the
// derivation procedure, not a production threshold: it is a lexical-overlap
// score, and a real embedding model would place the same pairs elsewhere.
// Shipping it in the production table would give it an authority it has not
// earned.
func TestTheProductionTableCarriesNoFixtureBaseline(t *testing.T) {
	for _, c := range domain.Calibrations() {
		if c.Fixture {
			t.Errorf("%s is a fixture baseline and must not ship as a production threshold", c.Model)
		}
		if c.Version == "" {
			t.Errorf("%s ships without a version, so a recalibration could not be told from the original", c.Model)
		}
		if c.Reject < c.NearDuplicate {
			t.Errorf("%s has a reject threshold below its near-duplicate threshold, which inverts the ladder", c.Model)
		}
	}
}
