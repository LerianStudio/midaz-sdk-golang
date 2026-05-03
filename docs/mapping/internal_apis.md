# Midaz Go SDK internal API map

This map is for SDK maintainers. It describes the implementation structure behind the public API. SDK consumers should prefer [external_apis.md](./external_apis.md).

## Runtime architecture

The current SDK is organized around a root client and an entity layer:

1. `client.Client` owns configuration, observability, lifecycle, and optional API surface initialization.
2. `pkg/config.Config` resolves service URLs, Access Manager settings, retry/debug options, HTTP client, and observability provider.
3. `entities.Entity` exposes the service interfaces used by consumers.
4. Private entity implementations such as `accountsEntity`, `transactionsEntity`, and `holdersEntity` translate service methods into HTTP requests.
5. `entities.HTTPClient` handles request construction, authentication headers, tenant/idempotency headers, tracing propagation, retry behavior, debug logging, and response/error conversion.
6. `models` contains public request/response structures, Midaz model aliases, list options, pagination metadata, and builder helpers.

The SDK does not currently use the older `apiClient`, `httpClient`, or per-resource `organizationClient` style architecture.

## Root client internals

`client.Client` includes:

- `Entity *entities.Entity` - Set only when `UseAllAPIs`, `UseEntityAPI`, or `UseEntity` is provided.
- `config *config.Config` - Resolved SDK configuration.
- `observability observability.Provider` - Optional tracing, metrics, and logging provider.
- `httpClient *http.Client` - Underlying HTTP client.
- Retry, timeout, debug, and URL override fields used while applying client options.

Entity initialization is gated by the `useEntity` flag. Docs and examples that access `c.Entity` must create the client with `client.UseAllAPIs()` or `client.UseEntityAPI()`.

## Configuration flow

