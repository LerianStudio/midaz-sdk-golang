# workflow-with-entities

End-to-end **complete entity workflow** reference. Walks every public
resource in the SDK (organization, ledger, asset, account, portfolio,
segment, account-type, operation-route, transaction-route, transaction)
through their full Create/Get/Update/List/Delete lifecycle.

## What this demonstrates

- Every public service on the client (`c.Organizations`, `c.Ledgers`,
  ..., `c.Transactions`) used at least once
- The CRUD lifecycle for each resource
- Concurrent and sequential transaction patterns
- "Insufficient funds" error handling
- Workflow orchestration with rollback / cleanup

## When to use this pattern

As a comprehensive reference for v3 API shape. Scan it to find the
specific call shape you need; copy the relevant function into your
own code; discard the rest.

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
