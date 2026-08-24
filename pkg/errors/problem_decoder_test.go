// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorDecoder covers the decode of BOTH live server error shapes into
// *Error — the RFC 9457 problem+json envelope (/v2 + Tracer) and the legacy /v1
// shape (code/message/title/entityType/fields) — including field mapping,
// retryability keyed on Status, and the Code suffix override the status cannot
// express.
func TestErrorDecoder(t *testing.T) {
	tests := []struct {
		name           string
		httpStatus     int
		body           string
		wantCategory   ErrorCategory
		wantAPICode    string
		wantTitle      string
		wantMessage    string
		wantEntityType string
		wantStatus     int
		wantRetryable  bool
		wantFields     []string
		checkDetails   func(t *testing.T, e *Error)
	}{
		{
			name:       "422 unprocessable without errors[] is non-retryable",
			httpStatus: http.StatusUnprocessableEntity,
			body: `{
				"code":"LEDGER-0042",
				"title":"Unprocessable",
				"detail":"business rule violated",
				"status":422
			}`,
			wantCategory:  CategoryUnprocessable,
			wantAPICode:   "LEDGER-0042",
			wantTitle:     "Unprocessable",
			wantMessage:   "business rule violated",
			wantStatus:    422,
			wantRetryable: false,
		},
		{
			name:       "503 service unavailable is retryable by status",
			httpStatus: http.StatusServiceUnavailable,
			body: `{
				"code":"LEDGER-9999",
				"title":"Service Unavailable",
				"detail":"upstream is down",
				"status":503
			}`,
			wantCategory:  CategoryNetwork,
			wantAPICode:   "LEDGER-9999",
			wantTitle:     "Service Unavailable",
			wantStatus:    503,
			wantRetryable: true,
		},
		{
			name:       "errors[] maps to field-errors",
			httpStatus: http.StatusBadRequest,
			body: `{
				"code":"LEDGER-0001",
				"title":"Validation failed",
				"detail":"invalid payload",
				"status":400,
				"errors":[
					{"location":"body.name","message":"is required"},
					{"location":"path.id","message":"malformed"}
				]
			}`,
			wantCategory:  CategoryValidation,
			wantAPICode:   "LEDGER-0001",
			wantTitle:     "Validation failed",
			wantStatus:    400,
			wantRetryable: false,
			wantFields:    []string{"body.name", "path.id"},
			checkDetails: func(t *testing.T, e *Error) {
				t.Helper()
				fieldErrs, ok := e.Details["errors"].([]any)
				require.True(t, ok, "Details[errors] should be a slice")
				require.Len(t, fieldErrs, 2)
			},
		},
		{
			name:       "code suffix 0178 overrides to retryable despite non-retryable status",
			httpStatus: http.StatusUnprocessableEntity,
			body: `{
				"code":"TRACER-0178",
				"title":"Temporarily unavailable",
				"detail":"try again",
				"status":422
			}`,
			wantCategory:  CategoryNetwork,
			wantAPICode:   "TRACER-0178",
			wantStatus:    422,
			wantRetryable: true,
		},
		{
			name:       "code suffix 0177 overrides to non-retryable despite retryable status",
			httpStatus: http.StatusServiceUnavailable,
			body: `{
				"code":"LEDGER-0177",
				"title":"Denied",
				"detail":"denied",
				"status":503
			}`,
			wantCategory:  CategoryUnprocessable,
			wantAPICode:   "LEDGER-0177",
			wantStatus:    503,
			wantRetryable: false,
		},
		{
			name: "envelope status wins over divergent transport status for category and retryability",
			// Transport observed 200 (a lie / proxy rewrite); the envelope
			// declares 503. Category and retryability must derive from the
			// envelope's 503 (CategoryNetwork, retryable), not the transport 200.
			httpStatus: http.StatusOK,
			body: `{
				"code":"LEDGER-9999",
				"title":"Service Unavailable",
				"detail":"upstream is down",
				"status":503
			}`,
			wantCategory:  CategoryNetwork,
			wantAPICode:   "LEDGER-9999",
			wantStatus:    503,
			wantRetryable: true,
		},
		{
			name:       "prefixed idempotency code 0084 maps to CodeIdempotency, conflict stays non-retryable",
			httpStatus: http.StatusConflict,
			body: `{
				"code":"LEDGER-0084",
				"title":"Idempotency conflict",
				"detail":"duplicate request",
				"status":409
			}`,
			wantCategory:  CategoryConflict,
			wantAPICode:   "LEDGER-0084",
			wantStatus:    409,
			wantRetryable: false,
			checkDetails: func(t *testing.T, e *Error) {
				t.Helper()
				assert.Equal(t, CodeIdempotency, e.Code, "prefixed 0084 must classify as idempotency")
			},
		},
		{
			name:          "empty body falls back to http status",
			httpStatus:    http.StatusServiceUnavailable,
			body:          ``,
			wantCategory:  CategoryNetwork,
			wantStatus:    503,
			wantRetryable: true,
		},
		{
			name:          "non-json body falls back to http status",
			httpStatus:    http.StatusUnprocessableEntity,
			body:          `<html>gateway error</html>`,
			wantCategory:  CategoryUnprocessable,
			wantStatus:    422,
			wantRetryable: false,
		},
		{
			// The whole /v1 surface emits this shape (application/json, LegacyError).
			// Before it was decoded, every /v1 error read "API error with status
			// code 400" and dropped the per-field detail entirely.
			name:       "v1 legacy body maps message, entityType and fields",
			httpStatus: http.StatusBadRequest,
			body: `{
					"code":"0065",
					"title":"Invalid Path Parameter",
					"message":"the account id is not a valid UUID",
					"entityType":"Account",
					"fields":{"name":"is required","chartOfAccounts":"unexpected field"}
				}`,
			wantCategory:   CategoryValidation,
			wantAPICode:    "0065",
			wantTitle:      "Invalid Path Parameter",
			wantMessage:    "the account id is not a valid UUID",
			wantEntityType: "Account",
			wantStatus:     400,
			wantRetryable:  false,
			// Sorted, so the slice is deterministic across map iterations.
			wantFields: []string{"chartOfAccounts", "name"},
			checkDetails: func(t *testing.T, e *Error) {
				t.Helper()
				legacyFields, ok := e.Details["fields"].(map[string]any)
				require.True(t, ok, "Details[fields] should carry the raw legacy map")
				assert.Equal(t, "is required", legacyFields["name"])
			},
		},
		{
			name:       "v1 legacy body without fields still carries the message",
			httpStatus: http.StatusConflict,
			body: `{
					"code":"0072",
					"title":"Duplicate Ledger",
					"message":"a ledger with this name already exists"
				}`,
			wantCategory:  CategoryConflict,
			wantAPICode:   "0072",
			wantTitle:     "Duplicate Ledger",
			wantMessage:   "a ledger with this name already exists",
			wantStatus:    409,
			wantRetryable: false,
			checkDetails: func(t *testing.T, e *Error) {
				t.Helper()
				assert.Nil(t, e.Fields, "no fields object means no field-errors")
			},
		},
		{
			// A body carrying both members reads as RFC 9457: "detail" and
			// "errors" win so a server mid-migration is never downgraded.
			name:       "rfc 9457 members win over the legacy ones",
			httpStatus: http.StatusBadRequest,
			body: `{
					"code":"LEDGER-0001",
					"title":"Validation failed",
					"detail":"rfc detail",
					"message":"legacy message",
					"errors":[{"location":"body.name","message":"is required"}],
					"fields":{"legacyOnly":"ignored"}
				}`,
			wantCategory:  CategoryValidation,
			wantAPICode:   "LEDGER-0001",
			wantTitle:     "Validation failed",
			wantMessage:   "rfc detail",
			wantStatus:    400,
			wantRetryable: false,
			wantFields:    []string{"body.name"},
			checkDetails: func(t *testing.T, e *Error) {
				t.Helper()
				assert.NotContains(t, e.Details, "fields", "legacy fields must not shadow errors[]")
			},
		},
		{
			name:          "legacy body with an empty message falls back to http status",
			httpStatus:    http.StatusInternalServerError,
			body:          `{"code":"0500","title":"Internal","message":""}`,
			wantCategory:  CategoryInternal,
			wantAPICode:   "0500",
			wantMessage:   "API error with status code 500",
			wantStatus:    500,
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeProblemJSON(tt.httpStatus, []byte(tt.body), "req-test")

			var sdkErr *Error
			require.ErrorAs(t, decoded, &sdkErr)

			assert.Equal(t, tt.wantCategory, sdkErr.Category, "category")
			assert.Equal(t, tt.wantStatus, sdkErr.StatusCode, "status code")
			assert.Equal(t, tt.wantRetryable, sdkErr.Retryable(), "retryable")

			if tt.wantAPICode != "" {
				assert.Equal(t, tt.wantAPICode, sdkErr.APICode, "api code")
			}

			if tt.wantTitle != "" {
				assert.Equal(t, tt.wantTitle, sdkErr.Title, "title")
			}

			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, sdkErr.Message, "message")
			}

			if tt.wantEntityType != "" {
				assert.Equal(t, tt.wantEntityType, sdkErr.EntityType, "entity type")
			}

			if tt.wantFields != nil {
				assert.Equal(t, tt.wantFields, sdkErr.Fields, "fields")
			}

			if tt.checkDetails != nil {
				tt.checkDetails(t, sdkErr)
			}
		})
	}
}
