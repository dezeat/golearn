# golearn — Project Specification

## Overview

**golearn** is a local-first TUI application for practising multiple-choice questions
(MCQs). It targets certification prep and technology learning. Questions are imported
from YAML/JSON pack files, stored in SQLite, and practised through an interactive
terminal UI.

## MVP Status: Complete

All core capabilities are implemented and tested:
- Import, export, session engine, selection policy, TUI, CLI

---

## Tech Stack

| Component   | Choice                                   |
|-------------|------------------------------------------|
| Language    | Go 1.22+                                |
| TUI         | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| Database    | SQLite via `modernc.org/sqlite` (CGo-free) |
| Pack format | YAML (`gopkg.in/yaml.v3`) + JSON (stdlib) |
| CLI         | stdlib (`os.Args` manual parsing)        |
| Testing     | stdlib `testing` (no external test deps)  |
| Linting     | `golangci-lint`                          |

### CLI Framework Decision

The CLI uses manual `os.Args` parsing instead of `cobra` or `flag`. This is intentional:
- Only 5 commands (`import`, `run`, `tui`, `export`, `help`) — minimal complexity
- No nested subcommands or complex flag interactions
- Avoids adding a dependency tree for simple routing
- If the command surface grows significantly, migration to `cobra` is straightforward

---

## Repository Structure

```
golearn/
├── cmd/
│   └── golearn/
│       └── main.go                # CLI entrypoint and command routing
├── internal/
│   ├── domain/                    # pure domain types + logic
│   │   ├── models.go              # Topic, Question, Session, Attempt, Pack types
│   │   ├── validation.go          # pack & question validation (7 rules)
│   │   ├── hashing.go             # stable SHA-256 content hashing + normalisation
│   │   └── correctness.go         # order-insensitive answer evaluation
│   ├── ports/                     # interfaces (driven + driving)
│   │   ├── repositories.go        # TopicRepo, QuestionRepo, SessionRepo, AttemptRepo, StatsRepo
│   │   └── sources.go             # PackReader interface
│   ├── app/                       # use cases / application services
│   │   ├── import_pack.go         # parse → validate → normalise → hash → persist
│   │   ├── export_pack.go         # load → sort → serialise to YAML/JSON
│   │   ├── session.go             # session lifecycle engine
│   │   └── selector.go            # question selection policy (unseen → weak → fill)
│   └── adapters/                  # infrastructure implementations
│       ├── sqlite/
│       │   ├── db.go              # Open, WAL + FK pragmas, sequential migrations
│       │   ├── topic_repo.go      # UpsertBySlug, GetBySlug, List
│       │   ├── question_repo.go   # InsertMany (batch + dedupe), ListByTopic
│       │   ├── session_repo.go    # Create, Finish
│       │   ├── attempt_repo.go    # Record, StatsByTopic
│       │   ├── stats_repo.go      # StatsRepository: aggregated user-scoped stats
│       │   ├── sqlite_test.go     # integration tests
│       │   └── stats_test.go      # stats aggregation tests
│       ├── pack/
│       │   └── reader.go          # YAML + JSON parser, directory support
│       └── tui/
│           ├── app.go             # Run(db), topic metadata loading
│           ├── model.go           # Bubble Tea model, screen routing
│           ├── screens_topic.go   # topic selection view
│           ├── screens_session.go # session configuration view
│           ├── screens_question.go # question + feedback views
│           ├── screens_summary.go # session summary view
│           └── screens_stats.go   # stats screens (global, pack list, pack detail) + home menu
├── examples/
│   ├── go-basics.yaml             # 3-question sample pack
│   ├── mvp-basics.yaml            # 10-question mixed pack
│   └── databricks-pde.yaml        # 30-question PDE certification pack
├── doc/
│   ├── WORKFLOW.md                # agent workflow and code standards
│   ├── PROJECT.md                 # this file — technical specification
│   ├── PROGRESS.md                # status, changelog, debt log
│   └── SPEC.md                    # product specification
├── .github/
│   └── workflows/
│       └── ci.yml                 # GitHub Actions: fmt + vet + lint + test + build
├── .golangci.yml                  # golangci-lint configuration
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Persistence

| Setting     | Default                    | Override          |
|-------------|----------------------------|-------------------|
| DB path     | `~/.golearn/golearn.db`    | `--db <path>`     |
| Config path | `~/.golearn/config.json`   | test-only injection in code |
| DB engine   | SQLite (CGo-free)          | —                 |
| WAL mode    | Enabled (`PRAGMA journal_mode=WAL`) | —       |
| Foreign keys| Enabled (`PRAGMA foreign_keys=ON`)  | —       |

The directory `~/.golearn/` is created automatically on first run if absent.

### WAL Mode

Write-Ahead Logging is enabled on every database open to prevent "database is locked"
errors when the application reads and writes concurrently. This is the recommended
mode for single-writer, multiple-reader workloads.

### Migrations

Schema migrations are sequential and version-tracked in a `schema_migrations` table.
Each migration is an embedded SQL string applied exactly once. This avoids external
migration tool dependencies while supporting future schema evolution.

### Local Profiles

`golearn` uses local profiles (not network auth):

- `users` table stores profile metadata (`handle`, optional `display_name`)
- Current profile is persisted in `~/.golearn/config.json` as `current_user_id`
- Topics and questions are shared globally across users
- Sessions, attempts, selection stats, topic accuracy, and review scope are per-user

Development-phase compatibility rule:

- If an older DB schema is detected (missing user fields), startup resets schema by
  dropping and recreating tables, then re-seeds default `local` profile.

---

## Canonical MCQ Model

### Question Types

| Type            | Correct Answers | Validation Rule                       |
|-----------------|-----------------|---------------------------------------|
| `single_select` | Exactly 1       | `len(correct_choice_ids) == 1`        |
| `multi_select`  | 1 or more       | `len(correct_choice_ids) >= 1`        |

### Question Fields

| Field                | Type               | Required | Notes                                      |
|----------------------|--------------------|----------|--------------------------------------------|
| `type`               | string             | yes      | `single_select` or `multi_select`          |
| `intro`              | string             | no       | Optional context block shown before prompt |
| `prompt`             | string             | yes      | The question text                          |
| `choices`            | `[]Choice`         | yes      | Ordered; ≥ 2 items                         |
| `correct_choice_ids` | `[]string`         | yes      | References `Choice.id`; validated          |
| `tags`               | `[]string`         | no       | Freeform topic tags                        |
| `difficulty`         | int                | no       | 1–5 scale (convention)                     |
| `rationale.correct`  | string             | no       | Shown in TUI review mode via 'e' toggle   |
| `rationale.per_choice` | map[string]string | no      | Per-choice explanations, keyed by choice ID |
| `source`             | string             | no       | Provenance, e.g. `manual:file`             |
| `source_ref`         | string             | no       | File path, URL, etc.                       |
| `confidence`         | float64            | no       | `0.0–1.0`; defaults to `1.0` for manual   |

### Choice Fields

| Field  | Type   | Required | Notes                            |
|--------|--------|----------|----------------------------------|
| `id`   | string | yes      | Stable, question-local (e.g. `"A"`, `"B"`) |
| `text`  | string | yes      | The answer text                  |

---

## Canonical Pack Format

```yaml
pack_version: "0.1.0"
topic:
  slug: "<kebab-case-identifier>"
  name: "<Human Readable Name>"
