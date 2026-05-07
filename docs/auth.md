# Authentication

The Midaz Go SDK has exactly two authentication paths in v3, and a client cannot be constructed without choosing one. This is deliberate: v2 silently accepted credential-less construction and surfaced the mistake as a 401 on the first API call. v3 closes that footgun at construction time with a typed configuration error.

## Quick reference

| Path | When to use |
| --- | --- |
| [`midaz.WithAccessManager`](#access-manager-oauth-via-the-lerian-access-manager) | Production. Tokens are minted by the Lerian Access Manager service from your `clientId`/`clientSecret`. |
| [`midaz.WithAnonymous`](#anonymous-mode-localdev-and-tests) | Explicit opt-out for an unsecured target stack, integration tests, or read-only inspection where the operator has confirmed the target endpoints don't require auth. |

There is intentionally no static-token (`WithAuthToken`) option. Static-token deployments configure their Access Manager to mint tokens.

## Access Manager (OAuth via the Lerian Access Manager)

This is the production-grade path. The SDK exchanges a `clientId` + `clientSecret` for a bearer token via the OAuth client-credentials flow, caches the token, and refreshes it automatically.

```go
import (
    "log"
    "os"

    "github.com/LerianStudio/midaz-sdk-golang/v3"
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
3. `entities.NewEntityWithConfig` eagerly fetches an initial token from `Address` so misconfigurations surface as construction errors, not as 401s on the first request.
4. The token is cached in-process and refreshed by the SDK on 401 responses.

### Loading credentials from the environment

`config.FromEnvironment()` reads `PLUGIN_AUTH_ENABLED`, `PLUGIN_AUTH_ADDRESS`, `MIDAZ_CLIENT_ID`, and `MIDAZ_CLIENT_SECRET`. Pass `WithConfig(config.NewConfig(config.FromEnvironment(), ...))` to opt in:

```go
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(midaz.WithConfig(cfg))
```

When `PLUGIN_AUTH_ENABLED=true` is in the environment, the resulting config has `AccessManager.Enabled=true`, satisfies the auth-required gate, and behaves exactly as if `WithAccessManager` were called programmatically.

## Anonymous mode (local-dev and tests)

`WithAnonymous()` is the explicit auth-less path. Use it only when the target stack does not enforce authentication:

```go
import (
    "log"

    "github.com/LerianStudio/midaz-sdk-golang/v3"
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

The SDK emits no `Authorization` header in this mode. This is an explicit caller opt-out, not an environment gate: the SDK allows it even when the configured URLs point at production, because self-hosted or proxy-fronted deployments may intentionally provide their own authentication layer outside the SDK. The HTTP client otherwise behaves identically — retries, idempotency, slow-call logging, observability all work.

## Auth-required gate

Without either `WithAccessManager` or `WithAnonymous`, `midaz.New` returns a typed `*errors.Error` with `Category == CategoryConfiguration`:

```text
configuration error during midaz.New: invalid configuration: no auth source configured; use WithAccessManager or WithAnonymous
```

Detect with the standard helpers:

```go
c, err := midaz.New(midaz.WithEnvironment(midaz.EnvironmentLocal))
if err != nil {
    var sdkErr *errors.Error
    if errors.As(err, &sdkErr) && sdkErr.Category == errors.CategoryConfiguration {
        // Setup mistake — fix the call site.
    }
    return err
}
```

Or check by sentinel:

```go
if errors.Is(err, errors.ErrConfiguration) { ... }
```

### Bypassing the gate (test plumbing only)

Setting `MIDAZ_SKIP_AUTH_CHECK=true` in the environment AND loading the config via `config.FromEnvironment()` bypasses the gate. This exists for tests that exercise partial-config code paths and is never the right answer for production code — it disables the construction-time check that catches misconfigurations (Access Manager enabled without an address or credentials) before any request goes out, and quietly defers those failures to runtime as 401 cascades.

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

See [docs/v3-dx-plan.md](v3-dx-plan.md) for the full design rationale.
