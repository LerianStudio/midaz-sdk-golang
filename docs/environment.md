# Environment configuration

This guide explains which environment variables the Midaz Go SDK reads and how they affect client behavior.

The SDK reads environment variables only from the current process environment. It does **not** load `.env` files by itself. If you keep configuration in a `.env` file, load it before creating the SDK config.

```go
_ = godotenv.Load()

cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := client.New(
    client.WithConfig(cfg),
    client.UseEntityAPI(),
)
if err != nil {
    return err
}

_ = c
```

## Supported environment variables

| Variable | Purpose | Default |
| --- | --- | --- |
| `MIDAZ_ENVIRONMENT` | Stores the environment label. Valid values: `local`, `development`, `production`. | `local` |
| `MIDAZ_BASE_URL` | Base URL used to derive Ledger and CRM service URLs. | Local defaults |
| `MIDAZ_ONBOARDING_URL` | Explicit Onboarding service URL. | Local Ledger URL |
| `MIDAZ_TRANSACTION_URL` | Explicit Transaction service URL. | Local Ledger URL |
| `MIDAZ_CRM_URL` | Explicit CRM service URL. | Local CRM URL |
| `MIDAZ_TIMEOUT` | HTTP timeout in seconds. | `60` |
| `MIDAZ_USER_AGENT` | User agent header value. | SDK version user agent |
| `MIDAZ_DEBUG` | Enables debug logging when set to `true`. | `false` |
| `MIDAZ_MAX_RETRIES` | Maximum retry attempts. | `3` |
| `MIDAZ_ENABLE_RETRIES` | Disables retries only for direct `entities.NewHTTPClient` usage when set to `false`. | Enabled |
| `MIDAZ_IDEMPOTENCY` | Enables or disables SDK idempotency support. | `true` |
| `PLUGIN_AUTH_ENABLED` | Enables Access Manager authentication when set to `true`. | `false` |
| `PLUGIN_AUTH_ADDRESS` | Access Manager base address. | Empty |
| `MIDAZ_CLIENT_ID` | Access Manager client ID. | Empty |
| `MIDAZ_CLIENT_SECRET` | Access Manager client secret. | Empty |
| `MIDAZ_SKIP_AUTH_CHECK` | Testing-only bypass for missing Access Manager address validation. | `false` |

`MIDAZ_AUTH_TOKEN` is not a configuration environment variable. `config.FromEnvironment()` does not read it.

## Loading `.env` files

The SDK calls `os.Getenv`. It does not parse `.env` files automatically.

If you want to use `.env`, load it in your application before you call `config.FromEnvironment()`.

```go
import (
    "github.com/joho/godotenv"

    client "github.com/LerianStudio/midaz-sdk-golang/v2"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

func newClient() (*client.Client, error) {
    _ = godotenv.Load()

    cfg, err := config.NewConfig(config.FromEnvironment())
    if err != nil {
        return nil, err
    }

    return client.New(
        client.WithConfig(cfg),
        client.UseEntityAPI(),
    )
}
```

## Authentication

The environment-based authentication path uses Access Manager.

```env
PLUGIN_AUTH_ENABLED=true
PLUGIN_AUTH_ADDRESS=http://localhost:4000
MIDAZ_CLIENT_ID=your-client-id
MIDAZ_CLIENT_SECRET=your-client-secret
```

When `PLUGIN_AUTH_ENABLED=true`, the entity client requests a token from:

```text
{PLUGIN_AUTH_ADDRESS}/v1/login/oauth/access_token
```

The request sends a client credentials payload using `MIDAZ_CLIENT_ID` and `MIDAZ_CLIENT_SECRET`. The returned `accessToken` becomes the `Authorization: Bearer ...` header for Midaz API requests.

If `PLUGIN_AUTH_ENABLED=true` and `PLUGIN_AUTH_ADDRESS` is empty, config validation fails unless `MIDAZ_SKIP_AUTH_CHECK=true` is set for tests.

## Service URLs and precedence

`config.FromEnvironment()` applies URL variables in this order:

1. `MIDAZ_BASE_URL`
2. Service-specific overrides: `MIDAZ_ONBOARDING_URL`, `MIDAZ_TRANSACTION_URL`, and `MIDAZ_CRM_URL`
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

Note: in the current codebase, `MIDAZ_ENVIRONMENT` stores the environment label but does not recompute service URLs by itself after defaults are initialized. Set `MIDAZ_BASE_URL` or explicit service URLs when you need non-local endpoints.

## HTTP behavior

Set `MIDAZ_TIMEOUT` as an integer number of seconds.

```env
MIDAZ_TIMEOUT=30
MIDAZ_USER_AGENT=MyService/1.0
MIDAZ_DEBUG=true
```

`MIDAZ_DEBUG=true` enables verbose request and response logging. Any value other than `true` leaves debug logging disabled.

## Retry behavior

The SDK supports retry configuration from both config and the entity HTTP layer.

```env
MIDAZ_MAX_RETRIES=5
```

`MIDAZ_MAX_RETRIES` is read by:

- `config.FromEnvironment()`
- `entities.NewHTTPClient()`

For the normal client path, use `config.FromEnvironment()` and `client.WithConfig(cfg)`. The client applies `cfg.MaxRetries` to the entity HTTP client during setup.

`MIDAZ_ENABLE_RETRIES=false` is read only by `entities.NewHTTPClient()`. The normal client config does not read this variable, and client setup may override the entity HTTP value from config defaults.

To disable retries at the client level, use `client.DisableRetries()`.

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := client.New(
    client.WithConfig(cfg),
    client.DisableRetries(),
    client.UseEntityAPI(),
)
```

The SDK does not read retry wait environment variables. Configure retry timing in code with `client.WithRetries(...)` or config retry options.

## Idempotency behavior

`MIDAZ_IDEMPOTENCY` controls whether the SDK may add idempotency headers.

```env
MIDAZ_IDEMPOTENCY=true
```

Idempotency support is enabled by default. Set it to `false` to disable SDK-generated idempotency behavior.

Automatic key generation is opt-in per request path. The entity HTTP client generates an `X-Idempotency` UUID only when all of these are true:

- Idempotency support is enabled.
- The HTTP method is unsafe: `POST`, `PUT`, `PATCH`, or `DELETE`.
- The request does not already include `X-Idempotency`.
- The request includes the internal opt-in header `X-Midaz-Auto-Idempotency: true`.

The SDK removes `X-Midaz-Auto-Idempotency` before sending the request.

Transaction creation opts in automatically. If you set `CreateTransactionInput.IdempotencyKey`, the SDK uses your key. Otherwise, it generates one when idempotency is enabled.

You can also provide an explicit key through context:

```go
ctx := entities.WithIdempotencyKey(context.Background(), "transaction-2026-04-27-001")
```

Unsafe requests without an `X-Idempotency` header do not retry, even when retries are enabled.

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

c, err := client.New(
    client.WithObservabilityProvider(provider),
    client.UseEntityAPI(),
)
if err != nil {
    return err
}

_ = c
```

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

# Retries
MIDAZ_MAX_RETRIES=3

# Idempotency
MIDAZ_IDEMPOTENCY=true
```
