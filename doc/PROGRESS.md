# golearn — Progress

## ✅ MVP COMPLETE

All core capabilities are implemented, tested, and documented.

## Current Status

| Area            | Status           | Notes                                |
|-----------------|------------------|--------------------------------------|
| Documentation   | ✅ Done          | `WORKFLOW.md`, `PROJECT.md`, `PROGRESS.md`, `SPEC.md` |
| Go module       | ✅ Done          | `go.mod` with yaml.v3 + modernc.org/sqlite + bubbletea |
| Domain models   | ✅ Done          | `models.go`, `validation.go`, `hashing.go`, `correctness.go` |
| Ports/interfaces| ✅ Done          | `repositories.go` (incl. StatsRepository), `sources.go` |
| Pack reader     | ✅ Done          | YAML + JSON parsing                  |
| SQLite adapter  | ✅ Done          | DB open, WAL, FK, migrations, all repos incl. stats |
| Import use case | ✅ Done          | Validate → normalise → hash → dedupe → insert |
| CLI (import)    | ✅ Done          | `golearn import <path>` with `--db` flag |
| Session engine  | ✅ Done          | StartSession, GetNextQuestion, RecordAttempt, EndSession |
| Selection policy| ✅ Done          | Unseen → weak → random fill          |
| CLI (run)       | ✅ Done          | `golearn run <topic-slug> --n N`     |
| Export use case | ✅ Done          | Deterministic ordering, YAML + JSON output |
| CLI (export)    | ✅ Done          | `golearn export <slug> --out <path> [--format]` |
| Bubble Tea TUI  | ✅ Done          | Profile menu, home menu, topic select, config, question, feedback, summary, stats |
| CLI (tui)       | ✅ Done          | `golearn tui` with alt-screen        |
| Home Menu       | ✅ Done          | Post-login hub: practice, review, stats, switch profile, quit |
| Stats feature   | ✅ Done          | Global, per-pack, difficulty, tags, weak questions, trends |
| Example packs   | ✅ Done          | `go-basics.yaml`, `mvp-basics.yaml`, `databricks-pde.yaml` |
| CLI (help)      | ✅ Done          | `golearn help` with examples         |
| Tests           | ✅ Done          | 59 tests: validation, hashing, correctness, selector, session, export, integration, profiles, stats |
| Makefile        | ✅ Done          | `fmt`, `vet`, `lint`, `test`, `check` |
| Lint config     | ✅ Done          | `.golangci.yml` with sensible rules  |
| CI pipeline     | ✅ Done          | GitHub Actions workflow (`.github/workflows/ci.yml`) |

---

## Changelog

### 2026-02-17 — Phase 9: Session Shuffle + Stats Timestamp Polish

- Added session-scoped answer shuffling in `SessionEngine`:
  - Introduced `SessionQuestion` with original `Question` pointer + `ShuffledChoices`
  - Choices are copied and shuffled per session using the existing session RNG seed
  - No DB writes/schema changes and no changes to `domain.Question`
- Preserved correctness logic using original choice IDs:
  - `RecordAttempt` still evaluates against original `CorrectChoiceIDs`
  - Multi-select remains order-insensitive via existing `domain.EvaluateCorrectness`
  - Explanation mapping (`rationale.per_choice`) remains unchanged since shuffled choices retain original IDs
- Updated consumers to render shuffled choices:
  - TUI question/review screens render `ShuffledChoices`
  - CLI `run` command also displays session-shuffled choice order
- Updated Pack Detail Stats timestamp formatting:
  - `Last:` now renders as `YYYY-MM-DD HH:MM`
  - Seconds removed using `t.Format("2006-01-02 15:04")`
  - Added robust parsing fallback for SQLite/RFC3339 timestamp variants
- Added tests:
  - Deterministic shuffle behavior with fixed RNG seed
  - Correctness preserved when display order changes (including multi-select)
  - Timestamp formatting test ensuring minute precision output

### 2026-02-17 — Phase 8: Stats Feature + Home Menu Navigation

- Added post-login Home Menu with 5 options:
  - Start Practice, Review Wrong Answers, Stats, Switch Profile, Quit
  - Profile login/register now routes to Home Menu instead of direct topic select
  - Topic select and review browse navigate back to Home Menu
- Implemented `StatsRepository` interface in `ports/repositories.go`:
  - `GlobalStats`: overall accuracy, answered/skipped, latency, most practiced, weakest topic
  - `TopicSummary`: per-topic coverage, accuracy, attempts, latency, last practiced
  - `DifficultyStats`: per-difficulty-bucket breakdown (Easy/Medium/Hard/Unrated)
  - `TagStats`: per-tag accuracy with minimum attempts filter
  - `WeakQuestions`: worst-performing questions ranked by wrong rate
  - `SessionTrend` / `SessionTrendGlobal`: accuracy per session for sparkline
