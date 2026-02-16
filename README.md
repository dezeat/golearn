# golearn

A local-first TUI tool for practising multiple-choice questions — built for
certification prep and learning new technologies.

## Features

- **Import** question packs from YAML or JSON files
- **Practice** in an interactive terminal UI with immediate feedback
- **Track** your sessions, accuracy, and weak areas in SQLite
- **Export** packs back to canonical format for sharing
- **Deduplicate** questions automatically via content hashing

## Quickstart

```bash
# Build
make build

# Import a question pack
./bin/golearn import examples/go-basics.yaml

# Launch the TUI
./bin/golearn tui
```

## Requirements

- Go 1.22+
- (Optional) `golangci-lint` for `make lint`

## Project Structure

```
cmd/golearn/          CLI entrypoint
internal/
  domain/             Pure domain types, validation, hashing
  ports/              Interfaces (repositories, pack source, selector)
  app/                Use cases (import, export, session, selection)
  adapters/
    sqlite/           SQLite persistence
    pack/             YAML/JSON pack reader/writer
    tui/              Bubble Tea terminal UI
examples/             Sample question packs
docs/                 Agent workflow, project spec, progress tracking
```

## Development

```bash
make build      # compile to ./bin/golearn
make test       # run all tests
make lint       # golangci-lint (if installed)
make vet        # go vet
make check      # vet + lint + test (CI gate)
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

See [docs/project.md](docs/project.md) for the full schema and validation rules.

## Documentation

| Document                              | Purpose                              |
|---------------------------------------|--------------------------------------|
| [docs/project.md](docs/project.md)    | Technical spec, data model, pack schema |
| [docs/workflow.md](docs/workflow.md)  | Agent workflow and code standards     |
| [docs/progress.md](docs/progress.md) | Status, changelog, milestones         |
| [doc/SPEC.md](doc/SPEC.md)           | Original design specification         |

## License

See [LICENSE](LICENSE).
