# Midaz Go SDK internal API map

This map is for SDK maintainers. It describes the implementation structure behind the public API. SDK consumers should prefer [external_apis.md](./external_apis.md).

## Runtime architecture

The current SDK is organized around a root client and an entity layer:

1. `midaz.Client` owns configuration, observability, lifecycle, and service initialization.
2. `pkg/config.Config` resolves service URLs, Access Manager settings, retry/debug options, HTTP client, and observability provider.
3. `entities.Entity` exposes the accessors used by consumers.
4. Most resources are concrete `*xFacade` structs over the generated plane client; only `Balances`, `Operations`, and `Aliases` remain interface-backed private entities (`balancesEntity`, `operationsEntity`, `aliasesEntity`) that translate service methods into HTTP requests.
5. `entities.HTTPClient` handles request construction, authentication headers, idempotency headers, tracing propagation, retry behavior, debug logging, and response/error conversion.
6. `models` contains public request/response structures, Midaz model aliases, list options, pagination metadata, and builder helpers.

The SDK does not currently use the older `apiClient`, `httpClient`, or per-resource `organizationClient` style architecture.

### Facade layer

The 13 primary ledger-plane accessors (`Organizations`, `Ledgers`, `Accounts`, `AccountTypes`, `Assets`, `AssetRates`, `Portfolios`, `Segments`, `OperationRoutes`, `TransactionRoutes`, `MetadataIndexes`, `Transactions`, `Holders`) are concrete facade structs (`*accountsFacade`, `*organizationsFacade`, ...) exposing generic CRUD (`List`/`Get`/`Create`/`Update`/`Delete`/`All`/`Pages`/`Count`) directly over the generated ledger plane client (`internal/genledger.ClientWithResponses`), bypassing the legacy per-service `entities.HTTPClient` path entirely. `Balances`, `Operations`, and `Aliases` have no facade yet and remain interface-backed (`BalancesService`, `OperationsService`, `AliasesService`) with explicit method names, wired over the shared legacy `entities.HTTPClient`.

## Root client internals

`midaz.Client` includes:

- `Entity *entities.Entity` - Initialized by `midaz.New(...)` when configuration validates; promoted service fields are also available directly on the client.
- `config *config.Config` - Resolved SDK configuration.
- `observability observability.Provider` - Optional tracing, metrics, and logging provider.
- `customRetryPolicy func(*http.Response, error) bool` - Optional retry predicate propagated to the entity HTTP client.
- `ctx context.Context` - Client base context used by client-level helpers and observability setup.

HTTP client ownership lives in `pkg/config.Config` and `entities.HTTPClient`, not directly on `midaz.Client`.

Entity initialization happens inside `midaz.New(...)` with an explicit auth posture such as `midaz.WithAccessManager(...)` or `midaz.WithAnonymous()`.

## Configuration flow

