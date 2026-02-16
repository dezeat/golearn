# golearn — Product Specification

> **Version:** 1.0 · **Status:** MVP Complete · **Date:** February 2026

---

## 1. Executive Summary

**golearn** is a local-first, terminal-native practice tool for multiple-choice question (MCQ) learning. It enables engineers and certification candidates to import structured question packs, run adaptive practice sessions, and track mastery over time — all without cloud dependencies, accounts, or network access.

The tool is designed for deliberate practice: high-frequency, low-friction repetition with intelligent question selection that surfaces weak areas automatically.

---

## 2. Problem Statement

Certification preparation and technical knowledge retention are high-value activities with poor tooling:

- **Cloud quiz platforms** require accounts, subscriptions, and internet access.
- **Flashcard apps** lack structured MCQ support (multi-select, rationale, typed answers).
- **Static PDF/doc dumps** provide no feedback loop, no tracking, and no adaptivity.
- **Team knowledge sharing** has no standard, portable format for question banks.

Engineers need a tool that fits into their existing workflow: terminal-first, file-based, version-controllable, and zero-configuration.

---

## 3. Product Vision

**golearn** turns structured question packs into an adaptive, trackable, terminal-based learning engine.

### Value Proposition

| Pillar                | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| **Local-first**       | Runs entirely offline. Data stored in a single SQLite file. No accounts.    |
| **Developer-grade**   | Terminal UI. YAML/JSON packs. Git-friendly. CI-integrable.                  |
| **Adaptive learning** | Prioritises unseen and weak questions. Tracks accuracy over time.           |
| **Portable packs**    | Import/export canonical YAML or JSON. Share via Git, email, or workspace.   |
| **Deterministic**     | Stable content hashing. Byte-identical exports. Reproducible deduplication. |

---

## 4. Target Personas

### 4.1 Individual Certification Candidate

**Profile:** Data engineer preparing for Databricks Professional Data Engineer, AWS Solutions Architect, or similar certifications.

**Needs:**
- Practice hundreds of MCQs locally without distraction
- Track weak areas across multiple study sessions
- Use curated, high-confidence question packs

**golearn fit:** Import a certification pack, launch the TUI, practice daily. The selection engine surfaces questions you got wrong before.

### 4.2 Engineering Team Lead

**Profile:** Technical lead maintaining a team knowledge base for onboarding and internal assessments.

**Needs:**
- Create and share standardised question packs
- Version-control questions alongside documentation
- Verify team understanding of key technologies

**golearn fit:** Author packs in YAML, commit to a shared repository, team members import and practice independently.

### 4.3 Self-Directed Learner

**Profile:** Developer learning a new technology (Go, Kubernetes, Spark) through structured recall practice.

**Needs:**
- Create personal question banks from documentation
- Quickly test understanding with immediate feedback
- Export and refine packs as knowledge deepens

**golearn fit:** Write questions in YAML as you read docs, import them, practice in the TUI, export refined packs.

---

## 5. Core MVP Capabilities

### 5.1 Question Pack Import

- Parse YAML and JSON pack files (single file or directory)
- Validate every question against 7 structural rules (type, prompt, choices, correct IDs)
- Normalise text (whitespace trim, line-ending canonicalisation)
- Compute stable SHA-256 content hash for deduplication
- Upsert topics by slug; skip duplicate questions by hash
- Report import summary (inserted, duplicates, validation errors)

### 5.2 Adaptive Practice Sessions

- Select questions using a three-tier priority policy:
  1. **Unseen** questions (zero prior attempts) — shuffled
  2. **Weak** questions (highest wrong rate) — sorted by error rate descending
  3. **Random fill** from remaining — shuffled
- No duplicates within a session
- Immediate correctness feedback after each answer
- Session summary with total answered, correct count, and accuracy percentage

### 5.3 Interactive Terminal UI (Bubble Tea)

