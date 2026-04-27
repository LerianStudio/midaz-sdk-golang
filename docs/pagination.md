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
| `WithLimit(int)` | Sets maximum items per page, capped by `models.MaxLimit` |
| `WithOffset(int)` | Compatibility input; common serialization converts it to `page` |
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

## Constants

```go
models.DefaultLimit
models.MaxLimit
models.DefaultOffset
models.DefaultPage
models.SortAscending
models.SortDescending
```

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

## Best practices

- Always set a bounded `Limit` for list calls.
- Prefer `NewListOptions()` instead of constructing `ListOptions` manually.
- Use `NextPageOptions()` and `PrevPageOptions()` when following response metadata.
- Treat `WithOffset` as compatibility input; current requests use `page` and `limit`.
- Use cursor pagination only for endpoints that document or return cursor metadata.