Configuration is explicit:

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(
    midaz.WithConfig(cfg),
    midaz.WithAnonymous(),
)
```

`config.FromEnvironment()` reads:

- `MIDAZ_ENVIRONMENT`
- `MIDAZ_BASE_URL`
- `MIDAZ_LEDGER_URL`
- `MIDAZ_CRM_URL`
- `MIDAZ_TIMEOUT`
- `MIDAZ_DEBUG`
- `MIDAZ_MAX_RETRIES`
- `MIDAZ_IDEMPOTENCY`
- `MIDAZ_ERROR_EXPOSE_BODY`
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

The `onboarding` and `transaction` keys are internal path-dispatch labels for Ledger API resources. Both keys are populated from the single public knob (`WithLedgerURL` / `MIDAZ_LEDGER_URL`) — the dual keys exist for per-service routing inside the entity layer, not as separate user-facing endpoints. CRM resources use the CRM URL and pass organization context via `X-Organization-Id`.

## Entity service implementations

Most ledger resources are served by the facade accessors described in
[external_apis.md](./external_apis.md) — concrete `*xFacade` structs exposing
generic CRUD (`List`/`Get`/`Create`/...) directly over the generated plane
client, with no separate public interface or private implementation type. Only
the legacy trio (Balances, Operations, Aliases) is still interface-backed with a
private implementation and explicit method names (`ListBalances`, etc.).

### Ledger API services

Interface-backed (legacy trio members on the ledger/transaction plane):

- `BalancesService` implemented by `balancesEntity`
- `OperationsService` implemented by `operationsEntity`

All other ledger resources (organizations, ledgers, accounts, account types,
assets, asset rates, portfolios, segments, operation routes, transaction routes,
transactions, metadata indexes) are facade-only — see [external_apis.md](./external_apis.md).

### CRM services

- `AliasesService` implemented by `aliasesEntity`

CRM requests set `X-Organization-Id` and use paths under `/aliases` and holder-scoped `/holders/{holderID}/aliases`. Tenant scope comes from Access Manager/JWT claims; the shared HTTP client does not add `X-Tenant-ID`.

## Transport pattern

The shared `entities.HTTPClient` is responsible for the transport cross-cutting concerns:

- Adds authorization after Access Manager resolves a token.
- Adds idempotency keys from `sdkctx.WithIdempotencyKey(ctx, key)`.
- Injects OpenTelemetry trace context and baggage into outbound HTTP headers when observability is enabled.
- Applies retry behavior for retryable responses and transient network failures.
- Avoids retrying unsafe methods unless `X-Idempotency` is present.
- Converts HTTP failures into `pkg/errors` structured errors.
- Attaches raw, unredacted, truncated upstream 4xx/5xx response bodies to structured errors when error body exposure is enabled.
- Emits debug logs when `MIDAZ_DEBUG=true` or debug options are enabled.

## Request path construction

The SDK currently builds endpoint paths inside each entity implementation instead of using a central endpoint registry.

Important path groups:

- Organizations: `/organizations`, `/organizations/{id}`
- Ledgers: `/organizations/{organizationID}/ledgers`, `/organizations/{organizationID}/ledgers/{ledgerID}`
- Ledger settings: `/organizations/{organizationID}/ledgers/{ledgerID}/settings` with `GET` and `PATCH` for `accounting.validateAccountType` and `accounting.validateRoutes`.
- Accounts: `/organizations/{organizationID}/ledgers/{ledgerID}/accounts`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/alias/{alias}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/external/{assetCode}`
- Balances: `/organizations/{organizationID}/ledgers/{ledgerID}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/balances/{balanceID}`, `/organizations/{organizationID}/ledgers/{ledgerID}/balances/{balanceID}/history?date={date}`
- Account balances: `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/balances/history?date={date}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/alias/{alias}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/external/{assetCode}/balances`
- Assets: `/organizations/{organizationID}/ledgers/{ledgerID}/assets`
- Asset rates: `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates`, `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates/{externalID}`, and `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates/from/{assetCode}` using cursor filters (`to`, `limit`, `start_date`, `end_date`, `sort_order`, `cursor`).
- Transactions: `/organizations/{organizationID}/ledgers/{ledgerID}/transactions`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/json`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}/commit`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}/cancel`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}/revert`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/inflow`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/outflow`, `/organizations/{organizationID}/ledgers/{ledgerID}/transactions/annotation`
- Operations: account-scoped reads use `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/operations` and `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/operations/{operationID}`. Updates are transaction-scoped through `PATCH /organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}/operations/{operationID}`.
- Routes: operation route endpoints use `/organizations/{organizationID}/ledgers/{ledgerID}/operation-routes`; transaction route endpoints use `/organizations/{organizationID}/ledgers/{ledgerID}/transaction-routes`.
- Metadata indexes: list uses `/settings/metadata-indexes` with optional `entity_name`; create uses `/settings/metadata-indexes/entities/{entityName}`; delete uses `/settings/metadata-indexes/entities/{entityName}/key/{metadataKey}`. The list endpoint returns a raw `[]MetadataIndex` slice, not a paginated `ListResponse`.
- CRM aliases: `/aliases`, `/holders/{holderID}/aliases`, `/holders/{holderID}/aliases/{aliasID}`, `/holders/{holderID}/aliases/{aliasID}/related-parties/{relatedPartyID}`

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

`doCountRequest` returns an internal SDK error when `X-Total-Count` is missing, blank, non-integer, negative, or overflowing. Account types do not expose a metrics-count method because the Midaz Ledger API does not provide that endpoint for account types.

## Model compatibility layer

