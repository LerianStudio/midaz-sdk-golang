# Error handling in the Midaz Go SDK

The SDK uses structured errors from `github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors`. Most SDK failures are represented as `*errors.Error`, which preserves SDK category/code, raw Midaz envelope fields, operation/resource context, HTTP status, request ID, and the wrapped underlying error.

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
    Source     errors.ErrorSource
    HTTPRequestSent      bool
    HTTPResponseReceived bool
    StatusCodeSource     errors.ErrorStatusCodeSource
    RequestID  string
    Method     string
    URLHost    string
    URLPath    string
    Err        error
}
```

`*errors.Error` implements `error`, `Unwrap`, and `Is`, so it works with the standard library `errors.Is` and `errors.As` helpers.

The diagnostic fields identify where the error came from and how much HTTP state is known. `Source` is one of `sdk`, `configuration`, `transport`, or `http_response`. `StatusCodeSource` is `none`, `synthetic`, or `upstream`. `HTTPRequestSent` and `HTTPResponseReceived` distinguish local validation failures, pre-response transport failures, and upstream HTTP responses without parsing the error string.

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

For received upstream HTTP errors (`4xx` and `5xx`), the SDK does not attach the raw Midaz response body by default. Enable this only for controlled diagnostics. When enabled, the raw body is available through `Error.UpstreamBody`, `Error.GetUpstreamBody()`, and `GetErrorDetails(err).UpstreamBody`.

Control this behavior with `midaz.WithErrorBodyExposure(true)`, `config.WithErrorBodyExposure(true)`, or `MIDAZ_ERROR_EXPOSE_BODY=true` when using `config.FromEnvironment()`.

Direct upstream body access is intentionally raw and unredacted. Treat it as diagnostic material that may contain server stack traces, echoed input, or credentials. `Error.Error()` renders a redacted projection of the attached body for the canonical logging path. `json.Marshal(*errors.Error)` remains a safe projection and does not include `UpstreamBody`.

The SDK truncates the attached body and reports truncation through `Error.UpstreamBodyTruncated` / `Error.IsUpstreamBodyTruncated()` and `Error.UpstreamBodyOriginalBytes` / `Error.GetUpstreamBodyOriginalBytes()`.

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
| `configuration` | SDK setup or client-construction failure |
| `network` | Pre-response transport failure such as DNS, TLS, or connection errors |
| `internal` | SDK or backend internal failure |
| `unprocessable` | Domain-specific failure such as insufficient balance |

## Sentinel errors

The package exposes sentinel values for stable matching:

- `errors.ErrValidation`
- `errors.ErrAuthentication`
- `errors.ErrAuth` — matches both authentication and authorization failures
- `errors.ErrPermission`
- `errors.ErrConfiguration`
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

Common constructors include `NewValidationError`, `NewInvalidInputError`, `NewNotFoundError`, `NewAuthenticationError`, `NewAuthorizationError`, `NewConfigurationError`, `NewConflictError`, `NewRateLimitError`, `NewTimeoutError`, `NewInternalError`, and `NewUnprocessableError`.

## Checking errors

Prefer SDK helper functions when branching on operational behavior:

```go
import sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"

account, err := c.V2.Accounts.Get(ctx, orgID, ledgerID, accountID)
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
- `IsBootstrapError(err)` — any supported `midaz.New(...)` bootstrap failure category
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

## Domain-specific error predicates

Beyond the category checkers above, `pkg/errors` exposes predicates that branch on specific server business conditions. They mirror the server error codes in `github.com/LerianStudio/midaz/v3/pkg/constant` and let callers react to a named condition instead of hardcoding raw `APICode` strings at call sites.

| Predicate | Server code | HTTP status | Condition |
| --- | --- | --- | --- |
| `IsSkipNotPermitted(err)` | `0490` | 422 | A per-call skip was requested without the enabling ledger override. |
| `IsHolderRequired(err)` | `0491` | 422 | Account creation requires a holder (KYC). |
| `IsHolderNotFound(err)` | `CRM-0006` | 404 | The referenced CRM holder does not exist. |
| `IsFeeError(err)` | `0179`–`0233` | mixed | The fee/billing engine rejected the operation. Family predicate over the whole fee-code block. |

`IsHolderNotFound` is distinct from the generic `IsNotFoundError`: it pins the CRM holder resource specifically, so a caller can tell "the holder you referenced is missing" apart from any other 404.

`IsFeeError` is a family predicate — it returns `true` when the server code suffix falls in the fee/billing block `0179`–`0233`. Callers branch on "fee/billing problem" rather than each of the ~55 internal fee codes.

Retryability note: these predicates need no retryability override. `0490`/`0491` (422) and `CRM-0006` (404) are already classified non-retryable by the SDK's HTTP-status→category mapping. The predicates are ergonomic — they let callers branch on the specific business condition — not a money-path concern.

### Feature-availability sentinel

Envelope encryption in legacy mode (no KMS vendor configured) surfaces as a 404 because its routes are never registered. Three symbols model that condition:

- `ErrFeatureNotAvailable` — the sentinel value.
- `MarkFeatureNotAvailable(err) error` — joins the sentinel onto a not-found error, preserving the underlying `*errors.Error` (404). A nil or non-not-found error is returned unchanged; the encryption facade uses it to tag its legacy-mode 404s.
- `IsFeatureNotAvailable(err) bool` — reports whether the error carries the sentinel marker.

`IsFeatureNotAvailable` keys on the marker, not the 404 status: a generic not-found does not match. The underlying `*errors.Error{StatusCode: 404}` remains reachable via `errors.As` and `IsNotFoundError`.

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

`GetErrorDetails` also includes structured API details when the original error is an SDK `*errors.Error`: `APICode`, `Title`, `EntityType`, `Fields`, `Details`, `UpstreamBody`, `UpstreamBodyTruncated`, `UpstreamBodyOriginalBytes`, and `RequestID`. `Fields`, `Details`, and `Title` are redacted in this projection. `UpstreamBody` remains raw.

For HTTP errors produced from a Midaz response, inspect the concrete SDK error when you need the raw API code:

```go
import (
    stderrors "errors"
    "log"

    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
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

    sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
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
import "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"

if fieldErrors, ok := err.(*validation.FieldErrors); ok {
    for _, fieldErr := range fieldErrors.GetFieldErrors() {
        log.Printf("field=%s message=%s", fieldErr.Field, fieldErr.Message)
    }
}
```

## Retry behavior

Retry policies live in `github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry`, not `pkg/errors`.

The SDK HTTP layer retries transient failures by default. Retryable HTTP status codes are:

- `408 Request Timeout`
- `425 Too Early`
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
import "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"

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
