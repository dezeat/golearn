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

// Package boundary holds no production code. It exists so the executable
// guards for D-015's binary-scoped offline law live inside the root module,
// where they fail the core gate the moment the core stops being offline.
//
// The guards are deliberately narrow, because a wider claim would be false.
// The core's dependency graph already contains net and net/url: both arrive
// transitively through github.com/google/uuid, which modernc.org/sqlite
// requires. Neither is reachable from first-party code. Asserting "the core
// imports no network package" would therefore fail on day one and invite
// someone to weaken the guard rather than fix a real leak.
//
// What is asserted instead, and why each line holds:
//
//   - net/http is absent from the core binary's transitive graph. Every
//     provider SDK and HTTP client pulls it, so its absence is the signal
//     that Forge's dependencies have not leaked across the module boundary.
//   - the root go.mod's direct requirements are exactly the four runtime
//     dependencies D-015 fixes. A nested addon module cannot widen this set
//     without the change showing up here.
//   - no first-party package imports a network package directly, which
//     catches golearn code reaching for the network even when a transitive
//     dependency already supplies the import.
package boundary
