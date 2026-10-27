---
name: add-scheduled-job
description: >-
  Add a scheduled background job with explicit overlap, tenancy, idempotency,
  timeout, lease, retry, shutdown, and observability semantics. Use for
  periodic cleanup, expiry, maintenance, or triggering reconciliation scans.
---

# Add a Scheduled Job

## Read First

1. `.handbook/implementation/scheduled-jobs.md` (design checklist) and `.handbook/templates/scheduled-job.md`.
2. `.handbook/patterns/reconciliation.md` if the job performs or converges external effects.
3. The command the job will call, deployment replica count, and existing jobs.

## Decide the Contract

Write down before coding: schedule and timezone; whether runs may overlap; global, per-tenant, or partitioned scope; run, item, and provider timeouts (independent of the interval); idempotency identity per item; partial-failure policy; claim/lease design; batch and concurrency bounds; shutdown behavior; alerts.

## Steps

1. Implement or reuse the command that does the work (`implement-usecase`). It claims items durably (owner + expiry), processes each with a stable operation ID, and returns honest counts.
2. Create `adapters/jobs/<job>.go` with a validated `Config`, a `Run(ctx)` that applies the timeout and calls the command once, and a `Loop(ctx)` driven by a ticker.
3. Start `Loop` from the lifecycle group in the composition root; validate configuration at startup.
4. Add metrics: run duration and outcome, items processed/failed, backlog and oldest age.

## Guardrails

- A global lock is optional and never replaces per-item claims and idempotency.
- No business transitions or storage calls in the job adapter.
- No detached goroutines; stop claiming before draining on shutdown.
- Never report full success after silently skipping failures.

## Verify

Config validation; `Run` with a fake command (success, failure, timeout); cancellation; in the command's tests: overlapping runs and lease expiry, duplicate invocation, tenant isolation, bounded pagination, honest counts.

## Done

- [ ] Contract decisions written down.
- [ ] Thin job adapter; command owns the work.
- [ ] Durable claims and idempotency; bounded and cancellable.
- [ ] Started and stopped by the lifecycle group; observable.
