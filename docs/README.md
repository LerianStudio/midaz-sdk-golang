# Midaz Go SDK documentation

This directory contains hand-written guides and generated package documentation for the Midaz Go SDK.

## Start here

- [Environment variables](./environment.md) - Runtime configuration and `.env` usage.
- [Error handling](./errors.md) - SDK error categories, helpers, and retry boundaries.
- [Architecture](./architecture.md) - SDK structure and implementation overview.
- [Examples](./examples.md) - Runnable examples and common workflows.
- [Pagination](./pagination.md) - List options, page metadata, and cursor behavior.
- [Tracing](./tracing.md) - OpenTelemetry setup and trace propagation.

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

Then open http://localhost:6060/pkg/github.com/LerianStudio/midaz-sdk-golang/v3/.

Generated docs currently include:

- [Root package](./godoc/index.txt)
- [Entities package](./godoc/entities/index.txt)
- [Models package](./godoc/models/index.txt)
- [Config package](./godoc/pkg/config/index.txt)
- [Errors package](./godoc/pkg/errors/index.txt)
- [Observability package](./godoc/pkg/observability/index.txt)
- [Validation package](./godoc/pkg/validation/index.txt)
- [Concurrent package](./godoc/pkg/concurrent/index.txt)
- [Retry package](./godoc/pkg/retry/index.txt)
- [Performance package](./godoc/pkg/performance/index.txt)
- [Pagination package](./godoc/pkg/pagination/index.txt)
- [Format package](./godoc/pkg/format/index.txt)

## Package structure

- `github.com/LerianStudio/midaz-sdk-golang/v3` - Root package. Exposes `Client`, `New`, and client functional options.
- `entities` - Entity service interfaces and HTTP implementations for Ledger and CRM API resources.
- `models` - Public SDK request/response types, fluent builders, aliases, pagination helpers, and common constants.
- `pkg/auth` - Plugin-based authentication using Access Manager credentials.
- `pkg/config` - Environment-aware configuration and service URL resolution.
- `pkg/errors` - Structured SDK error type, categories, codes, constructors, and checking helpers.
- `pkg/observability` - OpenTelemetry tracing, metrics, logging, propagation, and middleware helpers.
- `pkg/retry` - Retry policies, backoff, jitter, and HTTP retry helpers.
- `pkg/pagination` - Generic paginator utilities separate from `models.ListOptions`.
- `pkg/concurrent` - Worker pool, batch, and rate-limit helpers.
- `pkg/security`, `pkg/validation`, `pkg/format`, `pkg/transaction`, `pkg/version` - Supporting utility packages.

## Entity services

The root client exposes entity services through `c.Entity` when created with `client.UseAllAPIs()` or `client.UseEntityAPI()`:

- `Accounts`
- `AccountTypes`
- `Assets`
- `AssetRates`
- `Balances`
- `Holders`
- `Aliases`
- `Ledgers`
- `MetadataIndexes`
- `Operations`
- `OperationRoutes`
- `Organizations`
- `Portfolios`
- `Segments`
- `Transactions`
- `TransactionRoutes`

## Configuration baseline

Environment variables are not loaded implicitly. Use:

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

See [Environment variables](./environment.md) for the full list of supported variables.

Unsafe transaction create calls receive an auto-generated `X-Idempotency` header by default. Use `entities.WithIdempotencyKey` or input-level idempotency keys when a caller-chosen stable key is required, or when auto-idempotency has been disabled for a client or request.
