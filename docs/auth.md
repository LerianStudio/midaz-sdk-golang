# Authentication

The Midaz Go SDK has exactly two authentication paths in v3, and a client cannot be constructed without choosing one. This is deliberate: v2 silently accepted credential-less construction and surfaced the mistake as a 401 on the first API call. v3 closes that footgun at construction time with a typed configuration error.

## Quick reference

| Path | When to use |
| --- | --- |
| [`midaz.WithAccessManager`](#access-manager-oauth-via-the-lerian-access-manager) | Production. Tokens are minted by the Lerian Access Manager service from your `clientId`/`clientSecret`. |
| `midaz.WithAllowInsecureAccessManagerHTTP` | Trusted in-cluster networks only. Permits non-loopback `http://` Access Manager URLs when HTTPS terminates outside the SDK path. |
| [`midaz.WithAnonymous`](#anonymous-mode-localdev-and-tests) | Explicit caller-owned unsafe mode for an unsecured target stack, integration tests, or read-only inspection where the operator has confirmed the target endpoints don't require auth. Not recommended for production. |

There is intentionally no static-token (`WithAuthToken`) option. Static-token deployments configure their Access Manager to mint tokens.

## Access Manager (OAuth via the Lerian Access Manager)

This is the production-grade path. The SDK exchanges a `clientId` + `clientSecret` for a bearer token via the OAuth client-credentials flow, caches the token, and refreshes it automatically.

```go
import (
    "log"
    "os"

    "github.com/LerianStudio/midaz-sdk-golang/v6"
)

func main() {
    c, err := midaz.New(
        midaz.WithEnvironment(midaz.EnvironmentProduction),
        midaz.WithAccessManager(midaz.AccessManager{
            Address:      "https://auth.midaz.io",
            ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
            ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
        }),
    )
    if err != nil {
        log.Fatalf("midaz.New: %v", err)
    }

    // c is ready to issue authenticated requests.
}
```

### Behavior at construction time

1. `midaz.New` validates the supplied `AccessManager`. `Address`, `ClientID`, and `ClientSecret` are all required.
2. `Enabled` is auto-set to `true` — you do not touch the field. The act of calling `WithAccessManager` is the opt-in.
3. The SDK requires an explicit Midaz target when Access Manager is enabled. Set `WithEnvironment`, `WithBaseURL`, or a service-specific URL option so credentials are not paired with default local URLs by accident.
4. `entities.NewEntityWithConfig` eagerly fetches an initial token from `Address` so misconfigurations surface as construction errors, not as 401s on the first request.
5. The token is cached in-process and refreshed by the SDK on 401 responses.

### Access Manager URL security

Access Manager receives your `clientId` and `clientSecret`, so the SDK rejects plaintext remote token endpoints by default.

- Use `https://` for production and remote environments.
- Use plain `http://` only for loopback hosts such as `localhost` or `127.0.0.1`.
- Use `midaz.WithAllowInsecureAccessManagerHTTP(true)` or `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP=true` only for trusted in-cluster service names, such as a Kubernetes Service protected by a service mesh or private network segment.

The insecure HTTP opt-in is rejected when the SDK environment is `production`. When you enable it for a non-production in-cluster target, `midaz.New` emits a warning so the override is visible in logs.

### Loading credentials from the environment

`config.FromEnvironment()` reads `PLUGIN_AUTH_ENABLED`, `PLUGIN_AUTH_ADDRESS`, `MIDAZ_CLIENT_ID`, `MIDAZ_CLIENT_SECRET`, and `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP`. Pass `WithConfig(config.NewConfig(config.FromEnvironment(), ...))` to opt in:

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(midaz.WithConfig(cfg))
```

When `PLUGIN_AUTH_ENABLED=true` is in the environment, the resulting config has `AccessManager.Enabled=true` and behaves exactly as if `WithAccessManager` were called programmatically. Validation still requires `PLUGIN_AUTH_ADDRESS`, `MIDAZ_CLIENT_ID`, `MIDAZ_CLIENT_SECRET`, and an explicit target through `MIDAZ_ENVIRONMENT`, `MIDAZ_BASE_URL`, or a service-specific URL.

## Anonymous mode (local-dev and tests)

`WithAnonymous()` is the explicit auth-less path. Use it only when the target stack does not enforce authentication:

```go
import (
    "log"

    "github.com/LerianStudio/midaz-sdk-golang/v6"
)

func main() {
    c, err := midaz.New(
        midaz.WithEnvironment(midaz.EnvironmentLocal),
        midaz.WithBaseURL("http://localhost:3000"),
        midaz.WithAnonymous(),
    )
    if err != nil {
        log.Fatalf("midaz.New: %v", err)
    }

    // c issues unauthenticated requests.
}
```

The SDK emits no `Authorization` header in this mode. Treat this as a caller-owned unsafe mode: if you point it at non-local or production URLs, you are asserting that authentication is enforced outside the SDK (for example by a trusted private gateway) and accepting the operational risk. Do not use it as a shortcut for missing credentials. Production applications should use Access Manager.

The HTTP client otherwise behaves identically — retries, idempotency, slow-call logging, and observability all work.

## Tracer plane authentication

The two paths above choose credentials for `midaz.New` as a whole. Within that, the Tracer plane has an independent per-plane override.

By default the Tracer plane shares the Ledger's Access Manager Bearer token (or sends no `Authorization` header in anonymous mode). Setting `midaz.WithTracerAPIKey(...)`, or the `MIDAZ_TRACER_API_KEY` environment variable, makes Tracer-plane calls authenticate with an `X-API-Key` header carrying that value instead of the shared Bearer token:

```go
c, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(midaz.AccessManager{ /* ... */ }),
    midaz.WithTracerAPIKey(os.Getenv("MIDAZ_TRACER_API_KEY")),
)
```

This is a per-plane override, not a third construction-time auth gate: it does not satisfy the auth-required gate on its own, and an empty key is a no-op that leaves the Tracer plane on the shared Bearer token.

## Retry and idempotency boundaries

The SDK sends only `X-Idempotency`; it does not send `Idempotency-Key`. The Midaz server contract currently accepts `X-Idempotency`.

Automatic retries for unsafe requests require `X-Idempotency`, but transaction action endpoints have additional server-contract boundaries: commit and cancel do not honor `X-Idempotency`, and revert is not cleanly endpoint-idempotent. The SDK suppresses automatic retries for those actions rather than implying duplicate-safe behavior.

## Auth-required gate

Without either `WithAccessManager` or `WithAnonymous`, `midaz.New` returns a typed `*errors.Error` with `Category == CategoryConfiguration`:

```text
configuration error during midaz.New: invalid configuration: no auth source configured; use WithAccessManager or WithAnonymous
```

Detect with the standard helpers. The SDK's `pkg/errors` does not re-export
`As`/`Is`, so the example imports the stdlib `errors` package alongside the
SDK alias:

```go
import (
    "errors"

    "github.com/LerianStudio/midaz-sdk-golang/v6"
    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

c, err := midaz.New(midaz.WithEnvironment(midaz.EnvironmentLocal))
if err != nil {
    var sdkErr *sdkerrors.Error
    if errors.As(err, &sdkErr) && sdkErr.Category == sdkerrors.CategoryConfiguration {
        // Setup mistake — fix the call site.
    }
    return err
}
```

Or check by sentinel:

```go
if errors.Is(err, sdkerrors.ErrConfiguration) { ... }
```

## Mutual exclusion

`WithAccessManager` and `WithAnonymous` are mutually exclusive. The last-applied option wins:

```go
// Anonymous wins:
midaz.New(
    midaz.WithAccessManager(midaz.AccessManager{Address: "..."}),
    midaz.WithAnonymous(),
)

// Access Manager wins:
midaz.New(
    midaz.WithAnonymous(),
    midaz.WithAccessManager(midaz.AccessManager{Address: "..."}),
)
```

`WithAnonymous` clears `AccessManager.Enabled` (preserving `Address`/`ClientID`/`ClientSecret` so env-driven introspection still works). `WithAccessManager` clears `Anonymous`.

## Migration from v2

| v2 pattern | v3 replacement |
| --- | --- |
| `client.WithAuthToken("token")` | Not available. Configure your Access Manager to mint tokens. |
| `c.SetAuthToken("token")` post-construction | Not available. Tokens flow only through Access Manager. |
| `entities.WithPluginAuth(...)` | `midaz.WithAccessManager(...)` |
| `pkg/access-manager` import | `pkg/auth` (directory matches package name) |
| Construction with no auth source | Add `midaz.WithAnonymous()` for tests, or `midaz.WithAccessManager(...)` for production. |
