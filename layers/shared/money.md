# Money

Money is a domain value with an amount, a currency, a scale, and rounding rules. Never use `float32`/`float64` for it ([ADR-007](../../decisions/007-exact-money.md)).

## Representation

The default is `int64` **minor units** plus an ISO 4217 currency ([implementation](../../examples/orders/internal/orders/domain/money.go)):

```go
type Money struct {
	minor    int64
	currency Currency
}

func NewMoney(minor int64, currency Currency) (Money, error)

func (m Money) Add(other Money) (Money, error)        // ErrCurrencyMismatch, ErrAmountOverflow
func (m Money) Times(quantity int64) (Money, error)   // ErrAmountOverflow
```

- Operations on different currencies fail; nothing assumes one global currency.
- Arithmetic checks for overflow.
- The value is immutable; methods return new values.
- Construct from minor units or parse decimal strings — never pass through a float.

When you need more precision than minor units (unit prices with fractional cents, FX rates), pick one exact decimal type for the repository, record it in an ADR, and keep rates separate from money.

## Scale

Most currencies have two minor-unit digits, but not all (JPY has zero; some have three). If the system handles several currencies, keep a scale table and use it for formatting and parsing; arithmetic stays in minor units.

## Rounding and Allocation

- Round only at named business points (tax calculation, currency conversion) with an explicit rounding mode, never implicitly.
- Splitting an amount (installments, cost allocation) distributes the remainder deterministically so the parts always sum to the total. Test that property.

## Boundaries

| Boundary | Representation |
|---|---|
| Domain | `Money` value |
| Database | Integer minor-unit column (or exact `NUMERIC`) plus a currency column |
| Protobuf | `google.type.Money`, or `int64 amount_minor` plus `string currency_code` |
| JSON events | Integer minor units plus currency, or a decimal string — never a JSON number with a fraction |
| Providers | Validate the provider's precision and range before converting |

## Tests

Parsing and formatting round trips, currency mismatch, negative and zero amounts, overflow at the limits, allocation remainders, every rounding mode in use, and storage and wire conversions. Property tests suit invariants like "allocated parts sum to the original".
