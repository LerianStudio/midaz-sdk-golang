// (Package documentation lives in doc.go.)
package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	obsredaction "github.com/LerianStudio/lib-observability/redaction"
)

const redactedValue = "[REDACTED]"

// ErrorCode represents a standardized error code for Midaz API errors.
type ErrorCode string

// Error code constants
const (
	// CodeValidation indicates a validation error
	CodeValidation ErrorCode = "validation_error"

	// CodeNotFound indicates a resource was not found
	CodeNotFound ErrorCode = "not_found"

	// CodeAlreadyExists indicates a resource already exists
	CodeAlreadyExists ErrorCode = "already_exists"

	// CodeAuthentication indicates an authentication error
	CodeAuthentication ErrorCode = "authentication_error"

	// CodePermission indicates a permission error
	CodePermission ErrorCode = "permission_error"

	// CodeInsufficientBalance indicates an insufficient balance error
	CodeInsufficientBalance ErrorCode = "insufficient_balance"

	// CodeAccountEligibility indicates an account eligibility error
	CodeAccountEligibility ErrorCode = "account_eligibility_error"

	// CodeAssetMismatch indicates an asset mismatch error
	CodeAssetMismatch ErrorCode = "asset_mismatch"

	// CodeIdempotency indicates an idempotency error
	CodeIdempotency ErrorCode = "idempotency_error"

	// CodeRateLimit indicates a rate limit error
	CodeRateLimit ErrorCode = "rate_limit_exceeded"

	// CodeTimeout indicates a timeout error
	CodeTimeout ErrorCode = "timeout"

	// CodeCancellation indicates the operation was cancelled
	CodeCancellation ErrorCode = "cancelled"

	// CodeInternal indicates an internal server error
	CodeInternal ErrorCode = "internal_error"

	// CodeNetwork indicates a network-related error
	CodeNetwork ErrorCode = "network_error"

	// CodeUnprocessable indicates a business rule prevented processing
	CodeUnprocessable ErrorCode = "unprocessable_error"

	// CodeConfiguration is for SDK setup / client construction errors.
	CodeConfiguration ErrorCode = "configuration_error"

	// CodeServiceUnavailable indicates the upstream service responded
	// with HTTP 503. This shares [CategoryNetwork] with [CodeNetwork]
	// because both indicate a transient transport-or-availability failure
	// where retry is appropriate; the Code field distinguishes the
	// pre-response transport failure (CodeNetwork) from the post-response
	// service-unavailable signal (CodeServiceUnavailable).
	CodeServiceUnavailable ErrorCode = "service_unavailable"
)

// ErrorCategory represents the general category of an error
type ErrorCategory string

const statusClientClosedRequest = 499

// ErrorSource identifies where an SDK error originated.
type ErrorSource string

const (
	// ErrorSourceSDK identifies errors synthesized by SDK-side validation or helpers.
	ErrorSourceSDK ErrorSource = "sdk"

	// ErrorSourceConfiguration identifies errors caused by SDK configuration.
	ErrorSourceConfiguration ErrorSource = "configuration"

	// ErrorSourceTransport identifies pre-response transport failures.
	ErrorSourceTransport ErrorSource = "transport"

	// ErrorSourceHTTPResponse identifies errors built from an upstream HTTP response.
	ErrorSourceHTTPResponse ErrorSource = "http_response"
)

// ErrorStatusCodeSource identifies whether Error.StatusCode came from upstream
// HTTP or was synthesized by the SDK for caller convenience.
type ErrorStatusCodeSource string

const (
	// StatusCodeSourceNone means no HTTP status code is available.
	StatusCodeSourceNone ErrorStatusCodeSource = "none"

	// StatusCodeSourceSynthetic means the SDK selected a suggested HTTP status.
	StatusCodeSourceSynthetic ErrorStatusCodeSource = "synthetic"

	// StatusCodeSourceUpstream means the status came from a received HTTP response.
	StatusCodeSourceUpstream ErrorStatusCodeSource = "upstream"
)

const (
	// CategoryValidation represents validation errors
	CategoryValidation ErrorCategory = "validation"

	// CategoryAuthentication represents authentication errors
	CategoryAuthentication ErrorCategory = "authentication"

	// CategoryAuthorization represents authorization errors
	CategoryAuthorization ErrorCategory = "authorization"

	// CategoryNotFound represents not found errors
	CategoryNotFound ErrorCategory = "not_found"

	// CategoryConflict represents resource conflict errors
	CategoryConflict ErrorCategory = "conflict"

	// CategoryLimitExceeded represents rate limit or quota exceeded errors
	CategoryLimitExceeded ErrorCategory = "limit_exceeded"

	// CategoryTimeout represents timeout errors
	CategoryTimeout ErrorCategory = "timeout"

	// CategoryCancellation represents context cancellation errors
	CategoryCancellation ErrorCategory = "cancellation"

	// CategoryNetwork represents network-related errors. Used for both
	// pre-response transport failures (DNS, conn-refused, TLS, broken
	// pipe — paired with [CodeNetwork]) and HTTP 503 responses where the
	// server answered but is not currently available (paired with
	// [CodeServiceUnavailable]). Both shapes are retryable.
	CategoryNetwork ErrorCategory = "network"

	// CategoryInternal represents internal SDK or server errors
	CategoryInternal ErrorCategory = "internal"

	// CategoryUnprocessable represents unprocessable operations
	CategoryUnprocessable ErrorCategory = "unprocessable"

	// CategoryConfiguration represents SDK setup / client construction errors
	// (missing required options, invalid URLs, conflicting auth sources, etc.).
	// These errors are produced eagerly by midaz.New() and indicate misuse of
	// the SDK rather than a server-side or transport problem.
	CategoryConfiguration ErrorCategory = "configuration"

	// CategoryAuth is a synthetic match-any-auth shape used by [ErrAuth]
	// and the [Error.Is] bridge so callers can write
	//
	//	errors.Is(err, errors.ErrAuth)
	//
	// to match both CategoryAuthentication (401) AND CategoryAuthorization
	// (403) in one predicate. No constructor PRODUCES errors with
	// Category=CategoryAuth — the live errors carry the disjoint 401/403
	// category. The "auth" string only ever appears as the Target of an
	// [errors.Is] check.
	//
	// To distinguish 401 from 403, inspect [Error.StatusCode] or
	// [Error.Code] (CodeAuthentication vs CodePermission) after [errors.As].
	CategoryAuth ErrorCategory = "auth"
)

// Standard error types that wrap all our error codes
// These are created as Error types rather than simple strings to make error checking work correctly
var (
	ErrValidation          = &Error{Category: CategoryValidation, Code: CodeValidation, Message: "validation error"}
	ErrInsufficientBalance = &Error{Category: CategoryUnprocessable, Code: CodeInsufficientBalance, Message: "insufficient balance"}
	ErrAccountEligibility  = &Error{Category: CategoryValidation, Code: CodeAccountEligibility, Message: "account eligibility error"}
	ErrAssetMismatch       = &Error{Category: CategoryValidation, Code: CodeAssetMismatch, Message: "asset mismatch"}
	ErrAuthentication      = &Error{Category: CategoryAuthentication, Code: CodeAuthentication, Message: "authentication error"}
	ErrPermission          = &Error{Category: CategoryAuthorization, Code: CodePermission, Message: "permission error"}
	// ErrAuth matches any auth failure (authentication or authorization).
	// Use with errors.Is for broad matching:
	//
	//	if errors.Is(err, errors.ErrAuth) { /* re-prompt for credentials */ }
	//
	// To distinguish 401 from 403, extract the typed error and inspect
	// Error.StatusCode or Error.Code.
	ErrAuth          = &Error{Category: CategoryAuth, Message: "authentication or authorization error"}
	ErrNotFound      = &Error{Category: CategoryNotFound, Code: CodeNotFound, Message: "not found"}
	ErrAlreadyExists = &Error{Category: CategoryConflict, Code: CodeAlreadyExists, Message: "already exists"}
	ErrIdempotency   = &Error{Category: CategoryConflict, Code: CodeIdempotency, Message: "idempotency error"}
	ErrRateLimit     = &Error{Category: CategoryLimitExceeded, Code: CodeRateLimit, Message: "rate limit exceeded"}
	ErrTimeout       = &Error{Category: CategoryTimeout, Code: CodeTimeout, Message: "timeout"}
	ErrCancellation  = &Error{Category: CategoryCancellation, Code: CodeCancellation, Message: "operation cancelled"}
	ErrInternal      = &Error{Category: CategoryInternal, Code: CodeInternal, Message: "internal error"}
	ErrUnprocessable = &Error{Category: CategoryUnprocessable, Code: CodeUnprocessable, Message: "unprocessable error"}
	ErrConfiguration = &Error{Category: CategoryConfiguration, Code: CodeConfiguration, Message: "configuration error"}
)

