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
	"reflect"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

const (
	feeEstimateOrgID     = "11111111-1111-1111-1111-111111111111"
	feeEstimateLedgerID  = "22222222-2222-2222-2222-222222222222"
	feeEstimatePackageID = "33333333-3333-3333-3333-333333333333"
)

func feeEstimateBase() string {
	return "/v2/organizations/" + feeEstimateOrgID + "/ledgers/" + feeEstimateLedgerID + "/estimates"
}

func feeEstimateInput() *models.FeeEstimateInput {
	return &models.FeeEstimateInput{
		PackageID: feeEstimatePackageID,
		LedgerID:  feeEstimateLedgerID,
		Transaction: models.FeeEstimateTransactionInput{
			Description: "estimate",
			Send: &models.SendInput{
				Asset: "BRL",
				Value: "100.00",
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{AccountAlias: "@source", Amount: models.AmountInput{Asset: "BRL", Value: "100.00"}},
					},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{
						{AccountAlias: "@dest", Amount: models.AmountInput{Asset: "BRL", Value: "100.00"}},
					},
				},
			},
		},
	}
}

// TestFeeEstimateFacade_Applied is the money-adjacent happy path: a 2xx whose
// feesApplied is populated must decode the fee-adjusted transaction, and every
// amount (the fee-adjusted send value) must ride the wire as a string with no
// float hop. 0.333333333333333333 is unrepresentable in float64 and would drift.
func TestFeeEstimateFacade_Applied(t *testing.T) {
	const precise = "0.333333333333333333"
	resp := `{"message":"Successfully estimated fee.","feesApplied":{` +
		`"ledgerId":"` + feeEstimateLedgerID + `",` +
		`"segmentId":"44444444-4444-4444-4444-444444444444",` +
		`"transaction":{"description":"estimate","chartOfAccountsGroupName":"FUNDING","code":"C1","pending":false,` +
		`"routeId":"55555555-5555-5555-5555-555555555555",` +
		`"metadata":{"packageAppliedID":"pkg-1"},` +
		`"send":{"asset":"BRL","value":"` + precise + `",` +
		`"source":{"from":[{"accountAlias":"@source","amount":{"asset":"BRL","value":"` + precise + `"}}]},` +
		`"distribute":{"to":[{"accountAlias":"@dest","amount":{"asset":"BRL","value":"` + precise + `"}}]}}}}}`

	var m, p, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	out, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput())
	if err != nil {
		t.Fatalf("EstimateFee: %v", err)
	}
	if m != http.MethodPost || p != feeEstimateBase() {
		t.Fatalf("req = %s %s, want POST %s", m, p, feeEstimateBase())
	}
	if !strings.Contains(body, `"packageId":"`+feeEstimatePackageID+`"`) ||
		!strings.Contains(body, `"ledgerId":"`+feeEstimateLedgerID+`"`) {
		t.Fatalf("body = %q, want packageId + ledgerId", body)
	}
	if !strings.Contains(body, `"value":"100.00"`) {
		t.Fatalf("body = %q, want string send value (no float hop)", body)
	}
	if out.Message != "Successfully estimated fee." {
		t.Fatalf("Message = %q", out.Message)
	}
	if out.FeesApplied == nil {
		t.Fatalf("FeesApplied nil, want populated result")
	}
	if out.FeesApplied.LedgerID != feeEstimateLedgerID {
		t.Fatalf("LedgerID = %q", out.FeesApplied.LedgerID)
	}
	if out.FeesApplied.SegmentID == nil || *out.FeesApplied.SegmentID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("SegmentID = %v", out.FeesApplied.SegmentID)
	}
	tx := out.FeesApplied.Transaction
	if tx.Send == nil || tx.Send.Value != precise {
		t.Fatalf("fee-adjusted send value = %+v, want %q (no float hop)", tx.Send, precise)
	}
	if len(tx.Send.Source.From) != 1 || tx.Send.Source.From[0].Amount.Value != precise {
		t.Fatalf("source amount = %+v, want %q", tx.Send.Source, precise)
	}
	if tx.Send.Source.From[0].AccountAlias != "@source" {
		t.Fatalf("source leg accountAlias = %q, want %q", tx.Send.Source.From[0].AccountAlias, "@source")
	}
	if len(tx.Send.Distribute.To) != 1 || tx.Send.Distribute.To[0].Amount.Value != precise {
		t.Fatalf("distribute amount = %+v, want %q", tx.Send.Distribute, precise)
	}
	if tx.RouteID == nil || *tx.RouteID != "55555555-5555-5555-5555-555555555555" {
		t.Fatalf("routeId = %v", tx.RouteID)
	}
}

