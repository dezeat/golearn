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

// Package bench is the measurement core of the Forge benchmark harness
// (#139): one run-record schema, Wilson intervals as mandatory output, and
// verdicts that state whether the interval clears the criterion rather than
// whether a point estimate does. At the sample sizes this harness runs, a
// bare rate is not evidence — the interval is the result.
package bench

import "math"

// z975 is the two-sided 95% normal quantile. The harness deliberately offers
// exactly one confidence level: configurable z would invite picking the level
// that makes a result look decided.
const z975 = 1.959963985

// Wilson95 returns the 95% Wilson score interval for k successes in n trials.
// n = 0 yields the uninformative [0, 1] rather than NaN, so a run that
// executed nothing cannot masquerade as a decided one.
func Wilson95(k, n int) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	p := float64(k) / float64(n)
	nf := float64(n)
	denom := 1 + z975*z975/nf
	center := (p + z975*z975/(2*nf)) / denom
	half := z975 * math.Sqrt(p*(1-p)/nf+z975*z975/(4*nf*nf)) / denom
	lo = center - half
	hi = center + half
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}