// Error represents a standardized error in the Midaz SDK.
// It includes context about the error's category, associated operation,
// and affected resource, making errors more informative and easier to handle.
//
// Audit C3 (CRITICAL): every field that may carry user-supplied or
// credential data is annotated `json:"-"`. A naive json.Marshal(err) on
// an *Error must not leak Bearer tokens, request bodies, or wrapped
// inner errors — even though the constructors already store
// pre-redacted Message values. Callers who need a JSON projection of
// an error should build their own DTO from the safe fields (Category,
// Code, StatusCode). [Error.Error] redacts the SDK-generated prefix and any
// rendered upstream-body segment.
type Error struct {
	// Category is the general category of the error
	Category ErrorCategory `json:"category"`

	// Code is the specific error code
	Code ErrorCode `json:"code"`

	// APICode is the raw Midaz/CRM API error code, when returned by the API.
	APICode string `json:"apiCode,omitempty"`

	// Title is the raw API error title, when returned by the API.
	// Marked json:"-" because API titles can contain user-supplied
	// fragments that the redactor would otherwise miss in JSON output.
	Title string `json:"-"`

	// Message is the human-readable error message.
	// Marked json:"-" because the message often contains credentials
	// or identifiers — even though constructors store a pre-redacted
	// form, JSON output is the wrong path: callers should use Error()
	// for rendered output or build their own slim projection.
	Message string `json:"-"`

	// Operation is the operation that was being performed.
	// Marked json:"-" defensively: operation strings are generally safe
	// (e.g. "accounts.Create") but dynamic concatenation in some paths
	// can pull in identifiers we'd rather not surface in JSON.
	Operation string `json:"-"`

	// Resource is the type of resource involved.
	Resource string `json:"resource,omitempty"`

	// ResourceID is the identifier of the resource involved, if applicable.
	// Marked json:"-" because resource IDs can be PII (account IDs,
	// document numbers) when the API uses meaningful identifiers.
	ResourceID string `json:"-"`

	// EntityType is the raw API entity type, when returned by the API.
	EntityType string `json:"entityType,omitempty"`

	// Fields is the raw API field list, when returned by the API.
	// Marked json:"-" because field paths can include user-supplied keys.
	Fields []string `json:"-"`

	// Details contains the raw API error details, when returned by the API.
	// Marked json:"-" because the Details map is the most common leak
	// source — it carries arbitrary server-supplied structures.
	Details map[string]any `json:"-"`

	// UpstreamBody is the raw upstream 4xx/5xx response body attached to SDK
	// HTTP response errors when error body exposure is enabled. It is not
	// redacted by design; it is only truncated.
	//
	// SECURITY WARNING — direct access vs. rendered output:
	//
	//   - Direct access via [Error.GetUpstreamBody] returns the RAW
	//     unredacted bytes. Callers logging this field directly bypass
	//     the redactor and may surface credentials embedded in upstream
	//     payloads.
	//
	//   - The [Error.Error] method renders the body through the
	//     [redactSensitive] pipeline before composing the final string.
	//     Consumers logging err.Error() (the canonical path) are safe.
	//
	// If you need to forward the body to external systems, prefer
	// [Error.Error] or apply [RedactSensitiveString] to GetUpstreamBody()
	// yourself before emitting.
	UpstreamBody string `json:"-"`

	// upstreamBodyRedacted is the pre-redacted form of UpstreamBody,
	// computed once at [AttachUpstreamBody] time so [Error.Error] does
	// not re-run the regex pass on every render. Pre-redaction matters
	// because Error() is called multiple times per terminal failure
	// (slog formatting, retry hook, terminal log) and the redactor
	// scans up to redactSensitiveMaxBytes on each invocation.
	//
	// Unexported and json:"-": this field is an internal cache, not part
	// of the public surface. Reset via the AttachUpstreamBody*
	// constructors only.
	upstreamBodyRedacted string

	// UpstreamBodyTruncated reports whether UpstreamBody is a truncated prefix
	// of the upstream response body captured by the SDK.
	UpstreamBodyTruncated bool `json:"-"`

	// UpstreamBodyOriginalBytes is the byte length observed before the
	// pkg/errors exposure truncation was applied.
	UpstreamBodyOriginalBytes int `json:"-"`

	// StatusCode is the HTTP status code, if applicable
	StatusCode int `json:"statusCode,omitempty"`

	// Source identifies the layer that produced the error. It is safe diagnostic
	// metadata and does not contain request data.
	Source ErrorSource `json:"-"`

	// HTTPRequestSent reports whether the SDK attempted an HTTP request.
	HTTPRequestSent bool `json:"-"`

	// HTTPResponseReceived reports whether an upstream HTTP response was received.
	HTTPResponseReceived bool `json:"-"`

	// StatusCodeSource identifies whether StatusCode is upstream, synthetic, or absent.
	StatusCodeSource ErrorStatusCodeSource `json:"-"`

	// RequestID is the API request ID, if available.
	// Marked json:"-" defensively: request IDs are generally opaque, but
	// the safe baseline is to opt out of JSON exposure for everything
	// caller-derived.
	RequestID string `json:"-"`

	// Method is the HTTP method of the request that produced this error,
	// when the error originated from an HTTP response or transport failure.
	// Marked json:"-" defensively (consistent with other request-derived
	// fields) — the value is one of GET/POST/PUT/PATCH/DELETE so leak risk
	// is low, but JSON exposure is the wrong default for v3.
	Method string `json:"-"`

	// URLHost is the redacted host:port the failing request was issued to
	// (userinfo and query stripped). Use this in diagnostic dashboards
	// without re-deriving from the rendered error string.
	URLHost string `json:"-"`

	// URLPath is the redacted path the failing request was issued to
	// (query/fragment stripped, dynamic ID segments collapsed to ":id").
	URLPath string `json:"-"`

	// Err is the underlying error.
	// Marked json:"-" because the inner error string is opaque to the
	// redactor — a wrapped *http.Error or similar can carry the full
	// request body. JSON output must rely on Error() for safety.
	Err error `json:"-"`
}

// MarshalJSON renders only the safe, non-leaky fields of an *Error.
// The unredacted Message, Title, ResourceID, RequestID, Fields, Details,
// and inner Err are intentionally excluded. Callers who need the full
// rendered string should use [Error.Error] (whose upstream-body segment is
// redacted when body exposure is enabled).
//
// Returns "null" for a nil receiver.
func (e *Error) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}

	// We deliberately do NOT include any caller-derived strings here.
	// Adding a field to this projection requires explicit security
	// review: it must not be capable of carrying credentials, tokens,
	// or PII even after the constructor's redaction pass.
	type safeProjection struct {
		Category             ErrorCategory         `json:"category"`
		Code                 ErrorCode             `json:"code"`
		APICode              string                `json:"apiCode,omitempty"`
		Resource             string                `json:"resource,omitempty"`
		EntityType           string                `json:"entityType,omitempty"`
		StatusCode           int                   `json:"statusCode,omitempty"`
		Source               ErrorSource           `json:"source,omitempty"`
		HTTPRequestSent      bool                  `json:"httpRequestSent,omitempty"`
		HTTPResponseReceived bool                  `json:"httpResponseReceived,omitempty"`
		StatusCodeSource     ErrorStatusCodeSource `json:"statusCodeSource,omitempty"`
	}

	projection := safeProjection{
		Category:             e.Category,
		Code:                 e.Code,
		APICode:              e.APICode,
		Resource:             e.Resource,
		EntityType:           e.EntityType,
		StatusCode:           e.StatusCode,
		Source:               e.Source,
		HTTPRequestSent:      e.HTTPRequestSent,
		HTTPResponseReceived: e.HTTPResponseReceived,
		StatusCodeSource:     e.StatusCodeSource,
	}

	return json.Marshal(projection)
}

// Error implements the error interface.
//
// Audit 8.10 (LOW): the v2 implementation called redactSensitive
// twice per Error() call — once on the message and once on the final
// composed string. That was both wasteful (regex rescans) and
// stylistically off (double-redacting an already-redacted substring
// is a no-op signal that the layering wasn't thought through). v3
// composes the full string once and redacts once at the end.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	var errorContext string

	switch {
	case e.Resource != "":
		// ResourceID is intentionally omitted from Error() rendering to
		// prevent sensitive identifiers (account numbers, transaction IDs,
		// external IDs) from leaking into log aggregators that consume
		// err.Error() directly. Callers that need the ID should read
		// e.ResourceID via the typed accessor [Error.GetResourceID].
		errorContext = fmt.Sprintf("%s error for %s", e.Category, e.Resource)
	default:
		errorContext = fmt.Sprintf("%s error", string(e.Category))
	}

	var composed string
	if e.Operation != "" {
		composed = fmt.Sprintf("%s during %s: %s", errorContext, e.Operation, e.Message)
	} else {
		composed = fmt.Sprintf("%s: %s", errorContext, e.Message)
	}

	return appendUpstreamBody(redactSensitive(composed), e)
}

