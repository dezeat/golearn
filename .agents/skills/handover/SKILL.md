---
name: handover
description: Compact the current conversation into a handover posted to golearn's GitHub Handovers Discussion category, so a fresh Claude Code session — on any machine — can continue the work. Use at the end of a session, when context grows long, or before switching workstreams.
argument-hint: "What will the next session be used for?"
---

Write a handover summarising the current conversation so a fresh agent can
continue the work, and post it as a **GitHub Discussion** in the
**Handovers** category. Handovers live on GitHub, not in files (AGENTS.md
Operating model): a gitignored handover file is invisible on a second machine
and cannot serve the cross-machine continuity it exists for.

## Structure

1. **Goal** — what the overall task is and which roadmap item / issue it
   belongs to
2. **State** — what is done and verified, what is in progress, what is untouched
3. **Next steps** — concrete, ordered; the first one should be startable immediately
4. **Gotchas** — anything non-obvious that was learned the hard way this session
5. **Suggested skills** — which skills the next session should invoke
   (e.g. `grill-me-with-docs` before design work, `parallel` for independent
   workstreams, `pr` when opening a PR)

## Posting

Draft the body in the session scratchpad — never in the repo tree — then:

```bash
# 1. Resolve the repository and Handovers category ids
gh api graphql -f query='{ repository(owner: "dezeat", name: "golearn") {
  id discussionCategories(first: 10) { nodes { id name } } } }'

# 2. Create the discussion (title: "YYYY-MM-DD — <short topic>")
gh api graphql \
  -F repositoryId=<repo-id> -F categoryId=<handovers-category-id> \
  -F title='YYYY-MM-DD — <short topic>' -F body=@<scratchpad-file> \
  -f query='mutation($repositoryId: ID!, $categoryId: ID!, $title: String!, $body: String!) {
    createDiscussion(input: {repositoryId: $repositoryId, categoryId: $categoryId,
      title: $title, body: $body}) { discussion { number url } } }'
```

When the work maps to a GitHub issue (epics/stories/tickets live on the
Project board), drop a short pointer comment on that issue linking the
discussion. Report the discussion URL to the user at the end.

## Rules

- Do **not** duplicate content already captured in other artifacts
  (`docs/architecture.md`, `docs/DECISIONS.md`, commits, diffs, issues).
  Reference them by path, section, hash or number instead.
- **Public repo — every commit and Discussion is world-readable.** The
  bundled MCQ packs are synthetic educational content; the project needs no
  secrets by design. Never include real keys, tokens, or personal paths in a
  handover — hold it to committed-file standards, and redact anything
  sensitive the moment you notice it.
- Binary evidence (screenshots, captures) cannot be attached via the API —
  keep it in the PR that shipped the work, or locally under a gitignored
  path, and link the PR instead.
- If the user passed arguments, treat them as a description of what the next
  session will focus on and tailor the document accordingly.
