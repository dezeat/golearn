GoLearn — SPEC
0. Goal

GoLearn is a local-first TUI app to practice multiple-choice questions for certificates and new tech. It supports:

Importing question packs from files (YAML/JSON)

Running practice sessions in a TUI

Persisting questions + attempts in SQLite

Exporting question packs back to files (for sharing or syncing to places like a Databricks workspace later)

Non-goals (MVP):

No LLM/API integration

No free-text questions

No explanations/rationales shown in UI (fields reserved in data model)

1. User stories
MVP

As a user, I can import a question pack file and store questions locally.

As a user, I can launch a TUI, pick a topic, pick session length, and answer questions.

As a user, I see immediate correctness feedback and a session summary.

As a user, my attempts are recorded (correctness + time).

As a user, I can export my questions back to a canonical pack file.

Later

Generate new questions via LLM and store them the same way.

Use embeddings similarity to detect duplicates / cluster topics.

Sync exports to Databricks workspace (CLI wrapper around databricks workspace import etc.).

2. Core constraints

Language: Go

UI: TUI (Bubble Tea recommended)

Question types: only multiple-choice variations

single_select (exactly 1 correct)

multi_select (>=1 correct)

(optional) true_false as single_select with two choices

Validation: Every stored question must have a correct answer key.

Persistence: SQLite database.

Deduplication: prevent duplicate questions using a stable content hash.

Export: canonical format; stable choice IDs; future-proof for explanations.

3. Commands
CLI

learnkit tui

Launch TUI.

learnkit import <path> [--format yaml|json]

Import one file or a directory of pack files.

learnkit export <topic-slug> --out <path> [--format yaml|json]

Export canonical pack for a topic.

Optional later:

learnkit stats

learnkit review --weak --n 10

4. TUI specification
Screens

Topic Select

list topics (from DB)

show counts: total questions, accuracy (optional)

select topic to continue

Session Config

choose number of questions (default 10)

choose mode:

Practice (default): immediate correctness feedback

Exam (later): hide correctness until end

start session

Question Screen

show optional intro/description block

show prompt

show choices

controls:

Up/Down or j/k to navigate

Space to toggle selection (multi-select)

Enter to submit

s skip

q quit session

submission:

lock answer

show “Correct/Incorrect”

no explanation displayed in MVP

press Enter to proceed

Summary Screen

total questions answered

correct count, accuracy %

list of missed question counts (optional)

option: “Back to topics”

Session logic

Each session is stored with an ID.

Each asked question creates an attempt record:

selected choices

correct boolean

latency

skipped flag

5. Data model
Domain entities

Topic

id (uuid/int)

slug (string, unique, stable)

name (string)

Question

id (uuid/int)

topic_id

type (single_select | multi_select)

intro (optional string)

prompt (string)

choices (ordered list)

correct_choice_ids (list of choice IDs)

tags (optional)

difficulty (optional int)

rationale (reserved for later)

correct (optional string)

per_choice (optional map choice_id -> string)

source (string, e.g. manual:file, later llm:provider:model)

source_ref (optional string: file path, url, etc.)

confidence (float 0..1, default 1.0 for manual)

hash (string, unique) = stable hash of normalized content

created_at

Session

id

topic_id

mode (practice | exam)

requested_n

started_at

ended_at

Attempt

id

session_id

question_id

selected_choice_ids (list)

correct (bool)

skipped (bool)

latency_ms (int)

created_at

6. SQLite schema (logical)

Tables:

topics

id PK

slug UNIQUE

name

questions

id PK

topic_id FK

type

intro

prompt

choices_json

correct_choice_ids_json

tags_json

difficulty

rationale_correct

rationale_per_choice_json

source

source_ref

confidence

hash UNIQUE

created_at

sessions

id PK

topic_id FK

mode

requested_n

started_at

ended_at

attempts

id PK

session_id FK

question_id FK

selected_choice_ids_json

correct

skipped

latency_ms

created_at

Indexes:

questions(topic_id)

attempts(question_id)

attempts(session_id)

questions(hash) unique

Migrations:

Use a minimal migration mechanism (e.g., golang-migrate or embedded SQL files executed in order).

7. Question pack file format (canonical)

Support YAML and JSON with the same fields.

Pack structure

