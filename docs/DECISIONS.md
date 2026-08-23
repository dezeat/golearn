# golearn — DECISIONS

Append-only decision log. `docs/architecture.md` describes the _current_ state
of the system; this file records _why_ it got that way, one dated entry per
decision, newest at the bottom. Entries are **never edited after acceptance** —
a changed mind gets a new entry that supersedes the old one (mark the old entry
`superseded by D-NNN`; leave its text intact). The append-only rule protects
_content_, not presentation: a log-wide formatting normalization on 2026-07-26
restructured every entry for readability without altering substance.

## When a decision earns an entry

All three must hold (otherwise: just do it, no entry):

1. **Hard to reverse** — changing your mind later costs real effort (a data
   contract, a cross-cutting layout rule, a migration).
2. **Surprising without context** — a future reader would wonder "why on earth
   did they do it this way?"
3. **A genuine trade-off** — real alternatives existed and one was picked for
   specific reasons.

The explicit no-s matter as much as the yes-s: record a rejected alternative
when the rejection is non-obvious, so it isn't re-proposed in six months.

## Format

Entries use an `###` heading (an `##` heading gets a full-width underline in
GitHub's rendering, which turns the log into a wall of rules), bold inline
labels, and bullets for anything enumerable. Template for a new entry:

```md
### D-NNN — Short title of the decision

**Status:** accepted | superseded by D-NNN · **Date:** YYYY-MM-DD

**Context.** The situation and the forces that made this a choice, in a few
sentences.

**Decision.** What was picked; name the rejected alternative when the
rejection is the point. Break enumerable parts into bullets rather than
comma-chains.

**Consequences.**

- What this buys.
- What it costs.
- What it forbids.
```

Numbering: highest existing `D-NNN` + 1. Reference `architecture.md` sections
instead of repeating them.

---

### D-001 — CGo-free SQLite via `modernc.org/sqlite`

**Status:** accepted · **Date:** 2026-07-05

**Context.** golearn needs an embedded, file-based SQLite store, and the
popular driver (`mattn/go-sqlite3`) links the C library via CGo, which
requires a C toolchain and blocks static cross-compilation. Being a
local-first CLI shipped as prebuilt binaries for multiple platforms, a C build
step is disproportionate cost.

**Decision.** Use the pure-Go `modernc.org/sqlite` driver; never swap in a
CGo-based driver.

**Consequences.**

- The binary cross-compiles to any target with `GOOS`/`GOARCH` alone and CI
  needs no C toolchain.
- The cost is a larger binary and a modest per-query overhead versus the CGo
  driver — acceptable for a single-user interactive tool.
- Anyone reintroducing CGo breaks the release matrix.

### D-002 — Hexagonal layering with adapters that never import each other

**Status:** accepted · **Date:** 2026-07-05

**Context.** A learning-engine core (validation, hashing, selection, scoring)
must stay testable and swappable independently of SQLite, YAML parsing, and
the Bubble Tea TUI, which otherwise tend to entangle domain logic with I/O.

**Decision.** Adopt ports & adapters:

- `domain` — pure, stdlib-only;
- `ports` — interfaces;
- `app` — use cases over domain + ports;
- `adapters` — sqlite, pack, tui, localconfig;
- `cmd` — composition root;

with the hard rule that an adapter never imports another adapter and all
wiring happens in `cmd`.

**Consequences.**

- The domain and use cases test without a database or a terminal, and any
  adapter is replaceable behind its port.
- The cost is indirection and the discipline of injecting every dependency
  through the composition root.
- A `tui`-imports-`sqlite` shortcut is an architecture violation, not a
  convenience. See architecture.md "Hexagonal layout".

### D-003 — Manual `os.Args` CLI routing; a framework is deferred

**Status:** accepted · **Date:** 2026-07-05

**Context.** The command surface is small (`import`, `run`, `tui`, `export`,
`db reset`, `help`), and pulling in cobra or urfave/cli would add a runtime
dependency to a project that keeps them deliberately few.

**Decision.** Route commands with manual `os.Args` parsing in
`cmd/golearn/main.go` and a `switch`; parse global flags like `--db` at the
root before subcommand dispatch. Add commands by extending the switch, not by
adopting a framework.

**Consequences.**

- Zero CLI dependency and full control over parsing and help text.
- The cost is that flag handling is hand-rolled — a global flag registered on
  a subcommand is silently consumed by `os.Args` before the subcommand sees
  it, so root-level parsing is mandatory.
- Revisit cobra only if the surface grows significantly.

### D-004 — Import validation is all-or-nothing per file

**Status:** accepted · **Date:** 2026-07-05

**Context.** A pack file is a single authored unit; importing some questions
while rejecting others would leave a partially-applied pack that is hard to
reason about and hard to re-run idempotently.

**Decision.** Validate every question in a file before inserting any; a single
validation failure rejects the whole file. Directory imports process each file
independently, so one bad file does not sink its siblings.

**Consequences.**

- The database never holds a half-applied pack, and a failed import is safe to
  fix and retry.
- The cost is coarser feedback — one invalid question blocks its entire
  file — which is the intended contract, not a limitation to route around with
  partial inserts.

### D-005 — Determinism via an injected seeded `*rand.Rand`

**Status:** accepted · **Date:** 2026-07-05

**Context.** Question selection shuffles order, and export must be
byte-stable, yet tests need to assert exact orderings; the global `math/rand`
PRNG and `crypto/rand` are both process-global or non-reproducible and cannot
be pinned from a test.

**Decision.** Thread an explicit seeded `*rand.Rand` through every shuffle in
the selection and session code; never call the global PRNG or `crypto/rand`
for ordering.

**Consequences.**

- Same seed and same data yield byte-identical output, so selection and
  shuffle are asserted directly against fixed-seed fixtures.
- The cost is passing the RNG through call sites instead of reaching for a
  package global — a small, deliberate ergonomic tax that buys
  reproducibility.
- A test depending on wall-clock time or the global PRNG is a bug.

### D-006 — WAL journal mode enabled on every database open

**Status:** accepted · **Date:** 2026-07-05

**Context.** The TUI reads and writes concurrently (recording attempts while
rendering stats), and SQLite's default rollback journal serializes these into
"database is locked" errors. WAL is not reliably sticky — some operations can
reset the journal mode.

**Decision.** Issue `PRAGMA journal_mode=WAL` (and enable foreign keys) on
every `db.Open`, never assuming it persists from a prior session.

**Consequences.**

- Concurrent reader/writer access from the TUI stops deadlocking.
- The cost is the WAL sidecar files (`-wal`, `-shm`) that `db reset` must also
  clean up.
- Do not skip the pragma on the assumption it was set once before.

### D-007 — Stable content hashing for dedup, with a hash tie-break on export

**Status:** accepted · **Date:** 2026-07-05

**Context.** Re-importing an overlapping pack must not duplicate questions,
and export must be byte-reproducible — but questions imported in one batch
share an identical `created_at`, so ordering by timestamp alone is
non-deterministic.

**Decision.** Identify a question by a stable SHA-256 over its normalised,
null-byte-separated fields:

- topic slug, type, intro, prompt;
- ordered choices;
- sorted correct IDs;
- difficulty.

Enforce it as a UNIQUE constraint for dedup, and order exports by
`created_at ASC` with the content hash as the tie-break.

**Consequences.**

- Duplicate imports are skipped and export output is byte-identical across
  runs.
- The hash recipe is a data contract — changing normalisation, field order, or
  the separator silently changes every hash and breaks dedup — so it is frozen
  and any change is a new decision.
- Omitting the export tie-break reintroduces non-determinism whenever
  timestamps collide.

### D-008 — Presentation is computed at render time, never baked into pack data

**Status:** accepted · **Date:** 2026-07-05

**Context.** Choice display labels (`A`/`B`/`C`) and explanation correctness
prefixes ("Correct:", emoji) are presentation, but it is tempting to store
them in the pack — which would make labels wrong under shuffled display order
and would couple authored content to one renderer's styling.

**Decision.** Keep pack data content-only — `Choice.id` is an internal stable
identifier, never a display label, and explanations carry no prefixes; the TUI
derives labels from the current display order (`DisplayLabelForIndex`) and
adds prefixes at render (`FormatChoiceExplanation`, with
`StripExplanationPrefix` for legacy packs).

**Consequences.**

- Labels stay correct when choices are shuffled, and presentation styling
  changes without touching authored content or re-hashing.
- The cost is that authors and importers must resist encoding labels or
  prefixes into packs; doing so corrupts both the display and the content hash
  (D-007).

### D-009 — `AGENTS.md` is the single agent instruction file; `.agents/` the shared config tree

**Status:** accepted · **Date:** 2026-07-23

**Context.** The repo's agent harness was Claude-specific — instructions in
`CLAUDE.md`, skills and hooks under `.claude/`. Running a second agent (Codex)
meant either duplicating the instructions, which drift, or leaving the second
agent uninstructed. Meanwhile `AGENTS.md` has become the vendor-neutral
convention that Codex, Cursor, Copilot, Zed and others read, and Codex
discovers skills at `$REPO_ROOT/.agents/skills` natively.

**Decision.** The instructions live in root `AGENTS.md`; `CLAUDE.md` is
reduced to a single `@AGENTS.md` import line. Skills, hooks and subagents live
under `.agents/`, and `make agents` (`scripts/agents-link.sh`) generates the
gitignored vendor links `.claude/skills` and `.claude/agents`. A bare
`ln -s AGENTS.md CLAUDE.md` was rejected: the import survives a Windows clone
without Developer Mode and leaves room for Claude-only additions.

**Consequences.**

- One file to edit, and Codex reads both `AGENTS.md` and `.agents/skills` with
  no link at all.
- The cost is a setup step — a fresh clone or worktree must run `make agents`
  before Claude Code sees skills or subagents, which is why the links are
  generated rather than committed (a committed symlink breaks on clones
  without symlink support).

### D-010 — Review is delegated to subagents, split by scope

**Status:** accepted · **Date:** 2026-07-23

**Context.** Review quality depended on a single self-review step inside the
`pr` skill, performed by the same session that wrote the code — the session
least able to see what it missed. A reviewer with its own context window reads
the diff cold.

**Decision.** Two subagents under `.agents/agents/`, split by scope so neither
does the other's job:

- `architecture-reviewer` — judges the change against the binding docs
  (layering, determinism, CGo-free, decision and docs drift);
- `pr-reviewer` — judges correctness, standards, tests and PR conventions.

The `pr` skill delegates to both and falls back to self-review only when they
are unavailable. A single combined reviewer was rejected: one prompt covering
both scopes dilutes each, and the architecture pass needs to read the binding
docs in full before judging anything.

**Consequences.**

- Reviews get an independent context window, and the two prompts double as an
  executable statement of the repo's invariants.
- That is also the cost, since they must track `AGENTS.md` whenever the
  standards change.
- Subagents are Claude Code-only; an agent without them falls back to
  self-review, so the `pr` skill must keep that path working.

### D-011 — Security posture: trusted-operator, offline threat model

**Status:** accepted · **Date:** 2026-07-22

**Context.** golearn is a local-first, single-user, fully offline CLI/TUI with
no network path (CLAUDE.md core principle; `docs/architecture.md` §Purpose,
§Persistence). A pre-1.0.0 security audit needed an explicit threat model so
findings triage against a stated boundary rather than a generic web-app
checklist, which would flag non-issues (no auth, no encryption at rest) as
gaps.

**Decision.** Adopt a trusted-operator threat model — the machine owner and
local filesystem are inside the trust boundary; the only untrusted inputs are
imported pack files (YAML/JSON) and user-supplied paths (`--db`, import/export
targets). Required hardening is scoped to those inputs:

- parser resource limits;
- control-character sanitisation of pack text before storage/render;
- parameterised SQL;
- path validation on destructive operations.

Authentication, encryption at rest, config-file permissions, and process
sandboxing are explicit non-goals.

**Consequences.**

- The audit's security surface is bounded and its findings triageable; a
  future contributor cannot justify adding auth/encryption "for security"
  without superseding this entry.
- The cost is that a shared-account or multi-tenant deployment is out of scope
  by design — golearn protects against hostile _content_, not against a
  hostile local user.

### D-012 — Performance is an explicit non-goal at expected scale

**Status:** accepted · **Date:** 2026-07-22

**Context.** A pre-1.0.0 performance sweep found no egregious algorithmic
problems at the tool's expected scale (single user; hundreds to low-thousands
of questions per topic; interactive TUI): hot queries are index-served,
selection is O(n log n), the render loop does no DB work, and import memory is
bounded per file. The open question was whether 1.0.0 should commit to a
performance budget.

