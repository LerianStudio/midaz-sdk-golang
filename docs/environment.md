# Environment configuration

This guide explains which environment variables the Midaz Go SDK reads and how they affect client behavior.

## v3 contract: explicit over implicit

**The SDK reads environment variables only when the caller opts in via `config.FromEnvironment()`.** Every read happens inside `pkg/config/config.go`. No entity constructor, HTTP client builder, or service helper reads `os.Getenv` directly. Setting `MIDAZ_DEBUG=true` in your shell does **nothing** to a client constructed without `FromEnvironment()`.

The SDK does **not** load `.env` files automatically. If you keep configuration in a `.env` file, load it before building the config.

```go
import (
    "github.com/joho/godotenv"

    midaz "github.com/LerianStudio/midaz-sdk-golang/v4"
    "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
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
| `MIDAZ_BASE_URL` | Base URL used to derive Ledger and Tracer plane URLs. | env-derived |
| `MIDAZ_LEDGER_URL` | Explicit Ledger service URL (onboarding + transactions). | derived from `MIDAZ_BASE_URL` |
| `MIDAZ_TRACER_URL` | Explicit Tracer plane URL. Derived from `MIDAZ_BASE_URL` when unset. | derived from `MIDAZ_BASE_URL` |
| `MIDAZ_TRACER_API_KEY` | Optional X-API-Key for the Tracer plane; unset ⇒ shares the Ledger Bearer token. | unset |
| `MIDAZ_TIMEOUT` | HTTP timeout in seconds. | `60` |
| `MIDAZ_DEBUG` | Enables debug logging when set to `true`. | `false` |
| `MIDAZ_MAX_RETRIES` | Maximum retry attempts. Set to `0` to disable retries entirely. | `3` |
| `MIDAZ_IDEMPOTENCY` | Enables (`true`) or disables (`false`) auto idempotency support. | `true` |
| `MIDAZ_ERROR_EXPOSE_BODY` | Attaches raw upstream 4xx/5xx response bodies to SDK errors. Bodies are not redacted by the SDK; enable only for controlled diagnostics. | `false` |
| `PLUGIN_AUTH_ENABLED` | Enables Access Manager authentication when set to `true`. | `false` |
| `PLUGIN_AUTH_ADDRESS` | Access Manager base address. | empty |
| `MIDAZ_CLIENT_ID` | Access Manager client ID. | empty |
| `MIDAZ_CLIENT_SECRET` | Access Manager client secret. | empty |
| `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP` | Allows non-loopback `http://` Access Manager URLs for trusted in-cluster networks. Not allowed with `MIDAZ_ENVIRONMENT=production`. | `false` |
| `MIDAZ_ALLOW_INSECURE_HTTP` | Permits non-loopback `http://` Ledger / Tracer service URLs (`MIDAZ_LEDGER_URL` / `MIDAZ_TRACER_URL` / `MIDAZ_BASE_URL`) for trusted in-cluster networks. Not allowed with `MIDAZ_ENVIRONMENT=production`. | `false` |

`MIDAZ_AUTH_TOKEN` is **not** a configuration environment variable. `config.FromEnvironment()` does not read it, and v3 deliberately exposes no `WithAuthToken` option. The two sanctioned auth paths are `midaz.WithAccessManager(...)` (OAuth via the Lerian Access Manager service) and `midaz.WithAnonymous()` (explicit auth-less mode for local development and tests). Static-token deployments configure their access manager to mint tokens.

## Removed in v3

The following v2 environment variables have been deleted:

| Variable | Why | Migration |
| --- | --- | --- |
| `MIDAZ_ENABLE_RETRIES` | Undocumented hidden killswitch that duplicated `MIDAZ_MAX_RETRIES`. | Set `MIDAZ_MAX_RETRIES=0` to disable retries. |

The following implicit environment reads have been removed from entity constructors and HTTP client builders. They were never documented and now cannot bypass the explicit configuration path:

- `MIDAZ_DEBUG` (was read 14 times across `entities/*.go` plus `entities/http.go`)
- `MIDAZ_IDEMPOTENCY` (was read in `entities/http.go`)
- `MIDAZ_MAX_RETRIES` (was read in `entities/http.go`)

These variables are still honored — but only when the caller opts in via `config.FromEnvironment()`. The previously supported `MIDAZ_USER_AGENT` env var was deleted entirely; the SDK now emits `midaz-go-sdk/<version>` as the default `User-Agent` and exposes only the programmatic `midaz.WithUserAgent` option for overrides.

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

