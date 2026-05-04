# Pagination in the Midaz Go SDK

The SDK uses `models.ListOptions` for list request parameters and `models.ListResponse[T]` for paginated results. Entity list methods are accessed through `c.Entity`, for example `c.Entity.Accounts.ListAccounts(...)`.

## List options

Create options with defaults:

```go
options := models.NewListOptions().
    WithLimit(25).
    WithPage(1).
    WithOrderDirection(models.SortDescending).
    WithFilter("status", "ACTIVE")
```

Available common helpers:

| Helper | Behavior |
| --- | --- |
| `WithLimit(int)` | Sets maximum items per page, capped by `models.MaxLimit` (`100`) |
| `WithOffset(int)` | Compatibility input for older callers. Current Midaz list endpoints do not expose an `offset` wire parameter; use `WithPage` or `WithCursor` for new code. |
| `WithPage(int)` | Sets page number |
| `WithCursor(string)` | Sets cursor for cursor-aware endpoints |
| `WithOrderBy(string)` | Stored for compatibility; common serialization does not send it |
| `WithOrderDirection(models.SortDirection)` | Sends sort direction as `sort_order` |
| `WithFilter(string, string)` | Adds one filter query parameter |
| `WithFilters(map[string]string)` | Replaces the filter map |
| `WithDateRange(string, string)` | Adds `start_date` and `end_date` filters |
| `WithAdditionalParam(string, string)` | Adds an endpoint-specific query parameter |

CRM helper filters are also available on `ListOptions`:

- `WithIncludeDeleted(bool)`
- `WithHolderID(string)`
- `WithExternalID(string)`
- `WithDocument(string)`
- `WithAccountID(string)`
- `WithLedgerID(string)`
- `WithParticipantDocument(string)`
- `WithRelatedPartyDocument(string)`
- `WithBankingDetailsBranch(string)`
- `WithBankingDetailsAccount(string)`
- `WithBankingDetailsIBAN(string)`
- `WithRelatedPartyRole(string)`

## Constants

```go
models.DefaultLimit
models.MaxLimit
models.DefaultOffset
models.DefaultPage
models.SortAscending
models.SortDescending
```

`models.DefaultLimit` is `10`, and `models.MaxLimit` is `100`. The generic `pkg/pagination` paginator uses the same cap through `MaxPaginationLimit`, so SDK entity list methods and paginator helpers enforce the same maximum limit.

## Ledger and CRM pagination semantics

Midaz services do not use one universal pagination shape. Choose options based on the service you call:

| API family | Methods | Wire pagination | Notes |
| --- | --- | --- | --- |
| Ledger page-based lists | Organizations, Ledgers, Assets, Portfolios, Segments, Accounts, Account Types, and balances | `page`, `limit`, filters, and `sort_order` | Use `WithPage` and `WithLimit`. Do not rely on `offset` as a Midaz wire parameter. |
| Ledger cursor-aware lists | Transactions, Operations, Operation Routes, Transaction Routes, and Asset Rates | `cursor`, `limit`, filters, and `sort_order` | These endpoints do not accept `page` or `offset`; the SDK emits cursor-style query parameters only. Asset rates also support `to`, `start_date`, and `end_date`. |
| CRM page-based lists | Holders and Aliases | `page`, `limit`, CRM filters, and `sort_order` | CRM filters include `include_deleted`, `holder_id`, `external_id`, `document`, `account_id`, `ledger_id`, `banking_details_branch`, `banking_details_account`, `banking_details_iban`, `regulatory_fields_participant_document`, `related_party_document`, and `related_party_role`. |

## Paginated responses

List methods return `*models.ListResponse[T]`:

```go
accounts, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
if err != nil {
    return err
}

for _, account := range accounts.Items {
    fmt.Println(account.ID)
}

fmt.Printf(
    "page=%d total_pages=%d has_next=%t",
    accounts.Pagination.CurrentPage(),
    accounts.Pagination.TotalPages(),
    accounts.Pagination.HasNextPage(),
)
```

