# Configuration

Configuration enters the process once, at the composition root, is validated before anything starts, and reaches components as typed values. Nothing below `cmd/` reads environment variables or files directly.

## Load Once, Validate Fully

```go
type Config struct {
	GRPCAddr      string
	DatabaseURL   string // secret
	LogLevel      slog.Level
	Shutdown      ShutdownConfig
	Relay         outbox.Config
}

// Load reads configuration from getenv so tests can supply a map instead of
// mutating the process environment.
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	cfg := Config{
		GRPCAddr:    valueOr(getenv("GRPC_ADDR"), ":8080"),
		DatabaseURL: getenv("DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	timeout, err := durationOr(getenv("OUTBOX_PUBLISH_TIMEOUT"), 5*time.Second)
	if err != nil {
		errs = append(errs, fmt.Errorf("OUTBOX_PUBLISH_TIMEOUT: %w", err))
	}
	cfg.Relay.PublishTimeout = timeout
	// ...
	if err := cfg.Relay.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("outbox relay: %w", err))
	}
	return cfg, errors.Join(errs...)
}
```

- Report **every** invalid setting at once with `errors.Join`, then refuse to start.
- Validate semantics, not just presence: ranges, durations, cross-field rules (`Lease > PublishTimeout`, shutdown total below the platform grace period).
- Defaults live in one place and are documented. A missing optional value uses the default; an invalid value is an error, never silently replaced.
- Use `time.Duration` in Go and put units in variable names at the boundary (`OUTBOX_PUBLISH_TIMEOUT` parsed with `time.ParseDuration`, or `..._SECONDS` as an integer).
- Pass `getenv` (or a lookup function) instead of calling `os.Getenv` inside `Load`, and test it with a map. In tests that must set real variables, use `t.Setenv`.
- A small hand-written loader is usually enough; a library is fine if it supports validation and does not leak into inner packages.

## Secrets

- Secrets come from the platform's secret mechanism (mounted files or environment injected by the orchestrator), never from the repository.
- Never log configuration wholesale. If you log effective config at startup, redact secret fields explicitly.
- Rotate without redeploying where possible (re-read mounted files, short-lived credentials).

## Feature Flags

Flags are temporary lifecycle objects with an owner, a scope, an explicit default per environment and for missing configuration, tests for both states (including mixed-version replicas), telemetry proving correctness, a rollback statement, and a deletion condition. A flag must not become a permanent second architecture. See [API Contract Evolution](../../guides/api-contract-evolution.md#feature-flags-and-rollout-controls).

## Rules

- No package-level mutable configuration. Constants are for true invariants and documented defaults.
- Components receive only the settings they use (`outbox.Config`, not the whole `Config`).
- Configuration changes are reviewed like code; deployment manifests and the loader stay in sync.
