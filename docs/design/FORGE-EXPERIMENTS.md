# Forge — Experiment & Benchmark Log

**Status: working evidence log for epic #66.** This file records *what was
measured* while building Forge, so a design choice can be re-checked instead of
re-argued. `docs/DECISIONS.md` records what was decided and why;
`docs/architecture.md` records what is true now. This file records the
observations that justify both, and it is append-only within a section.

## Why this file exists

Forge's implementation runs on a deliberate discipline: **TDD where the answer
is known, experiment where it is not.**

- A failing test is a falsifiable prediction about behaviour that should exist.
  Where the required behaviour is already specified — schema validation,
  migrations, hash invariance, draft lifecycle — the test *is* the
  specification, and it is written first.
- Where the answer is genuinely unknown — how the Go toolchain resolves a
  nested module, whether a local model returns parseable JSON — writing the
  test first would encode a *guess* as a specification. There the order is
  **probe → observe → lock the observation into a regression test.**

Two rules keep that honest:

1. **Every guard must be shown to fail.** A test that has never gone red is not
   evidence; it may assert nothing. Mutation results are recorded alongside the
   green run.
2. **Thresholds are committed before the measurement run.** For anything
   non-deterministic, the pass criterion is written down first, or it silently
   becomes "whatever the run produced".

Each entry below follows: **Question → Hypothesis → Method → Result →
What it locked in.**

---

## Part A — Build & boundary experiments

These settled the shape of #125 before any implementation was committed.

### A-1 · Does `go test ./...` reach a nested module?

- **Question.** With a `go.work` tying the root module and `addons/forge`
  together, does the root's `go test ./...` traverse into the nested module?
- **Hypothesis.** It does not; `./...` is module-scoped, not workspace-scoped.
- **Method.** Minimal two-module reproduction in a scratch tree: root module
  with one testable package plus a nested module with its own testable package,
  joined by `go work init . ./addons/forge`. Ran `go test ./...` from the root.
- **Result.** **Confirmed — the nested module is not reached.** Output listed
  only the root package and its `cmd`; the nested module's passing test never
  ran.
- **What it locked in.** `make test`/`make check` and CI must invoke each module
  explicitly. Had this not been probed, every Forge test would have been
  silently absent from a green gate — the most dangerous possible failure, since
  the gate would have *reported* success.

### A-2 · Does committing `go.work` break the standalone core?

- **Question.** Does a committed workspace file break a clean-checkout build of
  the root module, or `go install <core>@latest`?
- **Hypothesis.** `GOWORK=off` restores standalone resolution; `go install`
  with a version suffix is unaffected because it ignores the local module
  context entirely.
- **Method.** Built the root module with `GOWORK=off` in the same reproduction;
  read `go help install` for the versioned-install contract.
- **Result.** **Both hold.** `GOWORK=off go build ./...` succeeds. `go install`
  with a version suffix "builds packages in module-aware mode, ignoring the
  go.mod file in the current directory or any parent directory".
- **What it locked in.** `go.work` stays **gitignored and optional**, matching
  FORGE.md §2's wording ("a local `go.work`") and the repo's existing
  `.gitignore`. Independence is carried by a `replace` directive in the addon
  instead, which required generating the addon's own `go.sum` — without it the
  addon could not resolve the core's transitive dependencies outside a
  workspace, and that failure was only visible once `GOWORK=off` was tried.
  The boundary guard sets `GOWORK=off` explicitly so a developer's workspace
  can never mask a leak that CI would catch.

### A-3 · Can the nested Forge module import the core's `internal/`?

- **Question.** Go's `internal/` rule is path-prefix based, but does a **module**
  boundary also block it? If it does, #125 must additionally expose a public
  API surface for Forge to consume — a materially larger story.
- **Hypothesis.** Access is granted on import-path prefix, not module identity,
  so `<core>/addons/forge` may import `<core>/internal/...`.
- **Method.** Three modules in one workspace: root (with `internal/secret`), a
  nested module at the root's path prefix, and a control module at an unrelated
  path. Built both consumers.
