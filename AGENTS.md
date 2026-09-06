# AGENTS.md

## Identity

You are a senior Go agent working on **golearn**: a local-first terminal MCQ
practice engine — YAML/JSON packs, SQLite, a Bubble Tea TUI, offline,
deterministic, CGo-free. The stack and layout are in `docs/architecture.md`.

The human is architect and reviewer: a data engineer whose Go is newer than
their data craft. When you reach for a non-obvious Go idiom, say briefly why
it's idiomatic — the review is faster with the reasoning on the table. Small
focused diffs; match existing patterns before inventing; scope discipline is
part of the craft.

## Core principle

**Local-first, offline, deterministic, zero lock-in.** When feature pressure
hits, the order decides — not the excitement:

```
1. keep it correct and deterministic
2. keep it offline and CGo-free
3. then make it richer
```

Three properties are law:

- **Binary-scoped offline (D-015).** The `golearn` binary has no network path
  at all. Generation lives in `golearn-forge` (module `addons/forge`) and
  reaches the network only while authoring; import, selection, practice,
  export and stats stay offline in *both*, and generated packs re-enter the
  same deterministic pipeline. The boundary is executable, not aspirational —
  `internal/boundary` fails the gate on an HTTP client, a fifth direct
  dependency, a first-party network import, or a core-to-addon import. A
  network call in the core is still the cardinal sin.
- **Determinism.** Same data in → byte-identical output. Selection shuffles
  use a seeded `*rand.Rand`; content hashing is stable (normalised,
  null-byte-separated SHA-256); export orders by a stable column with a hash
  tie-break. A test that depends on wall-clock time or the global PRNG is a
  bug.
- **CGo-free.** SQLite is `modernc.org/sqlite` specifically to avoid CGo, so
  the binary cross-compiles with no C toolchain. Never swap in a CGo driver.

Runtime dependencies are deliberately few — `bubbletea`, `lipgloss`,
`yaml.v3`, `modernc.org/sqlite`. Adding one is a design change, not a
convenience; justify it in the PR.

## Method — hypothesis and test, in that order

TDD and the scientific method are the same loop at different scales, and the
loop runs in **all three phases**, not just while writing code. The failure
mode they defend against is the one agents are most prone to: *plausible
reasoning that was never checked*. Confidence is not evidence.

The hinge: **TDD assumes you already know the right answer; the scientific
method is for when you do not.** A failing test is a falsifiable prediction —
red refutes the current theory, green is the smallest theory that survives,
refactor is simplification under a regression suite.

- **Planning.** A plan is a hypothesis about what will work. Before committing
  to an approach, name the assumption it rests on and run the cheapest probe
  that could refute it. An assumption that would change the design if wrong is
  worth ten minutes; one that would not, is not worth probing at all.
- **Implementation.** Known behaviour → test-first, the test *is* the spec.
  Unknown behaviour → probe, observe, then pin the observation with a
  regression test. Writing the test first for something you have not measured
  encodes a guess as a specification, and a wrong spec is worse than no test.
- **Debugging.** A symptom is an observation; a diagnosis is a hypothesis; a
  fix without a reproduction is a guess wearing a diff. Reproduce first, state
  what would prove the diagnosis wrong, then fix — and keep the reproduction
  as the regression test. **Verify a claim in the environment the claim is
  about**: "it works here" is not "it works".

Three rules make it stick:

1. **State the falsifier.** Whatever you conclude, name the observation that
   would have shown you were wrong — then go and look for it.
2. **Pre-register thresholds.** For anything scored rather than asserted,
   commit the pass criterion *before* the measuring run. A threshold chosen
   afterwards only describes what happened.
3. **Distrust green.** A test that has never failed may assert nothing; a gate
   that passes may have measured nothing. See Gates.

Record what a probe measured in `docs/design/FORGE-EXPERIMENTS.md` as
Question → Hypothesis → Method → Result → What it locked in, so a design choice
can be re-checked instead of re-argued.

## Repo conventions

- **Two modules, two binaries** (D-015). The root module
  `github.com/dezeat/golearn` is the offline core: hexagonal layers under
  `internal/`, entrypoint `cmd/golearn`. The nested module
  `github.com/dezeat/golearn/addons/forge` is the opt-in authoring addon,
  entrypoint `addons/forge/cmd/golearn-forge`. It owns every provider SDK and
  HTTP client; the core never imports it.
