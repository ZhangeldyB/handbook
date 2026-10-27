# Validation Runbook

How to verify the handbook after changing it. Run from the repository root.

## 1. Automated Checks

```bash
make verify
```

Runs:

| Target | Checks |
|---|---|
| `verify-examples` | `go vet` and `go test` for every example module |
| `verify-lint` | `golangci-lint` with [templates/golangci.yml](../templates/golangci.yml) against `examples/orders`, plus `config verify` |
| `verify-docs` | Relative Markdown links and `.handbook/...` references resolve; banned terms and known-bad snippets are absent; the root instruction file stays within 200 lines; every rule file has a `paths:` list |
| `verify-hooks` | Hook scripts parse (`bash -n`) and the PreToolUse fixtures behave |

## 2. Install Into a Scratch Repository

```bash
tmp=$(mktemp -d)
git -C "$tmp" init -q
./scripts/install.sh "$tmp"
cmp guides/ai-agent-instructions.md "$tmp/AGENTS.md"
cmp guides/ai-agent-instructions.md "$tmp/CLAUDE.md"
ls "$tmp/.claude/rules" "$tmp/.claude/skills" "$tmp/.claude/agents"
jq '.hooks | keys' "$tmp/.claude/settings.json"
```

Then run it a second time with an existing `.claude/settings.json` that contains an unrelated key and confirm the key survives.

## 3. Hook Behavior

```bash
bash .agents/hooks/testdata/run-hook-tests.sh
```

In the scratch repository, edit a Go file through an agent session and confirm the PostToolUse hook formats it, and that the Stop hook reports a deliberately failing test with actionable output.

## 4. Agent Behavior (Canary)

In a scratch service with the handbook installed, ask a fresh agent session to:

1. "Add a `ShipOrder` command to the orders context." — Expect: it reads the usecase and domain rules, adds the method to the aggregate with validate → mutate → record, a usecase with `Deps`, persistence through `Save`, a handler using the principal, and tests including a failed-save case.
2. "Make domain methods take a context." — Expect: it refuses or flags the conflict with the domain rules.
3. "Retry the payment capture if it times out." — Expect: it identifies the unknown-outcome problem and proposes reconciliation instead of a blind retry.

If a rule did not load, check its `paths:` globs against the file the agent touched.

## 5. Consistency Review

- Search for contradictory guidance after changing a rule: `grep -rn "<old term>" --include='*.md' .`
- Every claim about a tool or library version carries a "Last verified" date; re-verify when you touch it.
- The [distribution](distribution.md) table matches what `scripts/install.sh` does.
- Agent and skill frontmatter does not pin a model.
