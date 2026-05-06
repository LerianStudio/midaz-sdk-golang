# 04-listing-cursor

Demonstrates **cursor-based pagination** — the v3 idiom for endpoints
where the server's pagination contract is "give me a cursor, you give
me back items, you tell me the next cursor."

In v3, the SDK enforces the cursor/page distinction at the type system
level: cursor-only endpoints expose `Cursor` in their typed list-opts
struct and do NOT accept a `Page` field. Wrong-shape opts don't compile.

## What this demonstrates

- The iter.Seq2 trio for cursor-paginated data:
  - `ListTransactions` (one page)
  - `ListTransactionsAll` (every item, hides cursor advance)
  - `ListTransactionsPages` (every page envelope, with metadata)
- Early termination via plain `return` from the range-over-func loop
- Per-item error handling (the `_, err = range` shape)

## When to use this pattern

For cursor-only endpoints: Transactions, Operations, OperationRoutes,
TransactionRoutes. For page-based endpoints
(Organizations, Ledgers, Accounts, Assets, etc.) see
[`05-listing-pages/`](../05-listing-pages/).

If you mix them up the SDK won't compile — that's by design.

## How to run

```bash
go run ./examples/04-listing-cursor
```

Requires a local Midaz stack with at least a few transactions seeded.
Run [`03-end-to-end/`](../03-end-to-end/) first to create some.

## Expected output

```
[0] tx_01H... 100 USD SETTLED
[1] tx_01H... 250 USD PENDING
...
processed N transactions
```

## Related

- [`05-listing-pages/`](../05-listing-pages/) — page-based pagination (the other pattern)
- [`docs/pagination.md`](../../docs/pagination.md) — pagination contract and migration table