Configuration is explicit:

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := client.New(
    client.WithConfig(cfg),
    client.UseAllAPIs(),
)
```

`config.FromEnvironment()` reads:

- `MIDAZ_ENVIRONMENT`
- `MIDAZ_BASE_URL`
- `MIDAZ_ONBOARDING_URL`
- `MIDAZ_TRANSACTION_URL`
- `MIDAZ_CRM_URL`
- `MIDAZ_USER_AGENT`
- `MIDAZ_TIMEOUT`
- `MIDAZ_DEBUG`
- `MIDAZ_MAX_RETRIES`
- `MIDAZ_IDEMPOTENCY`
- `PLUGIN_AUTH_ENABLED`
- `PLUGIN_AUTH_ADDRESS`
- `MIDAZ_CLIENT_ID`
- `MIDAZ_CLIENT_SECRET`

Access Manager configuration uses `auth.AccessManager` and `config.WithAccessManager`. `MIDAZ_AUTH_TOKEN` is not part of `config.FromEnvironment()`.

`MIDAZ_ENVIRONMENT` recomputes default service URLs unless `MIDAZ_BASE_URL` or a service-specific URL has already been set. Explicit service URLs take precedence and are normalized by the entity layer to include `/v1`.

## Service URL model

The entity layer receives a service URL map. The current service keys are:

- `onboarding`
- `transaction`
- `crm`

Ledger resources are split between onboarding and transaction URL aliases for compatibility. CRM resources use the CRM URL and pass organization context via `X-Organization-Id`.

## Entity service implementations

Each service has a public interface and a private implementation type. Method names are explicit (`ListAccounts`, `CreateOrganization`, `CreateTransactionWithDSL`) rather than generic CRUD (`List`, `Create`).

### Ledger API services

- `OrganizationsService` implemented by `organizationsEntity`
- `LedgersService` implemented by `ledgersEntity`
- `AccountsService` implemented by `accountsEntity`
- `AccountTypesService` implemented by `accountTypesEntity`
- `AssetsService` implemented by `assetsEntity`
- `AssetRatesService` implemented by `assetRatesEntity`
- `BalancesService` implemented by `balancesEntity`
- `PortfoliosService` implemented by `portfoliosEntity`
- `SegmentsService` implemented by `segmentsEntity`
- `OperationsService` implemented by `operationsEntity`
- `OperationRoutesService` implemented by `operationRoutesEntity`
- `TransactionRoutesService` implemented by `transactionRoutesEntity`
- `TransactionsService` implemented by `transactionsEntity`
- `MetadataIndexesService` implemented by `metadataIndexesEntity`

### CRM services

- `HoldersService` implemented by `holdersEntity`
- `AliasesService` implemented by `aliasesEntity`

CRM requests set `X-Organization-Id` and use paths under `/holders` and `/aliases`.

## Transport pattern

The shared `entities.HTTPClient` is responsible for the transport cross-cutting concerns:

- Adds authorization after Access Manager resolves a token.
- Adds default tenant ID when configured.
- Adds idempotency keys from `entities.WithIdempotencyKey(ctx, key)`.
- Injects OpenTelemetry trace context and baggage into outbound HTTP headers when observability is enabled.
- Applies retry behavior for retryable responses and transient network failures.
- Avoids retrying unsafe methods unless `X-Idempotency` is present.
- Converts HTTP failures into `pkg/errors` structured errors.
- Emits debug logs when `MIDAZ_DEBUG=true` or debug options are enabled.

## Request path construction

The SDK currently builds endpoint paths inside each entity implementation instead of using a central endpoint registry.

Important path groups:

- Organizations: `/organizations`, `/organizations/{id}`
- Ledgers: `/organizations/{organizationID}/ledgers`, `/organizations/{organizationID}/ledgers/{ledgerID}`
- Accounts: `/organizations/{organizationID}/ledgers/{ledgerID}/accounts`, `/accounts/{id}`, `/accounts/alias/{alias}`, `/accounts/external/{assetCode}`
- Account balances: `/accounts/{accountID}/balances`, `/balances/{balanceID}`, balance history endpoints
- Assets: `/organizations/{organizationID}/ledgers/{ledgerID}/assets`
- Asset rates: asset-rate endpoints under organization/ledger scope, including asset-code filtered listing
- Transactions: `/transactions/json`, `/transactions/dsl`, `/transactions/{id}`, `/transactions/{id}/commit`, `/transactions/{id}/cancel`, `/transactions/{id}/revert`, `/transactions/inflow`, `/transactions/outflow`, `/transactions/annotation`
- Operations: account-scoped operation listing plus transaction operation update paths
- Routes: operation route and transaction route endpoints under organization/ledger scope
- Metadata indexes: `/settings/metadata-indexes`
- CRM holders: `/holders`, `/holders/{holderID}`
- CRM aliases: `/aliases`, `/holders/{holderID}/aliases`, `/holders/{holderID}/aliases/{aliasID}/related-parties/{relatedPartyID}`

Supported count paths use `HEAD` and read `X-Total-Count`:

| Resource | Method | Path |
| --- | --- | --- |
| Organizations | `GetOrganizationsMetricsCount` | `/organizations/metrics/count` |
| Ledgers | `GetLedgersMetricsCount` | `/organizations/{organizationID}/ledgers/metrics/count` |
| Assets | `GetAssetsMetricsCount` | `/organizations/{organizationID}/ledgers/{ledgerID}/assets/metrics/count` |
| Portfolios | `GetPortfoliosMetricsCount` | `/organizations/{organizationID}/ledgers/{ledgerID}/portfolios/metrics/count` |
| Segments | `GetSegmentsMetricsCount` | `/organizations/{organizationID}/ledgers/{ledgerID}/segments/metrics/count` |
| Accounts | `GetAccountsMetricsCount` | `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/metrics/count` |
| Transactions | `GetTransactionsMetricsCount` | `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/metrics/count` |

`doCountRequest` returns an internal SDK error when `X-Total-Count` is missing or cannot be parsed as an integer. `GetAccountTypesMetricsCount` is a deprecated compatibility-only method. It validates IDs and returns a validation error because Midaz Ledger does not expose account type count metrics.

## Model compatibility layer

Several SDK inputs wrap Midaz `mmodel` types to preserve the public SDK package path while using Midaz model contracts internally. Prefer fluent constructors in examples because wrapper fields can differ from direct composite literal expectations.

Common builders:

- `models.NewCreateOrganizationInput(legalName, legalDocument)`
- `models.NewUpdateOrganizationInput()`
- `models.NewCreateAssetInput(name, code)`
- `models.NewUpdateAssetInput()`
- `models.NewCreateAccountInput(name, assetCode, accountType)`
- `models.NewCreateTransactionInput(assetCode, amount)`
- `models.NewAssetRateListOptions()`

## List options and pagination internals

`models.ListOptions` fields:

```go
type ListOptions struct {
    Limit            int
    Offset           int
    Filters          map[string]string
    OrderBy          string
    OrderDirection   string
    Page             int
    Cursor           string
    StartDate        string
    EndDate          string
    AdditionalParams map[string]string
}
```

Query serialization rules:

- `limit` is always emitted and entity list requests are capped by `models.MaxLimit` (`100`).
- `Offset` is retained as compatibility input for older callers. Current Midaz endpoints should be documented and exercised through `page`, `limit`, and `cursor` where supported; do not describe `offset` as a supported Midaz wire parameter.
- `Page` is emitted as `page` when set and is the preferred page-based control.
- `Cursor` is emitted as `cursor` when set.
- `Filters` are emitted as query parameters by key.
- `OrderBy` is retained but not emitted by common serialization.
- `OrderDirection` is emitted as `sort_order`.
- `StartDate`, `EndDate`, and `AdditionalParams` are emitted when set.
- Transactions remove `page` when cursor pagination is used.

`models.ListResponse[T]` contains `Items []T` and `Pagination models.Pagination`. JSON unmarshalling supports both current top-level pagination fields and legacy nested `pagination` payloads.

`Pagination.TotalPages()` depends on `Pagination.Total`. Current Midaz responses commonly omit `total`, so traversal logic should use `HasNextPage`, `NextPageOptions`, and cursor metadata instead of assuming total pages are available.

Pagination behavior differs by API family:

| API family | Internal behavior |
| --- | --- |
| Ledger page-based resources | Common serialization sends `page`, `limit`, filters, and `sort_order`. |
| Ledger cursor-aware resources | Transactions use cursor-aware handling and remove page-style parameters when `Cursor` is set. |
| CRM holders and aliases | CRM services use page-based list calls plus CRM-specific filters stored in `AdditionalParams`. |

## Error model internals

The core SDK error type is `*errors.Error` in `pkg/errors`:

```go
type Error struct {
    Category   ErrorCategory
    Code       ErrorCode
    APICode    string
    Title      string
    Message    string
    Operation  string
    Resource   string
    ResourceID string
    EntityType string
    Fields     []string
    Details    map[string]any
    StatusCode int
    RequestID  string
    Err        error
}
```

It implements `error`, `Unwrap`, and `Is`, so callers can use `errors.Is`, `errors.As`, and SDK helper functions.

Standard sentinel errors include:

- `ErrValidation`
- `ErrAuthentication`
- `ErrPermission`
- `ErrNotFound`
- `ErrAlreadyExists`
- `ErrIdempotency`
- `ErrRateLimit`
- `ErrTimeout`
- `ErrCancellation`
- `ErrInternal`
- `ErrUnprocessable`
- `ErrInsufficientBalance`
- `ErrAccountEligibility`
- `ErrAssetMismatch`

Midaz wire error envelopes may contain `code`, `title`, `message`, `entityType`, and `fields`. CRM error responses may contain `err`. Preserve the wire `code` separately from the SDK-normalized `Code`, and keep expanded envelope data in `APICode`, `Title`, `EntityType`, `Fields`, and `Details` when available.

## Observability internals

The SDK observability package wraps OpenTelemetry and exposes a `Provider` interface with:

- `Tracer()`
- `Meter()`
- `Logger()`
- `Shutdown(ctx)`
- `IsEnabled()`

Entity HTTP requests inject propagation headers through `observability.InjectContext`. Server-side code can extract incoming context with `observability.ExtractContext` or use the HTTP middleware helpers.

Collector endpoints are passed to the OTLP gRPC exporter as `host:port` values, for example `localhost:4317`.

## Retry internals

Retry behavior is implemented in `pkg/retry` and integrated into `entities.HTTPClient`.

Root-client retry defaults come from `pkg/config` and are applied to entity HTTP clients during setup:

- Maximum retries: 3
- Initial delay: 1s
- Maximum delay: 30s
- Backoff factor: 2.0
- Jitter factor: 0.25
- Retryable status codes: 408, 429, 500, 502, 503, 504

Unsafe requests are retried only when an idempotency key is present.

## Maintainer checklist

When changing public API shape:

- Update service interfaces and private implementations together.
- Update `README.md`, `docs/README.md`, `docs/examples.md`, and `docs/mapping/external_apis.md`.
- Update this internal map when transport, config, retry, observability, or service URL behavior changes.
- Run targeted tests for changed packages and prefer `make ci` before PRs.
