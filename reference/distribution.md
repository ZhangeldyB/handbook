# Distribution

The handbook is installed into each service repository by [`scripts/install.sh`](../scripts/install.sh). Nothing is installed by hand.

## What the Script Does

```bash
./scripts/install.sh /path/to/service          # from a handbook checkout
./scripts/install.sh --force /path/to/service  # also overwrite unowned files at handbook paths
```

| Source (this repository) | Destination (target repository) | Behavior |
|---|---|---|
| Everything except repo-meta files | `.handbook/` | Replaced (removed, then copied); a `VERSION` file inside it records the handbook commit |
| `guides/ai-agent-instructions.md` | `AGENTS.md`, `CLAUDE.md` | Replaced, byte-identical to the source |
| `.agents/rules/*.md` | `.claude/rules/` | Handbook-owned files replaced; other files untouched |
| `.agents/skills/<name>/` | `.claude/skills/` | Handbook-owned directories replaced; others untouched |
| `.agents/agents/*.md` | `.claude/agents/` | Handbook-owned files replaced; others untouched |
| `.agents/settings.json` | `.claude/settings.json` | Merged with `jq`: previous handbook hook entries are removed and the current ones appended; every other setting and hook survives |

Repo-meta files are not copied: `.git`, `.github`, `.gitignore`, `CLAUDE.md` (this repository's authoring guide), `Makefile`, `go.work`, and `tools/`.

Because the copy is recursive, a new file in this repository is distributed automatically. Every installed file is production agent context: do not keep superseded material, archives, or investigation notes here — Git keeps history.

## Ownership

The script records every `.claude/` path it installs in `.claude/.handbook-manifest`. On the next install it removes exactly those paths before copying the new set, so:

- a rule, skill, or agent removed from the handbook disappears from the target;
- repository-local rules, skills, and agents with their own names (for example `.claude/rules/team-conventions.md`) are never touched;
- if a file already exists at a path the handbook wants and the manifest does not list it, the script stops instead of overwriting it (use `--force` after checking).

`.handbook/`, `AGENTS.md`, and `CLAUDE.md` are always replaced. Keep repository-specific root instructions in a separate, local rule or document rather than editing the projected root files.

## Paths

- Installed references use the `.handbook/...` prefix (for example `.handbook/patterns/outbox.md`).
- Inside the handbook, documents link to each other with relative paths, which resolve both here and under `.handbook/`.
- Directories starting with `.` are ignored by the Go tool, so the example modules under `.handbook/examples/` never affect the service's build, tests, or lint.

## Updating

Re-run the script after pulling a new handbook version, review the diff in the target repository, and commit it as its own change.

## Validation Contract

After changing the handbook or the script, follow the [Validation Runbook](validation-runbook.md).
