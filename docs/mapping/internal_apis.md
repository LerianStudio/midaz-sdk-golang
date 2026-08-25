# Midaz Go SDK internal API map

This map is for SDK maintainers. It describes the implementation structure behind the public API. SDK consumers should prefer [external_apis.md](./external_apis.md).

## Runtime architecture

The current SDK is organized around a root client and an entity layer:

1. `midaz.Client` owns configuration, observability, lifecycle, and service initialization.
2. `pkg/config.Config` resolves service URLs, Access Manager settings, retry/debug options, HTTP client, and observability provider.
3. `entities.Entity` exposes the accessors used by consumers, with the ledger-plane ones grouped by server version (`Entity.V1`, `Entity.V2`).
4. **Every** resource is a concrete `*xFacade` struct over a generated plane client. There are no interface-backed private entities and no hand-rolled HTTP left: `balancesEntity`, `operationsEntity` and `aliasesEntity` are gone, along with `BalancesService`, `OperationsService` and `AliasesService`.
5. `entities.HTTPClient` no longer serves any resource. It survives only because `(*Entity).GetEntityHTTPClient` hands it to callers for debug / user-agent / retry tuning; no request traffic routes through it.
6. `models` contains public request/response structures, Midaz model aliases, list options, pagination metadata, and builder helpers.

The SDK does not currently use the older `apiClient`, `httpClient`, or per-resource `organizationClient` style architecture.

### Facade layer

Ledger-plane accessors are grouped by the server version that serves them: `V1Services` (14 members) and `V2Services` (22). Both groups are held on `Entity` **by value**, not by pointer — a hand-rolled zero-value `&Entity{}` is legal, and with a value group its members are simply nil there, so the idiomatic `e != nil && e.V1.Accounts != nil` guard holds. A pointer group would panic one level below what that check can see.

Every accessor is a concrete facade struct (`*accountsFacade`, `*accountsV2Facade`, ...) over the generated ledger plane client (`internal/genledger.ClientWithResponses`). Tracer-plane accessors (`Rules`, `Limits`, `Validations`, `Reservations`, `AuditEvents`) stay flat on `Entity` over `internal/gentracer` — the Tracer serves one surface and versions itself in its base URL.

Facades are **unexported concrete types**, and the generated mocks are gone (`entities/mocks/` no longer exists). A consumer that needs to substitute an accessor declares a narrow consumer-side interface naming only the methods it calls — the pattern `pkg/integrity` uses for its `balancesGetter` / `accountsGetter`.

#### No facade reads through a generated `*WithResponse` parser

Every read, write and delete goes through the raw generated call plus a shared decode helper (`readOne` / `readList` / `readSlice` / `deleteResource` / `readRawResponse`). This is load-bearing rather than stylistic: the generated `Parse*Resp` functions unmarshal the body themselves whenever the content type says json, which fails *before* any facade logic runs. Three consequences the raw path avoids:

- A gateway **403 or 404 carrying an empty body** kept its real status instead of failing inside the parser's unmarshal and arriving as an SDK-internal 500. A caller can again tell "you are not allowed" from "it is not there".
- An unreadable 2xx is a **response-decode** error ("the server answered and the answer is unreadable", so on a write the operation may already have taken effect), not an SDK-internal fault.
- A non-UUID id in a single-object response no longer reports as an SDK bug.

A 2xx carrying **no resource** (`null`, `{}`, empty, whitespace) is refused rather than decoded into a zero-valued object with a nil error — on lists too, where it previously produced an empty page with no next cursor, so a caller walking a ledger concluded it was empty. Bare-array reads are deliberately exempt from the object guard: Go marshals a nil slice as the literal `null`, so there `null` is what a handler with no results legitimately emits.

Response bodies are capped at 10 MiB (`maxHTTPResponseBodyBytes`).

