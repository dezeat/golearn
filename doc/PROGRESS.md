# golearn — Progress

## Current Status

| Area            | Status           | Notes                                |
|-----------------|------------------|--------------------------------------|
| Documentation   | ✅ Done          | `workflow.md`, `project.md`, `progress.md` created |
| Go module       | ✅ Scaffold      | `go.mod` initialised; stub `main.go` |
| Domain models   | 🔲 Not started   |                                      |
| Ports/interfaces| 🔲 Not started   |                                      |
| SQLite adapter  | 🔲 Not started   |                                      |
| Pack import     | 🔲 Not started   |                                      |
| Pack export     | 🔲 Not started   |                                      |
| Session engine  | 🔲 Not started   |                                      |
| TUI             | 🔲 Not started   |                                      |
| Makefile        | ✅ Scaffold      | `build`, `test`, `lint`, `check`     |

---

## Changelog

### 2026-02-16 — Initial scaffold

- Created `docs/workflow.md` — agent workflow and code standards
- Created `docs/project.md` — technical spec, data model, pack schema
- Created `docs/progress.md` — this file
- Created `README.md` with project overview and quickstart
- Initialised `go.mod` (`github.com/dezeat/golearn`)
- Added stub `cmd/golearn/main.go`
- Added `Makefile` with `build`, `test`, `lint`, `vet`, `check` targets
- Added example pack `examples/go-basics.yaml`

---

## Next Milestones

### Milestone 1 — Foundation

- [ ] Domain models (`internal/domain/models.go`)
- [ ] Validation logic (`internal/domain/validation.go`)
- [ ] Stable hashing (`internal/domain/hashing.go`)
- [ ] Port interfaces (`internal/ports/`)
- [ ] SQLite adapter: open, migrate, WAL pragma (`internal/adapters/sqlite/db.go`)
- [ ] SQLite repo implementations (topic, question)
- [ ] Unit tests for domain + hashing

### Milestone 2 — Import / Export + Session Engine

- [ ] Pack reader: YAML + JSON (`internal/adapters/pack/`)
- [ ] Import use case with validation + dedup (`internal/app/import_pack.go`)
- [ ] Export use case with stable ordering (`internal/app/export_pack.go`)
- [ ] Session + attempt repos (SQLite)
- [ ] Question selector (unseen → weak → random fill)
- [ ] `start_session` + `record_attempt` use cases
- [ ] CLI wiring: `golearn import`, `golearn export`

### Milestone 3 — TUI + Polish

- [ ] Bubble Tea app shell (`internal/adapters/tui/`)
- [ ] Screens: topic select, session config, question, summary
- [ ] `golearn tui` command
- [ ] End-to-end integration tests
- [ ] CI pipeline (`make check` in GitHub Actions)

---

## Technical Debt Log

| ID  | Area            | Description                                         | Priority |
|-----|-----------------|-----------------------------------------------------|----------|
| D1  | Lint            | `golangci-lint` config not yet created (`.golangci.yml`) | Medium  |
| D2  | Export ordering  | Need to decide and document canonical sort column (`created_at` vs `id`) | Medium |
| D3  | Export versioning| Pack format is `0.1.0`; no migration strategy yet for schema changes | Low |
| D4  | Migrations       | No migration tool chosen yet; evaluate `golang-migrate` vs embedded SQL | Medium |
| D5  | CLI framework    | `cobra` vs stdlib `flag` not decided                | Low      |
