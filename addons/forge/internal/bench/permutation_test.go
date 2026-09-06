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

import "testing"

func TestPermutationIsAValidDeterministicShuffle(t *testing.T) {
	a := Permutation(4, 7)
	b := Permutation(4, 7)
	seen := make(map[int]bool, 4)
	for i, v := range a {
		if v < 0 || v >= 4 || seen[v] {
			t.Fatalf("Permutation(4,7) = %v is not a permutation", a)
		}
		seen[v] = true
		if b[i] != v {
			t.Fatalf("same seed produced different shuffles: %v vs %v", a, b)
		}
	}
}

func TestPermutationRespondsToTheSeed(t *testing.T) {
	same := 0
	for seed := int64(0); seed < 16; seed++ {
		p := Permutation(4, seed)
		if p[0] == 0 && p[1] == 1 && p[2] == 2 && p[3] == 3 {
			same++
		}
	}
	if same == 16 {
		t.Fatal("sixteen seeds all produced the identity — the seed is ignored")
	}
}
