// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorDecoder covers the unified RFC 9457 problem+json envelope decode
// into *Error: field mapping, retryability keyed on Status, and the Code
// suffix override the status cannot express.
func TestErrorDecoder(t *testing.T) {
	tests := []struct {
		name          string
		httpStatus    int
		body          string
		wantCategory  ErrorCategory
		wantAPICode   string
		wantTitle     string
		wantStatus    int
		wantRetryable bool
		wantFields    []string
		checkDetails  func(t *testing.T, e *Error)
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
			name:       "prefixed idempotency code 0084 maps to CodeIdempotency, conflict stays retryable",
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

			if tt.wantFields != nil {
				assert.Equal(t, tt.wantFields, sdkErr.Fields, "fields")
			}

			if tt.checkDetails != nil {
				tt.checkDetails(t, sdkErr)
			}
		})
	}
}
