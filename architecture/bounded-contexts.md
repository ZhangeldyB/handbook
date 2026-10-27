# Bounded Contexts

A bounded context is the boundary within which a model and its language are authoritative. It is a business-ownership boundary — not a directory convention and not automatically a separate microservice.

## Separate the Concepts

- A service may host one or several cohesive bounded contexts.
- A bounded context contains one or more aggregates.
- An aggregate is a transaction and invariant boundary, not the whole context.
- A foreign key, a parent-prefixed key, or a table hierarchy does not define aggregate ownership. Anything independently identified with its own lifecycle is a separate aggregate referenced by ID.

## Ownership

Every authoritative fact has one owner. Other contexts consume a contract, keep an explicitly non-authoritative projection, or ask the owner.

Never:

- import another context's `internal` packages;
- share a mutable aggregate as a library;
- read or write another context's tables as an undocumented API;
- pretend a multi-store write is atomic.

Share stable contract definitions (protobuf, event schemas) and domain-neutral primitives (money, IDs) only when they have an explicit owner and release policy. The same word may deliberately mean different things in different contexts — "customer" in billing is not "customer" in shipping.

## Integration Choices

Pick the boundary from consistency, availability, latency, and recovery needs.

| Need | Integration | Contract |
|---|---|---|
| React asynchronously to a fact | Published event | Versioned topic + payload; producer writes state and outbox atomically; consumer is idempotent |
| A current answer before continuing | Synchronous API call | Deadline, authorization, error translation, retry only if idempotent |
| High-volume reads that tolerate staleness | Local projection fed by events | Freshness promise, rebuild procedure, never authoritative for writes |
| Several owners' data in one response | Composition (BFF/gateway) | Partial-failure, deadline, and fan-out limits; the result is a DTO, not an aggregate |
| A provider call whose outcome matters | Durable external-effect workflow | Intent + idempotency key + confirmed/unknown outcomes + reconciliation |

Cross-context **writes** always go through the owner's API or the owner consuming an event. See [Cross-Context Reads](../patterns/cross-context-reads.md).

## Anti-Corruption Adapter

Translate a foreign contract into the consuming context's language at the adapter boundary:

```go
// ports/ports.go — the consumer's language.
type CustomerDirectory interface {
	CreditLimit(ctx context.Context, tenantID, customerID string) (domain.Money, error)
}

// adapters/clients/customers.go — the foreign protocol stays here.
type Customers struct {
	client customersv1.CustomerServiceClient
}

func (c *Customers) CreditLimit(ctx context.Context, tenantID, customerID string) (domain.Money, error) {
	resp, err := c.client.GetCustomer(ctx, &customersv1.GetCustomerRequest{CustomerId: customerID})
	if status.Code(err) == codes.NotFound {
		return domain.Money{}, ports.ErrCustomerNotFound
	}
	if err != nil {
		return domain.Money{}, fmt.Errorf("get customer %q: %w", customerID, err)
	}
	return toMoney(resp.GetCustomer().GetCreditLimit())
}
```

The adapter keeps SDK and wire types out of the domain. It does not make the call transactionally safe; correctness-critical effects still need durable intent and reconciliation.

## Context Map

Keep a context map per system (in the system's own repository, from deployed evidence — not invented). For each relationship record:

| Field | Question |
|---|---|
| Owner / consumer | Which context is authoritative, and who depends on it? |
| Contract | Exact RPC, topic and payload version, or projection |
| Identity | Which trusted tenant/actor keys scope every operation? |
| Consistency | Immediate, eventual (with what lag), or explicitly stale? |
| Failure model | Timeout, duplicate, reordering, partial or unknown outcome? |
| Recovery | Retry, idempotency, reconciliation, replay, rebuild, manual repair? |
| Compatibility | How do old and new versions overlap during rollout? |
| Observability | Which SLO, lag, backlog, and failure signals prove it is healthy? |
| Removal condition | For a transitional integration, what retires it? |

A diagram without failure and recovery semantics is incomplete.

## When to Split

Split a context (or extract a service) when language, invariant ownership, data lifecycle, team ownership, scaling profile, security boundary, or change cadence genuinely differs. Keep behavior together when separation would create chatty calls or distributed invariants.

Do not use "one entity, one service". Start with a cohesive modular service — contexts already isolated as described in [Service Layout](service-layout.md) — and extract when runtime and organizational evidence support it.

## Moving Ownership

Moving a data or contract owner is a production migration: document current and target ownership; version the contract and data transformation with rollback behavior; backfill with repeatable, tenant-safe tooling; handle old/new producer and consumer overlap; cut reads and writes to one owner; reconcile discrepancies; and remove the transitional path after the rollback window. Never leave permanent dual writes.

## Review Checklist

- [ ] Every invariant and authoritative fact has one named owner.
- [ ] Every independently identified mutable object is its own aggregate.
- [ ] Cross-context types are explicit contracts, not imported internals.
- [ ] Synchronous and asynchronous boundaries define timeout, replay, and unknown-outcome behavior.
- [ ] Tenant identity survives every mapping and projection.
- [ ] Projections state their freshness and rebuild procedure.
- [ ] Transitional compatibility has a removal condition.
