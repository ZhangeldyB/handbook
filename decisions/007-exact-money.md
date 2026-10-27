# ADR-007: Represent Money Exactly

**Status:** Accepted
**Date:** 2026-10-28

## Context

Binary floating point cannot represent most decimal fractions (`0.1 + 0.2 != 0.3`). Rounding errors compound across invoices, discounts, and settlements and are expensive to explain and repair.

## Decision

- Money is a domain value type pairing an amount with an ISO 4217 currency; operations on different currencies fail.
- The default representation is `int64` **minor units** (cents). Overflow is checked.
- When sub-minor precision or currency-agnostic scale is required (FX rates, unit prices with fractional cents), use an exact decimal type chosen once per repository and documented; rates are not money.
- Storage uses integer columns or exact `NUMERIC`; wire contracts use `google.type.Money` or `int64` minor units plus `currency_code`. Never JSON or protobuf floating point.
- Rounding happens only at named, business-approved points with an explicit rounding mode.
- `float32`/`float64` never hold money anywhere, including tests.

## Consequences

- **Easier:** exact arithmetic, simple storage, deterministic tests.
- **Harder:** currencies with three minor-unit digits need an explicit scale table; allocation (splitting amounts) needs a remainder strategy.

## Enforcement

Review and money-domain tests (overflow, mismatch, allocation remainders). There is no automated float-money check; do not claim one.