The invariant is enforced structurally rather than by review: `TestNoFacadeCallsAGeneratedParser` parses every non-test file in `entities/`, matches each selector against the operations read out of both generated clients, and refuses any facade naming a `*WithResponse` spelling. Sibling scans enforce the delete seam, the idempotency stamp, the path-id guard, and one-spelling-per-endpoint on V2.

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
- `MIDAZ_TRACER_URL`
- `MIDAZ_TRACER_API_KEY`
- `MIDAZ_TIMEOUT`
- `MIDAZ_DEBUG`
- `MIDAZ_MAX_RETRIES`
- `MIDAZ_IDEMPOTENCY`
- `MIDAZ_ERROR_EXPOSE_BODY`
- `MIDAZ_ALLOW_INSECURE_HTTP` — Ledger/Tracer planes; loaded BEFORE the three URL variables, which is what makes a cluster-internal `http://…svc.cluster.local` URL parse. Set through `config.WithAllowInsecureHTTP` / `midaz.WithAllowInsecureHTTP` when the URLs come from code instead, where the ordering is the caller's to get right. `Validate` refuses it together with `EnvironmentProduction`.
- `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP` — the auth plane's own knob, independent of the one above.
- `PLUGIN_AUTH_ENABLED`
- `PLUGIN_AUTH_ADDRESS`
- `MIDAZ_CLIENT_ID`
- `MIDAZ_CLIENT_SECRET`

Access Manager configuration uses `auth.AccessManager` and `config.WithAccessManager`. `MIDAZ_AUTH_TOKEN` is not part of `config.FromEnvironment()`.

`MIDAZ_ENVIRONMENT` recomputes default service URLs unless `MIDAZ_BASE_URL` or a service-specific URL has already been set. Explicit service URLs take precedence. The entity layer normalizes them per plane: the Ledger URL stays bare (its version rides inside each operation path, and a `/v1` or `/v2` suffix is rejected), while the Tracer URL is normalized to include `/v1`.

## Service URL model

The entity layer receives a service URL map with exactly **two** keys, one per plane:

- `onboarding` (`config.ServiceOnboarding`) — the Ledger plane, resolved from `Config.LedgerURL` (`WithLedgerURL` / `MIDAZ_LEDGER_URL`).
- `tracer` (`config.ServiceTracer`) — the Tracer plane, resolved from `Config.TracerURL` (`WithTracerURL` / `MIDAZ_TRACER_URL`).

The `transaction` and `crm` keys are **gone**. `transaction` was a phantom: after the facade migration nothing read it to build a request, yet config validation still required it — a mandatory key with no effect. `crm` went with the alias service, since Midaz folded those resources into the ledger surface. Neither ever existed as an environment variable; both were internal labels only.

`onboarding` is a rename candidate — it is the last place the pre-two-plane service naming survives, and it now labels a whole plane rather than one service.

## Entity service implementations

Every ledger resource is served by a facade accessor described in
[external_apis.md](./external_apis.md) — a concrete unexported `*xFacade` struct
over the generated plane client, with no public interface and no private
implementation type. The interface-backed trio is gone: `balancesEntity`,
`operationsEntity` and `aliasesEntity`, and with them `BalancesService`,
`OperationsService` and `AliasesService`.

Balances and operations were migrated onto the generated client; aliases was
deleted outright, because the server serves no alias route at any version — the
resource was renamed to instruments and is /v2 only.

## Transport pattern

Transport cross-cutting concerns live on the generated plane clients' request
editors and the shared `*http.Client` built for each plane, **not** on
`entities.HTTPClient`, which no longer carries request traffic:

- Adds authorization after Access Manager resolves a token. The Tracer plane can authenticate with an `X-API-Key` (`MIDAZ_TRACER_API_KEY`) instead of the shared Bearer token.
- Adds idempotency keys — auto-generated for unsafe methods by default, or caller-supplied via `sdkctx.WithIdempotencyKey(ctx, key)` / an input's `IdempotencyKey` field.
- Injects OpenTelemetry trace context and baggage into outbound HTTP headers when observability is enabled.
- Applies retry behavior for retryable responses and transient network failures.
- Avoids retrying unsafe methods unless `X-Idempotency` is present.
- Converts HTTP failures into `pkg/errors` structured errors, decoding both RFC 9457 problem documents and the /v1 legacy error shape (`message` / `fields` / `entityType`).
- Attaches raw, unredacted, truncated upstream 4xx/5xx response bodies to structured errors when error body exposure is enabled.
- Emits debug logs when `MIDAZ_DEBUG=true` or debug options are enabled.
- Refuses any cross-origin redirect (`ValidatePlaneRedirect`) and caps a response body at 10 MiB.

