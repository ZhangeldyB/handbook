# Observability

Observability answers three questions: what is the service doing, are users harmed, and how does an operator recover. It must not scatter duplicate logs and spans through every layer.

## Ownership

| Layer | Emits |
|---|---|
| Domain | Typed domain events only — no logger, tracer, meter, or context |
| Transport interceptors | Request count, latency, status, panics, unexpected-error logs (once) |
| Repositories and clients | Dependency latency, retries, and classified outcomes (mostly via instrumentation libraries) |
| Relays, consumers, jobs, reconcilers | Run, delivery, backlog, age, attempt, and terminal/unknown-outcome telemetry |
| Usecases | Only meaningful business decisions not already visible at a boundary |

Do not inject a logger or meter merely because a component could have one. Every signal needs an owner and an operational use.

Telemetry supplies evidence; it never drives behavior. Retry, acknowledgement, health, and remediation decisions use typed outcomes and explicit state — never parsed log text or how noisy an error is.

## Logs

Use `log/slog` with structured key-value pairs and stable messages ([Logging](../../implementation/logging.md)):

```go
logger.ErrorContext(ctx, "outbox record dead-lettered",
	"event_id", rec.ID, "topic", rec.Topic, "attempts", rec.Attempts, "reason", rec.LastError)
```

- Log at the boundary that handles or terminates the outcome; inner layers return errors. Never log and return.
- Never log secrets, tokens, authorization headers, raw payloads, or personal data. Store classified reasons, not raw provider responses.
- Expected outcomes (validation failures, not-found) are not errors.
- Do not log every happy-path transition — persist an event when a durable business record is needed.

Severity reflects operational meaning at the logging boundary:

| Level | Meaning |
|---|---|
| Debug | Temporary development detail |
| Info | Expected lifecycle outcome; no action needed |
| Warn | Degraded or user-action-required, contained and classified |
| Error | Unexpected failure, violated invariant, exhausted policy — needs investigation |

Changing a log's severity is never a fix for the behavior it reports.

## Metrics

Start with service outcomes:

- **Requests:** rate, errors, latency (by method and code).
- **Dependencies:** calls, latency, classified failures, retries, timeouts.
- **Async work:** backlog size, **oldest pending age**, claims, attempts, successes, retryable and terminal failures, unknown outcomes.
- **Runtime:** concurrency, queue depth, pool usage, goroutines, memory, shutdown duration.

Labels must be bounded: operation, method, code, dependency, topic, outcome class. Never tenant IDs, entity IDs, request IDs, emails, URLs, or error messages — they explode cardinality. Alert on impact and age (backlog age, error ratio, SLO burn), not on a single failure.

## Traces

Use OpenTelemetry ([Tracing](../../implementation/tracing.md)). Instrumentation libraries (`otelgrpc`, database drivers, HTTP transports) create most spans. Add a manual span only for meaningful work not already covered — a relay batch, a reconciliation cycle, a job run. Use low-cardinality span names, record errors on the span that owns the failed operation, and put identifiers in attributes only where privacy policy allows.

## Choosing the Signal

| Question | Signal |
|---|---|
| Is impact growing, tenant-local, or provider-wide? | Metrics and dashboards |
| What happened in this one attempt? | Structured log with trace correlation |
| Where did this request spend time and fail? | Trace |
| What state transition happened and why? | Domain event or durable transition record |
| How did attempts and decisions evolve over days and releases? | Durable operational record with policy/build version |

Create a durable record (table) only when the question must survive log retention or sampling, drive recovery or replay, explain a user-visible decision, or support audit. Store classified codes and sanitized context.

## Health

- **Startup** true once initialization is complete.
- **Readiness** false before accepting work that cannot finish, and immediately at shutdown.
- **Liveness** reflects whether a restart helps; it does not fail because an optional dependency is degraded.
- Background loops that can die silently (relay, consumer) expose progress so readiness or an alert fires when they stop advancing.

## Testing

- Metric names and labels stay bounded and classify the important outcomes.
- Sensitive values never appear in logs, spans, or metrics.
- Duplicate and retry paths do not report false success.
- Telemetry exporter failure does not fail the business operation.

Investigation workflow: [Evidence-Driven Debugging](../../guides/evidence-driven-debugging.md).
