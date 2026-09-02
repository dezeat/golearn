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
// Usage: bench <profile> <model> [n] [base|minmax] [jsonl-path]
package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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

const systemMsg = "You write multiple-choice questions. Reply with JSON only."
const userMsg = "Write one multiple-choice question about Go goroutines with exactly 4 choices."

// pace keeps the runner under entry-tier RPM caps (the #141 lane measured 10
// requests/min on a fresh OpenAI account); bursting past the cap turns the
// run into a measurement of admission control.
const pace = 7 * time.Second

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

	promptHash := sha256.Sum256([]byte(systemMsg + "\x00" + userMsg + "\x00" + schema))
	rec := bench.Record{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Provider:      profileID,
		Model:         model,
		PromptHash:    fmt.Sprintf("%x", promptHash[:8]),
		SchemaVariant: variant,
		Sampling:      "provider-default",
		Metrics:       map[string]bench.Proportion{},
		Verdicts:      map[string]bench.Verdict{},
	}

	parseOK, contractOK := 0, 0
	var latencySum float64
	var questions []quiz
	for attempt := 0; rec.Tally.Executed() < n && attempt < 2*n; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), perCallDeadline)
		start := time.Now()
		var q quiz
		gerr := client.Generate(ctx, ports.Request{System: systemMsg, User: userMsg, Schema: []byte(schema)}, &q)
		cancel()
		class := classify(gerr)
		rec.Tally.Add(class)
		if class != bench.ClassExecuted {
			fmt.Printf("GEN %02d %s (excluded) err=%v\n", attempt+1, class, gerr)
			time.Sleep(pace)
			continue
		}
		latencySum += time.Since(start).Seconds()
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
			attempt+1, pv, cv, time.Since(start).Seconds(), len(q.Choices), gerr)
		time.Sleep(pace)
	}
	executed := rec.Tally.Executed()
	if executed > 0 {
		rec.MeanLatencyS = latencySum / float64(executed)
	}
	rec.Metrics["parse_valid"] = bench.NewProportion(parseOK, executed)
	rec.Metrics["contract_valid"] = bench.NewProportion(contractOK, executed)
	rec.Verdicts["parse_valid"] = rec.Metrics["parse_valid"].Judge(0.80, 0.90)
	rec.Verdicts["contract_valid"] = rec.Metrics["contract_valid"].Judge(0.80, 0.90)

	// Guessability: answer from the choices alone, no stem. An interval above
	// chance (0.25) flags options that leak the answer — the distractor
	// weakness the item-writing literature predicts for generated items. The
	// rater is this same model, which biases the probe toward its own cue
	// preferences; the hardened cross-model protocol is #139 follow-up work.
	guessed, gN := 0, 0
	limit := min(len(questions), 8)
	for _, q := range questions[:limit] {
		var choicesList strings.Builder
		for i, ch := range q.Choices {
			fmt.Fprintf(&choicesList, "%d: %s\n", i, ch)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		var g struct {
			ChoiceIndex int `json:"choice_index"`
		}
		gerr := client.Generate(ctx, ports.Request{
			System: "You are given only the answer choices of a hidden multiple-choice question. Pick the index most likely to be the correct answer.",
			User:   choicesList.String(),
			Schema: []byte(guessSchema),
		}, &g)
		cancel()
		if gerr != nil {
			continue
		}
		gN++
		if g.ChoiceIndex == q.Correct {
			guessed++
		}
		time.Sleep(pace)
	}
	rec.Metrics["guessability"] = bench.NewProportion(guessed, gN)

	fmt.Printf("\nPARSE    %d/%d [%.3f, %.3f] %s\n", parseOK, executed,
		rec.Metrics["parse_valid"].Lo, rec.Metrics["parse_valid"].Hi, rec.Verdicts["parse_valid"])
	fmt.Printf("CONTRACT %d/%d [%.3f, %.3f] %s\n", contractOK, executed,
		rec.Metrics["contract_valid"].Lo, rec.Metrics["contract_valid"].Hi, rec.Verdicts["contract_valid"])
	fmt.Printf("GUESS    %d/%d [%.3f, %.3f] chance=0.25\n", guessed, gN,
		rec.Metrics["guessability"].Lo, rec.Metrics["guessability"].Hi)
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
