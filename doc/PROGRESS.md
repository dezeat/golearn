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
| 15    | 2026-02-19 | Foundation refactors R1–R5: dedup displayLabel/resolveUser/export, stdlib replacements, N+1 query fixes |
| 16    | 2026-02-19 | Foundation refactors R6–R12: TUI split, DI extraction, error surfacing, magic numbers, missing tests |
| 17    | 2026-02-20 | TUI footer/menu cleanup: remove redundant hints and menu items that duplicate keybinds |

---

## Technical Debt Log

| ID  | Area             | Description                                         | Priority |
|-----|------------------|-----------------------------------------------------|----------|
| D3  | Export versioning | Pack format `0.1.0`; import accepts any version string. Version-aware parsing needed if schema evolves. | Low |
| D7  | Session state    | Session engine holds in-memory queue; one active session per engine instance. Intentional MVP constraint. | Low |

---

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

---

## Changelog

### 2026-02-20 — TUI Footer and Menu Cleanup

**Principle:** footer hints must only advertise keys that have an effect on the
current screen; menu items must not duplicate actions already accessible via
keybinds shown in the footer.

- Added `footerMenuRoot`, `footerRegister`, and `footerBackOnly` footer constants
  to [keymap.go](../internal/adapters/tui/keymap.go).
- **Profile menu**: switched to `footerMenuRoot` (removed non-functional `[Esc] Back`);
  removed "Quit" menu item (duplicates `[Q]`).
- **Profile register**: switched to `footerRegister` (register uses Tab, not ↑/↓,
  so the old `footerMenuSub` was misleading).
- **Home menu**: removed "Switch Profile" (duplicates `[Esc] Back`) and "Quit"
  (duplicates `[Q]`) from menu options.
- **Stats menu**: removed "Back" menu item (duplicates `[Esc] Back`).
- **Stats global / stats pack detail**: switched to `footerBackOnly` — `[↑/↓]` and
  `[Enter]` had no effect on these read-only views.
- **Summary**: removed "Back to Home" menu item (duplicates `[Esc] Back`); replaced
  hardcoded cursor bounds with `len(summaryOptions())-1`.
- Updated `tui_test.go` assertions to match the revised menu contents.

All tests pass. `make check` green.

### 2026-02-19 — Foundation Refactors R1–R5

**R1 — Deduplicate `displayLabelForIndex`**
- Created `internal/domain/choice_label.go` with exported `DisplayLabelForIndex`.
- Removed duplicate implementations from `cmd/golearn/main.go` and
  `internal/adapters/tui/choice_labels.go`; both now call `domain.DisplayLabelForIndex`.

**R2 — Deduplicate `resolveCurrentUser` logic**
- Created `internal/app/resolve_user.go` with `ResolveUser()` and `EnsureDefaultUser()`.
- Removed `resolveCurrentUserID` from `main.go` and `resolveCurrentUser` from `tui/app.go`.
- Both CLI `run` command and TUI `Run()` now use the shared app-layer functions.

**R3 — Deduplicate export pack-building logic**
- Refactored `Export()` in `internal/app/export_pack.go` to delegate to `ExportToBytes()`
  then write the result to disk. Eliminated ~60 lines of duplicated pack-construction code.

**R4 — Replace hand-rolled stdlib reimplementations**
- `selector_difficulty.go`: replaced custom `itoa()` (25 lines) with `strconv.Itoa`.
- `stats_repo.go`: replaced `parseJSONStringArray`, `trimBrackets`, `trimQuotes`,
  `trimSpace`, `splitJSON` (~50 lines) with `encoding/json.Unmarshal`.
- `stats_repo.go`: replaced `sortTagStats` insertion sort with `sort.Slice`.
- `model.go`: replaced `sortPacksByAttempts` insertion sort with `sort.Slice`.

**R5 — Fix N+1 query patterns in `stats_repo.go`**
- `TopicSummaries`: replaced loop of `TopicSummary()` calls (N+1 × 6 queries per
  topic) with a single query using correlated subqueries.
- `TagStats`: replaced inner loop of per-question queries with a single
  `GROUP BY question_id` batch query, then aggregate per-tag in Go.
- `SessionTrend` / `SessionTrendGlobal`: replaced per-session accuracy queries with
  single `GROUP BY s.id` queries using subquery-based session filtering.

All 80+ tests pass. `make check` green.

### 2026-02-19 — Foundation Refactors R6–R12

**R6 — Split TUI `model.go`**
- Moved all 14 `update*()` handlers from `model.go` into their respective
  `screens_*.go` files, co-locating each handler with its view function.
- `model.go` reduced from ~900 lines to ~120 lines (Init/Update/View routing only).

**R7 — Move composition root out of TUI adapter**
- Defined `ports.ConfigStore` interface and `ports.LocalConfig` struct.
- Updated `localconfig.Store` to implement `ports.ConfigStore`.
- Changed `tui.Run()` to accept a `tui.RunParams` struct with all dependencies
  pre-constructed, eliminating `*sql.DB` parameter.
- Moved all DI wiring (repo construction, config loading, user resolution) from
  `tui/app.go` to `cmd/golearn/main.go`.
- Removed `sqlite` and `localconfig` adapter imports from `tui` package.

**R8 — Use `filepath.Join` for path construction**
- Replaced `fmt.Sprintf("%s/%s", dir, entry.Name())` in `import_pack.go` with
  `filepath.Join(dir, entry.Name())`.

**R9 — Surface silently swallowed errors**
- Added `lastError` field to TUI model, displayed in footer via `writeFooter()`.
- Replaced `_ = m.engine.EndSession()`, `_, _ = m.engine.RecordAttempt(...)`,
  and `_ = m.reloadTopicsForCurrentUser()` with proper error capture.
- Stats loading errors (`DifficultyStats`, `TagStats`, `WeakQuestions`,
  `SessionTrend`, `SessionTrendGlobal`) now surface via `statsError`.
- Errors auto-clear on the next keypress.

**R10 — Add missing test coverage**
- Added `internal/app/user_context_test.go` (4 tests).
- Added `internal/adapters/localconfig/config_test.go` (6 tests).
- Added `internal/adapters/pack/reader_test.go` (8 tests).
- Added `internal/app/import_test.go` (4 tests).

**R11 — Remove deprecated `GetNextQuestion`**
- Deleted `GetNextQuestion()` from `session.go`.
- Updated all call sites in `session_test.go` and `export_test.go` to use
  `GetNextSessionQuestion()` with `.Question` dereference.

**R12 — Extract magic numbers to named constants**
- `app.DefaultMinAttempts = 3` — weakest-question selection threshold.
- `app.DefaultMinTagAttempts = 5` — tag-based stats and selection threshold.
- `tui` constants: `statsTrendLimit = 10`, `statsTagMinAttempts = 5`,
  `statsWeakMinAttempts = 3`, `statsWeakQuestionsMax = 10`.
- `cmd/golearn` and `tui`: `defaultQuestionCount = 10`.
- `stats_repo.go`: added descriptive comment to existing `minForWeak` constant.

All tests pass. `make check` green.
