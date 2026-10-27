#!/usr/bin/env bash
# stop.sh — Stop hook.
#
# Lints and short-tests the Go packages changed in the working tree when the
# agent finishes. On failure it prints Claude Code's documented JSON
# {"decision":"block","reason":...} on stdout and exits 0, so the multi-line
# diagnostics reach the agent. This is a fast inner-loop gate, not the release
# gate: `make lint`, `go test ./...`, and CI still run the full checks.
#
# Environment:
#   HANDBOOK_STOP_SKIP=1          skip entirely for this session
#   HANDBOOK_STOP_INCLUDE_E2E=1   also test packages under */e2e/*

HOOK_INPUT=""
if [[ ! -t 0 ]]; then
    HOOK_INPUT=$(cat || true)
fi

# Break continuation loops: when the agent is already continuing because of a
# previous block, allow it to stop.
if [[ -n "$HOOK_INPUT" ]] && command -v jq >/dev/null 2>&1; then
    if [[ "$(printf '%s' "$HOOK_INPUT" | jq -r '.stop_hook_active // false' 2>/dev/null)" == "true" ]]; then
        exit 0
    fi
fi
[[ "${HANDBOOK_STOP_SKIP:-}" == "1" ]] && exit 0
command -v go >/dev/null 2>&1 || exit 0
git rev-parse --git-dir >/dev/null 2>&1 || exit 0

# Changed Go files: modified/added/renamed (not deleted) plus untracked.
CHANGED_FILES=$(
    {
        git diff --name-only --diff-filter=ACMR HEAD 2>/dev/null
        git ls-files --others --exclude-standard 2>/dev/null
    } | grep '\.go$' | sort -u
)

# Nothing changed: verifying the whole repository here would block on
# pre-existing failures and trap the agent in a loop.
if [[ -z "$CHANGED_FILES" ]]; then
    exit 0
fi

PKGS=""
while IFS= read -r dir; do
    [[ -z "$dir" || ! -d "$dir" ]] && continue
    # Skip generated code, vendored code, and the installed handbook.
    [[ "$dir" =~ (^|/)(gen|vendor|third_party|\.handbook)(/|$) ]] && continue
    if [[ "${HANDBOOK_STOP_INCLUDE_E2E:-}" != "1" && "$dir" =~ (^|/)e2e(/|$) ]]; then
        continue
    fi
    PKGS+="./$dir "
done < <(printf '%s\n' "$CHANGED_FILES" | xargs -n1 dirname | sort -u)
PKGS=$(echo "$PKGS" | xargs)
[[ -z "$PKGS" ]] && exit 0

REASON=""

if command -v golangci-lint >/dev/null 2>&1; then
    # shellcheck disable=SC2086 # word splitting of the package list is intended
    if ! LINT_OUT=$(golangci-lint run $PKGS 2>&1); then
        REASON+=$'LINT FAILURES (fix, then re-run `golangci-lint run`):\n'"$LINT_OUT"$'\n\n'
    fi
fi

# shellcheck disable=SC2086
if ! TEST_OUT=$(go test -count=1 -short $PKGS 2>&1); then
    REASON+=$'TEST FAILURES (fix, then re-run `go test -short '"$PKGS"$'`):\n'"$TEST_OUT"$'\n\n'
fi

[[ -z "$REASON" ]] && exit 0

REASON+='To skip this check for one session: HANDBOOK_STOP_SKIP=1.'
if command -v jq >/dev/null 2>&1; then
    jq -nc --arg r "$REASON" '{decision: "block", reason: $r}'
else
    printf '%s\n' "$REASON" >&2
    exit 2
fi
exit 0