Several SDK inputs wrap Midaz `mmodel` types to preserve the public SDK package path while using Midaz model contracts internally. Prefer fluent constructors in examples because wrapper fields can differ from direct composite literal expectations.

Common builders:

- `models.NewCreateOrganizationInput(legalName, legalDocument)`
- `models.NewUpdateOrganizationInput()`
- `models.NewCreateLedgerInput(name)`
- `models.NewUpdateLedgerInput()`
- `models.NewUpdateLedgerSettingsInput()`
- `models.NewCreateAccountInput(name, assetCode, accountType)`
- `models.NewUpdateAccountInput()`
- `models.NewCreateAccountTypeInput(name, keyValue)`
- `models.NewUpdateAccountTypeInput()`
- `models.NewCreateBalanceInput(key)` with `WithAllowSending`, `WithAllowReceiving`, `WithDirection`, and `WithSettings`.
- `models.NewCreateAssetInputWithType(name, code, assetType)`
- `models.NewCreateAssetInput(name, code)` - Deprecated compatibility builder; callers must set type with `WithType` before sending.
- `models.NewUpdateAssetInput()`
- `models.NewCreatePortfolioInput(entityID, name)`
- `models.NewUpdatePortfolioInput()`
- `models.NewCreateSegmentInput(name)`
- `models.NewUpdateSegmentInput()`
- `models.NewCreateTransactionInput(assetCode, amount)` - Must include `send.source` and `send.distribute` before sending, either through `WithSend(...)` or legacy operation adaptation. Unsafe SDK requests receive an auto-generated `X-Idempotency` header by default; set `IdempotencyKey` or use `sdkctx.WithIdempotencyKey` when the caller needs a stable key or has disabled auto-idempotency.
- `models.NewCreateInflowInput(assetCode, value, distribute)` - Requires a non-empty `distribute.to` payload.
- `models.NewCreateOutflowInput(assetCode, value, source)` - Requires a non-empty `source.from` payload.
- `models.NewCreateAnnotationInput(description, send...)` - `send` is optional. Omit it for metadata-only annotation transactions, or pass it for backend deployments that still require a send payload.
- `models.NewCreateOperationRouteInput(title, description, operationType)`
- `models.NewUpdateOperationRouteInput()`
- `models.NewCreateTransactionRouteInput(title, description, operationRouteIDs)`
- `models.NewUpdateTransactionRouteInput()`
- `models.NewCreateMetadataIndexInput(metadataKey)` with `WithUnique` and `WithSparse`.
- `models.NewCreateAssetRateInput(from, to, rate)` with `WithScale`, `WithSource`, `WithTTL`, `WithExternalID`, and `WithMetadata`.
- `models.AssetRatesListOpts` with embedded `CursorListOpts{Limit, Cursor, SortDirection, StartDate, EndDate}`, `Filters.To`, and `ToQueryParams`.
- `models.NewCreateHolderInput(holderType, name, document)` with `WithExternalID`, `WithAddresses`, `WithContact`, `WithNaturalPerson`, `WithLegalPerson`, and `WithMetadata`.
- `models.NewUpdateHolderInput()` with field setters and `WithNullFields` / `WithNullField` for explicit JSON null removals. Empty holder updates are rejected by the SDK.
- `models.NewCreateAliasInput(ledgerID, accountID)` with `WithMetadata`, `WithBankingDetails`, `WithRegulatoryFields`, and `WithRelatedParties`.
- `models.NewUpdateAliasInput()` with field setters and `WithNullFields` for explicit JSON null removals. Repeated `WithRelatedParties` calls replace the in-builder related-party list; empty alias updates are rejected by the SDK.

## List options and pagination internals

v4 deleted the old `models.ListOptions` mega-struct. List methods now accept endpoint-specific option structs embedding either `models.PageListOpts` or `models.CursorListOpts`; wrong-shape pagination does not compile.

Query serialization rules:

- `Limit` serializes as `limit` and entity list requests are capped by `models.MaxLimit` (`100`).
- Page-based opts serialize `Page` as `page`.
- Cursor-based opts serialize `Cursor` as `cursor` and never emit `page`.
- Entity-specific filter structs serialize only fields valid for that endpoint.
- `SortDirection` serializes as `sort_order`.
- Date ranges serialize as `start_date` and `end_date` where supported.

