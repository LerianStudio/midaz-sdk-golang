# Authentication Guide

## Overview

The Go SDK uses Access Manager for OAuth2 client credentials authentication.

## Access Manager Configuration

```go
client, err := sdk.NewClient(
    sdk.WithAccessManager(sdk.AccessManagerConfig{
        Enabled:      true,
        Address:      "https://auth.example.com",
        ClientID:     os.Getenv("CLIENT_ID"),
        ClientSecret: os.Getenv("CLIENT_SECRET"),
    }),
)
```

## Configuration Options

| Option | Type | Description |
|--------|------|-------------|
| `Enabled` | bool | Enable/disable authentication |
| `Address` | string | Auth server URL |
| `ClientID` | string | OAuth2 client ID |
| `ClientSecret` | string | OAuth2 client secret |

## Token Management

Access Manager handles:
- Initial token retrieval
- Automatic token refresh before expiry
- Token caching
- Thread-safe token access

```go
// Token is automatically injected into requests
// No manual token handling required
```

## Environment Variables

```bash
# Recommended approach
export MIDAZ_CLIENT_ID="your-client-id"
export MIDAZ_CLIENT_SECRET="your-client-secret"
export MIDAZ_AUTH_URL="https://auth.example.com"
```

## Disabled Authentication

For local development without auth:

```go
client, err := sdk.NewClient(
    sdk.WithEnvironment(sdk.EnvironmentLocal),
    sdk.WithAccessManager(sdk.AccessManagerConfig{
        Enabled: false,
    }),
)
```

## Custom Auth Header

For pre-authenticated scenarios:

```go
ctx := context.WithValue(ctx, "Authorization", "Bearer "+token)
result, err := client.Entity.Accounts().List(ctx, orgID, ledgerID)
```

## Key File References

| Purpose | File |
|---------|------|
| Access Manager | `pkg/access-manager/access-manager.go` |
| Token refresh | `pkg/access-manager/token.go` |
