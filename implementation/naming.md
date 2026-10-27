# Naming

A good name is **consistent** (easy to guess), **short** (easy to type), and **accurate** (easy to understand). Synthesized from Effective Go, Go Code Review Comments, and the Google Go Style Guide, plus the project conventions at the end.

## General

- `MixedCaps` for all identifiers; never `snake_case` or `ALL_CAPS`.
- Default to unexported; export only what callers need.
- Do not shadow builtins (`len`, `new`, `clear`) or imported packages (`log`, `url`, `errors`).
- Name by meaning, not type: `users`, not `userSlice`. When two forms of a value coexist, qualify one: `limitRaw` and `limit`.
- Name length grows with scope: `i` in a three-line loop, `defaultRequestTimeout` at package level.

## Packages

- Short, lowercase, one word: `orders`, `outbox`, `grpcerr`. No underscores or mixedCaps.
- Name for what the package **provides**. Never `util`, `common`, `helpers`, `misc`, `base`, `types`, `models`, `interfaces`.
- Avoid stutter — callers write `pkg.Name`: `outbox.Record`, not `outbox.OutboxRecord`; `order.New`, not `order.NewOrder` when the package has one primary type.
- Package names should not steal good variable names (`ctx` exists because `context` is a package).

## Files

- `snake_case.go`: `mark_paid.go`, `payment_captured.go`. Tests `_test.go`; platform suffixes `_linux.go`.
- One operation per file in `usecases/`; the file is named after the operation.

## Variables and Constants

| Scope | Guidance | Examples |
|---|---|---|
| Tiny, conventional | Short is clearest | `i`, `n`, `v`, `k`, `r` (reader/request), `w` (writer), `b` (bytes) |
| Local with several meanings | Name the role | `count`, `orders`, `buf` |
| Package-level | Specific and searchable | `defaultPageSize`, `maxAttempts` |

Conventional short names: `ctx`, `err`, `req`, `resp`, `cfg`, `db`, `tx` (database transaction), `msg`, `now`.

- `var x T` when the zero value is intentional; `x := ...` when initializing.
- Constants are `MixedCaps` named by role (`MaxPageSize`), never `MAX_PAGE_SIZE` or `kMax...`. Do not create a constant whose only meaning is its literal value.

## Functions and Methods

- Getters have no `Get`: `Status()`, not `GetStatus()`. Setters use `Set`. (Generated protobuf getters are the exception.)
- Functions that return a value read as nouns (`Total()`); functions that act read as verbs (`Place()`, `Publish()`).
- Use `Fetch`, `Load`, or `Compute` to signal expensive work.
- Do not repeat the receiver or package: `(c *Config) WriteTo(w)`, not `WriteConfigTo`.
- Named results only when they document meaning (`(lat, long float64, err error)`); never for naked returns.

## Constructors

- `New` when the package has one primary type (`queries.New`); `New<Type>` otherwise (`outbox.NewRelay`, `usecases.NewPlace`).
- Descriptive names for alternatives: `NewForCreate`, `NewForReconstitute`, `FromSnapshot`.

## Interfaces

- One method: method name + `-er` with correct English (`Publisher`, `Executor`, not `Executer`).
- Several methods: a descriptive noun (`OrderRepository`, `OrderReader`).
- Never suffix `Interface` or prefix `I`. Honor canonical names (`String()`, `Close()`, `Read()`).
- Name by capability, not technology: `PaymentGateway`, not `StripeClient`.

## Receivers

One or two letters, consistent across all methods of the type: `(o *Order)`, `(r *Relay)`, `(c *Place)`. Never `this`, `self`, or `me`.

## Initialisms

All caps or all lowercase, never mixed:

| Exported | Unexported | Wrong |
|---|---|---|
| `ID` | `id` | `Id` |
| `URL` | `url` | `Url` |
| `HTTP` | `http` | `Http` |
| `API` | `api` | `Api` |
| `JSON` | `json` | `Json` |
| `GRPC` | `grpc` | `Grpc` |

