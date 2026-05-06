# Multi-tenancy

The Midaz Go SDK lets a single `*midaz.Client` issue requests on behalf of any number of tenants. Tenant identity rides on the `X-Tenant-ID` HTTP header. v3 has a deliberate, single-direction precedence rule: per-request beats client-level beats environment.

> **Note on authority.** `X-Tenant-ID` is an optional compatibility signal for deployments that honor the header. The Midaz reference path derives tenant scope from authenticated claims (the access token issued by Access Manager already carries tenant context). The header exists to let SDK consumers flag intent at request time without modifying the access token; if your deployment ignores the header, your tenant routing still works through the bearer token.

## Setting tenant: three layers, one rule

```text
sdkctx.WithRequestTenantID(ctx, "...")   ← per-request override
       └── beats ──
midaz.WithTenantID("...")                  ← client-level default
       └── beats ──
MIDAZ_TENANT_ID env var                    ← environment-level default (only via FromEnvironment)
```

Per-request always wins. Client-level always wins over env. Env is the lowest-priority default.

## Client-level default

Apply once at construction:

```go
import (
    "github.com/LerianStudio/midaz-sdk-golang/v3"
)

c, err := midaz.New(
    midaz.WithAccessManager(midaz.AccessManager{ /* ... */ }),
    midaz.WithTenantID("acme-prod"),
)
if err != nil {
    return err
}

// All requests on c send X-Tenant-ID: acme-prod
acc, err := c.Accounts.GetAccount(ctx, "org-1", "ledger-1", "acc-1")
```

`midaz.WithTenantID` trims whitespace and accepts an empty value. Passing `WithTenantID("")` after a non-empty environment value clears the default — this is a *deliberate override*, not a bug. `c.tenantIDSet` flips to `true` so the construction logic knows you meant "no client-level tenant" rather than "I didn't set one".

## Per-request override

Use `pkg/sdkctx` to attach a tenant to a single request's context:

```go
import (
    "context"

    "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

// Override only this call. Subsequent calls on c still use the client default.
ctx := sdkctx.WithRequestTenantID(ctx, "acme-staging")
acc, err := c.Accounts.GetAccount(ctx, "org-1", "ledger-1", "acc-1")
```

Empty / whitespace-only inputs return the original context unchanged — they are no-ops, not "clear the tenant on this request". To send a request with no `X-Tenant-ID` header from a client that has a default configured, build a fresh context that doesn't inherit the override.

## Environment-level default

Set `MIDAZ_TENANT_ID` and load the env into your config via `config.FromEnvironment()`:

```go
import (
    "github.com/LerianStudio/midaz-sdk-golang/v3"
    "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
)

cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(midaz.WithConfig(cfg))
```

If `MIDAZ_TENANT_ID=acme-eu` is in the environment when this runs, the resulting client emits `X-Tenant-ID: acme-eu` on every request unless overridden.

> **Important:** `MIDAZ_TENANT_ID` only takes effect when `config.FromEnvironment()` is in the option chain. v3 does not silently consume environment variables — explicit opt-in is the rule (see [docs/environment.md](environment.md)). Setting `MIDAZ_TENANT_ID` in the shell while constructing a client without `FromEnvironment()` has zero effect.

## Header semantics

- The header is `X-Tenant-ID`.
- The header is omitted entirely when neither client default nor per-request value is set. The server sees no header rather than an empty value.
- Whitespace is trimmed. `WithTenantID("  acme  ")` and `WithTenantID("acme")` produce identical headers.
- The header carries the tenant value verbatim. The SDK does not inspect, validate, or rewrite the value.

## Patterns

### Multi-tenant batch processing

```go
c, _ := midaz.New(
    midaz.WithAccessManager(midaz.AccessManager{ /* ... */ }),
    midaz.WithTenantID("default-tenant"), // fallback for any call missing per-request override
)

for _, tenant := range tenants {
    ctx := sdkctx.WithRequestTenantID(context.Background(), tenant.ID)
    accs, err := c.Accounts.ListAccounts(ctx, tenant.OrgID, tenant.LedgerID, nil)
    // ...
}
```

### Per-request idempotency + tenant in one context

`sdkctx` helpers compose:

```go
ctx := sdkctx.WithIdempotencyKey(
    sdkctx.WithRequestTenantID(context.Background(), "acme-prod"),
    "transfer-2026-01-15-7af3",
)
_, err := c.Transactions.CreateTransaction(ctx, /* ... */)
```

### Verifying tenant routing in tests

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if got := r.Header.Get("X-Tenant-ID"); got != "expected-tenant" {
        t.Errorf("X-Tenant-ID = %q, want expected-tenant", got)
    }
    w.WriteHeader(http.StatusOK)
}))
defer srv.Close()

c, _ := midaz.New(
    midaz.WithBaseURL(srv.URL),
    midaz.WithAnonymous(),
    midaz.WithTenantID("expected-tenant"),
)
```

## Migration from v2

| v2 pattern | v3 replacement |
| --- | --- |
| `client.WithTenantID("...")` (root) | Same name, same surface — `midaz.WithTenantID(...)`. |
| `pkg/config.WithTenantID("...")` | Deleted. Use `midaz.WithTenantID` or assign `cfg.TenantID` directly on a config you own. |
| `entities.WithDefaultTenantID(...)` | Deleted. The Entity layer reads `Config.GetTenantID()` automatically; no Option needed. |
| `entities.WithTenantID(ctx, ...)` (deprecated context shim) | Deleted. Use `sdkctx.WithRequestTenantID(ctx, ...)` directly. |
| `(*HTTPClient).SetTenantID(...)` | Deleted from the public surface. The setter exists internally as `setTenantIDLocked`; external callers configure tenancy via the three layers above. |
| Reading `MIDAZ_TENANT_ID` implicitly | Now requires `config.FromEnvironment()` in the option chain. Implicit shell-set behavior was removed in Track 3. |

See [docs/v3-dx-plan.md](v3-dx-plan.md) (Track 2) for the full design rationale.
