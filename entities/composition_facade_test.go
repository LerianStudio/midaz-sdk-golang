// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
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
	compositionFacadeOrgID    = "11111111-1111-1111-1111-111111111111"
	compositionFacadeLedgerID = "22222222-2222-2222-2222-222222222222"
	compositionFacadeHolderID = "33333333-3333-3333-3333-333333333333"
)

func compositionFacadePath() string {
	return "/v1/organizations/" + compositionFacadeOrgID +
		"/ledgers/" + compositionFacadeLedgerID +
		"/holders/" + compositionFacadeHolderID + "/accounts"
}

func compositionInput() *models.CreateHolderAccountInput {
	return &models.CreateHolderAccountInput{
		Name:      "Ops Cash",
		AssetCode: "USD",
		Type:      "deposit",
	}
}

// TestCompositionFacade_CreateHolderAccount_FullSuccess is verification (a): a
// real HTTP 201 with both account and instrument populated must decode into the
// composite response — NOT be misclassified as an error the way a JSON200-gated
// ...WithResponse parser would (the Phase 2 success-gate defect: a 201 falls to
// the default branch). The wire route is asserted org+ledger+holder-in-path.
func TestCompositionFacade_CreateHolderAccount_FullSuccess(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"account":{"id":"44444444-4444-4444-4444-444444444444","name":"Ops Cash","assetCode":"USD"},
			"instrument":{"id":"55555555-5555-5555-5555-555555555555","holderId":"` + compositionFacadeHolderID + `"}
		}`))
	}))
	defer srv.Close()

	resp, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())
	if err != nil {
		t.Fatalf("CreateHolderAccount (201 full success): %v", err)
	}

	if method != http.MethodPost || path != compositionFacadePath() {
		t.Fatalf("req = %s %s, want POST %s", method, path, compositionFacadePath())
	}
	if !strings.Contains(body, `"name":"Ops Cash"`) || !strings.Contains(body, `"assetCode":"USD"`) {
		t.Fatalf("body = %q, want marshaled CreateHolderAccountInput", body)
	}
	if resp.Account == nil || resp.Account.ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("Account = %+v, want populated", resp.Account)
	}
	if resp.Instrument == nil || resp.Instrument.ID == nil {
		t.Fatalf("Instrument = %+v, want populated", resp.Instrument)
	}
	if resp.InstrumentError != nil {
		t.Fatalf("InstrumentError = %+v, want nil on full success", resp.InstrumentError)
	}
}

// TestCompositionFacade_CreateHolderAccount_PartialFailure is verification (b),
// the correctness crown jewel: the server persists the account, fails the
// instrument, and STILL returns HTTP 201 with {account, instrument:null,
// instrumentError:{status:FAILED,reason:...}}. This MUST be a Go SUCCESS —
// (resp, nil) with Account!=nil, Instrument==nil, InstrumentError populated. It
// must NEVER become a Go error: there is no rollback, the account is real.
func TestCompositionFacade_CreateHolderAccount_PartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"account":{"id":"44444444-4444-4444-4444-444444444444","name":"Ops Cash","assetCode":"USD"},
			"instrument":null,
			"instrumentError":{"status":"FAILED","reason":"INSTRUMENT-0007"}
		}`))
	}))
	defer srv.Close()

	resp, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())
	if err != nil {
		t.Fatalf("partial-failure must be a Go success, got err: %v", err)
	}
	if resp.Account == nil || resp.Account.ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("Account = %+v, want persisted account", resp.Account)
	}
	if resp.Instrument != nil {
		t.Fatalf("Instrument = %+v, want nil (instrument write failed)", resp.Instrument)
	}
	if resp.InstrumentError == nil {
		t.Fatalf("InstrumentError = nil, want populated failure block")
	}
	if resp.InstrumentError.Status != "FAILED" || resp.InstrumentError.Reason != "INSTRUMENT-0007" {
		t.Fatalf("InstrumentError = %+v, want {FAILED, INSTRUMENT-0007}", resp.InstrumentError)
	}
}

