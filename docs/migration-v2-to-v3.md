# Migrating from v2 to v3

v3 is a clean-cut major version. There is no transitional release, no deprecated-symbol window, no `// Deprecated:` shims. Customers swap their import from `/v2` to `/v3` and migrate at the same moment.

This guide walks every breaking change with side-by-side v2 / v3 code. It covers what's deleted, why, and what you write instead. If you hit a v2 pattern not listed here, search this file by keyword — and if you still can't find it, the SDK package's `*_test.go` files are the canonical executable reference.

> **Why a major version?** The v3 sweep retired ten classes of mistake at the type-system level: the naked-SDK construction footgun, silent auth misconfigurations, the v2 `ListOptions` mega-struct that no-op'd half its setters per endpoint, the `c.Entity.X.Y` indirection, the `mmodel.X` type-identity leak, the `MidazError` alias system, and several more. See [`docs/v3-dx-plan.md`](v3-dx-plan.md) for the full design rationale.

---

## Quickstart for migrators

The minimum-viable v2 → v3 swap, side by side:

### Before (v2)

```go
import (
    client "github.com/LerianStudio/midaz-sdk-golang"
    "github.com/LerianStudio/midaz-sdk-golang/entities"
    "github.com/LerianStudio/midaz-sdk-golang/models"
)

c, err := client.New(
    client.WithBaseURL("https://api.midaz.io"),
    client.UseAllAPIs(),
)
if err != nil {
    return err
}

opts := models.NewListOptions().
    WithLimit(20).
    WithFilter("status", "ACTIVE")

page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)
if err != nil {
    return err
}

for _, account := range page.Items {
    process(account)
}

if page.Pagination.HasNextPage() {
    opts = page.Pagination.NextPageOptions()
    // ... fetch the next page
}
```

### After (v3)

```go
import (
    "github.com/LerianStudio/midaz-sdk-golang/v3"
    "github.com/LerianStudio/midaz-sdk-golang/v3/models"
)

c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(midaz.AccessManager{
        Address:      "https://auth.midaz.io",
        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
    }),
)
if err != nil {
    return err
}
defer c.Shutdown(context.Background())

opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 20},
    Filters:      models.AccountsFilters{Status: "ACTIVE"},
}

for account, err := range c.Accounts.ListAccountsAll(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("list accounts: %w", err)
    }
    process(account)
}
```

The remaining sections walk every breaking change one by one.

---

## 1. Module path and package name

The module path is now versioned as `/v3`. The package name is now `midaz` (not `client`).

| Concern | v2 | v3 |
| --- | --- | --- |
| Module path | `github.com/LerianStudio/midaz-sdk-golang` | `github.com/LerianStudio/midaz-sdk-golang/v3` |
| Package name | `client` | `midaz` |
| Import alias idiom | `client "..."` (alias often required) | None — `midaz` is the package name |
| Constructor | `client.New(...)` | `midaz.New(...)` |
| Entry file | `client.go` | `midaz.go` |

Update your `go.mod` and your import statements in one pass:

```go
// v2
import client "github.com/LerianStudio/midaz-sdk-golang"
c, err := client.New(...)

// v3
import "github.com/LerianStudio/midaz-sdk-golang/v3"
c, err := midaz.New(...)
```

If you preferred the explicit alias for readability, you can keep it — `client "github.com/LerianStudio/midaz-sdk-golang/v3"` works fine, and `c, err := client.New(...)` is identical to `c, err := midaz.New(...)`.

The Go module system treats `/v2` and `/v3` as different modules, so you can run both side-by-side during migration without aliasing one out.

---

## 2. Authentication: `WithAuthToken` deleted; one auth source required

v2 silently accepted credential-less construction and surfaced the mistake as a 401 on the first API call. v3 closes that footgun at construction time with a typed `*errors.Error` of category `CategoryConfiguration`.

### Breaking changes

