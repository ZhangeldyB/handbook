# Anti-patterns

Use these as review signals, not as substitutes for understanding the invariant. Compiling Bad/Good pairs live in [`examples/antipatterns`](../examples/antipatterns/README.md).

## File-Local Fix Without a System Trace

Changing one function while ignoring its callers, authorization, transaction and outbox behavior, contracts, configuration, wiring, and rollout produces locally tidy, globally wrong code. Trace the system before and after the change.

## Clearing Events Before Commit

```go
events := order.DrainEvents() // pending work is gone
return db.InTx(ctx, func(tx Tx) error { /* write state + events */ })
```

If the transaction fails, a retry commits the state without its events. Snapshot, commit, then acknowledge ([ADR-004](../decisions/004-acknowledge-after-commit.md)).

## Pretend Multi-Store Atomicity

Writes to two databases, a database and a broker, or a database and a provider are not atomic because they are in one function or one "plan" object. Commit state plus durable intent, deliver at least once, process idempotently, reconcile.

## External Call Inside a Transaction

Transaction callbacks may be retried by the driver, and a rollback after a successful provider call leaves an effect with no local record. Persist intent; call the provider from a worker.

## Business Logic at the Edge

Handlers and adapters decide nothing about valid transitions, pricing, eligibility, or cross-entity policy. The inverse is also harmful: no I/O, contexts, SDK types, environment reads, or logging inside aggregates.

## Leaking Storage and Wire Types

Rows, transactions, query builders, and protobuf messages do not cross into the domain or application layers. Map at the owning adapter.

## Incomplete Mapping

A field added to the schema or the snapshot but not mapped silently disappears. Map everything through `toRow`/`toSnapshot` and keep a reflection-checked round-trip test.

## Last-Write-Wins by Accident

Saving without a version check silently overwrites concurrent changes. Every update is compare-and-set on the version ([ADR-005](../decisions/005-optimistic-concurrency.md)).

## Trusting the Request for Identity

A tenant or actor ID from the request body lets any caller act on any tenant. Derive identity from the authenticated principal and include the tenant in every key.

## Broad Interfaces and Wrapper Forests

An interface mirroring a whole SDK, or a chain of pass-through facades, hides ownership. Define small interfaces at the consumer; add a layer only for a real translation, policy, lifecycle, or seam.

## Hidden Stateful Workflow

A request-scoped object whose methods communicate through mutable fields makes ordering and partial failure invisible. Keep workflow values local and pass them explicitly.

## Log and Return Everywhere

Logging the same error in the repository, the usecase, and the handler creates noise and duplicates sensitive data. Wrap, return, and log once at the boundary.

## Classifying Errors by Text

`strings.Contains(err.Error(), "not found")` breaks on rewording and matches unrelated errors. Use `errors.Is`/`errors.As` and status codes.

## Blind Retry

Not every error is retryable. Distinguish business rejection, transient failure, permanent failure, cancellation, and unknown outcome; retry only safe operations with capped, jittered, cancellation-aware backoff.

## Unowned Goroutines

Fire-and-forget goroutines leak, outlive their dependencies, lose errors, and break shutdown. Every goroutine has an owner, a bound, a stop signal, and a waiter.

## Floating-Point Money

`float64` cannot represent most decimal amounts. Use integer minor units or an exact decimal ([Money](../layers/shared/money.md)).

## Numeric Quality Theater

Universal line limits, coverage percentages, test pyramids, and parameter-count rules are poor proxies for cohesion and risk. Require contract and failure coverage and understandable ownership.

## Compatibility by Accident

Hand-editing generated code, reusing protobuf field numbers, changing status-code meaning, or publishing an incompatible event without a migration all break consumers you cannot see. Change the source, regenerate, run `buf breaking`, and review old/new overlap.

## Stale Idioms

Training data is full of once-correct Go. Never generate the left column:

| Stale | Use instead |
|---|---|
| `io/ioutil` | `os`, `io` |
| `interface{}` | `any` |
| `golang.org/x/exp/slices`, `.../maps` | stdlib `slices`, `maps` |
| `x := x` loop-variable copies | delete (per-iteration variables since Go 1.22) |
| `for i := 0; i < b.N; i++` | `for b.Loop()` |
| `rand.Seed`, global `math/rand` | `math/rand/v2` |
| `os.Setenv` in tests | `t.Setenv` |
| `context.Background()` in tests | `t.Context()` |
| `wg.Add(1); go func() { defer wg.Done() }()` | `wg.Go(func() { ... })` |
| `grpc.Dial` / `grpc.DialContext` | `grpc.NewClient` |
| `log.Printf` for service logs | `log/slog` |
| `go.uber.org/automaxprocs` | the runtime's container-aware default |

`modernize`, `exptostd`, `intrange`, and `usetesting` in the [lint config](../templates/golangci.yml) catch most of these.

## Review Prompt

For every change ask: which invariant, identity, transaction, retry, concurrency, compatibility, lifecycle, and operational boundary could this code violate? Then prove the dangerous paths with the smallest tests that can.
