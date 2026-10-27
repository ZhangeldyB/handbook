#!/usr/bin/env bash
# post-tool-use.sh — PostToolUse hook for Write/Edit/MultiEdit.
#
# Formats the edited Go file with goimports (falling back to gofmt). It never
# blocks: PostToolUse runs after the edit, and formatting failures surface in
# the Stop hook's lint run instead.
#
# Set HANDBOOK_HOOK_DRY_RUN=1 to skip formatting (used by hook tests).

INPUT=$(cat)
FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null)

[[ -z "$FILE_PATH" || "$FILE_PATH" != *.go || ! -f "$FILE_PATH" ]] && exit 0
[[ "${HANDBOOK_HOOK_DRY_RUN:-}" == "1" ]] && exit 0

# Never rewrite generated code.
if head -n 5 "$FILE_PATH" | grep -q '^// Code generated .* DO NOT EDIT\.$'; then
    exit 0
fi

if command -v goimports >/dev/null 2>&1; then
    goimports -w "$FILE_PATH" 2>/dev/null
elif command -v gofmt >/dev/null 2>&1; then
    gofmt -w "$FILE_PATH" 2>/dev/null
fi

exit 0