So: `orderID`, `tenantID`, `HTTPClient`, `ParseURL`, `userIDs`.

## Booleans

Read naturally in conditions: `isNew`, `hasLines`, `canCancel`, `IsDirty()`. Avoid negatives (`isNotReady`) and meaningless names (`flag`, `ok2`).

## Errors

| Kind | Convention | Example |
|---|---|---|
| Exported sentinel | `Err` + condition | `ErrNotFound`, `ErrInvalidTransition` |
| Unexported sentinel | `err` + condition | `errClosed` |
| Error type | condition + `Error` | `LimitExceededError` |
| Message | lowercase, no trailing punctuation, no `failed to` | `"get order %q: %w"` |

The condition after `Err` never repeats "error". `errname` enforces the shape.

## Context

`ctx context.Context` is the first parameter, named `ctx`. When two contexts coexist, keep `ctx` for the primary and name the other by purpose (`shutdownCtx`, `writeCtx`). Never store a context in a struct.

## Tests

- `TestXxx` with `Xxx` starting uppercase: `TestPlaceRecordsEventWithTotal`. Method tests may use `TestType_Method`.
- Subtests and table cases have descriptive names (`"paid cannot be cancelled"`) — the name is the first line of a failure.
- `got`/`want` for results; failure messages read `Func(args) = got, want want`.
- Black-box tests use `package foo_test`; test helper packages append `test` (`outboxtest`).
- `BenchmarkXxx`, `ExampleXxx`, `FuzzXxx`.

## Import Aliases

Alias only to resolve collisions or uninformative names. Generated protobuf packages get a consistent `<name><version>` alias: `ordersv1 "example.com/orders/gen/acme/orders/v1"`. Use the same alias for the same package everywhere.

## Project Conventions

### Aggregates and Domain

- Aggregate variables use the entity name, never `agg`: `order, err := c.orders.Get(...)`.
- Events are past-tense facts: `OrderPlaced`, `PaymentCaptured`. Commands are imperative: `Place`, `Cancel`.
- Statuses are `Status` constants prefixed with `Status`: `StatusDraft`, `StatusPlaced`.
- Domain errors describe the violated rule: `ErrInvalidTransition`, `ErrEmptyOrder`.

### Application

- Usecase types are named by operation (`Place`, `CreateDraft`, `MarkPaid`) with an `Execute(ctx, <Op>Request)` method and receiver `c`.
- Requests and results: `<Op>Request`, `<Op>Result`.
- Dependencies: `<Op>Deps` with fields named by role (`Orders`, `Clock`, `IDs`).
- Ports by capability: `OrderRepository`, `OrderReader`, `Clock`, `IDGenerator`, `PaymentGateway`.

### Persistence and Messaging

- Repository methods: `Get`, `Save`, `SaveAll`; readers: `GetOrder`, `ListOrders`.
- Row types are unexported and suffixed `Row`: `orderRow`, `lineRow`. Mapping functions: `toRow`, `toSnapshot`, `toView`.
- Topics: `<context>.<event_snake_case>.v<N>` (`orders.order_placed.v1`); constants `Topic<Event>`.
- Idempotency scopes: the operation name (`CreateDraft`) or `consume:<topic>`.

### Abbreviations

| Name | Meaning |
|---|---|
| `ctx` | `context.Context` |
| `req` | request value |
| `tx` | database transaction |
| `rec` | outbox or idempotency record |
| `msg` | broker message |
| `now` | the operation's captured time |
| `c` | usecase receiver |
| `r` | repository or relay receiver |

## Sources

[Effective Go](https://go.dev/doc/effective_go#names) · [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) · [Google Go Style: Naming](https://google.github.io/styleguide/go/decisions#naming) · [What's in a Name](https://go.dev/talks/2014/names.slide)
