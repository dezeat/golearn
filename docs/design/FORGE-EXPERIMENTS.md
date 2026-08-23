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

### A-12 · A-2 in reverse — the developer's workspace hid a broken clean checkout

- **Question.** A-2 concluded that `go.work` stays gitignored and the addon's
  independence rides on its `replace` directive plus its own `go.sum`. Does
  that stay true as the addon's import graph grows?
- **Hypothesis (implicit, and never stated — which is the problem).** Once the
  addon has a `go.sum`, it stays correct.
- **Method.** A peer session checked out `origin/feat/forge` into a detached
  worktree with **no** `go.work` and ran `make check`.
- **Result. Falsified. Green locally, red in a clean checkout.**

  ```
  --> go vet in ./addons/forge
  ../../internal/app/export_pack.go:31:2: missing go.sum entry for module
  providing package gopkg.in/yaml.v3 (imported by .../internal/app)
  ```

  `addons/forge/go.sum` had **zero** `yaml.v3` entries. Dropping a `go.work`
  into the same worktree flipped it to exit 0; removing it reproduced the
  failure. That was the entire difference.
- **Cause.** The commit that made Forge alias the core pack types widened the
  addon's import graph to include `internal/app` and `internal/domain`, which
  import `yaml.v3`. The addon's `go.sum` was generated at #125, before that
  edge existed. **Under a workspace, module resolution is workspace-wide**, so
  the addon never needs its own entry and the gap is invisible to whoever has
  the workspace — which is exactly the person running the gate before
  committing.
