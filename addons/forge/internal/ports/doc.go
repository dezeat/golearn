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

// Package ports defines the interfaces Forge's adapters implement.
//
// It holds interfaces and nothing else — no implementations, no adapter
// imports — matching the core's ports layer and the layering law in AGENTS.md.
// The value types the interfaces exchange live in the Forge domain package.
//
// Three shaping decisions are recorded here because they are the ones a reader
// would otherwise have to reverse-engineer from four separate adapters:
//
// 1. Embedding is a separate, optional interface, not a Provider method.
// Anthropic ships no embeddings API, so a provider profile either has the
// capability or does not, and modeling that as a method returning a sentinel
// error would turn a permanent product fact into a runtime surprise. An
// adapter without an embeddings endpoint simply does not implement [Embedder];
// absence is then a compile-time fact that a test can assert, and
// [domain.ErrNoEmbeddingCapability] is the typed form for the pipeline's
// fail-clear path. This is also why "all four profiles satisfy one port
// contract" needs reading with care: Anthropic satisfies the chat contract,
// not the embedding one, and that is the design rather than a gap in it.
//
// 2. [SimilarityIndex] exchanges vectors, never providers. The similarity
// backend must not know how a vector was produced, and the embedding source
// must not know what it will be compared against. The pipeline holds both and
// fetches the vectors; giving the index a Provider reference would fuse the
// two seams and make either one unswappable. It also addresses one corpus
// only — library questions, by [domain.LibraryQuestionID] — because a
// candidate that may still be rejected must not leave a vector behind in a
// store whose entire contents are supposed to be practisable content (D-021).
//
// 3. The store interfaces are Forge-local and additive. They are implemented
// by a Forge adapter that opens the same database the core does and keeps its
// own migration registry, because a shared migration counter silently skips
// the core's next migration — measured, with the failing case, in
// docs/design/FORGE-EXPERIMENTS.md A-7. No Forge migration may alter a core
// table; that is what keeps a Forge-extended database compatible rather than
// newer, so the offline binary keeps opening it.
package ports