`models.ListResponse[T]` contains `Items []T` and `Pagination models.Pagination`. JSON unmarshalling supports both current top-level pagination fields and legacy nested `pagination` payloads. After unmarshalling, `Pagination.ItemCount` is set from the decoded item count so traversal heuristics can detect full pages even when the server omits `total`.

`models.Pagination` exposes `HasMore()`, `HasPrev()`, and `TotalKnown()` as the canonical traversal helpers. `HasMore()` prefers `NextCursor` for cursor endpoints, falls back to `Total` arithmetic when a total is present, and finally uses a full-page heuristic (`ItemCount >= Limit`) for page endpoints that omit totals. Callers that need a page count must compute it only when `TotalKnown()` is true and `Limit > 0`.

Internal iterator methods advance by copying typed opts and setting either `Page++` for page-based endpoints or `Cursor = page.Pagination.NextCursor` for cursor-based endpoints.

Pagination behavior differs by API family:

| API family | Internal behavior |
| --- | --- |
| Ledger page-based resources | Common serialization sends `page`, `limit`, filters, and `sort_order`. |
| Ledger cursor-based resources | Transactions, operations, operation routes, transaction routes, and asset rates advance with `Pagination.NextCursor`; typed opts never emit page-style parameters. |
| CRM aliases | Legacy CRM plane. Page-based list calls: the iterator advances `Page++` and stops on `!HasMore()`. Organization is sent as the `X-Organization-Id` header. |
| CRM holders | Ledger plane (re-homed in v4). Cursor-based: `HoldersListOpts` embeds `CursorListOpts`, so `Cursor` seeds/resumes pagination and `Pages`/`All` inject the response `next_cursor` as a `cursor` query param, stopping on an empty cursor. Dates are rejected (`ValidateCursorListOptsNoDates`); the facade never emits `page`. Organization is a path segment, not a header. |

## Error model internals

The core SDK error type is `*errors.Error` in `pkg/errors`:

```go
type Error struct {
    Category                  ErrorCategory
    Code                      ErrorCode
    APICode                   string
    Title                     string
    Message                   string
    Operation                 string
    Resource                  string
    ResourceID                string
    EntityType                string
    Fields                    []string
    Details                   map[string]any
    UpstreamBody              string
    UpstreamBodyTruncated     bool
    UpstreamBodyOriginalBytes int
    StatusCode                int
    Source                    ErrorSource
    HTTPRequestSent           bool
    HTTPResponseReceived      bool
    StatusCodeSource          ErrorStatusCodeSource
    RequestID                 string
    Method                    string
    URLHost                   string
    URLPath                   string
    Err                       error
}
```

It implements `error`, `Unwrap`, and `Is`, so callers can use `errors.Is`, `errors.As`, and SDK helper functions.

Standard sentinel errors include:

- `ErrValidation`
- `ErrAuthentication`
- `ErrPermission`
- `ErrAuth`
- `ErrNotFound`
- `ErrAlreadyExists`
- `ErrIdempotency`
- `ErrRateLimit`
- `ErrTimeout`
- `ErrCancellation`
- `ErrInternal`
- `ErrUnprocessable`
- `ErrConfiguration`
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

Collector endpoints are passed to the OTLP gRPC exporter as `host:port` values, for example `localhost:4317`. TLS is the default; local plaintext collectors require `observability.WithCollectorInsecure(true)` in development/local environments.

## Retry internals

Retry behavior is implemented in `pkg/retry` and integrated into `entities.HTTPClient`.

Root-client retry defaults come from `pkg/config` and are applied to entity HTTP clients during setup:

- Maximum retries: 3
- Initial delay: 1s
- Maximum delay: 30s
- Backoff factor: 2.0
- Jitter factor: 0.25
- Retryable status codes: 408, 425, 429, 500, 502, 503, 504

Unsafe requests are retried only when an idempotency key is present.

## Maintainer checklist

When changing public API shape:

- Update service interfaces and private implementations together.
- Update `README.md`, `docs/README.md`, `docs/examples.md`, and `docs/mapping/external_apis.md`.
- Update this internal map when transport, config, retry, observability, or service URL behavior changes.
- Run targeted tests for changed packages and prefer `make ci` before PRs.
