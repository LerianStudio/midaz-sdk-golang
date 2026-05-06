# Environment configuration

This guide explains which environment variables the Midaz Go SDK reads and how they affect client behavior.

## v3 contract: explicit over implicit

**The SDK reads environment variables only when the caller opts in via `config.FromEnvironment()`.** Every read happens inside `pkg/config/config.go`. No entity constructor, HTTP client builder, or service helper reads `os.Getenv` directly. Setting `MIDAZ_DEBUG=true` in your shell does **nothing** to a client constructed without `FromEnvironment()`.

The SDK does **not** load `.env` files automatically. If you keep configuration in a `.env` file, load it before building the config.

```go
import (
    "github.com/joho/godotenv"

    midaz "github.com/LerianStudio/midaz-sdk-golang/v3"
    "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
)

func newClient() (*midaz.Client, error) {
    _ = godotenv.Load() // optional: populate os environment from .env

    cfg, err := config.NewConfig(config.FromEnvironment())
    if err != nil {
        return nil, err
    }

    return midaz.New(midaz.WithConfig(cfg))
}
```

## Supported environment variables

All variables below are read by `config.FromEnvironment()`. Standard library reads (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`) are honored by Go's `net/http` transport regardless of SDK configuration.

| Variable | Purpose | Default |
| --- | --- | --- |
| `MIDAZ_ENVIRONMENT` | Environment label. Valid values: `local`, `development`, `production`. | unset |
| `MIDAZ_BASE_URL` | Base URL used to derive Onboarding, Transaction, and CRM service URLs. | env-derived |
| `MIDAZ_ONBOARDING_URL` | Explicit Onboarding service URL. | derived from `MIDAZ_BASE_URL` |
| `MIDAZ_TRANSACTION_URL` | Explicit Transaction service URL. | derived from `MIDAZ_BASE_URL` |
| `MIDAZ_CRM_URL` | Explicit CRM service URL. | derived from `MIDAZ_BASE_URL` |
| `MIDAZ_TIMEOUT` | HTTP timeout in seconds. | `60` |
| `MIDAZ_USER_AGENT` | User agent header value. | SDK version user agent |
| `MIDAZ_DEBUG` | Enables debug logging when set to `true`. | `false` |
| `MIDAZ_MAX_RETRIES` | Maximum retry attempts. Set to `0` to disable retries entirely. | `3` |
| `MIDAZ_IDEMPOTENCY` | Enables (`true`) or disables (`false`) auto idempotency support. | `true` |
| `PLUGIN_AUTH_ENABLED` | Enables Access Manager authentication when set to `true`. | `false` |
| `PLUGIN_AUTH_ADDRESS` | Access Manager base address. | empty |
| `MIDAZ_CLIENT_ID` | Access Manager client ID. | empty |
| `MIDAZ_CLIENT_SECRET` | Access Manager client secret. | empty |
| `MIDAZ_SKIP_AUTH_CHECK` | **Test plumbing only — never set in production.** Bypasses the construction-time gate that catches Access Manager misconfigurations (`PLUGIN_AUTH_ENABLED=true` without `PLUGIN_AUTH_ADDRESS`, `MIDAZ_CLIENT_ID`, or `MIDAZ_CLIENT_SECRET`) before the first request. Skipping it pushes those failures to runtime as 401 cascades. Programmatic configuration cannot set this. | `false` |

`MIDAZ_AUTH_TOKEN` is **not** a configuration environment variable. `config.FromEnvironment()` does not read it, and v3 deliberately exposes no `WithAuthToken` option. The two sanctioned auth paths are `midaz.WithAccessManager(...)` (OAuth via the Lerian Access Manager service) and `midaz.WithAnonymous()` (explicit auth-less mode for local development and tests). Static-token deployments configure their access manager to mint tokens.

## Removed in v3

The following v2 environment variables have been deleted:

| Variable | Why | Migration |
| --- | --- | --- |
| `MIDAZ_ENABLE_RETRIES` | Undocumented hidden killswitch that duplicated `MIDAZ_MAX_RETRIES`. | Set `MIDAZ_MAX_RETRIES=0` to disable retries. |

The following implicit environment reads have been removed from entity constructors and HTTP client builders. They were never documented and now cannot bypass the explicit configuration path:

- `MIDAZ_DEBUG` (was read 14 times across `entities/*.go` plus `entities/http.go`)
- `MIDAZ_USER_AGENT` (was read in `entities/http.go`)
- `MIDAZ_IDEMPOTENCY` (was read in `entities/http.go`)
- `MIDAZ_MAX_RETRIES` (was read in `entities/http.go`)

These variables are still honored — but only when the caller opts in via `config.FromEnvironment()`.

## Authentication

The environment-based authentication path uses Access Manager.

```env
PLUGIN_AUTH_ENABLED=true
PLUGIN_AUTH_ADDRESS=http://localhost:4000
MIDAZ_CLIENT_ID=your-client-id
MIDAZ_CLIENT_SECRET=your-client-secret
```

When `PLUGIN_AUTH_ENABLED=true`, the SDK requests a token from:

```text
{PLUGIN_AUTH_ADDRESS}/v1/login/oauth/access_token
```

The request sends a client credentials payload using `MIDAZ_CLIENT_ID` and `MIDAZ_CLIENT_SECRET`. The returned `accessToken` becomes the `Authorization: Bearer ...` header for Midaz API requests.

If `PLUGIN_AUTH_ENABLED=true` and `PLUGIN_AUTH_ADDRESS` is empty, config validation fails. Tests can set `MIDAZ_SKIP_AUTH_CHECK=true` to bypass this check, but only when configuration goes through `config.FromEnvironment()`.

## Service URLs and precedence

`config.FromEnvironment()` applies URL variables in this order:

1. `MIDAZ_BASE_URL`
2. Service-specific overrides: `MIDAZ_ONBOARDING_URL`, `MIDAZ_TRANSACTION_URL`, `MIDAZ_CRM_URL`
3. Existing config defaults for any service you do not override

Service-specific URLs override `MIDAZ_BASE_URL` for their service.

```env
MIDAZ_BASE_URL=https://midaz.example.com
MIDAZ_ONBOARDING_URL=https://onboarding.example.com/v1
```

With this configuration:

- Onboarding uses `https://onboarding.example.com/v1`
- Transaction uses `https://midaz.example.com/v1`
- CRM uses `https://midaz.example.com/v1`

`MIDAZ_BASE_URL` derives service URLs and appends `/v1` when needed. For localhost without a port, the SDK uses port `3002` for Ledger services and port `4003` for CRM.

```env
MIDAZ_BASE_URL=http://localhost
```

This resolves to:

```text
Onboarding:  http://localhost:3002/v1
Transaction: http://localhost:3002/v1
CRM:         http://localhost:4003/v1
```

`MIDAZ_ENVIRONMENT` recomputes default service URLs unless you explicitly set `MIDAZ_BASE_URL` or service-specific URLs. Explicit URLs always win.

## HTTP behavior

```env
MIDAZ_TIMEOUT=30
MIDAZ_USER_AGENT=MyService/1.0
MIDAZ_DEBUG=true
```

`MIDAZ_DEBUG=true` enables verbose request and response logging. Any value other than `true` leaves debug logging disabled.

## Retry behavior

```env
MIDAZ_MAX_RETRIES=5
```

`MIDAZ_MAX_RETRIES` flows through `config.FromEnvironment()` into the SDK's retry options. To disable retries entirely, set `MIDAZ_MAX_RETRIES=0`.

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(midaz.WithConfig(cfg))
```

The SDK does not read retry wait environment variables. Configure retry timing in code with `midaz.WithRetryOptions(retry.WithInitialDelay(...), retry.WithMaxDelay(...))`.

## Idempotency behavior

```env
MIDAZ_IDEMPOTENCY=true
```

Idempotency is enabled by default. Set it to `false` to disable SDK-generated idempotency behavior.

Automatic key generation applies to unsafe entity HTTP requests. The entity HTTP client generates an `X-Idempotency` UUID only when all of these are true:

- Idempotency support is enabled.
- The HTTP method is unsafe: `POST`, `PUT`, `PATCH`, or `DELETE`.
- The request does not already include `X-Idempotency`.

The SDK removes internal idempotency marker headers before sending the request.

You can also provide an explicit key through context:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"

ctx := sdkctx.WithIdempotencyKey(context.Background(), "transaction-2026-04-27-001")
```

Unsafe requests retry only when the idempotency key was supplied by the caller through `CreateTransactionInput.IdempotencyKey` or `sdkctx.WithIdempotencyKey`. SDK-generated keys provide server-side deduplication, but they do not enable unsafe retries by themselves.

## Observability

Observability is programmatic only. The SDK does not read `MIDAZ_OTEL_ENDPOINT` or `MIDAZ_LOG_LEVEL`.

Configure observability in code and pass the provider to the client.

```go
provider, err := observability.New(ctx,
    observability.WithServiceName("my-service"),
    observability.WithEnvironment("production"),
    observability.WithComponentEnabled(true, true, true),
)
if err != nil {
    return err
}

c, err := midaz.New(
    midaz.WithObservabilityProvider(provider),
    midaz.WithConfig(cfg),
)
```

## Standard library proxy variables

Go's `net/http` package honors the following at the transport level. The SDK does not read them itself, but they affect outbound traffic:

- `HTTP_PROXY` / `http_proxy`
- `HTTPS_PROXY` / `https_proxy`
- `NO_PROXY` / `no_proxy`

These are stdlib conventions and are unaffected by the v3 explicit-config principle.

## Example environment

```env
# Environment label
MIDAZ_ENVIRONMENT=local

# Service URLs
MIDAZ_BASE_URL=http://localhost
MIDAZ_ONBOARDING_URL=http://localhost:3002/v1
MIDAZ_TRANSACTION_URL=http://localhost:3002/v1
MIDAZ_CRM_URL=http://localhost:4003/v1

# Access Manager authentication
PLUGIN_AUTH_ENABLED=true
PLUGIN_AUTH_ADDRESS=http://localhost:4000
MIDAZ_CLIENT_ID=your-client-id
MIDAZ_CLIENT_SECRET=your-client-secret

# HTTP behavior
MIDAZ_TIMEOUT=30
MIDAZ_USER_AGENT=MyService/1.0
MIDAZ_DEBUG=false

# Retries (set to 0 to disable)
MIDAZ_MAX_RETRIES=3

# Idempotency
MIDAZ_IDEMPOTENCY=true
```