- **Result.** **Confirmed, with a passing negative control.**
  - `example.com/root/addons/forge` → build OK.
  - `example.com/outsider` → `use of internal package example.com/root/internal/secret not allowed`.
- **What it locked in.** Forge reuses core internals directly; no public API
  surface is needed for #125. The control matters: without it, the positive
  result could have come from the workspace short-circuiting the check rather
  than the prefix rule.
- **Open risk.** Verified under a `go.work` replace. The same must hold when
  `addons/forge` resolves the core from the module proxy at release time —
  tracked as a CI check rather than assumed.

### A-4 · Is "the core imports no network package" actually true?

- **Question.** The obvious boundary guard is "no network packages in the core".
  Is that assertion true today?
- **Hypothesis (going in).** True — the core is offline by design.
- **Method.** `go list -deps ./cmd/golearn` and `./...`, filtered for network
  packages; then traced the importer of each hit; then grepped first-party
  source for direct network imports.
- **Result.** **Hypothesis falsified.** The core binary's transitive graph
  already contains `net` and `net/url`. Both enter through
  `github.com/google/uuid`, required transitively by `modernc.org/sqlite`.
  `net/http` is **absent**. No first-party package imports any of them.
- **What it locked in.** The naive guard would have failed on its first run and
  invited someone to weaken it. The guard that shipped instead asserts three
  narrower, true things (see `internal/boundary`):
  1. `net/http` and friends absent from the core binary's graph — every provider
     SDK pulls `net/http`, so this is the real leak detector;
  2. the root `go.mod`'s direct requirements are exactly the four D-015 fixes;
  3. no *first-party* package imports a network primitive directly.
- **Note.** This is the clearest case in the log for probing before asserting. A
  test-first guard here would have encoded a false specification.

### A-5 · Mutation test — can the guards actually fail?

- **Question.** Every guard passes. Do they assert anything, or are they
  vacuous?
- **Method.** Four deliberate mutations, each reverted immediately after the
  observation.
- **Result.** **All four caught, each with an actionable message.**

  | # | Mutation | Guard(s) that fired | What the message said |
  | --- | --- | --- | --- |
  | 1 | first-party `net/http` import | `TestCoreBinaryHasNoHTTPPath`, `TestFirstPartyCodeImportsNoNetwork` | named `net/http` *and* the `crypto/tls` it drags in, cited D-015 |
  | 2 | fifth direct dependency in `go.mod` | `TestCoreModuleStaysAtFourRuntimeDeps` | named the module, pointed at `addons/forge` |
  | 3 | core package imports the addon | `TestCoreNeverImportsForge` | named the one-way rule and that resolution fails at load time |
  | 4 | config output echoes `OPENAI_API_KEY` | `TestReportLeaksNoCredentialShapes`, `TestReportSucceedsWithProviderEnvSet` | matched the credential shape and caught the sentinel value |

- **What it locked in.** The guards are load-bearing. Reverting each mutation
  restored green, so none of them is passing by accident.
- **Secondary finding (mutation 3).** The core module has no requirement on the
  addon, so a core file importing Forge fails to *resolve* before any assertion
  runs. The one-way dependency rule is therefore enforced structurally by Go
  itself; the test is belt-and-braces on top of that. The guard was given an
  explicit message for this case so a future contributor meets the rule rather
  than a bare toolchain error.

### A-6 · Does the credential redaction guard survive a hostile environment?

- **Question.** Forge's config output is the surface most likely to grow a
  credential by accident — "which provider am I using" and "with what key" are
  one careless line apart. Does the guard catch it?
- **Hypothesis.** A shape-based guard catches a leak that a name-based
  allowlist would miss, because it does not need to know the provider.
- **Method.** Set `OPENAI_API_KEY`, `ANTHROPIC_API_KEY` and
  `OPENROUTER_API_KEY` to a sentinel, then assert the report contains neither
  the sentinel nor anything matching a credential shape. Mutation 4 above is
  the falsification run.
- **Result.** **Confirmed.** Both guards fired on the mutation; the shape guard
  matched on the pattern alone, without knowing which provider was involved.
