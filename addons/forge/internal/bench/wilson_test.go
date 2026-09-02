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
	"math"
	"testing"
)

// Oracle values are the Wilson intervals committed in issue #139 before this
// package existed — computed independently of this implementation, which is
// what the repo's external-oracle rule requires.
func TestWilsonIntervalMatchesTheCommittedOracleValues(t *testing.T) {
	cases := []struct {
		name   string
		k, n   int
		lo, hi float64
	}{
		{"perfect recall on seven positives stays undecided", 7, 7, 0.646, 1.000},
		{"five of seven straddles the 0.80 floor", 5, 7, 0.359, 0.918},
		{"zero of six false positives still admits a 39 percent rate", 0, 6, 0.000, 0.390},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lo, hi := Wilson95(c.k, c.n)
			if math.Abs(lo-c.lo) > 0.0005 || math.Abs(hi-c.hi) > 0.0005 {
				t.Fatalf("Wilson95(%d,%d) = [%.4f, %.4f], oracle [%.3f, %.3f]",
					c.k, c.n, lo, hi, c.lo, c.hi)
			}
		})
	}
}

func TestWilsonIntervalOnAnEmptySampleSpansEverything(t *testing.T) {
	lo, hi := Wilson95(0, 0)
	if lo != 0 || hi != 1 {
		t.Fatalf("Wilson95(0,0) = [%v, %v], want the uninformative [0, 1]", lo, hi)
	}
}
