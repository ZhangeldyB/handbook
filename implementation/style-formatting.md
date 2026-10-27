# Style and Formatting

Tools own mechanical style; people review meaning.

## Tools

- `gofumpt` (a stricter `gofmt`) and `goimports` with the module as the local prefix, configured as golangci-lint formatters ([config](../templates/golangci.yml)).
- `golangci-lint run` locally and in CI. Do not argue in review about anything a formatter or linter decides.
- Editors run `goimports` on save; the PostToolUse hook does the same for agent edits.

## Readability Over Cleverness

- Do not compress complex statements into one line. Break long calls and composite literals one argument per line with a trailing comma so the formatter keeps them vertical.
- Prefer early returns and a straight happy path over nested `else` blocks.
- Keep related declarations together; separate logical steps of a function with a blank line, not a comment that restates the code.

Bad:

```go
if err := order.Cancel(c.ids.NewID(), req.Reason, c.clock.Now()); err != nil { c.logger.Error("cancel failed", "error", err); return fmt.Errorf("%w: %v", ErrCancel, err) }
```

Good:

```go
if err := order.Cancel(
	c.ids.NewID(),
	req.Reason,
	c.clock.Now(),
); err != nil {
	return err // the domain error already carries context
}
```

Do not reflow a short, clear call just because it has several arguments.

## Composite Literals

Use field names for structs from other packages and for any struct with more than a couple of fields:

```go
cmd, err := usecases.NewPlace(usecases.PlaceDeps{
	Orders: repo,
	Clock:  clock,
	IDs:    ids,
})
```

## Variable Naming in Practice

- Aggregates are named after the entity: `order`, `shipment` — never `agg`, `a`, `e`.
- Multi-aggregate code gives each its own descriptive name.
- Loop variables use short domain abbreviations when the body is small (`for _, rec := range records`).
- See [Naming](naming.md) for the full rules.

## File Organization

- One primary type per file, named after it; tests beside the code.
- In a domain package: the aggregate, its status/transition table, events, errors, and value objects in separate files when each has substance — do not split every tiny type into its own file.
- Generated code lives in its own directory and is excluded from formatters and linters.
