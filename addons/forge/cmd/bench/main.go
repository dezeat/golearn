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

// bench is the dev-lane measurement runner (#139): it drives one model on one
// provider through the shipped adapter and emits a bench.Record as JSONL.
// It is env-gated (a provider credential or a local endpoint), never part of
// a release build, and spends real money on hosted providers — the request
// count is capped and paced for exactly that reason.
//
// Usage: bench <profile> <model> [n] [base|minmax] [jsonl-path] [judge-profile judge-model] [prompt-variant]
//
// The judge probes (guessability, verify, critique) run on a separate judge
// client when one is given. Judging with the generator model is the
// self-preference bias the LLM-as-judge literature warns about; the runner
// permits it (small local setups may have one model) but says so in the
// record instead of hiding it.
package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/bench"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

type quiz struct {
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
	Correct int      `json:"correct_index"`
}

// Both schemas satisfy strict structured-output rules; minmax additionally
// moves the four-choice contract from the prompt into the schema, which the
// #141 improvement loop measured as eliminating count violations entirely.
const schemaBase = `{"type":"object","additionalProperties":false,"properties":{"prompt":{"type":"string"},"choices":{"type":"array","items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`
const schemaMinMax = `{"type":"object","additionalProperties":false,"properties":{"prompt":{"type":"string"},"choices":{"type":"array","minItems":4,"maxItems":4,"items":{"type":"string"}},"correct_index":{"type":"integer"}},"required":["prompt","choices","correct_index"]}`
const guessSchema = `{"type":"object","additionalProperties":false,"properties":{"choice_index":{"type":"integer"}},"required":["choice_index"]}`
const verifySchema = `{"type":"object","additionalProperties":false,"properties":{"choice_index":{"type":"integer"},"reasoning":{"type":"string"}},"required":["choice_index","reasoning"]}`
const critiqueProbeSchema = `{"type":"object","additionalProperties":false,"properties":{"distractors_plausible":{"type":"boolean"},"single_defensible_answer":{"type":"boolean"},"problem":{"type":"string"}},"required":["distractors_plausible","single_defensible_answer","problem"]}`

// critiqueSystem mirrors the pipeline's ungrounded critique stage so the
// bench aggregate measures the same judgment the product applies.
const critiqueSystem = `You are reviewing one multiple-choice question.

Judge only these, strictly:
- distractors_plausible: the wrong choices are plausible to someone who has not learned the material, and are definitely wrong.
- single_defensible_answer: exactly the marked choices are defensible; no unmarked choice is also arguably correct.

If either is false, state the single most serious problem in one sentence. If both are true, leave problem empty.
Reply with JSON only.`

const systemMsg = "You write multiple-choice questions. Reply with JSON only."

// systemMsgParallel is the anti-leak variant (#141 H2): the campaign measured
// that every model's options leak the answer — the correct answer is written
// carefully while distractors are short, absolute, or absurd. The variant
// names the craft rules the item-writing literature prescribes.
const systemMsgParallel = `You write multiple-choice questions. Reply with JSON only.

Distractor rules, all mandatory:
- Every wrong choice must match the correct answer in length, specificity and tone.
- Every wrong choice must be a plausible misconception a real learner holds, never an absurdity.
- Do not use absolute qualifiers (always, never, automatically) in a wrong choice unless the correct answer also carries one.
- A reader must not be able to pick the correct answer by style alone.`

const userMsg = "Write one multiple-choice question about Go goroutines with exactly 4 choices."

// The fictional domain exists because guessability is only interpretable
// where the judge has no parametric knowledge (#141 H2 refutation): on an
// invented specification, options-only success above chance can come only
// from style or elimination cues. The verify probe receives the spec; the
// guess probe never does.
const fictionalSpec = `The Zorvian Reckoning is a fictional calendar. Its rules, complete and exhaustive:
- A year has 14 months. Odd months have 23 days, even months have 24, except month 7 (Quillmarch), which has 30.
- A week has 6 days; the third day is called Restmark.
- Every 5th year is a Veil Year: one extra day, Veilday, is appended to month 14 and belongs to no week.
- The year begins at dawn on the 1st of month 1 (Firstreach).
- Public rest days are every Restmark and the last day of every even month.`

const userMsgFictional = "Using ONLY the following fictional specification, write one multiple-choice question with exactly 4 choices. Do not use any outside knowledge.\n\n" + fictionalSpec

