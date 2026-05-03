package entities

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
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
	switch errorClass {
	// Client-side / business-rule failures are noisy WARN events; everything
	// else (network, internal, sdk_error fallback) is ERROR. We deliberately
	// default to ERROR — when classification is uncertain we'd rather over-
	// report than silence a real fault.
	case "validation", "missing_parameter", "unauthorized", "forbidden", "not_found":
		return observability.WarnLevel
	default:
		return observability.ErrorLevel
	}
}

// filterBusinessFields drops any key not listed in safeBusinessFields and
// any value with an unexpected dynamic type. The numeric type set is
// deliberately conservative — we accept the common Go integer widths plus
// float64 (json.Unmarshal default) and bool. Anything else is dropped to
// avoid leaking large object graphs into log/event attributes.
func filterBusinessFields(fields map[string]any) map[string]any {
	filtered := map[string]any{}

	for key, value := range fields {
		if _, ok := safeBusinessFields[key]; !ok || value == nil {
			continue
		}

		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				filtered[key] = v
			}
		case fmt.Stringer:
			if text := strings.TrimSpace(v.String()); text != "" {
				filtered[key] = text
			}
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64, bool:
			filtered[key] = v
		}
	}

	return filtered
}

func businessAttributes(fields map[string]any) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))

	for key, value := range fields {
		attrKey := "midaz.business." + key

		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String(attrKey, v))
		case bool:
			attrs = append(attrs, attribute.Bool(attrKey, v))
		case int:
			attrs = append(attrs, attribute.Int(attrKey, v))
		case int8:
			attrs = append(attrs, attribute.Int(attrKey, int(v)))
		case int16:
			attrs = append(attrs, attribute.Int(attrKey, int(v)))
		case int32:
			attrs = append(attrs, attribute.Int64(attrKey, int64(v)))
		case int64:
			attrs = append(attrs, attribute.Int64(attrKey, v))
		case uint:
			// On 64-bit platforms uint can exceed int64 max. Saturate to
			// avoid an undefined wrap; gosec G115 enforces this for us.
			if v > math.MaxInt64 {
				attrs = append(attrs, attribute.Int64(attrKey, math.MaxInt64))
			} else {
				attrs = append(attrs, attribute.Int64(attrKey, int64(v))) //nolint:gosec // bounded above
			}
		case uint8:
			attrs = append(attrs, attribute.Int(attrKey, int(v)))
		case uint16:
			attrs = append(attrs, attribute.Int(attrKey, int(v)))
		case uint32:
			attrs = append(attrs, attribute.Int64(attrKey, int64(v)))
		case uint64:
			if v > math.MaxInt64 {
				attrs = append(attrs, attribute.Int64(attrKey, math.MaxInt64))
			} else {
				attrs = append(attrs, attribute.Int64(attrKey, int64(v))) //nolint:gosec // bounded above
			}
		case float32:
			attrs = append(attrs, attribute.Float64(attrKey, float64(v)))
		case float64:
			attrs = append(attrs, attribute.Float64(attrKey, v))
		}
	}

	return attrs
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
//  1. Typed *sdkerrors.Error / *sdkerrors.MidazError — the SDK already knows
//     the precise category and HTTP status of these.
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

	// 1) Typed SDK error with category + status code.
	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		if class := classifyByStatusCode(sdkErr.StatusCode); class != "" {
			return class
		}

		if class := classifyByCategory(sdkErr.Category, sdkErr.Code); class != "" {
			return class
		}
	}

	// 1b) Legacy *MidazError shape — code-only, no status.
	var legacy *sdkerrors.MidazError
	if errors.As(err, &legacy) && legacy != nil {
		if class := classifyByCategory("", legacy.Code); class != "" {
			return class
		}
	}

	// 2) Structural interface — anything that can self-report a status code.
	type statusCoder interface{ StatusCode() int }
	type httpStatusCoder interface{ HTTPStatus() int }

	var sc statusCoder
	if errors.As(err, &sc) {
		if class := classifyByStatusCode(sc.StatusCode()); class != "" {
			return class
		}
	}

	var hsc httpStatusCoder
	if errors.As(err, &hsc) {
		if class := classifyByStatusCode(hsc.HTTPStatus()); class != "" {
			return class
		}
	}

	// 3) Last-resort substring matching against precomputed status strings.
	// We deliberately do NOT match on the bare word "validation" or
	// "missing" anymore — those are too noisy and routinely misclassified
	// unrelated errors. Status-code digits are a more reliable signal.
	text := err.Error()
	switch {
	case strings.Contains(text, statusUnauthorizedStr):
		return "unauthorized"
	case strings.Contains(text, statusForbiddenStr):
		return "forbidden"
	case strings.Contains(text, statusNotFoundStr):
		return "not_found"
	case strings.Contains(text, statusUnprocessable):
		return "validation"
	default:
		return "sdk_error"
	}
}

// classifyByStatusCode maps an HTTP status code into the small set of
// business-error labels. It returns "" when the status is zero or unknown
// so callers can fall through to other classifiers.
func classifyByStatusCode(code int) string {
	switch code {
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 400, 422:
		return "validation"
	default:
		return ""
	}
}

// classifyByCategory translates a typed SDK category/code pair into a
// business-error label. Category wins; we fall back to Code only when the
// category is empty (e.g. legacy *MidazError, which carries only a code).
func classifyByCategory(category sdkerrors.ErrorCategory, code sdkerrors.ErrorCode) string {
	switch category {
	case sdkerrors.CategoryValidation, sdkerrors.CategoryUnprocessable:
		return "validation"
	case sdkerrors.CategoryAuthentication:
		return "unauthorized"
	case sdkerrors.CategoryAuthorization:
		return "forbidden"
	case sdkerrors.CategoryNotFound:
		return "not_found"
	}

	switch code {
	case sdkerrors.CodeValidation, sdkerrors.CodeUnprocessable:
		return "validation"
	case sdkerrors.CodeAuthentication:
		return "unauthorized"
	case sdkerrors.CodePermission:
		return "forbidden"
	case sdkerrors.CodeNotFound:
		return "not_found"
	}

	return ""
}