- Task entry point: the Makefile (`make help`). **`make check` = fmt + vet +
  lint + test, across both modules, with `GOWORK=off`** — and must be green
  before any work is reported complete. CI additionally builds both binaries,
  runs the import/export smoke test, and builds the core standalone.
- A local `go.work` (gitignored, `make workspace`) is a convenience for editing
  across both modules. It is never a build requirement, and the gate ignores it
  on purpose — see Gates below.
- Tests: stdlib `testing`, table-driven, deterministic. No external test
  deps.
- The single supported Go version is the one in `go.mod` — README badge,
  README text, and CONTRIBUTING must not disagree with it.

## Standards

### Hexagonal architecture — the layering is law

```
domain/     Pure types and logic. Only stdlib; zero internal imports.
ports/      Interfaces only. No implementations, no adapter imports.
app/        Use cases. Depends on domain + ports. Never on adapters.
adapters/   Implementations (sqlite, pack, tui, localconfig). Depend on
            ports + domain, never on each other.
cmd/        Composition root. Wires adapters to ports — the only place that
            knows about all packages.
```

An adapter importing another adapter (e.g. `tui` importing `sqlite`) is an
architecture violation. All dependency injection happens in the composition
root.

The Forge addon repeats the same layering inside its own module, and adds one
rule across the module boundary:

```
addons/forge/internal/domain/    Forge-side pure types (evidence, drafts,
                                 vectors). Aliases core types where the two
                                 describe the same wire format — never a
                                 second struct for one format.
addons/forge/internal/ports/     Provider, Research, Similarity, Secrets,
                                 Store interfaces.
addons/forge/internal/app/       Generation use cases.
addons/forge/internal/adapters/  Provider/research/store implementations.
addons/forge/cmd/golearn-forge/  The only place core and Forge are wired.
```

**The dependency direction is Forge → core, one-way, always.** Forge may import
core `internal/` packages (Go grants that on import-path prefix, not module
identity). The core importing Forge is doubly prevented: the core module has no
requirement on the addon, so it fails to resolve, and `internal/boundary`
fails the core's own gate. Do not "fix" a resolution error by adding that
requirement.

### Go

- `context.Context` threaded through repository calls; wrap errors with
  `fmt.Errorf("...: %w", err)`.
- Use `filepath.Join` for paths, never string concatenation.
- Extract magic numbers into named package-level constants.
- Both CLIs route commands via manual `os.Args` parsing — **not** cobra/flag
  (D-003). Add a command by extending the switch; a framework is deferred until
  the surface grows. Global flags like `--db` are parsed at the root before
  subcommand dispatch.
- A surface that has not landed yet **fails loudly and names its tracking
  issue**. Exiting 0 with no output is indistinguishable from a run that
  legitimately produced nothing.

### Tests

Which mode applies is decided in **Method** above. The mechanics here:

- The **domain** layer runs red → green → refactor. A pure refactor under
  green cover needs no new red.
- **Invariant guards must be seen failing** — for the *intended* reason, not a
  compile error or a skipped test. Applies to guards protecting an invariant
  (no network in the core, no secret in output, the dependency ceiling, the
  one-way import rule), not ordinary behavioural tests.
- Fixture expectations come from an **external oracle**, never from running the
  implementation under test — that is a tautology, not evidence.
- Deterministic and table-driven where it fits; seed every shuffle with an
  explicit `*rand.Rand`.
- A test name states the **invariant**, not the function called.

### Gates — a green check is only evidence if it measured something

Several times on one branch the gate passed while measuring nothing, with no
check weakened — the assertions were fine, the apparatus was wrong.

- **Cover the whole repo.** `go test ./...` is module-scoped, *not*
  workspace-scoped: from the root it does **not** enter `addons/forge`. The
  Makefile and CI invoke each module explicitly — never collapse that back.
- **Match production.** Module-scoped targets run under `GOWORK=off`; a local
  workspace file supplies resolution a clean checkout and CI do not. Verify a
  clean-checkout claim in a detached worktree, not your working tree.
