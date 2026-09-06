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

// Command judgecheck measures whether a chat model can classify the labeled
// similarity pairs that cosine similarity could not (A-25).
//
// A-22 and A-24 established that the blocker is the representation rather than
// the model: about half of the embedded string is answer options, so a
// legitimate non-duplicate sharing an option set is lexically near-identical
// while a real duplicate phrased in disjoint vocabulary shares almost nothing.
// A judge that sees both questions at once is not subject to that limit,
// because it never has to compress either one into a point first.
//
// It reuses the provider port rather than adding anything: the judgement is
// one structured Generate call against the chat model already configured, so
// there is no reranker model to pull, no embeddings endpoint, and no new
// dependency. Two orderings are run for every pair, because pairwise LLM
// judgements are known to move when the order moves.
//
// Usage:
//
//	FORGE_LIVE_ENDPOINT=<endpoint> go run ./cmd/judgecheck <chat-model>
//
// The endpoint comes from the environment and is never printed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// judgeTimeout bounds one judgement. B-2 measured ~50s for a full structured
// question on the reference host; a relation label is a far shorter output, so
// a call that exceeds this has stalled rather than slowed.
const judgeTimeout = 3 * time.Minute

// duplicateRelations are the labels that mean "the gate must catch this".
//
// The mapping is the fixture set's own and predates this run. The judge is
// asked only for the relation, never for the verdict, so that it cannot
// contradict itself and so the mapping stays ours rather than the model's.
var duplicateRelations = map[string]bool{
	"identical":  true,
	"paraphrase": true,
	"semantic":   true,
	"competency": false,
	"concept":    false,
	"unrelated":  false,
}

// judgeSystem defines the taxonomy in the fixture set's own words.
//
// It deliberately says nothing about which relations count as duplicates: the
// model classifies, the caller decides. Leaking the verdict mapping would let
// a model that has learned "say unrelated when unsure" score well for the
// wrong reason.
const judgeSystem = `You compare two multiple-choice questions from a study deck and classify their relationship.

Reply with exactly one label:

- "identical"  - the same assessment, the same wording (choice ids, choice order and formatting do not matter)
- "paraphrase" - the same assessment, reworded, with overlapping vocabulary
- "semantic"   - the same assessment, reworded with almost no shared vocabulary
- "competency" - the same concept, but a different competency is being tested
- "concept"    - the same topic, but a different concept is being asked about
- "unrelated"  - different topics

Judge what the question ASSESSES, not how much text the two share. Two questions
that share most of their answer options can still assess entirely different
facts. Two questions with no words in common can still assess exactly the same fact.`

const judgeSchema = `{
  "type": "object",
  "properties": {
    "relation": {
      "type": "string",
      "enum": ["identical", "paraphrase", "semantic", "competency", "concept", "unrelated"]
    }
  },
  "required": ["relation"]
}`

type judgeReply struct {
	Relation string `json:"relation"`
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

// render presents one question to the judge.
//
// It shows the same content the embedding path compares -- stem, options with
// correctness marked, tags -- so a difference in outcome is a difference in
// method rather than in what each method was allowed to see.
func render(q fixtureQuestion) string {
	pq := q.packQuestion()
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n", strings.TrimSpace(pq.Prompt))
	correct := map[string]bool{}
	for _, id := range pq.CorrectChoiceIDs {
		correct[strings.TrimSpace(id)] = true
	}
	for _, c := range pq.Choices {
		mark := " "
		if correct[strings.TrimSpace(c.ID)] {
			mark = "*"
		}
		fmt.Fprintf(&b, "  %s %s\n", mark, strings.TrimSpace(c.Text))
	}
	if len(pq.Tags) > 0 {
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(pq.Tags, ", "))
	}
	return b.String()
}

type fixturePair struct {
	Name      string          `json:"name"`
	Relation  string          `json:"relation"`
	Duplicate bool            `json:"duplicate"`
	A         fixtureQuestion `json:"a"`
	B         fixtureQuestion `json:"b"`
}

// verdict is one pair judged in both orderings.
type verdict struct {
	Name        string  `json:"name"`
	TrueLabel   string  `json:"true_relation"`
	TrueDup     bool    `json:"true_duplicate"`
	Forward     string  `json:"forward"`
	Reverse     string  `json:"reverse"`
	SecondsEach float64 `json:"seconds_each"`
}

// Consistent reports whether the ordering changed the verdict. The label may
// still differ while the duplicate/not decision agrees, which is the
// distinction that matters for the gate.
func (v verdict) Consistent() bool {
	return duplicateRelations[v.Forward] == duplicateRelations[v.Reverse]
}

