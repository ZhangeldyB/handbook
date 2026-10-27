# Context and Concurrency

## Context Rules

- `ctx context.Context` is the first parameter of every function that does I/O, blocks, or waits. Pure domain methods never take one.
- Never store a context in a struct (the `containedctx` linter enforces this). Long-lived components receive a context in `Run(ctx)`.
- A context carries cancellation, deadlines, trace data, and the authenticated principal. It never carries business parameters — pass those explicitly.
- Do not replace the caller's context with `context.Background()` on a request path. For work that must outlive the request (finishing an outcome write after the caller left), use `context.WithoutCancel(ctx)` **plus** a timeout.
- Always call the `cancel` returned by `WithTimeout`/`WithCancel` (`defer cancel()`), and in a loop cancel each iteration before the next.
- Preserve `context.Canceled` and `context.DeadlineExceeded` through wrapping so callers can classify them.

## Prefer Synchronous Code

Add concurrency only for a demonstrated latency, throughput, or ownership need. Concurrent code is harder to test, cancel, and reason about.

## Bounded Fan-Out

```go
func (q *Dashboard) Load(ctx context.Context, tenantID string, ids []string) ([]Summary, error) {
	results := make([]Summary, len(ids)) // one slot per input: no shared appends
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8) // from downstream capacity, set before any Go call
	for i, id := range ids {
		g.Go(func() error {
			summary, err := q.reader.Summary(ctx, tenantID, id)
			if err != nil {
				return fmt.Errorf("summary %q: %w", id, err)
			}
			results[i] = summary
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
```

- `errgroup.WithContext` cancels siblings on the first error; `SetLimit` bounds concurrency. Without `SetLimit` the group is unbounded.
- Write results to distinct indices (or protect shared state) and preserve input order when the API promises it.
- Decide partial-result semantics explicitly: fail-fast (above), or collect per-item results.
- For collect-all semantics use `sync.WaitGroup.Go` with a semaphore and `errors.Join` (see the [anti-pattern pair](../../examples/antipatterns/general/general.go)).
- Never mutate one aggregate from several goroutines.

## Goroutine Ownership

Every goroutine has:

1. an **owner** that starts it and waits for it;
2. a **bound** on how many can exist;
3. a **stop signal** (context cancellation);
4. a way to **report** its error or panic.

Never start goroutines in constructors or library functions that return before the goroutine ends without giving the caller a way to stop and wait. Correctness-critical background work is durable (outbox, operation records) or owned by a long-lived component with `Run(ctx) error`.

The only acceptable fire-and-forget shape is for best-effort work (telemetry, cache warm-up): detached context with a timeout, bounded by a semaphore, `recover()` with a structured log, and no required effect.

## Retries

```go
func retry(ctx context.Context, attempts int, base, maxDelay time.Duration, op func(context.Context) error) error {
	delay := base
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err = op(ctx); err == nil || !isRetryable(err) {
			return err
		}
		if attempt == attempts {
			break
		}
		jittered := delay/2 + rand.N(delay/2+1) //nolint:gosec // jitter, not security
		timer := time.NewTimer(jittered)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(err, ctx.Err())
		case <-timer.C:
		}
		delay = min(delay*2, maxDelay)
	}
	return err
}
```

- Retry only classified retryable failures of idempotent operations.
- Capped exponential backoff with jitter; honor `ctx.Done()` between attempts; bounded attempts.
- No `for { ...; time.Sleep() }` loops.
- gRPC client retries belong in the service config ([gRPC Clients](../infrastructure/grpc-clients.md#retries)).
- Pair per-item retry with a global brake (bounded worker concurrency) so a systemic outage becomes a slow trickle, not a storm.

## Channels and Synchronization

- Default channel capacity is 0 or 1; a larger buffer needs a stated capacity and overload policy.
- The goroutine that creates a channel normally closes it.
- Do not copy values containing mutexes. Use `sync/atomic` typed values only when atomic state is the simplest correct tool.
- Process-local locks never protect cross-replica invariants — use database constraints, version checks, or leases.

## Transactions and Concurrency

- Database transaction callbacks may be retried: keep them deterministic and free of external effects.
- Never hold a transaction across a network call.
- A timeout may leave an external outcome unknown; do not treat it as failure. See [Reconciliation](../../patterns/reconciliation.md).

## Testing

- `testing/synctest` for self-contained concurrent code driven by timers, channels, and contexts (fake time, deterministic blocking) — not for real network or process I/O.
- `go test -race` on changed concurrent code and representative integration paths; it only finds races on executed paths.
- Test start, stop, and wait for every owned goroutine; consider `goleak` for packages that own background goroutines.
