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

package bench

import "encoding/json"

// Verdict states whether an interval decided a criterion. There is no
// "passed on the point estimate" state on purpose.
type Verdict string

const (
	VerdictPass      Verdict = "PASS"
	VerdictFail      Verdict = "FAIL"
	VerdictUndecided Verdict = "UNDECIDED"
)

func (v Verdict) String() string { return string(v) }

// Class names what happened to one attempt. Only executed attempts enter a
// rate denominator; the other classes are counted but excluded, because a
// request the model never saw says nothing about the model. The taxonomy
// comes from the #141 live lane, where quota exhaustion, RPM throttling and
// full-deadline timeouts each produced a distinct failure signature.
type Class string

const (
	ClassExecuted  Class = "executed"
	ClassThrottled Class = "throttled"
	ClassQuota     Class = "quota"
	ClassTimeout   Class = "timeout"
)

// Tally counts attempts per class.
type Tally struct {
	Counts map[Class]int `json:"counts"`
}

func (t *Tally) Add(c Class) {
	if t.Counts == nil {
		t.Counts = make(map[Class]int)
	}
	t.Counts[c]++
}

// Executed is the only legitimate rate denominator.
func (t *Tally) Executed() int { return t.Counts[ClassExecuted] }

// Total keeps excluded attempts visible: exclusion without a visible count
// would be a quiet way to shop for a denominator.
func (t *Tally) Total() int {
	sum := 0
	for _, n := range t.Counts {
		sum += n
	}
	return sum
}

// Proportion is k successes over n executed attempts with its Wilson interval
// attached at construction, so no code path can carry the rate without the
// uncertainty.
type Proportion struct {
	K    int     `json:"k"`
	N    int     `json:"n"`
	Rate float64 `json:"rate"`
	Lo   float64 `json:"wilson95_lo"`
	Hi   float64 `json:"wilson95_hi"`
}

func NewProportion(k, n int) Proportion {
	lo, hi := Wilson95(k, n)
	rate := 0.0
	if n > 0 {
		rate = float64(k) / float64(n)
	}
	return Proportion{K: k, N: n, Rate: rate, Lo: lo, Hi: hi}
}

// Judge decides a floor/target criterion by the interval: PASS needs the
// whole interval above the floor and the rate at or above the target; FAIL
// needs the whole interval below the floor; everything else — including an
// empty sample — is undecided.
func (p Proportion) Judge(floor, target float64) Verdict {
	if p.N == 0 {
		return VerdictUndecided
	}
	if p.Lo > floor && p.Rate >= target {
		return VerdictPass
	}
	if p.Hi < floor {
		return VerdictFail
	}
	return VerdictUndecided
}

// Record is one benchmark run: the axes that make it comparable, the tally of
// what happened, and the measured proportions with their verdicts. Prompt and
// schema hashes are axes rather than metadata — a comparison table that omits
// them compares nothing (#139).
type Record struct {
	Timestamp     string                `json:"timestamp"`
	Provider      string                `json:"provider"`
	Model         string                `json:"model"`
	JudgeProvider string                `json:"judge_provider,omitempty"`
	JudgeModel    string                `json:"judge_model,omitempty"`
	PromptHash    string                `json:"prompt_hash"`
	PromptVariant string                `json:"prompt_variant,omitempty"`
	SchemaVariant string                `json:"schema_variant"`
	Sampling      string                `json:"sampling"`
	TopicVariant  string                `json:"topic_variant,omitempty"`
	Tally         Tally                 `json:"tally"`
	Metrics       map[string]Proportion `json:"metrics"`
	Verdicts      map[string]Verdict    `json:"verdicts"`
	MeanLatencyS  float64               `json:"mean_latency_s"`
	Notes         string                `json:"notes,omitempty"`
	// Questions preserves what was actually judged, so a later arm can
	// re-judge identical items — without it, every judge comparison is
	// confounded with regeneration sampling.
	Questions []StoredQuestion `json:"questions,omitempty"`
}

// StoredQuestion is one generated item as the probes saw it.
type StoredQuestion struct {
	Prompt      string   `json:"prompt"`
	Choices     []string `json:"choices"`
	Correct     int      `json:"correct_index"`
	ShuffleSeed int64    `json:"shuffle_seed"`
}

// JSONL renders the record as one line, the harness's on-disk format: append-
// only, diff-friendly, and trivially machine-readable for later analysis.
func (r Record) JSONL() ([]byte, error) { return json.Marshal(r) }
