# Evidence-Driven Debugging

Use this guide when investigating production behavior, incidents, noisy errors, provider failures, retry storms, or missing events, and before changing log severity, retry, acknowledgement, or integration-health policy. The goal is to establish what the system did, why, who was affected, what recovers automatically, and which decision is safe.

A request to explore, audit, or diagnose does **not** authorize a code, data, provider, or deployment change.

## Non-Negotiable Rules

- The symptom, log line, alert, and named file are starting points, not root causes.
- Separate observed facts, supported inferences, hypotheses, and unknowns.
- When production behavior is the subject, inspect the deployed revision, configuration, and registrations — current source alone cannot prove what ran.
- Never change behavior or severity because of frequency alone. A quiet retry storm is still a retry storm.
- Drive correctness from typed errors, protocol status codes, and structured provider fields — never from mutable human-readable text.
- Classify the operation independently from the integration: a failed attempt is not automatically a broken integration.
- Keep an explicit *unknown* when evidence cannot distinguish success from failure or one cause from another.
- Never log or persist passwords, tokens, authorization headers, private keys, or raw sensitive payloads.
- Prefer the smallest reversible next step. When evidence is weak, instrument and observe before changing behavior.

## The Protocol

1. **Map the runtime path** from trigger through adapter, provider or database, retry or replay, user-visible state, and operator signals. Verify the deployed revision and configuration when available.
2. **Separate facts from inferences.** Test competing hypotheses against code, protocol contracts, tests, logs, metrics, traces, and durable records.
3. **Classify on independent dimensions** (below).
4. **Use typed evidence** and preserve *unknown* where certainty is impossible.
5. **Gate destructive transitions.** Automatic credential clearing or "action required" states need conclusive evidence at the correct protocol stage, and must be idempotent, durable, auditable, concurrency-safe, and recoverable. Never delete history to represent health.
6. **Choose telemetry by the question:** metrics for trends, logs for one event, traces for causal paths, durable records for multi-day lifecycle and audit questions.

## The Investigation Loop

### 1. Define the Symptom and the Invariant

State what was observed, the time window, environment, operation, and known scope. Do not translate an error count into a user count. Write the invariant that may have failed: "every subscribed topic has a registered handler", "a rejected credential stops further login attempts".

### 2. Build the Runtime Map

```text
trigger → RPC/event/schedule → application operation → adapter/client
        → provider or database → outcome classification → persisted state
        → retry/replay/ack → public API → logs/metrics/traces/alerts
```

Inspect callers and registrations, configuration and feature flags, provider stage, timeouts and retries, transactions and idempotency, error mapping, the deployed build, and existing tests and dashboards. Names are not proof: a function called `clearTokens` may not clear anything.

### 3. Keep an Evidence Table

| Hypothesis | Expected evidence | Observed | Missing | Status |
|---|---|---|---|---|
| Credential rejected | Auth-stage error code; no later success with the same credential | ... | ... | confirmed / weakened / unknown |
| Provider outage | Timeouts or 5xx across tenants; provider status | ... | ... | ... |
| Handler missing in deployed build | Startup registry differs from source | ... | ... | ... |

Never mark "confirmed" from a substring, one log line, or absence of evidence. Record the source and time window of each observation.

### 4. Classify Independently

| Dimension | Question |
|---|---|
| Operation outcome | Did this attempt succeed, fail, or end unknown? |
| Retryability | Would the same input plausibly succeed later? Is retry safe and bounded? |
| Integration health | Absent, unverified, healthy, degraded, action-required, or disabled? |
| Scope | One request, tenant, credential, endpoint, provider, release, or everything? |
| Attribution | User configuration, our configuration or code, provider, data, or unknown? |
| Remediation | Retry, reconnect, fix permissions or data, deploy, contact provider, manual review? |
| Confidence | Which typed evidence supports it, and what would disprove it? |

Do not collapse these into `err != nil`, a severity, or one `connected` boolean.

### 5. Choose the Smallest Decision

For every proposal, state why the evidence is sufficient, what state changes and what history remains, how concurrent workers and repeated deliveries behave, how users and operators recover, which signals prove the rollout worked, and the rollback or kill switch. Use shadow classification before enabling a high-blast-radius automatic action.

## Failure and Remediation Matrix

The provider's contract is authoritative; this table prevents common category errors.

