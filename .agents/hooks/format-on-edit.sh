#!/bin/bash
# PostToolUse hook: format the Go file Claude just edited with gofmt.
# Deliberately a no-op when gofmt is absent or the file isn't Go — never
# installs anything itself; gofmt ships with the Go toolchain.

f=$(jq -r '.tool_response.filePath // .tool_input.file_path // empty' 2>/dev/null)
[ -n "$f" ] && [ -f "$f" ] || exit 0

case "$f" in
  *.go) ;;
  *) exit 0 ;;
esac

command -v gofmt >/dev/null 2>&1 || exit 0

gofmt -w "$f" >/dev/null 2>&1 || true
