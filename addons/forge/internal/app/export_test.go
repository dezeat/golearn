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

package app

import (
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// NewGateForTest builds a gate without running Preflight.
//
// It lives in a _test.go file deliberately: the Go toolchain compiles this
// file only when testing this package, so no production code can reach it.
//
// Its predecessor existed to hand the gate a threshold a test controlled,
// because the exported constructor took its numbers from the committed
// calibration table and nothing else. Under D-023 there is no threshold to
// supply, so this exists only to let a test assemble a gate with deliberately
// missing halves and observe how it refuses.
func NewGateForTest(
	embedder ports.Embedder,
	index ports.SimilarityIndex,
	library ports.LibraryReader,
	judge ports.DuplicateJudge,
) *Gate {
	return &Gate{embedder: embedder, index: index, library: library, judge: judge}
}
