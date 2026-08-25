# 05-listing-pages

Demonstrates **page-based pagination** — the v3 idiom for endpoints
where the server returns a fixed-size page identified by a 1-indexed
page number.

The same iter.Seq2 trio shape as cursor-based pagination, but with
typed `Page` and `Limit` fields on the opts struct.

## What this demonstrates

- `List` (one page)
- `All` (iter.Seq2[T, error] for every item)
- `Pages` (iter.Seq2[*ListResponse[T], error] for every page envelope)
- The early-termination idiom: just `break` from the range-over-func loop

All three are shown on `c.V2.Accounts` — /v2 is the surface to build
against, since Midaz deprecated all of /v1.

## When to use this pattern

For page-based endpoints: Organizations, Ledgers, Accounts, Assets,
Portfolios, Segments, AccountTypes. For cursor-only endpoints —
Transactions, Operations, Balances, AssetRates, Holders, OperationRoutes,
TransactionRoutes — see [`04-listing-cursor/`](../04-listing-cursor/).

The trio gives you three control points for the same data:
- One page → "give me a UI button-driven pager"
- Every item → "I want to think collection, not pages"
- Every page → "I need page metadata for checkpointing or progress"

## How to run

```bash
go run ./examples/05-listing-pages
```

Requires a local Midaz stack with at least one organization, one ledger,
and a few accounts. Run
[`workflow-with-entities/`](../workflow-with-entities/) or `make demo-data`
first to seed them.

## Expected output

```
--- V2.Accounts.List (one page) ---
page 1 returned 5 accounts (total available may be larger)
  - Account A (01H...)
  ...
--- V2.Accounts.All (every item) ---
  [1] Account A (01H...)
  ...
--- V2.Accounts.Pages (every page envelope) ---
  page 1: 5 items
  page 2: 5 items
  ...
```

## Related

- [`04-listing-cursor/`](../04-listing-cursor/) — the cursor pattern
- [`docs/pagination.md`](../../docs/pagination.md) — full pagination contract
