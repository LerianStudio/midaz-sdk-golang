# Error handling in the Midaz Go SDK

The SDK uses structured errors from `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors`. Most SDK failures are represented as `*errors.Error`, which preserves category, code, operation, resource, HTTP status, request ID, and the wrapped underlying error.

## Core error type

```go
type Error struct {
    Category   errors.ErrorCategory
    Code       errors.ErrorCode
    Message    string
    Operation  string
    Resource   string
    ResourceID string
    StatusCode int
    RequestID  string
    Err        error
}
```

`*errors.Error` implements `error`, `Unwrap`, and `Is`, so it works with the standard library `errors.Is` and `errors.As` helpers.

## Error categories

| Category | Common meaning |
| --- | --- |
| `validation` | Invalid input or malformed request data |
| `authentication` | Missing, invalid, or expired credentials |
| `authorization` | Authenticated caller lacks permission |
| `not_found` | Requested resource does not exist |
| `conflict` | Duplicate resource, idempotency conflict, or state conflict |
| `limit_exceeded` | Rate limit or quota exceeded |
| `timeout` | Operation timed out |
| `cancellation` | Context or caller cancelled the operation |
| `internal` | SDK or backend internal failure |
| `unprocessable` | Domain-specific failure such as insufficient balance |

## Sentinel errors

The package exposes sentinel values for stable matching:

- `errors.ErrValidation`
- `errors.ErrAuthentication`
- `errors.ErrPermission`
- `errors.ErrNotFound`
- `errors.ErrAlreadyExists`
- `errors.ErrIdempotency`
- `errors.ErrRateLimit`
- `errors.ErrTimeout`
- `errors.ErrCancellation`
- `errors.ErrInternal`
- `errors.ErrInsufficientBalance`
- `errors.ErrAccountEligibility`
- `errors.ErrAssetMismatch`

## Checking errors

Prefer SDK helper functions when branching on operational behavior:

```go
import sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"

account, err := c.Entity.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
if err != nil {
    switch {
    case sdkerrors.IsNotFoundError(err):
        return nil, fmt.Errorf("account %s was not found: %w", accountID, err)
    case sdkerrors.IsAuthenticationError(err):
        return nil, fmt.Errorf("authentication failed: %w", err)
    case sdkerrors.IsRateLimitError(err):
        return nil, fmt.Errorf("rate limited while fetching account: %w", err)
    default:
        return nil, fmt.Errorf("failed to fetch account: %w", err)
    }
}
```

Common checkers include:

- `IsValidationError(err)`
- `IsNotFoundError(err)`
- `IsAuthenticationError(err)`
- `IsAuthorizationError(err)`
- `IsPermissionError(err)`
- `IsConflictError(err)`
- `IsAlreadyExistsError(err)`
- `IsIdempotencyError(err)`
- `IsRateLimitError(err)`
- `IsTimeoutError(err)`
- `IsNetworkError(err)`
- `IsCancellationError(err)`
- `IsInternalError(err)`
- `IsInsufficientBalanceError(err)`
- `IsAccountEligibilityError(err)`
- `IsAssetMismatchError(err)`

## Reading error details

Use accessors instead of type-specific concrete error names:

```go
if err != nil {
    category := sdkerrors.GetErrorCategory(err)
    statusCode := sdkerrors.GetStatusCode(err)
    details := sdkerrors.GetErrorDetails(err)

    log.Printf(
        "category=%s status=%d code=%s message=%s original=%v",
        category,
        statusCode,
        details.Code,
        details.Message,
        details.OriginalError,
    )
}
```

If you need fields only available on `*errors.Error`, use `errors.As` from the standard library:

```go
import (
    stderrors "errors"
    "log"

    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

var sdkErr *sdkerrors.Error
if stderrors.As(err, &sdkErr) {
    log.Printf(
        "operation=%s resource=%s resource_id=%s request_id=%s",
        sdkErr.GetOperation(),
        sdkErr.GetResource(),
        sdkErr.GetResourceID(),
        sdkErr.GetRequestID(),
    )
}
```

## Validation field errors

Model validation can return regular errors or `pkg/validation.FieldErrors` depending on the validator used. Field-level errors live in the validation package, not `pkg/errors`:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"

if fieldErrors, ok := err.(*validation.FieldErrors); ok {
    for _, fieldErr := range fieldErrors.GetFieldErrors() {
        log.Printf("field=%s message=%s", fieldErr.Field, fieldErr.Message)
    }
}
```

## Retry behavior

Retry policies live in `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry`, not `pkg/errors`.

The SDK HTTP layer retries transient failures by default. Retryable HTTP status codes are:

- `408 Request Timeout`
- `429 Too Many Requests`
- `500 Internal Server Error`
- `502 Bad Gateway`
- `503 Service Unavailable`
- `504 Gateway Timeout`

Root-client retry defaults are:

- Max retries: `3`
- Initial delay: `1s`
- Max delay: `30s`
- Backoff factor: `2.0`
- Jitter factor: `0.25`

Unsafe HTTP methods are retried only when an idempotency key is present. Attach one with:

```go
ctx = entities.WithIdempotencyKey(ctx, "request-unique-key")
```

Configure client retries with:

```go
c, err := client.New(
    client.WithRetries(3, 100*time.Millisecond, 10*time.Second),
    client.UseAllAPIs(),
)
```

Or disable them:

```go
c, err := client.New(
    client.DisableRetries(),
    client.UseAllAPIs(),
)
```

`config.FromEnvironment()` currently reads `MIDAZ_MAX_RETRIES`. It does not read `MIDAZ_RETRY_WAIT_MIN` or `MIDAZ_RETRY_WAIT_MAX`.

## Best practices

- Always wrap SDK errors with operation context when returning them from your application.
- Use SDK checker functions for business branching.
- Use `GetStatusCode` for HTTP status-specific behavior.
- Use `errors.As` when you need request ID, resource, or operation fields.
- Use idempotency keys for unsafe calls that may be retried.
- Treat validation field errors as validation package errors, not transport errors.
