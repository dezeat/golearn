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
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	"github.com/dezeat/golearn/addons/forge/internal/ports"
)

// NewGateWithCalibration builds a gate around a caller-supplied threshold set.
//
// It lives in a _test.go file deliberately: the Go toolchain compiles this
// file only when testing this package, so no production code can reach it.
// That is what lets the exported [NewGate] take its thresholds solely from the
// committed table — closing the path by which a number picked by feel could
// reach a scoring decision — while the ladder's behavior still gets tested
// against thresholds a test controls.
//
// Preflight still applies to gates built this way, so a fixture or unversioned
// calibration is refused here exactly as it would be in production.
func NewGateWithCalibration(
	embedder ports.Embedder,
	index ports.SimilarityIndex,
	library ports.LibraryReader,
	calib domain.Calibration,
) *Gate {
	return &Gate{embedder: embedder, index: index, library: library, calib: calib}
}
