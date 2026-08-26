# Pagination in the Midaz Go SDK

The SDK uses typed list-opts per endpoint and ships paginated entity list methods in a trio:
`List` (one page), `All` (every item across pages, as `iter.Seq2`), and
`Pages` (every page envelope, as `iter.Seq2`). Cursor-based and
page-based endpoints have separate opts types — wrong-shape opts don't
compile. Metadata index listing is not paginated and does not use this trio.

## The list-method trio

Paginated entity list methods ship in three flavors. Using `Accounts`
as the worked example:

| Method | Returns | Use when |
| --- | --- | --- |
| `List` | `*models.ListResponse[Account]` (one page) | You want exactly one page and decide when to advance. |
| `All` | `iter.Seq2[Account, error]` | You want to consume every item linearly; the SDK handles paging. |
| `Pages` | `iter.Seq2[*ListResponse[Account], error]` | You need page-level metadata (cursor, total, page number) for checkpointing or stopping mid-page. |

Ledger accessors are reached through the version group that serves them —
`c.V1.Accounts` / `c.V2.Accounts` — and /v2 is the surface to build against,
since Midaz deprecated all of /v1.

Accessors whose resource has exactly one list share the bare
`List` / `All` / `Pages` trio: `Organizations`, `Ledgers`, `Accounts`,
`AccountTypes`, `Assets`, `Portfolios`, `Segments`, `OperationRoutes`,
`TransactionRoutes`, `Transactions`, and — V2 only — `Holders` and
`FeePackages`.

Accessors serving **more than one** list use a prefixed form instead, because a
bare `List` would not say which:

| Accessor | Spellings |
| --- | --- |
| `Balances` | `ListBalances{,All,Pages}` (ledger-wide), `ListAccountBalances{,All,Pages}` (account-scoped) |
| `Operations` | `ListOperations{,All,Pages}` |
| `AssetRates` (V1 only) | `ListAssetRatesByAssetCode{,All,Pages}` |
| `Instruments` (V2 only) | `List` / `ListAll` / `ListPages` for instruments, `ListAccountsByHolder{,All,Pages}` for the holder's accounts |
| `BillingPackages` (V2 only) | `List` / `ListAll` / `ListPages` — one list, but spelled with the prefix its facade was written with |

Two lists have **no `All` or `Pages` variant at all**, because the endpoints
are not paginated: `Balances.ListBalancesByAccountAlias` and
`ListBalancesByExternalCode` accept no query parameters and answer with a fixed
page, so an iterator over them could not advance. They take no opts either.
`MetadataIndexes.List` returns a plain slice rather than a `ListResponse`
envelope, for the same reason.

## Page-based vs cursor-based endpoints

Midaz exposes two pagination shapes. Each entity's typed opts struct
embeds the right base, so the type system picks the correct shape for
you:

| Shape | Endpoints | Opts base | Wire parameters |
| --- | --- | --- | --- |
| Page-based | Organizations, Ledgers, Assets, Portfolios, Segments, Accounts, Account Types, Billing Packages, Fee Packages | `models.PageListOpts` | `page`, `limit`, filters, `sort_order`, `start_date`, `end_date` |
| Cursor-based | Transactions, Operations, **Balances**, Operation Routes, Transaction Routes, Asset Rates, **Holders**, **Instruments**, and the whole Tracer plane (Rules, Limits, Validations, Audit Events) | `models.CursorListOpts` | `cursor`, `limit`, filters, `sort_order`, `start_date`, `end_date` |

Some cursor endpoints reject the date pair (`ValidateCursorListOptsNoDates`) —
Holders, Instruments and Limits among them — because the server does not accept
it there.

**Balances being cursor-based is load-bearing, not incidental.** The server
builds each balance page from limit / sort_order / the date range plus a cursor
and drops `page` on the floor. An opts shape carrying `Page` would compile,
send a parameter with no wire slot, and leave an iterator that increments it
re-requesting the first page forever — which is exactly what the SDK did before
v5, yielding the same balances indefinitely on any multi-page ledger.
`BalancesListOpts` embeds `CursorListOpts` so that shape cannot be expressed.

Not paginated at all, so excluded from the trio: Metadata Indexes
(`MetadataIndexes.List` returns a plain slice), and the alias and external-code
balance lookups.

## Iterator-based pagination (Go 1.23+ `iter.Seq2`)

