# Repositories

A repository is a persistence adapter. It maps storage rows to domain state, owns the transaction, and implements the ports usecases consume. It never owns business policy.

The contract below is database-agnostic. The [compiled example](../../examples/orders/internal/orders/adapters/persistence/repository.go) implements it on an in-memory transactional store; a SQL or document-store adapter implements the same steps with its own transaction API.

## Responsibilities

- Tenant-scoped reads and complete row → domain reconstitution.
- Complete domain → row mapping.
- One transaction per `Save` covering state (version-checked), outbox records, and idempotency records.
- Translating storage conditions into port errors.

## Get

```go
func (r *Repository) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	var row orderRow
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		return tx.Get(ordersTable, orderKey(tenantID, orderID), &row)
	})
	if errors.Is(err, memstore.ErrNotFound) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order row: %w", err)
	}
	snapshot, err := toSnapshot(row) // validates enums, money, lines
	if err != nil {
		return nil, err
	}
	return domain.Reconstitute(snapshot)
}
```

The tenant is part of the key, so another tenant's row with the same ID is invisible. Storage "no rows" becomes `ports.ErrNotFound`; driver errors never leak upward.

## Save

```go
func (r *Repository) Save(ctx context.Context, order *domain.Order, records ...ports.IdempotencyRecord) error {
	pending, ack, dirty := order.BeginPersist()
	if !dirty && len(records) == 0 {
		return nil
	}

	// 1. Map everything before opening the transaction.
	var row orderRow
	var outboxRecords []outbox.Record
	if dirty {
		total, err := order.Total()
		if err != nil {
			return err
		}
		row = toRow(order.Snapshot(), total, pending.NextVersion)
		for _, event := range pending.Events {
			rec, err := toOutboxRecord(event)
			if err != nil {
				return err
			}
			outboxRecords = append(outboxRecords, rec)
		}
	}

	// 2. One transaction: state + outbox + idempotency.
	err := r.db.InTx(ctx, func(tx *memstore.Tx) error {
		if dirty {
			if err := writeOrder(tx, row, pending.IsNew, pending.ExpectedVersion); err != nil {
				return err
			}
			for _, rec := range outboxRecords {
				if err := outbox.Append(tx, rec); err != nil {
					return err
				}
			}
		}
		for _, rec := range records {
			if err := insertIdempotency(tx, order.TenantID(), rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err // 3a. failure: pending work retained for a retry
	}
	if dirty {
		ack() // 3b. success: acknowledge exactly what was written
	}
	return nil
}
```

| Step | Rule |
|---|---|
| Begin | Capture pending state and events; skip when nothing is pending |
| Map | Build the row and outbox records outside the transaction; a mapping bug fails before storage is touched |
| Write state | Insert when new; otherwise compare-and-set on the stored version ([ADR-005](../../decisions/005-optimistic-concurrency.md)) |
| Outbox | Append every pending event in the same transaction ([Outbox](../../patterns/outbox.md)) |
| Idempotency | Insert — never upsert — every record; a duplicate key fails the whole transaction with `ports.ErrDuplicate` |
| Acknowledge | Only after commit ([ADR-004](../../decisions/004-acknowledge-after-commit.md)) |

## The Same Contract in SQL

A `database/sql` sketch of the version-checked write. Placeholders are dialect-specific, and `LinesJSON` stands for the encoded line items (or write them to a child table in the same transaction):

