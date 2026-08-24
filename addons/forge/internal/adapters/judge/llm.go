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

// Package judge decides how two questions relate, using the configured
// provider (D-023).
//
// It exists because embedding cosine cannot make this decision: about half of
// the compared text is answer options, so a legitimate non-duplicate sharing
// an option set is lexically near-identical while a real duplicate phrased in
// different words shares almost nothing. Two embedding models failed on the
// same pair for that reason (FORGE-EXPERIMENTS A-22, A-24); a model that reads
// both questions at once passed the same criteria (A-25).
package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// judgeTemperature pins sampling off.
//
// The gate is a repeatable decision, not a generation: the same pair judged
// twice should answer the same way. This does not make the judge fully
// deterministic across model versions, which is why that promise is stated
// rather than assumed (D-023).
var judgeTemperature = 0.0

// systemPrompt defines the taxonomy in the fixture set's own words.
//
// It deliberately never says which relations count as duplicates. The mapping
// is the project's ([domain.Relation.IsDuplicate]); a model told the mapping
// could score well by guessing the verdict instead of classifying, and the
// measurement would stop meaning anything.
//
// The closing paragraph is not decoration. It names the exact confusion that
// defeated both embedding models, because that is the discrimination being
// bought here.
var systemPrompt = buildSystemPrompt()

func buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You compare two multiple-choice questions from a study deck " +
		"and classify their relationship.\n\nReply with exactly one label:\n\n")
	for _, r := range domain.Relations() {
		fmt.Fprintf(&b, "- %q - %s\n", string(r), relationGloss[r])
	}
	b.WriteString("\nJudge what the question ASSESSES, not how much text the two share. " +
		"Two questions that share most of their answer options can still assess entirely " +
		"different facts. Two questions with no words in common can still assess exactly " +
		"the same fact.")
	return b.String()
}

var relationGloss = map[domain.Relation]string{
	domain.RelationIdentical:  "the same assessment, the same wording (choice ids, choice order and formatting do not matter)",
	domain.RelationParaphrase: "the same assessment, reworded, with overlapping vocabulary",
	domain.RelationSemantic:   "the same assessment, reworded with almost no shared vocabulary",
	domain.RelationCompetency: "the same concept, but a different competency is being tested",
	domain.RelationConcept:    "the same topic, but a different concept is being asked about",
	domain.RelationUnrelated:  "different topics",
}

// schema constrains decoding server-side rather than asking the model to
// behave, the same choice the generation path makes.
var schema = buildSchema()

func buildSchema() []byte {
	labels := make([]string, 0, len(domain.Relations()))
	for _, r := range domain.Relations() {
		labels = append(labels, string(r))
	}
	enum, _ := json.Marshal(labels)
	return []byte(fmt.Sprintf(
		`{"type":"object","properties":{"relation":{"type":"string","enum":%s}},"required":["relation"]}`,
		enum))
}

type reply struct {
	Relation string `json:"relation"`
}

// LLM judges with a chat provider.
type LLM struct {
	provider ports.Provider
}

var _ ports.DuplicateJudge = (*LLM)(nil)

// New wires a judge onto an already-configured provider.
//
// It takes the provider rather than building one: the judge deliberately has
// no credentials, no endpoint and no profile knowledge of its own, so there is
// exactly one place in the binary where a provider is constructed.
func New(p ports.Provider) *LLM { return &LLM{provider: p} }

// JudgeIdentity names the deciding model.
func (l *LLM) JudgeIdentity() coredomain.ModelIdentity { return l.provider.Identity() }

// Judge classifies the pair, and confirms a duplicate verdict in the reverse
// order before returning it.
//
// Pairwise LLM judgements are known to move when the ordering moves. A-25
// measured 13 of 13 orderings agreeing on this fixture set, so this is
// insurance rather than a fix for an observed defect — but it is cheap
// insurance, because it costs a second call only when the first call says
// "duplicate", and most pairs are not duplicates.
//
// The asymmetry is deliberate and matches FORGE.md 7's preference: a false
// positive discards a good question and spends repair budget, so a duplicate
// verdict has to survive being asked twice, while a permit does not.
func (l *LLM) Judge(ctx context.Context, a, b coredomain.PackQuestion) (domain.Relation, error) {
	forward, err := l.ask(ctx, a, b)
	if err != nil {
		return "", err
	}
	if !forward.IsDuplicate() {
		return forward, nil
	}

	reverse, err := l.ask(ctx, b, a)
	if err != nil {
		return "", err
	}
	if !reverse.IsDuplicate() {
		// The orderings disagree on the verdict. Resolve toward permitting,
		// because that is the direction that does not destroy a candidate on a
		// judgement the model would not repeat.
		return reverse, nil
	}
	return forward, nil
}

func (l *LLM) ask(ctx context.Context, first, second coredomain.PackQuestion) (domain.Relation, error) {
	var out reply
	err := l.provider.Generate(ctx, ports.Request{
		System: systemPrompt,
		User: fmt.Sprintf("First question:\n%s\nSecond question:\n%s\nClassify the relationship.",
			render(first), render(second)),
		Schema:      schema,
		Temperature: &judgeTemperature,
	}, &out)
	if err != nil {
		return "", fmt.Errorf("judge: %w", err)
	}
	relation, err := domain.ParseRelation(out.Relation)
	if err != nil {
		return "", fmt.Errorf("judge: %w", err)
	}
	return relation, nil
}

// render presents a question to the judge.
//
// It shows what the embedding path compares — stem, options with the correct
// one marked, tags — so that a difference in outcome between the two stages is
// a difference in method rather than in what each was allowed to see.
// Explanations are excluded, as #124 requires.
func render(q coredomain.PackQuestion) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n", coredomain.NormalizeText(q.Prompt))
	correct := make(map[string]bool, len(q.CorrectChoiceIDs))
	for _, id := range q.CorrectChoiceIDs {
		correct[strings.TrimSpace(id)] = true
	}
	for _, c := range q.Choices {
		mark := " "
		if correct[strings.TrimSpace(c.ID)] {
			mark = "*"
		}
		fmt.Fprintf(&b, "  %s %s\n", mark, coredomain.NormalizeText(c.Text))
	}
	if len(q.Tags) > 0 {
		tags := make([]string, 0, len(q.Tags))
		for _, t := range q.Tags {
			tags = append(tags, strings.ToLower(strings.TrimSpace(t)))
		}
		fmt.Fprintf(&b, "Tags: %s\n", strings.Join(tags, ", "))
	}
	return b.String()
}
