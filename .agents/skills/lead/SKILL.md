---
name: lead
description: Drive a story or small epic from board to merged — pull ready tasks, execute them in sequence or in parallel worktrees, gate every chunk through the reviewer subagents, and keep the GitHub board telling the truth. Use to execute an agreed plan, after `wayfinder` and `grill-me-with-docs` have settled the route.
---

Execute an agreed plan end to end. You are the **lead session**: you drive a
story (or a small epic) from Todo to merged, and you keep the board honest
while doing it.

This skill assumes the route is already settled. If open decisions still block
the breakdown, stop and run `wayfinder` (chart the epic) or
`grill-me-with-docs` (stress-test the design) first — executing through a
foggy route produces work that gets thrown away.

## Before the loop

1. **Reconcile from GitHub.** The board is canonical; your context window is
   not. Read the story issue and its task sub-issues, and confirm what is
   actually Todo, In Progress and Done before assuming anything.
2. **Pick the branching depth** (AGENTS.md Workflow §8). A single
   independently-shippable task takes one short-lived branch straight to a PR
   against `main`. Only a multi-task story or epic that must land as one
   coherent unit earns an **integration branch** cut off `main`, with each task
   PR'ing into it.
3. **Confirm the scope is the issue's.** The acceptance criteria on the card
   are the contract. Exceeding them silently is the failure this model exists
   to prevent — if the work needs more, say so and let the maintainer decide.

## The loop

For each ready, unblocked task:

1. **Pull it.** Move the card to In Progress before starting, not after.
2. **Execute.** Sequential in this session is the default. Reach for the
   `parallel` skill only when two tasks touch genuinely disjoint file-sets —
   two workstreams editing one file overwrite each other, and the coordination
   cost of a worktree is only worth paying for real independence (a domain
   hashing change alongside a TUI change is the shape that qualifies).
3. **Gate it.** `make check` green, then delegate review to the subagents —
   `architecture-reviewer` and `pr-reviewer` on `git diff <base>...HEAD`. Run
   them in parallel; their scopes don't overlap. Fix every "must fix" before
   the work counts as done. A discovered blocker moves the card to Blocked with
   what it waits on, rather than being worked around quietly.
4. **Land it** via the `pr` skill, targeting `main` or the integration branch
   per the depth chosen above. A per-chunk PR into an integration branch is
   yours to merge after review. **A PR to `main` is never yours** — the
   maintainer merges those.
5. **Reconcile the board** after every state change, then repeat. If the board
   and your working memory disagree, the board wins and your memory is stale.

## Definition of done

A card closes only when: acceptance criteria met · `make check` green · both
reviewers passed · sub-issues closed · column moved to Done. When the whole
story is assembled and green on an integration branch, verify it end to end on
that branch, then open the single integration→`main` PR for the maintainer.

## Closing the session

Post a `handover` before the session ends or context runs long — to the GitHub
Handovers Discussion, never a local file. The next session may be on another
machine, and unrecorded state is the dominant multi-agent failure mode.

## Rules that bind the lead specifically

- **Never commit to `main` or to an integration branch directly.** Both receive
  merged PRs only.
- **Never let the board lag the work.** A stale column is worse than no board,
  because it is believed.
- **Report deviations rather than absorbing them.** Anything done that wasn't
  asked, and anything asked that wasn't done, belongs in the completion report.
