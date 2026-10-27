#!/usr/bin/env bash
# session-start.sh — SessionStart hook.
#
# Reports which tools the handbook's workflow expects are available. It never
# blocks; missing tools are reported with an install hint.

MISSING=""

check() {
    local tool="$1" hint="$2"
    if command -v "$tool" >/dev/null 2>&1; then
        echo "$tool: available"
    else
        MISSING+=$'\n'"  $tool not found — $hint"
    fi
}

if command -v go >/dev/null 2>&1; then
    echo "$(go version)"
else
    MISSING+=$'\n  go not found — https://go.dev/dl/'
fi
check goimports "go install golang.org/x/tools/cmd/goimports@latest"
check golangci-lint "https://golangci-lint.run/welcome/install/"
check buf "go install github.com/bufbuild/buf/cmd/buf@<pinned version> (see Makefile)"
check govulncheck "go install golang.org/x/vuln/cmd/govulncheck@<pinned version> (see Makefile)"
check jq "required by the hooks — https://jqlang.org/download/"

if [[ -n "$MISSING" ]]; then
    echo "Missing tools:$MISSING"
fi
exit 0
