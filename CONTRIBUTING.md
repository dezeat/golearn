# Contributing to golearn

Thanks for contributing to golearn.

## Prerequisites

- Go 1.24.0 (or newer compatible stable release)
- `make`
- Optional: `golangci-lint` (required for full `make check`)

## Local Setup

```bash
git clone https://github.com/dezeat/golearn.git
cd golearn
make build
```

## Development Workflow

1. Create a branch from `main`.
2. Make focused, small changes.
3. Run quality gates locally.
4. Open a Pull Request.

### Branch Naming

Use short, descriptive branch names:

- `feat/<short-description>`
- `fix/<short-description>`
- `docs/<short-description>`
- `refactor/<short-description>`
- `chore/<short-description>`

Examples:
- `feat/add-json-export-flag`
- `fix/session-end-error`

## Build, Test, and Quality Gates

```bash
make build
make test
make check
```

`make check` is the project gate and runs formatting, vet, lint, and tests.

## Code Style

- Follow existing package boundaries (`domain`, `ports`, `app`, `adapters`, `cmd`).
- Keep changes minimal and task-focused.
- Prefer clear, simple implementations over clever abstractions.
- Use `filepath.Join` for paths.
- Wrap errors with context using `%w`.
- Keep tests deterministic and table-driven when appropriate.

## Commit Format

- Use imperative tense.
- Keep the first line concise (target: <= 72 chars).
- One logical change per commit.

Examples:
- `Add profile validation for empty handles`
- `Fix stats query for weak questions`
- `Update README quickstart examples`

## Pull Request Process

PRs should include:

- Clear summary of what changed and why
- Notes on risks/trade-offs (if any)
- Test evidence (`make check` output or equivalent)
- Linked issue (if applicable)

Before requesting review, verify:

- `make check` passes
- No unrelated files are modified
- Relevant docs are updated

## Reporting Issues

Use GitHub issue templates for bug reports and feature requests.
