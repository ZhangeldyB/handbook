# Go Engineering Standards

Deliver production-grade Go: correct, secure, readable, testable, operable.
Implement the smallest complete solution; never trade away data integrity,
recovery, tenant isolation, or required behavior to shrink a diff. Installed
as the root `AGENTS.md`/`CLAUDE.md`, this file holds only repository-wide
rules. Read the focused sources below before changing a specialized layer.

## Authority and Scope

Identify the repository type first. Services with real business invariants
use the domain/usecase/repository/outbox rules in this handbook. Libraries,
CLIs, and generators use idiomatic package-oriented Go with no ceremonial
layers. The nearest repository instructions, accepted ADRs, generated-code
contracts, and the declared Go version override generic preferences.
Correctness, security, public contracts, and persisted-data safety outrank
style.

Review, audit, diagnosis, and planning requests mean inspect and report — do
not edit unless changes are also requested. Ask before destructive actions,
external writes, or scope expansion; check repository evidence before asking
anything the code can answer.

## Current-Only Standard

Generate new code from the maintained templates, rules, and examples; if they
disagree, stop and resolve the conflict. Existing code is evidence, not
automatically a template. This never permits breaking external contracts:
preserve or deliberately evolve public APIs, protobuf field numbers, event
topics and payloads, persisted data, configuration, and rolling deployments.

## Holistic Change Checkpoint

Do not reason from the named file or the visible error alone. Before editing,
build a change map: entry points and real callers through domain behavior,
persistence, side effects, and responses; the owner of every affected
invariant; transactions, retries, idempotency, concurrency, cancellation, and
shutdown; existing tests; one success path and the riskiest failure/retry path
traced end to end.

Repeat the checkpoint after implementing: every caller and wiring surface
updated, one owner per invariant, tests proving the system contract, no stale
path left behind. A systemically incomplete patch is not done.

## Evidence-Driven Debugging

A diagnosis request does not authorize a fix. Investigate first; report
evidence, hypotheses, unknowns, and the safest next decision. Before changing
retry, acknowledgement, credential, integration-health, or log-severity
policy, read `guides/evidence-driven-debugging.md` and follow its protocol.

## Design, Quality, and Dependency Injection

The go-quality rules file (first sources row) is the complete quality ruleset;
read it for any Go change. Non-negotiables everywhere: put policy where the
state or invariant is owned; wrap errors with `%w` and classify with
`errors.Is`/`errors.As`; handle each error once — never log and return; pass
`context.Context` first and never store it in a struct; fix the invariant,
not the symptom.

Use plain constructor injection from one obvious composition root
(`cmd/<binary>/main.go`). A component receives only the collaborators it
calls, through a `<Name>Deps` value validated once at construction
(`deps.Require`). Request paths never re-check required dependencies for nil.

## Service Architecture Rules

Apply these only to a bounded context that genuinely owns business behavior.

Domain (`internal/<context>/domain`): aggregates own state transitions and
invariants with private state and read-only getters. Domain code is
deterministic and infrastructure-free — no `context.Context`, database,
network, logging, protobuf, or environment access. Time and IDs come from the
caller. Methods validate fully before mutating, then record events through the
aggregate's tracker in the same method.

Application (`usecases/`, `queries/`, `ports/`): one file and one named type
per operation (`usecases/place.go` → `type Place`), never
`usecases/place/place.go`. Usecases load through ports, call domain methods,
and persist through the repository; they never import adapters, storage
drivers, or generated wire types. Queries are read-only and return DTOs.

Persistence: `handler → usecase → domain → repository.Save`. `Save` writes
aggregate state, its pending events (to the outbox), and any idempotency
records in **one** transaction, uses an optimistic version check, and
acknowledges pending work only after commit. A failed save leaves the
aggregate retryable. Never clear events before the commit succeeds; never
expose rows, transactions, or mutations through a port.

Transport: handlers authenticate, validate wire input, map to an application
request, call one operation, and return its error unchanged for the central
error mapper. No business transitions or storage access in handlers.

## Transactions, Events, and External Effects