- **Topic select** — browse topics with question counts and accuracy stats
- **Session config** — adjust number of questions before starting
- **Question screen** — navigate choices with keyboard, toggle selections, submit
- **Feedback screen** — correct/incorrect indicator with correct answer display
- **Summary screen** — session results with option to return to topic select

### 5.4 Pack Export

- Export any topic to canonical YAML or JSON format
- Deterministic ordering: `created_at ASC`, content hash for tie-breaking
- Only include optional fields when they have meaningful values
- Byte-stable output: same data produces identical file content
- Format auto-detection from output file extension

### 5.5 CLI Interface

| Command                                      | Description                              |
|----------------------------------------------|------------------------------------------|
| `golearn import <path>`                      | Import pack file or directory            |
| `golearn tui`                                | Launch interactive terminal UI           |
| `golearn run <topic-slug> [--n N]`           | Text-mode practice session               |
| `golearn export <slug> --out <path>`         | Export topic to pack file                |
| `golearn help`                               | Display usage and examples               |

Global flag: `--db <path>` overrides the default database location (`~/.golearn/golearn.db`).

---

## 6. Differentiation

| Feature                | golearn            | Anki         | Cloud quiz platforms |
|------------------------|--------------------|--------------|----------------------|
| MCQ with multi-select  | ✅                 | ❌           | ✅                   |
| Local-first / offline  | ✅                 | ✅           | ❌                   |
| Terminal-native        | ✅                 | ❌           | ❌                   |
| YAML/JSON packs        | ✅                 | ❌           | ❌                   |
| Git-friendly format    | ✅                 | ❌           | ❌                   |
| Deterministic export   | ✅                 | ❌           | ❌                   |
| Content deduplication  | ✅ (SHA-256 hash)  | ❌           | ❌                   |
| Adaptive selection     | ✅                 | ✅ (SRS)     | Varies               |
| Zero configuration     | ✅                 | ❌           | ❌                   |
| Free / no account      | ✅                 | Freemium     | ❌                   |

---

## 7. Technical Foundation

### Architecture

Hexagonal (ports & adapters) architecture with clear separation:

```
cmd/golearn/          CLI entrypoint and command routing
internal/
  domain/             Pure types, validation, hashing, correctness evaluation
  ports/              Interfaces for repositories and pack sources
  app/                Use cases: import, export, session engine, selector
  adapters/
    sqlite/           Persistence (WAL mode, FK enforcement, migrations)
    pack/             YAML/JSON file reader
    tui/              Bubble Tea terminal UI
```

### Persistence

- **Engine:** SQLite via `modernc.org/sqlite` (CGo-free, zero external dependencies)
- **WAL mode:** Enabled by default for concurrent reader safety
- **Schema:** 4 tables (`topics`, `questions`, `sessions`, `attempts`) with foreign keys and indexes
- **Migrations:** Sequential, version-tracked, embedded in the binary

### Determinism Guarantees

- **Hashing:** SHA-256 over normalised content with null-byte field separators
- **Export ordering:** `created_at ASC` with content hash as tie-breaker
- **Selection:** Seeded PRNG for reproducible question ordering in tests
- **Deduplication:** Hash-based, immune to field ordering or whitespace variation

---

## 8. Example Use Cases

### Certification Preparation

```bash
# Import the Databricks PDE practice pack
golearn import examples/databricks-pde.yaml

# Practice 15 questions in the TUI
golearn tui
# → Select "Databricks Professional Data Engineer"
# → Set question count to 15
# → Practice with adaptive selection
```

### Team Knowledge Base

```bash
# Author questions in YAML alongside your docs
vim packs/kubernetes-networking.yaml

# Import into your local database
golearn import packs/

# After refinement, export the canonical version
golearn export kubernetes-networking --out packs/kubernetes-networking.yaml

# Commit to shared repo for team use
git add packs/ && git commit -m "Add K8s networking MCQs"
```

### Personal Study Loop

