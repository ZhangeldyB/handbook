# Testing Skills and Agents

Improve skills and agents by observing a fresh agent use them on real tasks.

## Two Sessions

- **Author session** — has the handbook context and edits the skill or agent.
- **Tester session** — a clean session in a scratch service with the handbook installed, given a natural task with no hints.

## Loop

1. Give the tester a realistic task that should trigger the skill, phrased the way a developer would ("add a `ShipOrder` command to orders").
2. Observe:
   - Did the right skill or agent trigger from the description alone?
   - Did it read the referenced rules and docs before writing code?
   - Did it follow the steps in order?
   - Did it avoid each guardrail violation?
   - Does the result satisfy every Done item?
3. Bring concrete observations to the author session ("it added `ctx` to the domain method despite the rule"; "it skipped the round-trip test").
4. Refine the skill: sharpen the description's trigger words, make a guardrail more specific, add a missing Done item.
5. Repeat on the same task until it passes, usually two or three iterations.

## What to Probe

| Signal | Meaning | Action |
|---|---|---|
| Skill does not trigger | Description does not match how people ask | Add the user's words to the description |
| Wrong skill triggers | Descriptions overlap | Differentiate them; state what each is *not* for |
| References skipped | Read-first list is not compelling or too long | Shorten it; put the checklist file first |
| Guardrail ignored | Too vague or buried | Make it concrete and move it up |
| Output misses quality the Done list does not mention | Done list incomplete | Add the item |

## Standard Probes

| Skill | Task | Watch for |
|---|---|---|
| `create-aggregate` | "Create a Shipment aggregate in the shipping context" | `ctx` in domain methods, exported fields, missing transition table |
| `add-repository` | "Add persistence for Shipment" | Acknowledging before commit, missing version check, incomplete round-trip fixture |
| `implement-usecase` | "Implement CancelShipment" | Transactions in the usecase, missing `Deps` validation |
| `add-grpc-handler` | "Expose CancelShipment over gRPC" | Tenant from the request, per-handler error switch |
| `add-event-consumer` | "React to payments.payment_refunded.v1" | Missing inbox record, acking poison forever-retry |
| `add-reconciler` | "Capture payment with the provider" | Provider call inside a transaction, timeout treated as failure |
