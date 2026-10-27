# Shared Concerns

Cross-cutting concerns that span layers.

| Document | Covers |
|---|---|
| [Context and Concurrency](context-concurrency.md) | Context rules, bounded fan-out, goroutine ownership, retries |
| [Configuration](configuration.md) | Loading and validating configuration at the composition root |
| [Observability](observability.md) | Logs, metrics, traces, health, and choosing the right signal |
| [Money](money.md) | Exact money values across domain, storage, and wire |

Implementation guides: [Logging with slog](../../implementation/logging.md), [Tracing with OpenTelemetry](../../implementation/tracing.md).