pack_version: string (e.g. "0.1.0")

topic:

slug: string

name: string

metadata (optional):

author

created_at

source

questions: list of MCQs

Question structure

id (optional stable string; if absent, system generates)

type: single_select | multi_select

intro (optional)

prompt

choices: ordered list of { id, text }

id must be stable (e.g., "A", "B", "C" or "1", "2")

correct_choice_ids: list of choice IDs

tags (optional list)

difficulty (optional int)

rationale (optional, reserved)

correct (optional string)

per_choice (optional map of choice_id -> string)

source (optional)

confidence (optional float 0..1)

Example YAML
pack_version: "0.1.0"
topic:
  slug: "go-basics"
  name: "Go Basics"
questions:
  - type: "single_select"
    intro: "Consider the following Go snippet."
    prompt: "What does `defer` do?"
    choices:
      - { id: "A", text: "Executes immediately" }
      - { id: "B", text: "Schedules execution when the surrounding function returns" }
      - { id: "C", text: "Pauses the goroutine" }
    correct_choice_ids: ["B"]
    tags: ["functions"]
    difficulty: 1

8. Import rules

Import behavior:

Accept a file or directory.

Parse YAML/JSON packs.

Validate each question:

type is supported

=2 choices

choice IDs are unique

correct_choice_ids non-empty

for single_select: exactly one correct id

correct ids must exist in choices

Normalize:

trim whitespace

canonicalize line endings

preserve choice order

Compute hash:

hash of (topic_slug + type + intro + prompt + choices ids+text + correct_choice_ids sorted)

Dedupe:

if hash exists, skip insert and report as duplicate

Upsert topic by slug.

Errors:

On invalid pack, show file path + question index + field that failed.

9. Export rules

Export behavior:

Export by topic_slug.

Output canonical pack format with stable choice IDs.

Include pack_version, topic info, and questions in deterministic order:

default order: created_at asc or prompt asc (choose one and document)

Include source/confidence fields from DB when present.

Rationale fields included if stored, but empty/omitted in MVP.

10. Selection policy (which questions to ask)

MVP selection:

Inputs: topic_id, n

Rank questions by priority score:

unseen questions first

then previously incorrect more often

then random fill

Ensure no duplicates within a session.

Implementation detail:

compute per-question stats from attempts:

attempts_count

wrong_count

last_attempt_at

choose:

unseen bucket (attempts_count == 0) random sample

weak bucket (wrong rate high) random weighted

fill remainder random

11. Architecture
Project layout
cmd/learnkit/main.go

internal/
  domain/
    models.go
    validation.go
    hashing.go

  ports/
    repositories.go
    sources.go
    selector.go

  app/
    import_pack.go
    export_pack.go
    start_session.go
    record_attempt.go
    select_questions.go

  adapters/
    sqlite/
      db.go
      migrations/
      topic_repo.go
      question_repo.go
      session_repo.go
      attempt_repo.go
    pack/
      yaml.go
      json.go
      normalize.go
    tui/
      app.go
      screens_*.go

Ports (interfaces)

Repositories

TopicRepository

UpsertBySlug(slug, name) -> topic

List() -> []topic

QuestionRepository

InsertMany([]Question) (inserted, skippedDuplicates)

ListByTopic(topicID) -> []Question

GetByIDs([]id) -> []Question

SessionRepository

Create(Session) -> id

Finish(id)

AttemptRepository

Record(Attempt)

StatsByTopic(topicID) -> per-question stats

PackSource (file)

ReadPack(path) -> Pack

WritePack(path, pack)

Selector

Select(topicID, n) -> []QuestionID

Future reserved ports (not implemented):

QuestionGenerator (LLM)

SimilarityIndex (embeddings)

Adapters

SQLite adapter implements repos.

Pack adapter handles YAML/JSON.

TUI adapter calls app layer use-cases.

12. Quality gates

go test ./... must pass

deterministic export

import validation errors are actionable

TUI usable with keyboard only

13. Future work (explicit)

LLM generation adapter:

Generate drafts -> validate -> insert -> sessions draw from DB

Explanations/rationales:

show on demand (:why) or after question

Embeddings similarity:

duplicate detection beyond hash

topic clustering

Databricks sync:

learnkit export to a local folder + CLI command to push to Databricks workspace