package entities

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Pre-computed status-code strings used for substring fallback in
// classifyBusinessError. We intentionally avoid fmt.Sprintf("%d", ...) on
// every classify call — these are constants and the previous implementation
// allocated for every emitted business error.
const (
	statusUnauthorizedStr = "401"
	statusForbiddenStr    = "403"
	statusNotFoundStr     = "404"
	statusUnprocessable   = "422"
)

const (
	businessErrorClassValidation       = "validation"
	businessErrorClassMissingParameter = "missing_parameter"
	businessErrorClassUnauthorized     = "unauthorized"
	businessErrorClassForbidden        = "forbidden"
	businessErrorClassNotFound         = "not_found"
	businessErrorClassSDK              = "sdk_error"
)

const (
	businessEventOrganizationCreated  = "midaz.organization.created"
	businessEventOrganizationUpdated  = "midaz.organization.updated"
	businessEventOrganizationDeleted  = "midaz.organization.deleted"
	businessEventLedgerCreated        = "midaz.ledger.created"
	businessEventLedgerUpdated        = "midaz.ledger.updated"
	businessEventLedgerDeleted        = "midaz.ledger.deleted"
	businessEventAssetCreated         = "midaz.asset.created"
	businessEventAssetUpdated         = "midaz.asset.updated"
	businessEventAssetDeleted         = "midaz.asset.deleted"
	businessEventAccountCreated       = "midaz.account.created"
	businessEventAccountUpdated       = "midaz.account.updated"
	businessEventAccountDeleted       = "midaz.account.deleted"
	businessEventTransactionCreated   = "midaz.transaction.created"
	businessEventTransactionUpdated   = "midaz.transaction.updated"
	businessEventTransactionCommitted = "midaz.transaction.committed"
	businessEventTransactionReverted  = "midaz.transaction.reverted"
	businessEventTransactionCancelled = "midaz.transaction.cancelled"
	businessEventOperationUpdated     = "midaz.operation.updated"
)

var safeBusinessFields = map[string]struct{}{
	"event":              {},
	"operation":          {},
	"organizationId":     {},
	"ledgerId":           {},
	"assetId":            {},
	"accountId":          {},
	"transactionId":      {},
	"operationId":        {},
	"portfolioId":        {},
	"segmentId":          {},
	"balanceId":          {},
	"holderId":           {},
	"aliasId":            {},
	"routeId":            {},
	"transactionRouteId": {},
	"operationRouteId":   {},
	"status":             {},
	"errorClass":         {},
	"httpStatus":         {},
}

var businessErrorLevels = map[string]observability.LogLevel{
	businessErrorClassValidation:       observability.WarnLevel,
	businessErrorClassMissingParameter: observability.WarnLevel,
	businessErrorClassUnauthorized:     observability.WarnLevel,
	businessErrorClassForbidden:        observability.WarnLevel,
	businessErrorClassNotFound:         observability.WarnLevel,
}

type businessStatusCoder interface{ StatusCode() int }

type businessHTTPStatusCoder interface{ HTTPStatus() int }

func (c *HTTPClient) emitBusinessEvent(ctx context.Context, event string, fields map[string]any) {
	c.emitBusiness(ctx, observability.InfoLevel, event, fields)
}

func (c *HTTPClient) emitBusinessError(ctx context.Context, event string, fields map[string]any, err error) {
	if err != nil {
		fields = cloneBusinessFields(fields)
		fields["errorClass"] = classifyBusinessError(err)
	}

	c.emitBusiness(ctx, businessLogLevelForError(fields["errorClass"]), event, fields)
}

func (c *HTTPClient) emitBusiness(ctx context.Context, level observability.LogLevel, event string, fields map[string]any) {
	snapshot := c.cloneConfiguration()
	if snapshot.observability == nil || !snapshot.observability.IsEnabled() {
		return
	}

	safeFields := filterBusinessFields(fields)
	safeFields["event"] = event

	span := trace.SpanFromContext(ctx)

	attrs := businessAttributes(safeFields)
	if span.IsRecording() {
		span.AddEvent(event, trace.WithAttributes(attrs...))
	}

	logger := snapshot.observability.Logger()
	if logger == nil {
		return
	}

	logger = logger.WithSpan(span).With(safeFields)

	switch level {
	case observability.ErrorLevel:
		logger.Error(event)
	case observability.WarnLevel:
		logger.Warn(event)
	default:
		logger.Info(event)
	}
}

func businessLogLevelForError(errorClass any) observability.LogLevel {
	label, ok := errorClass.(string)
	if !ok {
		return observability.ErrorLevel
	}

	if level, ok := businessErrorLevels[label]; ok {
		return level
	}

	return observability.ErrorLevel
}

// filterBusinessFields drops any key not listed in safeBusinessFields and
// any value with an unexpected dynamic type. The numeric type set is
// deliberately conservative — we accept the common Go integer widths plus
// float64 (json.Unmarshal default) and bool. Anything else is dropped to
// avoid leaking large object graphs into log/event attributes.
func filterBusinessFields(fields map[string]any) map[string]any {
	filtered := map[string]any{}

	for key, value := range fields {
		if _, ok := safeBusinessFields[key]; !ok {
			continue
		}

		normalized, ok := normalizeBusinessField(value)
		if !ok {
			continue
		}

		filtered[key] = normalized
	}

	return filtered
}

func businessAttributes(fields map[string]any) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))

	for key, value := range fields {
		attr, ok := businessAttribute(key, value)
		if !ok {
			continue
		}

		attrs = append(attrs, attr)
	}

	return attrs
}

