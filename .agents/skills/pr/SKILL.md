---
name: pr
description: Create a pull request following golearn's conventions — branch naming, conventional-commit title, rebase onto the base branch, a green `make check`, the repo PR template, and no AI attribution. Use whenever opening a PR or asked to prepare one.
---

Create the PR for the current work, end to end.

## Mechanics

1. Work must be on a feature branch (`feat/<area>-<slug>`, `fix/…`,
   `chore/…`, `ci/…`, `docs/…` — `<area>` is a hexagonal seam:
   `domain`/`app`/`adapters`/`cmd`/`tui`/`docs` when it applies).
   **Target depends on the branching model (AGENTS.md Workflow §8):** by
   default the PR targets `main`; when the work is part of an epic/multi-ticket
   story using an **integration branch**, the per-chunk PR targets that
   integration branch instead, and only the final assembled PR targets `main`.
   `main` is protected and is never committed to directly; an integration
   branch is also never committed to directly — it only receives merged PRs.
2. Rebase onto the up-to-date **base branch** before opening — `main` for a
   normal PR, or the integration branch for a per-chunk PR
   (`git fetch origin && git rebase origin/<base>`), resolving conflicts
   locally. Never merge the base into the branch; a PR is only opened from a
   branch rebased on the current base. Integration branches are themselves
   kept rebased on `main`, never merged from it.
3. Run `make check` before opening (after the rebase, so it validates the
   combined state); it must be green. The CI `check` job gates the merge.
4. PR title = Conventional Commit style, like a commit subject
   (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, `perf:`,
   `build:`, `ci:`).
5. Body follows `.github/PULL_REQUEST_TEMPLATE.md` exactly (Summary, Changes,
   Verification, Decisions, Checklist). Fill every section; write "none"
   rather than deleting one.
6. No AI attribution or tool mentions anywhere in the PR (see AGENTS.md
   Commits rules). Refer to the agent-instructions file generically if the
   diff touches it.
7. Before opening, **delegate the review to the reviewer subagents** rather
   than self-reviewing in the session that wrote the code: run
   `architecture-reviewer` (layering, determinism, CGo-free, decision and docs
   drift) and `pr-reviewer` (correctness, standards, tests, PR conventions) on
   `git diff main...HEAD`. Run them in parallel — their scopes don't overlap.
   Fix every "must fix" before the PR is opened; a nit is a judgement call.
   If the subagents are unavailable (a fresh clone that has not run
   `make agents`, or a non-Claude agent), self-review against the AGENTS.md
   standards instead — the same ground, one context window worse.
8. Open with `gh pr create` (`--base <integration-branch>` for a per-chunk
   PR; the default base is `main`). **Merging differs by target:** a PR to
   `main` is never merged by the agent — only the maintainer merges to
   `main`. A per-chunk PR _into an integration branch_ is merged by the
   coordinating author/lead after review (see the integration-branch flow
   below).
9. **Merge method (convention, applied at merge time).** Default:
   **squash-merge** a small, single-purpose PR (a chore, fix, docs tweak, or
   single ticket) so `main` gets one clean commit. Exception: land an
   **epic/story integration PR** (an integration branch → `main`) with a real
   **merge commit** or **rebase-and-merge**, never squash — squashing would
   flatten away the per-ticket conventional-commit history the integration
   branch exists to preserve. GitHub cannot enforce the method per-PR, so
   whoever merges picks it by hand.

## Integration-branch flow (epics / multi-ticket stories)

Use only when the work warrants an integration branch (AGENTS.md Workflow
§8) — most PRs skip this and target `main` directly.

- **Cut once, off `main`, and push it:** `git checkout -b <feat/area-slug>
  origin/main`, then push the integration branch to the remote so per-chunk
  PRs can target it (`gh pr create --base` needs the base on origin).
  Per-chunk branches then branch off the integration branch and PR back into
  it; each such PR follows the Mechanics above with the integration branch as
  base.
- **Lead reviews and merges each per-chunk PR into the integration branch.**
  Self-review the chunk diff against the AGENTS.md standards (§7); fix must-fix
  findings; then the coordinating author/lead merges the per-chunk PR into the
  integration branch and rebases in-flight sibling branches onto the new
  integration tip. These per-chunk merges are _not_ gated on the maintainer —
  only `main` is (see the final step). Never commit to the integration branch
  directly; it only receives reviewed merged PRs.
- **Keep it fresh:** rebase the integration branch on `main` periodically and
  after each `main` move, so the eventual merge isn't a big-bang conflict.
  Never merge `main` into it.
- **e2e gate before `main`:** once every chunk has landed on the integration
  branch, verify the assembled feature end to end on that branch (full
  `make check` plus whatever live/interactive verification the feature needs —
  see the `verify` skill) before opening the final PR.
- **One PR to `main` — the maintainer merges this one:** rebase the
  integration branch on up-to-date `main`, then open a single PR for the whole
  feature. Its body summarises the assembled change and references the
  per-chunk PRs. Unlike the per-chunk merges, this final merge to `main` is
  the maintainer's — never merge it yourself.
