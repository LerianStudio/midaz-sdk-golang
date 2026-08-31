// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/shopspring/decimal"
)

// The Tracer renamed the ISO-4217 money field from "currency" to "asset" across
// every request and response body it serves (server-side: zero json:"currency"
// tags remain). The SDK kept sending and reading "currency", so:
//
//   - every limit create, transaction validation and reservation was REJECTED by
//     the server (asset is `validate:"required"` there, so the omitted field is a
//     400, not a tolerated extra key);
//   - every limit / validation read silently decoded an EMPTY money asset.
//
// These tests assert the wire name in both directions. They are the regression
// guard for the rename: a revert to "currency" fails them on the request leg
// (body assertion) and on the response leg (decode assertion).

const wireAsset = "USD"

// assertAssetOnlyBody fails when the captured request body carries the retired
// "currency" key, or is missing the live "asset" key.
func assertAssetOnlyBody(t *testing.T, what, body string) {
	t.Helper()

	if !strings.Contains(body, `"asset":"`+wireAsset+`"`) {
		t.Fatalf("%s body = %s, want the live wire key \"asset\":%q", what, body, wireAsset)
	}

	if strings.Contains(body, `"currency"`) {
		t.Fatalf("%s body = %s, must not carry the retired wire key \"currency\"", what, body)
	}
}

// captureBody runs fn against a server that records the request body and answers
// with status + respBody.
func captureBody(t *testing.T, status int, respBody string, fn func(*httptest.Server)) string {
	t.Helper()

	var captured string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()

	fn(srv)

	return captured
}

// TestTracerWireAsset_Requests proves the three write paths that carry money
// emit "asset", never "currency".
func TestTracerWireAsset_Requests(t *testing.T) {
	tests := []struct {
		name   string
		status int
		resp   string
		call   func(*testing.T, *httptest.Server)
	}{
		{
			name:   "limits create",
			status: http.StatusCreated,
			resp:   limitAssetJSON(limitID, "DRAFT", moneyPrecise),
			call: func(t *testing.T, srv *httptest.Server) {
				t.Helper()

				if _, err := newTestLimitsFacade(t, srv).Create(context.Background(), newCreateLimit()); err != nil {
					t.Fatalf("Limits.Create: %v", err)
				}
			},
		},
		{
			name:   "validations evaluate",
			status: http.StatusCreated,
			resp:   `{"validationId":"11111111-1111-1111-1111-111111111111","decision":"ALLOW","reason":"ok","requestId":"22222222-2222-2222-2222-222222222222","evaluatedAt":"2026-01-01T00:00:00Z","processingTimeMs":1,"totalRulesLoaded":0,"truncated":false}`,
			call: func(t *testing.T, srv *httptest.Server) {
				t.Helper()

				input := models.NewValidateTransactionInput(
					"22222222-2222-2222-2222-222222222222",
					decimal.RequireFromString("10.00"),
					wireAsset,
					"2026-01-01T00:00:00Z",
					models.AccountContext{AccountID: "33333333-3333-3333-3333-333333333333"},
				)
				if _, err := newTestValidationsFacade(t, srv).Evaluate(context.Background(), input); err != nil {
					t.Fatalf("Validations.Evaluate: %v", err)
				}
			},
		},
		{
			name:   "reservations reserve",
			status: http.StatusCreated,
			resp:   `{"reservationId":"44444444-4444-4444-4444-444444444444","decision":"ALLOW","reason":"ok","expiresAt":"2026-01-01T00:05:00Z"}`,
			call: func(t *testing.T, srv *httptest.Server) {
				t.Helper()

				input := models.NewReserveInput(
					"55555555-5555-5555-5555-555555555555",
					"22222222-2222-2222-2222-222222222222",
					decimal.RequireFromString("10.00"),
					wireAsset,
					"2026-01-01T00:00:00Z",
				)
				if _, err := newTestReservationsFacade(t, srv).Reserve(context.Background(), input); err != nil {
					t.Fatalf("Reservations.Reserve: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := captureBody(t, tt.status, tt.resp, func(srv *httptest.Server) {
				tt.call(t, srv)
			})
			assertAssetOnlyBody(t, tt.name, body)
		})
	}
}

// TestTracerWireAsset_Responses proves the three read paths decode "asset" into
// the public Asset field. A server body carrying only "asset" must not leave the
// money asset empty.
func TestTracerWireAsset_Responses(t *testing.T) {
	t.Run("limit", func(t *testing.T) {
		srv := jsonServer(t, http.StatusOK, limitAssetJSON(limitID, "ACTIVE", moneyPrecise))

		got, err := newTestLimitsFacade(t, srv).Get(context.Background(), limitID)
		if err != nil {
			t.Fatalf("Limits.Get: %v", err)
		}

		if got.Asset != wireAsset {
			t.Fatalf("Limit.Asset = %q, want %q (decoded from the \"asset\" wire key)", got.Asset, wireAsset)
		}
	})

	t.Run("transaction validation", func(t *testing.T) {
		body := `{"validationId":"11111111-1111-1111-1111-111111111111","requestId":"22222222-2222-2222-2222-222222222222",` +
			`"decision":"ALLOW","reason":"ok","amount":"10.00","asset":"` + wireAsset + `","transactionType":"PIX",` +
			`"account":{"accountId":"33333333-3333-3333-3333-333333333333"},"processingTimeMs":1,` +
			`"totalRulesLoaded":0,"truncated":false,"transactionTimestamp":"2026-01-01T00:00:00Z","createdAt":"2026-01-01T00:00:00Z"}`
		srv := jsonServer(t, http.StatusOK, body)

		got, err := newTestValidationsFacade(t, srv).Get(context.Background(), "11111111-1111-1111-1111-111111111111")
		if err != nil {
			t.Fatalf("Validations.Get: %v", err)
		}

		if got.Asset != wireAsset {
			t.Fatalf("TransactionValidation.Asset = %q, want %q", got.Asset, wireAsset)
		}
	})

	t.Run("validation summary", func(t *testing.T) {
		body := `{"transactionValidations":[{"validationId":"11111111-1111-1111-1111-111111111111",` +
			`"accountId":"33333333-3333-3333-3333-333333333333","amount":"10.00","asset":"` + wireAsset + `",` +
			`"decision":"ALLOW","reason":"ok","transactionType":"PIX","processingTimeMs":1,` +
			`"createdAt":"2026-01-01T00:00:00Z"}],"hasMore":false,"nextCursor":""}`
		srv := jsonServer(t, http.StatusOK, body)

		page, err := newTestValidationsFacade(t, srv).List(context.Background(), models.ValidationsListOpts{})
		if err != nil {
			t.Fatalf("Validations.List: %v", err)
		}

		if len(page.Items) != 1 {
			t.Fatalf("List items = %d, want 1", len(page.Items))
		}

		if page.Items[0].Asset != wireAsset {
			t.Fatalf("ValidationSummary.Asset = %q, want %q", page.Items[0].Asset, wireAsset)
		}
	})
}

// jsonServer answers every request with status + body and closes with the test.
func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
}

// limitAssetJSON is limitJSON's asset-named twin: the body the server actually
// serves now.
func limitAssetJSON(id, status, maxAmount string) string {
	return `{"limitId":"` + id + `","name":"daily-cap","limitType":"DAILY",` +
		`"maxAmount":"` + maxAmount + `","asset":"` + wireAsset + `","status":"` + status + `",` +
		`"scopes":[{"transactionType":"` + limitScopeTxn + `"}],` +
		`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
}
