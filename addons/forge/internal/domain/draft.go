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
	"time"

	coredomain "github.com/dezeat/golearn/internal/domain"
)

// RunStatus is the lifecycle state of one generation run. FORGE.md 8 keeps run
// history minimal: status, timestamps, provider/model identity, source
// references, cost and attempt summaries, and a concise diagnostic outcome.
type RunStatus string

// The run lifecycle. A run that ends in any state other than RunSucceeded has
// produced no draft, which is what makes "no junk" checkable rather than
// aspirational.
const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCanceled  RunStatus = "canceled"
)

// Cost summarizes what a run spent. Local inference bills nothing, so zero is
// a real value here and not a missing one.
type Cost struct {
	InputTokens  int
	OutputTokens int
	// Attempts counts candidate generations, including the one permitted
	// repair. It is a summary, not the retry mechanics FORGE.md 8 forbids
	// persisting: how many, never which or why-in-detail.
	Attempts int
}

// Run is the minimal durable record of a generation attempt.
//
// What it may never carry is as much a part of the contract as what it does:
// no secrets, no raw prompts, no raw model or tool output, no copied pages, no
// request mechanics. Diagnostic is one concise human-readable clause, and the
// redaction guard in internal/config is what holds it to that.
type Run struct {
	ID         int64
	Spec       GenerationSpec
	Model      ModelIdentity
	Verifier   ModelIdentity
	Status     RunStatus
	StartedAt  time.Time
	FinishedAt *time.Time
	Sources    []SourceRef
	Cost       Cost
	Diagnostic string
}

// Draft is a fully validated, preview-ready pack that has not entered the
// library.
//
// FORGE.md 8's lifecycle in one sentence: a draft exists only once generation
// has fully succeeded, "Add to library" runs the standard atomic import and
// then deletes it, "Discard" deletes it, and a crash leaves it recoverable but
// never visible as library content. No draft is created during an active run,
// so a cancellation cannot leave a half-built one behind.
type Draft struct {
	ID         int64
	RunID      int64
	Pack       coredomain.Pack
	Spec       GenerationSpec
	Provenance Provenance
	CreatedAt  time.Time
}

// QuestionCount reports how many questions the draft would import.
func (d Draft) QuestionCount() int { return len(d.Pack.Questions) }

// Summary renders a one-line, disclosure-safe description for the draft
// screen. It names the model but never the endpoint that served it.
func (d Draft) Summary() string {
	return fmt.Sprintf("%s — %d questions, %s",
		d.Pack.Topic.Name, len(d.Pack.Questions), d.Provenance.Model)
}
