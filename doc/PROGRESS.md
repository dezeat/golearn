# golearn — Progress

## Current Status

| Area            | Status           | Notes                                |
|-----------------|------------------|--------------------------------------|
| Documentation   | ✅ Done          | `WORKFLOW.md`, `PROJECT.md`, `PROGRESS.md` |
| Go module       | ✅ Done          | `go.mod` with yaml.v3 + modernc.org/sqlite |
| Domain models   | ✅ Done          | `models.go`, `validation.go`, `hashing.go` |
| Ports/interfaces| ✅ Done          | `repositories.go`, `sources.go`      |
| Pack reader     | ✅ Done          | YAML + JSON parsing                  |
| SQLite adapter  | ✅ Done          | DB open, migrations, topic + question repos |
| Import use case | ✅ Done          | Validate → normalise → hash → dedupe → insert |
| CLI (import)    | ✅ Done          | `golearn import <path>` with `--db` flag |
| Tests           | ✅ Done          | Validation, hashing, SQLite dedupe   |
| Makefile        | ✅ Done          | `fmt`, `vet`, `lint`, `test`, `check` |
| Pack export     | 🔲 Not started   |                                      |
| Session engine  | 🔲 Not started   |                                      |
| TUI             | 🔲 Not started   |                                      |

---

## Changelog

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

### Milestone 2 — Export + Session Engine

- [ ] Export use case with stable ordering (`internal/app/export_pack.go`)
- [ ] Session + attempt repos (SQLite)
- [ ] Question selector (unseen → weak → random fill)
- [ ] `start_session` + `record_attempt` use cases
- [ ] CLI wiring: `golearn export`

### Milestone 3 — TUI + Polish

- [ ] Bubble Tea app shell (`internal/adapters/tui/`)
- [ ] Screens: topic select, session config, question, summary
- [ ] `golearn tui` command
- [ ] End-to-end integration tests
- [ ] CI pipeline (`make check` in GitHub Actions)

---

## Technical Debt Log

| ID  | Area             | Description                                         | Priority |
|-----|------------------|-----------------------------------------------------|----------|
| D1  | Lint             | `golangci-lint` config not yet created (`.golangci.yml`) | Medium  |
| D2  | Export ordering  | Need to decide canonical sort column (`created_at` vs `id`) | Medium |
| D3  | Export versioning| Pack format `0.1.0`; no migration strategy yet for schema changes | Low |
| D4  | CLI framework    | Using manual arg parsing; consider `cobra` for subcommands | Low |
| D5  | Error reporting  | Pack-level errors stop the file; consider partial import | Low |