- Implemented `sqlite/stats_repo.go` with SQL aggregate queries
  - All queries filtered by `user_id` for multi-user isolation
  - Difficulty bucketing: 1–2 → Easy, 3 → Medium, 4–5 → Hard, 0/NULL → Unrated
  - Tag stats use Go-side JSON array parsing (SQLite compatibility)
  - Weak questions exclude 100%-correct questions
- Added 3 new TUI screens:
  - Global Stats: accuracy, time, trend sparkline, most/weakest packs
  - Pack Stats List: fixed-column table of all packs with accuracy/attempts
  - Pack Detail Stats: header metrics, difficulty breakdown, weak/strong tags,
    weakest questions list, session trend sparkline with delta
- Updated Summary screen with navigable menu:
  - Review incorrect questions (if any)
  - View stats for this pack → Pack Detail Stats
  - Back to Home
- Added sparkline rendering with Unicode blocks (▁▂▃▄▅▆▇█)
  - Trend delta indicator (↑/↓/→) comparing first to last session
- Added 6 new stats aggregation tests:
  - `TestStatsRepo_MultiUserIsolation`: two-user same-topic correctness isolation
  - `TestStatsRepo_DifficultyBucketing`: difficulty 1–5 + 0 mapping verification
  - `TestStatsRepo_SessionTrend`: multi-session ascending accuracy verification
  - `TestStatsRepo_WeakQuestions`: wrong_rate ordering + minAttempts filter
  - `TestStatsRepo_TopicSummary_Coverage`: coverage %, seen questions
  - `TestStatsRepo_GlobalStats_Skipped`: skipped excluded from accuracy
- Added 7 new TUI tests:
  - `TestHomeMenuView`: home menu options rendering
  - `TestHomeMenuNavigation`: cursor movement
  - `TestSparkline`: sparkline rendering (4 sub-tests)
  - `TestTrendDelta`: delta formatting (4 sub-tests)
  - `TestTruncate`: string truncation
  - `TestStatsGlobalViewEmpty`: empty state for global stats
  - `TestSummaryViewHasStatsOption`: stats + home options in summary
- Updated `doc/PROJECT.md`:
  - Added Home Menu navigation flow
  - Added Stats metrics definitions
  - Added difficulty bucketing mapping
  - Updated repository structure (stats_repo.go, screens_stats.go)
- `make check` passes

### 2026-02-17 — Phase 7: Local Multi-User Profiles

- Added local profile system (no passwords, no network auth)
  - New `users` table with unique `handle`, optional `display_name`, timestamps
  - Seeded default profile: `local` / `Local`
- Added persisted current user config at `~/.golearn/config.json`
  - Stores `current_user_id`
  - Missing/invalid config falls back to seeded `local` and rewrites config
  - Config path is injectable in code paths used by tests
- Made sessions/attempts/stats user-scoped
  - Added `user_id` to `sessions` and `attempts`
  - Added indexes: `sessions(user_id, topic_id, started_at)`,
    `attempts(user_id, question_id, created_at)`, `attempts(user_id, session_id)`
  - Selection policy stats and topic accuracy now filter by current `user_id`
  - Review mode remains read-only and only reflects current user's mistakes
- Replaced startup intro with profile menu flow in TUI
  - Menu: Continue, Login, Register, Quit
  - Login: pick existing profile
  - Register: handle validation (`a-z`, `0-9`, `-`, `_`) + uniqueness
  - Successful login/register sets current profile and persists config
- Added schema compatibility reset strategy for dev phase
  - On startup, if legacy schema (missing user columns/tables) is detected,
    DB schema is dropped and recreated automatically
- Added tests
  - SQLite user repository create/list/get + unique handle enforcement
  - Two-user stats scoping correctness (`user A` not polluted by `user B`)
  - TUI profile selection state sets current user context
- `make check` passes

### 2026-02-17 — Phase 6: TUI UX Polish

- Fixed text overflow: all long text now hard-wraps to terminal width
  - Created `internal/adapters/tui/wrap.go` with `wrapText` and `wrapAndIndent` helpers
  - Uses `WindowSizeMsg` width with `contentWidth(padding)` method (default 80, min content 20)
  - Wrapping applied to: intro, prompt, choice text, rationale.correct, rationale.per_choice
  - Continuation lines use consistent indentation aligned to content start
- Redesigned review mode as read-only browse (no new quiz session)
  - New `screenReviewBrowse` mode: cycles through wrong answers without recording attempts
  - Shows correct answers (green ✔), user's wrong selections (red ✘), and explanations
  - Tracks user's original selections via `wrongAnswer{Question, SelectedIDs}` struct
  - Controls: enter/n/→ next, p/← previous, e toggle explanations, q exit to topics
  - Removed old review-as-quiz flow that created a new answering session
