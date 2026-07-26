# golearn — OPERATING MODEL

> The **recommended operating convention** for how work on golearn is shaped,
> routed, reviewed, and integrated — a repo-scoped, self-contained synthesis
> of the maintainer's portfolio operating model (v1, July 2026).

**Status: convention, not contract.** This is the default workflow the core
maintainer and their agent sessions run; a maintainer may deviate from it
deliberately. The *binding* rules stay where they always were — `AGENTS.md`
(identity, standards, gates, merge authority), `docs/architecture.md`
(project facts), `docs/DECISIONS.md` (accepted decisions) — and external
contributions are governed by `CONTRIBUTING.md` and the PR template, not this
file. What this file adds is shared workflow context: state, authority,
handoffs, and the board/label surface, so any session starts with the same
picture. GitHub is the coordination source of truth — the Project boards hold
state, issues hold work context, pull requests hold code review, and
Discussions hold non-binding handovers and ideas.

## 1. Principles

1. **Autonomy is the normal execution mode.** Once an item reaches Todo with
   authorization, an agent carries it through implementation and review
   without waiting for routine human approval.
2. **The Driver leads discovery and owns exceptional decisions.** Upstream
   direction is dialogue-based; a Driver decision is requested only when work
   cannot safely continue within its authority.
3. **The board is status.** The Status field is canonical; neither issue-body
   checklists nor local task lists replace it. (§2's state names and §6's
   label names are incorporated by `AGENTS.md` and are not freely deviable —
   the convention framing covers the workflow, not those vocabularies.)
4. **Evidence travels with the handoff.** Every transition records the minimum
   result the next role needs to continue without reconstructing context.
5. **Policy before machinery.** Manual role assignment is valid operation;
   future automation may enact this model but not silently change its
   semantics.

## 2. The state model

Main flow:

```text
Inbox → Exploring → Framing → Planning → Todo
    → In Progress → Agent Review → Integration → Done
```

**Blocked** and **Decision Needed** are exception states available from any
active stage — never terminal lanes; each records its return state.

| Status | Meaning | Typical owner |
| --- | --- | --- |
| Inbox | Unshaped idea or thread worth retaining. | Driver |
| Exploring | Evidence gathering, option discovery; no solution assumed. | research/planning agent |
| Framing | Dialogue reaches shared direction, boundaries, trade-offs. | Driver |
| Planning | The agreed concept is cut into feasible, routed work. | Driver + planning agent |
| Todo | Bounded, authorized item ready for autonomous execution — the sole executable queue. | implementation agent |
| In Progress | An agent actively performs the approved work. | implementation agent |
| Agent Review | Independent review of the result and its evidence. | reviewer subagents (D-010) |
| Integration | Reviewed change is validated and lands via PR merge. | maintainer (see §4) |
| Done | Integrated, evidence complete. | — |
| Blocked | Next action known and authorized, but a concrete prerequisite is missing. | role that found it |
| Decision Needed | A human must authorize the direction. | Driver |

An item enters **Todo** only when it states: objective and outcome, scope and
non-goals, acceptance criteria, dependencies/constraints, routing labels, and
any required start authorization (`Start Authorization: Authorised`). "More
research" is not Blocked — if the agent can still research or test within its
mandate, that is normal work.

**Blocked** handoffs record: cause, owner, wake-up event, return status.
**Decision Needed** handoffs record: the decision requested, options, a
recommendation, consequences, return status. Uncertainty alone is not an
escalation — research first, escalate genuine authority questions.

## 3. Authority envelope

Each work item grants an envelope: its objective, scope, non-goals,
acceptance criteria, this repo's charter and gates, and declared risk limits.
Within it, an agent investigates, implements, runs gates, and seeks review
freely. Move the item to **Decision Needed** before any action that exceeds
it: changing objective or agreed scope, choosing an unresolved trade-off with
materially different outcomes, crossing a security/privacy/secret boundary,
irreversible or externally visible effects, material cost or risk, a policy
exception, or proceeding on contradictory evidence.

**golearn's charter rule stands and is binding** (unlike the rest of this
convention): **a PR to `main` is the maintainer's to merge — never merge to
`main` yourself** (`AGENTS.md`, Workflow), and the review/gate guards below
hold regardless of who merges.

