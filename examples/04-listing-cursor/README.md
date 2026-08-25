# 04-listing-cursor

Demonstrates **cursor-based pagination** — the idiom for endpoints
where the server's pagination contract is "give me a cursor, you give
me back items, you tell me the next cursor."

The SDK enforces the cursor/page distinction at the type system
level: cursor-only endpoints expose `Cursor` in their typed list-opts
struct and do NOT accept a `Page` field. Wrong-shape opts don't compile.

## What this demonstrates

- The iter.Seq2 trio for cursor-paginated data, on `c.V2.Transactions`:
  - `List` (one page)
  - `All` (every item, hides cursor advance)
  - `Pages` (every page envelope, with metadata)
- Early termination via plain `break` from the range-over-func loop
- Per-item error handling (the `_, err = range` shape)
- `Count`, the only place on the transaction surface where narrowing by
  status or route works

## What a transaction list can narrow by

The date range, the sort direction, and **one metadata predicate**.
Nothing else.

Six fields on `models.TransactionsFilters` — `Status`, `AssetCode`,
`Reference`, `SourceAccount`, `DestinationAccount`, `Route` — are
**refused before the request is built**, on both surfaces. The ledger
never honored them: it parses two and drops them on the floor, and never
parses the other four. Setting one used to return the whole unfiltered
ledger with a nil error, which is why they are now a local refusal rather
than a silent full scan.

`Status` and `Route` *are* honored by `Count`. Note that Count's default
window is **today**, not the ledger: leave the dates unset and the server
fills in the current UTC day. Both dates take the same `YYYY-MM-DD`
spelling `List` takes, and each names a whole, inclusive day.

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
[1] Manual cursor loop — explicit page advance
  page 1: 50 items, hasMore=true
  page 2: 12 items, hasMore=false

[2] V2.Transactions.Pages — page-level iter.Seq2
  page 1: 3 items

[3] ListOperationsAll — flat item iter.Seq2
  total: 128 operations

[4] Early termination — break stops cursor advance
  found first canceled tx 01H..., stopping

[5] V2.Transactions.Count — where Status and Route do narrow
  approved between <start date> and <today>: <count>
```

The count window is the last 30 UTC days by default, so the dates above move
with the run. Set `DEMO_COUNT_DAYS` to widen or narrow the span.

## Related

- [`05-listing-pages/`](../05-listing-pages/) — page-based pagination (the other pattern)
- [`docs/pagination.md`](../../docs/pagination.md) — pagination contract and migration table
