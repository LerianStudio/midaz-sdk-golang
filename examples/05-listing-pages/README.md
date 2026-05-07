# 05-listing-pages

Demonstrates **page-based pagination** — the v3 idiom for endpoints
where the server returns a fixed-size page identified by a 1-indexed
page number.

The same iter.Seq2 trio shape as cursor-based pagination, but with
typed `Page` and `Limit` fields on the opts struct.

## What this demonstrates

- `ListAccounts` (one page)
- `ListAccountsAll` (iter.Seq2[T, error] for every item)
- `ListAccountsPages` (iter.Seq2[*ListResponse[T], error] for every page envelope)
- The early-termination idiom: just `return` from the range-over-func loop

## When to use this pattern

For page-based endpoints: Organizations, Ledgers, Accounts, Assets,
Portfolios, Segments, AssetRates, Holders, AccountTypes. For cursor-only
endpoints (Transactions etc.), see
[`04-listing-cursor/`](../04-listing-cursor/).

The trio gives you three control points for the same data:
- One page → "give me a UI button-driven pager"
- Every item → "I want to think collection, not pages"
- Every page → "I need page metadata for checkpointing or progress"

## How to run

```bash
go run ./examples/05-listing-pages
```

Requires a local Midaz stack with at least one organization, one ledger,
and a few accounts. Run [`03-end-to-end/`](../03-end-to-end/) first to
seed.

## Expected output

```
--- ListAccounts (one page) ---
page 1 returned 5 accounts (total available may be larger)
  - Account A (acc_01H...)
  ...
--- ListAccountsAll (every item) ---
  [1] Account A (acc_01H...)
  ...
--- ListAccountsPages (every page envelope) ---
  page 1: 5 items
  page 2: 5 items
  ...
```

## Related

- [`04-listing-cursor/`](../04-listing-cursor/) — the cursor pattern
- [`docs/pagination.md`](../../docs/pagination.md) — full pagination contract
