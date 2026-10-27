# Schema Migrations

The database schema is a contract shared by every running version of the service. Migrations change it without breaking the versions that overlap during a rolling deploy.

## Tooling

- Versioned, forward-only migration files under `migrations/`, applied by one migration tool chosen per repository (and recorded in an ADR). Each file has a monotonically increasing version and a descriptive name: `0007_add_orders_cancel_reason.sql`.
- Migrations run as a separate, explicit deployment step (a job or init container) — not implicitly from every replica at startup.
- Applied migrations are never edited. Fix forward with a new migration.
- CI applies all migrations to an empty database and runs the integration tests against the result.

## Expand, Migrate, Contract

Old and new application versions run side by side during a deploy, and a rollback reverts the code but not the schema. Every change is split so each step is compatible with both neighbors:

1. **Expand** — add the new column, table, or index in a backward-compatible way (nullable or with a default; no renames, no type changes in place). Old code ignores it.
2. **Deploy code that writes both and reads tolerantly** — new code writes the new shape and still accepts the old one.
3. **Migrate data** — backfill in bounded, idempotent, restartable batches with progress metrics. Never one giant transaction over a large table.
4. **Switch reads** to the new shape once the backfill is verified.
5. **Contract** — after the rollback window closes, stop writing the old shape, then drop it in a later migration.

| Change | Safe sequence |
|---|---|
| Add a column | Add nullable/defaulted → deploy code that writes it → backfill → (optionally) add `NOT NULL` |
| Rename a column | Add new → dual-write → backfill → switch reads → stop writing old → drop old |
| Change a type | Same as rename, with an explicit conversion and tests for the edge values |
| Add a unique constraint | Deduplicate data first; build the index without blocking writes if the database supports it; then add the constraint |
| Drop a column | Remove all reads → remove all writes → wait out the rollback window → drop |
| Add an index on a large table | Use the database's online/concurrent index build |

## Rules

- Every tenant-scoped table has `tenant_id` as the leading column of its primary key and of every index used by tenant queries.
- Aggregate tables have an integer `version` column for optimistic concurrency.
- The outbox and idempotency tables live in the same database as the aggregates they serve.
- Money columns are integer minor units or exact `NUMERIC` plus a currency column. Timestamps are stored in UTC.
- Long-running locks, table rewrites, and full scans need an explicit maintenance plan.
- Retention policies never delete unfinished outbox, claim, or operation records.

## Repository Code With a Migration

A new persisted field touches, in one change: the migration, the row type, `toRow`/`toSnapshot`, the `Snapshot`, the round-trip test fixture, and any read model. Reading code must tolerate rows written before the backfill (absent or default values).

## Verification

- Migrations apply cleanly to an empty database and to a copy of the previous schema with representative data.
- Old code runs against the new schema (rolling deploy) and new code tolerates old rows.
- Backfills are idempotent: running them twice yields the same result; restarting after a crash continues correctly.

Skill: [add-migration](../../.agents/skills/add-migration/SKILL.md). Contract evolution in general: [API Contract Evolution](../../guides/api-contract-evolution.md).