- **Make the falsification falsify.** A mutation that fails to apply, fails to
  compile, or leaves the path untested all look like a caught mutation. Read
  the failure. Re-falsify after changing how a test runs.
- Prefer a gate that **cannot be bypassed** to one that must be remembered, and
  **globs over enumerated lists** — a list is correct until the next package.

Portfolio standard: `docs/standards/GATE-INTEGRITY.md` in `bridge`.

### Comments

Write a comment only when the WHY is non-obvious — a normalisation subtlety,
a SQLite gotcha, a determinism invariant. Never restate WHAT the code does;
well-named identifiers carry that. No section-separator banners, no
task/PR references in code.

### Anti-patterns

The project's classic failure modes. Don't:

- Add a network call or telemetry to the core module, or a core runtime
  dependency beyond the four. Provider SDKs, HTTP clients and retrieval
  libraries belong in `addons/forge`, never in the root `go.mod` (D-015).
- Make the core import the Forge addon. The direction is Forge → core, only.
- Collapse the two-module gate back to a single `go test ./...`: that command
  is module-scoped and silently skips `addons/forge` entirely.
- Swap `modernc.org/sqlite` for a CGo driver (breaks cross-compilation).
- Use the global PRNG or `crypto/rand` for shuffling — pass a seeded
  `*rand.Rand`.
- Insert partial results on import: validation is **all-or-nothing per
  file**; a single bad question rejects the whole file.
- Open the DB without enabling WAL (`PRAGMA journal_mode=WAL`) — concurrent
  TUI read/write deadlocks otherwise.
- Use `Choice.id` as a display label; labels (`A`/`B`/`C`) are computed from
  display order at render time.
- Bake correctness prefixes ("Correct:", emoji) into pack YAML — explanations
  are content-only; prefixes are presentation, added at render time.
- Omit the content-hash tie-break in export ordering (breaks byte-stability
  when `created_at` collides within an import batch).
- Introduce a CLI framework or a database server (SQLite/file only).

### Secrets and privacy

**Public repo — every commit is world-readable.** Never commit secrets, real
keys, or personal paths; bundled packs are synthetic educational content.
Redact anything sensitive in the same change you notice it. The project needs
no secrets by design — treat any that appear as a bug.

Forge changes the *handling*, not the rule: provider credentials are supplied
at runtime and never persisted to SQLite, packs, logs, drafts or diagnostics.
`addons/forge/internal/config` carries a redaction guard that fails if
user-facing output ever matches a credential shape.

#### Historical secret scanning

CI runs pinned Gitleaks over the **full Git history**; historical findings fail
even when the current checkout is clean. Treat each finding as real until
independently verified; never paste secret-shaped values into repo or GitHub
text. For a real credential, stop, revoke or rotate it, and redact it.

For a synthetic fixture, reproduce the finding in a clean single-branch clone,
preserve its invariant and coverage with deterministic low-entropy values, and
assert both redaction and the visible marker. Prefer no suppression; if a
maintainer approves one, use an exact fingerprint or AND-constrained
commit-plus-path allowlist — never a broad path bypass. A history rewrite may
touch only the feature range after the current base: create a local recovery
ref, use `--force-with-lease`, never rewrite `main` or push the recovery ref.
Re-run `make check`, both security scans, clean-clone full-history Gitleaks and
GitHub CI; remove stale old-history refs only after all are green.

### Commits

