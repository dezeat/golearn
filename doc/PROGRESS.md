# golearn — Progress

## Status: MVP Complete (2026-02-18)

All core capabilities are implemented, tested (80+ tests), and documented.
Phases 0–14 were completed over 2026-02-16 → 2026-02-18. Full details are
available in the git log.

### Capability Summary

| Area                   | Status   |
|------------------------|----------|
| Domain layer           | ✅ Done — models, validation (8 rules), hashing (SHA-256), correctness, explanation helpers |
| Ports & interfaces     | ✅ Done — 6 repository interfaces, PackReader, StatsRepository |
| SQLite adapter         | ✅ Done — WAL, FK, sequential migrations (v1–v2), all repos |
| Pack reader            | ✅ Done — YAML + JSON, single file + directory |
| Import use case        | ✅ Done — validate → normalise → hash → dedupe → upsert |
| Export use case        | ✅ Done — deterministic ordering, byte-stable YAML/JSON output |
| Session engine         | ✅ Done — lifecycle, 3 selection modes (Balanced, By Difficulty, Weakest) |
| Bubble Tea TUI         | ✅ Done — profile, home, topics, config, quiz, review, summary, stats |
| CLI commands           | ✅ Done — import, run, tui, export, db reset, help |
| Local profiles         | ✅ Done — multi-user, per-user sessions/stats, persisted config |
| Stats feature          | ✅ Done — global, per-pack, difficulty, tags, weak questions, trends |
| Example packs          | ✅ Done — go-basics, mvp-basics, databricks-pde (×2) |
| Quality gates          | ✅ Done — Makefile (check), golangci-lint, GitHub Actions CI |

### Build History (condensed)

| Phase | Date       | Milestone |
|-------|------------|-----------|
| 0     | 2026-02-16 | Initial scaffold, go.mod, stub CLI, go-basics pack |
| 1     | 2026-02-16 | Domain models, validation, hashing, pack reader, SQLite adapter, import CLI |
| 2     | 2026-02-16 | Session engine, selection policy, correctness evaluation, `run` CLI |
| 3     | 2026-02-16 | Bubble Tea TUI, export use case, mvp-basics pack |
| 4     | 2026-02-16 | Product spec rewrite, lint config, CI pipeline, databricks-pde pack (30 Q) |
| 5     | 2026-02-17 | Quiz-show review mode, rationale/explanations, QUESTIONS.md, databricks-pde-explained pack (15 Q) |
| 6     | 2026-02-17 | Text wrapping, tabular topic list, read-only review browse, UX polish |
| 7     | 2026-02-17 | Local multi-user profiles, per-user stats scoping, config persistence |
| 8     | 2026-02-17 | Stats feature (global, per-pack, difficulty, tags, weak Qs, trends), home menu, sparklines |
| 9     | 2026-02-17 | Session-scoped answer shuffling, timestamp formatting |
| 10    | 2026-02-17 | Unified TUI keymap, consistent navigation, layout helpers, footer standardisation |
| 11    | 2026-02-17 | Choice ID / UI label decoupling, explanation cleanup, pack authoring standard update |
| 12    | 2026-02-17 | Databricks pack refactor to numeric IDs, full rationale coverage |
| 13    | 2026-02-17 | Difficulty enum (easy/medium/hard), explanation prefix policy, db reset command |
| 14    | 2026-02-18 | Selection modes (Balanced, By Difficulty, Weakest), stats menu, strongest pack metric |

---

## Technical Debt Log

| ID  | Area             | Description                                         | Priority |
|-----|------------------|-----------------------------------------------------|----------|
| D3  | Export versioning | Pack format `0.1.0`; import accepts any version string. Version-aware parsing needed if schema evolves. | Low |
| D7  | Session state    | Session engine holds in-memory queue; one active session per engine instance. Intentional MVP constraint. | Low |

---

## Planned: Foundation Refactors

_Identified 2026-02-19. Each item improves code quality, performance, or maintainability
without changing user-facing behaviour. Work in any order; each is independently mergeable._

- [ ] **R1 — Deduplicate `displayLabelForIndex`**: Identical function exists in both
      `cmd/golearn/main.go` and `internal/adapters/tui/choice_labels.go`. Extract to a
      shared location (e.g. `domain` or a new `internal/format` package) and import
      from both call sites.

- [ ] **R2 — Deduplicate `resolveCurrentUser` logic**: Near-identical user resolution
      code exists in `main.go` (`resolveCurrentUserID`) and `tui/app.go`
      (`resolveCurrentUser`). Extract to a shared function in the `app` layer that
      both CLI and TUI call.

- [ ] **R3 — Deduplicate export pack-building logic**: `Export()` and `ExportToBytes()`
      in `internal/app/export_pack.go` share ~80% identical pack-construction code.
      Have `Export()` call `ExportToBytes()` + write to file, eliminating the duplication.

- [ ] **R4 — Replace hand-rolled stdlib reimplementations**: Several adapter files
      contain unnecessary custom implementations of stdlib functionality:
      - `selector_difficulty.go`: custom `itoa()` → use `strconv.Itoa`
      - `stats_repo.go`: custom `parseJSONStringArray`, `trimBrackets`, `trimQuotes`,
        `trimSpace`, `splitJSON` → use `encoding/json.Unmarshal`
      - `stats_repo.go`: custom `sortTagStats`, `sortPacksByAttempts` insertion sorts
        → use `sort.Slice`

- [ ] **R5 — Fix N+1 query patterns in `stats_repo.go`**: Three methods issue queries
      in loops: `TopicSummaries` (one query per topic), `TagStats` (one per tag × question),
      `SessionTrend` / `SessionTrendGlobal` (one per session). Rewrite each as a single
      aggregate SQL query with GROUP BY to eliminate the N+1 pattern. This is the highest-
      impact performance improvement in the codebase.

