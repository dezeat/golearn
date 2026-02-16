# golearn — Agent Workflow

## Agent Identity

You are a **blank slate each session**. You have no memory of prior sessions.
Every session begins by reading the grounding documents before writing any code.

## Mandatory Grounding (every session)

1. `docs/workflow.md` — this file (process + standards)
2. `docs/project.md` — technical spec, data model, repo layout
3. `docs/progress.md` — current status, changelog, open debt

Only after reading all three should you plan or implement.

## Operational Loop

```
Understand → Plan → Implement → Validate → Document
```

| Phase      | Action                                                        |
|------------|---------------------------------------------------------------|
| Understand | Read grounding docs; clarify ambiguities with the user        |
| Plan       | Break work into small, ordered tasks; update `progress.md`    |
| Implement  | Write code in small increments; commit logical units          |
| Validate   | Run `make check`; fix until green                             |
| Document   | Update `progress.md` changelog + status; note new tech debt   |

## Quality Gates

- **Never finish a session** without running `make check` (lint + test + vet).
  If the Makefile doesn't exist yet, note it in `progress.md` as debt.
- All tests must be **deterministic** — no time-dependent or random assertions
  without fixed seeds.
- Exported pack files must be **byte-stable** given the same data.

## Pre-Completion Checklist

Before ending any implementation session, verify:

- [ ] `make check` passes (or gate documented as not-yet-available)
- [ ] `docs/progress.md` changelog updated with today's work
- [ ] New public functions have purpose-driven comments
- [ ] No secrets, credentials, or personal paths in committed code
- [ ] Any new tech debt logged in `progress.md`

## Code Standards

| Rule                    | Detail                                                     |
|-------------------------|-------------------------------------------------------------|
| Comments                | Purpose-driven: explain *why*, not *what*                   |
| Secrets                 | Never commit API keys, tokens, or personal paths            |
| Tests                   | Deterministic; table-driven where appropriate               |
| Error handling          | Wrap errors with `fmt.Errorf("context: %w", err)`          |
| Naming                  | Follow Go conventions — exported = public, unexported = pkg |
| Dependencies            | Minimise; justify new deps in PR/commit message             |

## Refactoring Triggers

Refactor when any of these are true:

- A single function exceeds ~60 lines of logic
- A package has more than one responsibility
- Duplicate code appears in ≥2 locations
- A test requires complex setup that masks intent
- An interface has >5 methods (split by concern)

## Troubleshooting Quick Reference

| Symptom                              | Likely Cause                            | Fix                                                      |
|--------------------------------------|-----------------------------------------|----------------------------------------------------------|
| Duplicate imports not detected       | Hash uses un-normalised text            | Trim whitespace, canonical line endings before hashing    |
| YAML parse error on valid file       | Strict YAML decoder vs. lenient input   | Use `yaml.v3` with `KnownFields(true)` for safety        |
| SQLite "database is locked"          | Concurrent writers without WAL          | Enable `PRAGMA journal_mode=WAL` on open                  |
| Lint failures on CI                  | Local Go version drift                  | Pin Go version in `go.mod`; use `golangci-lint` config    |
| Export ordering unstable             | Non-deterministic query order           | Always `ORDER BY` on a stable column (`created_at`, `id`) |
| `--db` flag ignored                  | Flag parsed after subcommand            | Register persistent flags at root command level           |
| Choice IDs collide across questions  | IDs not scoped to question              | Choice IDs are local to their question; hash includes all |
