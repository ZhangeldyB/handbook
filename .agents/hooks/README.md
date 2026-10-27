# Agent Hooks

Portable bash hooks for Claude Code, registered by [`../settings.json`](../settings.json) and installed into `.claude/settings.json` by `scripts/install.sh`. They require `bash` and `jq`; `goimports`/`gofmt`, `golangci-lint`, and `go` are used when present.

| Script | Event | Responsibility | Can block? |
|---|---|---|---|
| `pre-tool-use.sh` | PreToolUse (Bash) | Block an enumerated list of destructive commands | Yes — exit 2 with the reason on stderr |
| `post-tool-use.sh` | PostToolUse (Write/Edit/MultiEdit) | Format the edited Go file; skip generated files | No |
| `stop.sh` | Stop | `golangci-lint run` and `go test -short` on changed packages | Yes — JSON `{"decision":"block","reason":...}` on stdout |
| `session-start.sh` | SessionStart | Report tool availability | No |

Architecture rules are deliberately **not** implemented as regex checks here. `golangci-lint` (with `depguard`) is their single source of truth, run by the Stop hook, `make lint`, and CI — one AST-aware implementation instead of drifting copies.

## Stop Hook Details

- Only packages containing changed or untracked `.go` files are checked; generated (`gen/`), vendored, and `.handbook/` directories are skipped, as are `*/e2e/*` packages unless `HANDBOOK_STOP_INCLUDE_E2E=1`.
- With no changed Go files it exits immediately — running the whole repository would block the agent on pre-existing failures and trap it in a loop.
- `stop_hook_active` in the input breaks continuation loops.
- `HANDBOOK_STOP_SKIP=1` disables it for a session.
- It is the fast tier. Passing it does not imply `make lint`, `go test ./...`, or CI pass; agents state which gate they actually ran.

Every failure message names the command that reproduces it. A failure an agent cannot act on is a bug in the hook.

## Tests

```bash
bash .agents/hooks/testdata/run-hook-tests.sh
```

Covers every line of `dangerous_commands.txt` (must block) and `safe_commands.txt` (must allow), non-Bash passthrough, PostToolUse formatting and dry run, and `bash -n` for every script. Add a line to the fixture files whenever you change a pattern.

## Limits

The PreToolUse blocker is a guard rail against accidents, not a security boundary: a determined command can always be phrased to evade a pattern list. Keep the permission mode and human review as the real controls.