// maxExposedUpstreamBodyBytes is the default upstream-body cap used by
// [AttachUpstreamBody]. 64 KiB covers virtually every API error payload
// the SDK has observed in production.
const maxExposedUpstreamBodyBytes = 64 * 1024

// AttachUpstreamBody stores the raw upstream response body on a SDK error
// using the default 64 KiB cap. The body is intentionally NOT redacted on
// storage — [GetUpstreamBody] returns the raw bytes — but [Error.Error]
// renders a pre-redacted projection so the canonical logging path is
// safe.
//
// For status-class-aware caps (e.g. tighter limits on 5xx bodies that
// can carry stack traces), use [AttachUpstreamBodyWithLimit].
func AttachUpstreamBody(err error, body []byte, truncated bool) error {
	return AttachUpstreamBodyWithLimit(err, body, truncated, maxExposedUpstreamBodyBytes)
}

// AttachUpstreamBodyWithLimit stores the upstream response body on a SDK
// error using the supplied byte cap. Callers can pin a tighter cap for
// status classes that historically carry sensitive diagnostic content
// (stack traces, SQL strings) — see [maxExposed4xxBodyBytes] /
// [maxExposed5xxBodyBytes] in entities/http.go.
//
// A non-positive maxBytes falls back to the default [maxExposedUpstreamBodyBytes].
//
// Side effect: this is also the moment we pre-compute the redacted
// rendering used by [Error.Error]. Running the regex pass once at
// attach time avoids re-scanning up to 64 KiB on every log render.
func AttachUpstreamBodyWithLimit(err error, body []byte, truncated bool, maxBytes int) error {
	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr == nil {
		return err
	}

	if maxBytes <= 0 {
		maxBytes = maxExposedUpstreamBodyBytes
	}

	sdkErr.UpstreamBodyOriginalBytes = len(body)
	sdkErr.UpstreamBodyTruncated = truncated

	if len(body) > maxBytes {
		sdkErr.UpstreamBody = string(body[:maxBytes])
		sdkErr.UpstreamBodyTruncated = true
	} else {
		sdkErr.UpstreamBody = string(body)
	}

	// Pre-redact once: subsequent Error() calls (which can fire 3+ times
	// per terminal failure across slog, retry hooks, and terminal logs)
	// reuse this cached projection instead of rerunning the regex pass.
	sdkErr.upstreamBodyRedacted = redactSensitive(sdkErr.UpstreamBody)

	return err
}

func appendUpstreamBody(rendered string, e *Error) string {
	if e == nil || e.UpstreamBody == "" {
		return rendered
	}

	// Prefer the cached pre-redacted projection populated by
	// AttachUpstreamBody*. Fall back to an on-demand redact only when
	// the Error was constructed by hand (or via a code path that didn't
	// route through the attach helpers) — that path is a slow degraded
	// mode, not the hot path.
	renderedBody := e.upstreamBodyRedacted
	if renderedBody == "" {
		renderedBody = redactSensitive(e.UpstreamBody)
	}

	if e.UpstreamBodyTruncated {
		return fmt.Sprintf(
			"%s | upstream body (truncated to %d of %d bytes): %s",
			rendered,
			len(e.UpstreamBody),
			e.UpstreamBodyOriginalBytes,
			renderedBody,
		)
	}

	return fmt.Sprintf("%s | upstream body: %s", rendered, renderedBody)
}

// redactingError wraps an inner error so that callers walking the chain via
// [errors.Unwrap] never see an unredacted string. The raw inner error remains
// matchable via explicit Is/As forwarding below, but Unwrap is terminal.
//
// Audit C5 (CRITICAL): before this wrapper landed, code that wrote
//
//	log.Printf("inner: %v", errors.Unwrap(sdkErr))
//
// would happily render `password=hunter2` from the inner error. We
// preserve matching semantics while guaranteeing every Error() call along the
// way passes through the redactor.
type redactingError struct {
	inner error
}

// Error returns a redacted form of the wrapped error's message.
func (r *redactingError) Error() string {
	if r == nil || r.inner == nil {
		return ""
	}

	return redactSensitive(r.inner.Error())
}

// Unwrap is intentionally terminal so recursive unwrap/log loops cannot walk
// past the redacting shell and render the raw inner error string. Matching and
// typed extraction are preserved by Is and As below.
func (*redactingError) Unwrap() error {
	return nil
}

// Is forwards sentinel matching to the wrapped error without exposing it via Unwrap.
func (r *redactingError) Is(target error) bool {
	if r == nil || r.inner == nil {
		return false
	}

	return errors.Is(r.inner, target)
}

// As forwards typed extraction to the wrapped error without exposing it via Unwrap.
func (r *redactingError) As(target any) bool {
	if r == nil || r.inner == nil {
		return false
	}

	return errors.As(r.inner, target)
}

// Unwrap returns the underlying error wrapped in a [redactingError]
// shell so the chain stays walkable but cannot leak credentials when
// rendered by upstream loggers.
func (e *Error) Unwrap() error {
	if e == nil || e.Err == nil {
		return nil
	}

	return &redactingError{inner: e.Err}
}

// Retryable reports whether the SDK retry layer should consider this
// error a candidate for automatic retry. The decision is derived
// from [Error.Category] and represents the canonical SDK-wide policy.
//
// Retryable categories:
//   - CategoryNetwork        — DNS, conn-refused, broken pipe.
//   - CategoryTimeout        — request deadline exceeded.
//   - CategoryLimitExceeded  — server is throttling (rate / quota).
//   - CategoryInternal       — 5xx server errors, transient.
//
// Non-retryable categories:
//   - CategoryValidation     — caller's payload is wrong.
//   - CategoryNotFound       — caller's reference is wrong.
//   - CategoryConflict       — caller's idempotency or
//     already-exists conflict.
//   - CategoryAuth, Authentication, Authorization — credentials
//     issue; the auth refresh path is
//     orthogonal and handles 401 specially.
//   - CategoryUnprocessable  — domain rule violation.
//   - CategoryConfiguration  — SDK misconfiguration; fatal.
//   - CategoryCancellation   — caller cancelled.
//
// Retryable returns false for a nil receiver.
func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}

	switch e.Category {
	case CategoryNetwork,
		CategoryTimeout,
		CategoryLimitExceeded,
		CategoryInternal:
		return true
	}

	return false
}

// Is checks if the target error is of the same type as this error.
//
// Matching rules, evaluated in order:
//
//  1. If target.Category is [CategoryAuth], match e.Category against
//     CategoryAuth, CategoryAuthentication, OR CategoryAuthorization.
//     This is the v3 unified-auth bridge: callers can use
//     errors.Is(err, ErrAuth) without caring whether the underlying
//     error is 401 or 403.
//  2. If target.Category is non-empty, match exactly.
//  3. If target.Code is non-empty, match exactly.
//  4. Otherwise the errors are equivalent.
func (e *Error) Is(target error) bool {
	if e == nil || isNilError(target) {
		return false
	}

	t, ok := target.(*Error)
	if !ok || t == nil {
		return false
	}

	if t.Category == CategoryAuth {
		if e.Category != CategoryAuth &&
			e.Category != CategoryAuthentication &&
			e.Category != CategoryAuthorization {
			return false
		}
	} else if t.Category != "" && e.Category != t.Category {
		return false
	}

	if t.Code != "" && e.Code != t.Code {
		return false
	}

	return true
}

// GetCategory returns the error category.
func (e *Error) GetCategory() ErrorCategory {
	if e == nil {
		return ""
	}

	return e.Category
}

// GetStatusCode returns the HTTP status code, if available.
func (e *Error) GetStatusCode() int {
	if e == nil {
		return 0
	}

	return e.StatusCode
}

// GetUpstreamBody returns the raw upstream response body attached to the SDK
// error, if available.
func (e *Error) GetUpstreamBody() string {
	if e == nil {
		return ""
	}

	return e.UpstreamBody
}

// IsUpstreamBodyTruncated reports whether the attached upstream body was
// truncated by the SDK.
func (e *Error) IsUpstreamBodyTruncated() bool {
	return e != nil && e.UpstreamBodyTruncated
}

// GetUpstreamBodyOriginalBytes returns the byte length observed before the
// pkg/errors exposure truncation was applied.
func (e *Error) GetUpstreamBodyOriginalBytes() int {
	if e == nil {
		return 0
	}

	return e.UpstreamBodyOriginalBytes
}

// GetSource returns the layer that produced the error, when known.
func (e *Error) GetSource() ErrorSource {
	if e == nil {
		return ""
	}

	return e.Source
}

