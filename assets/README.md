# assets

Marketing and documentation assets for golearn. Everything here is either a
committed rendered artifact or the source that regenerates it — no fabricated
binaries.

## `hero.gif` — animated demo

Rendered from [`demo.tape`](demo.tape) with
[charmbracelet/vhs](https://github.com/charmbracelet/vhs). The tape itself
seeds a throwaway demo database at `/tmp/golearn-demo.db` from a bundled pack
(in a hidden setup block), so the recording never touches your real
`~/.golearn` data. It walks the ASCII start screen → a practice question with
quiz-show review + explanation → the session summary → the per-pack stats view.

Prerequisites on `PATH`: `vhs`, plus its runtime deps `ttyd` and `ffmpeg`.

Re-render **from the repo root** (so `./bin/golearn` in the tape resolves):

```bash
make build
vhs assets/demo.tape
```

## `architecture.svg` — hexagonal component diagram

A C4-style component diagram of the four hexagonal layers
(domain / ports / app / adapters) plus the composition root. Rendered offline
with Graphviz from [`architecture.dot`](architecture.dot); the same diagram is
also embedded as a Mermaid block in `docs/architecture.md` (which GitHub renders
natively). Every edge is a real import confirmed with `go list`.

Re-render:

```bash
dot -Tsvg assets/architecture.dot -o assets/architecture.svg
```
