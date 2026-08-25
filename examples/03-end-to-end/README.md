# 03-end-to-end

Creates a **transaction on the /v2 surface** — the money movement at the
end of the organization → ledger → asset → account hierarchy. The
resources it posts against are assumed to exist; see
[`workflow-with-entities/`](../workflow-with-entities/) for the flow that
creates them.

This is the example to read first if you want to understand the API shape
without auth complexity.

## What this demonstrates

- Anonymous client construction (local stack)
- `c.V2.Transactions.CreateDirect` — the /v2 creation path
- The flat /v2 transaction body: an asset, a total, and two leg arrays
  (debits and credits), with the action in the URL rather than the body
- Observability provider wired through `WithObservabilityProvider`

## Which surface, and why

Midaz deprecated all of /v1, so **/v2 is the surface to build against**.
The /v2 creates are top-level actions — `CreateDirect` settles
immediately, `CreateHold` reserves value for a later `Commit`.

The four /v1 creation styles (json, inflow, outflow, annotation) with
their nested `send`/`source`/`distribute` envelope have **no /v2 twin**.
They stay reachable as `c.V1.Transactions.CreateJSON` and friends for as
long as the server serves /v1 —
[`workflow-with-entities/`](../workflow-with-entities/) demonstrates them.

Each leg names the organization and ledger its account belongs to. The
facade stamps those from the pair you pass to `CreateDirect`, into a copy,
so one input can be reused against a second ledger without carrying the
first one's scope into it — and a leg naming a *different* pair is refused
rather than silently posted into the wrong ledger.

## When to use this pattern

As a reference for "make a financial movement". Real applications skip the
org/ledger/asset/account creation (that is infrastructure setup) and start
at transaction time, which is where this example starts.

## How to run

```bash
go run ./examples/03-end-to-end
```

Requires a local Midaz stack with auth disabled.

## Expected output

```
Created transaction: "01H..." (status APPROVED)
```

## Related

- [`05-listing-pages/`](../05-listing-pages/), [`04-listing-cursor/`](../04-listing-cursor/) — listing the resources you create
- [`06-idempotency/`](../06-idempotency/) — making transaction creation safe under retries
- [`workflow-with-entities/`](../workflow-with-entities/) — the same flow at scale, with concurrency