## Request path construction

Paths are **generated**, not hand-built: `scripts/generate-clients.sh` renders `api/ledger.openapi.yaml` (a copy of the server's own OAS) into `internal/genledger`, and each facade calls the generated request builder for its operation. `make check-codegen-drift` fails if the committed clients stop reproducing from the specs. No facade concatenates a path.

The groups below are written **without their version prefix**. On the wire every ledger path carries one: `/v1/organizations` for a `V1.*` accessor, `/v2/organizations` for a `V2.*` one. That prefix is the whole versioning mechanism — the Ledger base URL carries none, which is why a `/v1` suffix on it is rejected at construction. Where a family is served by only one version, it is marked below.

Important path groups:

- Organizations: `/organizations`, `/organizations/{id}`
- Ledgers: `/organizations/{organizationID}/ledgers`, `/organizations/{organizationID}/ledgers/{ledgerID}`
- Ledger settings: `/organizations/{organizationID}/ledgers/{ledgerID}/settings` with `GET` and `PATCH` for `accounting.validateAccountType` and `accounting.validateRoutes`.
- Accounts: `/organizations/{organizationID}/ledgers/{ledgerID}/accounts`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/alias/{alias}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/external/{assetCode}`
- Balances: `/organizations/{organizationID}/ledgers/{ledgerID}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/balances/{balanceID}`, `/organizations/{organizationID}/ledgers/{ledgerID}/balances/{balanceID}/history?date={date}`
- Account balances: `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/balances/history?date={date}`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/alias/{alias}/balances`, `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/external/{assetCode}/balances`
- Assets: `/organizations/{organizationID}/ledgers/{ledgerID}/assets`
- Asset rates: `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates`, `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates/{externalID}`, and `/organizations/{organizationID}/ledgers/{ledgerID}/asset-rates/from/{assetCode}` using cursor filters (`to`, `limit`, `start_date`, `end_date`, `sort_order`, `cursor`).
- Transactions — the one family whose two surfaces differ in path SHAPE, so both are spelled out with their prefixes:
  - Reads and transitions, identical shape on both: `/v{1,2}/organizations/{organizationID}/ledgers/{ledgerID}/transactions`, `.../transactions/{transactionID}`, `.../transactions/{transactionID}/commit`, `.../cancel`, `.../revert`, `.../transactions/metrics/count`, and the transaction-scoped operation update `.../transactions/{transactionID}/operations/{operationID}`.
  - **V1 creates** are ledger-scoped, one endpoint per style: `/v1/organizations/{organizationID}/ledgers/{ledgerID}/transactions/json`, `.../inflow`, `.../outflow`, `.../annotation`, plus `.../block` and `.../unblock`.
  - **V2 creates are TOP-LEVEL** and carry no organization or ledger in the URL at all: `/v2/transactions/direct`, `/v2/transactions/hold`, `/v2/transactions/block`, `/v2/transactions/unblock`. The scope travels per leg in the body instead, and the server refuses a body whose legs name different pairs.
- Operations: account-scoped reads use `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/operations` and `/organizations/{organizationID}/ledgers/{ledgerID}/accounts/{accountID}/operations/{operationID}`. Updates are transaction-scoped through `PATCH /organizations/{organizationID}/ledgers/{ledgerID}/transactions/{transactionID}/operations/{operationID}`.
- Routes: operation route endpoints use `/organizations/{organizationID}/ledgers/{ledgerID}/operation-routes`; transaction route endpoints use `/organizations/{organizationID}/ledgers/{ledgerID}/transaction-routes`.
- Metadata indexes: list uses `/settings/metadata-indexes` with optional `entity_name`; create uses `/settings/metadata-indexes/entities/{entityName}`; delete uses `/settings/metadata-indexes/entities/{entityName}/key/{metadataKey}`. The list endpoint returns a raw `[]MetadataIndex` slice, not a paginated `ListResponse`.
- Instruments (**/v2 only** — the resource /v1 served as "aliases", renamed): `/organizations/{organizationID}/instruments`, `/organizations/{organizationID}/holders/{holderID}/instruments`, `/organizations/{organizationID}/holders/{holderID}/instruments/{instrumentID}`, `/organizations/{organizationID}/holders/{holderID}/instruments/{instrumentID}/related-parties/{relatedPartyID}`. The old `/aliases` paths exist at no version.

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
- `models.NewCreateTransactionInput(assetCode, amount)` - Must include `send.source` and `send.distribute` before sending, through `WithSend(...)` — the legacy operation-adaptation path was removed in v4.2. Unsafe SDK requests receive an auto-generated `X-Idempotency` header by default; set `IdempotencyKey` or use `sdkctx.WithIdempotencyKey` when the caller needs a stable key or has disabled auto-idempotency.
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
- `models.NewCreateInstrumentInput(ledgerID, accountID)` with `WithBankingDetails`, `WithMetadata`, `WithRegulatoryFields`, and `WithRelatedParties`. The create endpoint declares `additionalProperties: false` and requires all four of ledger, account, banking details and metadata, so the input mirrors it exactly — banking details and metadata are set through builders but are not optional, and `Validate` refuses a payload missing either. There is no `type` or `document` on the create payload: the endpoint has no slot for them, and a body carrying one is rejected outright.
- `models.NewUpdateInstrumentInput()` with the same four setters plus `WithNullFields` / `WithNullField`. The PATCH contract requires `bankingDetails` and `metadata` even on a partial update — that is the server's choice and the SDK mirrors it, so `Validate` refuses a payload missing either. Consequently only `regulatoryFields` and `relatedParties` are clearable with an explicit null: clearing a required property would produce a body the endpoint refuses, so `Validate` names it as required rather than reporting a generic unsupported field. `document` is gone here too. Empty instrument updates are rejected.
- The alias builders (`NewCreateAliasInput`, `NewUpdateAliasInput`) are gone with the resource. The shared CRM value types they carried (`BankingDetails`, `RegulatoryFields`, `RelatedParty`) live in `models/crm_shared_types.go`.

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
| Balances | Cursor-based, and this is load-bearing: the server drops `page` on the floor for the balance lists, so a page-style iterator re-requested page 1 forever and yielded the same balances indefinitely. `BalancesListOpts` embeds `CursorListOpts` so that shape cannot be expressed. The alias and external-code balance lookups are not paginated at all and have no iterators. |
| Instruments (ex-aliases) | Ledger plane, /v2 only. Cursor-based; organization and holder are path segments, not headers. |
| Holders | Ledger plane, /v2 only. Cursor-based: `HoldersListOpts` embeds `CursorListOpts`, so `Cursor` seeds/resumes pagination and `Pages`/`All` inject the response `next_cursor` as a `cursor` query param, stopping on an empty cursor. Dates are rejected (`ValidateCursorListOptsNoDates`); the facade never emits `page`. Organization is a path segment, not a header. |

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

Collector endpoints are passed to the OTLP gRPC exporter, and the scheme selects the transport: `https://otel-collector:4317` exports over TLS, while a bare `host:port` such as `localhost:4317` is treated as plaintext. A plaintext endpoint is refused in a `production` environment, so production deployments need the `https://` prefix; local plaintext collectors belong in a development or local environment. `pkg/observability` rewrites that refusal to name both remedies, but the decision itself stays in lib-observability, including its `ALLOW_INSECURE_OTEL` override.

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