The `*All` and `*Pages` methods return `iter.Seq2[T, error]`, ranged
directly with `for ... range`. The first range variable is the value;
the second is a per-iteration error. Stop with `break`; the SDK aborts
in-flight paging.

### Iterating items with `*All`

`*All` is the right call when you consume items linearly and don't need
page metadata:

```go
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 100},
    Filters:      models.AccountsFilters{Status: "ACTIVE"},
}

for account, err := range c.V2.Accounts.All(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("list accounts: %w", err)
    }

    process(account)
}
```

`Limit` controls the per-request page size, not the total number of
items returned. `*All` keeps fetching pages until the server reports no
more.

### Iterating pages with `*Pages`

`*Pages` is the right call when you need cursor/total metadata, want to
checkpoint between pages, or stop mid-iteration on a per-page condition:

```go
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 100},
}

for page, err := range c.V2.Accounts.Pages(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("list accounts pages: %w", err)
    }

    log.Printf("page=%d items=%d total=%d",
        page.Pagination.Page, len(page.Items), page.Pagination.Total)

    if shouldStop(page) {
        break
    }
}
```

Each yielded `*ListResponse[T]` carries the full envelope — `Items`
plus `Pagination` (with `Page`, `Limit`, `Total`, `NextCursor`,
`PrevCursor`, and `HasMore()` for the canonical "more pages?" signal).

### Cursor-based example

Cursor endpoints use the same iterator shape with `CursorListOpts`:

```go
opts := models.TransactionsListOpts{
    CursorListOpts: models.CursorListOpts{Limit: 50},
}

for tx, err := range c.V2.Transactions.All(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("list transactions: %w", err)
    }

    process(tx)
}
```

See [`examples/04-listing-cursor/`](../examples/04-listing-cursor/) and
[`examples/05-listing-pages/`](../examples/05-listing-pages/) for runnable demos.

## Collecting iterator results into slices

Use `entities.Collect` or `entities.CollectAll` when you need a slice instead of streaming a `for ... range` loop:

```go
accounts, err := entities.Collect(
    c.V2.Accounts.All(ctx, orgID, ledgerID, opts),
    1000,
)
if err != nil {
    return fmt.Errorf("collect accounts: %w", err)
}
```

`Collect` stops after `maxItems` items and returns partial results with the first error. `CollectAll` drains the full iterator with no memory cap, so use it only when the result set is known to be small.

## Single-page calls with `List`

Use `List` when you control the page advance yourself — for example,
when paginating through a UI or when each page maps to a separate
job/checkpoint. `List` returns one `*ListResponse[T]`:

```go
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 25, Page: 1},
}

page, err := c.V2.Accounts.List(ctx, orgID, ledgerID, opts)
if err != nil {
    return err
}

for _, account := range page.Items {
    process(account)
}

if page.Pagination.HasMore() {
    // Fetch the next page yourself by incrementing Page (page-based) or
    // copying NextCursor into the next opts (cursor-based).
}
```

`Pagination.HasMore()` is the canonical "more pages?" signal. It uses
`NextCursor` for cursor endpoints, the `Total + Limit + Page`
arithmetic when `Total` is reported, and a `Limit == ItemCount`
heuristic otherwise.

## Choosing between `*All`, `*Pages`, and `List`

- `*All` — default for batch processing. Cleanest code; no manual paging.
- `*Pages` — when you need per-page metadata (cursor checkpointing,
  total-count display, mid-iteration termination based on page state).
- `List` — when paging is driven externally (UI controls, distributed
  workers, manual replay).

## Constants

```go
models.DefaultLimit  // 10
models.MaxLimit      // 100
models.SortAscending
models.SortDescending
```

`Limit > MaxLimit` causes the entity-level `Validate()` to return a
typed validation error before the request leaves the SDK.

## Best practices

- Set a bounded `Limit` on every list call. Prefer larger pages
  (50–100) for `*All` to reduce round trips.
- Prefer `*All` for batch processing; reach for `*Pages` only when you
  need page metadata.
- Stop iterators with `break` — the SDK aborts in-flight paging cleanly.
- For checkpointable jobs, snapshot `Pagination.NextCursor` (cursor
  endpoints) or `Pagination.Page` (page endpoints) inside `*Pages` and
  resume by seeding the next opts with the saved cursor/page.
- Use `Pagination.HasMore()` instead of comparing item counts manually.