// pace keeps the runner under entry-tier RPM caps (the #141 lane measured 10
// requests/min on a fresh OpenAI account); bursting past the cap turns the
// run into a measurement of admission control. BENCH_PACE_SECONDS overrides
// it for providers with roomier limits — inter-call idle is not a measured
// quantity, so the override cannot bias a metric.
var pace = 7 * time.Second

// workers is per-lane call concurrency (BENCH_CONCURRENCY, default 1 — the
// sequential behaviour). Pacing applies per worker, so effective request rate
// is workers/pace: size both to the provider's RPM budget, because past it
// the run measures admission control instead of the model.
var workers = 1

func init() {
	if v := os.Getenv("BENCH_PACE_SECONDS"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			pace = time.Duration(secs) * time.Second
		}
	}
	if v := os.Getenv("BENCH_CONCURRENCY"); v != "" {
		if c, err := strconv.Atoi(v); err == nil && c >= 1 {
			workers = c
		}
	}
}

// runPool executes tasks with the configured worker count, each worker
// sleeping one pace between its tasks.
func runPool(tasks []func(), workers int) {
	var wg sync.WaitGroup
	queue := make(chan func())
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range queue {
				task()
				time.Sleep(pace)
			}
		}()
	}
	for _, t := range tasks {
		queue <- t
	}
	close(queue)
	wg.Wait()
}

const perCallDeadline = 3 * time.Minute