## 4. Roles

| Role | Responsibility |
| --- | --- |
| Driver (maintainer) | Sets intent and priority, answers Decision Needed, owns design decisions and every merge to `main`. |
| Planning agent | Captures intent, researches, structures options, turns the agreed concept into routed work; never silently sets strategy. |
| Implementation agent | Delivers one authorized item, keeps evidence, opens the PR. |
| Review agents | `architecture-reviewer` + `pr-reviewer` (D-010) evaluate the diff independently; their verdicts land before the PR opens or on it. |
| Integration | The maintainer merges (§3); the session preparing the PR verifies the guards are documented: independent review, green gates, no open Decision Needed, charter-compliant target and merge method. |

One session may hold any role; the assignment must be visible on the issue or
PR, never inferred from chat context.

## 5. The boards

Two Projects, both public, both carrying the state model above:

- **golearn — v1.0.0 release** — everything gating the release (the four
  1.0.0 workstreams: generator, hardening, UX/themes, release presence).
- **golearn — maintenance & meta** — standing upkeep, tooling/governance, and
  parked post-1.0 epics (`Horizon: Later`).

Context fields on both (routing and portfolio context only — never a mirror
of issue content): **Item Type** (Goal / Epic / Delivery Issue / Decision /
Experiment), **Horizon / Priority** (Now / Next / Later / Learning Space —
`Now` is deliberately small), **Start Authorization** (Not applicable / Needs
Driver approval / Authorised), **Next Milestone** (one observable next
outcome). Hierarchy stays in native sub-issues; built-in fields (Milestone,
Parent issue, Assignees, Linked PRs) are used rather than recreated.

## 6. Labels

Labels describe the work's technical shape and the review expertise it calls
for — never hierarchy, workflow state, priority, or an agent. Normally one
`type:` and zero or more `area:` labels; carry the relevant ones onto the PR.

| `type:` | Work that is primarily |
| --- | --- |
| `type:feature` | a new or materially expanded capability |
| `type:bug` | correction of incorrect behaviour |
| `type:maintenance` | reliability, dependency, refactor, upkeep |
| `type:documentation` | documentation or instructional content |
| `type:research` | time-boxed discovery or experiment |

| `area:` | Primary change concerns |
| --- | --- |
| `area:frontend` | TUI presentation and interaction |
| `area:backend` | application logic, services, integrations |
| `area:data` | data model, storage, migrations, transformations |
| `area:infrastructure` | CI, build, release, runtime |
| `area:security` | trust boundaries, secrets, threat-sensitive behaviour |
| `area:api` | contracts and schemas (pack format, ports) |
| `area:ux` | user experience, accessibility, content flow |

A label present is a default review perspective for that PR (e.g.
`area:security` → dedicated security look; every code PR gets a baseline
security consideration regardless). Use the exact names above; new labels
need a demonstrated recurring need first. Legacy `epic`/`story`/`task` and
`wayfinder:*` labels remain on historical items only — new work never mints
them.

## 7. Wayfinder on this surface

The `wayfinder` skill (planning maps of decision tickets) expresses its
artifacts through the state model instead of labels:

| Wayfinder artifact | Item Type | Status while open | Routing label |
| --- | --- | --- | --- |
| Map | Goal | Planning | — |
| Research / prototype ticket | Experiment | Exploring | `type:research` |
| Grilling (decision) ticket | Decision | Framing | — |
| Task ticket | Delivery Issue | Todo | as fits |

Research and prototype tickets share board fields, so the ticket body carries
a `Kind:` line naming its exact kind — that line decides the AFK-vs-HITL
handling in the skill. Standalone spikes outside a map are Experiments too:
Todo when authorized, `type:research`. Closing a ticket moves it to Done
(board automation). Maps and their tickets live on the board that owns their
epic.

## 8. Manual-first, non-goals

The Driver or an authorized session picks the item and assigns the role
explicitly; the agent writes the same GitHub artifacts a future automation
would. This file defines no dispatcher, polling, leases, or runtime — those
may only ever implement these semantics, not change them.