| Evidence | Retry default | Integration decision |
|---|---|---|
| Optional configuration absent before any call | None until configured | Not configured — not "bad credentials" |
| Exact authentication rejection at the login/token stage | Stop retrying the same credential | Action required; preserve history |
| OAuth `invalid_grant` | Do not repeat the same grant | Re-authorization needed; the cause is not necessarily revocation |
| HTTP 401 on an authenticated call | Refresh once if the protocol allows, then classify | Possibly expired or wrong audience |
| HTTP 403 | No blind retry | Usually permission, scope, or account state |
| HTTP 400 | No generic retry | Request, data, or protocol error; keep the provider code |
| HTTP 404 | Depends on lifecycle | Usually a stale mapping — classify per operation |
| HTTP 429 | After provider guidance, with jittered backoff | Configured; degraded if sustained |
| 5xx, timeout, DNS, connection reset | Bounded backoff; reconcile non-idempotent unknowns | Degraded, not credential-invalid |
| Subscribed event with no handler | Do not acknowledge silently | Deployment invariant broken; retain the work |
| Business precondition fails | Only after state changes | Integration may be healthy; map to `FailedPrecondition` |
| Non-idempotent call ended unknown | No blind retry | Reconcile by stable operation key |

"Terminal" always names its scope: terminal attempt, payload, credential, workflow, or integration.

## Bad and Better

Bad — message text drives a destructive decision:

```go
if strings.Contains(err.Error(), "invalid_grant") {
	logger.WarnContext(ctx, "token was revoked")
	return deleteIntegration(ctx, tenantID)
}
```

It loses the protocol stage, asserts a cause the protocol does not guarantee, deletes history, and does not show what stops the retries or how the user recovers.

Better — a typed outcome, decided by the owner of health policy:

```go
outcome := tokenClient.Classify(err)
if outcome.Stage == StageRefresh && outcome.Code == CodeInvalidGrant {
	return health.RequireAction(ctx, RequireActionRequest{
		IntegrationID: integrationID,
		Reason:        ReasonGrantRejected,
		Evidence:      outcome.SafeEvidence(),
	})
}
return retryPolicy.Handle(ctx, outcome)
```

Bad — silence a symptom without checking the work lifecycle:

```go
if handler == nil {
	logger.InfoContext(ctx, "no handler; skipping")
	continue
}
```

Better — treat it as a broken invariant, keep the work, and make the build identifiable:

```go
if handler == nil {
	return fmt.Errorf("resolve handler for topic %q in build %q: %w", msg.Topic, buildVersion, ErrHandlerNotRegistered)
}
```

## Integration Health Is More Than a Boolean

Model the facts even if a public API derives a simpler status: configuration (absent, present, incomplete); verification (unverified, verified, rejected — with time and method); availability (healthy, degraded, unavailable, unknown); operator intent (enabled, disabled); required action (none, reconnect, update permissions, contact provider, internal repair). A "test connection" operation states what it proves (handshake, login, safe read) — never report "verified" when credentials were merely saved.

## Durable Operational Records

Use a durable record when operators must answer questions after log retention, across replicas and releases, or for product and audit decisions. Typical shape: a current health projection (state facts, reason code, last success, consecutive failures, next allowed probe, version for compare-and-set) plus an append-only attempt log (operation and attempt IDs, protocol stage, classified code and retryability, from/to state, safe error fingerprint, trace ID, build and policy version, timestamps). Define retention, access, and redaction. It complements metrics and traces; it does not replace them.

## Instrument Before Deciding

An instrumentation-only change preserves runtime behavior, captures the protocol stage and structured outcome before they are flattened, includes build and policy versions, uses bounded and redacted fields, states the observation window and decision threshold, and has an owner and removal plan. Review its cardinality, volume, and privacy like any other change.

## Validate "the Newer Version Fixes It"

Treat it as a hypothesis: reproduce on the current version, inspect the exact candidate release, confirm the fix is actually in the released (and, for generated code, regenerated) artifact, inventory API and transitive changes, add a focused regression test, and recheck runtime telemetry after rollout.

## Investigation Report

Return:

1. conclusion and confidence, including whether any change is justified;
2. impact and known scope;
3. the end-to-end success and failure/retry paths;
4. facts with sources, then inferences and unknowns;
5. the hypothesis table;
6. the classification and decision matrix;
7. security, data, retry, concurrency, and rollout risks;
8. existing telemetry and evidence gaps;
9. the smallest proposed diagnostic or behavior step, clearly separated;
10. the validation needed and remaining uncertainty.

If logs, traces, deployment metadata, or provider documentation were unavailable, say so — never imply they were inspected.

## References

[Google SRE: Effective Troubleshooting](https://sre.google/sre-book/effective-troubleshooting/) · [SRE Workbook: Monitoring](https://sre.google/workbook/monitoring/) · [OpenTelemetry Signals](https://opentelemetry.io/docs/concepts/signals/) · [OAuth 2.0 Error Response](https://www.rfc-editor.org/rfc/rfc6749#section-5.2) · [gRPC Status Codes](https://grpc.io/docs/guides/status-codes/)
