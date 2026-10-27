# Templates

Copy-source scaffolds. Replace `<module>`, `<context>`, `<Entity>`, and `<entity>` and adapt to the repository's domain language. The working, tested version of every Go template lives in [`examples/orders`](../examples/orders/README.md) — when in doubt, copy from there.

## Go Code

| Template | Produces |
|---|---|
| [domain.md](domain.md) | Aggregate with tracker, constructors, snapshot, transitions |
| [state-machine.md](state-machine.md) | Transition table and tests |
| [ports.md](ports.md) | Repository, clock, ID, and provider ports with port errors |
| [usecase.md](usecase.md) | Command with `Deps`, plus create-with-idempotency variant |
| [usecase-test.md](usecase-test.md) | Usecase tests with an all-or-nothing fake |
| [query.md](query.md) | Read-only query with keyset pagination |
| [repository.md](repository.md) | `Get`/`Save` with version check, outbox, idempotency |
| [grpc-handler.md](grpc-handler.md) | Handler with validation, principal, mapping |
| [event-consumer.md](event-consumer.md) | Consumer handler with outcome classification |
| [scheduled-job.md](scheduled-job.md) | Job adapter calling one command |
| [adapters.md](adapters.md) | Outbound provider adapter |
| [composition-root.md](composition-root.md) | `main`/`run` with config, wiring, lifecycle, shutdown |

## Configuration Files

| File | Copy to |
|---|---|
| [golangci.yml](golangci.yml) | `.golangci.yml` (replace `example.com/orders`) |
| [buf.yaml](buf.yaml) | `buf.yaml` |
| [buf.gen.yaml](buf.gen.yaml) | `buf.gen.yaml` (replace `<module>`) |
| [Makefile.service](Makefile.service) | `Makefile` |
| [Dockerfile](Dockerfile) | `Dockerfile` |

## Principles

- Templates show the supported path only. If a template and a rule disagree, the rule wins — fix the template.
- Do not paste templates mechanically: an aggregate without an invariant or a port without a seam is noise.
