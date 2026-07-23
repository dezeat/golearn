# golearn — DECISIONS

Append-only decision log. `docs/architecture.md` describes the _current_ state
of the system; this file records _why_ it got that way, one dated entry per
decision, newest at the bottom. Entries are **never edited after acceptance** —
a changed mind gets a new entry that supersedes the old one (mark the old entry
`superseded by D-NNN`; leave its text intact).

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

```md
## D-NNN — Short title of the decision

Status: accepted | superseded by D-NNN
Date: YYYY-MM-DD

Context: 2–3 sentences — the situation and the forces that made this a choice.
Decision: 1–2 sentences — what was picked. Name the rejected alternative when
the rejection is the point.
Consequences: 2–3 sentences — what this buys, what it costs, what it forbids.
```

Numbering: highest existing `D-NNN` + 1. Reference `architecture.md` sections
instead of repeating them.

---

## D-001 — CGo-free SQLite via `modernc.org/sqlite`

Status: accepted
Date: 2026-07-05

Context: golearn needs an embedded, file-based SQLite store, and the popular
driver (`mattn/go-sqlite3`) links the C library via CGo, which requires a C
toolchain and blocks static cross-compilation. Being a local-first CLI shipped
as prebuilt binaries for multiple platforms, a C build step is disproportionate
cost.
Decision: Use the pure-Go `modernc.org/sqlite` driver; never swap in a
CGo-based driver.
Consequences: The binary cross-compiles to any target with `GOOS`/`GOARCH`
alone and CI needs no C toolchain. The cost is a larger binary and a modest
per-query overhead versus the CGo driver — acceptable for a single-user
interactive tool. Anyone reintroducing CGo breaks the release matrix.

## D-002 — Hexagonal layering with adapters that never import each other

Status: accepted
Date: 2026-07-05

Context: A learning-engine core (validation, hashing, selection, scoring) must
stay testable and swappable independently of SQLite, YAML parsing, and the
Bubble Tea TUI, which otherwise tend to entangle domain logic with I/O.
Decision: Adopt ports & adapters — `domain` (pure, stdlib-only), `ports`
(interfaces), `app` (use cases over domain+ports), `adapters` (sqlite, pack,
tui, localconfig), `cmd` (composition root) — with the hard rule that an
adapter never imports another adapter and all wiring happens in `cmd`.
Consequences: The domain and use cases test without a database or a terminal,
and any adapter is replaceable behind its port. The cost is indirection and the
discipline of injecting every dependency through the composition root; a
`tui`-imports-`sqlite` shortcut is an architecture violation, not a convenience.
See architecture.md "Hexagonal layout".

## D-003 — Manual `os.Args` CLI routing; a framework is deferred

Status: accepted
Date: 2026-07-05

Context: The command surface is small (`import`, `run`, `tui`, `export`, `db
reset`, `help`), and pulling in cobra or urfave/cli would add a runtime
dependency to a project that keeps them deliberately few.
Decision: Route commands with manual `os.Args` parsing in `cmd/golearn/main.go`
and a `switch`; parse global flags like `--db` at the root before subcommand
dispatch. Add commands by extending the switch, not by adopting a framework.
Consequences: Zero CLI dependency and full control over parsing and help text.
The cost is that flag handling is hand-rolled — a global flag registered on a
subcommand is silently consumed by `os.Args` before the subcommand sees it, so
root-level parsing is mandatory. Revisit cobra only if the surface grows
significantly.

## D-004 — Import validation is all-or-nothing per file

Status: accepted
Date: 2026-07-05

Context: A pack file is a single authored unit; importing some questions while
rejecting others would leave a partially-applied pack that is hard to reason
about and hard to re-run idempotently.
Decision: Validate every question in a file before inserting any; a single
validation failure rejects the whole file. Directory imports process each file
independently, so one bad file does not sink its siblings.
Consequences: The database never holds a half-applied pack, and a failed import
is safe to fix and retry. The cost is coarser feedback — one invalid question
blocks its entire file — which is the intended contract, not a limitation to
route around with partial inserts.

## D-005 — Determinism via an injected seeded `*rand.Rand`

Status: accepted
Date: 2026-07-05

Context: Question selection shuffles order, and export must be byte-stable, yet
tests need to assert exact orderings; the global `math/rand` PRNG and
`crypto/rand` are both process-global or non-reproducible and cannot be pinned
from a test.
Decision: Thread an explicit seeded `*rand.Rand` through every shuffle in the
selection and session code; never call the global PRNG or `crypto/rand` for
ordering.
Consequences: Same seed and same data yield byte-identical output, so
selection and shuffle are asserted directly against fixed-seed fixtures. The
cost is passing the RNG through call sites instead of reaching for a package
global — a small, deliberate ergonomic tax that buys reproducibility. A test
depending on wall-clock time or the global PRNG is a bug.