// TestFeeEstimateFacade_GoldenRequestBody pins the estimate request body against
// a hand-written payload. The estimate reuses the transaction leg types, so any
// change to their serialization moves this endpoint too — and the quote it
// returns drives real fee-bearing transactions. The leg identity is accountAlias,
// matching the ledger DTO the engine unmarshals into
// (components/ledger/pkg/feeshared/model.FeeEstimate embeds
// pkg/mtransaction.Transaction, whose FromTo carries only accountAlias); money
// rides as decimal strings; a nil source/distribute is omitted, never null.
func TestFeeEstimateFacade_GoldenRequestBody(t *testing.T) {
	want := map[string]any{
		"packageId": feeEstimatePackageID,
		"ledgerId":  feeEstimateLedgerID,
		"transaction": map[string]any{
			"description": "estimate",
			"send": map[string]any{
				"asset": "BRL",
				"value": "100.00",
				"source": map[string]any{"from": []any{map[string]any{
					"accountAlias": "@source",
					"amount":       map[string]any{"asset": "BRL", "value": "100.00"},
				}}},
				"distribute": map[string]any{"to": []any{map[string]any{
					"accountAlias": "@dest",
					"amount":       map[string]any{"asset": "BRL", "value": "100.00"},
				}}},
			},
		},
	}

	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Successfully estimated fee.","feesApplied":null}`))
	}))
	defer srv.Close()

	if _, err := newTestFeeEstimateFacade(t, srv).EstimateFee(
		context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput()); err != nil {
		t.Fatalf("EstimateFee: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("request body is not a JSON object: %v (%s)", err, gotBody)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("estimate request body is not the golden payload\n got: %s\nwant: %v", gotBody, want)
	}
}

// TestFeeEstimateFacade_GoldenRequestBodyNumericValueAndShareLeg pins the two
// estimate inputs where the leg struct tags and the leg mappers DISAGREE — the
// only reason the request is well formed is that the leg types marshal through
// their mappers:
//
//   - a NUMERIC Value. Every Value field is `any`, so a caller may hand over an
//     int or a float; the mapper renders money as a decimal STRING, while the
//     tags would ship a JSON number and reopen a float hop on the money path.
//   - a leg priced by Share instead of Amount. The mapper omits the amount key,
//     while the tags would ship amount:{"asset":"","value":null} next to the
//     share, which the /transactions/json contract rejects.
//
// The pre-existing golden uses decimal strings and full amount legs — inputs
// where tags and mappers happen to agree — so it cannot see either regression.
func TestFeeEstimateFacade_GoldenRequestBodyNumericValueAndShareLeg(t *testing.T) {
	input := &models.FeeEstimateInput{
		PackageID: feeEstimatePackageID,
		LedgerID:  feeEstimateLedgerID,
		Transaction: models.FeeEstimateTransactionInput{
			Description: "estimate",
			Send: &models.SendInput{
				Asset: "BRL",
				Value: 100,
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{AccountAlias: "@source", Amount: models.AmountInput{Asset: "BRL", Value: 100}},
					},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{
						{AccountAlias: "@dest", Share: &models.Share{Percentage: 100}},
					},
				},
			},
		},
	}

	want := map[string]any{
		"packageId": feeEstimatePackageID,
		"ledgerId":  feeEstimateLedgerID,
		"transaction": map[string]any{
			"description": "estimate",
			"send": map[string]any{
				"asset": "BRL",
				"value": "100",
				"source": map[string]any{"from": []any{map[string]any{
					"accountAlias": "@source",
					"amount":       map[string]any{"asset": "BRL", "value": "100"},
				}}},
				"distribute": map[string]any{"to": []any{map[string]any{
					"accountAlias": "@dest",
					"share":        map[string]any{"percentage": float64(100)},
				}}},
			},
		},
	}

	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"Successfully estimated fee.","feesApplied":null}`))
	}))
	defer srv.Close()

	if _, err := newTestFeeEstimateFacade(t, srv).EstimateFee(
		context.Background(), feeEstimateOrgID, feeEstimateLedgerID, input); err != nil {
		t.Fatalf("EstimateFee: %v", err)
	}

	if !strings.Contains(string(gotBody), `"value":"100"`) {
		t.Errorf("body = %s, want a numeric Value rendered as the decimal string \"100\"", gotBody)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("request body is not a JSON object: %v (%s)", err, gotBody)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("estimate request body is not the golden payload\n got: %s\nwant: %v", gotBody, want)
	}
}

// TestFeeEstimateFacade_NoRules is the critical branch: a 2xx with feesApplied
// null (no fee/gratuity rules matched) is a SUCCESS, not an error. The facade
// must return (resp, nil) with FeesApplied == nil and the message intact.
func TestFeeEstimateFacade_NoRules(t *testing.T) {
	resp := `{"message":"No fee or gratuity rules were found for the given parameters.","feesApplied":null}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	out, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput())
	if err != nil {
		t.Fatalf("EstimateFee no-rules must NOT error: %v", err)
	}
	if out.FeesApplied != nil {
		t.Fatalf("FeesApplied = %+v, want nil", out.FeesApplied)
	}
	if !strings.Contains(out.Message, "No fee or gratuity rules") {
		t.Fatalf("Message = %q", out.Message)
	}
}

