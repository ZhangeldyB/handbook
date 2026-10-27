#!/usr/bin/env bash
# pre-tool-use.sh — PreToolUse hook for Bash commands.
#
# Blocks an enumerated list of destructive shell commands. Architecture rules
# are not checked here: golangci-lint (depguard) is their single source of
# truth, run by the Stop hook, `make lint`, and CI.
#
# Exit codes: 0 = allow, 2 = block (reason on stderr).

set -euo pipefail

INPUT=$(cat)
TOOL_NAME=$(printf '%s' "$INPUT" | jq -r '.tool_name // empty')
[[ "$TOOL_NAME" != "Bash" ]] && exit 0

COMMAND=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // empty')
[[ -z "$COMMAND" ]] && exit 0
LOWER=$(printf '%s' "$COMMAND" | tr '[:upper:]' '[:lower:]')

block() {
    echo "BLOCKED: $1" >&2
    exit 2
}

# Recursive force delete.
if [[ "$COMMAND" =~ rm[[:space:]]+-[a-zA-Z]*r[a-zA-Z]*f ]] || [[ "$COMMAND" =~ rm[[:space:]]+-[a-zA-Z]*f[a-zA-Z]*r ]]; then
    block "rm -rf is destructive. Remove specific files, or ask the user to run it."
fi

# History rewrites and discarded work.
if [[ "$COMMAND" =~ git[[:space:]]+push.*(--force|[[:space:]]-f([[:space:]]|$)) ]] && [[ ! "$COMMAND" =~ --force-with-lease ]]; then
    block "git push --force destroys remote history. Use --force-with-lease, and only when the user asked."
fi
if [[ "$COMMAND" =~ git[[:space:]]+reset.*--hard ]]; then
    block "git reset --hard discards uncommitted work. Use git stash, or ask the user."
fi
if [[ "$COMMAND" =~ git[[:space:]]+clean.*-[a-zA-Z]*f ]]; then
    block "git clean -f deletes untracked files permanently. Run git clean -n first and ask the user."
fi
if [[ "$COMMAND" =~ git[[:space:]]+checkout[[:space:]]+--[[:space:]]+\. ]] || [[ "$COMMAND" =~ git[[:space:]]+restore[[:space:]]+\.([[:space:]]|$) ]]; then
    block "Discarding all working-tree changes is irreversible. Restore specific files, or ask the user."
fi

# Data destruction.
if [[ "$LOWER" =~ drop[[:space:]]+(table|database|schema) ]]; then
    block "DROP is irreversible. Use a reviewed migration."
fi
if [[ "$LOWER" =~ truncate[[:space:]]+table ]]; then
    block "TRUNCATE deletes every row. Use a reviewed migration or a scoped DELETE."
fi
if [[ "$LOWER" =~ delete[[:space:]]+from ]] && [[ ! "$LOWER" =~ where ]]; then
    block "DELETE without WHERE deletes every row. Add a WHERE clause."
fi

# Permissions, clusters, containers, disks.
if [[ "$COMMAND" =~ chmod[[:space:]]+(-[a-zA-Z]*[[:space:]]+)?777 ]]; then
    block "chmod 777 makes files world-writable. Use specific permissions (644, 755)."
fi
if [[ "$COMMAND" =~ chmod[[:space:]]+-[a-zA-Z]*R ]]; then
    block "Recursive chmod is dangerous. Change permissions on specific files."
fi
if [[ "$COMMAND" =~ kubectl[[:space:]]+delete ]]; then
    block "kubectl delete destroys cluster resources. Ask the user to run it."
fi
if [[ "$COMMAND" =~ docker[[:space:]]+(rm|rmi)[[:space:]]+-[a-zA-Z]*f ]] || [[ "$COMMAND" =~ docker[[:space:]]+system[[:space:]]+prune ]]; then
    block "Force-removing containers or images. Ask the user to run it."
fi
if [[ "$COMMAND" =~ (^|[[:space:];&|])dd[[:space:]]+if= ]] || [[ "$COMMAND" =~ mkfs ]]; then
    block "Disk-level operations are never run by an agent."
fi
if [[ "$COMMAND" =~ :\(\)[[:space:]]*\{ ]]; then
    block "Fork bomb detected."
fi

exit 0
