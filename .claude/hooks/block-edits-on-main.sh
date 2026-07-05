#!/bin/bash
# PreToolUse hook: refuse Edit/Write/MultiEdit while the target file's repo is
# on the protected `main` branch. Enforces CLAUDE.md workflow §7 — every change
# starts on a feature branch or worktree *before* any edits.
#
# A detached HEAD (worktree mid-operation, rebase) reports no branch name and is
# deliberately allowed: this guards against the specific mistake of editing on
# main, not against every unusual git state.

f=$(jq -r '.tool_input.file_path // empty' 2>/dev/null)
[ -n "$f" ] || exit 0

branch=$(git -C "$(dirname "$f")" rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
[ "$branch" = "main" ] || exit 0

cat <<'JSON'
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "On the protected `main` branch. Per CLAUDE.md workflow §7, create a feature branch (git checkout -b feat/...) or a worktree (parallel skill) before editing, then retry."
  }
}
JSON