- **What it locked in.** Two things, one of them a correction to how this
  branch verifies:
  1. **`make check` now runs every module-scoped target under `GOWORK=off`**,
     so the local gate sees precisely what a clean checkout and CI see. A-2
     already applied this reasoning to a single boundary guard ("so a
     developer's workspace can never mask a leak that CI would catch"); the
     reasoning was right and its scope was too narrow. A `make tidy` target
     regenerates both modules' `go.sum` the same way.
  2. Any change that widens a module's import graph invalidates that module's
     `go.sum`, and **every remaining Forge story reaches further into core
     internals**, so this trap re-arms rather than being a one-off.
- **Falsifier for the fix.** Stripping the `yaml` lines from
  `addons/forge/go.sum` now fails `make vet` locally with the message above.
  Before this change the same mutation was invisible.
- **The deeper lesson, and it is the third time on this branch.** A-1: a gate
  that skips a module reports success. A-10: a mutation that fails to compile
  reads as a caught mutation. A-12: a gate that runs in a richer environment
  than production reports success. **Every one is the same failure — the
  measurement apparatus quietly not measuring, and reporting green for it.**
  The fix is never a better assertion; it is making the apparatus match the
  environment the claim is about.

### A-13 · Mutation test — provider profiles, capability and secret precedence

- **Question.** #123's claims are the security-sensitive ones on this branch:
  environment beats keychain, Anthropic cannot embed, reachability is not
  authentication, and no surface leaks a credential. Do the guards hold them?
- **Method.** Sixteen mutations of `addons/forge`, each reverted, control
  after. The harness was strengthened first — see the note below.
- **Result. Thirteen caught outright; three survived and each exposed a real
  defect in the guards rather than in the code.**

  | # | Mutation | Outcome |
  | --- | --- | --- |
  | 1 | an `Embedder` stub bolted onto Anthropic | caught by 3 guards |
  | 2 | precedence flipped to keychain-before-environment | caught |
  | 3 | provider error snippets stop being redacted | caught |
  | 4 | unreachable Ollama described in credential terms | caught |
  | 5 | rejected credential reported as missing | caught |
  | 6 | broken keychain reported as a missing credential | caught |
  | 7 | `sk-` shape removed from redaction | caught |
  | 8 | redaction made over-broad, destroying diagnostics | caught |
  | 9 | profile registry claims Anthropic embeds | caught |
  | 10 | Ollama required to have a credential | caught |
  | 11 | `sk-ant-` shape removed | **survived** — see below |
  | 12 | bearer-header echo no longer redacted | caught |
  | 13 | embedding reply index ignored | **survived** — see below |
  | 14 | incomplete embedding reply accepted | **survived** — see below |
  | 15 | Ollama accepts an empty vector | caught |
  | 16 | `sk-` shape removed (now the sole such pattern) | caught |

- **11 — a redundant guard that could not fail.** A separate `sk-ant-` pattern
  sat above the general `sk-` one. Every Anthropic-style key also matches
  `sk-`, so the specific pattern asserted nothing while implying coverage it
  did not add. It was **removed**, not fixed. A redundant guard is worse than
  a missing one: it reads as protection that was never there.
- **13 — a claim in a comment with no test under it.** The OpenAI embeddings
  adapter honours the per-item `index` rather than assuming arrival order, and
  a comment said why. Nothing tested it. A reordered reply would have
  mismatched vectors to questions and corrupted every similarity verdict
  downstream — presenting as a threshold problem, so it would have been
  debugged in entirely the wrong place. Three tests now cover mapping,
  out-of-range indices and short replies.
- **14 — two guards, one assertion.** The count check was subsumed by the
  per-vector nil check, so `err != nil` could not tell them apart. The count
  check earns its place through its *message* — it names the shortfall rather
  than reporting "no vector for input 1" — so the test now asserts the
  message. That is what makes it load-bearing instead of redundant.
- **A leak found by a test, before any mutation.** The credential-redaction
  test failed on first run: `readErrorSnippet` echoed the provider's error body
  verbatim into Forge's error, so a provider or proxy reflecting the submitted
  key back would put it straight into a diagnostic — the exact text users paste
  into bug reports. `domain.Redact` now scrubs credential shapes from anything
  arriving from outside. `Secret` covers the other direction. Neither
  substitutes for the other.
- **The harness had to be fixed first, and this is the fourth instance.**
  Mutation 7 first read as *survived*. It had not survived: `gofmt` had
  realigned the comment it anchored on, so the string replacement silently
  matched nothing and **the file was never modified**. The harness now
  verifies the file actually changed before running the tests, in addition to
  the build-failure check A-10 added. Three ways a mutation can fail to be
  evidence are now known: it does not apply, it does not compile, or it is
  semantically vacuous — and all three look identical to a naive harness.
  Compare A-1, A-10 and A-12: **the apparatus quietly not measuring, and
  reporting green for it.**
### A-14 · What does a search API actually return, and what do the bounds actually do?

- **Question.** #126 implements the research lane behind the frozen
  `ports.Research` contract. Two things had to be observed rather than assumed:
  the **wire format** of the development provider (SearXNG's JSON API), and the
  **behaviour of the Go bounds** the adapter leans on — whether a context error
  survives `http.Client.Do`, and whether an oversized body announces itself.
- **Hypothesis (going in).** The response shape is documented; `io.LimitReader`
  reports truncation as an error; `errors.Is(err, context.DeadlineExceeded)`
  holds through whatever `Do` returns.
- **Method.** Read the upstream serializer and result types
  (`searx/webutils.py` → `get_json_response`, `searx/result_types/_base.py`)
  plus the published API reference; then a throwaway Go probe against
  `net/http/httptest` for the six bound behaviours below. The probe was deleted
  after the observation; the tests that replace it are named in
  `addons/forge/internal/adapters/searxng/searxng_test.go`.
- **Result. Hypothesis falsified in two of three parts.**

  *Wire format.*

  | # | Observation |
  | --- | --- |
  | W1 | Top-level keys are `query`, `results`, `answers`, `corrections`, `infoboxes`, `suggestions`, `unresponsive_engines`. **`number_of_results` is absent** from the current serializer, though it appears in older releases and in much third-party documentation. |
  | W2 | The published API reference documents the *request* parameters and **says nothing about the response shape at all** — only that the format must be enabled. |
  | W3 | A result carries `url`, `title`, `content`, plus `engine`, `engines`, `score`, `category`, `positions`, `parsed_url`, `publishedDate`, `template`, `thumbnail`, `author` and more. `content` is a **snippet**, never the page body. |
  | W4 | There is **no result-count parameter** — only `pageno`. A result ceiling has to be applied client-side. |
  | W5 | `format=json` answers **403** until `json` is listed under `search.formats` in the instance's settings. |
  | W6 | The same URL found by several engines is already merged upstream — that is what `engines` (a set) and `positions` (a list) record. |

  *Bound behaviour, Go 1.25.*

  | # | Observation |
  | --- | --- |
  | B1 | `errors.Is(err, context.DeadlineExceeded)` and `…, context.Canceled` **hold** through the `*url.Error` returned by `Do`. Wrapping `ctx.Err()` by hand is unnecessary. |
  | B2 | A context failure during the **body read**, after headers, surfaces as the bare context error — also `errors.Is`-visible. |
  | B3 | A pre-canceled context produces **zero server hits**: `Do` fails before dialing. |
  | B4 | The handler observes `r.Context().Done()` when the client cancels, so a canceled request is genuinely released rather than merely abandoned. |
  | B5 | **`io.ReadAll(io.LimitReader(body, n))` returns the truncated bytes with a nil error.** Truncation is silent. |

- **What it locked in.**
  1. **Decode leniently, and require nothing not used.** W1 and W2 together are
     the argument: the one response field third-party documentation would have
     had the adapter depend on is the one the current serializer does not emit,
     and the authoritative document does not describe the payload at all. The
     adapter decodes `url`, `title` and `content` and ignores the rest;
     `TestUnknownResponseFieldsAreIgnoredRatherThanRefused` keeps it that way,
     because strict decoding would turn every upstream field addition into an
     outage.
  2. **B5 is the dangerous one and it shaped the code.** An adapter that
     trusted the read error would parse half a document as though it were
     whole — a silent, plausible, wrong answer, which is worse than a failure.
     The adapter reads `max+1` bytes and compares lengths.
     `TestAResponseBodyPastTheSizeBoundIsRejected` is the regression, and
     `TestABodyExactlyAtTheBoundIsAccepted` holds the off-by-one from the other
     side.
  3. **The result ceiling is the adapter's own work** (W4), not something the
     provider can be asked for.
  4. **The 403 names its cause** (W5) rather than reporting a bare status,
     because it is the setup mistake nearly every operator makes once.
  5. **No de-duplication by URL** (W6). SearXNG has already merged across
     engines; a second pass would be inventing canonicalization policy that
     belongs to #120.
  6. **Test-first would have been wrong here, and the log says why.** Writing
     the wire assertions before reading the serializer would have encoded
     `number_of_results` — a field that does not exist — as a specification,
     and would have asserted that an oversized body returns an error, which it
     does not. The bound *semantics* (what must happen at a limit) were still
     written test-first; only the observations were not.

### A-15 · Mutation test — the research adapter's bounds and trust boundary

- **Question.** The adapter's whole value is in its guards: bounds that hold,
  a taxonomy that distinguishes causes, and a trust boundary that does not
  leak. All tests green. Do they bite?
- **Method.** Twenty-eight mutations of
  `addons/forge/internal/adapters/searxng`, each run through three phases —
  **compile**, **run**, **revert-and-prove-clean** — with a control run after.
  The compile phase is explicit and mechanical (`go test -run '^$'`, which
  builds the test binary and runs nothing) because A-10 and A-11 both recorded
  the same trap: a mutation that fails to build tests nothing, and a harness
  that greps only for `--- FAIL` cannot tell that from a guard holding.
- **Result. 27 of 28 caught; the survivor is understood and recorded below.**

  | # | Mutation | Guard that fired |
  | --- | --- | --- |
  | 1 | zero `Timeout` becomes an expired deadline | `TestAZeroTimeoutLeavesTheDeadlineToTheContext` |
  | 2 | response size bound raised out of reach | `TestAResponseBodyPastTheSizeBoundIsRejected` |
  | 3 | per-source byte budget ignored | `TestContentIsTruncatedOnARuneBoundary` |
  | 4 | truncation cuts mid-rune | `TestContentIsTruncatedOnARuneBoundary` (UTF-8 validity) |
  | 5 | result ceiling off by one | `TestResultsAreTruncatedToTheRequestedMaximum` |
  | 6 | citation key stops depending on the URL | `TestTheCitationKeyIsStableForAUrlAcrossQueries` |
  | 7 | publisher invented from the record | `TestPublisherIsLeftEmptyRatherThanDerived` |
  | 8 | adapter declares a source admissible | `TestSourceQualityIsLeftUnclassified` |
  | 9 | unciteable (URL-less) results kept | `TestAResultWithoutAUrlIsDropped` |
  | 10 | every status treated as retryable | `TestARefusedRequestIsNotRetried`, `TestAClientErrorStatusIsNotRetried` |
  | 11 | attempt ceiling off by one | `TestRetryStopsAtTheAttemptCeiling`, `TestASingleAttemptCeilingMakesNoSecondRequest` |
  | 12 | retry loop stops checking the context | `TestALastAttemptCancellationIsNotReportedAsAnExhaustedBudget` |
  | 13 | retry loop checks the context **last** | **survived — see below** |
  | 14 | backoff stops watching the context | `TestCancellationDuringTheRetryBackoffIsImmediate` |
  | 15 | `format=json` not requested | `TestTheRequestCarriesTheObservedSearXNGParameters` |
  | 16 | language sent when none was asked for | `TestTheRequestCarriesTheObservedSearXNGParameters` |
  | 17 | endpoint subpath discarded | `TestASubpathEndpointKeepsItsPrefix` |
  | 18 | query result ceiling unvalidated | `TestAnUnboundedQueryIsRefusedBeforeAnyRequest` |
  | 19 | config attempt ceiling unvalidated | `TestAnUnboundedOrUnusableConfigIsRefused` |
  | 20 | endpoint scheme allowlist removed | `TestAnUnboundedOrUnusableConfigIsRefused/non-http_scheme_with_a_host` |
  | 21 | "no results" reported as a failure | `TestNothingFoundIsNotAFailure` |
  | 22 | unparseable body tolerated | `TestAMalformedResponseBodyIsRejected` (all three shapes) |
  | 23 | 403 loses its operator hint | `TestARefusedRequestIsNotRetried` |
  | 24 | unreachable endpoint reclassified as a bad response | `TestAnUnreachableEndpointIsNotAnAuthenticationFailure` |
  | 25 | exhausted budget drops its cause (`%v` for `%w`) | `TestRetryStopsAtTheAttemptCeiling`, `TestAnUnreachableEndpointIsNotAnAuthenticationFailure` |
  | 26 | decoding turned strict (`DisallowUnknownFields`) | `TestUnknownResponseFieldsAreIgnoredRatherThanRefused` + 16 others |
  | 27 | retrieved content sanitized instead of carried | `TestInjectionShapedPageTextIsCarriedAsQuotedData` |
  | 28 | retrieval time read from the wall clock | `TestEvidenceCarriesTheRecordedResultFields` |

- **Two tests were vacuous, and only the mutations exposed it.** Both were
  green, and both asserted less than they appeared to.
  1. Mutation 20 first *survived*. The non-http case used
     `file:///etc/passwd`, which has no host — so the **host** check refused it
     and the scheme allowlist was never reached. The case tested a different
     guard than its name claimed. A second spelling with a host
     (`ftp://searxng.example.test/`) is refusable only on its scheme, and the
     mutation is caught.
  2. Mutation 27 first *survived*. "Content is carried, not sanitized" was
     asserted with a substring check, which a mutation stripping fence
     sentinels passed comfortably. Carrying untrusted text **verbatim** is the
     actual property — quoting is the fence's job, not the adapter's — so the
     assertion is now equality against the source string.
- **The survivor, and why it stays.** Mutation 13 reorders the retry loop's
  `select` so the context is checked *after* the non-retryable branch rather
  than before. Nothing fires, and nothing should: every context failure reaches
  the loop through `client.Do`, which classifies it as a *retryable* transport
  error (A-13 B1), so `!retryable` and `ctx.Err() != nil` are disjoint in
  practice and the order cannot be observed. Mutation 12 — **removing** the
  check — is the one that proves the guard load-bearing, and it is caught. The
  ordering is documentation of intent; the presence is behaviour. Recorded
  rather than papered over: writing extra code purely to make a mutation fail
  would be gaming the measurement, which is the thing this log exists to
  prevent.
- **A determinism fix disarmed a guard, and only re-running the mutations
  caught it.** Two cancellation tests originally slept 50 ms before canceling,
  which is a timing assumption the gate should not carry. Replacing the sleep
  with a handshake — the handler announces its arrival, the test cancels on
  that signal — fixed the flake and **broke mutation 14**: the cancel now
  landed before the retry loop had entered its backoff, so the loop's own
  context check answered first and the backoff was never exercised. The test
  still passed, still had a plausible name, and no longer tested anything. The
  shipped form keeps the handshake *and* a short grace after it: the handshake
  removes the "did the request happen" assumption, the grace guarantees the
  cancel lands inside the wait, and because the backoff is three orders of
  magnitude longer, a slower machine makes the test more reliable rather than
  less. **The rule this reinforces: any change to a test's timing is a change
  to what it measures, and the only proof is re-running the mutation.**
- **A note on invalid mutations, a third time.** Mutation 25's first form
  deleted `lastErr` from the `fmt.Errorf` call and left it declared and unused,
  so the package did not build. The harness reported `INVALID — no evidence
  either way` rather than counting it, which is exactly the A-10/A-11 lesson
  mechanized. Rewritten as `%v` in place of `%w` — semantically valid, and
  caught.

---

### A-16 · Can a deterministic offline embedder calibrate the similarity gate?

- **Question.** The gate needs a near-duplicate threshold, and `make check`
  must stay offline, so the calibration run has to use a stand-in embedder. Can
  a number derived that way serve as a production threshold?
- **Hypothesis (committed before the run).** A hashed bag-of-words embedder
  would separate the lexical duplicates but **fail** the 0.80 recall floor on
  the full set, because two fixture pairs ask the same question with disjoint
  vocabulary. The pass criteria (no false positives, recall floor 0.80, margin
  0.02) were committed in an earlier commit than any measured number.
- **Method.** Thirteen labeled pairs in
  `addons/forge/internal/app/testdata/similarity_pairs.json`, labeled by
  relation (identical / paraphrase / semantic / competency / concept /
  unrelated) **before** anything was scored. Scored with a deterministic
  L2-normalized hashed-token embedder at 256 dims; threshold derived by
  `domain.Calibrate`.
- **Result. Hypothesis confirmed, and more sharply than expected.**

  | Pair | Relation | Score |
  | --- | --- | --- |
  | verbatim repeat | identical | 1.0000 |
  | different choice-id scheme and order | identical | 1.0000 |
  | paraphrase of a channel question | paraphrase | 0.9649 |
  | reordered options, reworded stem | paraphrase | 0.9619 |
  | paraphrase with filler words | paraphrase | 0.7670 |
  | **same option set, different question** | **concept (negative)** | **0.7368** |
  | shared stem, unrelated concepts | concept (negative) | 0.5618 |
  | same topic and stem, different concept | concept (negative) | 0.5000 |
  | **same fact, opposite direction** | **semantic (duplicate)** | **0.4739** |
  | same assessment, disjoint vocabulary | **semantic (duplicate)** | **0.2860** |
  | different topics entirely | unrelated (negative) | 0.2449 |
  | same difficulty and tag, different subject | unrelated (negative) | 0.1312 |
  | same concept, different competency | competency (negative) | 0.0767 |

  - Full set: threshold 0.76, 0 false positives, recall **0.714** — below the
    committed floor, so the derivation **fails** and returns an error.
  - Lexical subset (semantic pairs excluded): threshold **0.76**, 0 false
    positives, recall 1.00, margin 0.0232.
- **What it locked in.** The two semantic duplicates do not merely score low —
  they score **below three negatives the gate must let through**, so no
  threshold separates them. A lexical scorer is not a weak semantic scorer; it
  is a different measurement. Therefore:
  1. The production calibration table ships **empty** and an uncalibrated model
     is refused (D-022), rather than defaulting to a number this run produced.
  2. 0.76 is recorded as a **fixture baseline** — a self-consistency check on
     the derivation procedure — and `Preflight` refuses a calibration marked as
     one, so it can never decide real content.
  3. The guard is two-sided. Asserting "no false positives" alone is satisfied
     by a threshold of 1.0 that catches nothing, so the committed threshold is
     also asserted to sit at or below the weakest duplicate it must catch.
- **Note on circularity.** Labels were assigned to the *questions* before any
  score existed, and `domain.Calibrate` is unit-tested against hand-computed
  score tables rather than against embedder output. A calibration validated by
  the thing it calibrates would prove nothing.

### A-17 · At what corpus size does brute-force cosine stop being irrelevant?

- **Question.** D-020 rejected an ANN index on the grounds that "a full cosine
  scan is single-digit milliseconds in pure Go" at 10,000 × 768, and required
  that claim to be tested **adversarially**. Where does it break?
- **Hypothesis (going in).** The claim holds at expected scale; the interesting
  number is where it stops.
- **Method.** Real on-disk SQLite databases seeded with a fixed PRNG at seven
  corpus sizes, 768 dims, measured through `Nearest` — not over an in-memory
  slice, which would measure arithmetic no user waits for alone. Three
  benchmarks: one scan, one **pack pass** (20 candidates, the unit a user
  actually waits for), and a cold-cache pack pass reopening the database each
  iteration.
- **Result. The estimate is falsified by 10–20x; the conclusion survives at
  D-012's stated scale.**

  | Corpus | One scan | Pack pass (20), warm | Pack pass, cold |
  | --- | --- | --- | --- |
  | 100 | 0.92 ms | 12.5 ms | 15.4 ms |
  | 1,000 | 8.0 ms | 160 ms | 158 ms |
  | 2,000 | — | 313 ms | 323 ms |
  | 5,000 | — | 791 ms | 838 ms |
  | 10,000 | **79.5 ms** | **1.63 s** | 1.62 s |
  | 50,000 | 396 ms | 8.0 s | 8.6 s |
  | 100,000 | 798 ms | 17.0 s | 17.2 s |

  - Growth is **linear** with no superlinear term.
  - **Cold ≈ warm.** The cost is BLOB decode plus arithmetic, not disk.
  - Against a one-second budget for the pack pass, **the knee is near 6,000
    questions in one topic.**
- **What it locked in.** D-021: the arithmetic is corrected in the log, the
  decision stands inside D-012's "hundreds to low-thousands per topic", and the
  reopening trigger is named (~5,000 vectors in one topic) rather than left to
  judgement. Two cheap fixes are identified ahead of any ANN index — scan once
  per pack rather than once per candidate, or narrow with FTS5 first — both
  implementation changes behind the unchanged port.
- **The methodological point, which is why the third benchmark exists.**
  Benchmarking a single scan would have reported 79.5 ms and read as
  acceptable. The gate scans **once per candidate**, so the number a user meets
  is twenty times larger. A benchmark that measures the convenient unit is the
  same failure as a gate that skips a module: it reports success for something
  it did not measure. What the benchmark holds constant — dimensionality, pack
  size, cache warmth, single process, random vector content — is stated in the
  file rather than left to be inferred, and the cold benchmark records that it
  cannot drop the OS cache and is therefore a floor rather than a worst case.

### A-18 · Mutation test — the similarity index, gate and calibration guards

- **Question.** Twenty-four guards ship with this work and all are green. Do
  they bite?
- **Method.** Twenty-four mutations across `forgestore`, `app` and the
  calibration policy, each build-checked, run, and reverted, with a control
  afterwards.
- **Result. Twenty-three caught; one inert by construction, and one that
  exposed a defect in the code rather than in a test.**

  | # | Mutation | Guard that fired |
  | --- | --- | --- |
  | 1 | `Nearest` skips a mismatched vector instead of refusing | `TestASearchRefusesAStoredVectorOfAnotherDimension` |
  | 2 | a FOREIGN KEY from the Forge table to core `questions` | `TestStoredEmbeddingsDoNotBlockACoreSideDelete` |
  | 3 | `Put` inserts instead of replacing | `TestPutReplacesAVectorRatherThanAccumulating` |
  | 4 | `Nearest` orders ascending | `TestAStoredVectorIsItsOwnNearestNeighbor` |
  | 5 | `Nearest` ignores the model scope | `TestASearchNeverScoresVectorsFromAnotherModel` |
  | 6 | the write-side dimension guard always passes | `TestPutRefusesAVectorOfADifferentDimensionForTheSameModel` |
  | 7 | `Missing` ignores the model scope | `TestMissingReportsOnlyTheIdsWithoutAVector` |
  | 8 | `Count` ignores the model scope | `TestCountIsScopedToOneEmbeddingModel` |
  | 9 | an empty vector is accepted | `TestPutRefusesAnEmptyVector` |
  | 10 | an unresolved candidate is accepted, not rejected | `TestACandidateStillTooSimilarWhenTheBudgetRunsOutIsRejected` |
  | 11 | the no-budget-left break is removed | same, in 0.00 s with "got 3 attempts" |
  | 12 | the ladder's loop bound is lifted | **inert** — see below |
  | 13 | no embedding capability waves candidates through | `TestWithoutAnEmbeddingCapabilityTheGateRefusesRatherThanAcceptingEverything` |
  | 14 | library neighbors never cross the threshold | three ladder tests |
  | 15 | the reject threshold never routes to replacement | `TestANearIdenticalCandidateIsReplacedRatherThanRepaired` |
  | 16 | the exact-duplicate short circuit is removed | `TestAVerbatimDuplicateInOnePackSpendsNoRepairBudget` |
  | 17 | accepted peers never accumulate | `TestTwoCollidingCandidatesInOnePackKeepTheFirst` |
  | 18 | the backfill re-embeds the whole topic | `TestOnlyLibraryQuestionsWithoutAVectorAreEmbedded` |
  | 19 | a short embedder reply is accepted | `TestAShortEmbedderReplyIsRefusedRatherThanMisattributed` (panic) |
  | 20 | candidate vectors written into the library index | `TestTwoCollidingCandidatesInOnePackKeepTheFirst` |
  | 21 | the calibration/model mismatch check removed | `TestAThresholdCalibratedForAnotherModelIsRefused` |
  | 22 | `NewGate` invents a calibration instead of looking one up | `TestTheProductionConstructorRefusesAnUncalibratedModel` |
  | 23 | the fixture-baseline refusal is removed | `TestAFixtureBaselineIsRefusedAsAScoringThreshold` |
  | 24 | the unversioned-calibration refusal is removed | `TestAnUnversionedCalibrationIsRefused` |

- **Mutation 2 is the one that matters, and it discriminates.** Under a foreign
  key to `questions(id)`, `TestForgeMigrationsNeverTouchCoreTables` **passes** —
  it compares `pragma_table_info`, and an inbound foreign key changes no
  column — while the new delete test fails with `FOREIGN KEY constraint
  failed`. The core opens the database with `foreign_keys=ON`, so that
  constraint would make a core-side delete of a question fail: the offline
  binary would break because the user once ran Forge. The pre-existing guard
  could not have caught it, and now something does.

- **A fourth instance of the recurring failure, and this time it was in the
  harness.** Mutation 11's first form removed the budget check from a `for {}`
  loop whose only exit that check was. The mutated code did not misbehave — it
  **never returned**. `go test` hit its default timeout, printed a panic rather
  than a `--- FAIL:` line, and the harness's grep scored it **SURVIVED**. It
  was neither caught nor survived: the experiment never ran.

  Two fixes, and the first is the important one. **The gate's loop was
  restructured so termination is structural** — a bounded
  `for attempt := 0; attempt <= maxGateAttempts` — with rejection as what
  happens on falling through. The same mutation now fails in milliseconds
  saying "got 3 attempts", and the fail-safe direction is rejection rather than
  acceptance. The harness was then taught to report a timeout as
  **INCONCLUSIVE**, never as a result. Mutation 12 records the consequence:
  with the inner break intact the outer bound is unreachable, so lifting it
  changes nothing — it is a backstop, and mutation 11 is what proves the
  backstop is needed.

  A-1: a gate that skips a module reports success. A-10: a mutation that fails
  to compile reads as caught. A-12: a gate running in a richer environment than
  production reports success. A-18: **a mutation that hangs reads as
  survived.** Every one is the measurement apparatus quietly not measuring, and
  the fix is never a better assertion.

- **A defect the tests found, not the mutations.** `NewGate` accepted a
  caller-supplied `Calibration`, so the "refuse an uncalibrated model" policy
  built one commit earlier could be walked around by simply constructing one —
  a fixture baseline included. Mutations 22–24 exist because the fix does;
  before it there was no guard to mutate. A refusal a caller can bypass is
  decoration, and the empty table only means something because `NewGate` is now
  the sole path to a threshold.

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

#### B-2.1 · Model reasoning is unaffordable on the reference host

- **Question.** Can the installed models produce a schema-valid question pack
  candidate within a usable time on the CPU-only reference host?
- **Hypothesis (going in).** Yes; a 4B model on four cores is slow but
  workable, and the larger models are the ones at risk.
- **Method.** One identical structured-output request per configuration,
  through the shipped Ollama adapter, against the operator-managed host.
  Judged against the KPIs committed in B-1 — `structured_valid_rate`,
  `latency_total`, `throughput` — which were written down before any run.
- **Result. Falsified, and not by the size axis.**

  | Model | Reasoning | `latency_total` | `throughput` | `structured_valid` |
  | --- | --- | --- | --- | --- |
  | `qwen3:4b` | on (model default) | **>300 s — deadline exceeded** | — | **no output at all** |
  | `qwen3:4b` | off | 49.6 s | 2.1 tok/s | yes |
  | `qwen3.5:4b` | off | 68.9 s | 1.6 tok/s | yes |

  The first run returned nothing after five minutes. These are
  reasoning-capable models, and on four CPU cores the private reasoning pass
  costs more than six times the visible answer — the answer itself is ~100
  tokens.
- **What it locked in.** The Ollama adapter sends `think: false` by default,
  and states it **explicitly** rather than omitting the field: omission defers
  to each model's own default, which is precisely the silent variance that
  made the first run unreadable. Forge asks for one structured document and
  runs its own verification pass over it (D-016), so the model's private
  reasoning is a cost paid without a corresponding benefit to the pipeline.
  `WithReasoning(true)` overrides it, because this measurement is about one
  class of hardware and a GPU host would reasonably choose otherwise.
  `TestOllamaDisablesModelReasoningByDefault` asserts the request body, since
  a default that flips back is otherwise invisible.
- **Consequence for pipeline design, stated rather than discovered later.** At
  ~50–70 s per model call, a full D-016 chain — generate, fresh-context verify,
  critique, one bounded repair — costs roughly 3–5 minutes *per question* on
  this hardware. Pack size is therefore the parameter that gives, not the
  stage chain: D-016's Consequences say cutting stages reopens the decision
  rather than descoping it.
- **Withheld claim (#130's scope discipline).** This establishes **mechanism**
  — wire format, structured-output parseability, latency, throughput — on one
  CPU host. It says nothing about pack quality, and nothing about any other
  provider or model. `qwen3:4b` echoed the instruction back as the question
  prompt while `qwen3.5:4b` produced a genuine question; that is a single
  observation, not a quality finding, and it is recorded as such.

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
| similarity scan vs corpus size | **tests the no-vector-index claim** | one scan **79.5 ms** at 10k x 768; one 20-candidate pack pass **1.63 s**. Linear; cold ~ warm. **Knee ~6,000** questions/topic against a 1 s pack budget. Falsifies D-020's "single-digit ms" estimate (A-17, D-021) |
| embedding bytes per 1000 questions | validates BLOB-in-SQLite | **3.98 MB** (4,177,920 B) at 768 dims — 1.36x the 3.07 MB raw float32 payload, the remainder SQLite page and overflow overhead. Bounded by `TestEmbeddingFootprintPerThousandQuestions` |
| migration time, populated database | D-014 migrations must not stall startup | *pending* |

The similarity row is deliberately adversarial: the design argument for
Go-side cosine over a vector backend rests on the corpus being small enough
that brute force is irrelevant to the user. That argument is only as good as
the benchmark that tests it, so the benchmark is written to *look for* the
corpus size at which it stops being true.
