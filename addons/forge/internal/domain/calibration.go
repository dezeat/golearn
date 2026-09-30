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

package domain

import (
	"fmt"
	"math"
)

// ScoredPair is one labeled fixture pair with its measured similarity.
//
// Positive means a human labeled the two questions as near-duplicates: the
// gate is supposed to catch them. The label comes from the fixture set and
// never from a score, which is what keeps the calibration from being circular.
type ScoredPair struct {
	Name     string
	Positive bool
	Score    float64
}

// CalibrationCriteria are the pass conditions for a derivation run.
//
// They are committed before any measurement, because a criterion written
// afterwards is not a criterion — it is a description of whatever the run
// produced.
type CalibrationCriteria struct {
	// MaxFalsePositives is how many non-duplicates may sit above the
	// threshold. FORGE.md 7's quality target favors few false positives, and
	// the asymmetry is real: a false positive discards a good question and
	// spends repair budget, while a false negative ships a near-duplicate that
	// D-007's exact hashing may still catch downstream.
	MaxFalsePositives int
	// MinRecall is the share of labeled duplicates the threshold must catch.
	// Without a floor, "no false positives" is trivially satisfied by a
	// threshold of 1.0 that catches nothing, so this is the other side of the
	// two-sided guard.
	MinRecall float64
	// MinMargin is the separation required between the highest non-duplicate
	// and the threshold, so a fixture set that merely happens to separate does
	// not pass as one that separates robustly.
	MinMargin float64
}

// V1CalibrationCriteria are the pre-registered pass conditions for the V1
// similarity gate.
func V1CalibrationCriteria() CalibrationCriteria {
	return CalibrationCriteria{MaxFalsePositives: 0, MinRecall: 0.80, MinMargin: 0.02}
}

// CalibrationResult reports what a derivation run found.
type CalibrationResult struct {
	Threshold      float64
	FalsePositives int
	FalseNegatives int
	Recall         float64
	Margin         float64
	Positives      int
	Negatives      int
}

// Calibration is a versioned threshold set for one embedding model.
//
// It is scoped to a model because thresholds are no more portable between
// embedding models than the vectors are (D-020): the same pair of questions
// scores differently under different models, so a number derived under one is
// a guess under another.
type Calibration struct {
	Model   ModelIdentity
	Version string
	// NearDuplicate is the score at or above which a candidate is too similar
	// to ship unchanged.
	NearDuplicate float64
	// Reject is the score at or above which the similarity is beyond what
	// rewording can fix, so the candidate is replaced rather than repaired.
	Reject float64
	// Fixture marks a baseline derived from the deterministic test embedder
	// rather than from a real embedding model. It exists so such a number can
	// be recognized and excluded, never shipped as a production threshold.
	Fixture bool
}

// calibrations is the committed table.
//
// It is deliberately empty. Every entry must come from a measurement run
// against a real embedding model on the labeled fixture set, and no such run
// has happened — the live provider lane (#130) is what produces one. An
// invented entry would be indistinguishable from a measured one at the point
// of use, which is precisely why the empty table is the honest state and
// CalibrationFor refuses rather than defaulting.
var calibrations []Calibration

// Calibrations returns the committed threshold sets.
func Calibrations() []Calibration {
	out := make([]Calibration, len(calibrations))
	copy(out, calibrations)
	return out
}

// CalibrationFor returns the committed threshold set for an embedding model,
// or [ErrUncalibratedEmbeddingModel].
func CalibrationFor(m ModelIdentity) (Calibration, error) {
	for _, c := range calibrations {
		if c.Model == m {
			return c, nil
		}
	}
	return Calibration{}, fmt.Errorf(
		"%w: %s has no calibrated similarity threshold, and a threshold derived under another model is a guess",
		ErrUncalibratedEmbeddingModel, m)
}

// thresholdPrecision is the number of decimal places a derived threshold is
// reported to. A threshold carrying float64's full width would imply a
// precision the fixture set cannot support.
const thresholdPrecision = 100

// floatTolerance absorbs the representation error in a difference of two
// two-decimal quantities.
const floatTolerance = 1e-9

// Calibrate derives the lowest threshold that satisfies the criteria.
//
// The procedure, fixed before any measurement: take the highest-scoring
// negative, add the required margin, and round up. That is the lowest cut with
// no false positives, so recall is maximized subject to the criteria's primary
// constraint rather than traded against it. The run then either meets the
// recall floor or fails — it never lowers the cut to reach the floor, because
// doing so would trade away the false-positive guarantee the whole ordering
// exists to protect.
func Calibrate(pairs []ScoredPair, c CalibrationCriteria) (CalibrationResult, error) {
	var positives, negatives int
	highestNegative := math.Inf(-1)
	for _, p := range pairs {
		if p.Positive {
			positives++
			continue
		}
		negatives++
		if p.Score > highestNegative {
			highestNegative = p.Score
		}
	}
	if positives == 0 || negatives == 0 {
		return CalibrationResult{}, fmt.Errorf(
			"calibrate: need both labeled duplicates and labeled non-duplicates, got %d and %d",
			positives, negatives)
	}

	threshold := roundUpTo(highestNegative+c.MinMargin, thresholdPrecision)

	result := CalibrationResult{
		Threshold: threshold,
		Margin:    threshold - highestNegative,
		Positives: positives,
		Negatives: negatives,
	}
	for _, p := range pairs {
		switch {
		case p.Positive && p.Score < threshold:
			result.FalseNegatives++
		case !p.Positive && p.Score >= threshold:
			result.FalsePositives++
		}
	}
	result.Recall = float64(positives-result.FalseNegatives) / float64(positives)

	if result.FalsePositives > c.MaxFalsePositives {
		return result, fmt.Errorf(
			"calibrate: %d false positive(s) at threshold %.2f, criteria allow %d",
			result.FalsePositives, threshold, c.MaxFalsePositives)
	}
	if result.Recall < c.MinRecall {
		return result, fmt.Errorf(
			"calibrate: recall %.2f at threshold %.2f is below the committed floor %.2f; "+
				"the fixture set's classes overlap, so no threshold separates them — "+
				"lowering the cut to reach the floor would trade away the false-positive guarantee",
			result.Recall, threshold, c.MinRecall)
	}
	// Compared with a tolerance because the margin is a difference of two
	// float64s that construction already guarantees: 0.57 - 0.55 is not
	// exactly 0.02 in binary, and an exact comparison would reject the very
	// threshold the procedure just derived.
	if result.Margin < c.MinMargin-floatTolerance {
		return result, fmt.Errorf(
			"calibrate: margin %.4f below the committed minimum %.4f",
			result.Margin, c.MinMargin)
	}
	return result, nil
}

// roundUpTo rounds away from zero at the given precision.
//
// The epsilon is not decoration. A sum like 0.55 + 0.02 is fractionally above
// 0.57 in float64, so a bare Ceil would step it to 0.58 and silently widen
// every derived threshold by a hundredth — a change in the direction of more
// false negatives, arrived at by binary representation rather than by anyone's
// decision.
func roundUpTo(v float64, precision float64) float64 {
	const epsilon = 1e-9
	return math.Ceil(v*precision-epsilon) / precision
}
