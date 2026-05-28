# Configuration Guide

This is the canonical reference for configuring the Midaz Go SDK. It explains
the four configuration surfaces, the precedence order between them, every
environment variable the SDK reads, and the per-request override patterns.

> **Audience.** Anyone calling `midaz.New(...)` who wants to know "where
> does this knob live?" or "why didn't my override take?". Read sections 1
> and 2 to understand the model; section 3 is reference; sections 4 and 5
> are recipes.

---

## 1. The four configuration surfaces

The SDK has four distinct surfaces for setting configuration. Each owns a
different scope and lifetime. Knowing which surface to reach for is the
single biggest source of confusion in v2 — Track 6 of the v3 DX sweep
formalized the contract documented below.

```
┌─────────────────────────────────────────────────────────────────────┐
│  midaz.With*  ────────►  pkg/config.With*  ─────►  Config struct    │
│  (1) user-facing         (2) internal/test layer    (3) state       │
│      Options                  Options                                │
│                                                                      │
│  Per-request:  sdkctx.With*  ─►  context.Context  ─►  per-request   │
│  (4) request-level overrides                          state          │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.1 `midaz.With*` — the user-facing entry point

These are what you pass to `midaz.New(...)`. Each one is a thin wrapper
that delegates to the corresponding `pkg/config.With*` Option (when there
is one) or operates directly on the `Client` struct (for client-only
concerns like the logger or per-request retry overrides).

```go
client, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(midaz.AccessManager{
        Address:      os.Getenv("PLUGIN_AUTH_ADDRESS"),
        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
    }),
    midaz.WithTimeout(45*time.Second),
    midaz.WithUserAgent("my-app/2.0"),
)
if err != nil {
    return err
}
```

**You should reach for this surface 95% of the time.**

The full list (v3):

| Option | Concern |
|---|---|
| `WithAccessManager` | Plugin-based authentication (OAuth M2M) |
| `WithAllowInsecureAccessManagerHTTP` | Permit non-loopback `http://` Access Manager URLs for trusted in-cluster networks |
| `WithAnonymous` | Disable authentication (testing/local) |
| `WithBaseURL` | Override service base URL |
| `WithConfig` | Use a pre-built `*config.Config` (advanced) |
| `WithContext` | Override the client's default context |
| `WithCRMURL` | Override CRM service URL |
| `WithCustomRetryPolicy` | Per-response retry decision callback |
| `WithDebug` | Enable verbose request/response logging |
| `WithEnvironment` | Select production / development / local |
| `WithErrorBodyExposure` | Toggle raw upstream 4xx/5xx response body exposure on SDK errors |
| `WithHTTPClient` | Replace the underlying `*http.Client` |
| `WithIdempotency` | Toggle automatic `X-Idempotency` header |
| `WithLedgerURL` | Override Ledger service URL (onboarding + transactions) |
| `WithLogger` | Install a custom `*slog.Logger` |
| `WithObservabilityOptions` | Build OTel provider from `observability.Option` chain |
| `WithObservabilityProvider` | Install a pre-built `observability.Provider` |
| `WithoutRetries` | Disable the retry mechanism (`MaxRetries=0`) |
| `WithRetryOptions` | Thread `retry.Option` chain onto entity HTTPClient |
| `WithSlowCallThreshold` | Warn-level log when request exceeds duration |
| `WithTimeout` | HTTP request timeout |
| `WithUserAgent` | Override `User-Agent` header |

### 1.2 `pkg/config.With*` — the internal/test layer

These operate directly on a `*config.Config`. Most callers should not
invoke them through `midaz.New(...)` — instead use the `midaz.With*`
wrapper above. There are three legitimate reasons to reach for this
layer:

1. **Building a `*Config` separately** to inspect or modify before
   passing via `midaz.WithConfig(cfg)`.
2. **Tests that exercise pure configuration logic** without firing
   `NewEntityWithConfig`'s eager token-fetch path.
3. **Three retry knobs that have no `midaz.With*` wrapper by design:**
   `WithMaxRetries`, `WithRetryWaitMin`, `WithRetryWaitMax`. The midaz
   layer exposes retry tuning through `WithRetryOptions(retry.Option...)`
   instead, which composes uniformly with every other `retry.Option`.

```go
// Pattern (1): build cfg separately
cfg, err := config.NewConfig(
    config.WithEnvironment(config.EnvironmentProduction),
    config.WithAccessManager(auth.AccessManager{
        Address:      os.Getenv("PLUGIN_AUTH_ADDRESS"),
        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
    }),
    config.WithUserAgent("worker-pool/1.0"),
    config.WithMaxRetries(5),         // valid Config-only knob
    config.WithRetryWaitMin(200*time.Millisecond),
    config.WithRetryWaitMax(10*time.Second),
)
if err != nil { return err }
client, err := midaz.New(midaz.WithConfig(cfg))
if err != nil { return err }
```