When `PLUGIN_AUTH_ENABLED=true`, config validation also requires `MIDAZ_CLIENT_ID`, `MIDAZ_CLIENT_SECRET`, and an explicit target. Set `MIDAZ_ENVIRONMENT`, `MIDAZ_BASE_URL`, or at least one service-specific URL so the SDK does not silently pair Access Manager credentials with default local service URLs.

Access Manager URLs are strict by default. Use `https://` for remote targets. Plain `http://` is accepted only for loopback hosts such as `localhost` and `127.0.0.1`. If you run Access Manager behind a trusted in-cluster Kubernetes Service or service mesh, set `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP=true` to permit a non-loopback `http://` address. This escape hatch is rejected in production.

## Service URLs and precedence

The consolidated server exposes two planes: the **Ledger plane** (port `3002`,
serving what used to be split as onboarding + transactions) and the **Tracer
plane** (port `4020`). Both are served under `/v1`.

`config.FromEnvironment()` applies URL variables in this order:

1. `MIDAZ_BASE_URL`
2. Plane-specific overrides: `MIDAZ_LEDGER_URL`, `MIDAZ_TRACER_URL`
3. Existing config defaults for any plane you do not override

Plane-specific URLs override `MIDAZ_BASE_URL` for their plane.

```env
MIDAZ_BASE_URL=https://midaz.example.com
MIDAZ_LEDGER_URL=https://ledger.example.com/v1
```

With this configuration:

- Ledger (onboarding + transactions) uses `https://ledger.example.com/v1`
- Tracer uses `https://midaz.example.com/v1`

`MIDAZ_BASE_URL` derives plane URLs and appends `/v1` when needed. For localhost without a port, the SDK uses port `3002` for the Ledger plane and port `4020` for the Tracer plane.

```env
MIDAZ_BASE_URL=http://localhost
```

This resolves to:

```text
Ledger:  http://localhost:3002/v1
Tracer:  http://localhost:4020/v1
```

`MIDAZ_ENVIRONMENT` recomputes default plane URLs unless you explicitly set `MIDAZ_BASE_URL` or plane-specific URLs. Explicit URLs always win.

## HTTP behavior

```env
MIDAZ_TIMEOUT=30
MIDAZ_DEBUG=true
```

The SDK no longer reads a `MIDAZ_USER_AGENT` env var. The default `User-Agent` is `midaz-go-sdk/<version>`; override programmatically with `midaz.WithUserAgent("my-app/1.0")` when needed.

`MIDAZ_DEBUG` uses Go's strict `strconv.ParseBool` forms. Values such as `true`, `false`, `1`, and `0` are valid. Values such as `yes`, `no`, `on`, and `off` return a configuration error instead of silently defaulting.

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

Idempotency is enabled by default for ordinary unsafe SDK requests. Set it to `false` to disable SDK-generated idempotency behavior. Transaction and HTTP batch retries are stricter: when their retry count is greater than zero, callers must supply stable per-transaction or per-item idempotency keys so replayed batch attempts are deterministic.

Automatic key generation applies to unsafe entity HTTP requests. The entity HTTP client generates an `X-Idempotency` UUID only when all of these are true:

- Idempotency support is enabled.
- The HTTP method is unsafe: `POST`, `PUT`, `PATCH`, or `DELETE`.
- The request does not already include `X-Idempotency`.

The SDK sends only the public `X-Idempotency` header; it does not emit internal marker headers.

You can also provide an explicit key through context:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"

ctx := sdkctx.WithIdempotencyKey(context.Background(), "transaction-2026-04-27-001")
```

Unsafe requests retry only when `X-Idempotency` is present. Caller-supplied and SDK-generated keys both satisfy this retry gate.

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
MIDAZ_LEDGER_URL=http://localhost:3002/v1
MIDAZ_TRACER_URL=http://localhost:4020/v1

# Access Manager authentication
PLUGIN_AUTH_ENABLED=true
PLUGIN_AUTH_ADDRESS=http://localhost:4000
MIDAZ_CLIENT_ID=your-client-id
MIDAZ_CLIENT_SECRET=your-client-secret
# Optional: only for trusted non-loopback http:// Access Manager URLs
MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP=false

# HTTP behavior
MIDAZ_TIMEOUT=30
MIDAZ_DEBUG=false

# Retries (set to 0 to disable)
MIDAZ_MAX_RETRIES=3

# Idempotency
MIDAZ_IDEMPOTENCY=true
```