questions:
  - type: "single_select"         # or "multi_select"
    intro: "Optional context."     # optional
    prompt: "The question text?"
    choices:
      - { id: "A", text: "First option" }
      - { id: "B", text: "Second option" }
      - { id: "C", text: "Third option" }
    correct_choice_ids: ["B"]
    tags: ["optional-tag"]         # optional
    difficulty: 2                  # optional
    source: "manual:file"          # optional
    source_ref: "https://..."      # optional
    confidence: 1.0                # optional
```

### Pack Versioning Strategy

The `pack_version` field uses semantic versioning (`major.minor.patch`):

- **`0.1.0`** — current MVP format
- **Minor bumps** (e.g., `0.2.0`) — additive field additions (backward compatible)
- **Major bumps** (e.g., `1.0.0`) — breaking schema changes

The import pipeline currently accepts any `pack_version` value as long as it's non-empty.
Future versions may add version-aware parsing if the schema evolves.

### Schema Rules

- `pack_version`: **required**, semver string
- `topic.slug`: **required**, unique identifier, kebab-case
- `topic.name`: **required**, display name
- `questions`: **required**, non-empty list
- Each question must pass all validation rules (see below)

---

## Import Validation & Normalisation

### Validation Rules

1. `type` must be `single_select` or `multi_select`
2. `prompt` must be non-empty after trimming
3. `choices` must contain ≥ 2 items
4. All `Choice.id` values must be unique within the question
5. `correct_choice_ids` must be non-empty
6. For `single_select`: exactly one correct ID
7. Every ID in `correct_choice_ids` must exist in `choices`

### Error Handling

Import validates all questions in a pack file before inserting any. If validation
fails, the entire file is rejected with actionable error messages including file path,
question index, and field name. This prevents partial imports that could leave the
database in an inconsistent state. When importing a directory, each file is processed
independently — a failure in one file does not prevent others from being imported.

### Normalisation (before hashing and storage)

- Trim leading/trailing whitespace from all string fields
- Normalise line endings: `\r\n` → `\n`, standalone `\r` → `\n`
- Preserve choice order as authored

### Stable Hashing

The content hash is computed over the concatenation of:

```
SHA-256( topic_slug \x00 type \x00 normalise(intro) \x00 normalise(prompt) \x00
         for each choice in order: choice.id + normalise(choice.text) \x00
         sort(correct_choice_ids) joined by "," )