`ListResponse` unmarshalling supports both current top-level pagination fields and legacy nested `pagination` responses.

`TotalPages()` is meaningful only when the API returns `total`. Current Midaz list responses commonly omit `total`, so `TotalPages()` falls back to `1`. For traversal, use `HasNextPage()`, `NextPageOptions()`, and cursor metadata instead of assuming a total page count exists.

## Navigating pages

Use pagination helpers to avoid manually rebuilding options:

```go
options := models.NewListOptions().WithLimit(50)

for {
    page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
    if err != nil {
        return err
    }

    for _, account := range page.Items {
        process(account)
    }

    if !page.Pagination.HasNextPage() {
        break
    }

    options = page.Pagination.NextPageOptions()
}
```

Navigation helpers:

- `HasMorePages()`
- `HasPrevPage()`
- `HasNextPage()`
- `NextPageOptions()`
- `PrevPageOptions()`
- `CurrentPage()`
- `TotalPages()`

Use `TotalPages()` for display only after you confirm the API response includes `total`. Use `HasNextPage()` or `NextCursor` to decide whether to fetch another page.

## Cursor behavior

Cursor support is endpoint-specific. `ListOptions.WithCursor(...)` sets the `cursor` query parameter. Transaction listing has explicit cursor behavior: when a cursor is set, the SDK removes `page` and sends the cursor-based request.

```go
options := models.NewListOptions().
    WithLimit(100).
    WithCursor(nextCursor)

transactions, err := c.Entity.Transactions.ListTransactions(ctx, orgID, ledgerID, options)
```

## Filtering and sorting

Filters are sent as query parameters:

```go
options := models.NewListOptions().
    WithLimit(20).
    WithFilter("status", "ACTIVE").
    WithFilter("assetCode", "USD").
    WithDateRange("2026-01-01", "2026-12-31")
```

`WithOrderDirection` is sent as `sort_order`. `WithOrderBy` is retained on the options struct for compatibility, but common query serialization does not currently send it.

## Count methods

Supported count helpers issue `HEAD` requests to Midaz `metrics/count` endpoints. Midaz returns the count in the `X-Total-Count` response header, and the SDK converts that header into `models.MetricsCount`.

| Service | Method | Count field |
| --- | --- | --- |
| Organizations | `GetOrganizationsMetricsCount(ctx)` | `OrganizationsCount` |
| Ledgers | `GetLedgersMetricsCount(ctx, organizationID)` | `LedgersCount` |
| Assets | `GetAssetsMetricsCount(ctx, organizationID, ledgerID)` | `AssetsCount` |
| Portfolios | `GetPortfoliosMetricsCount(ctx, organizationID, ledgerID)` | `PortfoliosCount` |
| Segments | `GetSegmentsMetricsCount(ctx, organizationID, ledgerID)` | `SegmentsCount` |
| Accounts | `GetAccountsMetricsCount(ctx, organizationID, ledgerID)` | `AccountsCount` |
| Transactions | `GetTransactionsMetricsCount(ctx, organizationID, ledgerID, opts)` | `TransactionsCount` |

If Midaz omits `X-Total-Count` or returns a blank, non-integer, negative, or overflowing value, the SDK returns an internal SDK error for the count request. `GetAccountTypesMetricsCount` exists only as a deprecated compatibility method and returns a validation error because the Midaz Ledger API does not expose account type count metrics.

## Best practices

- Always set a bounded `Limit` for list calls.
- Prefer `NewListOptions()` instead of constructing `ListOptions` manually.
- Use `NextPageOptions()` and `PrevPageOptions()` when following response metadata.
- Treat `WithOffset` as compatibility input only. Current Midaz requests use `page`, `limit`, and `cursor` where supported; there is no supported `offset` wire contract.
- Use cursor pagination only for endpoints that document or return cursor metadata.
