# API Contract Design and Evolution

Public APIs, events, persisted schemas, configuration, and generated models are long-lived contracts. Design them so callers can express intent unambiguously, old and new deployments can overlap safely, and failures stay actionable. Internal package layout is not a contract.

## Start With the Contract Map

Before changing a field, RPC, event, or schema, identify:

- the authoritative owner and the business meaning;
- every producer, consumer, mapper, table, index, cache, and generated artifact;
- create, read, update, retry, replay, and delete behavior;
- the oldest producer and consumer that may overlap during rollout;
- whether rollback stays possible after data is written in the new form;
- the evidence that will prove the rollout worked.

Trace one success path and one invalid, absent, stale, or retry path end to end. Never infer the contract from a Go type or one handler.

## Specify Every Field

| Property | Decision |
|---|---|
| Meaning and owner | What does the value mean; which component is authoritative? |
| Presence | Required, optional, nullable, or computed? Is absent different from zero, false, or empty? |
| Default | Is there one, where is it applied, is the result stored or returned explicitly? |
| Validation | Formats, bounds, combinations, state-dependent rules |
| Units | Encoded by the type or explicit in the name |
| Mutability | Create-only, mutable, immutable, or server-managed; what omission means on update |
| Concurrency | Version or precondition; what a stale update returns |
| Security | Who may read or set it; which tenant owns it; is it sensitive? |
| Evolution | How older readers and writers, and unknown enum values, behave |

Write the semantics in a comment when the schema cannot express them. Do not add comments that merely repeat the field name.

## Presence and Defaults

```protobuf
// Maximum delivery attempts. When absent, the server applies the documented
// default (5). Zero explicitly disables automatic retries.
optional int32 max_attempts = 4;
```

Without presence, nobody can tell whether `0` means "unspecified", "disabled", or "invalid". The same applies to `false`, `""`, and empty lists.

- Defaults apply only to absent values and never overwrite an explicit value.
- Apply defaults at one documented trusted boundary and pass the explicit result into the domain.
- A new default is a behavior change: check stored data, old clients, update semantics, and mixed-version replicas.
- Test absent, explicit zero/false/empty, explicit non-zero, invalid, and old encodings separately.

## Validation Errors

Validate wire syntax at the transport and business invariants in the domain — not the same rule in both places. When clients can fix several fields in one request, return all violations with stable field paths and reasons ([gRPC Handlers](../layers/infrastructure/grpc-handlers.md#structured-validation)):

- Paths, reasons, and status mappings are stable and documented.
- Accumulate independent violations; stop checks that depend on a failed prerequisite or could expose data.
- A rejected request changes nothing.
- Programs classify from reasons and codes, never from human-readable descriptions.
- Never echo credentials, tokens, or sensitive values in error details.

## Expand, Migrate, Contract

1. Add a representation old readers safely ignore and new readers understand.
2. Deploy tolerant readers before new writers when the change requires it.
3. Write or migrate data with measurable, restartable, idempotent progress.
4. Verify every old/new producer-consumer combination and stored-data round trips.
5. Remove the old representation only after the overlap and rollback windows close; reserve removed protobuf numbers and names.

Never change a field's meaning in place — add a new field or version. A feature flag does not make an incompatible schema reversible. Schema specifics: [Migrations](../layers/infrastructure/migrations.md). Protobuf specifics: [Protobuf Contracts](../layers/infrastructure/proto-contracts.md#evolution).

## Generated Artifacts

Generated files are read-only outputs of a source definition and a pinned generator. Use the repository's generate target, run the paired verification (regenerate, fail on diff), and review generated diffs for dropped fields, renamed packages, changed defaults, or toolchain drift. Never hand-edit.

## Feature Flags and Rollout Controls

Use a flag only when staged exposure or fast disablement materially reduces risk. Every flag has:

- an owner and an affected scope (tenants, cohorts);
- an explicit default per environment and for missing configuration;
- tested enabled and disabled behavior, including mixed-version replicas;
- bounded telemetry proving correctness and detecting divergence;
- a rollback statement, especially after writes or external effects;
- a promotion condition and a deletion condition or date.

Prefer shadow comparison for a new pure decision or read path while the old path stays authoritative. Never duplicate a non-idempotent external effect to shadow it. A flag must not become a permanent second architecture.

## Contract Tests

| Risk | Evidence |
|---|---|
| Presence/default ambiguity | Absent versus explicit zero/false/empty fixtures |
| Mapping loss | Complete-mapping and round-trip tests |
| Version skew | Old-writer/new-reader and new-writer/old-reader fixtures |
| Default drift | Create/update/re-read tests with explicit expectations |
| Invalid input | Stable path, reason, and code; no partial write |
| Unknown enum or field | Documented tolerate, preserve, reject, or fail-closed behavior |
| Concurrent update | Version conflict and retry behavior |
| Decoder robustness | Fuzz tests with retained failing inputs |
| Rollout flag | Both states, mixed versions, telemetry, rollback |

Migrations additionally test restart after partial progress, repeated execution, concurrent live writes, and rollback or forward-fix.

## Completion Checkpoint

Before finishing a contract change, reopen the source schema and every changed mapper. Confirm presence, default, unit, and mutability semantics are documented; generated outputs are current; every old/new overlap is safe; tests distinguish presence from zero; `buf breaking` passes; and the diff contains no hand-edited generated file or open-ended compatibility branch.