**Two-layer parity is enforced by CI.** The script at
`scripts/check-config-parity.sh` runs in `make verify-sdk` (and therefore
`make ci`) and fails the build if any `pkg/config.With*` Option is added
without a matching `midaz.With*` wrapper, except for the three retry
knobs in the documented allow-list.

### 1.3 `Config` struct — the state

The actual configuration lives on `*config.Config`. After construction you
can read fields via `client.GetConfig()` or `client.GetConfiguration()`.
Both methods return an independent clone, not the live client configuration.
Mutating the returned value does not change the running client. Direct struct
mutation post-construction is not a runtime-tuning mechanism — prefer the
post-construction setters listed in section 1.4.

```go
fmt.Printf("environment: %s\n", client.GetConfig().Environment)
fmt.Printf("max retries: %d\n", client.GetConfig().MaxRetries)
fmt.Printf("user agent:  %s\n", client.GetConfig().UserAgent)
```

### 1.4 Post-construction setters on `*entities.HTTPClient` and `*entities.Entity`

Some concerns are tunable after the client is built. These are exposed as
canonical setters (matching the `SetX` Go idiom rather than the
construction-time `WithX` Option idiom). Reach for these when you need to
flip a knob mid-lifetime — for example, enabling debug mode based on a
runtime flag, or rotating the logger.

```go
// On *entities.HTTPClient:
client.GetEntityHTTPClient().SetDebug(true)
client.GetEntityHTTPClient().SetUserAgent("rotated-ua/2.0")
client.GetEntityHTTPClient().SetLogger(newLogger)
client.GetEntityHTTPClient().SetSlowCallThreshold(2*time.Second)
client.GetEntityHTTPClient().SetEnableIdempotency(false)
client.GetEntityHTTPClient().SetCustomRetryPolicy(myPolicyFn)

// On *entities.Entity:
client.SetObservability(newProvider)  // returns error
```

### 1.5 `sdkctx.With*` — per-request overrides

These functions take a `context.Context` and return a new context with a
specific override attached. They're scoped to a single API call (or any
sub-context derived from them). **Per-request overrides ALWAYS take
precedence over client-level configuration.**

```go
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"

ctx := sdkctx.WithIdempotencyKey(context.Background(), "user-action-42-2026-05-06")

// This single call uses the explicit key instead of an auto-generated UUID.
_, err := client.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
```

The full list:

| Function | Scope |
|---|---|
| `sdkctx.WithIdempotencyKey(ctx, key)` | Set an explicit `X-Idempotency` header (overrides auto-generation) |
| `sdkctx.WithoutAutoIdempotency(ctx)` | Skip auto-generation of `X-Idempotency` for this request |
| `sdkctx.WithoutHTTPRetries(ctx)` | Suppress the SDK HTTP retry loop for this request |
| `sdkctx.WithIncludeDeleted(ctx, true)` | Include soft-deleted resources in list responses |
| `sdkctx.WithHardDelete(ctx, true)` | Use hard-delete instead of soft-delete |

---

## 2. Precedence rules

When the same concern is set at multiple surfaces, the SDK resolves the
conflict in this order (highest to lowest priority):

```
┌──────────────────────────────────────────────────────────────────┐
│  1. Per-request context  (sdkctx.With*)                          │
│  2. Client-level option  (midaz.With* / pkg/config.With*)        │
│  3. Environment variable (read by config.FromEnvironment)        │
│  4. Default value        (config.DefaultConfig)                  │
└──────────────────────────────────────────────────────────────────┘
```

### 2.1 Concrete examples

**Tenant resolution:** tenant scope is derived from Access Manager/JWT claims.
The SDK does not expose tenant configuration and does not send `X-Tenant-ID`.
Use separate Access Manager credentials/token context when a workload needs a
different tenant scope.

**Retry behavior:**

```go
// Default: 3 retries, 1s..30s exponential backoff
// Env:     MIDAZ_MAX_RETRIES=5
// Option:  midaz.WithRetryOptions(retry.WithMaxRetries(2), retry.WithMaxDelay(5*time.Second))
//          OR midaz.WithoutRetries()  (disables; MaxRetries = 0)

// midaz.WithoutRetries is "soft" — a later WithRetryOptions wins:
midaz.New(
    midaz.WithoutRetries(),
    midaz.WithRetryOptions(retry.WithMaxRetries(2)),  // wins; 2 retries
)
```

**Option order within a single `midaz.New(...)` call:**