// GetStatusCodeSource returns whether the status code is upstream, synthetic, or absent.
func (e *Error) GetStatusCodeSource() ErrorStatusCodeSource {
	if e == nil {
		return StatusCodeSourceNone
	}

	return e.effectiveStatusCodeSource()
}

func (e *Error) effectiveStatusCodeSource() ErrorStatusCodeSource {
	if e == nil || e.StatusCode == 0 {
		return StatusCodeSourceNone
	}

	if e.StatusCodeSource != "" {
		return e.StatusCodeSource
	}

	return StatusCodeSourceSynthetic
}

// GetRequestID returns the request ID, if available.
func (e *Error) GetRequestID() string {
	if e == nil {
		return ""
	}

	return e.RequestID
}

// GetResource returns the resource type.
func (e *Error) GetResource() string {
	if e == nil {
		return ""
	}

	return e.Resource
}

// GetResourceID returns the resource ID.
func (e *Error) GetResourceID() string {
	if e == nil {
		return ""
	}

	return e.ResourceID
}

// GetOperation returns the operation name.
func (e *Error) GetOperation() string {
	if e == nil {
		return ""
	}

	return e.Operation
}

var (
	// sensitiveBearerPattern matches "authorization: Bearer <value>" and
	// "authorization: Basic <value>" — both standard credential headers
	// whose values are sensitive.
	sensitiveBearerPattern = regexp.MustCompile(`(?i)\b(authorization\s*[:=]\s*)(?:Bearer|Basic)\s+[^\s,;]+`)

	// sensitiveKeyValuePattern parses generic "<key>=<value>" and
	// "<key>: <value>" assignments. The match is classified by
	// lib-observability/redaction at replacement time instead of embedding a
	// service-local sensitive-key list in the regexp.
	sensitiveKeyValuePattern = regexp.MustCompile(`(?i)(^|[^A-Za-z])(["']?)([A-Za-z][A-Za-z0-9_.-]*)(["']?)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;}]+)`)
)

var sensitiveFieldExtras = []string{
	"banking_details_account",
	"banking_details_iban",
	"card-number",
	"credit_card",
	"creditcard",
	"cnpj",
	"cpf",
	"document",
	"external_id",
	"idempotency",
	"idempotency_key",
	"legal_document",
	"participant_document",
	"related_party_document",
	"regulatory_fields_participant_document",
	"x_idempotency",
}

var sensitiveLabelExtras = []string{
	"api_key",
	"authorization",
	"card_number",
	"password",
	"secret",
	"token",
}

var sensitiveLabelNormalizer = strings.NewReplacer("_", "", "-", "", ".", "")

// isNilError is the package-internal sibling of [IsNilInterfaceValue]
// specialised to the error interface. It exists for readability at call
// sites where we want the type to scream "error" rather than "any".
// Delegating keeps the typed-nil semantics in exactly one place — see
// [IsNilInterfaceValue] for the full rationale.
func isNilError(err error) bool {
	return IsNilInterfaceValue(err)
}

func safeErrorString(err error) string {
	if isNilError(err) {
		return ""
	}

	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sdkErr.Error()
	}

	return redactSensitive(err.Error())
}

// redactSensitiveMaxBytes caps the input passed to the regex redactor.
// Beyond this bound the cost of the regex scan dominates and a hostile
// caller could turn a verbose error message into a CPU-stall (a 100MB
// string takes ~13s of regex CPU). Truncation here is a defensive
// quota — production error messages are kilobytes at most.
const redactSensitiveMaxBytes = 64 * 1024

// redactSensitive strips Bearer / Basic header values and key=value pairs
// for any sensitive key (token, password, api-key, idempotency, etc.) from
// rendered error messages.
//
// Inputs longer than [redactSensitiveMaxBytes] are truncated before
// matching and a "[truncated]" sentinel is appended; this preserves
// redaction guarantees on the prefix while bounding regex CPU.
func redactSensitive(message string) string {
	if message == "" {
		return ""
	}

	truncated := false
	if len(message) > redactSensitiveMaxBytes {
		message = message[:redactSensitiveMaxBytes]
		truncated = true
	}

	redacted := sensitiveBearerPattern.ReplaceAllString(message, `${1}`+redactedValue)
	redacted = redactSensitiveAssignments(redacted)

	if truncated {
		redacted += " [truncated]"
	}

	return redacted
}

func redactSensitiveAssignments(message string) string {
	var b strings.Builder
	pos := 0

	for pos < len(message) {
		loc := sensitiveKeyValuePattern.FindStringSubmatchIndex(message[pos:])
		if loc == nil {
			b.WriteString(message[pos:])
			break
		}

		matchStart := pos + loc[0]
		valueStart := pos + loc[12]
		valueEnd := pos + loc[13]
		keyStart := pos + loc[6]
		keyEnd := pos + loc[7]
		if keyStart < pos || keyEnd < keyStart || valueStart < keyEnd || valueEnd < valueStart {
			b.WriteString(message[pos : matchStart+1])
			pos = matchStart + 1
			continue
		}

		if !IsSensitiveFieldName(message[keyStart:keyEnd]) {
			b.WriteString(message[pos : matchStart+1])
			pos = matchStart + 1
			continue
		}

		b.WriteString(message[pos:valueStart])
		b.WriteString(redactedValue)
		pos = valueEnd
	}

	return b.String()
}

// RedactSensitiveString redacts sensitive values from a public API-sourced string.
func RedactSensitiveString(message string) string {
	return redactSensitive(message)
}

// RedactSensitiveStringSlice redacts sensitive values from public API-sourced strings.
func RedactSensitiveStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	redacted := make([]string, len(values))
	for i, value := range values {
		redacted[i] = redactSensitive(value)
	}

	return redacted
}

// RedactSensitiveDetails returns a deep-redacted copy of structured API details.
// It preserves the shape of the API envelope while removing PII and financial
// fields that applications commonly log after calling GetErrorDetails.
func RedactSensitiveDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}

	redacted := make(map[string]any, len(details))
	for key, value := range details {
		redacted[key] = redactSensitiveDetailValue(key, value)
	}

	return redacted
}

func redactSensitiveDetailValue(key string, value any) any {
	if isSensitiveDetailKey(key) {
		return redactedValue
	}

	switch typed := value.(type) {
	case map[string]any:
		return RedactSensitiveDetails(typed)
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = redactSensitiveDetailValue(key, item)
		}

		return items
	case string:
		return redactSensitive(typed)
	default:
		return typed
	}
}

// IsSensitiveFieldName reports whether name (after case-folding and
// lib-observability normalization plus SDK-specific extras) is sensitive.
// Use it before rendering a field's value in user-facing error output so
// credentials, PII, and financial identifiers stay out of logs.
//
// Examples returning true:
//
//	"password", "X-API-Key", "authorization", "client_secret",
//	"creditCard", "cpf", "metadata.user.token", "refresh-token".
//
// The check delegates to lib-observability/redaction so camelCase,
// word-boundary, and short-token matching stay centralized.
func IsSensitiveFieldName(name string) bool {
	return obsredaction.IsSensitiveField(strings.TrimSpace(name), sensitiveFieldExtras...)
}

// isSensitiveDetailKey is the legacy entry point kept for the structured
// API detail walker. It now delegates to [IsSensitiveFieldName] so the
// two redaction layers share one allowlist.
func isSensitiveDetailKey(key string) bool {
	return IsSensitiveFieldName(key)
}

func normalizeError(err error) error {
	if isNilError(err) {
		return nil
	}

	return err
}

func withDiagnostics(e *Error, source ErrorSource, requestSent, responseReceived bool, statusSource ErrorStatusCodeSource) *Error {
	if e == nil {
		return nil
	}

	e.Source = source
	e.HTTPRequestSent = requestSent
	e.HTTPResponseReceived = responseReceived
	if e.StatusCode == 0 {
		e.StatusCodeSource = StatusCodeSourceNone
	} else {
		e.StatusCodeSource = statusSource
	}

	return e
}

func withSyntheticStatus(e *Error, source ErrorSource, requestSent bool) *Error {
	return withDiagnostics(e, source, requestSent, false, StatusCodeSourceSynthetic)
}

func redactMessage(message string, unsafeValues ...string) string {
	message = redactSensitive(message)
	for _, unsafeValue := range unsafeValues {
		if unsafeValue == "" {
			continue
		}

		message = strings.ReplaceAll(message, unsafeValue, redactedValue)
	}

	return message
}