| v2 pattern | v3 replacement |
| --- | --- |
| `client.WithAuthToken("token")` | Deleted. Configure your Access Manager to mint tokens. |
| `c.SetAuthToken("token")` post-construction | Deleted. Tokens flow only through Access Manager. |
| `entities.WithPluginAuth(...)` | `midaz.WithAccessManager(...)` |
| `pkg/access-manager` import | `pkg/auth` (directory matches package name) |
| Construction with no auth source | Add `midaz.WithAnonymous()` for tests, or `midaz.WithAccessManager(...)` for production. |

### Migration patterns

**Production (Access Manager OAuth):**

```go
c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(midaz.AccessManager{
        Address:      "https://auth.midaz.io",
        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
    }),
)
```

The SDK eagerly fetches an initial token at construction time; misconfigurations surface as `*errors.Error` with `Category == CategoryConfiguration` instead of as 401 cascades on the first request.

**Local development / tests (anonymous):**

```go
c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentLocal),
    midaz.WithAnonymous(),
)
```

`WithAccessManager` and `WithAnonymous` are mutually exclusive — calling `New()` with neither returns:

```text
configuration error during midaz.New: invalid configuration: no auth source configured; use WithAccessManager or WithAnonymous
```

See [docs/auth.md](auth.md) for the full walkthrough.

---

## 3. Pagination: typed list-opts replace `models.ListOptions`

The single biggest API change. v2's `models.ListOptions` mega-struct exposed 30+ fluent setters that mostly no-op'd on any given endpoint. v3 replaces it with per-endpoint typed opts that fail at compile time when you set a field the endpoint doesn't honor.

### What's deleted

`models.ListOptions` and the entire builder surface:

- `models.NewListOptions()`
- All 30 fluent setters: `WithLimit`, `WithOffset`, `WithPage`, `WithCursor`, `WithOrderBy`, `WithOrderDirection`, `WithFilter`, `WithFilters`, `WithDateRange`, `WithAdditionalParam`, `WithIncludeDeleted`, `WithHolderID`, `WithExternalID`, `WithDocument`, `WithAccountID`, `WithPortfolioID`, `WithSegmentID`, `WithStatusFilter`, `WithTypeFilter`, `WithAssetCode`, `WithEntityID`, `WithBlocked`, `WithParentAccountID`, `WithNameFilter`, `WithAlias`, `WithLedgerID`, `WithParticipantDocument`, `WithRelatedPartyDocument`, `WithBankingDetailsBranch`, `WithBankingDetailsAccount`, `WithBankingDetailsIBAN`, `WithRelatedPartyRole`
- `(*ListOptions).Clone`, `.NextPage`, `.ToQueryParams`, `.Validate`
- `models.NextPageOptionsFrom`
- `(*Pagination).HasNextPage()`, `.NextPageOptions()`, `.HasPrevPage()`, `.PrevPageOptions()`, `.CurrentPage()`, `.TotalPages()`

Also retired: legacy v2 list wrapper types `models.Accounts`, `models.AccountFilter`, `models.ListAccountInput`, `models.ListAccountResponse`, `models.Operations`, `models.OperationsResponse`. Every list response now rides the unified `models.ListResponse[T]` generic.

### What replaces it

Every endpoint has a typed opts struct embedding one of two base structs:

| Pagination shape | Endpoints | Base struct |
| --- | --- | --- |
| Page-based | Organizations, Ledgers, Assets, Portfolios, Segments, Accounts, AccountTypes, Balances, Holders, Aliases | `models.PageListOpts{Limit, Page, SortDirection, StartDate, EndDate}` |
| Cursor-based | Transactions, Operations, OperationRoutes, TransactionRoutes, AssetRates | `models.CursorListOpts{Limit, Cursor, SortDirection, StartDate, EndDate}` |

Each endpoint then attaches a typed `Filters` sub-struct exposing only the fields that endpoint actually honors:

```go
type AccountsListOpts struct {
    PageListOpts                   // Limit, Page, SortDirection, StartDate, EndDate
    Filters AccountsFilters        // Type, Status, AssetCode, HolderID, ...
}
```

### Migration patterns

**Page-based listing with filters:**

```go
// v2
opts := models.NewListOptions().
    WithLimit(20).
    WithFilter("status", "ACTIVE").
    WithFilter("asset_code", "USD")

page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)

// v3
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 20},
    Filters: models.AccountsFilters{
        Status:    "ACTIVE",
        AssetCode: "USD",
    },
}

page, err := c.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)
```

