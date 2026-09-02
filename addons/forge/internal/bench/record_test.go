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

import (
	"encoding/json"
	"testing"
)

// The verdict rule is the pre-registered #141 rule: a criterion is decided by
// the interval, not the point estimate, and an interval that straddles the
// floor can only ever be undecided — never quietly rounded to a pass.
func TestVerdictIsDecidedByTheIntervalNotThePointEstimate(t *testing.T) {
	cases := []struct {
		name          string
		k, n          int
		floor, target float64
		want          Verdict
	}{
		{"clears floor and target", 20, 20, 0.80, 0.90, VerdictPass},
		{"interval entirely below the floor", 11, 20, 0.80, 0.90, VerdictFail},
		{"rate above target but interval straddles the floor", 19, 20, 0.80, 0.90, VerdictUndecided},
		{"rate below target with the floor inside the interval", 15, 20, 0.80, 0.90, VerdictUndecided},
		{"nothing executed", 0, 0, 0.80, 0.90, VerdictUndecided},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := NewProportion(c.k, c.n)
			if got := p.Judge(c.floor, c.target); got != c.want {
				t.Fatalf("Judge(%d/%d, floor %.2f, target %.2f) = %s, want %s",
					c.k, c.n, c.floor, c.target, got, c.want)
			}
		})
	}
}

// A proportion serialises with its interval attached: a record that could be
// written without the interval would reintroduce the bare point estimate the
// harness exists to abolish.
func TestProportionSerialisesWithItsInterval(t *testing.T) {
	raw, err := json.Marshal(NewProportion(5, 7))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"k", "n", "rate", "wilson95_lo", "wilson95_hi"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("serialised proportion is missing %q: %s", key, raw)
		}
	}
}

// Excluded classes never leak into a denominator: a throttled or quota-blocked
// request reached admission control, not the model, and counting it against
// validity measures the account, not the wire.
func TestTallyKeepsExcludedClassesOutOfTheDenominator(t *testing.T) {
	var tl Tally
	tl.Add(ClassExecuted)
	tl.Add(ClassExecuted)
	tl.Add(ClassThrottled)
	tl.Add(ClassQuota)
	tl.Add(ClassTimeout)
	if got := tl.Executed(); got != 2 {
		t.Fatalf("Executed() = %d, want 2 — excluded classes leaked into the denominator", got)
	}
	if got := tl.Total(); got != 5 {
		t.Fatalf("Total() = %d, want 5 — attempts must stay visible even when excluded", got)
	}
}
