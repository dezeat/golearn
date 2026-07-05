---
name: parallel
description: Set up, list, or tear down git worktrees for parallel Claude Code sessions on independent workstreams (e.g. domain hashing logic vs the Bubble Tea TUI). Each worktree binds to a GitHub issue so the work stays coordinated and observable. Use when the user wants to work on multiple branches at once or asks for a worktree.
argument-hint: "<workstream description> | list | done <branch>"
---

Manage parallel workstreams via git worktrees, each bound to a GitHub issue on
the Project board so the work stays coordinated and observable.

## Conventions

- Worktrees live in `.worktrees/<branch-slug>/` (gitignored).
- Branch naming: `feat/<area>-<slug>`, `fix/<area>-<slug>` — where `<area>`
  is one of golearn's hexagonal seams: `domain` (pure types/logic), `app`
  (use cases), `adapters` (`sqlite`/`pack`/`tui`/`localconfig`), `cmd`
  (composition root + CLI), `tui` (the Bubble Tea adapter, when a change is
  TUI-only), or `docs`.
- One Claude Code session per worktree, started in that directory.
- Bind each worktree to its GitHub issue: the active issue is the session's
  authoritative scope (CLAUDE.md Workflow §2). Note the issue number in the
  branch slug or the opening handover so status stays on the board.

## Creating a workstream

1. Make sure `main` is clean and up to date; branch worktrees off `main`.
2. `git worktree add .worktrees/<slug> -b <branch>`
3. Run `make build` inside the worktree to prime the module cache and confirm
   the tree compiles before work starts.
4. Tell the user to start the parallel session with `claude` from
   `.worktrees/<slug>/`, and suggest writing a `handover` first if that
   session needs context from this one.

## Rules for parallelizing

- Only parallelize **independent** workstreams. The hexagonal layer
  boundaries (`domain` / `app` / `adapters` / `cmd`) are the natural seams;
  two sessions must never edit the same package. Recall the layering is law
  (CLAUDE.md): an adapter never imports another adapter, so an adapter-level
  split (e.g. `sqlite` vs `tui`) is genuinely independent.
- Changes to a `ports` interface or a `domain` type land on `main` first —
  dependent workstreams rebase onto the new tip, they don't guess.
- Keep workstreams short-lived; long-running worktrees drift and merge
  painfully.

## Listing

`git worktree list` — report each worktree with its branch, its bound issue,
and whether it has uncommitted changes or unpushed commits.

## Finishing a workstream (`done <branch>`)

1. In the worktree: commit or stash everything; run `make check` and ensure
   it is green.
2. Land the branch via a PR (the `pr` skill) — rebase onto up-to-date `main`
   first, never merge `main` in. **`main` is protected; the maintainer merges
   it.** Do not fast-forward or merge to `main` yourself.
3. Once the PR has merged, `git worktree remove .worktrees/<slug>` and
   `git branch -d <branch>`, then `git worktree prune`.
4. Never remove a worktree with uncommitted changes without asking first.