**Decision.** Treat performance as an explicit non-goal at expected scale. Do
not add caching layers, denormalised counters, or speculative indexes;
optimise only a measured regression that is user-visible at realistic data
sizes. Any optimisation must preserve the laws — determinism (D-005, D-007),
CGo-free (D-001), WAL (D-006), and the four-runtime-dependency ceiling.

**Consequences.**

- Contributors are spared premature-optimisation work and the review bar for a
  perf change is "prove it bites at realistic scale."
- The cost is that golearn makes no throughput promise; a future large-corpus
  use case (tens of thousands of questions across many topics) would reopen
  this as a new decision.
- The one known inefficiency (loading all rows to compute a topic count) is a
  deferred tidy-up, not a release gate.

### D-013 — 1.0.0 freezes data-at-rest contracts only; the CLI surface stays fluid

**Status:** accepted · **Date:** 2026-07-22

**Context.** Cutting 1.0.0 raises what semver's "public API" means for a local
CLI. Two contracts are expensive to change once users hold data — the DB
on-disk schema and the pack format (which embeds the D-007 content hash in a
UNIQUE constraint). The CLI/flag/UX surface is cheaper to evolve. A
conservative freeze (lock the CLI too) and a minimal freeze (data-at-rest
only) were both viable.

**Decision.** 1.0.0 freezes the data-at-rest contracts only:

- the DB schema (evolved forward via migrations, D-006/D-014);
- the pack format;
- the content-hash recipe (D-007, already frozen because it lives in every
  DB's dedup constraint).

The CLI/flag/UX surface stays fluid pre-2.0, changed with a deprecation path
rather than locked as a stable API. Because the port interfaces live under
`internal/`, threading `context.Context` through them is internal engineering,
not an API-freeze concern. The maintainer cuts 1.0.0 manually and retains
authority to adjust this scope up to release; a change after release is a new
superseding entry.

**Consequences.**

- Users' existing databases and packs are protected across 1.x, while the
  command surface can still improve toward a better UX before it, too, is
  locked at 2.0.
- The cost is that the CLI carries no stability promise at 1.0.0 — scripts
  binding golearn's flags may need updating within 1.x, mitigated by
  deprecation notices.

### D-014 — Incompatible-schema handling: fail loud, never destroy user data

**Status:** accepted · **Date:** 2026-07-22

**Context.** During development, an older-than-expected schema triggers a
silent drop-recreate-reseed on startup (`docs/architecture.md` §Persistence) —
total, unannounced loss of every profile, session, and attempt.
`architecture.md` already marks this a dev-only shortcut to remove before
public release; the audit confirmed it destroys data with no prompt, backup,
or log, and that there is no "schema newer than expected" branch at all.

**Decision.** Replace the drop-recreate-reseed with a fail-loud policy:
golearn never silently destroys user data.

- Compatible older schemas are migrated forward in place (data-preserving
  `ALTER`/backfill, each migration atomic with its version bump).
- A genuinely incompatible or newer schema causes golearn to refuse to open
  and print an actionable error directing the user to
  `golearn db reset --yes` (the one explicit, consented destructive path).
- The `resetSchema` / `ensureCompatibleSchema` startup path is deleted.

**Consequences.**

- A user upgrading or downgrading the binary can never lose data to an
  implicit reset; the worst case is a clear refusal with a documented
  recovery.
- The cost is real migration work — forward-only, atomic migrations plus an
  incompatibility gate — and that destructive recovery becomes an explicit
  user action rather than an automatic convenience.

### D-015 — Forge ships as a second binary from a nested addon module; the offline law is binary-scoped

**Status:** accepted · **Date:** 2026-07-26

**Context.** Epic #66 adds provider-backed question generation — the first
network-capable functionality in a project whose core principle is "no network
path by design". Provider SDKs in the root module would break the
four-runtime-dependency ceiling, and a build tag cannot prevent that: tagged
files still land their imports in the module graph and `go.sum`. Mixing
generation into the `golearn` binary would put a network path into the offline
product (#101, #66).

**Decision.** Ship two binaries:

- `golearn` — the existing offline Core + TUI, unchanged;
- `golearn-forge` — the same application plus the authoring extension.

Forge is a nested add-on module with its own `go.mod` — expected under
`addons/forge`, renaming the map's provisional `addons/authoring` — and it
owns every provider SDK, HTTP client, and retrieval dependency. It imports
Core/shared-TUI contracts strictly one-way: the Core never imports or knows
Forge. A single shared module and build-tag isolation were rejected for the
reasons above.

The offline law is rescoped from key-scoped to binary-scoped:

- `golearn` retains no network path at all;
- Forge's network activity is authoring-time only — import, selection,
  practice, export, and stats stay offline in both binaries;
- the binary scoping matters because a local provider endpoint (Ollama) needs
  no API key — "no key configured ⇒ no network" was never the real boundary.

**Consequences.**

- The core `go.mod` stays at four runtime deps, CGo-free and cross-compiling,
  and a `golearn`-only install behaves exactly as 0.x.
- The cost is two modules kept in step (a local `go.work` for joint
  development), a second release artifact, and the discipline that generated
  output re-enters the same deterministic import pipeline (D-004, D-005,
  D-007), whose guarantees are unchanged.
- `docs/architecture.md` gains its authoring-boundary section — and
  `AGENTS.md` its amended offline principle and network anti-pattern — when
  the Forge module actually lands; those files describe what is true now, so
  until then this entry is the binding statement.

### D-016 — Generated packs are accepted at pack level; trust comes from the generation pipeline

**Status:** accepted · **Date:** 2026-07-26

**Context.** Epic #66 originally required mandatory human review before
generated content enters the store, leaving open where the gate sits (#102):
per question, per pack, or only at publish. A 20-question pack behind a
per-question gate is 20 judgements — friction that pushes users back to manual
authoring — while gating only at publish would let unreviewed content into the
local practice corpus and sit badly with all-or-nothing import (D-004).

**Decision.** The gate is pack-level. Generation returns one complete,
validated pack with exactly the requested number of questions; the user's
actions are:

- **accept** — "Add to library", which invokes the standard atomic import path
  internally;
- **regenerate** — discard the result and run generation again;
- **discard**.

Per-question inspection or editing is an optional escape hatch, never a
required step. Nothing is imported merely because generation finished, and
nothing is auto-published to the canonical pack repository. Mandatory
per-question review and the publish-only gate were rejected.

**Consequences.**

- Trust moves from human inspection to the pipeline — grounding in retrieved
  evidence, deterministic validation, independent verification, critique,
  near-duplicate gating, bounded repair, and fail-closed behaviour when the
  pipeline cannot deliver.
- Those stages are therefore V1-mandatory: cutting them for scope reopens this
  decision, because they are what replaced the human gate.
- Unresolved drafts must be explicitly resolved (view / add / discard) before
  new generation starts — the no-junk rule.

### D-017 — Pack schema evolves via 0.2.0, frozen as 1.0.0 at release; generation metadata stays out of the hash

**Status:** accepted · **Date:** 2026-07-26

**Context.** Forge-generated packs need pack-level metadata — what was
requested and where content came from — that the current `0.1.0` pack schema
does not carry. D-013 freezes data-at-rest contracts at product 1.0.0, and the
D-007 hash recipe is already frozen (D-013), so schema evolution needs an
explicit staging plan rather than ad-hoc field growth.

**Decision.** Introduce a backwards-compatible pack schema `0.2.0` during
Forge implementation:

- `0.1.x` packs remain importable; Forge exports `0.2.0`;
- the importer/exporter carry an explicit compatibility policy with migration
  tests;
- `0.2.0` stays deliberately adjustable through prototype and evaluation
  feedback; at the product V1 release its final shape is promoted and frozen
  as pack schema `1.0.0`.

New pack-level fields split into:

- a structured `generation_spec` — every user-visible, content-shaping input:
  topic, optional description, requested count, difficulty, style/mode,
  language;
- durable provenance — generation time, provider/model identity, source
  references.

Excluded from packs categorically: secrets, raw prompts and raw model/tool
output, retry/repair counters, and provider request mechanics — those live
only in minimal local run history. The D-007 hash recipe is untouched:
pack-level metadata never feeds the per-question content hash.

**Consequences.**

- Forge-capable packs exist before the freeze, with room to learn from real
  prototype output; product version and schema major are intentionally
  decoupled until release.
- The cost is a compatibility matrix to test and an intent (style/mode)
  taxonomy that must be evaluation-gated before it freezes.
- Dedup behaviour is identical across `0.1.x` and `0.2.0` imports because the
  hash inputs do not change.

### D-018 — Embedding is an optional provider capability, not a `Provider` method

**Status:** accepted · **Date:** 2026-08-23

**Context.** Forge's similarity gate needs embeddings, and FORGE.md §7 leaves
the source open: "the recommendation must state the embedding source per
provider profile, the behaviour when none is available". The behaviour-when-
none case is not hypothetical or transitional — **Anthropic ships no
embeddings API at all**, so one of the four mandated V1 profiles can never
produce a vector however it is configured. The obvious shape, an `Embed`
method on the single provider port with a sentinel error for the profiles
that cannot honour it, makes a permanent product fact look like a runtime
failure and defers discovering it to the moment a user asks for a pack.

**Decision.** Model embedding as a **separate, optional interface**
(`ports.Embedder`) that a provider adapter either implements or does not.

- `ports.Provider` carries chat/structured generation only. All four V1
  profiles satisfy it.
- `ports.Embedder` is satisfied only by profiles that actually expose an
  embeddings endpoint. The Anthropic adapter does not implement it, so its
  absence is a **compile-time fact** rather than a runtime branch.
- `domain.ErrNoEmbeddingCapability` is the typed form for the pipeline's
  fail-clear path, checked *before* a strategy is chosen rather than
  discovered mid-run.
- Rejected: a `Capabilities` struct alongside the interface. Two sources of
  truth for the same fact can disagree, and the flag is the one that would
  drift.

**Consequences.**

- The similarity gate must decide its behaviour for a no-embedding profile
  explicitly, because it cannot obtain an `Embedder` to call in the first
  place.
- #123's acceptance wording — "all four profiles satisfy one port contract" —
  needs reading with care and is recorded as such: **Anthropic satisfies the
  chat contract, not the embedding one.** That is the design, not a gap in it.
- The cost is two interfaces where one looks simpler, and a type assertion at
  the seam that binds them.
- A test asserting the Anthropic adapter does *not* satisfy `Embedder` fails
  the moment someone bolts on a stub that pretends otherwise.

### D-019 — Forge resolves secrets from the environment first; the OS keychain is a desktop addition

**Status:** accepted · **Date:** 2026-08-23

**Context.** FORGE.md §6.2 states the direction as "keys live in the **OS
keychain**; environment variables override for automation", gated on spike
#106. #106 has not reported. Meanwhile the environments golearn is actually
developed, tested and run in for this work — containers, headless Linux, CI,
SSH sessions — are precisely the ones where no Secret Service exists, which
#106's own scope calls out as the risk. A keychain-first policy would make
the fallback the common path and the primary path the exception, and every
keychain library that could implement it is a **new dependency** in a repo
whose minimal-footprint rule requires one to be justified rather than
assumed.

**Decision.** Invert the emphasis: **the environment is the primary
resolution path**, and an OS keychain is a desktop convenience layered on
later.

- Precedence is environment → keychain → none, expressed in the resolver
  implementation where it can be tested, not in the interface.
- `domain.SecretOrigin` records which source supplied the active credential,
  so a user can ask "where is my key coming from?" and get an answer that is
  not the key.
- No keychain library is adopted. The `OriginKeychain` constant exists so the
  precedence rule is written and tested against the full order rather than
  retrofitted, but no adapter supplies it; the library choice stays #106's
  deliverable.
- This is recorded as an **explicit, documented deviation from FORGE.md
  §6.2**, not a silent flip. FORGE.md's §6.2 wording is superseded by this
  entry under the precedence rule (DECISIONS.md wins over FORGE.md).

**Consequences.**

- Forge works identically headless and on a desktop, and the path every test
  exercises is the path most runs take.
- Zero new dependencies for V1 secret handling.
- The cost is that desktop users get no keychain integration at V1 and must
  supply credentials through the environment, which is a weaker convenience
  story than §6.2 promised.
- Credentials are never persisted to SQLite, packs, drafts, logs or
  diagnostics. `domain.Secret` refuses to render its value under **every**
  formatting verb, `internal/config`'s shape-based guard covers output that
  never passes through that type, and neither is a substitute for the other.

### D-020 — Similarity uses float32 embedding BLOBs in the existing database with Go-side cosine

**Status:** accepted · **Date:** 2026-08-23

**Context.** FORGE.md §7 sets a backend priority order — a vector path in the
pure-Go SQLite stack, then a native-Go alternative, then Go-side scoring —
under hard constraints: no CGo, no native SQLite extensions (`sqlite-vec` is
off the table under D-001), no vector database or server, no per-platform
shared libraries. The constraints eliminate the first two rungs in practice,
and D-012 makes performance an explicit non-goal at golearn's scale (single
user, hundreds to low-thousands of questions per topic).

**Decision.** Store embeddings as **little-endian IEEE-754 float32 BLOBs in
the existing golearn SQLite database** and compute cosine similarity in Go.

- No vector database, no ANN index, no second database, and **no new
  dependency** — cosine is arithmetic.
- `float32` rather than `float64`: embedding models emit float32, so float64
  would store converted precision that never existed, at twice the bytes in a
  user's database. Accumulation inside the cosine is float64, because summing
  several hundred float32 products in float32 loses enough precision to move a
  score across a threshold.
- Byte order is explicit little-endian, because a database file is a portable
  artifact and a user moving one between architectures must not read their
  embeddings back as noise.
- Vectors from different embedding models are not comparable, so the model
  identity is stored with every vector and a mixed search is refused rather
  than scored.
- The backend sits behind `ports.SimilarityIndex`, which exchanges **vectors,
  never providers**: the index must not know how a vector was produced, and
  the embedding source must not know what it will be compared against.

**Consequences.**

- The similarity gate adds no operational surface: no server to run, no index
  to rebuild, no platform-specific artifact, and the CGo-free cross-compile
  matrix is untouched.
- The argument rests entirely on the corpus staying small, so it is benchmarked
  **adversarially** — the benchmark in `docs/design/FORGE-EXPERIMENTS.md`
  Part C looks for the corpus size at which brute force stops being
  irrelevant, rather than confirming that it is.
- The cost is a linear scan whose cost grows with the corpus; the swap to an
  indexed backend is a port implementation, not a change to pipeline logic.
- A future large-corpus use case reopens this as a new entry, exactly as D-012
  anticipates.
