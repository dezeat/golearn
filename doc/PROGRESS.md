# golearn — Progress

## ✅ MVP COMPLETE

All core capabilities are implemented, tested, and documented.

## Current Status

| Area            | Status           | Notes                                |
|-----------------|------------------|--------------------------------------|
| Documentation   | ✅ Done          | `WORKFLOW.md`, `PROJECT.md`, `PROGRESS.md`, `SPEC.md` |
| Go module       | ✅ Done          | `go.mod` with yaml.v3 + modernc.org/sqlite + bubbletea |
| Domain models   | ✅ Done          | `models.go`, `validation.go`, `hashing.go`, `correctness.go` |
| Ports/interfaces| ✅ Done          | `repositories.go`, `sources.go`      |
| Pack reader     | ✅ Done          | YAML + JSON parsing                  |
| SQLite adapter  | ✅ Done          | DB open, WAL, FK, migrations, all repos |
| Import use case | ✅ Done          | Validate → normalise → hash → dedupe → insert |
| CLI (import)    | ✅ Done          | `golearn import <path>` with `--db` flag |
| Session engine  | ✅ Done          | StartSession, GetNextQuestion, RecordAttempt, EndSession |
| Selection policy| ✅ Done          | Unseen → weak → random fill          |
| CLI (run)       | ✅ Done          | `golearn run <topic-slug> --n N`     |
| Export use case | ✅ Done          | Deterministic ordering, YAML + JSON output |
| CLI (export)    | ✅ Done          | `golearn export <slug> --out <path> [--format]` |
| Bubble Tea TUI  | ✅ Done          | Topic select, config, question, feedback, summary |
| CLI (tui)       | ✅ Done          | `golearn tui` with alt-screen        |
| Example packs   | ✅ Done          | `go-basics.yaml`, `mvp-basics.yaml`, `databricks-pde.yaml` |
| CLI (help)      | ✅ Done          | `golearn help` with examples         |
| Tests           | ✅ Done          | 31 tests: validation, hashing, correctness, selector, session, export, integration |
| Makefile        | ✅ Done          | `fmt`, `vet`, `lint`, `test`, `check` |
| Lint config     | ✅ Done          | `.golangci.yml` with sensible rules  |
| CI pipeline     | ✅ Done          | GitHub Actions workflow (`.github/workflows/ci.yml`) |

---

## Changelog

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
| D8  | TUI testing      | TUI screens have no unit tests (Bubble Tea model testing) | Medium |