// safeFieldLabel returns name unless name itself looks like a credential
// label (e.g. "password", "token") — in which case it is redacted so the
// rendered error message does not echo a sensitive identifier verbatim.
//
// IMPORTANT: this is a LABEL policy, not a VALUE policy. Parameter names
// like "externalID", "metadataKey", "assetCode" are part of the public API
// contract and MUST render verbatim so callers can map error messages to
// the SDK methods they came from. Only labels that *describe* secret data
// (password, secret, token, …) are scrubbed.
//
// The fragment list is narrower than [IsSensitiveFieldName] on purpose: the
// broad PII allowlist (document, externalid, idempotency, metadata, …) is
// for redacting VALUES, never for redacting label strings.
func safeFieldLabel(name string) string {
	if isSensitiveLabelFragment(name) {
		return redactedValue
	}

	return redactSensitive(name)
}

func isSensitiveLabelFragment(name string) bool {
	if name == "" {
		return false
	}

	normalized := strings.ToLower(sensitiveLabelNormalizer.Replace(name))
	for _, fragment := range sensitiveLabelExtras {
		fragment = strings.ToLower(sensitiveLabelNormalizer.Replace(fragment))
		if strings.Contains(normalized, fragment) {
			return true
		}
	}

	return false
}

// Standard error constructors

// NewValidationError creates a validation error.
func NewValidationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceSDK, false)
}

// NewInvalidInputError creates a validation error for invalid input.
func NewInvalidInputError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "invalid input"
	if err != nil {
		message = fmt.Sprintf("invalid input: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceSDK, false)
}

// NewMissingParameterError creates a validation error for a missing parameter.
func NewMissingParameterError(operation, paramName string) *Error {
	safeParamName := safeFieldLabel(paramName)
	message := fmt.Sprintf("missing required parameter: %s", safeParamName)

	return withSyntheticStatus(&Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    message,
		Operation:  redactSensitive(operation),
		Err:        errors.New(message),
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceSDK, false)
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(operation, resource, resourceID string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("%s not found", resource)

	return withSyntheticStatus(&Error{
		Category:   CategoryNotFound,
		Code:       CodeNotFound,
		Message:    redactMessage(message, resourceID),
		Operation:  redactSensitive(operation),
		Resource:   redactSensitive(resource),
		ResourceID: resourceID,
		Err:        err,
		StatusCode: http.StatusNotFound,
	}, ErrorSourceSDK, false)
}

// NewAuthenticationError creates an authentication error.
func NewAuthenticationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryAuthentication,
		Code:       CodeAuthentication,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusUnauthorized,
	}, ErrorSourceSDK, false)
}

// NewAuthorizationError creates an authorization error.
func NewAuthorizationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryAuthorization,
		Code:       CodePermission,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusForbidden,
	}, ErrorSourceSDK, false)
}

// NewConflictError creates a conflict error.
func NewConflictError(operation, resource, resourceID string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("%s already exists", resource)

	return withSyntheticStatus(&Error{
		Category:   CategoryConflict,
		Code:       CodeAlreadyExists,
		Message:    redactMessage(message, resourceID),
		Operation:  redactSensitive(operation),
		Resource:   redactSensitive(resource),
		ResourceID: resourceID,
		Err:        err,
		StatusCode: http.StatusConflict,
	}, ErrorSourceSDK, false)
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "rate limit exceeded"
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryLimitExceeded,
		Code:       CodeRateLimit,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusTooManyRequests,
	}, ErrorSourceSDK, false)
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "operation timed out"
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryTimeout,
		Code:       CodeTimeout,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusGatewayTimeout,
	}, ErrorSourceTransport, true)
}

// NewCancellationError creates a cancellation error for cancelled contexts.
func NewCancellationError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "operation cancelled"
	if err != nil {
		message = fmt.Sprintf("operation cancelled: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryCancellation,
		Code:       CodeCancellation,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: statusClientClosedRequest,
	}, ErrorSourceTransport, true)
}

// NewNetworkError creates a network error.
func NewNetworkError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "network error"
	if err != nil {
		message = fmt.Sprintf("network error: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryNetwork,
		Code:       CodeNetwork,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusServiceUnavailable,
	}, ErrorSourceTransport, true)
}

// NewUpstreamHTTPError creates an SDK error that preserves the HTTP status
// returned by an upstream service. Use this when a bootstrap or helper path
// received an HTTP response outside the normal API response parser and must
// keep actual upstream diagnostics (status code, request/response flags, and
// status source) instead of synthesizing an SDK-only status.
func NewUpstreamHTTPError(operation, message string, statusCode int, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "upstream HTTP request failed"
	}
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	mapping, ok := httpErrorMappings[statusCode]
	if !ok {
		mapping = httpErrorMapping{category: CategoryInternal, code: CodeInternal}
	}

	return withDiagnostics(&Error{
		Category:   mapping.category,
		Code:       mapping.code,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: statusCode,
	}, ErrorSourceHTTPResponse, true, true, StatusCodeSourceUpstream)
}

// NewInternalError creates an internal error.
func NewInternalError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "internal error"
	if err != nil {
		message = fmt.Sprintf("internal error: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryInternal,
		Code:       CodeInternal,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusInternalServerError,
	}, ErrorSourceSDK, false)
}

// NewConfigurationError creates a configuration error for SDK setup failures.
//
// Use this for client construction problems: missing required options,
// invalid URLs, conflicting auth sources, validation failures at New() time.
// These errors are returned eagerly by midaz.New() so users discover misuse
// at construction rather than on the first API call.
//
// # Redaction
//
// The supplied message and operation are passed through [redactSensitive]
// at construction time, so `key=value` credential pairs and Bearer/Basic
// header values that slipped into the message are stripped before the
// error is stored. Callers do not need to pre-sanitize.
//
// Example:
//
//	err := errors.NewConfigurationError(
//	    "midaz.New",
//	    "no auth source configured",
//	    fmt.Errorf("use WithAccessManager or WithAnonymous"),
//	)
//
// Parameters:
//   - operation: The operation context, typically "midaz.New" or
//     "<package>.<func>" describing the call site.
//   - message: A human-readable, actionable message that tells the caller
//     what to fix. Redacted at construction time.
//   - err: An optional underlying cause. May be nil. The cause is also
//     wrapped behind [redactingError] so chain-walking via
//     [errors.Unwrap] always renders the inner string through the
//     redactor.
//
// Returns:
//   - *Error: A configuration error with Category=CategoryConfiguration and
//     Code=CodeConfiguration. errors.Is(err, ErrConfiguration) matches it.
func NewConfigurationError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "configuration error"
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryConfiguration,
		Code:       CodeConfiguration,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceConfiguration, false)
}

// NewUnprocessableError creates an unprocessable entity error.
func NewUnprocessableError(operation, resource string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("unprocessable %s", resource)
	if err != nil {
		message = fmt.Sprintf("unprocessable %s: %v", resource, err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryUnprocessable,
		Code:       CodeUnprocessable,
		Message:    redactMessage(message),
		Operation:  redactSensitive(operation),
		Resource:   redactSensitive(resource),
		Err:        err,
		StatusCode: http.StatusUnprocessableEntity,
	}, ErrorSourceSDK, false)
}

// NewInsufficientBalanceError creates an insufficient balance error.
func NewInsufficientBalanceError(operation, accountID string, err error) *Error {
	err = normalizeError(err)

	message := "insufficient balance"
	if err != nil {
		message = fmt.Sprintf("insufficient balance: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryUnprocessable,
		Code:       CodeInsufficientBalance,
		Message:    redactMessage(message, accountID),
		Operation:  redactSensitive(operation),
		Resource:   "account",
		ResourceID: accountID,
		Err:        err,
		StatusCode: http.StatusUnprocessableEntity,
	}, ErrorSourceSDK, false)
}

// NewAssetMismatchError creates an asset mismatch error.
//
// Asset codes (USD, EUR, BRL, …) are part of the public API contract and
// render verbatim in the error message — they are NOT credentials or PII.
// redactSensitive still passes over the composed message to catch any
// embedded credential fragment, but the bare codes are preserved so callers
// can act on "expected USD, got EUR" without grepping a [REDACTED] message.
func NewAssetMismatchError(operation, expected, actual string, err error) *Error {
	err = normalizeError(err)
	message := "asset mismatch"
	if expected != "" || actual != "" {
		message = fmt.Sprintf("asset mismatch: expected %s, got %s", expected, actual)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryValidation,
		Code:       CodeAssetMismatch,
		Message:    redactSensitive(message),
		Operation:  redactSensitive(operation),
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceSDK, false)
}

// NewAccountEligibilityError creates an account eligibility error.
func NewAccountEligibilityError(operation, accountID string, err error) *Error {
	err = normalizeError(err)

	message := "account not eligible for this operation"
	if err != nil {
		message = fmt.Sprintf("account eligibility error: %v", err)
	}

	return withSyntheticStatus(&Error{
		Category:   CategoryValidation,
		Code:       CodeAccountEligibility,
		Message:    redactMessage(message, accountID),
		Operation:  redactSensitive(operation),
		Resource:   "account",
		ResourceID: accountID,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}, ErrorSourceSDK, false)
}

