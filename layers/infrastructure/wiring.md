# Wiring and Dependency Injection

Services use plain constructor injection, assembled in one composition root: `cmd/<binary>/main.go`. No DI framework, global registry, or service locator.

> A component receives only the collaborators it uses, validates every required dependency once during construction, and may then rely on them for its lifetime.

Invalid wiring fails at startup or in a constructor test — not on a live request — and request paths carry no defensive nil checks.

## Deps Values

```go
type PlaceDeps struct {
	Orders ports.OrderRepository
	Clock  ports.Clock
	IDs    ports.IDGenerator
}

func NewPlace(d PlaceDeps) (*Place, error) {
	if err := errors.Join(
		deps.Require("Orders", d.Orders),
		deps.Require("Clock", d.Clock),
		deps.Require("IDs", d.IDs),
	); err != nil {
		return nil, fmt.Errorf("new place: %w", err)
	}
	return &Place{orders: d.Orders, clock: d.Clock, ids: d.IDs}, nil
}
```

- Name it `<Component>Deps`. Every field is required and is something the component calls.
- Pass it by value and copy the fields into the component; do not retain a caller-owned pointer.
- `deps.Require` ([source](../../examples/orders/internal/platform/deps/deps.go)) reports missing dependencies by name and also catches a typed nil pointer stored in an interface. `errors.Join` reports all of them at once.
- `Require` checks presence only. Validate semantics (ranges, formats, cross-field rules) in a separate `Config` value.
- A single self-evident dependency may be a positional parameter: `queries.New(reader)` — still validated.

## Optional Behavior and Configuration

Keep optional collaborators and tunables out of `Deps`:

```go
type RelayDeps struct {
	DB        *memstore.DB
	Publisher messaging.Publisher
	Logger    *slog.Logger
	Now       func() time.Time
	NewToken  func() string
}

type Config struct {
	BatchSize      int
	PublishTimeout time.Duration
	Lease          time.Duration // must exceed PublishTimeout
	// ...
}

func NewRelay(d RelayDeps, cfg Config) (*Relay, error) // validates both
```

When "do nothing" is legitimate behavior, inject an explicit no-op implementation; never turn a missing required dependency into a silent no-op.

## Error or Panic

Default to `New(...) (*T, error)` and wrap with the component name so startup errors identify the broken edge. Provide `MustNew` only for documented startup wiring where panicking is the deliberate convention; never hide a panic inside an ordinary `New`.

## The Composition Root

```go
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		slog.Error("service failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := telemetry.NewLogger(cfg.Log)
	// 1. infrastructure: database, broker client, telemetry providers
	// 2. adapters: repositories, clients, publishers
	// 3. application: usecases and queries, each with its own Deps
	// 4. transports: gRPC server, consumers, relay, jobs
	// 5. run everything under one lifecycle group; shut down in reverse order
	...
}
```

- Construct infrastructure first, then adapters, operations, and transports. Each edge passes exactly the dependencies the next component declares.
- Handlers and consumers receive the operations they invoke — never the whole application graph.
- Tests use the same constructors with fakes, so invalid fixtures fail immediately.
- Keep one construction path; do not maintain parallel constructors with subtly different defaults.

Lifecycle and shutdown ordering: [Graceful Shutdown](graceful-shutdown.md). Configuration loading: [Configuration](../shared/configuration.md).

## Constructor Tests

```go
func TestConstructorsRejectMissingDependencies(t *testing.T) {
	tests := []struct {
		name string
		new  func() error
	}{
		{name: "place without orders", new: func() error {
			_, err := NewPlace(PlaceDeps{Clock: fixedClock{}, IDs: &sequentialIDs{}})
			return err
		}},
		{name: "mark paid with typed nil IDs", new: func() error {
			var ids *sequentialIDs
			_, err := NewMarkPaid(MarkPaidDeps{Orders: store, Clock: fixedClock{}, IDs: ids})
			return err
		}},
	}
	// ...
}
```

Test each missing dependency once and one valid construction; execution tests then never need nil-dependency cases.

## Review Checklist

- [ ] One composition root; construction through constructors only.
- [ ] Each component receives only what it calls.
- [ ] `Deps` validated once; no request-path nil checks.
- [ ] Optional configuration separate and semantically validated.
- [ ] No service locator, global registry, or facade-as-dependency-bag.
- [ ] Constructor tests cover every required dependency.