```

Hex-encoded (64 characters). Separator: `\x00` (null byte) between fields.

If a question's hash already exists in the DB, the import skips it and reports
it as a duplicate.

---

## Selection Policy

Questions are selected using a three-tier priority system:

1. **Unseen bucket** — questions with zero prior attempts, shuffled randomly
2. **Weak bucket** — questions with at least one wrong attempt, sorted by wrong rate descending
3. **Fill bucket** — remaining questions (all correct history), shuffled randomly

Buckets are concatenated in order and capped at `n` (requested session length).
A seeded `*rand.Rand` is used for all shuffling to ensure deterministic test behavior.

Per-question stats (`attempts_count`, `wrong_count`) are computed from the `attempts`
table grouped by `question_id` and filtered by `user_id`.

---

## Export Guarantees

- **Deterministic ordering:** questions sorted by `created_at ASC`, then by content hash for tie-breaking
- **Byte-stability:** exporting the same data twice produces identical output
- **Optional field omission:** fields are only included when they have meaningful non-default values
- **Roundtrip safety:** import → export → re-import produces zero new insertions (all deduplicated)

---

## Determinism Guarantees

| Property               | Mechanism                                                |
|------------------------|----------------------------------------------------------|
| Content hashing        | SHA-256 over normalised, null-byte-separated fields      |
| Export ordering         | `created_at ASC` + hash tie-breaking                    |
| Question selection      | Seeded PRNG (`*rand.Rand`) for shuffle reproducibility  |
| Deduplication           | Hash UNIQUE constraint in SQLite                        |
| Test reproducibility    | Fixed seeds, no time-dependent assertions               |

---

## Session Engine

The session engine manages the lifecycle of a practice session:

1. **StartSession** — resolve topic, load questions, compute user-scoped stats, select questions, persist session row with `user_id`
2. **GetNextQuestion** — serve from in-memory queue (no duplicates)
3. **RecordAttempt** — evaluate correctness via domain layer, persist attempt with `user_id`
4. **EndSession** — set `ended_at` timestamp

The engine holds in-memory state for one active session per instance. This is an
intentional MVP constraint — the engine is lightweight and instantiated per session.

---

## Future Extensions

| Extension               | Description                                                    |
|--------------------------|----------------------------------------------------------------|
| **LLM adapter**          | Generate draft questions via LLM → validate → insert into DB  |
| **Embeddings similarity**| Detect near-duplicate questions; cluster topics                |
| **Spaced repetition**    | SRS scheduling based on attempt history                        |
| **Exam mode**            | Deferred feedback until session end                            |

---

## TUI Navigation

### Home Menu

After login/register, users land on the Home Menu:

```
1) Start Practice     → Topic Select → Session Config → Quiz → Summary
2) Review Wrong       → Browse incorrect from last session (disabled if none)
3) Stats              → Global Stats → Pack List → Pack Detail
4) Switch Profile     → Profile Menu (login/register)
5) Quit
```

### Summary Screen

After completing a session, the summary shows:

- Review incorrect questions (if any)
- View stats for this pack → Pack Detail Stats
- Back to Home

---

## Stats Feature

### Metrics

All stats are **user-scoped** — switching profile changes all stats displays.

| Metric               | Scope     | Definition                                                |
|----------------------|-----------|-----------------------------------------------------------|
| Accuracy %           | Global/Topic | `correct / answered * 100` (skipped excluded)          |
| Avg response time    | Global/Topic | `sum(latency_ms) / answered` for non-skipped attempts  |
| Total time           | Global    | `sum(latency_ms)` across all non-skipped attempts         |
| Coverage %           | Topic     | `distinct_questions_attempted / total_questions * 100`    |
| Most practiced       | Global    | Topic with most non-skipped attempts                      |
| Weakest              | Global    | Topic with lowest accuracy (minimum 5 attempts)           |

### Difficulty Bucketing

Questions have an optional `difficulty` field (integer 1–5):

| Difficulty Value | Bucket   |
|-----------------|----------|
| 1–2             | Easy     |
| 3               | Medium   |
| 4–5             | Hard     |
| 0 / NULL        | Unrated  |

### Tag Stats

Tags are per-question freeform labels. Tag stats aggregate across all questions
sharing a tag within a topic. Minimum attempts threshold (default: 5) filters
out tags with insufficient data.

### Weak Questions

Questions ranked by wrong rate (descending). Only questions with at least
`minAttempts` (default: 3) non-skipped attempts and at least one wrong answer
are included.

### Session Trend

A sparkline showing accuracy per completed session, ordered chronologically.
Uses Unicode block characters: `▁▂▃▄▅▆▇█` mapped to 0–100%.

These are out of scope for MVP but the data model reserves the necessary fields.
