---
name: scout
description: Before charting a big idea, scan outward — web-search whether the concept already exists as a named, popular pattern, how existing tools and libraries solve it, and the standard vocabulary and known pitfalls — then produce a short, cited briefing that grounds the design in the state of the art instead of reinventing it. Use at the start of a big or foggy idea, or when a design smells like a solved problem.
---

A big idea has arrived. Before charting a route through it or interrogating it against your own docs, look **outward**: almost every substantial idea is a partly-solved problem that already has a name, a handful of popular approaches, a standard shape, and a set of known failure modes. `scout` goes ahead of the expedition and reports the terrain — so the design starts grounded in what already exists rather than from a blank slate.

## Where it sits

Three complementary moves; `scout` is the only one that points outward.

| Skill                    | Points                        | Answers                                                                                   |
| ------------------------ | ----------------------------- | ----------------------------------------------------------------------------------------- |
| **`scout`**              | outward — the web / prior art | _Does this already exist? What is it called? How do others do it? What are the pitfalls?_ |
| **`wayfinder`**          | inward — the issue tracker    | _What must we decide?_ (the map of decision tickets)                                      |
| **`grill-me-with-docs`** | inward — our own docs         | _What do we decide?_ (one question at a time)                                             |

`wayfinder` and `grill-me-with-docs` are **optional companions** — if this repo has them, `scout` feeds them; if not, `scout` stands alone as an ideation-time research pass. `scout` never depends on them.

## When to use

- At the **start of a big or foggy idea**, before naming a destination or cutting tickets — its briefing sharpens the destination and seeds the option space with real, named alternatives.
- When a design **smells like a solved problem** — "surely someone has a standard pattern for this."
- As the resolver for a **research / prior-art question** inside a larger planning effort (e.g. a `wayfinder` research ticket): "survey how the popular libraries in this space are built," "does adopting framework X earn its place."

## The flow

1. **Frame for search.** Turn the loose idea into candidate _canonical_ terms — the words the field actually uses, not the words in the prompt. If you don't know them yet, an exploratory search to find the vocabulary is the first step.
2. **Fan out.** Web-search across distinct angles, not one query: the named concept/pattern, the popular tools & libraries that implement it, the standard architecture(s), the documented pitfalls and anti-patterns, and any relevant benchmarks or comparisons. Fetch the primary sources — a project's own docs beat a listicle about it.
3. **Sort real prior art from superficial matches.** Adversarially check that a "match" solves the _same_ problem under your constraints, not a look-alike. A popular tool that assumes a server, a GPU, or an always-online client is not prior art for an offline single-binary — say so.
4. **Brief.** Produce a compact, **cited** report: _what this is commonly called · the popular approaches (A/B/C) and how they differ · the standard pattern and where it bends · the known pitfalls · the vocabulary worth adopting · and — sharpest of all — what is genuinely novel here versus already solved._ Every non-obvious claim carries a source link.
5. **Hand off.** Feed the briefing into whatever comes next — sharpen a `wayfinder` destination, seed its fog with named options, adopt the canonical terms in `grill-me-with-docs`, or just inform the decision at hand.

## Guardrails

- **Web search is a dev-time planning act, not a product feature.** Scouting the field is something _you_ do while designing; it says nothing about whether the _product_ may reach the network at runtime. Never let a `scout` session be cited as precedent for a runtime network call, telemetry, or a dependency the product's invariants forbid — the two live in different worlds.
- **Prior art informs, it never overrides invariants.** A pattern can be popular, canonical, and still disqualified because it needs a dependency, a runtime, or a coupling this repo's laws reject (read them — e.g. `AGENTS.md`, the architecture spec, the decision log). Surface the popular option _and_ the reason it does or doesn't survive the constraints. The field's consensus does not get a vote your architecture didn't grant it.
- **Cite or cut it.** A briefing without sources is a guess wearing a suit. If a claim can't be traced to a fetched source, mark it as an assumption or drop it.

## Where the output lands

The briefing is **exploratory, not binding** — it belongs in the non-binding idea parking lot (an _Ideas_ Discussion, a `docs/IDEAS`, or wherever this repo keeps loose ideas), or as a context-pointer comment on the planning issue/ticket it informs. It does **not** become a spec or a decision record on its own. Prior art that ends up _deciding_ something graduates into the rationale of the proper decision entry (the repo's ADR / `DECISIONS` log), cited there — the scout briefing is the evidence, not the verdict.
