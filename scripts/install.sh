#!/usr/bin/env bash
# install.sh — install this handbook into a service repository.
#
# Usage: scripts/install.sh [--force] <target-repository>
#
#   .handbook/                 full handbook (replaced on every install)
#   AGENTS.md, CLAUDE.md       copies of guides/ai-agent-instructions.md
#   .claude/rules/*.md         path-scoped rules       ┐ files the handbook owns are
#   .claude/skills/<name>/     skills                   │ tracked in .claude/.handbook-manifest
#   .claude/agents/*.md        subagents                ┘ and replaced; other files are untouched
#   .claude/settings.json      handbook hook entries merged; unrelated settings kept
#
# --force overwrites a pre-existing file at a path the handbook wants to own.
# Requires bash, tar, and jq.
set -euo pipefail

FORCE=0
if [[ "${1:-}" == "--force" ]]; then
    FORCE=1
    shift
fi
if [[ $# -ne 1 ]]; then
    echo "usage: $0 [--force] <target-repository>" >&2
    exit 64
fi

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$(cd "$1" && pwd)"
if [[ "$SRC" == "$TARGET" ]]; then
    echo "error: target is the handbook itself" >&2
    exit 64
fi
for tool in tar jq; do
    command -v "$tool" >/dev/null 2>&1 || { echo "error: $tool is required" >&2; exit 69; }
done

# 1. The full handbook under .handbook/, minus repository meta files.
rm -rf "$TARGET/.handbook"
mkdir -p "$TARGET/.handbook"
(
    cd "$SRC"
    tar -cf - \
        --exclude=./.git --exclude=./.github --exclude=./.gitignore \
        --exclude=./.idea --exclude=./.vscode \
        --exclude=./CLAUDE.md --exclude=./Makefile --exclude=./go.work --exclude=./go.work.sum \
        --exclude=./tools \
        .
) | (cd "$TARGET/.handbook" && tar -xf -)

VERSION="unknown"
if git -C "$SRC" rev-parse --git-dir >/dev/null 2>&1; then
    VERSION="$(git -C "$SRC" describe --tags --always --dirty 2>/dev/null || echo unknown)"
fi
printf '%s\n' "$VERSION" > "$TARGET/.handbook/VERSION"

# 2. Root instructions.
cp "$SRC/guides/ai-agent-instructions.md" "$TARGET/AGENTS.md"
cp "$SRC/guides/ai-agent-instructions.md" "$TARGET/CLAUDE.md"

# 3. Managed .claude files.
CLAUDE_DIR="$TARGET/.claude"
MANIFEST="$CLAUDE_DIR/.handbook-manifest"
mkdir -p "$CLAUDE_DIR/rules" "$CLAUDE_DIR/skills" "$CLAUDE_DIR/agents"

touch "$MANIFEST"
previously_installed() { grep -qxF "$1" "$MANIFEST"; }

# Planned paths, relative to .claude/.
PLANNED=()
for f in "$SRC"/.agents/rules/*.md; do PLANNED+=("rules/$(basename "$f")"); done
for d in "$SRC"/.agents/skills/*/; do PLANNED+=("skills/$(basename "$d")"); done
for f in "$SRC"/.agents/agents/*.md; do PLANNED+=("agents/$(basename "$f")"); done

# Refuse to clobber files the handbook did not install.
for path in "${PLANNED[@]}"; do
    if [[ -e "$CLAUDE_DIR/$path" ]] && ! previously_installed "$path" && [[ "$FORCE" -ne 1 ]]; then
        echo "error: $CLAUDE_DIR/$path exists and was not installed by the handbook; rename it or re-run with --force" >&2
        exit 73
    fi
done

# Remove everything the previous install owned, then copy the new set.
while IFS= read -r path; do
    [[ -n "$path" ]] && rm -rf "${CLAUDE_DIR:?}/$path"
done < "$MANIFEST"
for path in "${PLANNED[@]}"; do
    rm -rf "${CLAUDE_DIR:?}/$path"
done
cp "$SRC"/.agents/rules/*.md "$CLAUDE_DIR/rules/"
cp -R "$SRC"/.agents/skills/* "$CLAUDE_DIR/skills/"
cp "$SRC"/.agents/agents/*.md "$CLAUDE_DIR/agents/"
printf '%s\n' "${PLANNED[@]}" | sort > "$MANIFEST"

# 4. Merge hook registrations: drop previous handbook hook entries, append the
# current ones, and keep every other setting.
SETTINGS="$CLAUDE_DIR/settings.json"
HOOKS="$SRC/.agents/settings.json"
if [[ -f "$SETTINGS" ]]; then
    merged="$(mktemp)"
    jq -s '
      .[0] as $current | .[1] as $handbook
      | $current
      | .hooks = (reduce ($handbook.hooks | keys[]) as $event ((.hooks // {});
          .[$event] = (
            ((.[$event] // []) | map(select(
              (.hooks // []) | all((.command // "") | contains(".handbook/.agents/hooks/") | not)
            )))
            + $handbook.hooks[$event]
          )))
    ' "$SETTINGS" "$HOOKS" > "$merged"
    mv "$merged" "$SETTINGS"
else
    cp "$HOOKS" "$SETTINGS"
fi

echo "Installed handbook $VERSION into $TARGET"
echo "  .handbook/, AGENTS.md, CLAUDE.md"
echo "  .claude/: ${#PLANNED[@]} managed rules, skills, and agents (see .claude/.handbook-manifest)"
echo "  .claude/settings.json: hook entries merged"
echo "Review the diff and commit it as its own change."