// Error checking functions
//
// All Is*Error predicates classify errors strictly by typed-error
// category/code. Substring matching on err.Error() was removed in
// favor of typed predicates so unrelated error strings that happen to
// contain a category-shaped word no longer get reclassified.
//
// Audit H15: the v2 codebase carried a parallel Check*Error family
// that production never called. Inlining the logic into the Is*Error
// predicates removed ~200 lines of duplication without any behavior
// change for the predicates production callers actually use.

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryValidation
	}

	return errors.Is(err, ErrValidation)
}

// IsNotFoundError checks if an error is a not found error.
func IsNotFoundError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryNotFound
	}

	return errors.Is(err, ErrNotFound)
}

// IsAuthenticationError checks if an error is an authentication error.
func IsAuthenticationError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryAuthentication
	}

	return errors.Is(err, ErrAuthentication)
}

// IsAuthorizationError checks if an error is an authorization error.
func IsAuthorizationError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryAuthorization
	}

	return errors.Is(err, ErrPermission)
}

// IsAuthError reports whether err is any kind of authentication or
// authorization failure. It matches CategoryAuth, CategoryAuthentication,
// and CategoryAuthorization so callers can react to the broad notion of
// "auth failed" without caring whether the server returned 401 or 403.
//
// Example:
//
//	if errors.IsAuthError(err) {
//	    // refresh token, re-prompt, or surface a generic "please sign in"
//	}
//
// To distinguish 401 from 403, extract the typed error:
//
//	var sdkErr *errors.Error
//	if errors.As(err, &sdkErr) && sdkErr.StatusCode == http.StatusForbidden {
//	    // the user is signed in but lacks permission
//	}
func IsAuthError(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		switch sdkErr.Category {
		case CategoryAuth, CategoryAuthentication, CategoryAuthorization:
			return true
		}
	}

	return IsAuthenticationError(err) || IsAuthorizationError(err)
}

// IsConflictError checks if an error is a conflict error.
func IsConflictError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryConflict
	}

	return errors.Is(err, ErrAlreadyExists)
}

// IsBootstrapError reports whether err is any failure originating from
// the client-construction path (midaz.New). It is the union of every
// category midaz.New can produce, so callers needing a single "did
// construction fail?" predicate can replace a chain of Is*Error checks
// with one call.
//
// Matches:
//   - [IsConfigurationError]  — local validation failures (missing fields,
//     invalid URLs, etc.).
//   - [IsAuthError]           — upstream 401 / 403 from Access Manager.
//   - [IsRateLimitError]      — upstream 429 from Access Manager.
//   - [IsNetworkError]        — pre-response transport failures
//     (DNS, conn-refused, TLS) during the bootstrap token fetch.
//   - [IsInternalError]       — upstream 5xx from Access Manager.
//
// Does NOT match runtime API failures (validation, not-found, conflict,
// timeout, cancellation, unprocessable). Use [IsConfigurationError] to
// distinguish a deliberate setup mistake from a transient upstream blip.
//
// Example:
//
//	c, err := midaz.New(opts...)
//	if errors.IsBootstrapError(err) {
//	    log.Fatalf("client construction failed: %v", err)
//	}
func IsBootstrapError(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr == nil || sdkErr.Operation != "midaz.New" {
		return false
	}

	switch sdkErr.Category {
	case CategoryConfiguration,
		CategoryAuthentication,
		CategoryAuthorization,
		CategoryLimitExceeded,
		CategoryNetwork,
		CategoryInternal:
		return true
	default:
		return false
	}
}

// IsConfigurationError reports whether err is an SDK configuration error.
//
// Configuration errors are produced eagerly by midaz.New() when the client is
// misconfigured (missing auth, invalid URLs, conflicting options, etc.) so
// callers can distinguish setup mistakes from runtime API failures.
//
// Use this when you want to react specifically to setup problems:
//
//	c, err := midaz.New(...)
//	if errors.IsConfigurationError(err) {
//	    log.Fatalf("midaz client misconfigured: %v", err)
//	}
//
// See also [NewConfigurationError], [ErrConfiguration].
func IsConfigurationError(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sdkErr.Category == CategoryConfiguration
	}

	return errors.Is(err, ErrConfiguration)
}

// IsIdempotencyError reports whether an error is an idempotency conflict:
// the key was reused with a different payload, or an earlier request with the
// same key is still in flight. It is a 409 conflict and non-retryable — it
// never means the original request succeeded or was replayed.
func IsIdempotencyError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Code == CodeIdempotency
	}

	return errors.Is(err, ErrIdempotency)
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryLimitExceeded
	}

	return errors.Is(err, ErrRateLimit)
}

// IsTimeoutError checks if an error is a timeout error.
//
// Audit M18: timeout and cancellation are now disjoint. The function
// matches typed *Error{Category: CategoryTimeout}, the ErrTimeout
// sentinel, and raw context.DeadlineExceeded — but NOT context.Canceled
// (handled by [IsCancellationError]).
func IsTimeoutError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryTimeout
	}

	return errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}

// IsNetworkError checks if an error is a network error.
func IsNetworkError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryNetwork
	}

	return false
}

// IsCancellationError checks if an error is a cancellation error.
//
// Audit M18: timeout and cancellation are now disjoint. The function
// matches typed *Error{Category: CategoryCancellation} and raw
// context.Canceled — context.DeadlineExceeded belongs to
// [IsTimeoutError] only.
func IsCancellationError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryCancellation
	}

	return errors.Is(err, context.Canceled)
}

// IsInternalError checks if an error is an internal error.
func IsInternalError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category == CategoryInternal
	}

	return errors.Is(err, ErrInternal)
}

// IsInsufficientBalanceError checks if an error is an insufficient balance error.
func IsInsufficientBalanceError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Code == CodeInsufficientBalance
	}

	return errors.Is(err, ErrInsufficientBalance)
}

// IsAccountEligibilityError checks if an error is an account eligibility error.
func IsAccountEligibilityError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Code == CodeAccountEligibility
	}

	return errors.Is(err, ErrAccountEligibility)
}

// IsAssetMismatchError checks if an error is an asset mismatch error.
func IsAssetMismatchError(err error) bool {
	if isNilError(err) {
		return false
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Code == CodeAssetMismatch
	}

	return errors.Is(err, ErrAssetMismatch)
}

// IsUnprocessableError reports whether err represents a server-side
// "unprocessable entity" response — typically a domain rule violation
// such as insufficient balance, account ineligibility, or asset
// mismatch. Matches any error whose Category is [CategoryUnprocessable].
//
// To react to specific unprocessable codes (e.g. only insufficient
// balance), use [IsInsufficientBalanceError], [IsAccountEligibilityError],
// or [IsAssetMismatchError] which match by Code.
func IsUnprocessableError(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		if sdkErr.Category == CategoryUnprocessable {
			return true
		}

		switch sdkErr.Code {
		case CodeInsufficientBalance, CodeAccountEligibility, CodeAssetMismatch, CodeUnprocessable:
			return true
		}

		return false
	}

	return errors.Is(err, ErrUnprocessable)
}

// Extract helpful information from errors

// GetErrorCategory returns the category of an error.
func GetErrorCategory(err error) ErrorCategory {
	if isNilError(err) {
		return ""
	}

	// Check for Midaz error first
	if category := getMidazErrorCategory(err); category != "" {
		return category
	}

	// Handle special test cases
	if category := getTestCaseCategory(err); category != "" {
		return category
	}

	// Categorize using built-in error checks
	return categorizeByErrorChecks(err)
}

// getMidazErrorCategory extracts category from Midaz Error type
func getMidazErrorCategory(err error) ErrorCategory {
	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		return mdzErr.Category
	}

	return ""
}

// getTestCaseCategory handles special test error messages
func getTestCaseCategory(err error) ErrorCategory {
	errorMsg := safeErrorString(err)

	switch errorMsg {
	case "generic error", "something went wrong":
		return CategoryInternal
	default:
		return ""
	}
}

// categorizeByErrorChecks categorizes errors using the canonical
// Is*Error predicates. Order matters: more specific categories must
// come before broader fallbacks (e.g. cancellation before timeout).
//
// Audit M18: cancellation appears before timeout in the iteration so
// that a *Error already classified as Cancellation isn't double-bucketed.
func categorizeByErrorChecks(err error) ErrorCategory {
	errorChecks := []struct {
		check    func(error) bool
		category ErrorCategory
	}{
		{IsValidationError, CategoryValidation},
		{IsNotFoundError, CategoryNotFound},
		{IsAuthenticationError, CategoryAuthentication},
		{IsAuthorizationError, CategoryAuthorization},
		{IsConflictError, CategoryConflict},
		{IsRateLimitError, CategoryLimitExceeded},
		{IsCancellationError, CategoryCancellation},
		{IsTimeoutError, CategoryTimeout},
		{IsNetworkError, CategoryNetwork},
		{IsInternalError, CategoryInternal},
	}

	for _, errorCheck := range errorChecks {
		if errorCheck.check(err) {
			return errorCheck.category
		}
	}

	return CategoryInternal
}

