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

// Command calibrate derives a similarity threshold for one real embedding
// model from the labeled fixture set, using the committed procedure.
//
// It exists because D-022 refuses an uncalibrated model and the only way to
// lift that refusal is a measurement against a real embedding model, which by
// definition cannot happen inside `make check` (D-015: the gate stays
// offline). So the derivation lives here, gated on an endpoint supplied at
// runtime, and prints the score table an entry in domain's calibration table
// must be reproducible from.
//
// It decides nothing. The selection rule is [domain.Calibrate] with
// [domain.V1CalibrationCriteria], both committed before any number this
// program produced existed; this is only the apparatus that feeds them real
// scores instead of a lexical stand-in's.
//
// Usage:
//
//	FORGE_LIVE_ENDPOINT=<endpoint> go run ./addons/forge/cmd/calibrate <embedding-model>
//
// The endpoint comes from the environment and is never printed: a deployment
// address is operator information (FORGE.md 6.1), and this program's output is
// meant to be pasted into a public evidence log.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// fixturePath is the labeled set the gate is calibrated against. It is the
// same file the offline fixture baseline uses, read rather than copied, so a
// drifting fixture set cannot leave a stale calibration looking valid.
var fixturePath = filepath.Join("addons", "forge", "internal", "app", "testdata", "similarity_pairs.json")

// embedTimeout bounds the whole run. Embedding is cheap next to generation
// even on a CPU-only host, so a run that exceeds this has stalled rather than
// slowed.
const embedTimeout = 10 * time.Minute

type fixtureQuestion struct {
	Prompt  string     `json:"prompt"`
	Choices [][]string `json:"choices"`
	Correct []string   `json:"correct"`
	Tags    []string   `json:"tags"`
}

// packQuestion projects a fixture question onto the core type, so the text
// that gets embedded here is produced by exactly the function the gate calls.
// Reimplementing the projection would measure a different string than the one
// production compares.
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

