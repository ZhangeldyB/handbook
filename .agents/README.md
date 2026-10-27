# Agent Bundle

Instruction surfaces installed into every service repository by `scripts/install.sh`. They are written for coding agents (Claude Code reads all of them; the root file is shared with other agents through `AGENTS.md`).

## Layer 1 — Root Instructions

[`guides/ai-agent-instructions.md`](../guides/ai-agent-instructions.md) is copied byte-for-byte to the target's `AGENTS.md` and `CLAUDE.md`. It holds only repository-wide rules and a routing table to focused sources, within a 200-line budget.

## Layer 2 — Path-Scoped Rules (`.claude/rules/`)

Each file declares `paths:` globs and loads only when the agent works on matching files.

| Rule | Paths | Covers |
|---|---|---|
| [go-quality.md](rules/go-quality.md) | `**/*.go` | The complete Go quality ruleset: APIs, errors, concurrency, testing, modern Go |
| [domain-rules.md](rules/domain-rules.md) | `**/domain/**` | Purity, tracker lifecycle, transitions, value objects |
| [usecase-rules.md](rules/usecase-rules.md) | `**/usecases/**`, `**/queries/**`, `**/ports/**` | Ownership, persistence through `Save`, idempotency, dependencies |
| [repo-rules.md](rules/repo-rules.md) | `**/adapters/persistence/**`, `**/migrations/**` | Retry-safe save, mapping completeness, transactions |
| [handler-rules.md](rules/handler-rules.md) | `**/adapters/grpc/**` | Transport boundaries, identity, validation, error mapping |
| [adapter-rules.md](rules/adapter-rules.md) | `**/adapters/**`, `**/platform/**` | Ports, outbound adapters, event consumers |
| [proto-rules.md](rules/proto-rules.md) | `**/*.proto`, `buf*.yaml` | Naming, field semantics, evolution |

## Layer 3 — Skills (`.claude/skills/`)

Task recipes, loaded on demand or invoked as `/<name>`:

| Skill | Use for |
|---|---|
| `scaffold-service` | A new service skeleton |
| `create-aggregate` | A new aggregate and its state machine |
| `implement-usecase` | A new command |
| `add-repository` | Persistence for an aggregate |
| `add-grpc-handler` | A new RPC |
| `add-query` | A read endpoint |
| `add-domain-event` | Publishing a new event |
| `add-event-consumer` | Reacting to another context's event |
| `add-scheduled-job` | Periodic work |
| `add-reconciler` | External effects with uncertain outcomes |
| `add-port-adapter` | A new outbound dependency |
| `add-migration` | Schema changes |
| `add-e2e-test` | Public-boundary tests |

Skills point at the authoritative docs and the compiled examples instead of carrying their own copies of code, so they cannot drift from the handbook.

## Layer 4 — Subagents (`.claude/agents/`)

| Agent | When |
|---|---|
| `code-reviewer` | After writing or modifying Go code |
| `architect` | Boundaries, integration, service decomposition |
| `domain-expert` | Aggregates, invariants, state machines |
| `event-driven-expert` | Outbox, consumers, idempotency, reconciliation |
| `proto-expert` | Protobuf and gRPC contracts |
| `security-reviewer` | Authentication, tenancy, injection, secrets |
| `testing-specialist` | Test strategy and test code |
| `performance-optimizer` | Measured performance work |
| `lint-fixer` | Making the lint gate pass |
| `guidelines-checker` | Naming, style, and documentation |

Agent frontmatter never pins a model; model choice belongs to the calling tool.

## Hooks

Portable bash hooks registered through [`settings.json`](settings.json): destructive-command blocking, Go formatting after edits, fast lint and tests on stop, and a tool check at session start. See [hooks/README.md](hooks/README.md).

## Maintaining the Bundle

- One authoritative source per topic: rules hold rules, docs hold explanations, skills hold workflows, examples hold code. Link instead of restating.
- When a rule changes, update every skill, agent, template, and example that generates the affected pattern, then run `make verify`.
- Test skill and agent changes with a fresh session: [docs/skill-testing.md](docs/skill-testing.md).
