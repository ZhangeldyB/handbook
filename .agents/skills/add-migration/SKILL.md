---
name: add-migration
description: >-
  Add a database schema migration that stays compatible with the rolling
  deploy: expand/migrate/contract sequencing, tenant-leading keys, version
  columns, idempotent backfills, and the matching repository mapping changes.
  Use when adding or changing tables, columns, indexes, or constraints.
---

# Add a Migration

## Read First

1. `.handbook/layers/infrastructure/migrations.md`.
2. `.handbook/guides/api-contract-evolution.md` (expand, migrate, contract).
3. The repository's migration tool, existing migrations, and the affected repository mapping.

## Steps

1. Classify the change with the table in `migrations.md` (add column, rename, type change, constraint, drop, index) and plan every deploy step. A change that cannot be done in one compatible step becomes several migrations and deploys.
2. Write the next numbered, forward-only migration file. New aggregate tables lead with `tenant_id` and include `version`; money is integer minor units or exact `NUMERIC` plus currency; timestamps in UTC.
3. Large tables: online index builds, batched idempotent backfills with progress metrics, no long table-locking statements.
4. Update the repository in the same change: row type, `toRow`/`toSnapshot`, the snapshot, and the round-trip fixture. Reads tolerate rows written before the backfill.
5. Never edit an applied migration; fix forward.

## Guardrails

- No renames or type changes in place.
- No drop until all reads and writes are gone and the rollback window has passed.
- Retention never deletes unfinished outbox, inbox, or operation rows.

## Verify

Apply all migrations to an empty database and run integration tests; run the previous release's tests against the new schema (rolling deploy); run backfills twice and after a simulated crash.

## Done

- [ ] Every deploy step is compatible with its neighbors.
- [ ] Mapping, snapshot, and round-trip test updated.
- [ ] Migrations and integration tests pass on an empty database.
