# Midaz Go SDK documentation

This directory contains hand-written guides and generated package documentation for the Midaz Go SDK.

## Start here

- [Authentication](./auth.md) - Access Manager OAuth and anonymous mode.
- [Configuration](./configuration.md) - Four configuration surfaces, precedence rules, every option.
- [Environment variables](./environment.md) - Runtime configuration and `.env` usage.
- [Error handling](./errors.md) - SDK error categories, helpers, upstream bodies, and retry boundaries.
- [Architecture](./comprehensive-architecture.md) - SDK structure and implementation overview. (Older [architecture.md](./architecture.md) is retained as historical v2 context only.)
- [Examples](./examples.md) - Runnable examples and common workflows.
- [Pagination](./pagination.md) - List options, page metadata, and cursor behavior.
- [Multi-tenancy](./multi-tenancy.md) - Tenant resolution, header vs claims, propagation patterns.
- [Logging](./logging.md) - `*slog.Logger` integration recipes for stdlib slog, zap, zerolog, charmbracelet/log.

## API mapping

- [External API mapping](./mapping/external_apis.md) - Public SDK constructors, options, services, and model helpers.
- [Internal API mapping](./mapping/internal_apis.md) - Maintainer-facing map of internal transport and service implementation patterns.

## Generated package documentation

Generate or refresh static package docs with:

```bash
make docs
```

Run an interactive Go documentation server with:

```bash
make godoc
```

Then open http://localhost:6060/pkg/github.com/LerianStudio/midaz-sdk-golang/v5/.

Generated docs currently include:

- [Root package](./godoc/index.txt)
- [Entities package](./godoc/entities/index.txt)
- [Models package](./godoc/models/index.txt)
- [Auth package](./godoc/pkg/auth/index.txt)
- [Config package](./godoc/pkg/config/index.txt)
- [Errors package](./godoc/pkg/errors/index.txt)
- [Observability package](./godoc/pkg/observability/index.txt)
- [SDK context package](./godoc/pkg/sdkctx/index.txt)
- [Validation package](./godoc/pkg/validation/index.txt)
- [Concurrent package](./godoc/pkg/concurrent/index.txt)
- [Retry package](./godoc/pkg/retry/index.txt)
- [Performance package](./godoc/pkg/performance/index.txt)
- [Generator package](./godoc/pkg/generator/index.txt)
- [Format package](./godoc/pkg/format/index.txt)

## Package structure

- `github.com/LerianStudio/midaz-sdk-golang/v5` - Root package. Exposes `Client`, `New`, and client functional options.
- `entities` - The accessor layer: concrete facades over the two generated plane clients (Ledger and Tracer), grouped by server version for the Ledger plane.
- `models` - Public SDK request/response types, fluent builders, aliases, pagination helpers, and common constants.
- `pkg/auth` - Plugin-based authentication using Access Manager credentials.
- `pkg/config` - Environment-aware configuration and service URL resolution.
- `pkg/errors` - Structured SDK error type, categories, codes, constructors, and checking helpers.
- `pkg/observability` - OpenTelemetry tracing, metrics, logging, propagation, and middleware helpers.
- `pkg/retry` - Retry policies, backoff, jitter, and HTTP retry helpers.
- `pkg/concurrent` - Worker pool, batch, and rate-limit helpers.
- `pkg/performance` - Batch sizing, HTTP pooling, JSON reuse, and client optimization helpers.
- `pkg/generator` - Demo-data generation primitives used by `examples/mass-demo-generator`.
- `pkg/security`, `pkg/validation`, `pkg/format`, `pkg/transaction`, `pkg/version` - Supporting utility packages.

## Entity services

The root client initializes entity services when `midaz.New(...)` succeeds.

Midaz serves **two** ledger surfaces — `/v1`, deprecated but alive, and `/v2`,
the current one — and does not mirror every resource across them. Ledger
accessors are therefore grouped by the version that serves them, reached as
`c.V1.<Service>` / `c.V2.<Service>` (or through the embedded `c.Entity` field).
The version travels in the request path, not in the base URL.

**Build against `c.V2`.** It is the wider surface: 22 services against V1's 14.

Served by both (13 families):

- `Organizations`, `Ledgers`, `Accounts`, `AccountTypes`, `Assets`
- `Balances`, `Operations`, `Transactions`, `MetadataIndexes`
- `Portfolios`, `Segments`, `OperationRoutes`, `TransactionRoutes`

`c.V2` only, because Midaz removed them from `/v1`:

- `Holders`, `Instruments`, `Encryption`, `Composition`, `ProtectionAudit`
- `BillingPackages`, `FeePackages`, `FeeEstimates`, `BillingCalculations`
  (ledger-scoped on `/v2` — the family moved from organization scope)

`c.V1` only, because `/v2` dropped them:

- `AssetRates`
- the four `Transactions` creation styles (`CreateJSON`, `CreateInflow`,
  `CreateOutflow`, `CreateAnnotation`) — `/v2` replaced them with the top-level
  `CreateDirect` / `CreateHold`

Tracer-plane accessors are **not** version-grouped — the Tracer serves one
surface and versions itself in its base URL — so they stay flat on the client:
`c.Rules`, `c.Limits`, `c.Validations`, `c.Reservations`, `c.AuditEvents`.

## Configuration baseline

Environment variables are not loaded implicitly. Use:

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

See [Environment variables](./environment.md) for the full list of supported variables.

Unsafe SDK requests (`POST`, `PUT`, `PATCH`, `DELETE`) receive an auto-generated `X-Idempotency` header by default. Use `sdkctx.WithIdempotencyKey` or input-level idempotency keys when a caller-chosen stable key is required, or when auto-idempotency has been disabled for a client or request.