- **What it locked in.** The guard is written against *shape*, not against
  today's provider names — a name-based check stops working the moment a new
  provider is added, which is exactly when it is most needed. It is in place
  before #123 introduces real credentials, rather than after.

### A-7 · Where do Forge's migrations live, and what does the core do with them?

- **Question.** Forge extends the *existing* database (FORGE.md §8). The core
  tracks schema state in `schema_migrations`, keyed by a position-derived
  version (`version := i + 1` in `internal/adapters/sqlite/db.go`). Does Forge
  append to that counter, or own a separate one? And what does today's core
  binary actually do when it opens a database a Forge binary has already
  extended — #121's first acceptance criterion turns on that answer.
- **Hypothesis (going in).** A shared counter is the simpler design and the
  core already fails loud on an unrecognised schema (D-014), so the shared
  counter is safe.
- **Method.** Four throwaway probes against the real `Open()` path, each
  seeding a database, mutating it the way a Forge binary would, reopening it
  with the unmodified core, and reading back what survived. The probe was
  deleted after the observation; the regression tests that replace it are
  named below.
- **Result.** **Hypothesis falsified twice, in different places.**

  | # | Scenario | Observed |
  | --- | --- | --- |
  | P1 | Forge owns `forge_schema_migrations`; adds only new tables | core reopens fine, core rows intact, Forge tables untouched |
  | P2 | Forge appends version 3 to the shared `schema_migrations` | **core reopens silently** — there is no newer-schema gate at all |
  | P3 | Forge claimed v3; the core later ships its own v3 | **the core's own migration is silently skipped** — `APPLIED=false`, no error |
  | P4 | Legacy tables without `users` (the pre-existing branch) | **1 row in → 0 rows out, no error, no prompt** |

- **What it locked in.**
  1. **Forge gets its own registry, `forge_schema_migrations`.** P3 is the
     falsifier for the shared counter and it is the worst class of bug
     available here: the core would *believe* it had migrated. The existence
     check is per-version (`WHERE version = ?`), so a version number another
     module already claimed reads as "already applied". A shared counter is
     not merely untidy, it is silently lossy the first time the two modules
     ship a migration each.
  2. **A Forge-extended database is *compatible*, not *newer*, for the core**
     (P1). Forge adds tables and never touches core tables, so the core opens
     it and works — which is what D-015's "the offline product is unchanged"
     requires. This is a deliberate reading of #121's acceptance bullet
     ("fails clearly under a core-only binary according to D-014"): D-014's
     behaviour for a compatible schema *is* to open. The bullet binds the
     genuinely-incompatible case — a Forge release that alters a core table —
     and that case is now detectable because of (3). Precedence is
     DECISIONS.md > FORGE.md > issues, so D-014's text governs the issue's
     phrasing. The Forge store commit carries the guard that holds Forge to
     it: a test asserting no Forge migration alters a core table.
  3. **D-014 is not implemented, and the gap is load-bearing for #121.** P2
     shows no newer-schema gate exists; P4 shows the drop-recreate path D-014
     ordered deleted is still live and still destroys data silently. #121
     cannot honour its first acceptance criterion on top of either. Both are
     fixed in their own commit, ahead of the Forge tables, with P2 and P4 as
     the regression tests.
- **Note on method.** Every one of these is an *environment* unknown, not a
  specification — the answer lives in how `migrate()` and `schemaNeedsReset()`
  behave, not in what anyone intended. Writing P3 as a test-first assertion
  would have encoded the going-in hypothesis, which was wrong.

### A-8 · Mutation test — do the D-014 schema guards actually fail?

- **Question.** A-7's probes justified replacing the drop-recreate path with a
  refuse-and-leave-intact guard. The replacement's tests are green. Do they
  assert anything?
- **Method.** Five deliberate mutations of `internal/adapters/sqlite`, each
  reverted immediately after the observation, with a control run afterwards to
  confirm the revert.