A state change and its outbox record share one real database transaction;
nothing makes separate stores, brokers, or providers atomic. Assume
at-least-once delivery everywhere: stable event IDs, durable idempotency,
replay-safe consumers. Never call a provider inside a database transaction or
from an aggregate. For correctness-critical external effects: persist intent,
execute asynchronously with a stable idempotency key, record confirmed
success, confirmed failure, or *unknown*, and reconcile unknowns before any
retry. A timeout does not prove the provider did nothing. Every goroutine has
an owner, a bound, a cancellation path, and a waiter.

## Public Contracts, Security, and Tenancy

Protobufs, event topics and payloads, schemas, public errors, and
configuration are contracts. Review producer/consumer and old/new replica
overlap before changing them; never reuse a removed protobuf field number; run
`buf lint` and `buf breaking`. Derive tenant and actor identity from the
authenticated principal, never from the request body. Include the tenant in
every read, write, uniqueness rule, idempotency key, cache key, and event.
Never log secrets, tokens, or raw sensitive payloads. Bound request sizes,
page sizes, batches, retries, and concurrency.

## Go Baseline and Dependencies

<!-- Last verified: 2026-10-28 against the go1.27.0 toolchain -->

Use the current stable Go release for services; the minimum supported
language version is Go 1.26 (examples use `new(expr)`, `wg.Go`,
`t.Context()`, `b.Loop()`). Libraries may keep an older `go` directive
deliberately. Dependency updates are separate, reviewed maintenance — read
`guides/dependency-maintenance.md`.

## Testing and Verification

Tests prove behavior and contracts, not implementation trivia; full rules in
`testing/README.md` and go-quality. Cover success, invalid input, failure,
cancellation, retry, and idempotent replay; prove state is unchanged after a
rejected command and pending work survives a failed save. Use fakes at owned
ports, real-store integration tests at transaction boundaries, and no sleeps.

Before declaring done, run `make lint` and targeted `go test` for the changed
packages and report the exact commands and results. Escalate to `-race`,
integration, E2E, and `govulncheck` in proportion to risk. If a gate cannot
run, say so and state the remaining risk — never imply it passed.

## Focused Sources

Read only the sources relevant to the task, but read them fully:

| Work | Required source |
|---|---|
| Any Go change | `.handbook/.agents/rules/go-quality.md` |
| Domain aggregate | `.agents/rules/domain-rules.md`, `layers/domain/aggregates.md` |
| Usecase | `.agents/rules/usecase-rules.md`, `templates/usecase.md` |
| Repository / transactions | `.agents/rules/repo-rules.md`, `layers/infrastructure/repositories.md` |
| gRPC handler / proto | `.agents/rules/handler-rules.md`, `.agents/rules/proto-rules.md` |
| API, event, or schema contract | `guides/api-contract-evolution.md` |
| Adapter / event consumer | `.agents/rules/adapter-rules.md`, `patterns/event-consumer.md` |
| Outbox, idempotency, external effects | `patterns/outbox.md`, `patterns/idempotency.md`, `patterns/reconciliation.md` |
| Debugging / failure policy | `guides/evidence-driven-debugging.md` |
| Tests | `testing/README.md` |
| New service from scratch | `guides/new-service.md` |
| Final validation | `reference/checklist.md` |

All paths are beneath `.handbook/`; the first row shows the full prefix.

To scaffold a component, read and follow
`.handbook/.agents/skills/<name>/SKILL.md`, then verify the output against
the rules above: `scaffold-service`, `create-aggregate`, `implement-usecase`,
`add-repository`, `add-grpc-handler`, `add-query`, `add-domain-event`,
`add-event-consumer`, `add-scheduled-job`, `add-reconciler`,
`add-port-adapter`, `add-migration`, `add-e2e-test`.

## Code Review Rules

Lead with concrete findings ordered by severity: exact location, violated
invariant, realistic failure mode, specific correction. Separate correctness
defects from optional improvements; include missing tests and residual risk.
Route deep follow-ups to the specialist agents: `domain-expert`,
`security-reviewer`, `event-driven-expert`, `proto-expert`, `lint-fixer`.

Implementation reports state what changed, why it is correct, which checks
actually ran, assumptions, and open limitations. Never claim robustness without
naming the invariant that provides it.