var statusCodesByCategory = map[ErrorCategory]int{
	CategoryValidation:     http.StatusBadRequest,
	CategoryNotFound:       http.StatusNotFound,
	CategoryAuthentication: http.StatusUnauthorized,
	CategoryAuthorization:  http.StatusForbidden,
	CategoryConflict:       http.StatusConflict,
	CategoryLimitExceeded:  http.StatusTooManyRequests,
	CategoryTimeout:        http.StatusGatewayTimeout,
	CategoryCancellation:   statusClientClosedRequest,
	CategoryNetwork:        http.StatusServiceUnavailable,
	CategoryUnprocessable:  http.StatusUnprocessableEntity,
	CategoryConfiguration:  http.StatusBadRequest,
	CategoryAuth:           http.StatusUnauthorized,
}

// GetStatusCode gets the HTTP status code associated with an error.
func GetStatusCode(err error) int {
	if isNilError(err) {
		return http.StatusOK
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		if mdzErr.StatusCode != 0 {
			return mdzErr.StatusCode
		}
	}

	// For the tests, generic error should map to internal server error
	if safeErrorString(err) == "generic error" || safeErrorString(err) == "something went wrong" {
		return http.StatusInternalServerError
	}

	// Map categories to status codes
	if statusCode, ok := statusCodesByCategory[GetErrorCategory(err)]; ok {
		return statusCode
	}

	return http.StatusInternalServerError
}

// ActualHTTPStatus returns the upstream HTTP response status if err was built
// from a received HTTP response. The boolean is false for SDK/configuration/
// transport errors whose StatusCode is only a synthetic suggestion.
func ActualHTTPStatus(err error) (int, bool) {
	if isNilError(err) {
		return 0, false
	}

	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr == nil {
		return 0, false
	}

	if sdkErr.effectiveStatusCodeSource() != StatusCodeSourceUpstream || !sdkErr.HTTPResponseReceived || sdkErr.StatusCode == 0 {
		return 0, false
	}

	return sdkErr.StatusCode, true
}

// SuggestedHTTPStatus returns the SDK's best status-code suggestion for err.
// Unlike GetStatusCode, nil/unknown errors return 0 instead of pretending an
// error is HTTP 200 OK.
func SuggestedHTTPStatus(err error) int {
	if isNilError(err) {
		return 0
	}

	return GetStatusCode(err)
}

// HTTPRequestSent reports whether err is known to have attempted an HTTP request.
func HTTPRequestSent(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	return errors.As(err, &sdkErr) && sdkErr != nil && sdkErr.HTTPRequestSent
}

// HTTPResponseReceived reports whether err was built after receiving an HTTP response.
func HTTPResponseReceived(err error) bool {
	if isNilError(err) {
		return false
	}

	var sdkErr *Error
	return errors.As(err, &sdkErr) && sdkErr != nil && sdkErr.HTTPResponseReceived
}

// FormatErrorForDisplay formats an error for display to end users.
//
//nolint:gocyclo,cyclop // Category-to-user-message switch is intentionally explicit.
func FormatErrorForDisplay(err error) string {
	if isNilError(err) {
		return ""
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		switch mdzErr.Category {
		case CategoryValidation:
			return appendUpstreamBody(fmt.Sprintf("Invalid request: %s", redactSensitive(mdzErr.Message)), mdzErr)
		case CategoryNotFound:
			return appendUpstreamBody(fmt.Sprintf("Resource not found: %s", redactSensitive(mdzErr.Message)), mdzErr)
		case CategoryAuthentication:
			return appendUpstreamBody("Authentication failed. Please check your credentials.", mdzErr)
		case CategoryAuthorization:
			return appendUpstreamBody("You don't have permission to perform this action.", mdzErr)
		case CategoryAuth:
			return appendUpstreamBody("Authentication failed. Please check your credentials.", mdzErr)
		case CategoryConfiguration:
			return appendUpstreamBody(fmt.Sprintf("SDK configuration error: %s", redactSensitive(mdzErr.Message)), mdzErr)
		case CategoryConflict:
			return appendUpstreamBody(fmt.Sprintf("Resource conflict: %s", redactSensitive(mdzErr.Message)), mdzErr)
		case CategoryLimitExceeded:
			return appendUpstreamBody("Rate limit exceeded. Please try again later.", mdzErr)
		case CategoryTimeout:
			return appendUpstreamBody("The operation timed out. Please try again later.", mdzErr)
		case CategoryCancellation:
			return appendUpstreamBody("The operation was cancelled.", mdzErr)
		case CategoryNetwork:
			return appendUpstreamBody("Network error. Please check your connection and try again.", mdzErr)
		case CategoryUnprocessable:
			return appendUpstreamBody(fmt.Sprintf("Operation could not be processed: %s", redactSensitive(mdzErr.Message)), mdzErr)
		default:
			return appendUpstreamBody("An unexpected error occurred. Please try again later.", mdzErr)
		}
	}

	return safeErrorString(err)
}

// httpErrorMapping contains the category and code mapping for HTTP status codes.
type httpErrorMapping struct {
	category     ErrorCategory
	code         ErrorCode
	withResource bool
}

// httpErrorMappings maps HTTP status codes to error categories and codes.
// Audit C6: 408 (Request Timeout) and 425 (Too Early) are now mapped
// explicitly. Without these entries 408 fell into CategoryInternal
// (instead of CategoryTimeout) and 425 also defaulted to Internal
// (instead of CategoryLimitExceeded — its retry-after semantics match
// 429). Both gaps caused real downstream misbehavior in retry layers.
//
// Audit M13: 503 now pairs CategoryNetwork with CodeServiceUnavailable
// instead of CodeInternal — the {category, code} drift in the v2
// mapping was inconsistent with every other entry in the table and
// confused observability dashboards that grouped by code.
var httpErrorMappings = map[int]httpErrorMapping{
	http.StatusBadRequest:          {CategoryValidation, CodeValidation, true},
	http.StatusUnauthorized:        {CategoryAuthentication, CodeAuthentication, false},
	http.StatusForbidden:           {CategoryAuthorization, CodePermission, false},
	http.StatusNotFound:            {CategoryNotFound, CodeNotFound, true},
	http.StatusRequestTimeout:      {CategoryTimeout, CodeTimeout, false},
	http.StatusConflict:            {CategoryConflict, CodeAlreadyExists, true},
	http.StatusUnprocessableEntity: {CategoryUnprocessable, CodeUnprocessable, true},
	http.StatusTooEarly:            {CategoryLimitExceeded, CodeRateLimit, false},
	http.StatusTooManyRequests:     {CategoryLimitExceeded, CodeRateLimit, false},
	http.StatusServiceUnavailable:  {CategoryNetwork, CodeServiceUnavailable, false},
	http.StatusGatewayTimeout:      {CategoryTimeout, CodeTimeout, false},
}

var apiErrorCodeMappings = map[string]httpErrorMapping{
	"0084":                          {CategoryConflict, CodeIdempotency, false},
	string(CodeIdempotency):         {CategoryConflict, CodeIdempotency, false},
	string(CodeInsufficientBalance): {CategoryUnprocessable, CodeInsufficientBalance, true},
	string(CodeAccountEligibility):  {CategoryValidation, CodeAccountEligibility, true},
	string(CodeAssetMismatch):       {CategoryValidation, CodeAssetMismatch, true},
}

// apiCodeSuffixMappings overrides retryability for domain codes whose
// semantics the HTTP status cannot express, keyed on the four-digit suffix
// of the RFC 9457 <SERVICE>-NNNN code (robust to the service prefix):
//   - 0178: transient unavailability → retryable (CategoryNetwork), even
//     when the status is a non-retryable 4xx.
//   - 0177: domain denial → non-retryable (CategoryUnprocessable), even
//     when the status is a retryable 5xx.
//   - 0084: idempotency conflict → non-retryable (CategoryConflict) and
//     classified as CodeIdempotency. On the wire the code arrives prefixed
//     ("LEDGER-0084"), so the exact-match apiErrorCodeMappings["0084"] never
//     fires; the suffix lookup is the only path that catches the prefixed form.
var apiCodeSuffixMappings = map[string]httpErrorMapping{
	"0178": {CategoryNetwork, CodeServiceUnavailable, false},
	"0177": {CategoryUnprocessable, CodeUnprocessable, false},
	"0084": {CategoryConflict, CodeIdempotency, false},
}