// TestFeeEstimateFacade_AppliedNilSend is the pointer nil-safety guard: a 2xx
// whose feesApplied is populated but transaction.send is null must decode without
// panicking (FeeAdjustedTransaction.Send is a *pointer) and return the result with
// a nil Send.
func TestFeeEstimateFacade_AppliedNilSend(t *testing.T) {
	resp := `{"message":"Successfully estimated fee.","feesApplied":{` +
		`"ledgerId":"` + feeEstimateLedgerID + `",` +
		`"transaction":{"description":"estimate","send":null}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	out, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput())
	if err != nil {
		t.Fatalf("EstimateFee with null send must NOT error: %v", err)
	}
	if out.FeesApplied == nil {
		t.Fatalf("FeesApplied nil, want populated result")
	}
	if out.FeesApplied.LedgerID != feeEstimateLedgerID {
		t.Fatalf("LedgerID = %q", out.FeesApplied.LedgerID)
	}
	if out.FeesApplied.Transaction.Send != nil {
		t.Fatalf("Send = %+v, want nil (null send decoded to nil pointer)", out.FeesApplied.Transaction.Send)
	}
}

// TestFeeEstimateFacade_ErrorDecodes asserts a non-2xx maps to *errors.Error
// with RFC 9457 fields + request-ID correlation.
func TestFeeEstimateFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-est-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0088","title":"Unprocessable","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput())
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0088" || sdkErr.RequestID != "req-est-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestFeeEstimateFacade_WriteReplaySafe is the 401-replay guard: the request
// body must survive a token-refresh replay intact (rewindable writeJSON body).
func TestFeeEstimateFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"message":"No fee or gratuity rules were found for the given parameters.","feesApplied":null}`))
	}))
	defer srv.Close()

	_, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, feeEstimateInput())
	if err != nil {
		t.Fatalf("EstimateFee with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"packageId":"`+feeEstimatePackageID+`"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

// TestFeeEstimateFacade_Validation rejects bad input before any request leaves.
func TestFeeEstimateFacade_Validation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("no request should reach the server on validation failure")
	}))
	defer srv.Close()

	facade := newTestFeeEstimateFacade(t, srv)

	if _, err := facade.EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, &models.FeeEstimateInput{}); err == nil {
		t.Fatal("EstimateFee with empty input must fail validation")
	}
}

// TestFeeEstimateFacade_LedgerReconciliation covers the path-vs-body ledger: the
// server takes the ledger from the URL AND requires ledgerId in the body, so an
// empty body value must inherit the addressed ledger and a contradicting one must
// never reach the wire (it would estimate fees against a ledger the caller did
// not address).
func TestFeeEstimateFacade_LedgerReconciliation(t *testing.T) {
	t.Run("empty body ledgerId inherits the path ledger", func(t *testing.T) {
		var body string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"no rules matched"}`))
		}))
		defer srv.Close()

		input := feeEstimateInput()
		input.LedgerID = ""

		if _, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, input); err != nil {
			t.Fatalf("EstimateFee: %v", err)
		}

		if !strings.Contains(body, `"ledgerId":"`+feeEstimateLedgerID+`"`) {
			t.Fatalf("body = %q, want the path ledger filled into ledgerId", body)
		}

		if input.LedgerID != "" {
			t.Fatalf("caller input mutated: LedgerID = %q, want it left empty", input.LedgerID)
		}
	})

	t.Run("mismatched body ledgerId is rejected before transport", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("no request may reach the server when the ledgers disagree")
		}))
		defer srv.Close()

		const otherLedger = "66666666-6666-6666-6666-666666666666"

		input := feeEstimateInput()
		input.LedgerID = otherLedger

		_, err := newTestFeeEstimateFacade(t, srv).EstimateFee(context.Background(), feeEstimateOrgID, feeEstimateLedgerID, input)
		if err == nil {
			t.Fatal("EstimateFee must reject a body ledgerId that differs from the path ledger")
		}

		if !strings.Contains(err.Error(), otherLedger) || !strings.Contains(err.Error(), feeEstimateLedgerID) {
			t.Fatalf("error = %v, want both ledger IDs named", err)
		}
	})
}

func newTestFeeEstimateFacade(t *testing.T, srv *httptest.Server) *feeEstimateFacade {
	t.Helper()
	return newFeeEstimateFacade(newTestLedgerClient(t, srv))
}