- Fixed question-count selection controls to match visual hints
  - Session config now uses ←/→ (and j/k) to increment/decrement count
  - Hint text updated from "↑/↓" to "←/→" to match the ◀ N ▶ display
- Implemented tabular topic list with fixed-column layout
  - Column 1: topic name (variable width, truncated with "…" to fit)
  - Column 2: "N questions" (right-aligned, 14-char column)
  - Column 3: "N%" accuracy (right-aligned, 5-char column)
  - Layout adapts to terminal width; never overflows horizontally
  - Selected row highlighting preserves column alignment
- Added 5 new tests (42 total):
  - `TestWrapText`: 12 table-driven sub-tests covering word wrap, long words, newlines, edge cases
  - `TestWrapText_NoLineExceedsWidth`: property test verifying no output line exceeds width
  - `TestWrapAndIndent`: indent + wrap integration
  - `TestReviewBrowseView`: browse view renders header, prompt, and controls
  - `TestContentWidth`: padding and default width calculations
- Updated `TestReviewSessionSetup` for new browse-mode review (wrongAnswer type, screenReviewBrowse)
- All 42 tests pass, `make check` green

### 2026-02-17 — Phase 5: UX Polish + Explained Pack + Question Standard

- Created `examples/databricks-pde-explained.yaml`: 15-question PDE pack with full rationale
  - 10 single_select + 5 multi_select questions
  - Every question includes `rationale.correct` and `rationale.per_choice` explanations
  - Topics: Auto Loader schema, Delta ACID, OPTIMIZE+ZORDER, VACUUM, CDF, checkpointing,
    watermarks, trigger modes, Unity Catalog permissions, medallion architecture,
    isolation levels, deletion vectors, stream-static joins, MERGE, table constraints
  - All answers sourced from official Databricks documentation
- Upgraded TUI to quiz-show review mode:
  - After submitting: full question + choices remain visible with colour-coded feedback
  - Green ✔ for correct choices, red ✘ for incorrect selected choices
  - Press 'e' to toggle per-choice explanations and rationale
  - Explicit enter required to proceed to next question
  - Uses Lip Gloss styling for visual clarity
- Added ASCII intro splash screen on TUI startup (press any key to continue)
- Improved session summary:
  - Shows accuracy %, average response time
  - Shows count of wrong questions with review suggestion
  - Press 'r' from summary to replay wrong questions in review mode
- Added review mode: replays only incorrectly answered questions from last session
- Fixed export to include rationale data (was previously omitted)
- Created `doc/QUESTIONS.md`: formal question authoring standard
  - Question philosophy, structure rules, explanation rules
  - Tone guidelines, validation checklist, reference policy
  - Gold standard example question
- Added 8 new tests (37 total):
  - `TestImport_PDEExplainedPack`: validates pack imports with rationale integrity
  - `TestExportRoundtrip_WithRationale`: verifies rationale survives export/reimport
  - `TestReviewState_ExplanationToggle`: TUI explanation toggle state logic
  - `TestReviewState_CorrectFeedback`: correct answer visual feedback
  - `TestReviewState_IncorrectFeedback`: incorrect answer visual feedback
  - `TestReviewState_SkippedFeedback`: skipped answer visual feedback
  - `TestIntroScreen`: ASCII intro screen renders
  - `TestReviewSessionSetup`: review session initialisation
- Updated README with new pack, TUI features, QUESTIONS.md doc link
- All 37 tests pass, `make check` green

### 2026-02-16 — Phase 4: Polish + Product Reframe + Content Expansion

- Replaced `doc/SPEC.md` with business-unit product specification:
  - Product vision, value proposition, target personas
  - Use cases, deployment scenarios, differentiation matrix
  - 6–12 month roadmap with phased milestones
  - Enterprise expansion opportunities (LLM generation, team assessment)
- Resolved technical debt:
  - D1: Added `.golangci.yml` with errcheck, govet, staticcheck, gocritic, misspell
  - D5: Documented import error handling strategy (fail-per-file, not fail-fast)
  - D6: Added `GetBySlug` to `TopicRepository` and `TopicRepo`; refactored
    `session.go` and `export_pack.go` to use direct lookup instead of List()+filter
  - D9: Added `.github/workflows/ci.yml` (fmt + vet + lint + test + build + smoke test)
- Updated `doc/PROJECT.md`:
  - Repository structure matches actual file layout
  - Added CLI framework decision rationale (stdlib vs cobra)
  - Added pack versioning strategy
  - Added "Export Guarantees" section
  - Added "Determinism Guarantees" section
  - Added "Session Engine" constraints documentation
  - Corrected hashing and selection policy descriptions
- Updated `doc/PROGRESS.md`:
  - Added "MVP COMPLETE" marker
  - Cleaned technical debt log (removed resolved items, updated remaining)
  - Added Phase 4 changelog entry
