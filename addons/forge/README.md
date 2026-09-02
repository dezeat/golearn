# Forge — assisted question authoring

Forge is golearn's opt-in authoring companion: a separate `golearn-forge`
binary that drafts multiple-choice questions with a model of **your** choice —
local or hosted — runs every candidate through a bounded trust chain
(generate → independent verify → critique → one repair), and hands the result
to the offline golearn you already use. The practice engine itself stays
exactly what it is: fully offline, no network path, no new dependencies.

**Bring your own model.** Four provider adapters ship today, all built on the
Go standard library alone — no provider SDKs, no third-party HTTP clients:

| Provider | Ships | Verified |
| --- | --- | --- |
| **Ollama** (local) | ✅ adapter + embeddings | ✅ live, end to end — full pipeline plus draft → library → pack export |
| **OpenAI** | ✅ adapter + embeddings | ✅ live, end to end — full pipeline plus draft → library → pack export |
| **OpenRouter** | ✅ adapter + embeddings | ✅ live at the wire level; full-pipeline run pending |
| **Anthropic** | ✅ adapter | fixture-tested; live verification pending |

A credential is read from the environment (`OPENAI_API_KEY`,
`OPENROUTER_API_KEY`, `ANTHROPIC_API_KEY`), never stored, and never written to
disk, logs, or packs.

## Measured, not asserted

Every number below comes from a recorded, pre-registered run against the real
provider — same prompt, same schema, Wilson 95% intervals instead of bare
percentages. The full lab notebook lives in
[`docs/design/FORGE-EXPERIMENTS.md`](../../docs/design/FORGE-EXPERIMENTS.md);
the live campaign is tracked in
[#141](https://github.com/dezeat/golearn/issues/141).

| Model · provider | Structured output | Question craft (judge-scored) | Latency / call | Cost / call |
| --- | --- | --- | --- | --- |
| `gpt-5.6-luna` · OpenAI | **20/20** (CI 0.84–1.00) | best in field — critique clean under two judges | **~3 s** | fractions of a cent |
| `gpt-5-nano` · OpenAI | 15/15 | clean critique; slow (hidden reasoning) | ~8–14 s | fractions of a cent |
| `deepseek-v4-flash-0731` · OpenRouter | 20/20 (CI 0.84–1.00) | solid; occasional routing timeouts | ~2–10 s | **$0.0001 measured** |
| `z-ai/glm-5.3-flash` · OpenRouter | 20/20 (CI 0.84–1.00) | **weakest** — 2 judges flag implausible distractors, 2 answer-key defects | ~3–19 s | ~$0.0001 |
| `qwen3.5:4b` · Ollama (CPU-only N100) | valid | not yet judge-scored | ~50–70 s | $0 — your hardware |

One finding is universal and worth knowing before you trust any generated
pack: **a strong judge picks the right answer from the choices alone far
above chance, on every model's questions.** Answer keys check out
(answer-blind verification passes across the field). Measured on a knowledge-free
fictional domain — where style is the only possible cue — guessability falls
to chance: the effect is the correct option being the only true statement,
not sloppy distractor style. A property of factual MCQs and their
knowledgeable readers, not an item defect for the learner who doesn't yet
know the fact. Two pre-registered iterations built that instrument; the raw
records ship in `evaluation/records/`.

Three honest caveats, because they are the point:

- This is a **mechanism** benchmark: wire format, structured output, timeout
  and cancellation behaviour. It is not a question-quality ranking — those
  KPIs (answer-key accuracy, distractor quality, guessability) have their own
  harness in the works ([#139](https://github.com/dezeat/golearn/issues/139)).
  The early semantic probes already discriminate: a choices-only guessability
  check flagged one budget model's questions as answerable without reading
  the question — measured, recorded, and now a target for the pipeline's
  critique stage.
- One model on one machine proves nothing about another. Local latency is
  hardware; hosted latency is routing. Measure yours — the harness is being
  productised precisely so you can.
- Findings get fixed in the open: the live campaign caught a strict-mode
  schema gap that failed every OpenAI pipeline call while fixtures stayed
  green. It is fixed and pinned by a test on this branch.

## Try it

```
export OPENAI_API_KEY=...   # or run a local Ollama and skip the key
golearn-forge generate --topic "Go goroutines and channels" \
  --provider openai --model gpt-5-nano --count 1 --allow-ungrounded
golearn-forge drafts        # review, then: drafts add <id>
golearn export go-goroutines-and-channels --out pack.yaml
```

Drafts are never library content until you accept them, a failed chain writes
nothing, and an ungrounded run says so on the record — fail-clear is a
feature, not an apology.