- **Result.** **All five caught, each by exactly the intended guard.**

  | # | Mutation | Guard(s) that fired |
  | --- | --- | --- |
  | 1 | `guardSchemaCompatibility` returns nil unconditionally | all four refusal guards |
  | 2 | refusal message drops the recovery hint | `TestSchemaRefusalNamesTheConsentedRecoveryPath` |
  | 3 | newer-schema gate removed | `TestNewerSchemaRefusesToOpen` |
  | 4 | required-column gate removed | `TestTrackedDatabaseMissingARequiredColumnIsRefusedNotReset` |
  | 5 | legacy branch drops tables, *then* refuses | `TestPopulatedLegacyDatabaseSurvivesAnOpenAttempt` |

- **What it locked in.** Mutation 5 is the one that matters and it was written
  deliberately: mutations that simply stop refusing fail on the "must refuse"
  assertion and never reach the row count, so they would leave the
  data-survival claim — the entire point of D-014 — untested. A mutation that
  destroys data *and still returns the correct error* is what proves the row
  count is load-bearing. A guard that only checks the error type would have
  passed it.

### A-9 · Mutation test — do the Wave 0 contract guards actually fail?

- **Question.** The port freeze ships value types carrying real behaviour: the
  prompt-injection fence, credential redaction, cosine, the BLOB encoding, and
  the similarity comparison representation. All tests green. Do they bite?
- **Method.** Nine mutations of `addons/forge/internal/domain`, each reverted
  immediately, with a control run afterwards.
- **Result.** **All nine caught, each by exactly the intended guard.**

  | # | Mutation | Guard that fired |
  | --- | --- | --- |
  | 1 | fence stops neutralizing sentinels in content | `TestFencedContentCannotCloseItsOwnFence` |
  | 2 | fence stops neutralizing the id | `TestFencedIdCannotForgeADelimiter` |
  | 3 | `UntrustedText` loses `Format` | `TestUntrustedTextNeverRendersItsContentUnderAnyVerb` |
  | 4 | `Secret` loses `Format` | `TestSecretNeverRendersItsValueUnderAnyVerb` |
  | 5 | cosine scores incomparable vectors as 0 | `TestCosineRefusesIncomparableInputRatherThanScoringIt` |
  | 6 | canonical text includes the intro | `TestCanonicalTextIgnoresIntroAndRationale` |
  | 7 | canonical text stops sorting options | `TestCanonicalTextIsInvariantToPresentationOrder` |
  | 8 | canonical text drops the correctness marker | `TestCanonicalTextDistinguishesTheCorrectAnswer` |
  | 9 | vector blob written big-endian | `TestVectorBlobEncodingIsLittleEndianIEEE754` |

- **Bug found by the tests, not by the mutations.** The redaction test was
  written to cover every formatting verb rather than the obvious one, and it
  failed on first run: `%#v` printed `domain.UntrustedText{value:"..."}` and
  the mismatched verb `%d` printed `{%!d(string=...)}`. **A `String()` method
  is not a redaction boundary** — `%#v` ignores `Stringer` entirely and prints
  unexported fields verbatim. Both types now implement `fmt.Formatter`, which
  is verb-agnostic and therefore total. Had the test covered only `%s` and
  `%v` it would have been green, and the leak would have shipped inside the
  very type built to prevent it.
- **What it locked in.** Redaction and fencing are asserted against *every*
  verb, not the ones a developer would naturally reach for. Mutations 3 and 4
  exist to keep it that way: removing `Format` restores exactly the leak the
  first run exposed.

### A-10 · Mutation test — the schema 0.2.0 compatibility and hash guards

- **Question.** D-017's compatibility promise ("0.1.x stays importable, Forge
  exports 0.2.0") and its hash promise ("pack-level metadata never feeds the
  per-question content hash") are now enforced by tests. Do the tests bite?
- **Method.** Seven mutations of `internal/domain` and the pack adapter, each
  reverted, with a control run afterwards.
