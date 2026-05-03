package entities

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	case "validation", "missing_parameter", "unauthorized", "forbidden", "not_found":
		return observability.WarnLevel
	default:
		return observability.ErrorLevel
	}
}

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
		case int, int64, bool:
			filtered[key] = v
		}
	}

	return filtered
}

func businessAttributes(fields map[string]any) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(fields))
	for key, value := range fields {
		switch v := value.(type) {
		case string:
			attrs = append(attrs, attribute.String("midaz.business."+key, v))
		case int:
			attrs = append(attrs, attribute.Int("midaz.business."+key, v))
		case int64:
			attrs = append(attrs, attribute.Int64("midaz.business."+key, v))
		case bool:
			attrs = append(attrs, attribute.Bool("midaz.business."+key, v))
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

func classifyBusinessError(err error) string {
	if err == nil {
		return ""
	}

	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "validation"):
		return "validation"
	case strings.Contains(text, "missing"):
		return "missing_parameter"
	case strings.Contains(text, fmt.Sprintf("%d", http.StatusUnauthorized)):
		return "unauthorized"
	case strings.Contains(text, fmt.Sprintf("%d", http.StatusForbidden)):
		return "forbidden"
	case strings.Contains(text, fmt.Sprintf("%d", http.StatusNotFound)):
		return "not_found"
	default:
		return "sdk_error"
	}
}
