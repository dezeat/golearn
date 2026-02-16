# golearn

A local-first TUI tool for practising multiple-choice questions — built for
certification prep and learning new technologies.

## Features

- **Import** question packs from YAML or JSON files
- **Practice** in an interactive Bubble Tea TUI with immediate feedback
- **Track** your sessions, accuracy, and weak areas in SQLite
- **Export** packs back to canonical format for sharing
- **Deduplicate** questions automatically via content hashing

## Quickstart

```bash
# Build
make build

# Import the built-in example pack (10 questions)
./bin/golearn import examples/mvp-basics.yaml

# Launch the interactive TUI
./bin/golearn tui

# Or use the text-mode session runner
./bin/golearn run mvp-basics --n 5

# Export a topic back to a pack file
./bin/golearn export mvp-basics --out out.yaml
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
    tui/                  Bubble Tea terminal UI
examples/                 Sample question packs
doc/                      Spec, workflow, project docs, progress
```

## Development

```bash
make build      # compile to ./bin/golearn
make test       # run all tests
make fmt        # check gofmt formatting
make vet        # go vet
make lint       # golangci-lint (if installed)
make check      # fmt + vet + lint + test (CI gate)
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
    prompt: "What does `defer` do in Go?"
    choices:
      - { id: "A", text: "Executes immediately" }
      - { id: "B", text: "Schedules call for function return" }
      - { id: "C", text: "Pauses the goroutine" }
    correct_choice_ids: ["B"]
```

See [doc/PROJECT.md](doc/PROJECT.md) for the full schema and validation rules.

## Import

```bash
# Import a single file
./bin/golearn import examples/go-basics.yaml

# Import all packs in a directory
./bin/golearn import path/to/packs/

# Use a custom DB path
./bin/golearn --db /tmp/test.db import examples/go-basics.yaml
```

Import validates every question and reports errors with file path, question index, and field:

```
examples/bad.yaml: question[2].choices: must have >= 2 choices, got 1
```

## Export

```bash
# Export a topic to YAML
./bin/golearn export mvp-basics --out pack.yaml

# Export to JSON
./bin/golearn export mvp-basics --out pack.json --format json

# Re-import the exported file — zero duplicates
./bin/golearn import pack.yaml
```

## TUI

```bash
./bin/golearn tui
```

The TUI provides:
- **Topic select** — browse topics with question counts and accuracy
- **Session config** — choose number of questions
- **Question screen** — navigate choices with ↑/↓, toggle with space, submit with enter
- **Summary** — view total answered, correct count, and accuracy %

## Documentation

| Document                              | Purpose                              |
|---------------------------------------|--------------------------------------|
| [doc/PROJECT.md](doc/PROJECT.md)      | Technical spec, data model, pack schema |
| [doc/WORKFLOW.md](doc/WORKFLOW.md)    | Agent workflow and code standards     |
| [doc/PROGRESS.md](doc/PROGRESS.md)   | Status, changelog, milestones         |
| [doc/SPEC.md](doc/SPEC.md)           | Original design specification         |

## License

See [LICENSE](LICENSE).
