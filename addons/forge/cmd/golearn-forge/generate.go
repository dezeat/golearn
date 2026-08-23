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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	"github.com/dezeat/golearn/addons/forge/internal/adapters/provider"
	"github.com/dezeat/golearn/addons/forge/internal/adapters/secrets"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/pipeline"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coredomain "github.com/dezeat/golearn/internal/domain"
)

// generateFlags is the minimal surface needed to reach the pipeline.
//
// Parsing stays manual os.Args handling (D-003) — the same convention the core
// binary uses. This is the smallest command that makes #122 reachable; the
// full CLI surface is #129's.
type generateFlags struct {
	topic       string
	description string
	count       int
	difficulty  string
	providerID  string
	model       string
	embedModel  string
	endpoint    string
	dbPath      string
	language    string
	dryRun      bool
}

func parseGenerateFlags(args []string) (generateFlags, error) {
	f := generateFlags{
		count:      3,
		providerID: string(domain.ProfileOllama),
		language:   "en",
		embedModel: "nomic-embed-text",
		dbPath:     coresqlite.DefaultDBPath(),
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", arg)
			}
			i++
			return args[i], nil
		}

		var err error
		switch arg {
		case "--topic", "-t":
			f.topic, err = value()
		case "--description", "-d":
			f.description, err = value()
		case "--count", "-n":
			var raw string
			if raw, err = value(); err == nil {
				f.count, err = strconv.Atoi(raw)
				if err != nil {
					err = fmt.Errorf("--count must be a number, got %q", raw)
				}
			}
		case "--difficulty":
			f.difficulty, err = value()
		case "--provider":
			f.providerID, err = value()
		case "--model", "-m":
			f.model, err = value()
		case "--embedding-model":
			f.embedModel, err = value()
		case "--endpoint":
			f.endpoint, err = value()
		case "--db":
			f.dbPath, err = value()
		case "--language":
			f.language, err = value()
		case "--dry-run":
			f.dryRun = true
		default:
			if strings.HasPrefix(arg, "-") {
				err = fmt.Errorf("unknown flag %q", arg)
			}
		}
		if err != nil {
			return generateFlags{}, err
		}
	}

	if strings.TrimSpace(f.topic) == "" {
		return generateFlags{}, errors.New("--topic is required")
	}
	if f.model == "" {
		return generateFlags{}, errors.New("--model is required (there is no default model; the choice is yours)")
	}
	return f, nil
}

// runGenerate wires the composition root and runs one generation.
//
// This is the only place that knows about every package, which is what the
// hexagonal rules require of a composition root.
func runGenerate(args []string, stdout, stderr io.Writer) int {
	flags, err := parseGenerateFlags(args)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n\n%s", err, generateUsage()))
		return 2
	}

	spec := domain.GenerationSpec{
		Topic:       flags.topic,
		Description: flags.description,
		Count:       flags.count,
		Difficulty:  coredomain.Difficulty(flags.difficulty),
		Language:    flags.language,
	}
	if err := pipeline.ValidateSpec(spec); err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 2
	}

	// Cancellation is wired before anything expensive starts, so Ctrl-C during
	// a multi-minute run releases the provider request rather than orphaning
	// it — and the run is recorded as canceled, not failed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := coresqlite.Open(flags.dbPath)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}
	defer func() { _ = db.Close() }()

	store, err := forgestore.New(ctx, db)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	resolver := secrets.New()
	opts := []provider.ClientOption{provider.WithEmbeddingModel(flags.embedModel)}
	if flags.endpoint != "" {
		opts = append(opts, provider.WithEndpoint(flags.endpoint))
	}
	llm, err := provider.Open(ctx, resolver, domain.ProfileID(flags.providerID), flags.model, opts...)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	if flags.dryRun {
		write(stdout, fmt.Sprintf("%s\n", describePlan(flags, spec, llm.Identity())))
		return 0
	}

	// Research is not wired here. #126 landed the adapter but its endpoint
	// configuration and the source-authority policy are #120's, so wiring it
	// would mean choosing a policy this binary has no authority to choose.
	// The pipeline reports the absence rather than pretending grounding ran.
	deps := pipeline.Deps{
		Provider: llm, Runs: store, Drafts: store,
		Now: time.Now, ForgeVersion: version,
	}
	p, err := pipeline.New(deps, pipeline.DefaultBudgets())
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	write(stdout, fmt.Sprintf("Generating %d question(s) on %q using %s.\n",
		spec.Count, spec.Topic, llm.Identity()))
	write(stdout, "The full trust chain runs on every candidate, so this takes minutes rather than seconds. Ctrl-C cancels.\n\n")

	start := time.Now()
	result, err := p.Generate(ctx, spec)
	if err != nil {
		reportFailure(stderr, err, time.Since(start))
		return 1
	}

	printDraft(stdout, result, time.Since(start))
	write(stdout, fmt.Sprintf("\nDraft %d is saved and is NOT yet library content.\n", result.Draft.ID))
	write(stdout, "Run 'golearn-forge drafts' to review, add or discard it.\n")
	return 0
}