- **Result.** **All seven caught.**

  | # | Mutation | Guard that fired |
  | --- | --- | --- |
  | 1 | compatibility reverts to an exact-minor match | `...AcceptsOlderMinorsAndRefusesNewer/0.1.0`, `/0.1.7`, plus the pre-existing pack-version test |
  | 2 | minors above the ceiling accepted | `...AcceptsOlderMinorsAndRefusesNewer/0.3.0` |
  | 3 | a newer major accepted | `...AcceptsOlderMinorsAndRefusesNewer/1.0.0` |
  | 4 | `source_ref` and `confidence` fed into the content hash | `TestTheSameQuestionHashesIdenticallyAcrossSchemaVersions` |
  | 5 | generated confidence raised to the manual default | `TestGeneratedConfidenceIsStrictlyBelowTheManualDefault` |
  | 6 | `style` validated against a closed enum | `TestUnknownStyleIsAcceptedRatherThanValidated` |
  | 7 | pack metadata dropped from the wire format tags | `TestGeneratedPackParsesAndValidates` |

- **A flaw in the mutation harness itself, worth recording.** Mutations 2 and 3
  first appeared *uncaught*. They were not: both had made the package fail to
  **compile**, and the harness only grepped for `--- FAIL` lines, so a build
  failure and a surviving mutation looked identical. A mutation that does not
  compile is not evidence either way — it tests nothing and must not be
  counted as caught. The harness now reports a build failure explicitly, and
  both mutations were rewritten to be semantically valid (`> ceiling+10`,
  `major < supported`) before they became evidence. **A green mutation report
  is only as trustworthy as its ability to tell "the guard held" from "the
  experiment never ran"** — the same failure mode as A-1, one level up.

### A-11 · Mutation test — the run, draft and migration-isolation guards

- **Question.** #121's acceptance rests on claims that are easy to state and
  easy to lose: Forge never touches core tables, a draft is never library
  content, only a succeeded run may produce one, and acceptance runs the
  standard import. Do the guards hold them?
- **Method.** Ten mutations of `addons/forge`, each reverted, control after.
- **Result.** **All ten caught.**

  | # | Mutation | Guard that fired |
  | --- | --- | --- |
  | 1 | a Forge migration `ALTER`s the core `questions` table | `TestForgeMigrationsNeverTouchCoreTables` |
  | 2 | Forge writes into the core's migration registry | the whole store suite — see below |
  | 3 | a draft accepted from a non-succeeded run | `TestOnlyASucceededRunMayProduceADraft` (all three states) |
  | 4 | draft pack no longer validated | `TestDraftMustBeAValidGeneratedPack` |
  | 5 | generated-confidence gate neutered | `TestDraftMustBeAValidGeneratedPack/{omits,claims hand-authored}` |
  | 6 | provenance no longer required | `TestDraftMustBeAValidGeneratedPack/missing_provenance` |
  | 7 | `DeleteDraft` stops being idempotent | `TestDeletingADraftIsIdempotent`, `TestReacceptingADraftDuplicatesNothing` |
  | 8 | acceptance deletes the draft *before* importing | all three acceptance tests |
  | 9 | drafts listed newest-first | `TestDraftsListOldestFirst` |
  | 10 | `FinishRun` accepts "running" as terminal | `TestFinishingARunRequiresATerminalStatus` |

- **Mutation 2 reproduced A-7's P3 in situ, which is the point of running it.**
  Pointing Forge at `schema_migrations` did not produce a clash or an error —
  Forge's version 1 was already recorded by the core, so Forge's migration was
  treated as applied and **silently skipped**. Its tables were never created,
  and fifteen tests failed on missing tables rather than on anything resembling
  the actual cause. In production the same mechanism runs the other way and is
  worse: it is the *core's* next migration that gets skipped, silently, with
  every table still present. The scratch probe and the real code agree.
- **A bug the tests found, not the mutations.** The end-to-end lifecycle test
  asserted that an accepted draft's questions reach the library carrying
  generated provenance. It failed: the questions arrived at confidence **1.0**,
  indistinguishable from hand-authored content, because the importer's default
  is 1.0 and the draft simply omitted the field. D-017 requires generated
  questions to sit strictly below the manual default. The rule now lives in
  `SaveDraft` rather than in the generator: a generator that forgets is a bug
  that ships, whereas a store that refuses is a bug that cannot. Mutation 5
  keeps it honest.