- [ ] **R6 — Split TUI `model.go`**: At ~900 lines with ~80 fields and 14 `update*()`
      handlers, this file is the largest and most complex. Co-locate each screen's
      `update*()` method with its `view*()` function (e.g. move `updateQuestion` into
      `screens_question.go`). Consider grouping related model fields into embedded
      sub-structs (e.g. `quizState`, `reviewState`, `statsState`).

- [ ] **R7 — Move composition root out of TUI adapter**: `tui/app.go` directly imports
      `sqlite` and `localconfig` packages to construct repositories. This violates
      hexagonal architecture — the adapter shouldn't wire other adapters. Move DI
      wiring to `main.go` (or a dedicated `wire.go`) and pass constructed repos into
      the TUI via its `Run()` function signature.

- [ ] **R8 — Use `filepath.Join` for path construction**: `import_pack.go` builds
      directory paths with `fmt.Sprintf("%s/%s", ...)` instead of `filepath.Join()`.
      Not portable on Windows. Replace all path concatenation with `filepath.Join`.

- [ ] **R9 — Surface silently swallowed errors**: Several TUI update handlers discard
      errors from `RecordAttempt`, `EndSession`, stats loading, and session start.
      At minimum, display a user-facing error message in the TUI when these fail.
      Optionally log to a debug log file.

- [ ] **R10 — Add missing test coverage**: The following packages have zero test files:
      `ImportService` (app/import_pack.go), `pack.Reader` (adapters/pack/reader.go),
      `localconfig.Store` (adapters/localconfig/config.go), and `UserContext`
      (app/user_context.go). Add unit tests for each — these are straightforward
      to test and cover critical paths.

- [ ] **R11 — Remove deprecated `GetNextQuestion`**: `session.go` has both
      `GetNextQuestion()` (deprecated) and `GetNextSessionQuestion()`. Remove the
      deprecated method and update any remaining call sites.

- [ ] **R12 — Extract magic numbers to named constants**: Hardcoded thresholds are
      scattered across the codebase: `minAttempts = 3` (selectors), `minAttempts = 5`
      (stats), `limitN = 10` (trends), default question count `10`. Define these as
      package-level constants with descriptive names.

---

## Planned: OSS Release Preparation

_Identified 2026-02-19. Checklist to transform this from a personal project into a
publishable open-source repository. Items are ordered by priority (do top items first)._

### Must-Have (before first public release)

- [ ] **O1 — Fix `go.mod` Go version**: Currently declares `go 1.25.7` which doesn't
      exist. Change to a real stable release (e.g. `go 1.22` or `go 1.23`). Verify
      CI also pins the same version.

- [ ] **O2 — Add CONTRIBUTING.md**: Write a contributor guide covering: how to build,
      how to run tests, how to submit a PR, code style expectations (link to
      WORKFLOW.md code standards), branch naming convention, commit message format.

- [ ] **O3 — Add CODE_OF_CONDUCT.md**: Adopt Contributor Covenant v2.1 or similar.
      Required for most OSS hosting platforms' community standards.

- [ ] **O4 — Add issue and PR templates**: Create `.github/ISSUE_TEMPLATE/bug_report.md`
      and `.github/ISSUE_TEMPLATE/feature_request.md`, plus
      `.github/PULL_REQUEST_TEMPLATE.md` with a checklist (tests pass, docs updated,
      `make check` green).

- [ ] **O5 — Add GoDoc package comments**: Every exported package (domain, ports, app,
      and each adapter) needs a package-level doc comment. Add doc comments to all
      exported functions and types that currently lack them. This enables
      `pkg.go.dev` documentation to render correctly.

- [ ] **O6 — Add `go install` support**: Ensure the module path supports
      `go install github.com/<org>/golearn/cmd/golearn@latest`. Test the install
      path works end-to-end. Document it in the README quickstart.

- [ ] **O7 — Review and clean README**: The README is already good, but needs:
      - A project logo/banner or clear one-liner at the very top
      - Badges (CI status, Go version, license)
      - A GIF or screenshot of the TUI in action
      - Remove references to `databricks-pde-explained.yaml` (internal content
        that may have IP concerns for public release)
      - Add a "Why golearn?" section with 3–4 bullet differentiators

- [ ] **O8 — Audit example packs for public release**: The Databricks PDE packs
      reference specific certification content. Evaluate whether these can be
      published as-is or need to be replaced with generic example packs. Consider
      creating a `examples/sample-quiz.yaml` with technology-agnostic questions.

- [ ] **O9 — Add LICENSE header to source files**: Apache 2.0 recommends adding
      a boilerplate copyright notice to source files. Add a one-line header
      or use a tool like `addlicense` to automate it.

### Nice-to-Have (post-launch polish)

- [ ] **O10 — Create GitHub Releases with binaries**: Set up GoReleaser or a GitHub
      Actions workflow that builds cross-platform binaries (linux/amd64, darwin/arm64,
      windows/amd64) on tag push and attaches them to a GitHub Release.

- [ ] **O11 — Add a CHANGELOG.md**: The PROGRESS.md build history is internal.
      Create a user-facing CHANGELOG.md following Keep a Changelog format, starting
      from v0.1.0.

- [ ] **O12 — Add Homebrew / package manager formula**: After binary releases work,
      create a Homebrew tap for easy macOS/Linux installation.

- [ ] **O13 — Set up GitHub Discussions or wiki**: For community questions, pack
      sharing, and feature requests beyond issue tracking.
