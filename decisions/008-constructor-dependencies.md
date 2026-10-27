# ADR-008: Validated Constructor Dependencies

**Status:** Accepted
**Date:** 2026-10-28

## Context

Long positional constructors are easy to call wrongly; DI frameworks and service locators hide the dependency graph; and nil dependencies discovered on a live request path become defensive nil checks sprinkled through business code.

## Decision

- Each component declares a `<Name>Deps` struct containing only the collaborators it calls. Every field is required.
- The constructor validates it once — `errors.Join(deps.Require("Orders", d.Orders), ...)` — and returns an error naming every missing field. `deps.Require` also catches typed nil pointers stored in interfaces.
- After construction, methods rely on that invariant and never re-check required collaborators for nil.
- Optional behavior and tunables live in a separate `Config` value validated for semantics (ranges, cross-field rules).
- A single self-evident dependency may be a positional parameter (still validated).
- `MustNew` is allowed only for documented startup wiring where panicking is the deliberate convention.
- Construction happens in one composition root (`cmd/<binary>`). No DI framework, global registry, or facade passed as a dependency bag.

## Consequences

- **Easier:** call sites are self-documenting; wiring errors fail at startup or in constructor tests.
- **Harder:** adding a required field leaves old call sites compiling until the constructor test or startup catches it — constructor tests are mandatory.

## Enforcement

Constructor tests reject each missing dependency. Review rejects request-path nil checks on required collaborators.
