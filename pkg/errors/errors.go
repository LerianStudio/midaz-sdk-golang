// (Package documentation lives in doc.go.)
package errors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

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

	// CodeServiceUnavailable indicates a 503 Service Unavailable from a
	// dependency. Distinct from CodeNetwork (pre-response transport
	// failure) — the server answered, it just isn't ready.
	CodeServiceUnavailable ErrorCode = "service_unavailable"
)

// ErrorCategory represents the general category of an error
type ErrorCategory string

const statusClientClosedRequest = 499

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

	// CategoryNetwork represents network-related errors
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

	// CategoryAuth is the canonical v3 category for any authentication or
	// authorization failure. It replaces the v2 split between
	// CategoryAuthentication and CategoryAuthorization. Callers that need
	// to distinguish 401 from 403 should inspect [Error.StatusCode] or
	// [Error.Code] (CodeAuthentication vs CodePermission).
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
// Code, StatusCode), or call [Error.Error] which always redacts.
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

	// StatusCode is the HTTP status code, if applicable
	StatusCode int `json:"statusCode,omitempty"`

	// RequestID is the API request ID, if available.
	// Marked json:"-" defensively: request IDs are generally opaque, but
	// the safe baseline is to opt out of JSON exposure for everything
	// caller-derived.
	RequestID string `json:"-"`

	// Err is the underlying error.
	// Marked json:"-" because the inner error string is opaque to the
	// redactor — a wrapped *http.Error or similar can carry the full
	// request body. JSON output must rely on Error() for safety.
	Err error `json:"-"`
}

// MarshalJSON renders only the safe, non-leaky fields of an *Error.
// The unredacted Message, Title, ResourceID, RequestID, Fields, Details,
// and inner Err are intentionally excluded. Callers who need the full
// rendered string should use [Error.Error] (which is redacted).
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
		Category   ErrorCategory `json:"category"`
		Code       ErrorCode     `json:"code"`
		APICode    string        `json:"apiCode,omitempty"`
		Resource   string        `json:"resource,omitempty"`
		EntityType string        `json:"entityType,omitempty"`
		StatusCode int           `json:"statusCode,omitempty"`
	}

	projection := safeProjection{
		Category:   e.Category,
		Code:       e.Code,
		APICode:    e.APICode,
		Resource:   e.Resource,
		EntityType: e.EntityType,
		StatusCode: e.StatusCode,
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
	case e.Resource != "" && e.ResourceID != "":
		errorContext = fmt.Sprintf("%s error for %s %s", e.Category, e.Resource, e.ResourceID)
	case e.Resource != "":
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

	return redactSensitive(composed)
}

// redactingError wraps an inner error so that callers walking the
// chain via [errors.Unwrap] never see an unredacted string. The inner
// error remains accessible to [errors.Is] / [errors.As] via the
// embedded Unwrap, so sentinel-matching still works.
//
// Audit C5 (CRITICAL): before this wrapper landed, code that wrote
//
//	log.Printf("inner: %v", errors.Unwrap(sdkErr))
//
// would happily render `password=hunter2` from the inner error. We
// preserve the chain (so errors.Is still walks through) while
// guaranteeing every Error() call along the way passes through the
// redactor.
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

