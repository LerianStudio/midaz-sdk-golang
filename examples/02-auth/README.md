# 02-auth

Demonstrates authentication via the Lerian Access Manager (the v3
production-shape auth path).

## What this demonstrates

- Loading auth configuration from environment variables via `config.FromEnvironment()`
- Building a client with `WithAccessManager`
- Constructing an organization via the typed input builder
- Calling `c.Organizations.CreateOrganization` under a traced context

## When to use this pattern

Production. Access Manager handles OAuth client-credentials flow,
token refresh, and credential rotation. v3 has exactly two sanctioned
auth sources: `WithAccessManager` (this example) or `WithAnonymous`
(local stacks only — see `examples/01-hello-world`). Calling `midaz.New()`
with neither returns a typed configuration error.

## How to run

```bash
# In a shell or in a .env file the example will load:
export PLUGIN_AUTH_ENABLED=true
export PLUGIN_AUTH_ADDRESS=https://auth.midaz.io
export MIDAZ_CLIENT_ID=your_client_id
export MIDAZ_CLIENT_SECRET=your_client_secret

go run ./examples/02-auth
```

The example also accepts a local `.env` file in the working directory
(it calls `godotenv.Load` if available).

## Expected output

```
Creating organization with legal name: "Lerian Demo"
Created organization org_01H...
```

## Related

- [`docs/auth.md`](../../docs/auth.md) — full auth-source matrix and migration table
- [`docs/multi-tenancy.md`](../../docs/multi-tenancy.md) — adding tenant routing on top
