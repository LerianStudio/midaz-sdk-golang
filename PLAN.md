# PLAN — Transaction status vocabulary, lifecycle errors, and revert/cancel correctness

> Status: ready to implement / decisions locked
> Origin: discovered while integrating `plugin-br-bank-transfer` (TED chargeback
> revert reconciliation). The plugin could not prove a Midaz revert by reading a
> transaction's status back, because the SDK's published status constants do not
> match what the Midaz server actually emits. This plan fixes the root causes in
> the SDK so downstream consumers stop hand-rolling raw-string workarounds.
> SDK checkout at authoring time: module `github.com/LerianStudio/midaz-sdk-golang/v3`, `v4.0.0-2-gbcf6656`.

## Ground truth (verified against `../midaz`, server module `github.com/LerianStudio/midaz/v3`)

Every claim below was read from the live server source, not inferred:

- **Status enum is exactly 5 values** (`midaz/pkg/constant/transaction.go`):
  `CREATED`, `APPROVED`, `PENDING`, `CANCELED`, `NOTED`. The server documents this
  set explicitly in its own filter handler
  (`count_transactions_by_filters.go:41`: `Enums(CREATED, APPROVED, PENDING, CANCELED, NOTED)`).
- **There is no `REJECTED`, `FAILED`, or `DECLINED` status anywhere** in the
  non-test server code. The set of 5 is complete and authoritative — the enum can
  be frozen with confidence. The SDK example's `"REJECTED"` branch is dead code.
- **Lifecycle** (`transaction_state_handlers.go`):
  - `create(pending:true)` → `PENDING`; `create(pending:false)` → `APPROVED`.
  - annotation create → `NOTED` (no balance impact).
  - **commit** requires `PENDING` (`:421`) → `APPROVED` (`:84`).
  - **cancel** requires `PENDING` (`:421`) → `CANCELED` (`:142`).
  - **revert** requires original `APPROVED` (`:227`); creates a **new child**
    reversal transaction (`CREATED`→`APPROVED`, `:287`); the original's status is
    **never mutated**. Re-attempt is rejected via the parent-transaction guard
    (`:199` → 0087, `:218` → 0088).
- **SDK currently licensed Apache 2.0** (`LICENSE.md`); server is **Elastic License 2.0**.

## Decisions locked (with Fred, 2026-06-01)

1. **No backward compatibility.** Remove the wrong constants outright and do it
   right per the server contract. The old constants are already 100% broken
   against a real server and have zero internal usages, so removal breaks only
   code that never worked. Ship as a major bump.
2. **Fix everything, properly** — not just the constants, but every broken helper,
   doc comment, and example that carries the phantom vocabulary.
3. **Server as a (build-tagged) test-only dependency** for the drift guard, importing
   `github.com/LerianStudio/midaz/v3/pkg/constant` directly. This is the strongest
   anti-drift guarantee. Because that places ELv2 code in the SDK's module graph,
   **the SDK is relicensed Apache 2.0 → Elastic License 2.0** to keep the licensing
   consistent (server and SDK both ELv2).
   - **Sequencing:** the relicense is a **separate PR** (via the `ring:dev-licensing`
     skill) and is a **prerequisite** of adding the go.mod edge. Do not bury a
     license change inside the enum fix.

## TL;DR

1. **The transaction status constants are factually wrong.** The SDK publishes
   `pending/completed/failed/cancelled` (lowercase, UK spelling). The server emits
   `CREATED / APPROVED / PENDING / CANCELED / NOTED`. Replace with a typed enum
   mirroring the server exactly.
2. **The transaction lifecycle business-error codes are not exposed as typed
   constants.** Codes like `0087` ("revert already exists") are only reachable as
   a raw `*sdkerrors.Error.APICode` string. Publish typed constants + predicates.
3. **Revert / Commit / Cancel are already enabled**, but their doc comments
   describe the wrong status vocabulary and expose no precondition/idempotency
   contract. Fix the contract and docs.

---

## Problem 1 — Transaction status enum chaos

### Evidence

SDK (`models/constants.go:41-59`) — all wrong:
```go
TransactionStatusPending   = "pending"      // server: "PENDING" (case)
TransactionStatusCompleted = "completed"    // server: no such state; committed tx is "APPROVED"
TransactionStatusFailed    = "failed"       // server: no failure state in the status enum
TransactionStatusCancelled = "cancelled"    // server: "CANCELED" (US spelling, single L)
```

Internal contradictions confirmed across the SDK:
- `models/constants.go:3-35` — doc block describes a phantom `pending→completed/failed` lifecycle.
- `models/transaction.go:27-31` — doc claims `PENDING, COMPLETED, FAILED, CANCELED` (a third spelling).
- `models/transactions_list_opts.go:41` — filter doc says `(e.g. "COMPLETED")`, a phantom value.
- `examples/04-listing-cursor/main.go:92` filters with `"APPROVED"` (correct value, but bypasses constants);
  `:184` branches on `"REJECTED"` — **dead code**, server never emits it.
