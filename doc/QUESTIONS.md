# golearn — Question Writing Standard

## 1. Purpose

This document is the canonical guide for authoring golearn question packs. It applies to:

- **Humans** writing question packs manually
- **Agents** generating question packs programmatically
- **Reviewers** validating question quality before merging

Every question committed to this repository must conform to this specification.

---

## 2. Question Philosophy

### Core principles

- **No trick questions.** Questions must test understanding, not reading comprehension traps.
- **No ambiguity.** Every question must have exactly one defensible correct answer set.
- **No outdated APIs.** Do not test deprecated features, removed configurations, or sunset services.
- **No guessing-based distractors.** Every wrong answer must represent a realistic misconception that a practitioner might hold.
- **Focus on conceptual understanding.** Questions should test *why* and *how*, not rote memorisation of syntax.

### Distractor quality

Each incorrect answer (distractor) must:

1. Be plausible to someone with partial knowledge
2. Represent a common misunderstanding or confusion
3. Not be obviously absurd or off-topic
4. Be roughly the same length and specificity as the correct answer

---

## 3. Structure Rules

### Question types

| Type            | Correct Answers        | Validation Rule                     |
|-----------------|------------------------|-------------------------------------|
| `single_select` | Exactly 1              | `len(correct_choice_ids) == 1`      |
| `multi_select`  | 1 or more              | `len(correct_choice_ids) >= 1`      |

### Structural constraints

- All choices must be **plausible** — no filler options.
- All `correct_choice_ids` must reference existing `Choice.id` values.
- No **overlapping semantics** between choices (two choices should not mean the same thing).
- Choice text length should be **roughly balanced** — avoid one choice being 5× longer than others.
- Avoid **"All of the above"** and **"None of the above"** — these are testing anti-patterns.
- Minimum **2 choices**, recommended **4 choices** for single select statements.
- Choice IDs are stable internal identifiers (not UI labels).
- Recommended choice ID styles:
  - Numeric strings: `"1"`, `"2"`, `"3"`
  - Prefixed IDs: `"opt_1"`, `"opt_2"`
  - Other short stable tokens
- UI labels (`A/B/C/...`) are generated at render time from current display order (including shuffled order).
- `Choice.id` must remain stable for `correct_choice_ids`, `rationale.per_choice`, hashing, and DB storage.

### Optional fields

| Field        | When to use                                                |
|--------------|------------------------------------------------------------|
| `intro`      | When a question needs context (scenario, code block, etc.) |
| `tags`       | Always recommended — enables filtering by topic            |
| `difficulty` | `easy`, `medium`, or `hard` (optional enum)                |
| `source_ref` | Strongly recommended for certification packs               |
| `confidence` | Defaults to 1.0 for manually authored questions            |

---

## 4. Explanation Rules

Every question **MUST** include a `rationale` section:

```yaml
rationale:
  correct: "Explanation of why the correct concept is true."
  per_choice:
    "1": "Why this choice is correct or incorrect."
    "2": "Why this choice is correct or incorrect."
    "3": "Why this choice is correct or incorrect."
    "4": "Why this choice is correct or incorrect."
```

### Rules

- Every choice must have a `per_choice` explanation — no exceptions.
- Wrong choices must explain **why they are wrong**, not just state "this is incorrect."
- Explanations must reference **specific concepts** (e.g., "Auto Loader uses `cloudFiles` format to..."), not vague phrases (e.g., "this is not how it works").
- Correct explanations should be self-contained — a reader should understand the answer without prior knowledge.
- Explanation text must be content-only; do not start with prefixes like `Correct:`, `Incorrect:`, `✅`, or `❌`.
- Each explanation should stand alone and be readable without an explicit correctness label.
- Where possible, include `source_ref` pointing to official documentation.

### Anti-patterns to avoid

| Bad                                         | Good                                                    |
|---------------------------------------------|---------------------------------------------------------|
| "This is wrong."                            | "VACUUM default retention is 7 days, not 30 days."      |
| "Not the correct answer."                   | "checkpointLocation stores offsets, not schema data."   |
| "B is better."                              | "WriteSerializable permits concurrent appends, unlike Serializable which blocks on any read-write conflict." |

---

## 5. Tone Guidelines

- **Certification-style** — match the register of professional exams.
- **Neutral** — no humour, opinions, or subjective language.
- **Technically precise** — use exact terminology from official documentation.
- **No conversational filler** — avoid "In this scenario..." or "As you know..."
- **No exam trick phrasing** — avoid double negatives, "which is NOT", or misleading qualifiers.

---

## 6. Validation Checklist

Before committing a question pack, verify every item:

- [ ] Each question is **unambiguous** — one defensible correct answer set
- [ ] Official documentation supports correctness
- [ ] All distractors represent **realistic misunderstandings**
- [ ] No **deprecated features** or removed APIs are tested
- [ ] Pack passes `golearn import` without errors
- [ ] No duplicate hashes after normalisation
- [ ] All `correct_choice_ids` reference valid choice IDs
- [ ] Every choice has a `per_choice` explanation
- [ ] Choice text is roughly balanced in length
- [ ] `single_select` questions have exactly 1 correct answer
- [ ] `multi_select` questions have ≥1 correct answer

---

## 7. Reference Policy

Certification packs **should** include documentation references:

```yaml
source_ref: "https://docs.databricks.com/en/delta/vacuum.html"
```

References are not mandatory for general knowledge packs, but are **strongly recommended** for:

- Certification preparation packs
- Technology-specific packs
- Any pack where correctness may be debated

Preferred documentation sources:

- Official vendor documentation (e.g., `docs.databricks.com`)
- Language specifications (e.g., Go spec)
- RFC documents for standards-based topics
- Peer-reviewed technical resources

---

## 8. Gold Standard Example

```yaml
- type: "single_select"
  intro: >-
    VACUUM removes data files that are no longer referenced by the
    Delta transaction log.
  prompt: "What is the default retention threshold used by VACUUM on a Delta table?"
  choices:
    - { id: "1", text: "24 hours" }
    - { id: "2", text: "7 days" }
    - { id: "3", text: "30 days" }
    - { id: "4", text: "90 days" }
  correct_choice_ids: ["2"]
  tags: ["delta-lake", "vacuum"]
  difficulty: easy
  rationale:
    correct: >-
      Delta Lake defaults to a 7-day (168-hour) retention threshold
      for VACUUM. Files older than this threshold that are no longer
      in the current table version can be deleted.
    per_choice:
      "1": >-
        24 hours is too short. Using such a low retention
        can break time travel and concurrent readers. The default is
        7 days.
      "2": >-
        The default retention is 7 days (168 hours), providing
        a safe window for time travel queries and long-running jobs.
      "3": >-
        30 days is not the default. Users may configure
        longer retention, but the out-of-the-box setting is 7 days.
      "4": >-
        90 days is not the default retention period for
        VACUUM.
  source_ref: "https://docs.databricks.com/en/delta/vacuum.html"
```

This example demonstrates:

- Clear, unambiguous prompt
- Optional intro providing context
- Stable internal choice IDs decoupled from UI labels
- Four plausible choices with balanced length
- Exactly one correct answer for `single_select`
- Per-choice explanations that educate, not just evaluate
- Official documentation reference
- Appropriate tags and difficulty rating
