![banner](image/midaz-banner.png)

<div align="center">

[![Latest Release](https://img.shields.io/github/v/release/LerianStudio/midaz-sdk-golang?include_prereleases)](https://github.com/LerianStudio/midaz-sdk-golang/releases)
[![Go Report](https://goreportcard.com/badge/github.com/lerianstudio/midaz-sdk-golang)](https://goreportcard.com/report/github.com/lerianstudio/midaz-sdk-golang)
[![Discord](https://img.shields.io/badge/Discord-Lerian%20Studio-%237289da.svg?logo=discord)](https://discord.gg/DnhqKwkGv3)
[![Go Version](https://img.shields.io/github/go-mod/go-version/LerianStudio/midaz-sdk-golang)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Elastic%202.0-blue.svg)](LICENSE.md)

</div>

# Midaz Go SDK

The Midaz Go SDK is the idiomatic v5 client for the Midaz financial-ledger
APIs: typed list-opts, `iter.Seq2`-based pagination, structured errors with
retry classification, `*slog.Logger` canonical logging, OpenTelemetry
observability, and a single canonical auth surface (Access Manager OAuth or
anonymous local-stack mode).

## What's new in v5

v5 serves **both** Midaz ledger surfaces — `/v1`, deprecated but alive, and
`/v2`, the current one — with the version in the request path rather than
pinned into a base URL. Reach them through `c.V1.<Service>` and
`c.V2.<Service>`; **build against `c.V2`**.

Breaking changes, each one closing a way the SDK could return a wrong answer
with a nil error:

- **The six dead transaction list filters are refused**, on both surfaces:
  `Status`, `AssetCode`, `Reference`, `SourceAccount`, `DestinationAccount`,
  `Route`. The ledger never honored them — it parses two and discards them, and
  never parses the other four — so setting one returned the whole unfiltered
  ledger. They are now a local refusal naming every filter you set. `Status` and
  `Route` still narrow on `Count`.
- **`Count` takes `YYYY-MM-DD` dates**, the same format `List` takes from the
  same opts struct; the SDK widens each day to its inclusive instant bounds.
  Note the **default window is TODAY**, not the ledger.
- **An empty 2xx list or object body is refused** rather than decoded into a
  zero value with a nil error. A dropped body used to read as an empty page with
  no next cursor, so a caller walking a ledger concluded it was empty.
- **`%` and path-hostile ids are refused locally.** A `..` in an id popped a
  path segment, so deleting a ledger could issue `DELETE /v1/organizations/{org}`.
- The dead transaction list query renderer (`TransactionsListOpts.ToQueryParams`)
  is deleted, as is the alias service — the server serves no alias route at any
  version (renamed to instruments, `/v2` only).

All reads go through the raw response path rather than a generated parser, so a
gateway 403 stays a 403 and a 404 stays a 404 even with an empty body, instead
of failing inside an unmarshal and arriving as an SDK-internal 500.

Carried over and still current:

- **One auth source, enforced**: `WithAccessManager` for production OAuth,
  `WithAnonymous` for local stacks. Calling `New()` with neither returns a
  typed configuration error at construction time.
- **Typed pagination opts at the type system**: page-based and cursor-based
  endpoints have separate opts types. Wrong-shape opts don't compile.
- **`iter.Seq2[T, error]`**: every list method ships in a trio —
  `List` (one page) / `All` (every item) / `Pages` (every page envelope).
- **Structured errors**: every error is a `*pkg/errors.Error` with
  `Category`, `Code`, `Operation`, `Resource`, and a canonical
  `Retryable()` method. Real network/timeout/auth/validation classification.
- **Canonical logging**: `*slog.Logger` is the canonical client/application
  logger surface. The SDK is silent by default (`slog.DiscardHandler`); opt
  in with `WithLogger`. A separate `Provider.Logger` (OTel-correlated) is
  also exposed by `pkg/observability` for span-aware logging from inside
  SDK callbacks.
- **OpenTelemetry first-class**: spans + metrics + logs through one
  `observability.Provider` wired by `WithObservabilityProvider`.
- **Idempotency by default**: auto-generated `X-Idempotency` per unsafe
  request. Override with `sdkctx.WithIdempotencyKey` for stable
  caller-supplied keys; suppress per-call with `WithoutAutoIdempotency`.
- **Consumer-declared test doubles**: every accessor is a concrete facade, so
  there are no pre-generated mocks to import. Declare the narrow interface your
  code actually calls — the accessor satisfies it structurally. See
  [Testing with mocks](#testing-with-mocks).

## Installation

```bash
go get github.com/LerianStudio/midaz-sdk-golang/v6
```

Requires Go 1.26+ — the toolchain pinned in `go.mod`. The SDK uses
`iter.Seq2` (Go 1.23+) and `log/slog` (Go 1.21+) in its public API; the
1.26 floor matches the rest of the Lerian Go stack.

## Quick start

The minimum-viable shape — local stack, anonymous auth, list 5 organizations:

```go
package main

import (
    "context"
    "fmt"
    "log"

    midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
    "github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

func main() {
    c, err := midaz.New(
        midaz.WithEnvironment(midaz.EnvironmentLocal),
        midaz.WithAnonymous(),
    )
    if err != nil {
        log.Fatalf("midaz.New: %v", err)
    }
    defer c.Shutdown(context.Background())

    page, err := c.V2.Organizations.List(context.Background(),
        models.OrganizationsListOpts{
            PageListOpts: models.PageListOpts{Limit: 5},
        })
    if err != nil {
        log.Fatalf("ListOrganizations: %v", err)
    }
    for _, org := range page.Items {
        fmt.Printf("- %s (%s)\n", org.LegalName, org.ID)
    }
}
```

For Access Manager auth (production) and the full client-construction
matrix see [`docs/auth.md`](docs/auth.md) and [`docs/configuration.md`](docs/configuration.md).

## Core surfaces

### Service access

Midaz serves two ledger surfaces at once — `/v1`, deprecated but alive, and
`/v2`, the current one — and does not mirror every resource across them. The SDK
groups the ledger services by the version that actually serves them, so reaching
for a resource the other version does not have is a compile error instead of a
404:

**Build against `c.V2`.** Midaz deprecated all of `/v1`, and `c.V2` is the
wider surface — 22 services against V1's 14, not a small add-on.

```go
orgs, err := c.V2.Organizations.List(ctx, opts)
ledger, err := c.V2.Ledgers.Create(ctx, orgID, input)
account, err := c.V2.Accounts.Get(ctx, orgID, ledgerID, accountID)
balance, err := c.V2.Balances.GetBalance(ctx, orgID, ledgerID, balanceID)
tx, err := c.V2.Transactions.CreateDirect(ctx, orgID, ledgerID, input)
holder, err := c.V2.Holders.Get(ctx, orgID, holderID)

// /v1 for the two things it alone still serves:
rate, err := c.V1.AssetRates.GetAssetRate(ctx, orgID, ledgerID, externalID)
legacy, err := c.V1.Transactions.CreateJSON(ctx, orgID, ledgerID, sendInput)
```

**Served by both** (13 families, same method names on each):
`Organizations`, `Ledgers`, `Accounts`, `AccountTypes`, `Assets`, `Balances`,
`Operations`, `Portfolios`, `Segments`, `OperationRoutes`, `TransactionRoutes`,
`Transactions`, `MetadataIndexes`.

**`c.V2` only** — Midaz *removed* these from `/v1`: `Holders`, `Instruments`,
`Encryption`, `Composition`, `ProtectionAudit`, `BillingPackages`,
`FeePackages`, `FeeEstimates`, `BillingCalculations`. The billing family is also
**ledger-scoped** on `/v2` (it moved from organization scope), so its methods
take a ledger ID.

**`c.V1` only** — `/v2` dropped these: `AssetRates`, and the four transaction
creation styles `CreateJSON` / `CreateInflow` / `CreateOutflow` /
`CreateAnnotation`.

`Transactions` is the one family whose two surfaces differ in **contract**, not
just path. `/v1` nests a `send` envelope behind four creation endpoints. `/v2`
creates at the **top level** (`POST /v2/transactions/direct`) with a flat body —
an asset, a total, and two leg arrays — and the organization and ledger
travelling per leg rather than in the URL. `c.V2.Transactions` answers with
`models.TransactionV2`, which drops four `/v1` fields and carries two `/v1`
dropped. `c.V2.Accounts` also does not re-expose the account-scoped balance and
operation reads: `/v1` spells each on two accessors, `/v2` spells each once, on
`c.V2.Balances` and `c.V2.Operations`.

The version lives in the request **path**, never in the base URL. The Ledger
base URL is bare, and a `/v1` or `/v2` suffix on it is rejected at construction.

Tracer-plane services are **not** version-grouped — the Tracer serves one
surface and carries its version in the base URL — so they stay flat on the
client: `c.Rules`, `c.Limits`, `c.Validations`, `c.Reservations`,
`c.AuditEvents`.

### Settlement waits

An accepted transaction (HTTP 201) is not settled. Wait on the balance effect
with `transaction.WaitForSettlement(ctx, c.V2.Balances, orgID, ledgerID,
accountID, settled)`, passing a `func(models.Balance) bool` predicate (pin the
asset on a multi-asset account). See
[`pkg/transaction`](pkg/transaction/settlement.go).

### Fee, billing, and composition builders

`models.NewFeeEstimateInput(packageID, ledgerID, send)`,
`models.NewBillingCalculateInput(ledgerID, period)`, and
`models.NewCreateHolderAccountInput(assetCode, accountType)` (each with `With*`
optionals) build the inputs for `FeeEstimates`, `BillingCalculations`, and
`Composition` respectively.

### Pagination

Every list method ships in three flavors:

```go
// One page, you decide when to advance.
page, err := c.V2.Accounts.List(ctx, orgID, ledgerID, opts)

// iter.Seq2 over every item across every page (SDK handles paging).
for acc, err := range c.V2.Accounts.All(ctx, orgID, ledgerID, opts) {
    if err != nil { return err }
    process(acc)
}

// iter.Seq2 over page envelopes (with metadata for checkpointing).
for batch, err := range c.V2.Accounts.Pages(ctx, orgID, ledgerID, opts) {
    if err != nil { return err }
    log.Printf("page %d: %d items", batch.Pagination.Page, len(batch.Items))
}
```

Page-based and cursor-based endpoints use separate opts types. See
[`examples/05-listing-pages/`](examples/05-listing-pages/) and
[`examples/04-listing-cursor/`](examples/04-listing-cursor/).

### Idempotency

Auto-on by default. The SDK emits `X-Idempotency: <uuid>` on every unsafe
request. Override per-call:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"

// Stable key for at-least-once producers (saga steps, outbox rows, UI submissions):
ctx := sdkctx.WithIdempotencyKey(ctx, "tx-2026-05-06-001")

// Suppress for one call (rare — fire-and-forget administrative endpoints):
ctx := sdkctx.WithoutAutoIdempotency(ctx)
```

Disable globally with `midaz.WithIdempotency(false)`. See
[`examples/06-idempotency/`](examples/06-idempotency/).

### Errors

Every error is a `*pkg/errors.Error`. Use the typed predicates or
`errors.As` for structured field access:

```go
import sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"

acc, err := c.V2.Accounts.Get(ctx, orgID, ledgerID, accountID)
if err != nil {
    switch {
    case sdkerrors.IsNotFoundError(err):
        return fmt.Errorf("account not found: %w", err)
    case sdkerrors.IsAuthError(err):
        return fmt.Errorf("re-authenticate: %w", err)
    case sdkerrors.IsValidationError(err):
        return fmt.Errorf("input invalid: %w", err)
    case sdkerrors.IsNetworkError(err):
        return fmt.Errorf("transient transport: %w", err) // retry-safe
    }
}
```

Or walk fields:

```go
var sdkErr *sdkerrors.Error
if errors.As(err, &sdkErr) {
    log.Printf("op=%s resource=%s code=%s retryable=%v",
        sdkErr.Operation, sdkErr.Resource, sdkErr.Code, sdkErr.Retryable())
}
```

`Retryable()` is the canonical retry-policy source — derived from
`Category`. Use it instead of re-implementing a category switch in
consumer code.

### Logging

Inject a `*slog.Logger`:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentLocal),
    midaz.WithAnonymous(),
    midaz.WithLogger(logger),
)
```

Adapters for zap, zerolog, logrus all go through `slog.Handler`. See
[`docs/logging.md`](docs/logging.md) and [`examples/08-logging-slog/`](examples/08-logging-slog/).

### Retries

Default policy: 3 retries, exponential backoff with 25% jitter, retryable
on transport errors + 5xx + 408 + 425 + 429. Customize:

Unsafe methods (`POST`, `PUT`, `PATCH`, `DELETE`) retry only when
`X-Idempotency` is present. The SDK auto-generates this header by default;
`WithoutAutoIdempotency` or `WithIdempotency(false)` disables automatic unsafe
retries unless the caller supplies `X-Idempotency` explicitly.

```go
import "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"

c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentLocal),
    midaz.WithAnonymous(),
    midaz.WithRetryOptions(
        retry.WithMaxRetries(5),
        retry.WithInitialDelay(200*time.Millisecond),
    ),
)
```

Or `WithCustomRetryPolicy(func(*Response, error) bool)` for arbitrary
predicates. Disable with `WithoutRetries()`. See [`examples/07-retries/`](examples/07-retries/).

### Observability (OpenTelemetry)

```go
import "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/observability"

provider, err := observability.New(ctx,
    observability.WithServiceName("payments-api"),
    observability.WithEnvironment("production"),
    observability.WithComponentEnabled(true, true, true), // tracing, metrics, logs
)
if err != nil { return err }
defer provider.Shutdown(ctx)

c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(am),
    midaz.WithObservabilityProvider(provider),
)
```

The SDK emits one HTTP span per outbound request with proper W3C
`traceparent` propagation. Business logs carry safe IDs only — never
payloads, names, addresses, or auth headers. See
[`examples/10-observability-otel/`](examples/10-observability-otel/).

### Multi-tenancy

```go
c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(am),
)
```

Tenant scope is derived from the Access Manager/JWT claims used to obtain the
token. The SDK does not expose tenant configuration and does not send
`X-Tenant-ID`; use separate Access Manager credentials/token context when tenant
scope differs.

See [`docs/multi-tenancy.md`](docs/multi-tenancy.md).

### Testing with mocks

Depend on a narrow interface you declare yourself — only the methods you call.
`c.V2.Accounts` (and the other accessors) satisfy it structurally in production, and
a tiny local mock satisfies it in tests ("accept interfaces, return structs"):

```go
// The narrow slice of the Accounts accessor your code needs.
type accountSource interface {
    GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}

type mockAccounts struct {
    getByAlias func(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}

func (m *mockAccounts) GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error) {
    return m.getByAlias(ctx, orgID, ledgerID, alias)
}

func TestMyHandler(t *testing.T) {
    svc := &mockAccounts{
        getByAlias: func(_ context.Context, _, _, _ string) (*models.Account, error) {
            return &models.Account{ID: "acc-1"}, nil
        },
    }
    // ... inject svc into your code under test; in production pass c.V2.Accounts.
}
```

See [`examples/09-testing-with-mocks/`](examples/09-testing-with-mocks/) for a full worked example.

## Environment variables

The SDK reads env vars only when `config.FromEnvironment()` is in the
option chain — there is no implicit env-var loading. The authoritative
templates are [`.env.example`](.env.example) (alias of
[`.env.local.example`](.env.local.example)) and
[`.env.production.example`](.env.production.example). Copy one with
`make set-env` and edit in place.

| Variable | Type | Effect |
|---|---|---|
| `MIDAZ_ENVIRONMENT` | `local\|development\|production` | Selects per-environment URL defaults |
| `MIDAZ_BASE_URL` | URL | Host base. The Ledger plane uses it as-is (the SDK versions each Ledger path itself, so a `/v1` or `/v2` suffix is rejected); the Tracer plane gets `/v1` appended |
| `MIDAZ_LEDGER_URL` | URL | Specific override for the Ledger plane (onboarding + transactions). Wins over `MIDAZ_BASE_URL` |
| `MIDAZ_TRACER_URL` | URL | Specific override for the Tracer plane. Derived from `MIDAZ_BASE_URL` when unset |
| `MIDAZ_TRACER_API_KEY` | string | **Optional** `X-API-Key` for the Tracer plane. When unset, the Tracer plane shares the Ledger Bearer token |
| `MIDAZ_TIMEOUT` | int (seconds) | HTTP client timeout |
| `MIDAZ_MAX_RETRIES` | int | Maximum retry attempts |
| `MIDAZ_DEBUG` | bool | Verbose SDK logging |
| `MIDAZ_IDEMPOTENCY` | bool | Toggle auto-generated `X-Idempotency` headers |
| `MIDAZ_ERROR_EXPOSE_BODY` | bool | Include the raw response body inside `*pkg/errors.Error` |
| `PLUGIN_AUTH_ENABLED` | bool | **Sentinel** — must be set for the four Access Manager vars below to take effect |
| `PLUGIN_AUTH_ADDRESS` | URL | Access Manager endpoint |
| `MIDAZ_CLIENT_ID` | string | OAuth client ID |
| `MIDAZ_CLIENT_SECRET` | string | OAuth client secret |
| `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP` | bool | Permit plain HTTP to the Access Manager. Local development only |
| `MIDAZ_ALLOW_INSECURE_HTTP` | bool | Permit plain HTTP to the Ledger / Tracer service URLs for non-loopback hosts. Intended for Kubernetes cluster-internal services (`*.svc.cluster.local`) reached over the cluster mesh and dev/test deployments behind a controlled network boundary. Leave false for public-internet deployments; rejected at validation time when `MIDAZ_ENVIRONMENT=production` |

The `User-Agent` header is fixed by the SDK to `midaz-go-sdk/<version>`;
override programmatically with `midaz.WithUserAgent` if needed. See
[`docs/configuration.md`](docs/configuration.md) for the full matrix and
precedence rules.

## Documentation

- [`docs/auth.md`](docs/auth.md) — authentication setup and migration
- [`docs/configuration.md`](docs/configuration.md) — every available SDK option, both layers
- [`docs/multi-tenancy.md`](docs/multi-tenancy.md) — tenant routing
- [`docs/logging.md`](docs/logging.md) — `*slog.Logger` contract + adapter recipes
- [`docs/pagination.md`](docs/pagination.md) — pagination contract
- [`docs/errors.md`](docs/errors.md) — error categories, codes, retry boundaries
- [`docs/examples.md`](docs/examples.md) — runnable example index
- [`pkg.go.dev/github.com/LerianStudio/midaz-sdk-golang/v6`](https://pkg.go.dev/github.com/LerianStudio/midaz-sdk-golang/v6) — generated API reference

Generate docs locally:

```bash
make docs       # static HTML to docs/godoc/
make godoc      # interactive server at http://localhost:6060
```

## Examples

See [`examples/README.md`](examples/README.md) for the full numbered list
with a Start-Here / Common workflows / Behavior & resilience / Testing &
observability / Reference structure. Highlights:

- [`examples/01-hello-world/`](examples/01-hello-world/) — minimum-viable shape (~17 body lines)
- [`examples/02-auth/`](examples/02-auth/) — Access Manager authentication
- [`examples/03-end-to-end/`](examples/03-end-to-end/) — org → ledger → account → transaction
- [`examples/06-idempotency/`](examples/06-idempotency/) — idempotency mode reference
- [`examples/07-retries/`](examples/07-retries/) — retry policies
- [`examples/08-logging-slog/`](examples/08-logging-slog/) — `*slog.Logger` integration
- [`examples/09-testing-with-mocks/`](examples/09-testing-with-mocks/) — unit-testing pattern
- [`examples/10-observability-otel/`](examples/10-observability-otel/) — OpenTelemetry
- [`examples/mass-demo-generator/`](examples/mass-demo-generator/) — production-shaped data generator at scale
- [`examples/workflow-with-entities/`](examples/workflow-with-entities/) — every public service through full CRUD

Run the demo data generator:

```bash
make demo-data
# or
DEMO_NON_INTERACTIVE=1 go run ./examples/mass-demo-generator --org-locale=br
```

Build all examples:

```bash
make examples-test
```

## Testing

```bash
make test                    # unit tests
make coverage                # HTML coverage report under artifacts/
make verify-sdk              # API compatibility + parity checks
make examples-test           # build every example, run example tests
make ci                      # full local pipeline: tidy + fmt + lint + gosec + test + verify-sdk
```

## Contributing

Contributions are welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Elastic License 2.0. See [`LICENSE.md`](LICENSE.md) for details.

Copyright 2025 Lerian Studio