Options run sequentially in the order written. Later writes win when two
Options touch the same field. This is true for both `midaz.With*` and
`pkg/config.With*`.

```go
midaz.New(
    midaz.WithDebug(false),
    midaz.WithDebug(true),  // wins; Config.Debug == true
)
```

### 2.2 The one exception: `WithObservabilityOptions` and `WithObservabilityProvider`

These two REPLACE the entire observability provider rather than mutating
specific fields. The default disabled provider that `midaz.New()` installs
at construction time is overwritten the first time either of these
options runs. Subsequent calls likewise replace. No merge step. See the
godoc on each function for the full explanation.

---

## 3. Environment variables

The SDK reads environment variables only when the caller explicitly opts
in via `config.NewConfig(config.FromEnvironment())`. Calling `midaz.New()`
without that option produces a client that has never touched the
environment.

| Variable | Type | Default | Behavior |
|---|---|---|---|
| `MIDAZ_ENVIRONMENT` | enum | `local` | One of `production`, `development`, `local` |
| `MIDAZ_BASE_URL` | URL | (env-derived) | Override the unified base URL for all services |
| `MIDAZ_LEDGER_URL` | URL | (env-derived) | Override the Ledger service URL (onboarding + transactions). Wins over `MIDAZ_BASE_URL` |
| `MIDAZ_CRM_URL` | URL | (env-derived) | Override only the CRM service URL |
| `MIDAZ_TIMEOUT` | duration | `60s` | HTTP request timeout |
| `MIDAZ_DEBUG` | bool | `false` | Enable verbose request/response logging (also upgrades the default logger to stderr) |
| `MIDAZ_MAX_RETRIES` | int | `3` | Maximum retry attempts; `0` disables retries |
| `MIDAZ_IDEMPOTENCY` | bool | `true` | Toggle automatic `X-Idempotency` header generation |
| `MIDAZ_ERROR_EXPOSE_BODY` | bool | `false` | Attach raw upstream 4xx/5xx response bodies to SDK errors. Bodies are not redacted by the SDK; enable only for tightly controlled diagnostics and never as a production default. |
| `PLUGIN_AUTH_ENABLED` | bool | `false` | Enable plugin-based OAuth authentication |
| `PLUGIN_AUTH_ADDRESS` | URL | — | Auth plugin endpoint (required when `PLUGIN_AUTH_ENABLED=true`) |
| `MIDAZ_CLIENT_ID` | string | — | OAuth M2M client ID (required when `PLUGIN_AUTH_ENABLED=true`) |
| `MIDAZ_CLIENT_SECRET` | string | — | OAuth M2M client secret (required when `PLUGIN_AUTH_ENABLED=true`) |
| `MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP` | bool | `false` | Permit non-loopback `http://` Access Manager URLs for trusted in-cluster networks. Not allowed with `MIDAZ_ENVIRONMENT=production`. |

> Boolean parsing uses Go's [`strconv.ParseBool`](https://pkg.go.dev/strconv#ParseBool)
> and accepts only its canonical forms: `1`, `t`, `T`, `TRUE`, `true`, `True`,
> `0`, `f`, `F`, `FALSE`, `false`, and `False`. Any other value (including
> `yes`/`no`/`on`/`off`) returns a configuration error rather than silently
> defaulting — a typo no longer flips a flag the wrong way.

> `MIDAZ_DEBUG=true` has a secondary effect: when no `WithLogger` was
> passed, it upgrades the default-discard logger to a stderr text handler
> at debug level. User-supplied `WithLogger` always wins.

### 3.1 Loading the environment

```go
// Pattern: load env into Config, then construct client.
cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}
client, err := midaz.New(midaz.WithConfig(cfg))
```

You can also layer client-level options on top of the env-loaded Config:

```go
cfg, _ := config.NewConfig(config.FromEnvironment())  // base from env
client, _ := midaz.New(
    midaz.WithConfig(cfg),
    midaz.WithUserAgent("my-app/1.0"),  // override the default versioned User-Agent
    midaz.WithDebug(true),              // overrides MIDAZ_DEBUG
)
```

---

## 4. Per-request overrides with sdkctx

Many SDK calls accept a `context.Context` as their first parameter. The
`sdkctx` package provides helpers that thread per-request overrides
through that context, taking precedence over any client-level
configuration.

### 4.1 Idempotency keys

By default, the SDK generates a UUID-based `X-Idempotency` header for
every unsafe HTTP method (POST/PUT/PATCH/DELETE). You can override the
auto-generated key for a specific call:

```go
// Use an explicit key (e.g. derived from a user-action ID):
ctx := sdkctx.WithIdempotencyKey(context.Background(),
    fmt.Sprintf("user-%d-action-%s", userID, actionUUID))
_, err := client.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)

// Skip auto-generation entirely (caller does NOT want the header):
ctx := sdkctx.WithoutAutoIdempotency(context.Background())
_, err = client.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
```

