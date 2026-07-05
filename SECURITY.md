# Security Policy

## Reporting a Vulnerability

Please report security vulnerabilities **privately** through GitHub Security
Advisories, not through public issues:

**https://github.com/dezeat/golearn/security/advisories/new**

We aim to acknowledge a report within **7 days** and will keep you updated as
we investigate and prepare a fix. Once a fix is available, we will coordinate
disclosure with you.

### Threat model

golearn is **local-first and fully offline** — there is no network path by
design. Nothing leaves the machine, and the tool talks to no server. The
practical attack surface is therefore:

- **Pack import** — untrusted YAML/JSON question packs parsed from disk.
  Validation is all-or-nothing per file; a single malformed question rejects
  the whole file.
- **Local SQLite database** — the on-disk store the TUI reads and writes.

There is no authentication, no remote endpoint, and no telemetry to attack.
Report anything that would let a crafted pack or a tampered database escalate
beyond reading or corrupting a user's own local data.

## Supply-chain posture

Because the project ships a binary that users run locally, the integrity of
the build and its dependencies is part of its security. The CI and dependency
posture the project holds itself to:

- **SHA-pinned GitHub Actions** — third-party actions are pinned to a full
  commit SHA, not a mutable tag, so a compromised upstream tag cannot alter a
  build.
- **Least-privilege workflows** — workflows declare minimal `permissions` and
  use concurrency groups to avoid overlapping privileged runs.
- **Standing secret scan** — a gitleaks scan runs in CI to catch accidentally
  committed credentials. The project needs no secrets by design; any that
  appear are treated as a bug.
- **`govulncheck` on every run** — the Go vulnerability database is checked on
  each CI run so known-vulnerable dependencies surface immediately.
- **Dependency review on pull requests** — new or changed dependencies are
  reviewed automatically, failing on high-severity advisories and denying
  GPL/AGPL-licensed additions.
- **Conventional-Commit PR titles** — PR titles are validated to keep history
  machine-readable and releases auditable.
- **Automated dependency updates** — Dependabot tracks both Go modules
  (`gomod`) and GitHub Actions, keeping dependencies current.
- **CGo-free SQLite** — the datastore is `modernc.org/sqlite`, a pure-Go
  driver, so builds are reproducible and cross-compile with no C toolchain in
  the trusted build path.

This describes the target posture; exact workflow filenames and settings may
evolve, but these guards are the standard the project maintains.