## D-006 — WAL journal mode enabled on every database open

Status: accepted
Date: 2026-07-05

Context: The TUI reads and writes concurrently (recording attempts while
rendering stats), and SQLite's default rollback journal serializes these into
"database is locked" errors. WAL is not reliably sticky — some operations can
reset the journal mode.
Decision: Issue `PRAGMA journal_mode=WAL` (and enable foreign keys) on every
`db.Open`, never assuming it persists from a prior session.
Consequences: Concurrent reader/writer access from the TUI stops deadlocking.
The cost is the WAL sidecar files (`-wal`, `-shm`) that `db reset` must also
clean up. Do not skip the pragma on the assumption it was set once before.

## D-007 — Stable content hashing for dedup, with a hash tie-break on export

Status: accepted
Date: 2026-07-05

Context: Re-importing an overlapping pack must not duplicate questions, and
export must be byte-reproducible — but questions imported in one batch share an
identical `created_at`, so ordering by timestamp alone is non-deterministic.
Decision: Identify a question by a stable SHA-256 over its normalised,
null-byte-separated fields (topic slug, type, intro, prompt, ordered choices,
sorted correct IDs, difficulty); enforce it as a UNIQUE constraint for dedup,
and order exports by `created_at ASC` with the content hash as the tie-break.
Consequences: Duplicate imports are skipped and export output is byte-identical
across runs. The hash recipe is a data contract — changing normalisation, field
order, or the separator silently changes every hash and breaks dedup — so it is
frozen and any change is a new decision. Omitting the export tie-break
reintroduces non-determinism whenever timestamps collide.

## D-008 — Presentation is computed at render time, never baked into pack data

Status: accepted
Date: 2026-07-05

Context: Choice display labels (`A`/`B`/`C`) and explanation correctness
prefixes ("Correct:", emoji) are presentation, but it is tempting to store them
in the pack — which would make labels wrong under shuffled display order and
would couple authored content to one renderer's styling.
Decision: Keep pack data content-only — `Choice.id` is an internal stable
identifier, never a display label, and explanations carry no prefixes; the TUI
derives labels from the current display order (`DisplayLabelForIndex`) and adds
prefixes at render (`FormatChoiceExplanation`, with `StripExplanationPrefix`
for legacy packs).
Consequences: Labels stay correct when choices are shuffled, and presentation
styling changes without touching authored content or re-hashing. The cost is
that authors and importers must resist encoding labels or prefixes into packs;
doing so corrupts both the display and the content hash (D-007).

## D-009 — `AGENTS.md` is the single agent instruction file; `.agents/` the shared config tree

Status: accepted
Date: 2026-07-23

Context: The repo's agent harness was Claude-specific — instructions in
`CLAUDE.md`, skills and hooks under `.claude/`. Running a second agent (Codex)
meant either duplicating the instructions, which drift, or leaving the second
agent uninstructed. Meanwhile `AGENTS.md` has become the vendor-neutral
convention that Codex, Cursor, Copilot, Zed and others read, and Codex
discovers skills at `$REPO_ROOT/.agents/skills` natively.
Decision: The instructions live in root `AGENTS.md`; `CLAUDE.md` is reduced to
a single `@AGENTS.md` import line. Skills, hooks and subagents live under
`.agents/`, and `make agents` (`scripts/agents-link.sh`) generates the
gitignored vendor links `.claude/skills` and `.claude/agents`. A bare
`ln -s AGENTS.md CLAUDE.md` was rejected: the import survives a Windows clone
without Developer Mode and leaves room for Claude-only additions.
Consequences: One file to edit, and Codex reads both `AGENTS.md` and
`.agents/skills` with no link at all. The cost is a setup step — a fresh clone
or worktree must run `make agents` before Claude Code sees skills or subagents,
which is why the links are generated rather than committed (a committed symlink
breaks on clones without symlink support).

## D-010 — Reviewer subagents inlined in-repo until the `crew` plugin ships

Status: accepted
Date: 2026-07-23

Context: Review quality depended on a single self-review step inside the `pr`
skill, performed by the same session that wrote the code. Dedicated reviewer
subagents with their own context window catch what the author's session is
blind to. These reviewers are slated to lift into the shared `dezeat/crew`
marketplace plugin, so adding them here duplicates a planned artifact.
Decision: Inline `pr-reviewer` and `architecture-reviewer` under
`.agents/agents/` now, adapted to this repo's laws, on the same terms as the
inlined operating model — provisional, and removed when `crew` ships. The two
reviewers are split by scope deliberately: architecture judges the binding docs,
the PR reviewer judges correctness and conventions, and neither does the
other's job.
Consequences: Reviews get a second, independent context window immediately,
and the prompts double as an executable statement of the repo's invariants. The
cost is a known duplication to unwind at extraction, and two more files that
must track `AGENTS.md` when the standards change.
