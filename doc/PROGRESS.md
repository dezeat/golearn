# golearn — Progress

## Current Status

| Area            | Status           | Notes                                |
|-----------------|------------------|--------------------------------------|
| Documentation   | ✅ Done          | `WORKFLOW.md`, `PROJECT.md`, `PROGRESS.md` |
| Go module       | ✅ Done          | `go.mod` with yaml.v3 + modernc.org/sqlite + bubbletea |
| Domain models   | ✅ Done          | `models.go`, `validation.go`, `hashing.go`, `correctness.go` |
| Ports/interfaces| ✅ Done          | `repositories.go`, `sources.go`      |
| Pack reader     | ✅ Done          | YAML + JSON parsing                  |
| SQLite adapter  | ✅ Done          | DB open, migrations, all repos       |
| Import use case | ✅ Done          | Validate → normalise → hash → dedupe → insert |
| CLI (import)    | ✅ Done          | `golearn import <path>` with `--db` flag |
| Session engine  | ✅ Done          | StartSession, GetNextQuestion, RecordAttempt, EndSession |
| Selection policy| ✅ Done          | Unseen → weak → random fill          |
| CLI (run)       | ✅ Done          | `golearn run <topic-slug> --n N`     |
| Export use case | ✅ Done          | Deterministic ordering, YAML + JSON output |
| CLI (export)    | ✅ Done          | `golearn export <slug> --out <path> [--format]` |
| Bubble Tea TUI  | ✅ Done          | Topic select, config, question, feedback, summary |
| CLI (tui)       | ✅ Done          | `golearn tui` with alt-screen        |
| Example pack    | ✅ Done          | `examples/mvp-basics.yaml` (10 questions) |
| CLI (help)      | ✅ Done          | `golearn help` with examples         |
| Tests           | ✅ Done          | 31 tests: validation, hashing, correctness, selector, session, export, integration |
| Makefile        | ✅ Done          | `fmt`, `vet`, `lint`, `test`, `check` |

---

## Changelog

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

### 2026-02-16 — Session engine + selection policy + CLI run loop (Phase 2)

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
- Added CLI `golearn run <topic-slug> [--n N]` command:
  - Interactive stdin loop with intro/prompt/choices display
  - Supports comma-separated choice IDs, 's' to skip, 'q' to quit
  - Measures answer latency, prints correct/incorrect feedback
  - Prints session summary: total answered, correct count, accuracy %
- Added 16 new tests (27 total):
  - Correctness: 9 table-driven cases (exact match, order-insensitive, edge cases)
  - Selector: 6 tests (unseen priority, weak sorting, cap, determinism, empty input)
  - Session lifecycle: 5 tests (full lifecycle, correctness eval, skip, topic not found, stats affect selection)
- All tests deterministic with fixed random seeds

### 2026-02-16 — Foundation layer

- Implemented domain models: `Topic`, `Question`, `Session`, `Attempt`, `Pack*` types
- Implemented validation (7 rules): type, prompt, choices ≥2, unique IDs, correct refs, single_select count
- Implemented stable hashing: SHA-256 over normalised content with null-byte separators
- Implemented normalisation: whitespace trim, `\r\n` → `\n`
- Defined port interfaces: `TopicRepository`, `QuestionRepository`, `PackReader`
- Implemented pack reader adapter: YAML (`yaml.v3`) + JSON, file + directory support
- Implemented SQLite adapter: `Open()` with WAL + FK pragmas, embedded migrations
- Implemented SQLite repos: `TopicRepo` (upsert by slug), `QuestionRepo` (INSERT OR IGNORE by hash)
- Implemented import use case: parse → normalise → validate → hash → upsert topic → insert questions
- Wired CLI: `golearn import <path>` with `--db` flag (default `~/.golearn/golearn.db`)
- Added unit tests: validation (good + bad), hashing stability, SQLite insert + dedupe
- Updated Makefile: added `fmt` target, `check` = fmt + vet + lint + test
- Updated `.gitignore`: added `.golearn/`, `bin/`, `dist/`, `tmp/`
- Dependencies: `gopkg.in/yaml.v3`, `modernc.org/sqlite` (CGo-free)

### 2026-02-16 — Initial scaffold

- Created `doc/WORKFLOW.md`, `PROJECT.md`, `PROGRESS.md`
- Created `README.md` with project overview and quickstart
- Initialised `go.mod` (`github.com/dezeat/golearn`)
- Added stub `cmd/golearn/main.go`
- Added `Makefile` with `build`, `test`, `lint`, `vet`, `check` targets
- Added example pack `examples/go-basics.yaml`

---

## Next Milestones

### Milestone 4 — Polish + CI

- [ ] CI pipeline (`make check` in GitHub Actions)
- [ ] `golangci-lint` config (`.golangci.yml`)
- [ ] Consider `cobra` for CLI if more subcommands are needed
- [ ] Lipgloss styling improvements for TUI
- [ ] `golearn stats` command for viewing per-topic statistics

---

## Technical Debt Log

| ID  | Area             | Description                                         | Priority |
|-----|------------------|-----------------------------------------------------|----------|
| D1  | Lint             | `golangci-lint` config not yet created (`.golangci.yml`) | Medium  |
| D2  | ~~Export ordering~~  | ~~Need to decide canonical sort column~~ — Resolved: `created_at ASC`, hash tie-break | Done |
| D3  | Export versioning| Pack format `0.1.0`; no migration strategy yet for schema changes | Low |
| D4  | CLI framework    | Using manual arg parsing; consider `cobra` for subcommands | Low |
| D5  | Error reporting  | Pack-level errors stop the file; consider partial import | Low |
| D6  | ~~Topic lookup~~     | ~~`StartSession` lists all topics then filters~~ — Acceptable for MVP scale | Closed |
| D7  | Session state    | Session engine holds in-memory queue; only one active session per engine instance | Low |
| D8  | TUI testing      | TUI screens have no unit tests (Bubble Tea model testing) | Medium |
| D9  | CI pipeline      | No GitHub Actions workflow yet for automated `make check` | Medium |