func describePlan(flags generateFlags, spec domain.GenerationSpec, identity domain.ModelIdentity) string {
	var b strings.Builder
	b.WriteString("Plan (dry run — nothing was generated and no call was made)\n")
	fmt.Fprintf(&b, "  topic       %s\n", spec.Topic)
	fmt.Fprintf(&b, "  count       %d\n", spec.Count)
	if spec.Difficulty != coredomain.DifficultyUnset {
		fmt.Fprintf(&b, "  difficulty  %s\n", spec.Difficulty)
	}
	fmt.Fprintf(&b, "  provider    %s\n", identity)
	fmt.Fprintf(&b, "  database    %s\n", flags.dbPath)
	b.WriteString("  grounding   not wired (research adapter selection is tracked in #120)\n")
	return b.String()
}

// reportFailure surfaces a short actionable reason, which is all FORGE.md 3.2
// permits on failure — pipeline internals stay internal.
func reportFailure(stderr io.Writer, err error, elapsed time.Duration) {
	switch {
	case errors.Is(err, context.Canceled):
		write(stderr, fmt.Sprintf("\nCanceled after %s. No draft was created.\n", elapsed.Round(time.Second)))
	case errors.Is(err, pipeline.ErrShortfall):
		write(stderr, fmt.Sprintf("\nerror: %v\n\n", err))
		write(stderr, "Forge will not emit a pack it could not verify. Try a narrower topic, a smaller count, or a stronger model.\n")
	case errors.Is(err, domain.ErrProviderUnreachable):
		write(stderr, fmt.Sprintf("\nerror: %v\n\nCheck that the provider endpoint is running and reachable.\n", err))
	case errors.Is(err, domain.ErrCredentialMissing), errors.Is(err, domain.ErrCredentialRejected):
		write(stderr, fmt.Sprintf("\nerror: %v\n", err))
	default:
		write(stderr, fmt.Sprintf("\nerror: %v\n", err))
	}
}

func printDraft(stdout io.Writer, result pipeline.Result, elapsed time.Duration) {
	pack := result.Draft.Pack
	write(stdout, fmt.Sprintf("Generated %d question(s) in %s (%d round(s), %d rejected, %d repaired).\n\n",
		len(pack.Questions), elapsed.Round(time.Second), result.Rounds, result.Rejected, result.Repaired))
	printQuestions(stdout, pack.Questions)
}

// printQuestions renders a pack for review.
//
// Choice labels are computed from display order at render time (D-008); the
// stored choice id is never used as a display label.
func printQuestions(stdout io.Writer, questions []coredomain.PackQuestion) {
	for i, q := range questions {
		write(stdout, fmt.Sprintf("%d. %s\n", i+1, q.Prompt))
		correct := map[string]bool{}
		for _, id := range q.CorrectChoiceIDs {
			correct[id] = true
		}
		for j, c := range q.Choices {
			marker := " "
			if correct[c.ID] {
				marker = "*"
			}
			// Labels are computed from display order at render time (D-008);
			// the stored choice id is never a display label.
			write(stdout, fmt.Sprintf("   %s %c) %s\n", marker, 'A'+rune(j), c.Text))
		}
		if q.Rationale != nil && q.Rationale.Correct != "" {
			write(stdout, fmt.Sprintf("     %s\n", q.Rationale.Correct))
		}
		write(stdout, "\n")
	}
}

func generateUsage() string {
	return `Usage:
  golearn-forge generate --topic "<topic>" --model "<model>" [options]

Required:
  --topic, -t        What the questions should be about
  --model, -m        The model to use; there is no default

Options:
  --count, -n        How many questions (default 3, max 20)
  --description, -d  Extra steer on what to focus on
  --difficulty       easy | medium | hard
  --provider         openai | anthropic | openrouter | ollama (default ollama)
  --endpoint         Override the provider endpoint
  --embedding-model  Embedding model for the similarity gate
  --language         Content language (default en)
  --db               Database path
  --dry-run          Show the plan without generating

Notes:
  The full trust chain runs on every candidate — generation, validation,
  independent verification, critique, similarity, and at most one repair —
  so a run takes minutes per question, not seconds.
  If Forge cannot produce the number of questions you asked for, it fails
  and tells you why. It never emits a partly verified pack.
`
}