- `entities/transactions.go:1048` — on cancel-with-no-body, the SDK already synthesizes
  `Status{Code: "CANCELED"}` (the correct US spelling), proving the transport layer already
  knows the right vocabulary the public surface gets wrong.
- `models/common.go:21` `Status.Code` is a free string; its `enum` tag
  (`ACTIVE,INACTIVE,PENDING,SUSPENDED,DELETED`) is the *resource* status set, a
  different concept — leave it, but document the distinction.

### Impact
Any consumer doing `tx.Status.Code == models.TransactionStatusCompleted` (or
`Pending`/`Cancelled`) silently never matches. This broke the bank-transfer
plugin's chargeback revert proof and forced raw-string matching.

### Fix
1. Introduce the typed status, mirroring the server's 5-value set exactly:
   ```go
   // TransactionStatusCode is the canonical Midaz ledger transaction status.
   // Values mirror github.com/LerianStudio/midaz/v3/pkg/constant (server source of truth).
   type TransactionStatusCode string

   const (
       TransactionStatusCreated  TransactionStatusCode = "CREATED"
       TransactionStatusPending  TransactionStatusCode = "PENDING"
       TransactionStatusApproved TransactionStatusCode = "APPROVED"
       TransactionStatusCanceled TransactionStatusCode = "CANCELED"
       TransactionStatusNoted    TransactionStatusCode = "NOTED"
   )
   ```
