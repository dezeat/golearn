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
	"encoding/json"
	"errors"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// fixtureDim is the width of the deterministic test embedder. Wide enough that
// hash collisions between the fixture set's tokens are rare, narrow enough to
// stay cheap.
const fixtureDim = 256

// lexicalEmbedder is a deterministic, dependency-free stand-in: it hashes
// tokens into a bag-of-words vector and L2-normalizes it, so cosine over its
// output is lexical overlap.
//
// It is emphatically NOT a semantic embedding model, and the calibration run
// below exists partly to demonstrate that difference rather than paper over
// it. It earns its place by being reproducible on any machine with no network
// and no dependency, which is what a fixture needs to be.
type lexicalEmbedder struct{}

func (lexicalEmbedder) identity() domain.ModelIdentity {
	return domain.ModelIdentity{Provider: "fixture", Model: "hashed-bow-256"}
}

func (lexicalEmbedder) embed(text string) domain.Vector {
	v := make(domain.Vector, fixtureDim)
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		v[h.Sum32()%fixtureDim]++
	}
	var norm float64
	for _, f := range v {
		norm += float64(f) * float64(f)
	}
	if norm == 0 {
		return v
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

type fixtureQuestion struct {
	Prompt  string     `json:"prompt"`
	Choices [][]string `json:"choices"`
	Correct []string   `json:"correct"`
	Tags    []string   `json:"tags"`
}

func (q fixtureQuestion) packQuestion() coredomain.PackQuestion {
	choices := make([]coredomain.Choice, 0, len(q.Choices))
	for _, c := range q.Choices {
		choices = append(choices, coredomain.Choice{ID: c[0], Text: c[1]})
	}
	return coredomain.PackQuestion{
		Type:             coredomain.SingleSelect,
		Prompt:           q.Prompt,
		Choices:          choices,
		CorrectChoiceIDs: q.Correct,
		Tags:             q.Tags,
	}
}

type fixturePair struct {
	Name      string          `json:"name"`
	Relation  string          `json:"relation"`
	Duplicate bool            `json:"duplicate"`
	A         fixtureQuestion `json:"a"`
	B         fixtureQuestion `json:"b"`
}

func loadFixturePairs(t *testing.T) []fixturePair {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "similarity_pairs.json"))
	if err != nil {
		t.Fatalf("read fixture set: %v", err)
	}
	var file struct {
		Pairs []fixturePair `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("parse fixture set: %v", err)
	}
	if len(file.Pairs) == 0 {
		t.Fatal("the fixture set is empty")
	}
	return file.Pairs
}

// scoreFixturePairs measures every pair under the lexical embedder.
func scoreFixturePairs(t *testing.T, pairs []fixturePair) []domain.ScoredPair {
	t.Helper()
	var e lexicalEmbedder
	out := make([]domain.ScoredPair, 0, len(pairs))
	for _, p := range pairs {
		a, b := p.A.packQuestion(), p.B.packQuestion()
		score, err := domain.Cosine(e.embed(domain.CanonicalText(&a)), e.embed(domain.CanonicalText(&b)))
		if err != nil {
			t.Fatalf("scoring %q: %v", p.Name, err)
		}
		out = append(out, domain.ScoredPair{Name: p.Name, Positive: p.Duplicate, Score: score})
	}
	return out
}

// The measured baseline for the lexical fixture embedder over the pairs it can
// in principle separate. Derived by the committed procedure, not chosen: the
// highest non-duplicate scores 0.7368, the margin is 0.02, and rounding up
// lands on 0.76.
//
// This is a self-consistency baseline for the procedure and the fixture set.
// It is NOT a production threshold and must never be used as one — see
// TestALexicalScorerCannotSeparateTheSemanticPairs for the reason, and
// domain.Calibrations() for the table that stays empty until a real embedding
// model has been measured.
const (
	fixtureNearDuplicateThreshold = 0.76
	fixtureRejectThreshold        = 0.99
)

func withoutSemanticPairs(scored []domain.ScoredPair, relations map[string]string) []domain.ScoredPair {
	var out []domain.ScoredPair
	for _, s := range scored {
		if relations[s.Name] != "semantic" {
			out = append(out, s)
		}
	}
	return out
}

func relationsOf(pairs []fixturePair) map[string]string {
	out := map[string]string{}
	for _, p := range pairs {
		out[p.Name] = p.Relation
	}
	return out
}

// The derivation is reproducible: same fixture set, same procedure, same
// number. A drifting fixture set or a changed procedure moves it, and this is
// what notices.
func TestTheFixtureCalibrationReproducesItsCommittedThreshold(t *testing.T) {
	pairs := loadFixturePairs(t)
	scored := withoutSemanticPairs(scoreFixturePairs(t, pairs), relationsOf(pairs))

	got, err := domain.Calibrate(scored, domain.V1CalibrationCriteria())
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if math.Abs(got.Threshold-fixtureNearDuplicateThreshold) > 1e-9 {
		t.Errorf("threshold: want the committed %.2f, got %.4f", fixtureNearDuplicateThreshold, got.Threshold)
	}
	if got.FalsePositives != 0 {
		t.Errorf("the criteria forbid false positives, got %d", got.FalsePositives)
	}
	if got.Recall < domain.V1CalibrationCriteria().MinRecall {
		t.Errorf("recall %.4f fell below the committed floor", got.Recall)
	}

	// And it stays a baseline. Seeding the production table with this number
	// would give a lexical-overlap score the authority of a measured semantic
	// threshold, which is the one thing this whole exercise must not do.
	if _, err := domain.CalibrationFor(lexicalEmbedder{}.identity()); !errors.Is(err, domain.ErrUncalibratedEmbeddingModel) {
		t.Errorf("the fixture embedder has a production calibration entry; the baseline leaked out of the fixtures: %v", err)
	}
}

// The two-sided guard. A threshold is only meaningful if moving it either way
// breaks something: down, and a non-duplicate is caught; up, and real
// duplicates are missed. A test asserting only "no false positives" would be
// satisfied by a threshold of 1.0 that catches nothing at all.
func TestMovingTheFixtureThresholdEitherWayBreaksIt(t *testing.T) {
	pairs := loadFixturePairs(t)
	scored := withoutSemanticPairs(scoreFixturePairs(t, pairs), relationsOf(pairs))

	var highestNegative, lowestCaughtPositive float64
	lowestCaughtPositive = math.Inf(1)
	for _, s := range scored {
		if s.Positive {
			if s.Score >= fixtureNearDuplicateThreshold && s.Score < lowestCaughtPositive {
				lowestCaughtPositive = s.Score
			}
			continue
		}
		if s.Score > highestNegative {
			highestNegative = s.Score
		}
	}

	// Down: the threshold must sit strictly above every non-duplicate, so
	// lowering it past the highest one starts producing false positives.
	if fixtureNearDuplicateThreshold <= highestNegative {
		t.Errorf("threshold %.2f is not above the highest non-duplicate %.4f, so it already admits a false positive",
			fixtureNearDuplicateThreshold, highestNegative)
	}
	// Up: the threshold must sit at or below the weakest duplicate it is
	// supposed to catch, so raising it starts producing false negatives.
	if fixtureNearDuplicateThreshold > lowestCaughtPositive {
		t.Errorf("threshold %.2f is above the weakest caught duplicate %.4f, so it misses a duplicate it should catch",
			fixtureNearDuplicateThreshold, lowestCaughtPositive)
	}

	belowHighestNegative := highestNegative - 0.01
	var falsePositives int
	for _, s := range scored {
		if !s.Positive && s.Score >= belowHighestNegative {
			falsePositives++
		}
	}
	if falsePositives == 0 {
		t.Error("lowering the threshold below the highest non-duplicate produced no false positive, so the fixture set has no hard negative and the calibration proves nothing")
	}
}

// The reject threshold separates "reworded" from "the same question", because
// above it no amount of rewording will help and the repair attempt would be
// spent to learn that.
func TestTheFixtureRejectThresholdSeparatesIdenticalFromParaphrased(t *testing.T) {
	pairs := loadFixturePairs(t)
	relations := relationsOf(pairs)
	scored := scoreFixturePairs(t, pairs)

	// Only verbatim-identical pairs are beyond repair; everything else,
	// including a close paraphrase, is something rewording could move.
	irreparable := make([]domain.ScoredPair, 0, len(scored))
	for _, s := range scored {
		s.Positive = relations[s.Name] == "identical"
		irreparable = append(irreparable, s)
	}

	got, err := domain.Calibrate(irreparable, domain.V1CalibrationCriteria())
	if err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if math.Abs(got.Threshold-fixtureRejectThreshold) > 1e-9 {
		t.Errorf("reject threshold: want the committed %.2f, got %.4f", fixtureRejectThreshold, got.Threshold)
	}
	if fixtureRejectThreshold <= fixtureNearDuplicateThreshold {
		t.Errorf("the reject threshold %.2f must sit above the near-duplicate threshold %.2f, or the ladder inverts",
			fixtureRejectThreshold, fixtureNearDuplicateThreshold)
	}
}

// The load-bearing negative result, and the reason the production calibration
// table ships empty.
//
// Two fixture pairs ask the same thing in different words. A lexical scorer
// does not merely rank them low — it ranks them BELOW unrelated-concept pairs
// it is supposed to let through, so no threshold exists that catches them
// without also catching the negatives. That is not a tuning problem, it is the
// difference between lexical overlap and semantic similarity, and it is why a
// number derived from this embedder must never be shipped as a production
// threshold.
//
// If someone later "fixes" this by relabeling the semantic pairs or dropping
// them from the fixture set, this test fails and says so.
func TestALexicalScorerCannotSeparateTheSemanticPairs(t *testing.T) {
	pairs := loadFixturePairs(t)
	relations := relationsOf(pairs)
	scored := scoreFixturePairs(t, pairs)

	var semantic, negatives []domain.ScoredPair
	for _, s := range scored {
		switch {
		case relations[s.Name] == "semantic":
			semantic = append(semantic, s)
		case !s.Positive:
			negatives = append(negatives, s)
		}
	}
	if len(semantic) == 0 {
		t.Fatal("the fixture set no longer contains a disjoint-vocabulary duplicate, so it no longer covers the case semantic embeddings exist for")
	}

	var highestNegative float64
	for _, n := range negatives {
		if n.Score > highestNegative {
			highestNegative = n.Score
		}
	}
	for _, s := range semantic {
		if s.Score >= highestNegative {
			t.Errorf("%q scores %.4f, at or above the highest non-duplicate %.4f — the lexical embedder separated a semantic pair, which would make this whole argument wrong",
				s.Name, s.Score, highestNegative)
		}
	}

	// And therefore the full set cannot be calibrated at all.
	if _, err := domain.Calibrate(scored, domain.V1CalibrationCriteria()); err == nil {
		t.Error("the full fixture set calibrated cleanly under a lexical scorer; if that is true the production table could be seeded from it, so this must be re-examined rather than ignored")
	}
}
