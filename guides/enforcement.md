# Enforcement

Three complementary surfaces keep code aligned with the handbook. They do not duplicate each other.

```text
agent instructions, rules, skills, reviewers   → guide design; require whole-change verification
agent hooks                                     → format edited Go files, block destructive shell
                                                  commands, run fast lint + tests when an agent stops
golangci-lint + buf + tests in CI               → authoritative machine checks
```

Human and agent review remain necessary for what static analysis cannot prove: transaction boundaries, complete mapping, event meaning, retry safety, and compatibility.

## What Is Machine-Checked

| Rule | Check |
|---|---|
| Domain imports no context, storage, network, OS, logging, transport, or adapters | `depguard` rule `domain` |
| Usecases, queries, and ports import no drivers, transport, or adapters | `depguard` rule `application` |
| No context stored in structs | `containedctx` |
| Errors compared with `errors.Is`/`As` | `errorlint` |
| Sentinel and type naming (`ErrX`, `XError`) | `errname` |
| Switches over enums are exhaustive | `exhaustive` |
| Suppressions are specific and explained | `nolintlint` |
| No `fmt.Print*` logging, no `time.Sleep` outside tests | `forbidigo` |
| Consistent `slog` usage with context | `sloglint` |
| Stale idioms | `modernize`, `exptostd`, `intrange`, `usetesting` |
| Formatting and imports | `gofumpt`, `goimports` |
| Proto naming and compatibility | `buf lint`, `buf breaking` |
| Known vulnerabilities in reachable code | `govulncheck` |

Configuration: [templates/golangci.yml](../templates/golangci.yml) (verified with `golangci-lint config verify`). Everything else is **review-enforced** — do not assume a documented rule is machine-checked.

## Agent Hooks

Installed into `.claude/settings.json` by the install script; implementations in [.agents/hooks](../.agents/hooks/README.md).

| Hook | Responsibility |
|---|---|
| `PreToolUse` (Bash) | Block an enumerated list of destructive commands (`rm -rf`, `git push --force`, `git reset --hard`, `DROP TABLE`, ...) |
| `PostToolUse` (Write/Edit) | Run `goimports` (or `gofmt`) on the edited Go file |
| `Stop` | Lint and short-test the Go packages changed in the working tree; block the stop with actionable output on failure |
| `SessionStart` | Report tool availability |

The Stop hook is a fast inner-loop guard (changed packages, `-short` tests), not the release gate. Passing a lower tier never implies the higher tiers pass:

1. Stop hook — changed packages, fast.
2. `make lint` and `go test ./...` — the whole repository.
3. CI — race, integration, E2E, buf, govulncheck, image scan.

## Rolling Out a New Rule

New enforcement is additive:

- **Baseline invariant:** the shipped configuration must pass on all existing code. Stricter checks start in a warning or new-code-only tier (`golangci-lint run --new-from-rev=origin/main`).
- Promote a rule to the baseline only after existing violations are fixed and false positives are resolved.
- Record significant scoping decisions as ADRs.

## Exceptions

Use golangci-lint's specific form, adjacent to the finding, with the reason and the removal condition:

```go
return half + rand.N(half+1) //nolint:gosec // retry jitter needs spread, not unpredictability
```

- Temporary exceptions link a tracking issue.
- Permanent exceptions explain why the rule can never apply here.
- Never add a blanket `//nolint` or exclude a whole package to get a green build.

## When a Check Is Wrong

Reduce a reproducer, understand which invariant the rule protects, and either fix the code, tune the rule's scope in the shared config, or add a narrow explained exception. A failure message an agent cannot act on is a bug in the gate — every hook and CI failure must name the repair command.