```go
func (r *Repository) writeOrder(ctx context.Context, tx *sql.Tx, row orderRow, isNew bool, expected int64) error {
	if isNew {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO orders (tenant_id, order_id, customer_id, currency, status, lines, total_minor,
			                    created_at, placed_at, payment_id, cancel_reason, version)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			row.TenantID, row.OrderID, row.CustomerID, row.Currency, row.Status, row.LinesJSON,
			row.TotalMinor, row.CreatedAt, row.PlacedAt, row.PaymentID, row.CancelReason, row.Version)
		if isUniqueViolation(err) { // driver-specific error code check
			return fmt.Errorf("insert order %q: %w", row.OrderID, ports.ErrConcurrentModification)
		}
		return err
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE orders SET status = $3, lines = $4, total_minor = $5, placed_at = $6,
		                  payment_id = $7, cancel_reason = $8, version = $9
		WHERE tenant_id = $1 AND order_id = $2 AND version = $10`,
		row.TenantID, row.OrderID, row.Status, row.LinesJSON, row.TotalMinor, row.PlacedAt,
		row.PaymentID, row.CancelReason, row.Version, expected)
	if err != nil {
		return fmt.Errorf("update order %q: %w", row.OrderID, err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("update order %q: %w", row.OrderID, err)
	} else if n == 0 {
		return fmt.Errorf("update order %q at version %d: %w", row.OrderID, expected, ports.ErrConcurrentModification)
	}
	return nil
}
```

Run the transaction through one helper that begins, calls the callback, and commits or rolls back, checking the rollback error without masking the primary one. Some drivers and databases retry aborted transactions by re-running the callback, so callbacks must be deterministic and free of external effects.

## Same-Type Batch: SaveAll

```go
// SaveAll begins every order, writes all of them in one transaction, and
// acknowledges all only after the commit.
func (r *Repository) SaveAll(ctx context.Context, orders []*domain.Order) error
```

Calling `Save` in a loop is wrong when the batch must be atomic: earlier orders would be committed and acknowledged even if a later one fails. See the [implementation and test](../../examples/orders/internal/orders/adapters/persistence/repository_test.go). Bound batch sizes from measured database limits.

## Mapping Completeness

- `toRow(Snapshot)` and `toSnapshot(row)` are the only mapping functions; every field goes through them.
- A round-trip test builds a `Snapshot` with every field set — checked by reflecting over the struct — so adding a field without mapping it fails CI:

```go
v := reflect.ValueOf(full)
for i := range v.NumField() {
	if v.Field(i).IsZero() {
		t.Fatalf("fixture leaves Snapshot.%s zero; set it so the round trip covers it", v.Type().Field(i).Name)
	}
}
```

- Validate on read: parse enums (reject unknown), rebuild money and value objects through their constructors.
- Store enums as stable strings, money as integer minor units (or exact `NUMERIC`) plus currency, times as UTC timestamps, absent values as `NULL` — not as `""` or `0` unless that is the documented meaning.
- Denormalized read columns (`total_minor`) are written in the same transaction as the source state.

## Reads for Queries

The same adapter may implement the query reader port. Use index-backed keyset queries, select only needed columns on hot paths, bound every result, close rows and check `rows.Err()`, and never return rows to handlers.

## Transactions and Resources

- Never hold a transaction across a call to another service or provider.
- Use the database's isolation level deliberately; document it where an invariant depends on it.
- Close rows, statements, and cursors on every path.
- Every key, query, unique constraint, and index that needs isolation starts with `tenant_id`.
- Parameterize every value; allowlist dynamic columns.

## Fakes

Application tests use small fakes of the port, not of the database. A fake must be all-or-nothing like the real adapter — it validates everything before changing any map — otherwise it hides partial-write bugs. Persistence correctness (constraints, isolation, query plans) is proven by integration tests against the real database, typically in a container started by the test.

## Tests

- Round-trip mapping completeness.
- Insert versus update routing; clean aggregates are a no-op.
- State and outbox commit together; a failed commit retains pending work and the retry writes the outbox exactly once.
- Stale writer → `ErrConcurrentModification`.
- Duplicate idempotency key → `ErrDuplicate` and nothing committed.
- Tenant isolation for `Get` and lists.
- `SaveAll` acknowledges nothing when the batch fails.
- Integration: constraints, isolation, and query plans on the real database.

## Review Checklist

- [ ] The port exposes only `Get`/`Save` (and `SaveAll` where needed).
- [ ] One transaction per save; acknowledgement only after commit.
- [ ] Version check on every update.
- [ ] Every field mapped both ways and covered by the round-trip test.
- [ ] Storage errors translated to port errors.
- [ ] No external call inside a transaction.
- [ ] Tenant in every key and query.
