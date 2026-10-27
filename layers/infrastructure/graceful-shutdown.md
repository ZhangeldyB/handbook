# Graceful Shutdown

Graceful shutdown is a bounded ownership protocol: stop admitting work, let owned work finish or cancel it, close dependencies after their users stop, flush telemetry, and return from `main` before the platform kills the process.

## Order

```text
first SIGTERM/SIGINT
  → readiness false; gRPC health NOT_SERVING
  → stop intake: no new RPCs, message pulls, outbox claims, or job runs
  → drain in-flight work, bounded by a deadline (force-stop when it expires)
  → close clients and the database after every user has exited
  → flush telemetry last (it reports the failures above)
  → return from run(); main chooses the exit code
second signal → exit immediately
```

Independent servers and workers may drain concurrently, but a dependency stays open until every component that calls it has stopped — never close the database while a handler or consumer can still commit.

## Budget

The deployment's termination grace period (for Kubernetes, `terminationGracePeriodSeconds`, which also covers any `preStop` hook) is the hard limit. Keep the application budget strictly below it:

```go
type ShutdownConfig struct {
	Total       time.Duration // whole shutdown, below the platform grace period
	Drain       time.Duration // in-flight work
	ForceMargin time.Duration // time left for force-stop and cleanup
}

func (c ShutdownConfig) Validate(platformGrace time.Duration) error {
	if c.Total <= 0 || c.Drain <= 0 || c.ForceMargin <= 0 {
		return errors.New("shutdown durations must be positive")
	}
	if c.Drain+c.ForceMargin > c.Total {
		return errors.New("shutdown phases exceed the total budget")
	}
	if c.Total >= platformGrace {
		return fmt.Errorf("shutdown total %s must be below platform grace %s", c.Total, platformGrace)
	}
	return nil
}
```

Derive the numbers from measured request and job durations, client retry behavior, and load-balancer convergence — not from a constant copied between services.

## Signals and Contexts

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}
```

Inside `run`, wait for `ctx.Done()`, call `stop()` so a second signal gets the default (immediate) behavior, then create the shutdown context from a **live** root — the signal context is already canceled:

```go
<-ctx.Done()
stop() // a second signal now terminates the process immediately

shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Total)
defer cancel()
```

Handle the signals your platform actually sends (`SIGTERM` on Kubernetes, `SIGINT` locally). Use exec-form container entrypoints (`ENTRYPOINT ["/app"]`) so the process receives them.

## Bounded gRPC Drain

`GracefulStop` waits for in-flight RPCs indefinitely, so pair it with a forced `Stop`:

```go
func stopGRPC(ctx context.Context, srv *grpc.Server) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.GracefulStop()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		srv.Stop() // cancels remaining RPCs
		<-done
		return fmt.Errorf("graceful gRPC shutdown: %w", ctx.Err())
	}
}
```

Set the health status to `NOT_SERVING` first. Long-lived streams need explicit shutdown semantics (select on the stream context and an owner-provided stop signal).

## Workers, Consumers, Relays, and Jobs

Every background component implements both phases:

- **Stop intake** — no new claims, pulls, or scheduled runs.
- **Drain** — owned in-flight work finishes or is canceled when the phase deadline expires; all goroutines are waited for.

Run them under one lifecycle group so the first failure cancels the rest and `run` returns the first error:

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return serveGRPC(ctx, server, listener) })
g.Go(func() error { return relay.Run(ctx) })
g.Go(func() error { return consumer.Run(ctx) })
if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
	return err
}
```

Subscribers stop pulling before closing their client and leave unfinished messages for redelivery (or nack them) — never acknowledge unprocessed work to shut down faster. Claimed outbox records and job leases expire and are recovered by another replica; process termination must never turn an unknown external outcome into a blind retry.

## Closing Dependencies

```go
func closeAll(ctx context.Context, db *sql.DB, conns []*grpc.ClientConn, tp *sdktrace.TracerProvider) error {
	var errs []error
	for _, c := range conns {
		if err := c.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close client conn: %w", err))
		}
	}
	if err := db.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close database: %w", err))
	}
	if err := tp.Shutdown(ctx); err != nil { // last, so earlier failures are exported
		errs = append(errs, fmt.Errorf("shutdown tracer provider: %w", err))
	}
	return errors.Join(errs...)
}
```

Close in reverse dependency order, keep going after one failure, and report all of them.

## Health Probes

- **Startup**: true once initialization needed for serving is complete.
- **Readiness**: false before accepting work the process cannot finish, and immediately on shutdown.
- **Liveness**: whether a restart would help; it does not fail because an optional dependency is degraded, and it stays true while draining.

## Runtime Notes

- The Go runtime sets `GOMAXPROCS` from the container CPU limit; do not add `automaxprocs` or a manual override without measurement. Set CPU limits deliberately.

## Verification

- New requests are rejected after intake stops; in-flight requests finish while dependencies remain open.
- A stuck RPC triggers the bounded `Stop` fallback.
- Workers stop claiming and every goroutine exits.
- Dependencies close only after their users finish; multiple close errors are all reported.
- The process exits before the platform grace period; a second signal exits immediately.

Run shutdown tests under `-race`. In a real environment, exercise SIGTERM with the deployed probe and load-balancer configuration and keep the phase-duration logs.