// scoreRecord is one measured pair, in the shape the offline regression
// fixture stores. Scores only: the raw model output is not recorded anywhere,
// and the vectors are not either.
type scoreRecord struct {
	Name      string  `json:"name"`
	Relation  string  `json:"relation"`
	Duplicate bool    `json:"duplicate"`
	Score     float64 `json:"score"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "calibrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	endpoint := os.Getenv("FORGE_LIVE_ENDPOINT")
	if endpoint == "" {
		fmt.Println("FORGE_LIVE_ENDPOINT unset; skipping (this program measures a live model and never runs in the gate)")
		return nil
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: FORGE_LIVE_ENDPOINT=<endpoint> calibrate <embedding-model> [fixture-path]")
	}
	model := os.Args[1]
	if len(os.Args) > 2 {
		fixturePath = os.Args[2]
	}

	pairs, err := loadPairs(fixturePath)
	if err != nil {
		return err
	}

	profile, _ := domain.ProfileByID(domain.ProfileOllama)
	client := provider.NewOllama(profile, "", provider.WithEndpoint(endpoint),
		provider.WithEmbeddingModel(model))

	ctx, cancel := context.WithTimeout(context.Background(), embedTimeout)
	defer cancel()

	// Every text in one batch, in a fixed order, so the run is one request
	// and the pairing is positional exactly as the gate's is.
	texts := make([]string, 0, len(pairs)*2)
	for _, p := range pairs {
		a, b := p.A.packQuestion(), p.B.packQuestion()
		texts = append(texts, domain.CanonicalText(&a), domain.CanonicalText(&b))
	}

	start := time.Now()
	vectors, err := client.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding the fixture set: %w", err)
	}
	elapsed := time.Since(start)
	if len(vectors) != len(texts) {
		return fmt.Errorf("asked for %d vectors, received %d", len(texts), len(vectors))
	}

	identity := client.EmbeddingIdentity()
	fmt.Printf("MODEL   %s\n", identity)
	fmt.Printf("DIM     %d\n", vectors[0].Dim())
	fmt.Printf("TEXTS   %d in one batch, %.2fs\n", len(texts), elapsed.Seconds())

	// Every vector must share one width, or the scores below compare
	// geometries rather than questions.
	for i, v := range vectors {
		if v.Dim() != vectors[0].Dim() {
			return fmt.Errorf("vector %d has dim %d, first has %d", i, v.Dim(), vectors[0].Dim())
		}
	}

	if err := probeDeterminism(ctx, client, texts[0]); err != nil {
		return err
	}
	if err := probeBatching(ctx, client, texts[:2], vectors[:2]); err != nil {
		return err
	}

	records := make([]scoreRecord, 0, len(pairs))
	scored := make([]domain.ScoredPair, 0, len(pairs))
	for i, p := range pairs {
		score, err := domain.Cosine(vectors[2*i], vectors[2*i+1])
		if err != nil {
			return fmt.Errorf("scoring %q: %w", p.Name, err)
		}
		records = append(records, scoreRecord{
			Name: p.Name, Relation: p.Relation, Duplicate: p.Duplicate, Score: score,
		})
		scored = append(scored, domain.ScoredPair{Name: p.Name, Positive: p.Duplicate, Score: score})
	}

	report(records)
	derive("NEAR-DUPLICATE (full set)", scored)
	derive("REJECT (identical-only positives)", asIdenticalOnly(records))

	// The canonical texts are published as digests rather than content: an
	// independent re-implementation can prove it embedded the same strings
	// without this log carrying the fixture text a second time.
	digests := make([]string, len(texts))
	for i, t := range texts {
		sum := sha256.Sum256([]byte(t))
		digests[i] = hex.EncodeToString(sum[:])
	}

	blob, err := json.MarshalIndent(struct {
		Model    domain.ModelIdentity `json:"model"`
		Dim      int                  `json:"dim"`
		TextSHA  []string             `json:"canonical_text_sha256"`
		Measured []scoreRecord        `json:"pairs"`
	}{identity, vectors[0].Dim(), digests, records}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("\n--- score table ---\n%s\n", blob)
	return nil
}

// probeDeterminism embeds one text twice and reports whether the model returns
// the identical vector.
//
// It is not a formality. A regression fixture pinning a score to four decimals
// is only meaningful if the model is deterministic, and a threshold sitting a
// hundredth above the highest negative is only robust if that negative does
// not move between runs.
func probeDeterminism(ctx context.Context, c *provider.Ollama, text string) error {
	first, err := c.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("determinism probe: %w", err)
	}
	second, err := c.Embed(ctx, []string{text})
	if err != nil {
		return fmt.Errorf("determinism probe: %w", err)
	}
	self, err := domain.Cosine(first[0], second[0])
	if err != nil {
		return fmt.Errorf("determinism probe: %w", err)
	}
	identical := true
	for i := range first[0] {
		if first[0][i] != second[0][i] {
			identical = false
			break
		}
	}
	fmt.Printf("DETERM  bit_identical=%v self_cosine=%.10f\n", identical, self)
	return nil
}

// probeBatching re-embeds two texts one at a time and compares against the
// batched vectors.
//
// The gate takes both paths — embedCandidates batches, embedOne does not — so
// a model whose output depends on batch composition would score the same
// candidate differently depending on which path reached it.
func probeBatching(ctx context.Context, c *provider.Ollama, texts []string, batched []domain.Vector) error {
	worst := 1.0
	for i, t := range texts {
		single, err := c.Embed(ctx, []string{t})
		if err != nil {
			return fmt.Errorf("batching probe: %w", err)
		}
		agreement, err := domain.Cosine(single[0], batched[i])
		if err != nil {
			return fmt.Errorf("batching probe: %w", err)
		}
		if agreement < worst {
			worst = agreement
		}
	}
	fmt.Printf("BATCH   single_vs_batched_worst_cosine=%.10f\n", worst)
	return nil
}

// asIdenticalOnly relabels the set so only verbatim-identical pairs count as
// positives, which is what the reject threshold separates: above it, rewording
// cannot help, so the repair budget is not spent to learn that.
func asIdenticalOnly(records []scoreRecord) []domain.ScoredPair {
	out := make([]domain.ScoredPair, 0, len(records))
	for _, r := range records {
		out = append(out, domain.ScoredPair{
			Name: r.Name, Positive: r.Relation == "identical", Score: r.Score,
		})
	}
	return out
}

func report(records []scoreRecord) {
	sorted := make([]scoreRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	fmt.Printf("\n%-52s %-11s %-4s %s\n", "PAIR", "RELATION", "DUP", "SCORE")
	for _, r := range sorted {
		fmt.Printf("%-52s %-11s %-4v %.4f\n", r.Name, r.Relation, r.Duplicate, r.Score)
	}
}

// derive runs the committed selection rule and prints whatever it says,
// including a failure. A run that fails the criteria is a result, not a reason
// to try a different rule.
func derive(label string, scored []domain.ScoredPair) {
	got, err := domain.Calibrate(scored, domain.V1CalibrationCriteria())
	fmt.Printf("\n%s\n", label)
	fmt.Printf("  threshold=%.4f fp=%d fn=%d recall=%.4f margin=%.4f positives=%d negatives=%d\n",
		got.Threshold, got.FalsePositives, got.FalseNegatives, got.Recall, got.Margin,
		got.Positives, got.Negatives)
	if err != nil {
		fmt.Printf("  FAILS THE COMMITTED CRITERIA: %v\n", err)
		return
	}
	fmt.Printf("  PASSES the committed criteria (fp<=%d, recall>=%.2f, margin>=%.2f)\n",
		domain.V1CalibrationCriteria().MaxFalsePositives,
		domain.V1CalibrationCriteria().MinRecall,
		domain.V1CalibrationCriteria().MinMargin)
}

func loadPairs(path string) ([]fixturePair, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture set: %w", err)
	}
	var file struct {
		Pairs []fixturePair `json:"pairs"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("parse fixture set: %w", err)
	}
	if len(file.Pairs) == 0 {
		return nil, fmt.Errorf("the fixture set is empty")
	}
	return file.Pairs, nil
}
