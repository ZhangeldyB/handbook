---
description: Go code quality, API design, errors, concurrency, testing, and safety rules
paths:
  - "**/*.go"
---

# Go Code Quality Rules

## How to Apply These Rules

- Match the repository's declared Go version, checked-in conventions, generated-code policy, and supported platforms. Do not churn safe existing code for style alone; apply improvements to new and materially changed code.
- **must** marks a correctness, security, compatibility, or architecture requirement. **prefer** marks a default that may yield to clearer code or measured evidence. Let `gofumpt`, `goimports`, `go vet`, and `golangci-lint` own mechanical rules; do not repeat them in review comments.
- Source hierarchy and rationale: `.handbook/reference/go-engineering-sources.md`.

## Whole-Change Review

- Before editing, inspect entry points, callers, the authoritative state owner, persistence and side-effect boundaries, public contracts, generated code, configuration, wiring, tests, and the operational path. Never infer the system from one file or one failing test. For substantial work, trace a success path and the most important failure/retry path end to end.
- Before completion, re-read the entire diff in repository context: callers and registrations still valid, every changed invariant has one owner, tests exercise the contract, and no schema, event, configuration, lifecycle, or deployment surface was missed.
- For production diagnosis or a change to retry, acknowledgement, severity, credential state, or integration health, follow `.handbook/guides/evidence-driven-debugging.md`. Classify from typed or protocol evidence, never from error-message substrings or log frequency, and keep an explicit *unknown* class when evidence is incomplete.

## Clarity and Simplicity

- Optimize for clarity first, then simplicity, maintainability, consistency, and only then concision. Prefer straightforward control flow and early returns; a longer function is fine when it is the clearest expression of one cohesive operation.
- Extract code when the new name clarifies intent or creates a useful test or ownership boundary — not to satisfy line, nesting, or complexity counts. Avoid speculative abstractions; generalize after behavior and ownership are understood.
- A little local duplication beats a premature abstraction, but never duplicate security rules, wire mappings, or business invariants that need one authoritative implementation. Look for an existing implementation before adding another.

## Packages, APIs, and Dependencies

- Default to unexported identifiers. Keep exported APIs small, documented, and hard to misuse.
- Define a minimal interface in the consuming package when substitution or an architecture boundary is useful — never a producer-side interface created only for mocks. Accept the behavior you need; return concrete types unless constraining behavior is the intended contract.
- A component receives only the collaborators it uses, through a `<Name>Deps` struct passed by value and validated once in the constructor (`errors.Join(deps.Require("Repo", d.Repo), ...)`). A single self-evident dependency may be a direct parameter. After construction succeeds, request paths **must not** re-check required collaborators for nil.
- Keep optional behavior and tunables (timeouts, retry policy, batch limits) in an explicit `Config`/options value validated for semantics (ranges, cross-field rules), separate from required dependencies. Constants suit true invariants; mutable package-level configuration does not.
- No service locators, broad dependency bags, or observable mutable globals. Return an error for configuration failures; reserve `MustX`/panic for documented programmer wiring errors at startup.
- Make ownership explicit: if a type retains or can mutate caller-owned slices or maps, copy at that boundary or document the transfer. Prefer useful zero values; when construction is required, make misuse fail early.
- Generated files are read-only: change the source, run the repository's generate target, and confirm a second generation is a no-op.

## Naming and Documentation

- Package names are short, lowercase, descriptive — no `util`, `helpers`, `common`, `models`. Avoid stutter: `order.Status`, not `order.OrderStatus`.
- Use `ID`, `HTTP`, `URL`, and other initialisms consistently; getters omit `Get`. Receivers are short, consistent abbreviations, never `this` or `self`.
- Name values by meaning, not representation (`users`, not `usersMap`); name length grows with scope. Use `time.Duration` in Go APIs and put units in names at unitless boundaries (`timeout_seconds` in JSON, proto, SQL, env).
- Enums whose zero value is meaningless reserve zero as invalid/unspecified and reject unknown values.
- Doc comments on exported APIs state the contract: ownership, concurrency, errors callers may inspect, and non-obvious constraints. Comments explain why; they do not narrate code. Full guide: `.handbook/implementation/naming.md`, `.handbook/implementation/godoc.md`.

## Error Handling

- Check every error unless the API documents it as safe to ignore; make intentional ignores visible (`_ = x.Close()` with a reason when non-obvious).
- Add concise operation context when it helps diagnosis: `fmt.Errorf("get order %q: %w", id, err)`. Wrapping with `%w` makes the cause part of your contract — translate or hide infrastructure errors at boundaries callers must not depend on.
- Use sentinel errors only for stable conditions callers classify, typed errors when they need structured data. Document matchable errors.
- Use `errors.Join` for genuinely independent failures (a primary failure plus a cleanup failure, several shutdown failures, per-item batch failures wrapped with item identity). Never aggregate by concatenating `.Error()` strings.
- Handle an error once: return it, translate it, or log it and recover/degrade. Do not log and return the same failure.
- Error strings are lowercase, without trailing punctuation or `failed to` chains. Exported functions return the `error` interface. Use a `(T, bool)` or `(T, error)` result when a sentinel value would be ambiguous.
- Classify with `errors.Is`/`errors.As` (or `errors.AsType`), never by message text.

## Values, Mapping, and Nil

