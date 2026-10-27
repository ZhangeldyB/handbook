# Scheduled Job Template

Guide: [Scheduled Jobs](../implementation/scheduled-jobs.md).

`internal/<context>/adapters/jobs/<job_name>.go`:

```go
type <job>Command interface {
	Execute(ctx context.Context) (usecases.<Job>Result, error)
}

type <Job>Config struct {
	Interval time.Duration // from the freshness SLO
	Timeout  time.Duration // from the workload; independent of Interval
}

func (c <Job>Config) Validate() error {
	if c.Interval <= 0 || c.Timeout <= 0 {
		return errors.New("<job> interval and timeout must be positive")
	}
	return nil
}

type <Job> struct {
	cmd    <job>Command
	logger *slog.Logger
	cfg    <Job>Config
}

func New<Job>(cmd <job>Command, logger *slog.Logger, cfg <Job>Config) (*<Job>, error) {
	if err := errors.Join(deps.Require("Command", cmd), deps.Require("Logger", logger), cfg.Validate()); err != nil {
		return nil, fmt.Errorf("new <job>: %w", err)
	}
	return &<Job>{cmd: cmd, logger: logger, cfg: cfg}, nil
}

// Run executes once with its own deadline. Tests call it directly.
func (j *<Job>) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, j.cfg.Timeout)
	defer cancel()
	result, err := j.cmd.Execute(ctx)
	if err != nil {
		return fmt.Errorf("<job>: %w", err)
	}
	j.logger.InfoContext(ctx, "<job> finished", "processed", result.Processed, "failed", result.Failed)
	return nil
}

// Loop runs until ctx is canceled; started once by the lifecycle group.
func (j *<Job>) Loop(ctx context.Context) error {
	ticker := time.NewTicker(j.cfg.Interval)
	defer ticker.Stop()
	for {
		if err := j.Run(ctx); err != nil && ctx.Err() == nil {
			j.logger.ErrorContext(ctx, "scheduled job failed", "job", "<job>", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
```

The command claims items durably (owner + lease), processes each with a stable operation ID, and returns honest counts. Tests: config validation, `Run` with a fake command, cancellation, overlapping runs and lease expiry in the command's tests.