### 4.2 Retry suppression

Use `WithoutHTTPRetries` when a higher-level operation already owns the retry
budget and you need to avoid retry amplification:

```go
ctx := sdkctx.WithoutHTTPRetries(context.Background())
_, err := client.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
```

### 4.3 Soft-delete vs hard-delete

```go
// Include soft-deleted records in a list response:
ctx := sdkctx.WithIncludeDeleted(context.Background(), true)
list, _ := client.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)

// Hard-delete instead of soft-delete (irreversible; admin only):
ctx := sdkctx.WithHardDelete(context.Background(), true)
err := client.Accounts.DeleteAccount(ctx, orgID, ledgerID, accountID)
```

---

## 5. Common patterns

### 5.1 Production client (env-based)

```go
import (
    "github.com/LerianStudio/midaz-sdk-golang/v3"
    "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
)

func newClient() (*midaz.Client, error) {
    cfg, err := config.NewConfig(config.FromEnvironment())
    if err != nil {
        return nil, err
    }
    return midaz.New(midaz.WithConfig(cfg))
}
```

### 5.2 Anonymous client (local/dev, no auth)

```go
client, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentLocal),
    midaz.WithAnonymous(),
)
if err != nil { return err }
```

### 5.3 Aggressive retries with custom error classification

```go
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"

client, err := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentProduction),
    midaz.WithAccessManager(midaz.AccessManager{
        Address:      os.Getenv("PLUGIN_AUTH_ADDRESS"),
        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
    }),
    midaz.WithRetryOptions(
        retry.WithMaxRetries(5),
        retry.WithInitialDelay(200*time.Millisecond),
        retry.WithMaxDelay(10*time.Second),
        retry.WithJitterFactor(0.4),
        retry.WithRetryableHTTPCodes([]int{408, 425, 429, 500, 502, 503, 504}),
    ),
)
if err != nil { return err }
```

### 5.4 No retries (test posture)

```go
client, _ := midaz.New(
    midaz.WithEnvironment(midaz.EnvironmentLocal),
    midaz.WithAnonymous(),
    midaz.WithoutRetries(),
)
```

### 5.5 Shared observability provider across multiple clients

```go
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"

// Build once, share across multiple clients:
provider, _ := observability.New(ctx,
    observability.WithServiceName("payments-service"),
    observability.WithCollectorEndpoint("otel-collector:4317"),
    observability.WithComponentEnabled(true, true, true),
)

for _, am := range accessManagers {
    client, _ := midaz.New(
        midaz.WithEnvironment(midaz.EnvironmentProduction),
        midaz.WithAccessManager(am),
        midaz.WithObservabilityProvider(provider),  // share, don't rebuild
    )
    handle(client)
}
```

### 5.6 Custom logger

```go
import (
    "log/slog"
    "os"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client, _ := midaz.New(
    midaz.WithAnonymous(),
    midaz.WithLogger(logger),
)
```

### 5.7 Idempotency off (upstream gateway handles it)

```go
client, _ := midaz.New(
    midaz.WithAnonymous(),
    midaz.WithIdempotency(false),
)
// Now the SDK never generates X-Idempotency. The caller can still set
// one explicitly per request via sdkctx.WithIdempotencyKey.
```

---

## 6. Where to look next

- **`docs/auth.md`** — Plugin-based OAuth M2M authentication.
- **`docs/multi-tenancy.md`** — Tenant scope via Access Manager/JWT claims.
- **`docs/errors.md`** — `*errors.Error`, error categories,
  `IsConfigurationError`, retry classification.
- **`docs/examples.md`** — Observability and tracing examples.
- **godoc** — Every Option carries detailed godoc. Run `make godoc` to
  serve at `http://localhost:6060`.

---

## 7. Summary cheat sheet

```
WHAT YOU WANT                            REACH FOR
─────────────────────────────────        ─────────────────────────────────
Configure at construction                midaz.With*
Build *Config separately                 pkg/config.With*  →  midaz.WithConfig
Tune retry mechanics                     midaz.WithRetryOptions(retry.Option...)
Disable retries                          midaz.WithoutRetries()
Tune retry waits granularly              pkg/config.WithRetryWaitMin/Max
Override per request                     sdkctx.With*
Tune mid-lifetime                        client.GetEntityHTTPClient().Set*
Configure from env                       config.NewConfig(config.FromEnvironment())
Build OTel provider from chain           midaz.WithObservabilityOptions
Install pre-built OTel provider          midaz.WithObservabilityProvider
Custom slog logger                       midaz.WithLogger
```
