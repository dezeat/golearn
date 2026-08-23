# golearn Forge — Design Spec

**Status: design intent, partially implemented.** This document is the plan of
record for `golearn-forge`, the assisted-authoring addon (epic #66). It
consolidates what previously lived only in issue comments and session
handovers into one reviewable place. Three rules govern it:

1. **Precedence.** `docs/DECISIONS.md` (binding rationale — D-015–D-017 for the
   product frame, D-018–D-020 for the surfaces settled during implementation)
   and `docs/architecture.md` (implemented current state) win over this file on
   any conflict. Issues and the Project board carry work units and status, never
   design truth. `docs/design/FORGE-EXPERIMENTS.md` carries what was *measured*.
2. **The spike gates are closed.** The two surfaces this file once marked
   **⏳ spike-gated** — secret storage and the similarity backend/embedding
   source — are decided in D-018–D-020, and those sections now say so. What
   remains open is listed in §12, and one item there is easy to misread: a
   self-hosted SearXNG is the *development and verification* adapter, **not**
   the shipped V1 choice, which is still a maintainer call.
3. **Graduation, in progress.** The module topology (§2) has landed and has
   already graduated: `docs/architecture.md` carries the **Authoring boundary**
   section, and `AGENTS.md`'s core principle and anti-patterns are amended for
   the binary-scoped offline law per D-015. The remaining sections graduate as
   their stories land, after which this file reduces to a pointer. This spec
   describes the road; `architecture.md` describes the destination once
   reached.

---

## 1. Product frame

golearn grows from a *practice* engine into a **practice + authoring** engine
for 1.0.0 — without touching the offline product:

- **Bring your own packs, or bring your own key.** Pack authoring/import by
  hand stays the default; generation is purely opt-in via a self-managed
  provider (API key, or a local Ollama endpoint).
- **Two binaries** (D-015): `golearn` — the existing offline Core + TUI,
  byte-for-byte unchanged, no network path at all; `golearn-forge` — the same
  application plus question generation.
- The offline law is **binary-scoped, not key-scoped**: Forge's network
  activity exists only at authoring time; import, selection, practice, export,
  and stats stay offline in both binaries. golearn never holds, proxies, or
  defaults a key.
- Forge is additive, not a fork: it must feel like the normal golearn
  application with one extra capability, not a second product.

## 2. Module topology & build (D-015)

- Forge is a **nested addon module** with its own `go.mod`, expected under
  `addons/forge`. It owns every provider SDK, HTTP client, and retrieval
  dependency and their transitive graphs.
- The root module stays the Core module at **exactly four runtime deps**
  (`bubbletea`, `lipgloss`, `yaml.v3`, `modernc.org/sqlite`), CGo-free,
  cross-compiling with no C toolchain.
- Dependency direction is strictly one-way: Forge imports stable Core /
  shared-TUI contracts; **the Core never imports or knows Forge**. The Forge
  binary's composition root is the only place wiring Core and Forge adapters
  together (hexagonal rules unchanged: domain pure, ports interfaces, app
  inward, adapters never import each other).
- Build tags were rejected as the isolation mechanism (they select files but
  leave dependencies in the module graph and `go.sum`). A local `go.work`
  ties the two modules together for joint development and testing.
- Release ships both binaries; the Core's release matrix is unchanged.
- CLI routing follows D-003 (#76): both binaries keep manual `os.Args`
  routing as two coherent TUI applications — no cobra migration.

## 3. User experience

### 3.1 Entry and inputs

- The only visible change in Forge is one start-menu entry, **Generate
  Questions**. Library, practice, import/export, and stats flows are the
  Core's, unchanged. The plain `golearn` binary never shows the entry.
- Generation inputs: topic, optional description, question count, difficulty,
  and enabled style/mode controls (taxonomy evaluation-gated, §9).
- **Advanced Options are collapsed** and expose effort presets — *Sparsam*,
  *Standard* (default), *Gründlich* — never raw tokens or retry counters.

### 3.2 Waiting, result, and acceptance (D-016)

- Generation shows a terminal-native animated ASCII forge/loading screen with
  elapsed time and Cancel — no fake percentage bar. Respect narrow terminals,
  `NO_COLOR`, and deterministic TUI testing.
- The normal flow is deliberately clean: request → wait → one finished,
  already quality-checked pack preview (with provenance and cost/run info) →
  explicit **Add to library** or **Discard**. *Add to library* invokes the
  standard atomic import path internally (D-004); the user never manages
  files. *Regenerate* is the third action. Nothing imports or publishes
  automatically.
- Per-question inspection/editing is an optional escape hatch, never required.
- Pipeline detail surfaces only on failure: a short actionable reason plus a
  regenerate offer, not internal mechanics.

### 3.3 Drafts — the no-junk rule

- Entering Forge with unresolved drafts opens a draft screen first: each draft
  can be viewed, added to the library, or discarded. **New generation starts
  only after outstanding drafts are resolved.** Menu polish is a prototype
  concern; the rule is binding.

## 4. Generation pipeline & trust boundary

Forge V1 is an **explicit, bounded Go workflow** — not a free-running agent
and not built on an agent framework (#80: no langchaingo, agent SDK, Temporal,
or dynamic loops).

1. Validate the user request; resolve the configured provider/model.
2. Perform automatic web research (§5).
3. Build evidence records from retrieved sources.
4. Generate a candidate pack grounded in that evidence.
5. Run deterministic schema and domain validation.
6. Verify answers/keys in a **fresh context on the same provider/model**
   (cross-provider verification is deferred).
7. Run critique, source-grounding, and near-duplicate gates (§7).
8. Allow **exactly one** targeted repair and re-verification.
9. Discard/replace still-invalid or insufficiently distinct candidates within
   the chosen budget; if Forge cannot deliver the requested number of valid
   questions, **fail clearly** — never emit a silently degraded pack.

Trust rules: retrieved pages and search results are **untrusted data** — they
cannot alter workflow rules, invoke actions, or override policy
(prompt-injection boundary). Forge owns stop conditions, source policy,
budgets, cancellation, validation, and the acceptance gate. Generation may be
non-deterministic; the accepted pack re-enters the unchanged deterministic
pipeline (D-004/D-005/D-007) like any hand-authored pack.

## 5. Research

- **Automatic research only in V1** — users do not supply URLs or documents.
- An app-owned **Research port** (search/fetch/evidence-build) returns
  standardised evidence records: source identity, URL, title/publisher where
  available, retrieval time, relevant content, quality/policy metadata. The
  pipeline, generator, verifier, and critic consume evidence records; nothing
  browses independently.
- **Exactly one concrete search/fetch adapter in V1** — provider selection,
  reliability, cost, rate limits, attribution, and injection defenses are the
  second spike's deliverable (**⏳ spike-gated**; ticket to mint). The adapter
  knows only the provider's API/auth/wire format; query planning, source
  policy, evidence assembly, budgets, and citations stay in the pipeline.
- Prefer authoritative sources; never silently lower the quality bar to reach
  the requested count. Insufficient evidence ⇒ bounded automatic retry, then a
  clear failure. The **source-authority policy** (categories, ranking,
  exclusions, exceptions) is a separate versioned specification, not prompt
  text hidden in an adapter.
- Adapter tests run against recorded fixtures; no live calls in `make check`.

## 6. Providers & secrets

### 6.1 Profiles

User flow is **Provider → Model**; Forge remembers the selection, no
vendor/model is globally forced.

| Provider   | Auth       | Notes                                              |
| ---------- | ---------- | -------------------------------------------------- |
| OpenAI     | API key    | initial integration                                |
| Anthropic  | API key    | initial integration                                |
| OpenRouter | API key    | OpenAI-compatible                                  |
| Ollama     | none (local) | endpoint (default `http://localhost:11434`) + model |

- Ollama's "login" is a **reachability check**, not key validation:
  `GET /api/tags` answers "is Ollama running?" and "which models are
  installed?" in one call and feeds model selection. Connection refused
  surfaces as "Ollama is not running at `<endpoint>`", not a raw HTTP error.
  `OLLAMA_HOST` is the env-override convention. Reverse-proxy auth headers are
  deferred beyond V1.
- OAuth/PKCE is deferred to provider-specific follow-on work; a login or
  subscription in another product is never treated as API authorization.

### 6.2 Secrets (**decided — D-019 supersedes this section's original direction**)

- **The environment is the primary source**; the OS keychain is a desktop
  addition on top of it. Never in SQLite, packs, logs, drafts, diagnostics, or
  the repo.
- This **inverts** the direction originally recorded here (keychain-primary,
  env for automation). The maintainer ruled headless/container-first, on the
  grounds that the environment path is the one actually used in practice. The
  inversion is deliberate and documented, not a drift — see D-019.
- Spike #106 is no longer a gate. What it still owes is narrower: the keychain
  library choice under the CGo-free constraint, the platform matrix, and
  whether the keychain path earns its dependency at all now that the primary
  path does not depend on it.

## 7. Similarity & near-duplicate gate

- Similarity is a **background pipeline gate**, not a user-facing review
  chore: too-similar candidates are repaired, replaced, or rejected within
  bounded attempts before the pack is presented. Existing library content is
  never modified automatically. Quality target favours few false positives;
  user-visible similarity output is exceptional diagnostics only.
- Comparison representation: normalised question prompt + answer options +
  correct answer(s) + tags. Introductions and explanations are excluded (they
  share source language and cause false positives).
- Exact content-hash dedup (D-007) is separate and unchanged; semantic
  similarity is additional behaviour, never a hash replacement.
- **Backend (decided — D-020):** float32 embedding BLOBs in the existing
  database, cosine computed in Go. D-012 settles it: performance is an explicit
  non-goal at this corpus size, so an index or an embedded vector store would
  be complexity spent on a problem that does not exist, and a second
  persistence surface is an explicit non-goal of the storage story. The spike
  ticket FORGE.md §12 once called for was never minted, because the rulings
  answered what it existed to decide. The original priority order, kept for the
  record:
  (1) can a compatible upgrade of the pure-Go SQLite stack expose a usable
  vector path; (2) a mature native-Go/CGo-free alternative; (3) fallback:
  Go-side similarity scoring, optionally with FTS5 lexical candidate
  narrowing. **FTS5 is verified working** in `modernc.org/sqlite` v1.54.0
  (smoke-tested 2026-07-26), so the fallback stands on tested ground. Hard
  constraints: no CGo, no native SQLite extensions (`sqlite-vec`/`sqlite-vss`
  are off the table under D-001), no vector DB/server, per-platform shared
  libraries forbidden.
- **Embedding source (decided — D-018):** follow the chat provider, and
  **fail clear where the provider has none**. Anthropic ships no embeddings
  API, so embedding is modelled as an *optional capability a provider
  advertises*, not a method every provider must implement — the absence is
  typed and testable rather than a runtime surprise. Background, kept because
  it explains the shape: pure-Go offline
  inference is effectively unavailable (ONNX runtimes need CGo); provider-API
  embeddings are legitimate inside Forge but tie similarity quality to the
  configured provider (Ollama exposes `/api/embeddings`, coverage varies by
  model). The recommendation must state the source per provider profile, the
  behaviour when none is available, and cost/latency impact. The backend stays
  behind a Forge port so a later swap never touches Core logic.

## 8. Persistence & drafts

Forge **extends the existing golearn SQLite database** — no second database.

- **Run history is minimal:** status, timestamps, provider/model identity,
  source references, cost/attempt summaries, and a concise diagnostic
  outcome. Categorically never persisted: secrets, raw prompts, raw
  model/tool output, copied web pages, retry counters, request mechanics.
- **Draft lifecycle:** a fully validated, preview-ready pack is saved
  atomically as a Forge draft. *Add to library* runs the standard atomic
  import, then deletes the draft; *Discard* deletes it. On close/crash the
  draft survives but is not library content. No draft is created during
  active generation; cancellation leaves at most minimal run diagnostics.
- Migrations follow the Core's rules: forward-only, atomic, fail-loud on
  incompatible/newer schema (D-014); a `golearn`-only user opening a
  Forge-migrated database gets the D-014 behaviour, never silent damage. The
  exact table shapes are implementation-story work under the D-013 freeze
  discipline.

## 9. Pack schema evolution (D-017)

- Current schema `0.1.0` → backwards-compatible **`0.2.0`** during Forge
  implementation: `0.1.x` stays importable, Forge exports `0.2.0`, the
  importer/exporter carry an explicit compatibility policy with migration
  tests. `0.2.0` remains adjustable through prototype/evaluation feedback; at
  the product V1 release it is promoted and frozen as pack schema **`1.0.0`**.
  Product version and schema major are intentionally decoupled until then.
- New pack-level fields:
  - **`generation_spec`** — every user-visible, content-shaping input: topic,
    optional description, requested count, difficulty, style/mode, language.
    Answers "what was requested?" and enables inspection, filtering, and
    partial reproduction.
  - **Provenance** — generation time, provider/model identity, source
    references.
- Per-question evidence keeps the existing `source` / `source_ref` /
  `confidence` fields; generated questions carry `confidence < 1.0`
  (hand-authored content keeps the manual default of 1.0). The finer
  assurance-level taxonomy (Ideas #99) is deferred.
- The style/mode **intent taxonomy** is a small structured enum, refined via
  the #105 spike and evaluation evidence before it freezes.
- **The D-007 hash recipe is untouched:** pack-level metadata never feeds the
  per-question content hash, so dedup is identical across `0.1.x`/`0.2.0`.
- Open (flagged on #77, to confirm): the generator applies a
  stricter-than-schema rule for multi-select correct-answer counts (≥2) while
  the schema keeps ≥1 — recommended, not yet confirmed.

## 10. Evaluation

- A versioned, repo-shipped, medium-sized **evaluation matrix** covers input
  combinations, difficulties, modes/styles, and required invariants. It is a
  development/release asset, not a runtime feature.
- Fixture-backed tests cover deterministic mechanics with **no live provider
  calls in `make check`**. At material prompt/model/threshold changes and at
  releases, selected cases run against real providers and are scored with a
  rubric.
- Runtime prompts may include a few generic, content-neutral examples for
  structure/difficulty/source-use/distractors; evaluation cases themselves are
  never embedded in live prompts.

## 11. V1 scope

**Required for trustworthy V1** (the pipeline stages are D-016-mandatory —
they replaced the per-question human gate and cannot be cut without reopening
that decision). Items 1, 2, 6, and 9 name required *capabilities* whose
*mechanisms* were spike-gated; most are now settled — see §12:

1. Inputs: topic, description, count, difficulty, style/mode controls.
2. Automatic grounding/research with source references (one adapter).
3. Draft generation; deterministic schema + domain validation.
4. Independent answer/key verification (fresh context, same provider/model).
5. Critique + exactly one bounded repair; fail-clear completion semantics.
6. Background near-duplicate gate against the chosen corpus.
7. Complete valid pack draft with provenance; draft lifecycle + no-junk rule.
8. Cost visibility and run info sufficient to diagnose a failure.
9. Provider profiles (4) with safe secret handling.
10. Pack schema `0.2.0` + migration/compatibility tests.
11. Evaluation matrix + fixture strategy (no live calls in checks).

**Deferred beyond V1** unless a later scope decision promotes them:
OAuth/PKCE; cross-provider verifier routing and capability matrix;
user-supplied documents/URLs; coverage planning and blueprints; full
span-level tracing and content-addressed prompt artifacts; pack-level judge
beyond the required chain; assurance-level taxonomy; broader multi-agent
research orchestration.

## 12. Open items gating "100% defined"

| Item | Home | Status |
| --- | --- | --- |
| Similarity backend | D-020 | **decided** — float32 BLOBs + Go-side cosine |
| Embedding source | D-018 | **decided** — follow the chat provider, fail clear where absent |
| Secret resolution order | D-019 | **decided** — environment first, keychain a desktop addition |
| OS-keychain library & platform matrix | spike #106 | open, no longer a gate |
| V1 research adapter selection | spike #120 | open — a self-hosted SearXNG is the *development* adapter only; the shipped V1 choice is a separate, maintainer-owned call |
| Page fetch + content extraction | spike #120 | open — §5 describes the port as search/fetch/evidence-build, but the landed adapter does **search only**. Extraction semantics are #120's to define, and Go's stdlib has no HTML parser, so building it would mean inventing the policy *and* spending dependency budget. Evidence is currently snippet-derived; the discriminating axis between V1 candidates is snippets-vs-extracted-content, not price |
| multi-select ≥2-vs-≥1 confirmation | #77 comment | open |
| Source-authority policy spec | with the research story | open |
| Style/mode intent enum | spike #105 (after pipeline exists) | open |

Everything else in this spec is settled at the product/architecture level
(D-015–D-017) and ready for the implementation-ticket hierarchy.