// Duplicate is the judgement the gate would act on. A pair counts as caught
// only when BOTH orderings say duplicate: position bias is a known failure of
// pairwise LLM judging, and taking the optimistic reading of a disagreement
// would measure the bias away rather than measure it.
func (v verdict) Duplicate() bool {
	return duplicateRelations[v.Forward] && duplicateRelations[v.Reverse]
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "judgecheck: %v\n", err)
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
		return fmt.Errorf("usage: FORGE_LIVE_ENDPOINT=<endpoint> judgecheck <chat-model> [fixture-path]")
	}
	model := os.Args[1]
	path := filepath.Join("internal", "app", "testdata", "similarity_pairs.json")
	if len(os.Args) > 2 {
		path = os.Args[2]
	}

	pairs, err := loadPairs(path)
	if err != nil {
		return err
	}

	profile, _ := domain.ProfileByID(domain.ProfileOllama)
	zero := 0.0
	client := provider.NewOllama(profile, model, provider.WithEndpoint(endpoint))

	fmt.Printf("MODEL   %s\n", client.Identity())
	fmt.Printf("PAIRS   %d, judged in both orderings (%d calls)\n", len(pairs), 2*len(pairs))

	verdicts := make([]verdict, 0, len(pairs))
	var total time.Duration
	for _, p := range pairs {
		v := verdict{Name: p.Name, TrueLabel: p.Relation, TrueDup: p.Duplicate}

		start := time.Now()
		fwd, err := judge(client, render(p.A), render(p.B), &zero)
		if err != nil {
			return fmt.Errorf("judging %q forward: %w", p.Name, err)
		}
		rev, err := judge(client, render(p.B), render(p.A), &zero)
		if err != nil {
			return fmt.Errorf("judging %q reverse: %w", p.Name, err)
		}
		elapsed := time.Since(start)
		total += elapsed

		v.Forward, v.Reverse, v.SecondsEach = fwd, rev, elapsed.Seconds()/2
		verdicts = append(verdicts, v)
		fmt.Printf("  %-52s true=%-11s fwd=%-11s rev=%-11s %.1fs/call\n",
			truncate(p.Name, 52), p.Relation, fwd, rev, v.SecondsEach)
	}

	report(verdicts, total)

	blob, err := json.MarshalIndent(struct {
		Model    domain.ModelIdentity `json:"model"`
		Verdicts []verdict            `json:"verdicts"`
	}{client.Identity(), verdicts}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("\n--- verdict table ---\n%s\n", blob)
	return nil
}

// judge asks for one relation label. Temperature is pinned to zero so that a
// rerun measures the model rather than its sampler.
func judge(c *provider.Ollama, first, second string, temp *float64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer cancel()

	var reply judgeReply
	err := c.Generate(ctx, ports.Request{
		System:      judgeSystem,
		User:        fmt.Sprintf("First question:\n%s\nSecond question:\n%s\nClassify the relationship.", first, second),
		Schema:      []byte(judgeSchema),
		Temperature: temp,
	}, &reply)
	if err != nil {
		return "", err
	}
	if _, ok := duplicateRelations[reply.Relation]; !ok {
		return "", fmt.Errorf("model returned relation %q, which is not in the taxonomy", reply.Relation)
	}
	return reply.Relation, nil
}

// report applies the pre-registered criteria. It prints whatever it finds,
// including a failure: a run that fails is a result, not a reason to try a
// different rule.
func report(verdicts []verdict, total time.Duration) {
	var positives, negatives, falsePositives, caught, exact, inconsistent int
	for _, v := range verdicts {
		if !v.Consistent() {
			inconsistent++
		}
		if v.Forward == v.TrueLabel {
			exact++
		}
		if v.TrueDup {
			positives++
			if v.Duplicate() {
				caught++
			}
			continue
		}
		negatives++
		if v.Duplicate() {
			falsePositives++
		}
	}
	recall := float64(caught) / float64(positives)
	consistency := float64(len(verdicts)-inconsistent) / float64(len(verdicts))

	fmt.Printf("\nRESULT\n")
	fmt.Printf("  false positives    %d (criterion: 0)\n", falsePositives)
	fmt.Printf("  recall             %.4f (%d/%d) (criterion: >= 0.80)\n", recall, caught, positives)
	fmt.Printf("  position consistency %.4f (%d/%d agreed under order swap) (criterion: >= 0.90)\n",
		consistency, len(verdicts)-inconsistent, len(verdicts))
	fmt.Printf("  exact 6-way label  %.4f (%d/%d) (reported, not a criterion)\n",
		float64(exact)/float64(len(verdicts)), exact, len(verdicts))
	fmt.Printf("  latency            %.1fs total, %.1fs per call\n",
		total.Seconds(), total.Seconds()/float64(2*len(verdicts)))

	pass := falsePositives == 0 && recall >= 0.80 && consistency >= 0.90
	if pass {
		fmt.Printf("  VERDICT: PASSES the pre-registered criteria\n")
		return
	}
	fmt.Printf("  VERDICT: FAILS the pre-registered criteria\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
	sort.SliceStable(file.Pairs, func(i, j int) bool { return file.Pairs[i].Name < file.Pairs[j].Name })
	return file.Pairs, nil
}
