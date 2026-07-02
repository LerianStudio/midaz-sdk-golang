// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const (
	billingCalcOrgID    = "11111111-1111-1111-1111-111111111111"
	billingCalcLedgerID = "22222222-2222-2222-2222-222222222222"
)

func billingCalcBase() string {
	return "/v1/organizations/" + billingCalcOrgID + "/billing/calculate"
}

func billingCalcInput() *models.BillingCalculateInput {
	return &models.BillingCalculateInput{
		LedgerID: billingCalcLedgerID,
		Period:   "2026-01",
		Type:     "volume",
	}
}

// TestBillingCalculateFacade_Calculate is the money-adjacent happy path: a 2xx
// compound response must decode both results and summary, and every net-amount
// (result + summary TotalNetAmount) must ride the wire as a string with no float
// hop. 0.333333333333333333 is unrepresentable in float64 and would drift.
func TestBillingCalculateFacade_Calculate(t *testing.T) {
	const precise = "0.333333333333333333"
	resp := `{"results":[{` +
		`"billingPackageId":"pkg-1","billingPackageLabel":"Monthly Volume Billing","billingType":"volume",` +
		`"period":"2026-01","totalAccounts":500,"totalCharged":480,"totalSkipped":20,` +
		`"totalNetAmount":"` + precise + `",` +
		`"transactionPayload":{"metadata":{"unitPrice":"0.10"}}}],` +
		`"summary":{"totalResults":1,"totalVolume":1,"totalMaintenance":0,"totalNetAmount":"` + precise + `"}}`

	var m, p, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	out, err := newTestBillingCalculateFacade(t, srv).CalculateBilling(context.Background(), billingCalcOrgID, billingCalcInput())
	if err != nil {
		t.Fatalf("CalculateBilling: %v", err)
	}
	if m != http.MethodPost || p != billingCalcBase() {
		t.Fatalf("req = %s %s, want POST %s", m, p, billingCalcBase())
	}
	if !strings.Contains(body, `"ledgerId":"`+billingCalcLedgerID+`"`) ||
		!strings.Contains(body, `"period":"2026-01"`) ||
		!strings.Contains(body, `"type":"volume"`) {
		t.Fatalf("body = %q, want ledgerId + period + type", body)
	}
	if len(out.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(out.Results))
	}
	r0 := out.Results[0]
	if r0.BillingPackageID != "pkg-1" || r0.BillingType != "volume" || r0.Period != "2026-01" {
		t.Fatalf("result = %+v", r0)
	}
	if r0.TotalAccounts != 500 || r0.TotalCharged != 480 || r0.TotalSkipped != 20 {
		t.Fatalf("result counts = %+v", r0)
	}
	// Money third rail: string, exact, no float hop.
	if r0.TotalNetAmount != precise {
		t.Fatalf("result TotalNetAmount = %q, want %q (no float hop)", r0.TotalNetAmount, precise)
	}
	if out.Summary.TotalNetAmount != precise {
		t.Fatalf("summary TotalNetAmount = %q, want %q (no float hop)", out.Summary.TotalNetAmount, precise)
	}
	if out.Summary.TotalResults != 1 || out.Summary.TotalVolume != 1 || out.Summary.TotalMaintenance != 0 {
		t.Fatalf("summary = %+v", out.Summary)
	}
	// TransactionPayload survives as raw JSON.
	var payload map[string]any
	if err := json.Unmarshal(r0.TransactionPayload, &payload); err != nil {
		t.Fatalf("TransactionPayload unmarshal: %v", err)
	}
	if _, ok := payload["metadata"]; !ok {
		t.Fatalf("TransactionPayload = %s, want metadata key", r0.TransactionPayload)
	}
}

