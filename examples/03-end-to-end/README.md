# 03-end-to-end

Walks the canonical resource hierarchy:
**organization → ledger → asset → account → transaction**.

This is the example to read first if you want to understand the v3 API
shape without auth complexity.

## What this demonstrates

- Anonymous client construction (local stack)
- `c.Organizations`, `c.Ledgers`, `c.Assets`, `c.Accounts`, `c.Transactions`
- Building a transaction via the SDK's clean DSL input format (no
  lib-commons types leak into your code)
- Observability provider wired through `WithObservabilityProvider`

## When to use this pattern

As a reference for the typical "set up, then make a financial movement"
shape. Real applications usually skip the org/ledger/asset/account
creation (those are infrastructure setup) and start at transaction-time.

## How to run

```bash
go run ./examples/03-end-to-end
```

Requires a local Midaz stack with auth disabled.

## Expected output

```
Created transaction: "tx_01H..."
```

## Related

- [`05-listing-pages/`](../05-listing-pages/), [`04-listing-cursor/`](../04-listing-cursor/) — listing the resources you create
- [`06-idempotency/`](../06-idempotency/) — making transaction creation safe under retries
- [`workflow-with-entities/`](../workflow-with-entities/) — the same flow at scale, with concurrency