// classify maps an adapter error onto the tally taxonomy. Quota exhaustion is
// checked before generic 429 because providers ship both under the same
// status code, and only one of them is retryable.
func classify(err error) bench.Class {
	if err == nil {
		return bench.ClassExecuted
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "credit_balance_exhausted") || strings.Contains(msg, "insufficient_quota"):
		return bench.ClassQuota
	case strings.Contains(msg, "HTTP 429"):
		return bench.ClassThrottled
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "context deadline exceeded"):
		return bench.ClassTimeout
	default:
		return bench.ClassExecuted
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: bench <profile> <model> [n] [base|minmax] [jsonl-path]")
		os.Exit(2)
	}
	profileID, model := os.Args[1], os.Args[2]
	n := 20
	variant := "base"
	jsonlPath := ""
	if len(os.Args) > 3 {
		if v, err := strconv.Atoi(os.Args[3]); err == nil {
			n = v
		}
	}
	if len(os.Args) > 4 {
		variant = os.Args[4]
	}
	if len(os.Args) > 5 {
		jsonlPath = os.Args[5]
	}
	judgeProfileID, judgeModel := "", ""
	if len(os.Args) > 7 {
		judgeProfileID, judgeModel = os.Args[6], os.Args[7]
	}
	promptVariant := "base"
	if len(os.Args) > 8 {
		promptVariant = os.Args[8]
	}
	system := systemMsg
	if promptVariant == "parallel" {
		system = systemMsgParallel
	}
	topicVariant := os.Getenv("BENCH_TOPIC")
	if topicVariant == "" {
		topicVariant = "go"
	}
	user := userMsg
	verifyContext := ""
	if topicVariant == "fictional" {
		user = userMsgFictional
		verifyContext = fictionalSpec + "\n\n"
	}
	schema := schemaBase
	if variant == "minmax" {
		schema = schemaMinMax
	}

	profile, err := domain.ProfileByID(domain.ProfileID(profileID))
	if err != nil {
		fmt.Fprintln(os.Stderr, "unknown profile:", err)
		os.Exit(2)
	}
	key := os.Getenv(profile.CredentialEnvVar)
	if profile.NeedsCredential() && key == "" {
		fmt.Fprintf(os.Stderr, "%s unset; aborting\n", profile.CredentialEnvVar)
		os.Exit(2)
	}

	var client ports.Provider
	if profile.NeedsCredential() {
		client = provider.NewOpenAICompatible(profile, model, domain.NewSecret(key, domain.OriginEnvironment))
	} else {
		client = provider.NewOllama(profile, model)
	}

	judge := client
	judgeNote := "judge is the generator model — self-preference bias applies"
	if judgeProfileID != "" {
		jp, err := domain.ProfileByID(domain.ProfileID(judgeProfileID))
		if err != nil {
			fmt.Fprintln(os.Stderr, "unknown judge profile:", err)
			os.Exit(2)
		}
		jkey := os.Getenv(jp.CredentialEnvVar)
		if jp.NeedsCredential() && jkey == "" {
			fmt.Fprintf(os.Stderr, "%s unset; aborting\n", jp.CredentialEnvVar)
			os.Exit(2)
		}
		if jp.NeedsCredential() {
			judge = provider.NewOpenAICompatible(jp, judgeModel, domain.NewSecret(jkey, domain.OriginEnvironment))
		} else {
			judge = provider.NewOllama(jp, judgeModel)
		}
		judgeNote = ""
	}

	promptHash := sha256.Sum256([]byte(system + "\x00" + user + "\x00" + schema))
	rec := bench.Record{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Provider:      profileID,
		Model:         model,
		JudgeProvider: judgeProfileID,
		JudgeModel:    judgeModel,
		Notes:         judgeNote,
		PromptHash:    fmt.Sprintf("%x", promptHash[:8]),
		PromptVariant: promptVariant,
		SchemaVariant: variant,
		TopicVariant:  topicVariant,
		Sampling:      "provider-default",
		Metrics:       map[string]bench.Proportion{},
		Verdicts:      map[string]bench.Verdict{},
	}

	parseOK, contractOK := 0, 0
	var latencySum float64
	var questions []quiz
	var mu sync.Mutex

	generateOne := func(label int) {
		ctx, cancel := context.WithTimeout(context.Background(), perCallDeadline)
		start := time.Now()
		var q quiz
		gerr := client.Generate(ctx, ports.Request{System: system, User: user, Schema: []byte(schema)}, &q)
		cancel()
		elapsed := time.Since(start).Seconds()
		class := classify(gerr)
		mu.Lock()
		defer mu.Unlock()
		rec.Tally.Add(class)
		if class != bench.ClassExecuted {
			fmt.Printf("GEN %02d %s (excluded) err=%v\n", label, class, gerr)
			return
		}
		latencySum += elapsed
		pv := gerr == nil
		cv := pv && q.Prompt != "" && len(q.Choices) == 4 && q.Correct >= 0 && q.Correct < 4
		if pv {
			parseOK++
		}
		if cv {
			contractOK++
			questions = append(questions, q)
		}
		fmt.Printf("GEN %02d parse=%v contract=%v latency=%.2fs choices=%d err=%v\n",
			label, pv, cv, elapsed, len(q.Choices), gerr)
	}

	// Two batches rather than a retry loop: the first asks for n, the second
	// tops up whatever admission control ate, and the 2n attempt cap from the
	// sequential runner still holds.
	attempts := 0
	for batch := 0; batch < 2 && rec.Tally.Executed() < n && attempts < 2*n; batch++ {
		want := n - rec.Tally.Executed()
		if attempts+want > 2*n {
			want = 2*n - attempts
		}
		tasks := make([]func(), 0, want)
		for i := 0; i < want; i++ {
			label := attempts + i + 1
			tasks = append(tasks, func() { generateOne(label) })
		}
		runPool(tasks, workers)
		attempts += want
	}
	executed := rec.Tally.Executed()
	if executed > 0 {
		rec.MeanLatencyS = latencySum / float64(executed)
	}
	rec.Metrics["parse_valid"] = bench.NewProportion(parseOK, executed)
	rec.Metrics["contract_valid"] = bench.NewProportion(contractOK, executed)
	rec.Verdicts["parse_valid"] = rec.Metrics["parse_valid"].Judge(0.80, 0.90)
	rec.Verdicts["contract_valid"] = rec.Metrics["contract_valid"].Judge(0.80, 0.90)

	// Semantic probes, run by the judge client on up to eight contract-valid
	// questions. Choice order is shuffled per probe with a seeded permutation
	// so position bias cannot masquerade as a verdict, and the shuffle is
	// reproducible for a given question index.
	//
	//   guessability — answer from the choices alone, no stem: an interval
	//   above chance (0.25) flags options that leak the answer.
	//   verify_pass — answer-blind re-answering of the full question, the
	//   bench aggregate of the pipeline's verify stage.
	//   critique_pass — the ungrounded critique booleans; pass means both
	//   distractors_plausible and single_defensible_answer hold.
	guessed, gN := 0, 0
	verified, vN := 0, 0
	critiqued, cN := 0, 0
	limit := min(len(questions), 8)
	probe := func(system, user, schema string, out any) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		return judge.Generate(ctx, ports.Request{System: system, User: user, Schema: []byte(schema)}, out) == nil
	}
	var probeTasks []func()
	for qi, q := range questions[:limit] {
		seed := int64(qi) + 1
		perm := bench.Permutation(len(q.Choices), seed)
		shuffledCorrect := -1
		var choicesList strings.Builder
		for pos, orig := range perm {
			fmt.Fprintf(&choicesList, "%d: %s\n", pos, q.Choices[orig])
			if orig == q.Correct {
				shuffledCorrect = pos
			}
		}
		choices, correct, prompt := choicesList.String(), shuffledCorrect, q.Prompt
		rec.Questions = append(rec.Questions, bench.StoredQuestion{
			Prompt: q.Prompt, Choices: q.Choices, Correct: q.Correct, ShuffleSeed: seed,
		})

		probeTasks = append(probeTasks, func() {
			var g struct {
				ChoiceIndex int `json:"choice_index"`
			}
			ok := probe("You are given only the answer choices of a hidden multiple-choice question. Pick the index most likely to be the correct answer.",
				choices, guessSchema, &g)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				gN++
				if g.ChoiceIndex == correct {
					guessed++
				}
			}
		}, func() {
			var v struct {
				ChoiceIndex int    `json:"choice_index"`
				Reasoning   string `json:"reasoning"`
			}
			ok := probe("You are answering a multiple-choice question. Choose the correct answer. Reply with the choice index and one sentence of reasoning, as JSON only.",
				verifyContext+prompt+"\n\n"+choices, verifySchema, &v)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				vN++
				if v.ChoiceIndex == correct {
					verified++
				}
			}
		}, func() {
			var cr struct {
				DistractorsPlausible   bool   `json:"distractors_plausible"`
				SingleDefensibleAnswer bool   `json:"single_defensible_answer"`
				Problem                string `json:"problem"`
			}
			ok := probe(critiqueSystem, prompt+"\n\n"+choices, critiqueProbeSchema, &cr)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				cN++
				if cr.DistractorsPlausible && cr.SingleDefensibleAnswer {
					critiqued++
				}
			}
		})
	}
	runPool(probeTasks, workers)
	rec.Metrics["guessability"] = bench.NewProportion(guessed, gN)
	rec.Metrics["verify_pass"] = bench.NewProportion(verified, vN)
	rec.Metrics["critique_pass"] = bench.NewProportion(critiqued, cN)
	rec.Verdicts["verify_pass"] = rec.Metrics["verify_pass"].Judge(0.80, 0.90)

	fmt.Printf("\nPARSE    %d/%d [%.3f, %.3f] %s\n", parseOK, executed,
		rec.Metrics["parse_valid"].Lo, rec.Metrics["parse_valid"].Hi, rec.Verdicts["parse_valid"])
	fmt.Printf("CONTRACT %d/%d [%.3f, %.3f] %s\n", contractOK, executed,
		rec.Metrics["contract_valid"].Lo, rec.Metrics["contract_valid"].Hi, rec.Verdicts["contract_valid"])
	fmt.Printf("GUESS    %d/%d [%.3f, %.3f] chance=0.25\n", guessed, gN,
		rec.Metrics["guessability"].Lo, rec.Metrics["guessability"].Hi)
	fmt.Printf("VERIFY   %d/%d [%.3f, %.3f] %s\n", verified, vN,
		rec.Metrics["verify_pass"].Lo, rec.Metrics["verify_pass"].Hi, rec.Verdicts["verify_pass"])
	fmt.Printf("CRITIQUE %d/%d [%.3f, %.3f]\n", critiqued, cN,
		rec.Metrics["critique_pass"].Lo, rec.Metrics["critique_pass"].Hi)
	fmt.Printf("TALLY    %v mean_latency=%.2fs\n", rec.Tally.Counts, rec.MeanLatencyS)

	line, err := rec.JSONL()
	if err != nil {
		fmt.Fprintln(os.Stderr, "record:", err)
		os.Exit(1)
	}
	if jsonlPath != "" {
		f, err := os.OpenFile(jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "record:", err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintln(f, string(line))
	} else {
		fmt.Println(string(line))
	}
}
