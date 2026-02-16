# golearn — Project Specification

## Overview

**golearn** is a local-first TUI application for practising multiple-choice questions
(MCQs). It targets certification prep and technology learning. Questions are imported
from YAML/JSON pack files, stored in SQLite, and practised through an interactive
terminal UI.

## MVP Goals

- Import MCQ packs from YAML/JSON files (single file or directory)
- Persist questions, topics, sessions, and attempts in SQLite
- Run practice sessions in a Bubble Tea TUI with immediate feedback
- Export question packs back to canonical YAML/JSON for sharing
- Deduplicate questions via stable content hashing

## Explicit Non-Goals (MVP)

- No LLM / API integration for question generation
- No embeddings or similarity-based deduplication
- No free-text questions — MCQ only (`single_select`, `multi_select`)
- No explanations or rationales displayed in the UI (fields reserved in data model)

---

## Tech Stack

| Component   | Choice                                   |
|-------------|------------------------------------------|
| Language    | Go (1.22+)                               |
| TUI         | [Bubble Tea](https://github.com/charmbracelet/bubbletea) (planned) |
| Database    | SQLite via `modernc.org/sqlite` (CGo-free) or `mattn/go-sqlite3` |
| Pack format | YAML (`gopkg.in/yaml.v3`) + JSON (stdlib) |
| CLI         | `cobra` or stdlib `flag` (TBD)           |
| Testing     | stdlib `testing` + `testify` assertions  |
| Linting     | `golangci-lint`                          |

## Repository Structure

```
golearn/
├── cmd/
│   └── golearn/
│       └── main.go              # entrypoint
├── internal/
│   ├── domain/                  # pure domain types + logic
│   │   ├── models.go            # Topic, Question, Session, Attempt
│   │   ├── validation.go        # pack & question validation
│   │   └── hashing.go           # stable content hashing
│   ├── ports/                   # interfaces (driven + driving)
│   │   ├── repositories.go      # TopicRepo, QuestionRepo, SessionRepo, AttemptRepo
│   │   └── sources.go           # PackSource (read/write pack files)
│   ├── app/                     # use cases / application services
│   │   ├── import_pack.go
│   │   ├── export_pack.go       # (planned)
│   │   ├── session.go           # session lifecycle engine
│   │   └── selector.go          # question selection policy
│   └── adapters/                # infrastructure implementations
│       ├── sqlite/
│       │   ├── db.go            # open, migrate, pragma setup
│       │   ├── topic_repo.go
│       │   ├── question_repo.go
│       │   ├── session_repo.go
│       │   └── attempt_repo.go
│       ├── pack/
│       │   ├── yaml.go
│       │   ├── json.go
│       │   └── normalize.go     # whitespace / line-ending normalisation
│       └── tui/
│           ├── app.go
│           └── screens.go       # topic select, config, question, summary
├── examples/
│   └── go-basics.yaml           # sample question pack
├── docs/
│   ├── workflow.md
│   ├── project.md
│   └── progress.md
│   └── SPEC.md                  # original specification
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
| DB engine   | SQLite                     | —                 |
| WAL mode    | Enabled (`PRAGMA journal_mode=WAL`) | —       |

The directory `~/.golearn/` is created automatically on first run if absent.

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
| `rationale.correct`  | string             | no       | **Reserved** — not shown in MVP UI         |
| `rationale.per_choice` | map[string]string | no      | **Reserved** — keyed by choice ID          |
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
    confidence: 1.0                # optional
```

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
2. `choices` must contain ≥ 2 items
3. All `Choice.id` values must be unique within the question
4. `correct_choice_ids` must be non-empty
5. For `single_select`: exactly one correct ID
6. Every ID in `correct_choice_ids` must exist in `choices`
7. `prompt` must be non-empty after trimming

### Normalisation (before hashing and storage)

- Trim leading/trailing whitespace from all string fields
- Normalise line endings to `\n`
- Preserve choice order as authored

### Stable Hashing

The content hash is computed over the concatenation of:

```
SHA-256( topic_slug | type | normalise(intro) | normalise(prompt) |
         for each choice in order: choice.id + normalise(choice.text) |
         sort(correct_choice_ids) joined by "," )
```

Hex-encoded. Separator: `\x00` (null byte) between fields.

If a question's hash already exists in the DB, the import skips it and reports
it as a duplicate.

---

## Example Pack

```yaml
pack_version: "0.1.0"
topic:
  slug: "go-basics"
  name: "Go Basics"
questions:
  - type: "single_select"
    intro: "Consider Go's control flow mechanisms."
    prompt: "What does `defer` do in Go?"
    choices:
      - { id: "A", text: "Executes the function call immediately" }
      - { id: "B", text: "Schedules the call to run when the surrounding function returns" }
      - { id: "C", text: "Pauses the current goroutine" }
    correct_choice_ids: ["B"]
    tags: ["functions", "control-flow"]
    difficulty: 1

  - type: "multi_select"
    prompt: "Which of these are valid Go data types?"
    choices:
      - { id: "A", text: "int" }
      - { id: "B", text: "float64" }
      - { id: "C", text: "char" }
      - { id: "D", text: "string" }
    correct_choice_ids: ["A", "B", "D"]
    tags: ["types"]
    difficulty: 1
```

---

## Future Extensions

| Extension               | Description                                                    |
|--------------------------|----------------------------------------------------------------|
| **LLM adapter**          | Generate draft questions via LLM → validate → insert into DB  |
| **Embeddings similarity**| Detect near-duplicate questions; cluster topics                |
| **Rationale display**    | Show explanation on demand (`:why`) or after answering         |
| **Workspace sync/export**| Export to a folder and push to a remote workspace via CLI      |

These are out of scope for MVP but the data model reserves the necessary fields.