// TestBillingCalculateFacade_Empty covers a 2xx with no matching packages: a
// null results array is a success, not an error.
func TestBillingCalculateFacade_Empty(t *testing.T) {
	resp := `{"results":null,"summary":{"totalResults":0,"totalVolume":0,"totalMaintenance":0,"totalNetAmount":"0"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	out, err := newTestBillingCalculateFacade(t, srv).CalculateBilling(context.Background(), billingCalcOrgID, billingCalcInput())
	if err != nil {
		t.Fatalf("CalculateBilling empty must NOT error: %v", err)
	}
	if len(out.Results) != 0 {
		t.Fatalf("Results = %+v, want empty", out.Results)
	}
	if out.Summary.TotalResults != 0 || out.Summary.TotalNetAmount != "0" {
		t.Fatalf("summary = %+v", out.Summary)
	}
}

// TestBillingCalculateFacade_EmptyTypeOmitted proves the both-types calculation
// path is reached correctly: with Type="" the marshaled body must OMIT the "type"
// key (json:"type,omitempty"), so the server calculates both volume and
// maintenance packages instead of an empty-string type filter matching nothing.
func TestBillingCalculateFacade_EmptyTypeOmitted(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":null,"summary":{"totalResults":0,"totalVolume":0,"totalMaintenance":0,"totalNetAmount":"0"}}`))
	}))
	defer srv.Close()

	_, err := newTestBillingCalculateFacade(t, srv).CalculateBilling(context.Background(), billingCalcOrgID,
		&models.BillingCalculateInput{LedgerID: billingCalcLedgerID, Period: "2026-01"})
	if err != nil {
		t.Fatalf("CalculateBilling: %v", err)
	}
	if !strings.Contains(body, `"ledgerId":"`+billingCalcLedgerID+`"`) || !strings.Contains(body, `"period":"2026-01"`) {
		t.Fatalf("body = %q, want ledgerId + period", body)
	}
	if strings.Contains(body, `"type"`) {
		t.Fatalf("body = %q, empty Type must omit the type key (both-types calculation)", body)
	}
}

// TestBillingCalculateFacade_ErrorDecodes asserts a non-2xx maps to *errors.Error
// with RFC 9457 fields + request-ID correlation.
func TestBillingCalculateFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-calc-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0088","title":"Unprocessable","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestBillingCalculateFacade(t, srv).CalculateBilling(context.Background(), billingCalcOrgID, billingCalcInput())
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0088" || sdkErr.RequestID != "req-calc-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestBillingCalculateFacade_WriteReplaySafe is the 401-replay guard: the request
// body must survive a token-refresh replay intact (rewindable writeJSON body).
func TestBillingCalculateFacade_WriteReplaySafe(t *testing.T) {
	var attempts int
	var replayed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"LEDGER-0001","title":"Unauthorized","status":401}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		replayed = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":null,"summary":{"totalResults":0,"totalVolume":0,"totalMaintenance":0,"totalNetAmount":"0"}}`))
	}))
	defer srv.Close()

	_, err := newTestBillingCalculateFacade(t, srv).CalculateBilling(context.Background(), billingCalcOrgID, billingCalcInput())
	if err != nil {
		t.Fatalf("CalculateBilling with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"ledgerId":"`+billingCalcLedgerID+`"`) ||
		!strings.Contains(replayed, `"period":"2026-01"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestBillingCalculateFacade_Validation rejects bad input before any request leaves.
func TestBillingCalculateFacade_Validation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("no request should reach the server on validation failure")
	}))
	defer srv.Close()

	facade := newTestBillingCalculateFacade(t, srv)

	// Missing period.
	if _, err := facade.CalculateBilling(context.Background(), billingCalcOrgID, &models.BillingCalculateInput{LedgerID: billingCalcLedgerID}); err == nil {
		t.Fatal("CalculateBilling with empty period must fail validation")
	}
	// Missing ledgerId.
	if _, err := facade.CalculateBilling(context.Background(), billingCalcOrgID, &models.BillingCalculateInput{Period: "2026-01"}); err == nil {
		t.Fatal("CalculateBilling with empty ledgerId must fail validation")
	}
}

func newTestBillingCalculateFacade(t *testing.T, srv *httptest.Server) *billingCalculateFacade {
	t.Helper()
	return newBillingCalculateFacade(newTestLedgerClient(t, srv))
}
