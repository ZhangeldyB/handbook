# Scheduled Jobs

Scheduled work becomes a distributed system as soon as a service has more than one replica. A schedule firing does not prove the work ran once, completed, or committed.

## Design Checklist

Before implementing, write down:

1. freshness or backlog SLO, schedule, timezone, jitter, and catch-up behavior;
2. scope: global, per tenant, or partitioned;
3. overlap policy — may two runs coexist? what happens after a lease expires?
4. stable idempotency identity for each unit of work;
5. run, item, provider, lease, and shutdown time budgets (independent of the interval);
6. page, batch, and concurrency limits;
7. failure semantics: fail-fast, continue-and-report, or durable per-item state;
8. retry, terminal, and manual-repair policy;
9. metrics, alerts, and an operator replay procedure.

## Shape

A job is a thin adapter that triggers one application command:

```go
type ExpireDraftsJob struct {
	expire  expireCommand // usecases.ExpireDrafts
	timeout time.Duration
}

func (j *ExpireDraftsJob) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, j.timeout)
	defer cancel()
	return j.expire.Execute(ctx)
}
```

The command owns the workflow and calls repositories; the job contains no business transitions and no storage or provider calls. Tests call `Run(ctx)` directly.

## Scheduling

A minimal in-process scheduler is a ticker loop owned by the lifecycle group:

```go
func Every(ctx context.Context, interval time.Duration, logger *slog.Logger, name string, run func(context.Context) error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := run(ctx); err != nil && ctx.Err() == nil {
			logger.ErrorContext(ctx, "scheduled job failed", "job", name, "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
```

Use a scheduling library or the platform's scheduler (for example a Kubernetes CronJob running a one-shot command) when you need calendar schedules, catch-up, or centralized history. Whichever you use, the correctness rules below still apply.

## Overlap, Locks, and Leases

A global lock ("only one replica runs the scan") reduces duplicate work but is **not** a correctness guarantee — locks expire, processes pause, networks partition. Correctness comes from durable per-item claims:

```text
due items → atomically claim (owner, expiry, version)
          → process with a stable operation ID
          → persist confirmed or unknown outcome
          → expired claims are reclaimed; idempotency makes the overlap safe
```

This is the same technique as the [outbox relay](../patterns/outbox.md#claiming). Effectful jobs follow [Reconciliation](../patterns/reconciliation.md).

## Multi-Tenant Scans

Enumerate all tenants only when the job is genuinely global. Tenant identity scopes every list, claim, idempotency key, and provider call. Page deterministically and checkpoint large scans. Decide explicitly whether one tenant's failure stops the run or produces a durable per-tenant result — never continue silently and report success.

## Time

The interval and the timeout are independent decisions derived from workload evidence. Business cutoffs use an injected clock; be explicit about UTC versus local calendars and daylight-saving behavior.

## Shutdown

Stop starting new runs and claims, cancel or drain active work within the budget, wait for every goroutine, and leave claimed items recoverable after lease expiry.

## Observability

Schedule delay, run duration and outcome, items scanned, claimed, succeeded, and failed, backlog and oldest age, lease contention and expiry, unknown and terminal outcomes. Labels: job name and outcome — never tenant or entity IDs. Alert on age and convergence, not on one failed run.

## Testing

Configuration validation; cutoff boundaries; pagination; tenant isolation; overlapping runs and lease expiry (simulate two replicas); duplicate invocation; partial-failure counts; cancellation and shutdown; `-race` on concurrent paths. For effectful jobs, crash before and after the provider effect and before and after the local outcome commit.

Skill: [add-scheduled-job](../.agents/skills/add-scheduled-job/SKILL.md).