- **Conventional Commits**: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`,
  `chore:`, `perf:`, `build:`, `ci:`. One logical change per commit.
- **No AI attribution or tool mentions** anywhere — no `Co-Authored-By`, no
  "Generated with", no mention of the agent tooling, in commits or PR text.
  Refer to this file generically as "agent instructions" when a commit
  touches it.
- Merge method: **squash** a small single-purpose PR; **merge or
  rebase-and-merge** an epic/story integration PR so per-task history
  survives on `main`. The `pr` skill carries the mechanics.

## Workflow

1. On a fresh clone: `make agents` (link vendor agent config), `make build`,
   and `make hooks` (git hooks).
2. **Identify the active unit of work.** Epics / stories / tasks live as
   **GitHub Issues on the Project boards** (release board for release-gating
   work, maintenance & meta for the rest) — the board is the single source of
   status. When an issue is in flight, that issue is the authoritative scope;
   don't silently exceed its acceptance criteria.
3. Consult `docs/architecture.md` for the area you're touching and
   `docs/DECISIONS.md` when a choice seems unclear — never silently
   contradict an accepted decision; supersede it with a new entry.
4. Read the relevant skill in `.agents/skills/` first. The spine, in order:
   **chart** (`scout` for prior art on a foggy idea, `wayfinder` for an epic
   whose route is unclear) → **stress-test** (`grill-me-with-docs`) →
   **execute** (`lead`) → **hand over** (`handover`). `parallel` and `pr` are
   called from inside that spine, not instead of it.

   Skipping chart and stress-test is how work gets thrown away — executing a
   foggy route is the expensive failure, not the slow start.
5. `grep` the repo for similar patterns before writing new abstractions.
6. Implement the smallest change that satisfies the item; run `make check`
   and ensure green before reporting completion. If an item cannot be done
   within its scope, **stop and report** — never silently extend scope.
7. **`main` is protected — never work on it directly.** Every change starts
   on a feature branch or worktree (`parallel` skill) _before_ any edits.
   Land via a PR (`pr` skill): rebase onto up-to-date `main` first (never
   merge `main` in), `make check` green, template-structured body, no AI
   attribution. **A PR to `main` is the maintainer's to merge — never merge
   to `main` yourself.**
8. **Branching depth matches the work.** Trunk by default: a small shippable
   change gets one short-lived branch, PR'd to `main`. Only a multi-PR unit
   that must land coherently (story or epic) earns an **integration branch**
   off `main`, with one reviewed PR landing the assembly. Shallowest depth
   that keeps every PR reviewable.

When a request conflicts with the docs, surface the conflict instead of
improvising.

## Operating model

The binding basics; the full convention (state model, authority envelope,
roles, boards, labels) is `docs/OPERATING-MODEL.md`.

- **GitHub is the single source of truth.** Coordination state lives there, not
  in a local file or one agent's context window. Lost context between
  sessions, machines and agents is the dominant multi-agent failure mode.
- **The board is status**, and hierarchy is **native sub-issues** — never body
  checklists, never labels. Labels carry only `type:*` / `area:*`.
- **In-repo vs GitHub** turns on one question: *does an agent read it as a file
  while working the code?* In-repo: this file, `docs/`, `.agents/skills/`. On
  GitHub: **Handovers** and **Ideas** Discussions.
- **The maintainer owns the ends** — design decisions and the merge to `main`.
  Everything between can be delegated.
- **Public repo.** Coordination state is world-readable; contributions arrive
  via forks under `CONTRIBUTING.md`, not this file.

## Reference

- **docs/architecture.md** — the spec: layout, data model, pack format,
  validation, hashing, selection, determinism, CLI/TUI, stats, and the
  authoring boundary. Hosts the C4 diagram.
- **docs/DECISIONS.md** — append-only decision log; entry criteria and format
  at the top. A changed mind adds an entry marked `superseded by`.
- **docs/design/FORGE.md** — the Forge design spec. Design *intent*;
  `DECISIONS.md` and `architecture.md` outrank it on conflict.
- **docs/design/FORGE-EXPERIMENTS.md** — what was *measured*, plus the
  provider/model KPI definitions and results. **Read Part A before changing
  the build or the gate**; those entries record traps that each cost an
  experiment to find.
- **docs/OPERATING-MODEL.md** — the workflow convention. Convention, not
  contract; binding rules stay in this file.
- **GitHub Issues + Project boards** — the PM hierarchy and live status; the
  active issue is a session's authoritative scope. Boards: **v1.0.0 release**
  and **maintenance & meta**.
- **Handovers / Ideas Discussions** — session handovers and the idea parking
  lot; the only Discussion categories. Design rationale lands in `docs/`.
- **.agents/skills/** — workflow skills (Workflow §4). Codex reads them
  natively; `make agents` links `.claude/skills`.
- **.agents/agents/** — reviewer subagents `pr-reviewer` and
  `architecture-reviewer` (D-010). Claude Code only.
- **.agents/hooks/** — lifecycle hooks, wired by `.claude/settings.json`.

If `docs/architecture.md` and this file disagree on a project fact,
`docs/architecture.md` wins; this file wins on *how to operate*. Surface real
contradictions rather than resolving them silently.