- Treat an exhaustive-mapping failure as a design signal: trace every target field and distinguish a legitimate zero from a placeholder inserted to silence a linter.
- For every public or persisted field define meaning and owner, presence (required, optional, nullable), absent-versus-zero behavior, default and where it is applied, bounds and units, mutability, tenant/security policy, and old/new reader behavior. See `.handbook/guides/api-contract-evolution.md`.
- Nil slices are valid empty values; nil maps are safe to read but must be initialized before writing. Represent absence explicitly (`(T, bool)`, `*T` when nil is meaningful data). Use comma-ok assertions or type switches when the dynamic type can differ.

## Concurrency, Lifecycle, and Process Boundaries

- Prefer synchronous APIs; add concurrency only for a demonstrated latency, throughput, or ownership need. Every goroutine **must** have an owner, a bounded creation rate, a cancellation path, and something that waits for it.
- Bounded fan-out uses `errgroup.WithContext` with `SetLimit(n)` called before scheduling, or `sync.WaitGroup.Go` plus a semaphore. Define partial-result semantics explicitly. Never use fire-and-forget goroutines for correctness-critical work.
- A deliberately detached task (best-effort telemetry, cache warm-up) uses `context.WithoutCancel` or a fresh context **with a timeout**, is bounded, recovers panics with a structured log, and never carries required work.
- No hand-rolled `for { ...; time.Sleep(...) }` retry loops: retries use capped, jittered exponential backoff, honor `ctx.Done()` between attempts, and apply only to classified retryable and idempotent operations. gRPC client retries belong in the client service config, not in adapters.
- Any function that blocks, waits, or does I/O takes `ctx context.Context` first. Pure domain methods never take a context.
- Bound parallelism and queues; choose channel capacity from backpressure semantics (0 or 1 by default; larger buffers need a stated reason). The goroutine that creates a channel normally closes it. Never store a context in a struct.
- Process-local locks do not protect cross-replica invariants — enforce those with database constraints, version checks, or leases.
- Do not copy values containing mutexes. Prefer typed `sync/atomic` only when it is the simplest correct tool.
- Avoid `init` when explicit construction works. Acquire resources next to their cleanup and check meaningful `Close`, `Flush`, and rollback errors without masking the primary failure.
- Panic only for programmer errors or impossible states. Call `os.Exit` only from `main`; keep startup logic in a `run(ctx) error` function so defers and tests work.

## Testing

- Test observable behavior, invariants, and contracts, not private call sequences. A regression test fails before the fix and passes after it. Prefer small hand-written fakes or the real implementation; use strict mocks only when the interaction itself is the contract.
- Choose test levels by risk: unit tests for domain logic, integration tests for real adapters and transactions, E2E through the public transport for a few critical workflows. No universal percentage targets.
- Deterministic clocks, fixed IDs, isolated state, and bounded eventual assertions for async behavior. Never sleep to synchronize or to advance time.
- Helpers that call `t.Fatal`/`t.Error` call `t.Helper()`. Use `t.Cleanup`, `t.Context()`, `t.Setenv`, `t.TempDir`, `t.Chdir`. Never call `t.Fatal` from another goroutine. Use `t.Parallel()` only with proven isolation.
- Compare errors with `errors.Is`/`errors.As` or `status.Code(err)`; never assert on message substrings. Use `go-cmp` (`cmp.Diff`) for complex values and report `got, want`.
- For public or persisted contracts, test absent versus explicit zero/false/empty, defaults, stable validation paths and codes, unknown enum values, complete mapping, round trips, and old/new compatibility.
- Use `testing/synctest` for self-contained concurrent code driven by timers, channels, and contexts — never around real network or process I/O. Run `go test -race` on changed concurrent code and representative integration paths.
- Fuzz parsers, decoders, and other untrusted structured input when useful invariants can be stated. Benchmarks use `for b.Loop()`.

## Performance

- Correctness and clarity first. Optimize only a measured hot path with benchmarks or profiles, record the workload, and compare with `benchstat`.
- Preallocate when the final size is known or evidence shows benefit. Use `map[K]struct{}` for sets and `map[K]bool` when the boolean carries meaning.

## Modern Go

<!-- Last verified: 2026-10-28 against the go1.27.0 toolchain -->
- Use stdlib `slices`, `maps`, `cmp`, `min`/`max`, range-over-int and iterators, `any`, typed atomics, `log/slog`, `math/rand/v2`, and per-iteration loop variables when they make code clearer. Delete `x := x` copies and `golang.org/x/exp` imports.
- `new(expr)`, `errors.AsType`, `wg.Go`, `t.Context()`, `b.Loop()`, and `testing/synctest` are available at the handbook's Go 1.26 minimum.
- The runtime derives `GOMAXPROCS` from container CPU limits; do not add `automaxprocs`. Do not enable experiments or disable runtime defaults without a measured, documented decision.
- Run `go fix` modernizers only as a dedicated, reviewed change with full tests.

## Verification

1. Run formatting and the repository's generate target for changed sources; confirm generated output is clean.
2. Run targeted tests for changed packages, `make lint`, and `go test ./...` when practical; `go test -race` for affected concurrent paths.
3. Run `govulncheck ./...` when dependencies, the toolchain, parsers, authentication, or security-sensitive code change.
4. Repeat the whole-change review, then inspect the final diff for unrelated edits, stale generated files, accidental contract changes, and missing callers, configuration, or wiring.

Never claim a check passed unless it ran successfully. Report exact commands that could not run and the remaining risk.