- Created `examples/databricks-pde.yaml`: 30-question Databricks PDE certification pack
  - 20 single_select + 10 multi_select questions
  - Topics: Auto Loader, Delta Lake, Structured Streaming, Unity Catalog,
    Change Data Feed, VACUUM, ZORDER, medallion architecture, DLT, checkpointing,
    watermarks, CDC, partitioning, isolation levels
  - Source references to official Databricks documentation
- Improved CLI help formatting with clearer structure
- Improved import summary output formatting
- All tests pass, `make check` green

### 2026-02-16 — Phase 3: Bubble Tea TUI + Export + MVP polish

- Implemented `internal/app/export_pack.go`: pack export use case
  - Export topics to canonical YAML or JSON format
  - Deterministic ordering: `created_at ASC`, hash for tie-breaking
  - Only includes optional fields when they have meaningful values
  - `ExportToBytes()` method for in-memory testing
- Implemented Bubble Tea TUI under `internal/adapters/tui/`:
  - `app.go`: TUI entry point with `Run(db)`, topic metadata loading
  - `model.go`: Bubble Tea model with `Init/Update/View`, screen routing
  - `screens_topic.go`: topic selection with question counts and accuracy %
  - `screens_session.go`: session configuration (adjust question count)
  - `screens_question.go`: question display with choice navigation, selection, feedback
  - `screens_summary.go`: session summary with total/correct/accuracy
  - Full TUI flow: topics → config → question → feedback → summary → back to topics
  - Controls: ↑/↓ or j/k navigate, space toggle, enter submit, s skip, q quit
- Wired CLI commands:
  - `golearn tui` — launches Bubble Tea TUI with alt-screen
  - `golearn export <slug> --out <path> [--format yaml|json]` — exports topic to file
  - `golearn help` / `--help` / `-h` — improved help with examples
  - Format auto-detection from file extension for export
- Created `examples/mvp-basics.yaml`: 10 questions (5 single_select, 5 multi_select)
  - Covers Go basics, CLI, databases, concurrency, general programming
  - Mixed formats: with/without intro, 2/4/5 choices, varied difficulty
- Added 4 new tests (31 total):
  - `TestExportRoundtrip`: import → export → re-import → 0 duplicates
  - `TestExportDeterministic`: two exports produce identical output
  - `TestExportJSON`: JSON export and re-import roundtrip
  - `TestIntegration_ImportAndSession`: end-to-end import → session → stats verification
- Added dependencies: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`
- Updated README with TUI, export, and example pack documentation
- All 31 tests pass, `make check` green

### 2026-02-16 — Phase 2: Session engine + selection policy + CLI run loop

- Added `domain/correctness.go`: centralised answer evaluation — exact, order-insensitive set match
- Extended `ports/repositories.go`: added `SessionRepository`, `AttemptRepository`, `QuestionStats` type
- Implemented `sqlite/session_repo.go`: Create (insert session row) + Finish (set ended_at)
- Implemented `sqlite/attempt_repo.go`: Record (insert attempt) + StatsByTopic (aggregated per-question stats)
- Implemented `app/selector.go`: question selection policy — unseen → weak (highest wrong rate) → random fill
  - Uses seeded `*rand.Rand` for test determinism
  - Weak bucket sorted by wrong rate descending
- Implemented `app/session.go`: full session lifecycle engine
  - `StartSession(topicSlug, n, mode)` — validates topic, selects questions, persists session
  - `GetNextQuestion()` — serves from in-memory queue, no duplicates
  - `RecordAttempt()` — evaluates correctness via domain layer, persists attempt
  - `EndSession()` — sets ended_at timestamp
- Added CLI `golearn run <topic-slug> [--n N]` command
- Added 16 new tests (27 total)
- All tests deterministic with fixed random seeds

### 2026-02-16 — Phase 1: Foundation layer

- Implemented domain models: `Topic`, `Question`, `Session`, `Attempt`, `Pack*` types
- Implemented validation (7 rules), stable hashing, normalisation
- Defined port interfaces, implemented pack reader, SQLite adapter, import use case
- Wired CLI: `golearn import <path>` with `--db` flag
- Added unit tests, Makefile, `.gitignore`
- Dependencies: `gopkg.in/yaml.v3`, `modernc.org/sqlite`

### 2026-02-16 — Phase 0: Initial scaffold

- Created doc files, README, `go.mod`, stub `main.go`, Makefile
- Added example pack `examples/go-basics.yaml`

---

## Technical Debt Log

| ID  | Area             | Description                                         | Priority |
|-----|------------------|-----------------------------------------------------|----------|
| D3  | Export versioning | Pack format `0.1.0`; import accepts any version string. Version-aware parsing needed if schema evolves. | Low |
| D7  | Session state    | Session engine holds in-memory queue; one active session per engine instance. Intentional MVP constraint. | Low |
