---
name: grill-me-with-docs
description: Relentless interview about a plan or design, one question at a time, challenged against docs/architecture.md, docs/DECISIONS.md and the GitHub Project board (the roadmap), updating the in-repo docs inline as decisions crystallize. Use when the user wants to stress-test a plan, says "grill me", or before starting a new epic/story.
---

<what-to-do>

Interview me relentlessly about every aspect of this plan until we reach a
shared understanding. Walk down each branch of the design tree, resolving
dependencies between decisions one-by-one. For each question, provide your
recommended answer.

Ask the questions **one at a time**, waiting for feedback on each question
before continuing.

If a question can be answered by exploring the codebase or the docs,
explore instead of asking.

</what-to-do>

<supporting-info>

## The documentation set

golearn's binding context (read it before the first question):

- `docs/architecture.md` — current state of the architecture: hexagonal
  layout (`domain`/`ports`/`app`/`adapters`/`cmd`), data model, pack format,
  validation rules, content hashing, selection policy, determinism
  guarantees, CLI/TUI surface, stats. Its non-goals and scope guardrails are
  binding.
- `docs/DECISIONS.md` — append-only decision log; the `D-NNN` format and the
  entry criteria are defined at the top of that file.
- **GitHub Issues + Project board** — the roadmap: epics / stories / tickets
  as native sub-issues, with the board column as their live status. The
  active issue is the session's authoritative scope.

## During the session

### Challenge against the architecture

When the user uses a term that conflicts with `docs/architecture.md`'s
language, call it out immediately: "architecture.md defines the `tui` as an
adapter that depends only on ports, but you seem to have it reaching into
`sqlite` directly — that's a layering violation; which is it?"

### Challenge against the layering law

The hexagonal layering is law (AGENTS.md): `domain` imports only stdlib,
`ports` are interfaces only, `app` never imports adapters, an adapter never
imports another adapter, and all wiring happens in the `cmd` composition
root. When a plan smears logic across a boundary — putting a use case in an
adapter, or a SQLite detail in the domain — say so before going deeper.

### Challenge against scope

When a plan drifts toward a non-goal in `docs/architecture.md` (a network
call, a CGo driver, a CLI framework, a database server), or past the active
issue's acceptance criteria, say so before going deeper. Scope guardrails
beat excitement.

### Sharpen fuzzy language

When the user uses vague or overloaded terms, propose a precise canonical
term consistent with the docs ("you said 'question data' — the imported pack
YAML, the stored `Question` row, or the selected-and-shuffled render model?
Determinism means those are three different things").

### Discuss concrete scenarios

Stress-test relationships with specific scenarios that probe edge cases and
force precision (e.g. "an MCQ pack re-imports with one question's text
changed — does the content hash change, does the old row survive, and does
export ordering stay byte-stable if `created_at` collides in the batch?").

### Cross-reference with code

When the user states how something works, check whether the code agrees.
Surface contradictions: "architecture.md says selection shuffles use a
seeded `*rand.Rand`, but this call reaches for the global PRNG — which is
right?"

### Update the docs inline

When a question resolves into a change of the current state, update
`docs/architecture.md` right there — don't batch it up. Keep it describing
the _current_ state only; the _why_ and the trade-off go to
`docs/DECISIONS.md`.

### Record decisions sparingly

Offer a `docs/DECISIONS.md` entry only when all the criteria at the top of
that file hold (hard to reverse, surprising without context, a real
trade-off). Use its `D-NNN` format. A changed mind never edits an old
entry — it adds a new one marked `superseded by`.

</supporting-info>
