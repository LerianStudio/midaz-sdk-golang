# workflow-with-entities

End-to-end **entity workflow** reference. Walks the core ledger
resources — organization, ledger, asset, account, portfolio, segment,
account-type, operation-route, transaction-route, transaction — through
their full Create/Get/Update/List/Delete lifecycle.

**Scope, stated honestly.** This walks the resource families the ledger
serves on BOTH surfaces, through `c.V2.*` (`c.V2.Organizations`,
`c.V2.Ledgers`, ..., `c.V2.TransactionRoutes`). The transaction flows
stay on `c.V1.Transactions` on purpose: the nested send/source/distribute
creation style exists only on /v1, so there is no /v2 twin to swap onto.

The nine **V2-only** services — `Holders`, `Instruments`, `Encryption`,
`Composition`, `ProtectionAudit`, `FeePackages`, `FeeEstimates`,
`BillingPackages`, `BillingCalculations` — are **not** walked here. For a
runnable pass over them, use
[`../mass-demo-generator/`](../mass-demo-generator/): its V2 phase exercises all
nine against a live stack, alongside the /v2 transaction cycle and its balance
proof. Set `DEMO_RUN_V2=true` (the default).

## What this demonstrates

- Each dual-served resource family used at least once
- The CRUD lifecycle for each resource
- Concurrent and sequential transaction patterns
- "Insufficient funds" error handling
- Workflow orchestration with rollback / cleanup

## When to use this pattern

As a broad reference for API shape. Scan it to find the specific call
shape you need; copy the relevant function into your own code; discard
the rest.

This example is intentionally large and unfocused — it is NOT a
tutorial. For tutorials, see the numbered examples (01-10).

## How to run

```bash
go run ./examples/workflow-with-entities
```

Requires a local Midaz stack with auth disabled.

## Related

- [`mass-demo-generator/`](../mass-demo-generator/) — same shape at scale
- All numbered examples — focused tutorials for individual concepts