// Unwrap exposes the inner error for [errors.Is] / [errors.As] walks.
// The chain semantics survive: callers can still match sentinels and
// extract typed errors, they just can't render the inner string
// without going through Error() (which is now always redacted).
func (r *redactingError) Unwrap() error {
	if r == nil {
		return nil
	}

	return r.inner
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
	sensitiveBearerPattern = regexp.MustCompile(`(?i)\b(authorization\s*[:=]\s*(?:Bearer|Basic)\s+)[^\s,;]+`)

	// sensitiveKeyValuePattern matches "<key>=<value>" or "<key>: <value>"
	// for any sensitive-named key. Whitelist evolution (Audit C2): v3
	// extends the v2 list to cover the credential-header variants
	// (api-key, x-api-key, access_token, refresh_token, id_token, jwt)
	// so rendered error messages cannot leak common token forms.
	//
	// The leading boundary `(?:^|[^A-Za-z])` matches start-of-string
	// or any non-letter char (digit, underscore, hyphen, dot, space,
	// punctuation). This catches the keyword in compound identifiers
	// like `client_secret=` and `x-idempotency-key=` while NOT
	// triggering on operation names like `CreateMetadataIndex:`
	// where the keyword sits in the middle of a CamelCase word.
	// The trailing `[\w.-]*` allows compound suffixes like `-key`.
	sensitiveKeyValuePattern = regexp.MustCompile(`(?i)(?:^|[^A-Za-z])((?:token|password|apikey|api[-_]?key|x[-_]api[-_]?key|access[-_]?token|refresh[-_]?token|id[-_]?token|jwt|authorization|secret|(?:x[-_])?idempotency|document|legal_document|external_id|banking_details_account|banking_details_iban|metadata(?:\.[\w.-]+)?|related_party_document|regulatory_fields_participant_document)[\w.-]*)(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
)

func isNilError(err error) bool {
	if err == nil {
		return true
	}

	value := reflect.ValueOf(err)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func safeErrorString(err error) string {
	if isNilError(err) {
		return ""
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

	redacted := sensitiveBearerPattern.ReplaceAllString(message, `${1}[REDACTED]`)
	redacted = sensitiveKeyValuePattern.ReplaceAllString(redacted, `${1}${2}[REDACTED]`)

	if truncated {
		redacted += " [truncated]"
	}

	return redacted
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
		return "[REDACTED]"
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

// sensitiveFieldFragments is the canonical list of substrings (after
// case-folding and stripping '_', '-', '.') that mark a field as
// carrying sensitive data — credentials, PII, financial identifiers,
// or storage-layer metadata. Any field whose normalized name contains
// one of these fragments must be redacted before its value is rendered.
//
// The list is the single source of truth for both the API-shape
// detail walker ([RedactSensitiveDetails]) and the field-name
// predicate ([IsSensitiveFieldName]) used by the validation layer.
var sensitiveFieldFragments = []string{
	"authorization",
	"apikey",
	"accesstoken",
	"refreshtoken",
	"idtoken",
	"jwt",
	"bankingdetails",
	"cookie",
	"creditcard",
	"cardnumber",
	"cpf",
	"cnpj",
	"document",
	"externalid",
	"idempotency",
	"metadata",
	"password",
	"participantdocument",
	"relatedparty",
	"secret",
	"ssn",
	"token",
}

// IsSensitiveFieldName reports whether name (after case-folding and
// stripping underscores, hyphens, and dots) contains any fragment from
// the SDK-wide sensitive-field allowlist. Use it before rendering a
// field's value in user-facing error output so credentials, PII, and
// financial identifiers stay out of logs.
//
// Examples returning true:
//
//	"password", "X-API-Key", "authorization", "client_secret",
//	"creditCard", "cpf", "metadata.user.token", "refresh-token".
//
// The check is intentionally substring-based on the normalized name so
// near-variants (apiKey vs api_key vs X-API-Key) all trip the same
// redaction path.
func IsSensitiveFieldName(name string) bool {
	if name == "" {
		return false
	}

	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(name))
	for _, fragment := range sensitiveFieldFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}

	return false
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

// Standard error constructors

// NewValidationError creates a validation error.
func NewValidationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return &Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
}

// NewInvalidInputError creates a validation error for invalid input.
func NewInvalidInputError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "invalid input"
	if err != nil {
		message = fmt.Sprintf("invalid input: %v", err)
	}

	return &Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
}

// NewMissingParameterError creates a validation error for a missing parameter.
func NewMissingParameterError(operation, paramName string) *Error {
	message := fmt.Sprintf("missing required parameter: %s", paramName)

	return &Error{
		Category:   CategoryValidation,
		Code:       CodeValidation,
		Message:    message,
		Operation:  operation,
		Err:        errors.New(message),
		StatusCode: http.StatusBadRequest,
	}
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(operation, resource, resourceID string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("%s not found", resource)
	if resourceID != "" {
		message = fmt.Sprintf("%s not found: %s", resource, resourceID)
	}

	return &Error{
		Category:   CategoryNotFound,
		Code:       CodeNotFound,
		Message:    redactSensitive(message),
		Operation:  operation,
		Resource:   resource,
		ResourceID: resourceID,
		Err:        err,
		StatusCode: http.StatusNotFound,
	}
}

// NewAuthenticationError creates an authentication error.
func NewAuthenticationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return &Error{
		Category:   CategoryAuthentication,
		Code:       CodeAuthentication,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewAuthorizationError creates an authorization error.
func NewAuthorizationError(operation, message string, err error) *Error {
	err = normalizeError(err)
	if err != nil {
		message = fmt.Sprintf("%s: %v", message, err)
	}

	return &Error{
		Category:   CategoryAuthorization,
		Code:       CodePermission,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusForbidden,
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(operation, resource, resourceID string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("%s already exists", resource)
	if resourceID != "" {
		message = fmt.Sprintf("%s already exists: %s", resource, resourceID)
	}

	return &Error{
		Category:   CategoryConflict,
		Code:       CodeAlreadyExists,
		Message:    redactSensitive(message),
		Operation:  operation,
		Resource:   resource,
		ResourceID: resourceID,
		Err:        err,
		StatusCode: http.StatusConflict,
	}
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "rate limit exceeded"
	}

	return &Error{
		Category:   CategoryLimitExceeded,
		Code:       CodeRateLimit,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusTooManyRequests,
	}
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "operation timed out"
	}

	return &Error{
		Category:   CategoryTimeout,
		Code:       CodeTimeout,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusGatewayTimeout,
	}
}

// NewCancellationError creates a cancellation error for cancelled contexts.
func NewCancellationError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "operation cancelled"
	if err != nil {
		message = fmt.Sprintf("operation cancelled: %v", err)
	}

	return &Error{
		Category:   CategoryCancellation,
		Code:       CodeCancellation,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: statusClientClosedRequest,
	}
}

// NewNetworkError creates a network error.
func NewNetworkError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "network error"
	if err != nil {
		message = fmt.Sprintf("network error: %v", err)
	}

	return &Error{
		Category:   CategoryNetwork,
		Code:       CodeNetwork,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusServiceUnavailable,
	}
}

// NewInternalError creates an internal error.
func NewInternalError(operation string, err error) *Error {
	err = normalizeError(err)

	message := "internal error"
	if err != nil {
		message = fmt.Sprintf("internal error: %v", err)
	}

	return &Error{
		Category:   CategoryInternal,
		Code:       CodeInternal,
		Message:    redactSensitive(message),
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewConfigurationError creates a configuration error for SDK setup failures.
//
// Use this for client construction problems: missing required options,
// invalid URLs, conflicting auth sources, validation failures at New() time.
// These errors are returned eagerly by midaz.New() so users discover misuse
// at construction rather than on the first API call.
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
//     what to fix.
//   - err: An optional underlying cause. May be nil.
//
// Returns:
//   - *Error: A configuration error with Category=CategoryConfiguration and
//     Code=CodeConfiguration. errors.Is(err, ErrConfiguration) matches it.
func NewConfigurationError(operation, message string, err error) *Error {
	err = normalizeError(err)

	if message == "" {
		message = "configuration error"
	}

	return &Error{
		Category:   CategoryConfiguration,
		Code:       CodeConfiguration,
		Message:    message,
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
}

// NewUnprocessableError creates an unprocessable entity error.
func NewUnprocessableError(operation, resource string, err error) *Error {
	err = normalizeError(err)

	message := fmt.Sprintf("unprocessable %s", resource)
	if err != nil {
		message = fmt.Sprintf("unprocessable %s: %v", resource, err)
	}

	return &Error{
		Category:   CategoryUnprocessable,
		Code:       CodeUnprocessable,
		Message:    message,
		Operation:  operation,
		Resource:   resource,
		Err:        err,
		StatusCode: http.StatusUnprocessableEntity,
	}
}

// NewInsufficientBalanceError creates an insufficient balance error.
func NewInsufficientBalanceError(operation, accountID string, err error) *Error {
	err = normalizeError(err)

	message := "insufficient balance"
	if err != nil {
		message = fmt.Sprintf("insufficient balance: %v", err)
	}

	return &Error{
		Category:   CategoryUnprocessable,
		Code:       CodeInsufficientBalance,
		Message:    message,
		Operation:  operation,
		Resource:   "account",
		ResourceID: accountID,
		Err:        err,
		StatusCode: http.StatusUnprocessableEntity,
	}
}

// NewAssetMismatchError creates an asset mismatch error.
func NewAssetMismatchError(operation, expected, actual string, err error) *Error {
	err = normalizeError(err)
	message := fmt.Sprintf("asset mismatch: expected %s, got %s", expected, actual)

	return &Error{
		Category:   CategoryValidation,
		Code:       CodeAssetMismatch,
		Message:    message,
		Operation:  operation,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
}

// NewAccountEligibilityError creates an account eligibility error.
func NewAccountEligibilityError(operation, accountID string, err error) *Error {
	err = normalizeError(err)

	message := "account not eligible for this operation"
	if err != nil {
		message = fmt.Sprintf("account eligibility error: %v", err)
	}

	return &Error{
		Category:   CategoryValidation,
		Code:       CodeAccountEligibility,
		Message:    message,
		Operation:  operation,
		Resource:   "account",
		ResourceID: accountID,
		Err:        err,
		StatusCode: http.StatusBadRequest,
	}
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

// IsIdempotencyError checks if an error is an idempotency error.
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
		return sdkErr.Category == CategoryUnprocessable
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

// FormatErrorForDisplay formats an error for display to end users.
func FormatErrorForDisplay(err error) string {
	if isNilError(err) {
		return ""
	}

	var mdzErr *Error
	if errors.As(err, &mdzErr) && mdzErr != nil {
		switch mdzErr.Category {
		case CategoryValidation:
			return fmt.Sprintf("Invalid request: %s", redactSensitive(mdzErr.Message))
		case CategoryNotFound:
			return fmt.Sprintf("Resource not found: %s", redactSensitive(mdzErr.Message))
		case CategoryAuthentication:
			return "Authentication failed. Please check your credentials."
		case CategoryAuthorization:
			return "You don't have permission to perform this action."
		case CategoryConflict:
			return fmt.Sprintf("Resource conflict: %s", redactSensitive(mdzErr.Message))
		case CategoryLimitExceeded:
			return "Rate limit exceeded. Please try again later."
		case CategoryTimeout:
			return "The operation timed out. Please try again later."
		case CategoryCancellation:
			return "The operation was cancelled."
		case CategoryNetwork:
			return "Network error. Please check your connection and try again."
		case CategoryUnprocessable:
			return fmt.Sprintf("Operation could not be processed: %s", redactSensitive(mdzErr.Message))
		default:
			return "An unexpected error occurred. Please try again later."
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

func applyAPICodeMapping(mapping httpErrorMapping, apiCode string) httpErrorMapping {
	apiCode = strings.TrimSpace(apiCode)
	if apiCode == "" {
		return mapping
	}

	apiMapping, ok := apiErrorCodeMappings[apiCode]
	if !ok {
		return mapping
	}

	apiMapping.withResource = apiMapping.withResource || mapping.withResource

	return apiMapping
}

// ErrorFromHTTPResponse creates an appropriate error based on the HTTP response
func ErrorFromHTTPResponse(statusCode int, requestID, message, apiCode, entityType, resourceID string) error {
	return ErrorFromHTTPResponseWithDetails(statusCode, requestID, message, apiCode, entityType, resourceID, "", nil, nil)
}

// ErrorFromHTTPResponseWithDetails creates an appropriate error based on the HTTP response
// and preserves raw structured API envelope metadata when available.
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
	err := &Error{
		Category:   mapping.category,
		Code:       mapping.code,
		APICode:    apiCode,
		Title:      redactSensitive(title),
		Message:    redactSensitive(message),
		StatusCode: statusCode,
		RequestID:  requestID,
		EntityType: entityType,
		Fields:     RedactSensitiveStringSlice(fields),
		Details:    RedactSensitiveDetails(details),
	}

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
		return fmt.Sprintf("%s failed: %s - %s", operationType, message, redactSensitive(mdzErr.Message))
	}

	return fmt.Sprintf("%s failed: %s", operationType, redactSensitive(mdzErr.Message))
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
