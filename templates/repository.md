# Repository Template

Working version: [repository.go](../examples/orders/internal/orders/adapters/persistence/repository.go), [rows.go](../examples/orders/internal/orders/adapters/persistence/rows.go), [events.go](../examples/orders/internal/orders/adapters/persistence/events.go), [tests](../examples/orders/internal/orders/adapters/persistence/repository_test.go). Contract: [Repositories](../layers/infrastructure/repositories.md).

## Files

```text
internal/<context>/adapters/persistence/
├── repository.go       # Get, Save, SaveAll, Lookup, reader methods
├── rows.go             # <entity>Row, toRow, toSnapshot
├── events.go           # topics, payload types, toOutboxRecord
└── repository_test.go  # round trip, retention, conflict, duplicate, tenancy
```

## Save Skeleton

```go
func (r *Repository) Save(ctx context.Context, e *domain.<Entity>, records ...ports.IdempotencyRecord) error {
	pending, ack, dirty := e.BeginPersist()
	if !dirty && len(records) == 0 {
		return nil
	}

	// Map outside the transaction.
	var row <entity>Row
	var outboxRecords []outbox.Record
	if dirty {
		row = toRow(e.Snapshot(), pending.NextVersion)
		for _, event := range pending.Events {
			rec, err := toOutboxRecord(event)
			if err != nil {
				return err
			}
			outboxRecords = append(outboxRecords, rec)
		}
	}

	err := r.db.InTx(ctx, func(tx Tx) error {
		if dirty {
			if err := write<Entity>(ctx, tx, row, pending.IsNew, pending.ExpectedVersion); err != nil {
				return err // ports.ErrConcurrentModification on version mismatch
			}
			for _, rec := range outboxRecords {
				if err := outbox.Append(ctx, tx, rec); err != nil {
					return err
				}
			}
		}
		for _, rec := range records {
			if err := insertIdempotency(ctx, tx, e.TenantID(), rec); err != nil {
				return err // ports.ErrDuplicate on an existing key
			}
		}
		return nil
	})
	if err != nil {
		return err // nothing acknowledged
	}
	if dirty {
		ack()
	}
	return nil
}
```

## Round-Trip Test

```go
func TestRowMappingIsComplete(t *testing.T) {
	full := domain.Snapshot{ /* set EVERY field to a non-zero value */ }
	v := reflect.ValueOf(full)
	for i := range v.NumField() {
		if v.Field(i).IsZero() {
			t.Fatalf("fixture leaves Snapshot.%s zero", v.Type().Field(i).Name)
		}
	}
	got, err := toSnapshot(toRow(full, full.Version))
	if err != nil {
		t.Fatalf("toSnapshot() error = %v", err)
	}
	if diff := cmp.Diff(full, got, cmp.AllowUnexported(/* value types */)); diff != "" {
		t.Fatalf("round trip mismatch (-want +got):\n%s", diff)
	}
}
```

## Checklist

- [ ] `var _ ports.<Entity>Repository = (*Repository)(nil)`.
- [ ] Storage errors translated to port errors.
- [ ] One transaction; version check; outbox; idempotency insert; ack after commit.
- [ ] Every field mapped; reads validated through domain constructors.
- [ ] Tenant first in every key and query.
- [ ] Tests: round trip, commit together, failed commit retains work and retry writes once, stale writer, duplicate key, tenant isolation, `SaveAll` atomicity.
