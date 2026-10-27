#!/usr/bin/env bash
# run-hook-tests.sh — verifies hook behavior without a Claude Code session.
# Usage: bash .agents/hooks/testdata/run-hook-tests.sh
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PRE="$DIR/../pre-tool-use.sh"
POST="$DIR/../post-tool-use.sh"
PASS=0
FAIL=0

expect() {
    local description="$1" hook="$2" input="$3" want="$4"
    local got=0
    printf '%s' "$input" | bash "$hook" >/dev/null 2>&1 || got=$?
    if [[ "$got" -eq "$want" ]]; then
        PASS=$((PASS + 1))
    else
        echo "FAIL: $description (exit $got, want $want)"
        FAIL=$((FAIL + 1))
    fi
}

bash_input() { jq -nc --arg c "$1" '{tool_name: "Bash", tool_input: {command: $c}}'; }

while IFS= read -r cmd; do
    [[ -z "$cmd" || "$cmd" == \#* ]] && continue
    expect "blocks: $cmd" "$PRE" "$(bash_input "$cmd")" 2
done < "$DIR/dangerous_commands.txt"

while IFS= read -r cmd; do
    [[ -z "$cmd" || "$cmd" == \#* ]] && continue
    expect "allows: $cmd" "$PRE" "$(bash_input "$cmd")" 0
done < "$DIR/safe_commands.txt"

expect "pre-tool-use ignores non-Bash tools" "$PRE" '{"tool_name":"Write","tool_input":{"file_path":"x.go"}}' 0
expect "post-tool-use ignores non-Go files" "$POST" '{"tool_name":"Write","tool_input":{"file_path":"README.md"}}' 0
HANDBOOK_HOOK_DRY_RUN=1 expect "post-tool-use dry run" "$POST" '{"tool_name":"Edit","tool_input":{"file_path":"main.go"}}' 0

# post-tool-use formats an unformatted Go file when a formatter is installed.
if command -v gofmt >/dev/null 2>&1; then
    tmp=$(mktemp -d)
    printf 'package main\nfunc main( ) {   }\n' > "$tmp/main.go"
    printf '{"tool_name":"Write","tool_input":{"file_path":"%s"}}' "$tmp/main.go" | bash "$POST"
    if gofmt -l "$tmp/main.go" | grep -q .; then
        echo "FAIL: post-tool-use did not format $tmp/main.go"
        FAIL=$((FAIL + 1))
    else
        PASS=$((PASS + 1))
    fi
    rm -rf "$tmp"
fi

for script in "$DIR"/../*.sh; do
    if bash -n "$script"; then PASS=$((PASS + 1)); else echo "FAIL: syntax $script"; FAIL=$((FAIL + 1)); fi
done

echo "hook tests: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