func normalizeBusinessField(value any) (any, bool) {
	if value == nil {
		return nil, false
	}

	switch v := value.(type) {
	case string:
		return nonBlankBusinessString(v)
	case fmt.Stringer:
		return nonBlankBusinessString(v.String())
	case bool:
		return v, true
	}

	return normalizeBusinessNumber(value)
}

func nonBlankBusinessString(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", false
	}

	return value, true
}

func normalizeBusinessNumber(value any) (any, bool) {
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		unsigned := reflected.Uint()
		if unsigned > math.MaxInt64 {
			return int64(math.MaxInt64), true
		}

		return int64(unsigned), true
	case reflect.Float32, reflect.Float64:
		return reflected.Float(), true
	default:
		return nil, false
	}
}

func businessAttribute(key string, value any) (attribute.KeyValue, bool) {
	normalized, ok := normalizeBusinessField(value)
	if !ok {
		return attribute.KeyValue{}, false
	}

	attrKey := "midaz.business." + key

	switch v := normalized.(type) {
	case string:
		return attribute.String(attrKey, v), true
	case bool:
		return attribute.Bool(attrKey, v), true
	case int64:
		return attribute.Int64(attrKey, v), true
	case float64:
		return attribute.Float64(attrKey, v), true
	default:
		return attribute.KeyValue{}, false
	}
}

func cloneBusinessFields(fields map[string]any) map[string]any {
	clone := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		clone[key] = value
	}

	return clone
}

// classifyBusinessError converts an error into a stable, low-cardinality
// string label used for log/metric attributes and for picking the correct
// observability level. Classification proceeds in three layers, from most-
// trustworthy to least:
//
//  1. Typed *sdkerrors.Error — the SDK already knows the precise category
//     and HTTP status of these.
//  2. Anything that exposes a status code via a small structural interface
//     (StatusCode() int / HTTPStatus() int).
//  3. A small set of substring fallbacks for opaque transport errors that
//     stringify the HTTP status. Substring matching is kept ONLY as a last
//     resort — typed paths win when present.
//
// When nothing matches we return "sdk_error" and the caller logs at ERROR
// level (not WARN), so unclassified failures stay loud.
func classifyBusinessError(err error) string {
	if err == nil {
		return ""
	}

	if class := classifyTypedBusinessError(err); class != "" {
		return class
	}

	if class := classifyStructuralBusinessError(err); class != "" {
		return class
	}

	return classifyBusinessErrorText(err.Error())
}

func classifyTypedBusinessError(err error) string {
	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		if class := classifyByStatusCode(sdkErr.StatusCode); class != "" {
			return class
		}

		if class := classifyByCategory(sdkErr.Category, sdkErr.Code); class != "" {
			return class
		}
	}

	return ""
}

func classifyStructuralBusinessError(err error) string {
	var sc businessStatusCoder
	if errors.As(err, &sc) {
		if class := classifyByStatusCode(sc.StatusCode()); class != "" {
			return class
		}
	}

	var hsc businessHTTPStatusCoder
	if errors.As(err, &hsc) {
		if class := classifyByStatusCode(hsc.HTTPStatus()); class != "" {
			return class
		}
	}

	return ""
}

func classifyBusinessErrorText(text string) string {
	switch {
	case strings.Contains(text, statusUnauthorizedStr):
		return businessErrorClassUnauthorized
	case strings.Contains(text, statusForbiddenStr):
		return businessErrorClassForbidden
	case strings.Contains(text, statusNotFoundStr):
		return businessErrorClassNotFound
	case strings.Contains(text, statusUnprocessable):
		return businessErrorClassValidation
	default:
		return businessErrorClassSDK
	}
}

// classifyByStatusCode maps an HTTP status code into the small set of
// business-error labels. It returns "" when the status is zero or unknown
// so callers can fall through to other classifiers.
func classifyByStatusCode(code int) string {
	switch code {
	case 401:
		return businessErrorClassUnauthorized
	case 403:
		return businessErrorClassForbidden
	case 404:
		return businessErrorClassNotFound
	case 400, 422:
		return businessErrorClassValidation
	default:
		return ""
	}
}

// classifyByCategory translates a typed SDK category/code pair into a
// business-error label. Category wins; we fall back to Code only when
// the category is empty.
//
// CategoryAuth (the v3 collapsed auth category) splits into
// unauthorized vs forbidden via the Code (CodeAuthentication = 401,
// CodePermission = 403). The legacy CategoryAuthentication and
// CategoryAuthorization are still matched for back-compat with code
// paths that haven't migrated.
func classifyByCategory(category sdkerrors.ErrorCategory, code sdkerrors.ErrorCode) string {
	switch category {
	case sdkerrors.CategoryValidation, sdkerrors.CategoryUnprocessable:
		return businessErrorClassValidation
	case sdkerrors.CategoryAuthentication:
		return businessErrorClassUnauthorized
	case sdkerrors.CategoryAuthorization:
		return businessErrorClassForbidden
	case sdkerrors.CategoryAuth:
		// v3: discriminate via Code (401 vs 403).
		if code == sdkerrors.CodePermission {
			return businessErrorClassForbidden
		}

		return businessErrorClassUnauthorized
	case sdkerrors.CategoryNotFound:
		return businessErrorClassNotFound
	}

	switch code {
	case sdkerrors.CodeValidation, sdkerrors.CodeUnprocessable:
		return businessErrorClassValidation
	case sdkerrors.CodeAuthentication:
		return businessErrorClassUnauthorized
	case sdkerrors.CodePermission:
		return businessErrorClassForbidden
	case sdkerrors.CodeNotFound:
		return businessErrorClassNotFound
	}

	return ""
}
