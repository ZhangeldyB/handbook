# Paired Anti-pattern Examples

Small compiling contrasts for recurring mistakes. Functions ending in `Bad` are intentionally wrong and exist only to make the violated rule concrete; never copy them.

| Package | Pairs |
|---|---|
| `domain` | Context and wall clock in domain methods; leaking internal slices; mutating before validating |
| `persistence` | Draining events before commit; calling a provider inside a transaction; leaking storage errors |
| `transport` | Trusting the tenant from the request; business logic in a handler; log-and-return |
| `general` | Floating-point money; business data in context; classifying errors by text; unowned goroutines |

```bash
go vet ./... && go build ./...
```

Explanations: [Anti-patterns](../../patterns/antipatterns.md). When a standard changes, update or delete the pair immediately.
