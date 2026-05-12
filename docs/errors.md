# Error handling in the Midaz Go SDK

The SDK uses structured errors from `github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors`. Most SDK failures are represented as `*errors.Error`, which preserves SDK category/code, raw Midaz envelope fields, operation/resource context, HTTP status, request ID, and the wrapped underlying error.

## Core error type

```go
type Error struct {
    Category   errors.ErrorCategory
    Code       errors.ErrorCode
    APICode    string
    Title      string
    Message    string
    Operation  string
    Resource   string
    ResourceID string
    EntityType string
    Fields     []string
    Details    map[string]any
    UpstreamBody              string
    UpstreamBodyTruncated     bool
    UpstreamBodyOriginalBytes int
    StatusCode int
    RequestID  string
    Err        error
}
```

`*errors.Error` implements `error`, `Unwrap`, and `Is`, so it works with the standard library `errors.Is` and `errors.As` helpers.

## Midaz wire error envelope

Midaz APIs return structured error bodies. Ledger responses commonly use these fields:

| Wire field | Meaning | SDK handling |
| --- | --- | --- |
| `code` | Raw Midaz API error code | Stored as `Error.APICode` when available and mapped to SDK `Error.Code` for known cases. |
| `title` | Short error title | Stored as `Error.Title`. |
| `message` | Human-readable error message | Used as the SDK error message. |
| `entityType` | Resource type related to the error | Stored as `Error.EntityType` and mapped to SDK resource context when available. |
| `fields` | Field-level validation details from the API | Stored as `Error.Fields`. |

CRM responses may also return an `err` field. Treat `err` as the CRM-specific error string when `message` is absent.

The SDK preserves expanded envelope fields on `*errors.Error` through `APICode`, `Title`, `EntityType`, `Fields`, and `Details`. Prefer these structured fields over parsing the rendered error string.

## Upstream response body exposure

For received upstream HTTP errors (`4xx` and `5xx`), the SDK attaches the raw Midaz response body to the structured error by default. The body is available through `Error.UpstreamBody`, `Error.GetUpstreamBody()`, and formatted error strings such as `err.Error()`.

Control this behavior with `midaz.WithErrorBodyExposure(false)`, `config.WithErrorBodyExposure(false)`, or `MIDAZ_ERROR_EXPOSE_BODY=false` when using `config.FromEnvironment()`.

The SDK does not redact this upstream body by design. It only truncates it and reports truncation through `Error.UpstreamBodyTruncated` / `Error.IsUpstreamBodyTruncated()` and `Error.UpstreamBodyOriginalBytes` / `Error.GetUpstreamBodyOriginalBytes()`. `json.Marshal(*errors.Error)` remains a safe projection and does not include `UpstreamBody`.

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
- `errors.ErrUnprocessable`
- `errors.ErrInsufficientBalance`
- `errors.ErrAccountEligibility`
- `errors.ErrAssetMismatch`

Common constructors include `NewValidationError`, `NewInvalidInputError`, `NewNotFoundError`, `NewAuthenticationError`, `NewAuthorizationError`, `NewConflictError`, `NewRateLimitError`, `NewTimeoutError`, `NewInternalError`, and `NewUnprocessableError`.

## Checking errors

Prefer SDK helper functions when branching on operational behavior:

```go
import sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"

account, err := c.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
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
- `IsAuthError(err)` — matches both authentication (401) and authorization (403)
- `IsConfigurationError(err)` — SDK setup or client-construction errors
- `IsConflictError(err)` — covers 409 conflicts including "already exists" responses
- `IsIdempotencyError(err)`
- `IsRateLimitError(err)`
- `IsTimeoutError(err)`
- `IsNetworkError(err)`
- `IsCancellationError(err)`
- `IsInternalError(err)`
- `IsInsufficientBalanceError(err)`
- `IsAccountEligibilityError(err)`
- `IsAssetMismatchError(err)`
- `IsUnprocessableError(err)`

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

`GetErrorDetails` also includes structured API details when the original error is an SDK `*errors.Error`: `APICode`, `Title`, `EntityType`, `Fields`, `Details`, and `RequestID`.

For HTTP errors produced from a Midaz response, inspect the concrete SDK error when you need the raw API code:

```go
import (
    stderrors "errors"
    "log"

    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

var sdkErr *sdkerrors.Error
if stderrors.As(err, &sdkErr) {
    log.Printf(
        "api_code=%s sdk_code=%s status=%d resource=%s request_id=%s",
        sdkErr.APICode,
        sdkErr.Code,
        sdkErr.StatusCode,
        sdkErr.Resource,
        sdkErr.RequestID,
    )
}
```

If you need fields only available on `*errors.Error`, use `errors.As` from the standard library:

```go
import (
    stderrors "errors"
    "log"

    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"

if fieldErrors, ok := err.(*validation.FieldErrors); ok {
    for _, fieldErr := range fieldErrors.GetFieldErrors() {
        log.Printf("field=%s message=%s", fieldErr.Field, fieldErr.Message)
    }
}
```

## Retry behavior

Retry policies live in `github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry`, not `pkg/errors`.

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
ctx = sdkctx.WithIdempotencyKey(ctx, "request-unique-key")
```

Configure client retries with `pkg/retry` options:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"

c, err := midaz.New(
    midaz.WithAnonymous(),
    midaz.WithRetryOptions(
        retry.WithMaxRetries(3),
        retry.WithInitialDelay(100*time.Millisecond),
        retry.WithMaxDelay(10*time.Second),
    ),
)
```

Or disable them:

```go
c, err := midaz.New(
    midaz.WithoutRetries(),
    midaz.WithAnonymous(),
)
```

Use `Error.Retryable()` as the canonical retry-policy source on `*errors.Error`:

```go
var sdkErr *sdkerrors.Error
if errors.As(err, &sdkErr) && sdkErr.Retryable() {
    // The SDK already classifies the error as retryable; apply your
    // application-level retry strategy here.
}
```

`config.FromEnvironment()` currently reads `MIDAZ_MAX_RETRIES`. It does not read `MIDAZ_RETRY_WAIT_MIN` or `MIDAZ_RETRY_WAIT_MAX`.

## Best practices

- Always wrap SDK errors with operation context when returning them from your application.
- Use SDK checker functions for business branching.
- Use `GetStatusCode` for HTTP status-specific behavior.
- Use `errors.As` when you need request ID, resource, operation, or raw API envelope fields.
- Use idempotency keys for unsafe calls that may be retried.
- Treat validation field errors as validation package errors, not transport errors.
