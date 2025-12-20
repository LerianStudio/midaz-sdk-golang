# Error Handling Guide

## Overview

The Go SDK provides structured error handling with `SDKError` type and semantic error mapping from HTTP status codes.

## SDKError Type

```go
import "github.com/lerianstudio/midaz-sdk-go/v2/pkg/errors"

result, err := client.Entity.Accounts().Create(ctx, input)
if err != nil {
    var sdkErr *errors.SDKError
    if errors.As(err, &sdkErr) {
        fmt.Printf("API Error: %s\n", sdkErr.Message)
        fmt.Printf("Request ID: %s\n", sdkErr.RequestID)
        fmt.Printf("Status Code: %d\n", sdkErr.StatusCode)
    }
}
```

See: `pkg/errors/errors.go`

## Error Properties

```go
type SDKError struct {
    Message    string            // Human-readable message
    Code       string            // Machine-readable error code
    StatusCode int               // HTTP status code
    RequestID  string            // Request ID for debugging
    Details    map[string]any    // Additional context
}
```

## HTTP Status Mapping

| Status | Error Type | Description |
|--------|------------|-------------|
| 400 | `ErrValidation` | Invalid input data |
| 401 | `ErrAuthentication` | Invalid/missing credentials |
| 403 | `ErrAuthorization` | Insufficient permissions |
| 404 | `ErrNotFound` | Resource doesn't exist |
| 409 | `ErrConflict` | Resource conflict |
| 429 | `ErrRateLimited` | Rate limit exceeded |
| 500 | `ErrInternal` | Server error |

## Error Checking Patterns

### Using errors.Is

```go
import "github.com/lerianstudio/midaz-sdk-go/v2/pkg/errors"

result, err := client.Entity.Accounts().Get(ctx, orgID, ledgerID, accountID)
if err != nil {
    if errors.Is(err, errors.ErrNotFound) {
        // Handle not found
        return nil, nil
    }
    if errors.Is(err, errors.ErrValidation) {
        // Handle validation error
        return nil, fmt.Errorf("invalid input: %w", err)
    }
    return nil, err
}
```

### Using errors.As

```go
result, err := client.Entity.Transactions().Create(ctx, input)
if err != nil {
    var sdkErr *errors.SDKError
    if errors.As(err, &sdkErr) {
        switch sdkErr.StatusCode {
        case 400:
            log.Error("validation error", "details", sdkErr.Details)
        case 404:
            log.Warn("resource not found")
        case 429:
            retryAfter := sdkErr.Details["retry_after"]
            time.Sleep(time.Duration(retryAfter.(int)) * time.Second)
        default:
            log.Error("API error", "request_id", sdkErr.RequestID)
        }
    }
}
```

## Wrapping Errors

```go
result, err := client.Entity.Accounts().Create(ctx, input)
if err != nil {
    return nil, fmt.Errorf("failed to create account: %w", err)
}
```

## Request ID for Debugging

Every API response includes a request ID for troubleshooting:

```go
result, err := client.Entity.Accounts().Create(ctx, input)
if err != nil {
    var sdkErr *errors.SDKError
    if errors.As(err, &sdkErr) {
        // Log request ID for support tickets
        log.Error("API call failed",
            "request_id", sdkErr.RequestID,
            "error", sdkErr.Message,
        )
    }
}
```

## Retryable Errors

Some errors are automatically retried by the SDK:

```go
// These are retried automatically:
// - 408 Request Timeout
// - 429 Too Many Requests (with backoff)
// - 500, 502, 503, 504 Server Errors

// These are NOT retried:
// - 400 Validation errors
// - 401 Authentication errors
// - 403 Authorization errors
// - 404 Not found
// - 409 Conflict
```

## Custom Error Handling

```go
type ErrorHandler func(error) error

client, err := sdk.NewClient(
    sdk.WithErrorHandler(func(err error) error {
        var sdkErr *errors.SDKError
        if errors.As(err, &sdkErr) {
            // Custom logging, metrics, alerting
            metrics.IncrementCounter("api_errors",
                "status", strconv.Itoa(sdkErr.StatusCode),
            )
        }
        return err
    }),
)
```

## Key File References

| Purpose | File |
|---------|------|
| Error types | `pkg/errors/errors.go` |
| Error mapping | `pkg/errors/http_errors.go` |
| Retry decisions | `pkg/retry/retry.go` |