2. **Remove** the old `TransactionStatus{Pending,Completed,Failed,Cancelled}`
   constants entirely (no deprecation shim — decision #1). The new typed set
   reclaims the clean names; the name collision on `TransactionStatusPending`
   is resolved because the old one no longer exists.
3. Keep the account/resource `Status*` set (`ACTIVE/INACTIVE/PENDING/CLOSED`)
   as-is; add a doc note distinguishing it from transaction status so the
   `*Pending` near-collision stops confusing readers.
4. Fix all the doc comments and examples enumerated above (kill the phantom
   `completed`/`failed`/`REJECTED` vocabulary).

---

## Problem 2 — Lifecycle business-error codes are not typed

### Evidence
Server lifecycle/revert error codes (`midaz/pkg/constant/errors.go`) — the plan's
original four plus the ones it missed:
```go
ErrParentTransactionIDNotFound              = "0021"  // revert target's parent not found
ErrTransactionIDHasAlreadyParentTransaction = "0087"  // revert already exists (idempotency signal)
ErrTransactionIDIsAlreadyARevert            = "0088"  // target is itself a revert
ErrTransactionCantRevert                    = "0089"  // reversal would be empty
ErrTransactionAmbiguous                     = "0090"  // ambiguous revert target
ErrParentIDSameID                           = "0091"  // parent == self
ErrCommitTransactionNotPending              = "0099"  // commit/cancel/revert status precondition
ErrRevertOnlyBidirectional                  = "0165"  // revert restricted to bidirectional routes
```
The SDK surfaces these only as a raw string on `*sdkerrors.Error.APICode`
(`pkg/errors/details.go:108`). No named constants, no predicates.

### Impact
Consumers must hardcode `err.APICode == "0087"` to detect, e.g., an idempotent
double-revert. Magic strings in financial-reconciliation code are a correctness
hazard.

### Fix
1. Publish typed API-code constants in `pkg/errors` mirroring the server set
   (`APICodeRevertAlreadyExists = "0087"`, `APICodeAlreadyARevert = "0088"`,
   `APICodeCannotRevert = "0089"`, `APICodeStatusPreconditionFailed = "0099"`,
   plus 0021/0090/0091/0165), cross-checked against the contract drift test.
2. Add predicates composing with the existing `Is*Error` family
   (`pkg/errors/errors.go:1457+`), e.g. `IsRevertAlreadyExistsError(err) bool`
   (matches 0087/0088) — the idempotency signal a revert caller actually needs.
3. Document on `RevertTransaction` that a re-attempt on an already-reverted
   transaction returns 0087/0088 and that this is the canonical "already done"
   proof (the original transaction's status never changes — see Problem 3).

---

## Problem 3 — Revert / Commit / Cancel correctness (already enabled, under-specified)

### Current state (no wiring work needed)
`entities/transactions.go` already exposes and implements all three:
- `RevertTransaction` (:103), `CommitTransaction` (:111),
  `CancelTransaction` (:119), `CancelTransactionWithResponse` (:122).
Wired into the facade and used by `pkg/transaction/helpers.go` and `pkg/generator`.
What's missing is correctness of contract and docs.

### Gaps
1. Doc comments imply a phantom `pending→completed/failed` model. The real
   lifecycle is the one in Ground truth above.
2. Preconditions are undocumented. Server rules: commit/cancel require `PENDING`
   (`:421`); revert requires `APPROVED` (`:227`). The SDK does no client-side
   validation (correct — server is authoritative) but must *document* these.
3. Idempotency contract is implicit. Revert is idempotent server-side via the
   parent-transaction check (re-attempt → 0087/0088).

### Fix
1. Rewrite commit/cancel/revert doc comments to describe the real lifecycle,
   preconditions, and the "revert creates a child; original stays APPROVED"
   semantics.
2. Reference the new typed error predicates (Problem 2) for the idempotency path.
3. Optionally add `IsTransactionReverted(ctx, ...)` resolving completion via the
   child reversal (`ParentTransactionID`) rather than the original's status —
   encapsulating the correct check once in the SDK.

---

## Broken-helper / doc inventory (Problem 1 fan-out — decision #2: fix all)

| Location | Current (wrong) | Fix |
|---|---|---|
| `pkg/transaction/helpers.go:727` | `IsTransactionSuccessful` → `Status.Code == "COMPLETED"` (always false) | `== string(TransactionStatusApproved)` |
| `pkg/transaction/helpers.go:737-751` | `GetTransactionStatus` switch on `COMPLETED/FAILED` (phantom) | map the real 5-value vocabulary |
| `models/transactions_list_opts.go:41` | filter doc `(e.g. "COMPLETED")` | real value |
| `pkg/format/format.go:625` | doc example `Code: "COMPLETED"` | real value |
| `models/transaction.go:27-31` | lifecycle doc `PENDING/COMPLETED/FAILED/CANCELED` | real lifecycle |
| `models/constants.go:3-35` | phantom lifecycle doc block | real lifecycle |
| `examples/04-listing-cursor/main.go:184` | `Status.Code == "REJECTED"` (dead) | remove / replace with a real terminal check |
| `pkg/transaction/batch.go:717-728` | lowercase `cancelled/completed/failed` | **verify** this is internal batch state, not tx status; leave if confirmed distinct |

`IsTransactionSuccessful` is the highest-value fix: it is a public function that
**always returns false** against a real server — the same failure mode that
motivated this plan, embedded in the SDK's own helper.

---

## Acceptance criteria

- [x] `models` exposes typed `TransactionStatusCode` with exactly the 5 server values;
      old untyped constants are removed (not deprecated).
- [x] **Contract/drift test** imports `github.com/LerianStudio/midaz/v3/pkg/constant`
      (pinned `v3.7.5`) and asserts the SDK's typed status set and lifecycle error
      codes match the server's, failing on drift. Implemented as a **nested module**
      (`contract/go.mod`) rather than a build tag: the nested module keeps the server
      edge out of the SDK's published `go.mod` entirely (confirmed — `go.sum` 1.7K,
      server graph pruned to nothing because `pkg/constant` is a leaf). Run with
      `make test-contract`.
- [x] SDK relicensed to Elastic License 2.0. Done in this branch (decision: enum
      theme first, then `LICENSE.md`); existing SPDX `Apache-2.0` headers in 29 `.go`
      files corrected to `Elastic-2.0`; README badge + license line updated. No
      per-file headers added.
- [x] `IsRevertAlreadyExistsError` returns true for a real double-revert (0087/0088)
      and false otherwise; typed predicates added for the lifecycle error set.
- [x] Every entry in the broken-helper/doc inventory is fixed; `IsTransactionSuccessful`
      matches `APPROVED`; no phantom `completed`/`failed`/`REJECTED` remains in code,
      docs, or examples. (Plus an out-of-inventory leak fixed: `pkg/validation/suggestion.go`
      was emitting phantom statuses to users.)
- [x] commit/cancel/revert doc comments describe the real lifecycle and preconditions.
- [ ] CHANGELOG documents the status-value correction and the license change as
      breaking changes, with a migration table. **(remaining)**

## Remaining / optional
- CHANGELOG entry (breaking: status values + license).
- Decide whether `make test-contract` is wired into `make ci` (adds a network fetch
  of the server module to every CI run) — currently standalone.
- `IsTransactionReverted` helper (Problem 3 fix #3) — not implemented; decide in/out.

## Out of scope
- Account/resource status constants (`ACTIVE/INACTIVE/PENDING/CLOSED`) — correct
  as-is; only a doc note on the `*Pending` naming distinction.
- Server-side changes — SDK-only; server `pkg/constant` is the source of truth.

## Open items
- The Apache→ELv2 switch is **catch-up, not a posture change**: Midaz already moved
  to ELv2 org-wide; the SDK staying on Apache was an oversight. The relicense PR
  just aligns it. Ensure source-file headers are updated alongside `LICENSE.md`
  (the `ring:dev-licensing` skill handles both).
- Decide whether `IsTransactionReverted` (Problem 3 fix #3) ships in this plan or a follow-up.
