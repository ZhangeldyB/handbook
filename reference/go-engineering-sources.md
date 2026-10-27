# Engineering Sources and Rule Hierarchy

<!-- Last verified: 2026-10-28 -->

The external sources behind the handbook's rules, and how to resolve conflicts between them.

## Resolving Conflicts

Apply guidance in this order:

1. The user-approved behavior, security policy, and public or persisted contracts.
2. The repository's `go.mod`, accepted ADRs, generated-code rules, lint configuration, build targets, and local conventions.
3. This handbook's architecture rules, for the repository types they apply to.
4. Official Go language, standard library, security, and tool documentation.
5. The Google Go Style Guide.
6. Project guides (Uber, Kubernetes, and similar) as evidence and defaults, not universal law.

When two respected sources disagree, do not combine their strongest wording. Prefer the rule that protects a concrete invariant with the least mechanism, and record a local decision when consistency matters.

Use **must** only for correctness, security, compatibility, generated-code, or explicit architecture constraints; **prefer** for contextual defaults. Avoid numeric thresholds and benchmark claims unless enforced or measured.

## Go Project

- [Effective Go](https://go.dev/doc/effective_go) — core idioms (written in 2009; not updated for newer language features).
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) — contexts, errors, interfaces, naming, synchronous APIs, goroutine lifetimes.
- [Go Doc Comments](https://go.dev/doc/comment).
- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) — wrapping is an API decision.
- [`context`](https://pkg.go.dev/context) — cancellation propagation and ownership.
- [The Go Memory Model](https://go.dev/ref/mem) and the [race detector](https://go.dev/doc/articles/race_detector).
- [Fuzzing](https://go.dev/doc/security/fuzz/), [`testing/synctest`](https://pkg.go.dev/testing/synctest).
- [Vulnerability management](https://go.dev/doc/security/vuln/) — `govulncheck`.
- [Release history](https://go.dev/doc/devel/release), [toolchains](https://go.dev/doc/toolchain), [modules reference](https://go.dev/ref/mod), [Go 1 compatibility](https://go.dev/doc/go1compat).

## Style Guides

- [Google Go Style Guide](https://google.github.io/styleguide/go/guide), [Decisions](https://google.github.io/styleguide/go/decisions), [Best Practices](https://google.github.io/styleguide/go/best-practices) — priority order: clarity, simplicity, concision, maintainability, consistency; explicitly rejects one-size-fits-all formulas.
- [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) — error ownership, enum zero values, goroutine lifetimes, functional options; performance rules scoped to hot paths.

## Architecture and APIs

- [Kubernetes API conventions](https://github.com/kubernetes/community/blob/main/contributors/devel/sig-architecture/api-conventions.md) — a strong model for presence, defaulting, compatibility, optimistic concurrency, and level-based reconciliation (adopted as principles, not machinery).
- [Protocol Buffers best practices](https://protobuf.dev/best-practices/dos-donts/) and [buf documentation](https://buf.build/docs/).
- [gRPC status codes](https://grpc.io/docs/guides/status-codes/), [deadlines](https://grpc.io/docs/guides/deadlines/), [cancellation](https://grpc.io/docs/guides/cancellation/), [retry](https://grpc.io/docs/guides/retry/), [graceful stop](https://grpc.io/docs/guides/server-graceful-stop/), [error details](https://grpc.io/docs/guides/error/).
- [Google AIP](https://google.aip.dev/) — resource-oriented API design, pagination, and field behavior (a useful reference for naming and pagination).

## Operations

- [Google SRE: Effective Troubleshooting](https://sre.google/sre-book/effective-troubleshooting/) and [Monitoring](https://sre.google/workbook/monitoring/).
- [OpenTelemetry signals](https://opentelemetry.io/docs/concepts/signals/) and [Go instrumentation](https://opentelemetry.io/docs/languages/go/).
- [Prometheus instrumentation practices](https://prometheus.io/docs/practices/instrumentation/) — failure ratios, latency, cardinality.
- [Kubernetes Pod termination](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination).
- [OAuth 2.0 error responses](https://www.rfc-editor.org/rfc/rfc6749#section-5.2).

## Coding-Agent Instructions

- [Claude Code best practices](https://code.claude.com/docs/en/best-practices) and [memory and rules](https://code.claude.com/docs/en/memory) — concise root instructions, specialized rules scoped with `paths`, skills for workflows.
- `AGENTS.md` conventions — the root file is shared by several agents, so it contains no tool-specific model selection.

Instruction files are executable engineering policy, not a textbook. Do not repeat a rule across the root file, rules, skills, and examples unless each has a distinct routing or generation job. A compiling example is not enough: its "good" path must be semantically current.

## Deliberate Local Decisions

These are this handbook's choices, not universal Go style; each has an ADR in [decisions/](../decisions/README.md): pure domain packages; repository-owned transactions with `Save`; acknowledge-after-commit pending work; versioned full-state writes; transactional outbox; exact money; validated `Deps`; gRPC with buf; central error mapping. Libraries, CLIs, and small tools keep idiomatic package-oriented Go instead.

## Rules Intentionally Rejected

- Fixed maximum function length, nesting depth, cyclomatic complexity, or parameter count.
- A fixed number of duplications before extracting.
- Universal test-distribution ratios or coverage percentages.
- Preallocation, `map[K]struct{}`, atomics, or concurrency without a semantic or measured reason.
- Interfaces created only for mocks.
- Blanket bans on `init`, goroutines, `else`, pointers for optional values, or legitimate zero values.

## Maintaining a Rule

1. Classify it: universal Go guidance, handbook architecture, or repository-local policy.
2. Link a primary source or ADR for non-obvious claims; add a "Last verified" line for version-specific claims.
3. State exceptions that materially affect correctness or clarity.
4. Update every template, skill, agent, and example that generates the affected pattern.
5. Search the whole handbook for contradictions (`make verify`).
