# Contributing to golearn

Thanks for contributing to golearn.

## Prerequisites

- Go 1.25+ (language floor; the pinned build toolchain is in `go.mod`)
- `make`
- Optional: `golangci-lint` (required for full `make check`)

## Local Setup

```bash
git clone https://github.com/dezeat/golearn.git
cd golearn
make build
```

### Git hooks (opt-in)

Install local hooks that mirror the CI gate:

```bash
make hooks
```

This points `core.hooksPath` at `.githooks/`. The `pre-commit` hook runs
`gofmt` on staged Go files, `go vet`, and a large-file guard (rejects any
staged file over 1 MB); `pre-push` runs the full `make check`.

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

golearn uses [Conventional Commits](https://www.conventionalcommits.org/).
Releases and the `CHANGELOG.md` are generated from commit history, so the
format is load-bearing — not just a style preference.

Shape: `type(scope): subject`

- **type** — one of `feat`, `fix`, `refactor`, `test`, `docs`, `chore`,
  `perf`, `build`, `ci`.
- **scope** — optional; the area touched (e.g. `sqlite`, `tui`, `export`).
- **subject** — imperative mood, no trailing period, target <= 72 chars.
- One logical change per commit.

Examples:
- `feat: add json export flag`
- `fix: correct stats query for weak questions`
- `docs: update quickstart`

`feat:` and `fix:` drive minor and patch version bumps respectively; a
`!` after the type/scope (or a `BREAKING CHANGE:` footer) signals a major
bump.

### No AI attribution

Commit messages and PR text must contain **no AI attribution or tool
mentions** — no `Co-Authored-By` trailers for an agent, no "Generated with"
lines, no reference to the tooling used to author the change. This keeps the
history about the change, not the authoring method.

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
