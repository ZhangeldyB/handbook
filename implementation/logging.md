# Logging

Services log with the standard library's `log/slog`: structured key-value records, a JSON handler in production, and one logger built at the composition root.

## Setup

```go
func NewLogger(cfg LogConfig) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     cfg.Level,           // slog.LevelInfo in production
		AddSource: cfg.AddSource,
	})
	return slog.New(handler).With(
		"service", cfg.ServiceName,
		"version", cfg.BuildVersion,
	)
}
```

- Inject `*slog.Logger` into the components that own a logging boundary (interceptors, relays, consumers, jobs, `main`). Do not inject it everywhere "just in case", and never into the domain.
- Write to stdout; the platform collects it. Do not write log files from the service.
- Correlate with traces by wrapping the handler with one that adds `trace_id`/`span_id` from the context (the OpenTelemetry contrib `otelslog` bridge does this), and always log with the `...Context` methods.

## Writing Records

```go
logger.ErrorContext(ctx, "outbox record dead-lettered",
	"event_id", rec.ID,
	"topic", rec.Topic,
	"attempts", rec.Attempts,
	"reason", rec.LastError,
)
```

- Messages are stable, lowercase, and describe the event (`"outbox record dead-lettered"`), not printf sentences with values inside. Values go in attributes so they can be queried.
- Keys are `snake_case`, consistent across the codebase (`event_id`, `tenant_id`, `order_id`, `attempt`, `error`).
- Use key-value pairs consistently (`sloglint` with `kv-only`); do not mix with `slog.Attr` styles.
- Put the error under `"error"`.
- Use `logger.With(...)` to build a scoped logger for a job run or a consumer, instead of repeating identity fields.

## What Never to Log

Passwords, tokens, API keys, authorization headers, session cookies, full request or response bodies, payment data, and personal data beyond what policy allows. If a value might be sensitive, log a classification or a hash, not the value. Implement `slog.LogValuer` on types that carry secrets so they redact themselves:

```go
type Credentials struct{ User, Password string }

func (c Credentials) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", c.User), slog.String("password", "REDACTED"))
}
```

## Where and When

- Log once at the boundary that handles the outcome; inner layers return errors.
- Expected outcomes (validation failures, not-found, duplicate deliveries) are not errors — count them with metrics, log at debug or info if at all.
- Context cancellation during shutdown is expected; do not log it as an error.
- Do not log every happy-path step. Lifecycle milestones (startup, shutdown phases, config summary without secrets) are useful at info.

Severity guidance: [Observability](../layers/shared/observability.md#logs).

## Testing

Inject a logger writing to a buffer and assert on the decision that was logged, not on prose:

```go
var buf bytes.Buffer
logger := slog.New(slog.NewJSONHandler(&buf, nil))
// ... exercise code ...
if !strings.Contains(buf.String(), `"msg":"unexpected rpc failure"`) { ... }
```

Use `slog.New(slog.DiscardHandler)` when the test does not care about logs. Test that sensitive values do **not** appear in output for code that handles them.