```bash
# Quick practice session from the terminal
golearn run go-basics --n 10

# Review accuracy over time in the TUI
golearn tui
# → Topic list shows per-topic accuracy %
```

---

## 9. Deployment Scenarios

| Scenario               | Method                                              |
|------------------------|------------------------------------------------------|
| **Local workstation**  | `go install` or `make build` → single binary         |
| **CI training check**  | Import pack + run session in headless mode (planned)  |
| **Team distribution**  | Share packs via Git repo; each member imports locally |
| **Databricks workspace** | Export packs → upload to workspace files (planned)  |

---

## 10. Roadmap

### Phase 1 — MVP (Complete)

- [x] Pack import with validation and deduplication
- [x] SQLite persistence with WAL mode
- [x] Session engine with adaptive question selection
- [x] Bubble Tea interactive TUI
- [x] Pack export with deterministic ordering
- [x] CLI with import, run, tui, export, help commands
- [x] 15+ unit and integration tests

### Phase 2 — Polish & CI (Current: Q1 2026)

- [x] `golangci-lint` configuration
- [ ] GitHub Actions CI pipeline (`make check`)
- [ ] `golearn stats` command for per-topic statistics
- [ ] Enhanced TUI styling with Lipgloss
- [ ] Exam mode (deferred feedback until session end)

### Phase 3 — Content & Community (Q2 2026)

- [ ] Pack marketplace / curated pack repository
- [ ] Pack metadata: author, license, version history
- [ ] Question tagging and filtered practice sessions
- [ ] Spaced repetition scheduling (SRS)
- [ ] Session history and trend visualisation

### Phase 4 — Enterprise & Integration (H2 2026)

- [ ] LLM-assisted question generation (draft → validate → insert)
- [ ] Embedding-based near-duplicate detection
- [ ] Multi-user team dashboards (via shared SQLite or API)
- [ ] Databricks workspace sync (export → workspace files API)
- [ ] Rationale display (`:why` command in TUI)
- [ ] Custom scoring models (weighted difficulty, time penalties)

---

## 11. Expansion Opportunities

### LLM Question Generation

Future integration with language models to generate draft questions from documentation:

1. User provides a documentation URL or text passage
2. LLM generates candidate MCQs in pack format
3. golearn validates, hashes, and stores them with `source: "llm:<provider>:<model>"`
4. User reviews and adjusts confidence scores
5. Questions enter the regular practice rotation

The data model already reserves `source`, `source_ref`, and `confidence` fields for this workflow. The `confidence` field (0.0–1.0) allows lower trust for generated questions.

### Team Knowledge Assessment

With pack sharing and per-user databases, teams can:

- Maintain canonical question banks per technology area
- Track individual and team-wide accuracy trends
- Identify knowledge gaps across the organisation
- Integrate practice sessions into onboarding workflows

---

## 12. Quality & Testing

| Guarantee                          | Implementation                                      |
|------------------------------------|------------------------------------------------------|
| All tests deterministic            | Fixed PRNG seeds, no time-dependent assertions        |
| Export byte-stability              | Verified by roundtrip tests (import → export → diff)  |
| Deduplication correctness          | SHA-256 hash with normalised content inputs           |
| Validation completeness            | 7 rules with table-driven tests for each rule         |
| Zero external runtime dependencies | CGo-free SQLite, stdlib JSON, pure Go YAML parser     |

---

## 13. Non-Goals (Explicit)

The following are intentionally excluded from the current scope:

- Free-text questions or essay-style answers
- Cloud synchronisation or user accounts
- Real-time multiplayer / competitive modes
- Mobile or web interfaces
- Question difficulty auto-calibration
- Analytics dashboards beyond CLI/TUI summary stats

These may be reconsidered in future phases based on user demand.

---

*This specification describes golearn as of MVP completion (February 2026). It serves as the authoritative product reference for feature planning, stakeholder communication, and contributor onboarding.*