**Cursor-based listing:**

```go
// v2
opts := models.NewListOptions().
    WithLimit(50).
    WithCursor(savedCursor)

page, err := c.Entity.Transactions.ListTransactions(ctx, orgID, ledgerID, opts)

// v3
opts := models.TransactionsListOpts{
    CursorListOpts: models.CursorListOpts{
        Limit:  50,
        Cursor: savedCursor,
    },
}

page, err := c.Transactions.ListTransactions(ctx, orgID, ledgerID, opts)
```

> :::warning
> **Compile-time prevention of audit finding 5.5.** v2 let you set `WithPage(5)` on a cursor endpoint and the SDK silently dropped the value, emitting only a stderr warning. In v3, `TransactionsListOpts.Page` doesn't exist — the type system rejects the wrong shape at compile time.
> :::

**Date-range and sort:**

```go
// v2
opts := models.NewListOptions().
    WithLimit(100).
    WithDateRange("2026-01-01", "2026-04-30").
    WithOrderDirection(models.SortDescending)

// v3
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{
        Limit:         100,
        StartDate:     "2026-01-01",
        EndDate:       "2026-04-30",
        SortDirection: models.SortDescending,
    },
}
```

`WithOrderBy` is gone with no replacement. Midaz list endpoints sort server-side by `createdAt` (or the endpoint's natural order); only direction is controllable.

---

## 4. Iteration: `iter.Seq2` replaces manual page loops

v2 required a manual page-fetch loop. v3 provides a `List` / `ListXxxAll` / `ListXxxPages` trio per endpoint, leveraging Go 1.23+ range-over-func.

### The iterator trio

| Method | Returns | Use when |
| --- | --- | --- |
| `ListAccounts` | `*models.ListResponse[Account]` (one page) | You want exactly one page and decide when to advance. |
| `ListAccountsAll` | `iter.Seq2[Account, error]` | You want to consume every item; the SDK handles paging. |
| `ListAccountsPages` | `iter.Seq2[*ListResponse[Account], error]` | You need page-level metadata for checkpointing or stopping mid-page. |

The same trio shape applies to every list endpoint: `ListLedgers` / `ListLedgersAll` / `ListLedgersPages`, `ListTransactions` / `ListTransactionsAll` / `ListTransactionsPages`, and so on.

### Migration patterns

**Drain every account into a function (most common case):**

```go
// v2
opts := models.NewListOptions().WithLimit(100)

for {
    page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)
    if err != nil {
        return err
    }

    for _, account := range page.Items {
        process(account)
    }

    if !page.Pagination.HasNextPage() {
        break
    }

    opts = page.Pagination.NextPageOptions()
}

// v3
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 100},
}

for account, err := range c.Accounts.ListAccountsAll(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return err
    }
    process(account)
}
```

**Paginate with page-level metadata (checkpointing, batching):**

```go
// v3
for page, err := range c.Accounts.ListAccountsPages(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return err
    }
    log.Printf("page=%d items=%d next_cursor=%q",
        page.Pagination.Page, len(page.Items), page.Pagination.NextCursor)

    for _, account := range page.Items {
        process(account)
    }

    if shouldStop(page) {
        break  // SDK aborts in-flight paging cleanly
    }
}
```

**One page at a time (UI pagination):**

```go
// v3
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{Limit: 25, Page: pageNumber},
}

page, err := c.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)
if page.Pagination.HasMore() {
    // Page-based: increment opts.Page; cursor-based: copy page.Pagination.NextCursor.
}
```

### Pagination metadata methods

| v2 | v3 | Notes |
| --- | --- | --- |
| `Pagination.HasNextPage()` | `Pagination.HasMore()` | Now uses `NextCursor` first (definitive for cursor endpoints), then `Total + Limit + Page` arithmetic, then a `Limit == ItemCount` heuristic. Nil-receiver-safe. |
| `Pagination.HasPrevPage()` | `Pagination.HasPrev()` | Nil-receiver-safe. |
| `Pagination.NextPageOptions()` | Build the next opts yourself: increment `opts.Page` (page-based) or copy `page.Pagination.NextCursor` into `opts.Cursor` (cursor-based). The `*All` and `*Pages` iterators do this automatically. |
| `Pagination.PrevPageOptions()` | Same — build it yourself, or use `*Pages` to traverse forward. |
| `Pagination.CurrentPage()` | `Pagination.Page` (field, not method) |
| `Pagination.TotalPages()` | `Pagination.TotalKnown()` returns whether `Total` is populated; compute pages with `(Total + Limit - 1) / Limit` when known. The v2 `TotalPages()` silently returned `1` when `Total` was unknown, producing misleading "Page N of 1" UIs. |

For the full pagination contract, see [docs/pagination.md](pagination.md).

---

## 5. Service access: `c.Entity.X.Y` → `c.X.Y`

v2 routed every service call through the `c.Entity` sub-struct. v3 promotes the embedded `*entities.Entity` so services are reachable directly on the client.

```go
// v2
c.Entity.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
c.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
c.Entity.Organizations.ListOrganizations(ctx, opts)

// v3
c.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
c.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
c.Organizations.ListOrganizations(ctx, opts)
```

The `c.Entity` field still exists in v3 (the embedded pointer remains accessible) so existing code compiles, but `c.X` is the canonical idiom and what every example, godoc, and doc references.

---

## 6. Models: `mmodel.X` type-identity leak retired

v2 had `type Organization = mmodel.Organization` — Go aliases that made `models.Organization` and the server-internal `mmodel.Organization` the *same* type. The SDK's public API thus inherited type identity from the server-internal package, which dragged `github.com/LerianStudio/midaz/v3/pkg/mmodel` into every customer's `go.sum`.

v3 makes 11+ entity families SDK-native. Wire format is identical (JSON tags align byte-for-byte), but the type identity is owned by `models/`.

### What this means in practice

| v2 pattern | v3 replacement |
| --- | --- |
| `import "github.com/LerianStudio/midaz/v3/pkg/mmodel"` | Delete the import. |
| `models.Organization` (was an alias to `mmodel.Organization`) | Same name, now an SDK-owned struct. Same JSON, same fields. |
| `ToMmodelOrganization(...)` / `FromMmodelOrganization(...)` | Deleted. Use `models.Organization` directly. |
| `models.Account` (was alias) | Now SDK-native. |
| `models.Status`, `models.Address` | Now SDK-native. JSON-compatible. |
| `mmodel.AccountingEntries` | Still aliased (see below). |

The 11 SDK-native families: Status, Address, Organization, Ledger, Asset, Portfolio, Segment, AccountType, OperationRoute, TransactionRoute, Account, Balance, Queue (plus their `Create*Input` / `Update*Input` siblings).

The deliberate exception: `models.AccountingEntries` remains aliased to `mmodel.AccountingEntries`. The accounting-entries tree is ~150 lines of nested wire-format detail with strict server-side scenario validation; hand-mirroring it offered no public-API benefit.

### Most callers don't need to do anything

If your code constructed and consumed `models.X` types via field literals — `org := models.Organization{LegalName: "X"}` — your code already compiles in v3. You only need to act if you imported `mmodel` directly, called `ToMmodelX` / `FromMmodelX` adapters, or did type assertions like `o.(*mmodel.Organization)`.

---

## 7. Errors: `MidazError` and alias predicates retired

v2 carried a parallel `*MidazError` type (deprecated since v2 but still working) plus duplicate predicates. v3 deletes both.

### What's deleted

| v2 | v3 |
| --- | --- |
| `*MidazError` type | Use `*pkg/errors.Error`. |
| `NewMidazError(...)` | Use `NewValidationError`, `NewNotFoundError`, etc. |
| `IsPermissionError(err)` | `IsAuthorizationError(err)` (canonical name). |
| `IsAlreadyExistsError(err)` | `IsConflictError(err)` (matches all 409 conflicts including "already exists"). |
| `FormatTransactionError(...)` | `FormatUnifiedTransactionError(...)`. |
| `models.ErrorResponse` (parallel public type) | Wire format is now owned by `entities.parseErrorResponse` and populated into `*errors.Error` via `ErrorFromHTTPResponseWithDetails`. |

### What's new

- **`Error.Retryable() bool`** — the canonical retry-policy source on `*Error`. Use this instead of inspecting the category and writing your own classification.
- **`CategoryAuth`** — new umbrella category for any authentication-or-authorization error. Discriminate 401 vs 403 via `Code` when needed.
- **`IsAuthError(err)`** — matches both 401 and 403.
- **`IsConfigurationError(err)`** — matches construction-time configuration errors. Use this to distinguish setup mistakes from runtime API failures.
- **`IsUnprocessableError(err)`** — matches 422 unprocessable entity responses.

### Migration patterns

**Branching on errors:**

```go
// v2
if err != nil {
    switch {
    case sdkerrors.IsPermissionError(err):
        return nil, fmt.Errorf("permission denied: %w", err)
    case sdkerrors.IsAlreadyExistsError(err):
        return nil, fmt.Errorf("already exists: %w", err)
    }
}

// v3
if err != nil {
    switch {
    case sdkerrors.IsAuthorizationError(err):
        return nil, fmt.Errorf("permission denied: %w", err)
    case sdkerrors.IsConflictError(err):
        return nil, fmt.Errorf("already exists: %w", err)
    }
}
```

**Retry classification:**

```go
// v2 — handwritten classification
shouldRetry := errors.Is(err, sdkerrors.ErrTimeout) ||
               errors.Is(err, sdkerrors.ErrRateLimit) ||
               sdkerrors.IsNetworkError(err)

// v3 — canonical via Error.Retryable()
var sdkErr *sdkerrors.Error
if errors.As(err, &sdkErr) && sdkErr.Retryable() {
    // Apply your retry logic.
}
```

For the full error contract, see [docs/errors.md](errors.md).

---

## 8. Configuration: option surface canonicalized

v2 had ~120 `With*` options across `client`, `config`, `entities`, `pkg/retry`, `pkg/observability`, and `pkg/performance`. v3 reduces to ~60 with a clear separation: user-facing options on `midaz.With*`, internal layer on `pkg/config.With*`, per-request overrides via `sdkctx.With*`.

### Retry options

| v2 | v3 |
| --- | --- |
| `client.WithRetries(maxRetries int, initialDelay, maxDelay time.Duration)` | `midaz.WithRetryOptions(retry.WithMaxRetries(N), retry.WithInitialDelay(d1), retry.WithMaxDelay(d2))` |
| `client.DisableRetries()` | `midaz.WithoutRetries()` (soft-disable; a later `WithRetryOptions(retry.WithMaxRetries(N))` re-enables) |
| `client.WithRetries(false)` | `midaz.WithoutRetries()` |
| `pkg/retry.WithNoRetry()`, `WithHTTPNoRetry()` | Deleted. Use `midaz.WithoutRetries()`. |
| `config.WithRetries(bool)` | Deleted. Use `WithMaxRetries(0)`. |
| `config.WithRetryConfig(maxRetries int, minWait, maxWait time.Duration)` | Deleted. Use `config.WithMaxRetries`, `config.WithRetryWaitMin`, `config.WithRetryWaitMax` separately. |

```go
// v2
c, err := client.New(
    client.WithRetries(5, 200*time.Millisecond, 10*time.Second),
)

// v3
c, err := midaz.New(
    midaz.WithAnonymous(),
    midaz.WithRetryOptions(
        retry.WithMaxRetries(5),
        retry.WithInitialDelay(200*time.Millisecond),
        retry.WithMaxDelay(10*time.Second),
    ),
)
```

### Observability options

| v2 | v3 |
| --- | --- |
| `client.WithObservability(tracing, metrics, logging bool)` | `midaz.WithObservabilityOptions(observability.WithComponentEnabled(t, m, l))` |
| `client.WithCollectorEndpoint(string)` | `midaz.WithObservabilityOptions(observability.WithCollectorEndpoint(...))` |
| `client.WithObservabilityOptions(...)` | `midaz.WithObservabilityOptions(...)` (renamed wrapper, same shape) |
| `client.WithObservabilityProvider(p)` | `midaz.WithObservabilityProvider(p)` |

> :::warning
> **Replacement semantics, not merge.** `WithObservabilityOptions` and `WithObservabilityProvider` REPLACE any previously installed provider. Subsequent calls likewise replace. To start from a known set of defaults, include `observability.WithDevelopmentDefaults` or `observability.WithProductionDefaults` as the first item in the chain.
> :::

### Tenant ID

| v2 | v3 |
| --- | --- |
| `client.WithTenantID(string)` | `midaz.WithTenantID(string)` (unchanged) |
| `entities.WithDefaultTenantID(...)` | Deleted. Use `midaz.WithTenantID`. |
| `config.WithTenantID(...)` | Deleted. Use `midaz.WithTenantID` or set `MIDAZ_TENANT_ID` and load via `config.FromEnvironment`. |
| `entities.WithTenantID(ctx, id)` (per-request) | `sdkctx.WithRequestTenantID(ctx, id)` — renamed for clarity. |

### Idempotency

| v2 | v3 |
| --- | --- |
| `entities.WithIdempotencyKey(ctx, key)` | `sdkctx.WithIdempotencyKey(ctx, key)` — moved to the dedicated `pkg/sdkctx` package. |
| `entities.WithoutAutoIdempotency(ctx)` | `sdkctx.WithoutAutoIdempotency(ctx)` |

The auto-generation contract is unchanged: SDK auto-generates an `X-Idempotency` UUID for unsafe HTTP methods (POST/PUT/PATCH/DELETE) when no key is set. `MIDAZ_IDEMPOTENCY=false` in the environment disables this globally.

For the full configuration map, see [docs/configuration.md](configuration.md).

---

## 9. Logging: `*slog.Logger` is the canonical surface

v2 exposed a bespoke `observability.Logger` interface as the primary logger. v3 makes `*slog.Logger` (the Go stdlib idiom since 1.21) the canonical surface; the OTel-correlated logger lives only behind the observability provider.

| v2 | v3 |
| --- | --- |
| `Client.Logger() observability.Logger` | `Client.Logger() *slog.Logger` — discard handler by default; opt in with `WithLogger`. |
| `client.WithLogger(observability.Logger)` (if you used it) | `midaz.WithLogger(*slog.Logger)`. |
| Setting `MIDAZ_DEBUG=true` to upgrade the logger | `MIDAZ_DEBUG=true` still works — but only when no `WithLogger` was passed (user-supplied loggers always win). |
| Bespoke OTel-correlated logger | `c.GetObservabilityProvider().Logger()` — kept for span-aware logging within SDK internals. |

```go
// v3 — JSON to stdout via stdlib slog
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
c, _ := midaz.New(
    midaz.WithLogger(logger),
    midaz.WithAnonymous(),
)

// v3 — slow-call warnings on the configured logger
c, _ := midaz.New(
    midaz.WithLogger(logger),
    midaz.WithSlowCallThreshold(2*time.Second),
    midaz.WithAnonymous(),
)
```

For zap, zerolog, or charmbracelet/log integration, see [docs/logging.md](logging.md).

---

## 10. Update inputs: `input any` typed out

v2's `UpdateTransaction` and `UpdateTransactionOperation` accepted `input any` — no compile-time guarantee that the caller passed the right type. v3 takes typed pointers.

```go
// v2
err := c.Entity.Transactions.UpdateTransaction(ctx, orgID, ledgerID, txID, anyInput)

// v3
err := c.Transactions.UpdateTransaction(ctx, orgID, ledgerID, txID, &models.UpdateTransactionInput{
    Description: "Updated transaction",
})
```

Empty update payloads now return a typed validation error from `Validate()` instead of being silently sent to the API. If you relied on the silent no-op, audit the call site — chances are it was a bug.

---

## 11. Soft-delete and hard-delete via context

v2 mixed three patterns: boolean `hardDelete` parameters on some methods, `includeDeleted bool` on others, query-string flags on a third group. v3 routes both through `sdkctx`:

```go
// v2 — boolean parameter
err := c.Entity.Holders.DeleteHolder(ctx, orgID, holderID, true /* hardDelete */)

// v3 — context flag
ctx = sdkctx.WithHardDelete(ctx, true)
err := c.Holders.DeleteHolder(ctx, orgID, holderID)

// v2 — boolean parameter
holder, err := c.Entity.Holders.GetHolder(ctx, orgID, holderID, true /* includeDeleted */)

// v3 — context flag
ctx = sdkctx.WithIncludeDeleted(ctx, true)
holder, err := c.Holders.GetHolder(ctx, orgID, holderID)
```

This unifies the surface: every "soft-delete vs hard-delete vs include-deleted" toggle is a context flag, never a parameter on the method signature.

---

## 12. Other deletions worth noting

A non-exhaustive list of smaller surface that's gone:

- `client.UseAllAPIs()`, `client.UseEntityAPI()`, `client.UseEntity()` — services are always initialized in v3.
- `entities.NewXxxEntity(...)` constructors — services are always initialized via `setupEntity()` and never constructed directly by callers.
- `entities.WithContext`, `entities.WithObservability`, `entities.WithPluginAuth` — moved or renamed (see Track 2).
- `entities.New(baseURL string, ...)`, `entities.NewWithServiceURLs(...)` — internal-only construction is now via `entities.NewEntityWithConfig`.
- `pkg/performance.WithJSONIterator`, `pkg/concurrent.WithWaitGroup` — were no-ops.
- `pkg/performance.WithMaxIdleConnsPerHost` (Option variant; the TransportOption variant is kept).
- `pkg/pagination` package — its surface was moved to `models` and `entities`.
- `MIDAZ_ENABLE_RETRIES` environment variable — duplicated `MIDAZ_MAX_RETRIES`.
- Implicit env reads from entity constructors (`MIDAZ_DEBUG`, `MIDAZ_USER_AGENT`, `MIDAZ_IDEMPOTENCY`, `MIDAZ_MAX_RETRIES` — all now require `config.FromEnvironment()` to be honored).

---

## 13. Things that did NOT change

If you're paranoid about migration cost, here's the surface that's stable v2 → v3:

- The wire format. JSON request/response shapes are byte-compatible.
- Service method names: `CreateAccount`, `GetTransaction`, `ListLedgers`, etc.
- Resource hierarchy: organizations contain ledgers; ledgers contain accounts/assets/balances/transactions.
- Idempotency contract: SDK auto-generates `X-Idempotency` for unsafe methods unless told otherwise.
- Retry defaults: 3 attempts, 1s..30s exponential backoff, 408/429/500/502/503/504 retryable.
- Observability: OpenTelemetry first-class, OTLP gRPC for collector export.
- Default timeout: 60s.
- Environment-variable contract via `config.FromEnvironment()` (with the four removals listed above).

---

## 14. Sunset timeline

- **T-0 (v3.0.0 release):** v3 GA. v2 enters maintenance mode (bug fixes only). This guide covers every breaking change.
- **T+90 days:** v2 reaches end-of-feature-life. Security patches only.
- **T+365 days:** v2 fully sunset. No further updates.

The `/v2` and `/v3` modules continue to coexist on Go module proxies indefinitely — sunset means no SDK-team commits, not module deletion.

---

## Where to look next

- [docs/auth.md](auth.md) — authentication walkthrough.
- [docs/configuration.md](configuration.md) — full configuration map (four surfaces, precedence rules).
- [docs/pagination.md](pagination.md) — pagination contract (List trio, page-vs-cursor, iterator semantics).
- [docs/errors.md](errors.md) — error type, categories, predicates, retry classification.
- [docs/examples.md](examples.md) — runnable examples per domain.
- [docs/v3-dx-plan.md](v3-dx-plan.md) — design rationale (audit findings, decision log).
- `examples/01-hello-world/` — the smallest possible v3 demo.
- `examples/03-end-to-end/` — full resource hierarchy walk.

If you hit a v2 pattern not covered here, file an issue with the v2 code and what error you got at compile/run time. We'll either add it to this guide or fix the gap in the SDK.
