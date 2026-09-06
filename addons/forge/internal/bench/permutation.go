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

import "math/rand"

// Permutation returns a seeded Fisher-Yates shuffle of 0..n-1. Judge probes
// shuffle choice order to counter position bias, and the shuffle is seeded so
// a run is reproducible — the repo's determinism rule applies to the harness
// as much as to the product.
func Permutation(n int, seed int64) []int {
	r := rand.New(rand.NewSource(seed))
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	r.Shuffle(n, func(i, j int) { p[i], p[j] = p[j], p[i] })
	return p
}
