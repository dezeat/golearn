# golearn

**Local-first TUI for practising multiple-choice questions — certification prep, team knowledge, and self-directed learning.**

[![CI](https://github.com/dezeat/golearn/actions/workflows/ci.yml/badge.svg)](https://github.com/dezeat/golearn/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

---

## Why golearn?

- **Zero dependencies, zero accounts** — runs entirely offline with a single SQLite file. No cloud, no subscriptions, no sign-up.
- **Adaptive practice** — automatically surfaces unseen and weak questions so you study what matters most.
- **Author-friendly packs** — write questions in YAML with full rationale, version-control them with Git, share via any channel.
- **Terminal-native** — a polished Bubble Tea TUI that fits into your existing dev workflow. No browser tabs required.

---

## Quickstart

```bash
# Build
make build

# Import a question pack
./bin/golearn import packs/go-basics.yaml

# Launch the interactive TUI
./bin/golearn tui

# Or use the text-mode session runner
./bin/golearn run go-basics --n 5

# Export a topic back to a pack file
./bin/golearn export go-basics --out out.yaml

# Reset the database (delete all data)
./bin/golearn db reset --yes
```

## Requirements

- Go 1.22+
- (Optional) `golangci-lint` for `make lint`

## Project Structure

```
cmd/golearn/              CLI entrypoint
internal/
  domain/                 Pure domain types, validation, hashing
  ports/                  Interfaces (repositories, pack source)
  app/                    Use cases (import, export, session)
  adapters/
    sqlite/               SQLite persistence + migrations
    pack/                 YAML/JSON pack reader
    localconfig/          Local user profile config
    tui/                  Bubble Tea terminal UI
packs/                    Bundled question packs
```

## Development

```bash
make build      # compile to ./bin/golearn
make test       # run all tests
make fmt        # check gofmt formatting
make vet        # go vet
make lint       # golangci-lint (if installed)
make check      # fmt + vet + lint + test (CI gate)
make db-reset   # delete default database
make clean      # remove build artifacts
```

## Configuration

| Flag       | Default                 | Description            |
|------------|-------------------------|------------------------|
| `--db`     | `~/.golearn/golearn.db` | Path to SQLite database |

## Question Pack Format

Packs are YAML or JSON files with this structure:

```yaml
pack_version: "0.1.0"
topic:
  slug: "go-basics"
  name: "Go Basics"
questions:
  - type: "single_select"
    intro: "Go's `defer` statement schedules calls for function return."
    prompt: "When does a deferred function call execute in Go?"
    choices:
      - { id: "1", text: "Immediately when the defer statement is reached" }
      - { id: "2", text: "When the surrounding function returns" }
      - { id: "3", text: "When the program exits" }
      - { id: "4", text: "At the end of the current block scope" }
    correct_choice_ids: ["2"]
    tags: ["defer", "control-flow"]
    difficulty: easy
    rationale:
      correct: "Deferred calls execute when the surrounding function returns, in LIFO order."
      per_choice:
        1: "Arguments are evaluated immediately, but the call is deferred."
        2: "Deferred calls run on function return in reverse order (LIFO)."
        3: "Defers are function-scoped, not program-scoped."
        4: "Go's defer is function-scoped, not block-scoped."
```

## Bundled Packs

| Pack                      | Questions | Description                                       |
|---------------------------|-----------|----------------------------------------------------|
| `packs/go-basics.yaml`   | 15        | Go language fundamentals with full rationale       |
| `packs/llm-agents.yaml`  | 15        | LLM agents & agentic AI for curious non-engineers  |

Additional packs (certification prep, technology deep-dives) live in the
separate [golearn-packs](https://github.com/dezeat/golearn-packs) repository.

```bash
# Import from the packs repo
git clone https://github.com/dezeat/golearn-packs.git
./bin/golearn import golearn-packs/packs/
```

## Import

```bash
# Import a single file
./bin/golearn import packs/go-basics.yaml

# Import all packs in a directory
./bin/golearn import packs/

# Use a custom DB path
./bin/golearn --db /tmp/test.db import packs/go-basics.yaml
```

Import validates every question and reports errors with file path, question index, and field:

```
packs/bad.yaml: question[2].choices: must have >= 2 choices, got 1
```

## Export

```bash
# Export a topic to YAML
./bin/golearn export go-basics --out pack.yaml

# Export to JSON
./bin/golearn export llm-agents --out pack.json --format json

# Re-import the exported file — zero duplicates
./bin/golearn import pack.yaml
```

## TUI

```bash
./bin/golearn tui
```

The TUI provides:
- **Profile select** — multi-user local profiles with per-user stats
- **Topic select** — browse topics with question counts and accuracy
- **Session config** — choose number of questions and selection mode
- **Question screen** — navigate choices with ↑/↓, toggle with Space, submit with Enter
- **Quiz-show review** — colour-coded feedback with ✔/✘ markers, press `E` for explanations
- **Summary** — accuracy %, average response time, review wrong questions with `R`
- **Stats dashboard** — global accuracy, per-pack breakdowns, difficulty distribution, weak questions, trends

## License

[Apache 2.0](LICENSE)
