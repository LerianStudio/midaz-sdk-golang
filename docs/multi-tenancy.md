# Multi-tenancy

Midaz tenant scope is derived from Access Manager/JWT claims. The Go SDK does **not** expose tenant configuration and does **not** send `X-Tenant-ID`.

## Operational rule

- Use `midaz.WithAccessManager(...)` with credentials that mint a token for the intended tenant scope.
- If a workload needs to operate under a different tenant, use a different Access Manager credential set or token context for that tenant.
- Do not attempt tenant switching with request headers; the SDK intentionally has no `WithTenantID`, no request-context tenant helper, and no `MIDAZ_TENANT_ID` configuration.

```go
c, err := midaz.New(
    midaz.WithAccessManager(midaz.AccessManager{
        Address:      accessManagerURL,
        ClientID:     tenantScopedClientID,
        ClientSecret: tenantScopedClientSecret,
    }),
)
if err != nil {
    return err
}

// Tenant is implied by the access token returned by Access Manager.
acc, err := c.V2.Accounts.Get(ctx, orgID, ledgerID, accountID)
```

## One client for many tenants

Set `AccessManager.CredentialSource` to resolve the credential per request from the request context, so a single `*midaz.Client` serves every tenant. The source returns the `clientID` and `clientSecret` for the tenant carried by `ctx`; `ClientID` and `ClientSecret` may then stay empty, and `midaz.New` performs no token exchange. Tokens are cached and refreshed per credential, so a 401 for one tenant never replays another tenant's token.

```go
midaz.WithAccessManager(midaz.AccessManager{
    Address: accessManagerURL,
    CredentialSource: midaz.CredentialSourceFunc(func(ctx context.Context) (string, string, error) {
        return credentialsFor(ctx) // e.g. looked up by the tenant in ctx
    }),
})
```

An error from the source fails the request immediately and is never retried.

## Migration from tenant-header based code

Remove all usages of:

- `midaz.WithTenantID(...)`
- `sdkctx.WithRequestTenantID(...)`
- `sdkctx.TenantIDFromContext(...)`
- `MIDAZ_TENANT_ID`
- assertions that expect `X-Tenant-ID` on SDK requests

Replace them with tenant-scoped Access Manager credentials/token acquisition. Organization and ledger identifiers remain explicit method arguments where Midaz APIs require them.

Historical planning notes may still mention the removed header path; operational code and docs should not use it.