- **A note on invalid mutations, again.** Mutation 5's first form
  (`if false {`) left a declared-and-unused variable and did not compile — the
  A-10 failure mode exactly. It was rewritten to an unreachable threshold
  (`*c > 2.0`) before it counted. Recording it a second time because the
  temptation each time is to read the build failure as a caught mutation.

---

## Part B — Provider & model benchmarks

**Scope discipline (#130).** These measurements establish *mechanism* —
wire-format compatibility, structured-output parseability, timeout and
cancellation behaviour, throughput. One model on one machine proves nothing
about pack quality across providers, and #130's non-goals say so explicitly.
Quality claims are reported per model and never generalised.

**Privacy.** Endpoints, hostnames, network topology and account identity are
never recorded here. Models are named because a model identifier is safe to
disclose; the deployment that served it is not.

### B-0 · Environment under test

| Property | Value |
| --- | --- |
| Class | Operator-managed local inference host, CPU-only |
| CPU | Intel N100, 4 cores / 4 threads |
| GPU | none (integrated display adapter only) |
| RAM | 15 GiB total, ~12 GiB available to inference |
| Runtime | Ollama, upgraded 0.13.5 → 0.32.15 during setup |

The upgrade was needed because the installed runtime refused current model
manifests (HTTP 412, "requires a newer version of Ollama"). Post-upgrade the
host's own service health suite ran 120 passed / 1 failed / 6 skipped; the
single failure was proven pre-existing and unrelated by diffing the host's
working tree against its committed configuration.

### B-1 · KPI definitions

Committed **before** any measurement run, so no threshold can drift to fit a
result. The pipeline's viability rests on the first metric far more than the
rest: everything downstream assumes a parseable candidate.

| KPI | Definition | Why it matters |
| --- | --- | --- |
| `structured_valid_rate` | share of responses parsing as schema-valid JSON with no repair | D-016's pipeline replaced human review; unparseable output collapses it |
| `answer_key_accuracy` | share where the marked correct answer is genuinely correct, rubric-scored | the single worst defect a practice tool can ship |
| `grounding_fidelity` | share of claims supported by the supplied evidence records | separates grounded generation from fluent invention |
| `distractor_quality` | rubric score: plausible-but-wrong vs obviously wrong | decides whether a question tests anything |
| `repair_rate` | share of candidates needing the one permitted repair | calibrates the bounded-repair budget |
| `rejection_rate` | share discarded by validation / verification / critique / similarity | high values mean the model cannot meet the bar at that count |
| `intra_pack_dup_rate` | near-duplicate collisions within one generated pack | drives similarity-gate thresholds |
| `latency_total` | wall-clock per completed pack | product viability on modest hardware |
| `throughput` | generated tokens per second | comparability across models and hosts |
| `cancel_latency` | time from cancel signal to released request | a hung cancel is a UX and resource defect |
| `cost_per_pack` | provider tokens billed (zero for local inference) | the cost figure shown at the preview surface |

### B-2 · Results

*Pending. Populated by the #130 live lane once the pipeline slices land. Each
row records the model identifier, the sample size, and the pre-registered
threshold it is judged against.*

---

## Part C — Application benchmarks

Measurable properties of the product itself, tracked so a regression is visible
rather than inferred.

| Metric | Why it is tracked | Baseline |
| --- | --- | --- |
| core binary size | D-015 promises an unchanged offline product | *pending* |
| forge binary size | the cost of the authoring capability | *pending* |
| core direct dependencies | the four-dependency ceiling; guarded by `internal/boundary` | **4** |
| `net/http` in core graph | the offline law's leak detector | **absent** |
| `make check` wall time | the gate must stay fast enough to be run | *pending* |
| similarity scan vs corpus size | **tests the no-vector-index claim** | *pending* |
| embedding bytes per 1000 questions | validates BLOB-in-SQLite | *pending* |
| migration time, populated database | D-014 migrations must not stall startup | *pending* |

The similarity row is deliberately adversarial: the design argument for
Go-side cosine over a vector backend rests on the corpus being small enough
that brute force is irrelevant to the user. That argument is only as good as
the benchmark that tests it, so the benchmark is written to *look for* the
corpus size at which it stops being true.
