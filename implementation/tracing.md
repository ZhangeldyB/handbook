# Tracing

Services use [OpenTelemetry for Go](https://opentelemetry.io/docs/languages/go/) directly: the SDK at the composition root, instrumentation libraries for transports and clients, and a few deliberate manual spans. The exporter backend is deployment configuration, not code.

## Providers

```go
func NewTracerProvider(ctx context.Context, cfg TraceConfig) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracegrpc.New(ctx) // endpoint and headers from OTEL_EXPORTER_OTLP_* env
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.BuildVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("build trace resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp, nil
}
```

- Build it once in `run`, shut it down **last** with a bounded context so it can export spans from the other shutdown phases ([Graceful Shutdown](../layers/infrastructure/graceful-shutdown.md)).
- Sampling is configuration. Keep it out of business code.
- Pin the OpenTelemetry module versions together; they are released as a set.

## Automatic Instrumentation

| Boundary | Instrumentation |
|---|---|
| gRPC server | `grpc.StatsHandler(otelgrpc.NewServerHandler())` |
| gRPC client | `grpc.WithStatsHandler(otelgrpc.NewClientHandler())` |
| HTTP client | `otelhttp.NewTransport(http.DefaultTransport)` |
| HTTP server | `otelhttp.NewHandler(mux, "server")` |
| Database | Your driver's OpenTelemetry integration, if it has one |
| Broker | Propagate trace context in message headers on publish; extract it in the consumer |

These create the root and dependency spans. Do not create a second root span where instrumentation already created one.

## Manual Spans

Add a span for meaningful work that is not otherwise visible — a job run, an outbox relay cycle, a reconciliation cycle, a significant multi-step usecase:

```go
var tracer = otel.Tracer("example.com/orders/internal/platform/outbox")

func (r *Relay) RunOnce(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "outbox.relay_cycle")
	defer span.End()

	published, err := r.runOnce(ctx)
	span.SetAttributes(attribute.Int("outbox.published", published))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "relay cycle failed")
	}
	return published, err
}
```

- Span names are stable and low-cardinality: `<component>.<operation>` (`outbox.relay_cycle`, `jobs.expire_drafts`). IDs never go into names.
- Attributes may carry IDs only where the privacy policy permits; never secrets or payloads.
- Record the error on the span that owns the failed operation. `defer span.End()` immediately after `Start`.
- Do not span every helper, getter, or domain method. Domain code has no tracer.
- Background work (jobs, consumers) starts its own root span per run or per message, linked to the producer's context when one was propagated.

## Context Propagation

Spans form a tree only if `ctx` flows. Pass the context through every call; never replace it with `context.Background()`. In fan-out, pass the group's context to each goroutine.

## Troubleshooting

| Symptom | Check |
|---|---|
| Spans missing entirely | Provider initialized before servers start; exporter endpoint configured; provider not shut down early |
| Database or client spans without a parent | The caller dropped `ctx` or used `context.Background()` |
| `unknown_service` | Resource built without `service.name` |
| Broken traces across async hops | Trace context not injected into message headers on publish or not extracted on consume |
| Trace volume too high | Lower the sampling ratio; remove spans around trivial functions |