// TestCompositionFacade_CreateHolderAccount_Success200 locks the OTHER half of
// the success gate: the server returns HTTP 200 (not 201) with a populated
// {account, instrument} body. isSuccess(2xx) must accept it exactly like 201.
// A regression narrowing the gate to 201-only (or re-adopting the JSON200-vs-
// default parser) would still pass every 201 test — this is the only test that
// catches it.
func TestCompositionFacade_CreateHolderAccount_Success200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"account":{"id":"44444444-4444-4444-4444-444444444444","name":"Ops Cash","assetCode":"USD"},
			"instrument":{"id":"55555555-5555-5555-5555-555555555555","holderId":"` + compositionFacadeHolderID + `"}
		}`))
	}))
	defer srv.Close()

	resp, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())
	if err != nil {
		t.Fatalf("CreateHolderAccount (200 full success): %v", err)
	}
	if resp.Account == nil || resp.Account.ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("Account = %+v, want populated on 200", resp.Account)
	}
	if resp.Instrument == nil || resp.Instrument.ID == nil {
		t.Fatalf("Instrument = %+v, want populated on 200", resp.Instrument)
	}
	if resp.InstrumentError != nil {
		t.Fatalf("InstrumentError = %+v, want nil on full success", resp.InstrumentError)
	}
}

// TestCompositionFacade_CreateHolderAccount_AccountOnly is the account-only
// success path: no instrument was requested, so the server returns 201 with
// {account, instrument:null} and NO instrumentError. This is distinct from the
// partial-failure path (null + instrumentError): a null instrument with no
// error means "none requested", NOT "instrument failed". A future change that
// mistreats null-instrument-no-error as a partial failure must be caught here.
func TestCompositionFacade_CreateHolderAccount_AccountOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"account":{"id":"44444444-4444-4444-4444-444444444444","name":"Ops Cash","assetCode":"USD"},
			"instrument":null
		}`))
	}))
	defer srv.Close()

	resp, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())
	if err != nil {
		t.Fatalf("account-only creation must be a Go success, got err: %v", err)
	}
	if resp.Account == nil || resp.Account.ID != "44444444-4444-4444-4444-444444444444" {
		t.Fatalf("Account = %+v, want persisted account", resp.Account)
	}
	if resp.Instrument != nil {
		t.Fatalf("Instrument = %+v, want nil (none requested)", resp.Instrument)
	}
	if resp.InstrumentError != nil {
		t.Fatalf("InstrumentError = %+v, want nil (no instrument failed — none was requested)", resp.InstrumentError)
	}
}

// TestCompositionFacade_CreateHolderAccount_Error is verification (c): a
// transport-level non-2xx surfaces as *errors.Error with RFC 9457 decode and
// X-Request-ID correlation.
func TestCompositionFacade_CreateHolderAccount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-comp-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-comp-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestCompositionFacade_CreateHolderAccount_Validation asserts input.Validate()
// runs at the boundary: a missing assetCode never reaches the wire.
func TestCompositionFacade_CreateHolderAccount_Validation(t *testing.T) {
	var reached bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	_, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID,
		&models.CreateHolderAccountInput{Type: "deposit"})
	if err == nil {
		t.Fatalf("want validation error for missing assetCode, got nil")
	}
	if reached {
		t.Fatalf("request reached the server; validation must reject before the wire")
	}
}

// TestCompositionFacade_CreateHolderAccount_ReplaySafe is the 401-replay guard:
// the write body must survive the auth round tripper's post-401 replay
// (rewindable *bytes.Reader), since the composition endpoint has no idempotency
// slot.
func TestCompositionFacade_CreateHolderAccount_ReplaySafe(t *testing.T) {
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
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"account":{"id":"44444444-4444-4444-4444-444444444444","name":"Ops Cash"}}`))
	}))
	defer srv.Close()

	_, err := newTestCompositionFacade(t, srv).CreateHolderAccount(
		context.Background(), compositionFacadeOrgID, compositionFacadeLedgerID, compositionFacadeHolderID, compositionInput())
	if err != nil {
		t.Fatalf("CreateHolderAccount with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"name":"Ops Cash"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

func newTestCompositionFacade(t *testing.T, srv *httptest.Server) *compositionFacade {
	t.Helper()
	return newCompositionFacade(newTestLedgerClient(t, srv))
}