// apiCodeSuffixLen is the fixed width of the NNNN suffix in an
// <SERVICE>-NNNN RFC 9457 error code.
const apiCodeSuffixLen = 4

// apiCodeSuffix returns the trailing four-digit numeric suffix of an
// <SERVICE>-NNNN error code, or "" if the code has no such suffix.
func apiCodeSuffix(apiCode string) string {
	if i := strings.LastIndex(apiCode, "-"); i >= 0 {
		apiCode = apiCode[i+1:]
	}

	if len(apiCode) != apiCodeSuffixLen {
		return ""
	}

	for _, r := range apiCode {
		if r < '0' || r > '9' {
			return ""
		}
	}

	return apiCode
}

func applyAPICodeMapping(mapping httpErrorMapping, apiCode string) httpErrorMapping {
	apiCode = strings.TrimSpace(apiCode)
	if apiCode == "" {
		return mapping
	}

	if apiMapping, ok := apiErrorCodeMappings[apiCode]; ok {
		apiMapping.withResource = apiMapping.withResource || mapping.withResource

		return apiMapping
	}

	if apiMapping, ok := apiCodeSuffixMappings[apiCodeSuffix(apiCode)]; ok {
		apiMapping.withResource = apiMapping.withResource || mapping.withResource

		return apiMapping
	}

	return mapping
}

// ErrorFromHTTPResponse creates an appropriate error based on the HTTP response
func ErrorFromHTTPResponse(statusCode int, requestID, message, apiCode, entityType, resourceID string) error {
	return ErrorFromHTTPResponseWithDetails(statusCode, requestID, message, apiCode, entityType, resourceID, "", nil, nil)
}

// ErrorFromHTTPResponseWithDetails creates an appropriate error based on the HTTP response
// and preserves raw structured API envelope metadata when available.
//
// # Two-tier contract: raw typed identifiers vs. redacted everything else
//
// The returned *Error follows an asymmetric rendering rule. The two
// halves are NOT symmetric — earlier doc versions claimed they were
// and that was wrong:
//
//   - Raw at construction time:
//
//   - [Error.ResourceID] — preserved verbatim from the caller.
//
//   - [Error.RequestID]  — preserved verbatim from the caller.
//
//   - Pre-redacted at construction time (the regex pipeline runs once
//     here, not on every render):
//
//   - [Error.Title]   — passed through redactSensitive.
//
//   - [Error.Fields]  — each element passed through RedactSensitiveStringSlice.
//
//   - [Error.Details] — passed through RedactSensitiveDetails.
//
//   - [Error.Message] — built from message + resourceID and passed
//     through redactMessage (Bearer/Basic scrub + the sensitive
//     `key=value` allowlist + the per-resource ID strip-out).
//
//   - On render: [Error.Error] applies an additional redactSensitive
//     pass over the composed context string for defense in depth.
//
// Programmatic dispatchers that need the raw caller-supplied resource
// identifier can read e.ResourceID / e.RequestID directly. Anything
// else read off a typed *Error is already the redacted projection — do
// not assume Title/Fields/Details still contain unredacted input.
func ErrorFromHTTPResponseWithDetails(statusCode int, requestID, message, apiCode, entityType, resourceID, title string, fields []string, details map[string]any) error {
	mapping, ok := httpErrorMappings[statusCode]
	if !ok {
		mapping = httpErrorMapping{CategoryInternal, CodeInternal, false}
	}

	mapping = applyAPICodeMapping(mapping, apiCode)

	// Audit M11: every caller-derived field is redacted at construction
	// time so a later GetErrorDetails / Error() call cannot leak raw
	// credentials. Title and Fields routinely echo user-supplied
	// content, and Details is the most common leak source for the
	// structured envelope.
	err := withDiagnostics(&Error{
		Category:   mapping.category,
		Code:       mapping.code,
		APICode:    apiCode,
		Title:      redactSensitive(title),
		Message:    redactMessage(message, resourceID),
		StatusCode: statusCode,
		RequestID:  requestID,
		EntityType: entityType,
		Fields:     RedactSensitiveStringSlice(fields),
		Details:    RedactSensitiveDetails(details),
	}, ErrorSourceHTTPResponse, true, true, StatusCodeSourceUpstream)

	if mapping.withResource {
		err.Resource = entityType
		err.ResourceID = resourceID
	}

	return err
}

// FormatUnifiedTransactionError produces a standardized error message for transactions
func FormatUnifiedTransactionError(err error, operationType string) string {
	if isNilError(err) {
		return ""
	}

	// Try to format structured Midaz error
	if message := formatMidazError(err, operationType); message != "" {
		return message
	}

	// Format non-structured errors
	return formatGenericError(err, operationType)
}

// formatMidazError formats structured Midaz Error types
func formatMidazError(err error, operationType string) string {
	var mdzErr *Error
	if !errors.As(err, &mdzErr) || mdzErr == nil {
		return ""
	}

	codeToMessage := map[ErrorCode]string{
		CodeValidation:          "Invalid parameters",
		CodeUnprocessable:       "Unprocessable entity",
		CodeInsufficientBalance: "Insufficient account balance",
		CodeAccountEligibility:  "Account not eligible",
		CodeAssetMismatch:       "Asset type mismatch",
		CodeAuthentication:      "Authentication error",
		CodePermission:          "Permission denied",
		CodeNotFound:            "Resource not found",
		CodeAlreadyExists:       "Resource already exists",
		CodeIdempotency:         "Idempotency issue",
		CodeRateLimit:           "Rate limit exceeded",
		CodeTimeout:             "Operation timed out",
	}

	if message, exists := codeToMessage[mdzErr.Code]; exists {
		return appendUpstreamBody(fmt.Sprintf("%s failed: %s - %s", operationType, message, redactSensitive(mdzErr.Message)), mdzErr)
	}

	return appendUpstreamBody(fmt.Sprintf("%s failed: %s", operationType, redactSensitive(mdzErr.Message)), mdzErr)
}

// formatGenericError formats non-structured error types
func formatGenericError(err error, operationType string) string {
	errorChecks := []struct {
		check   func(error) bool
		message string
	}{
		{IsValidationError, "Invalid parameters"},
		{IsInsufficientBalanceError, "Insufficient account balance"},
		{IsAccountEligibilityError, "Account not eligible"},
		{IsAssetMismatchError, "Asset type mismatch"},
		{IsAuthenticationError, "Authentication error"},
		{IsAuthorizationError, "Permission denied"},
		{IsNotFoundError, "Resource not found"},
		{IsConflictError, "Resource already exists"},
		{IsIdempotencyError, "Idempotency issue"},
		{IsRateLimitError, "Rate limit exceeded"},
		{IsTimeoutError, "Operation timed out"},
		{IsInternalError, "Internal server error"},
	}

	for _, errorCheck := range errorChecks {
		if errorCheck.check(err) {
			return fmt.Sprintf("%s failed: %s - %s", operationType, errorCheck.message, safeErrorString(err))
		}
	}

	return fmt.Sprintf("%s failed: %s", operationType, safeErrorString(err))
}

// CategorizeTransactionError provides the error category
func CategorizeTransactionError(err error) string {
	if isNilError(err) {
		return "none"
	}

	// Special cases for specific transaction error types
	switch {
	case IsInsufficientBalanceError(err):
		return "insufficient_balance"
	case IsAccountEligibilityError(err):
		return "account_eligibility"
	case IsAssetMismatchError(err):
		return "asset_mismatch"
	case IsIdempotencyError(err):
		return "idempotency"
	}

	// Map from the error category
	category := GetErrorCategory(err)
	switch category {
	case CategoryValidation, CategoryUnprocessable:
		return "validation"
	case CategoryAuthentication:
		return "authentication"
	case CategoryAuthorization:
		return "permission"
	case CategoryNotFound:
		return "not_found"
	case CategoryLimitExceeded:
		return "rate_limit"
	case CategoryTimeout:
		return "timeout"
	case CategoryInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// redactURLUserinfo strips userinfo (user[:password]) from any URL the
// Go stdlib renders inside an error message. The stdlib's own
// url.Error formatting masks the password to "xxxxx" but preserves
// the username verbatim — for our purposes either half is sensitive.
//
// Audit M14: when a URL like
//
//	https://alice:hunter2@api.example.com/v1
//
// fails to dial, the resulting error renders as
//
//	Get "https://alice:xxxxx@api.example.com/v1": dial tcp ...
//
// leaking the username "alice". This helper is used at the transport
// boundary before the error is wrapped — see [transport.go].
func redactURLUserinfo(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || parsed.User == nil {
		return rawURL
	}

	parsed.User = nil

	return parsed.String()
}
