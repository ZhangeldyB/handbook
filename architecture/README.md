# Architecture

How services are structured and why.

| Document | What it answers |
|---|---|
| [Overview](overview.md) | Principles, dependency direction, and when to use (or skip) the layered model |
| [Request Flow](request-flow.md) | One command traced from handler to commit and outbox, including every failure path |
| [Service Layout](service-layout.md) | The canonical package tree and what belongs in each directory |
| [Bounded Contexts](bounded-contexts.md) | Ownership, integration choices, context maps, and when to split |

The short version:

```text
transport (gRPC handlers, consumers, jobs)
        │  maps protocol ↔ application types, derives identity
        ▼
application (usecases, queries, ports)
        │  orchestrates one operation, owns authorization
        ▼
domain (aggregates, value objects, events)
           owns invariants; pure, no I/O

adapters (persistence, clients, publishers) implement ports and point inward
```

Architecture decisions are recorded in [decisions/](../decisions/README.md).
