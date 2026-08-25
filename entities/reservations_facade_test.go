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

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	reserveTxID   = "12121212-1212-1212-1212-121212121212"
	reservationID = "34343434-3434-3434-3434-343434343434"
	reserveResID1 = "56565656-5656-5656-5656-565656565656"
	reserveResID2 = "78787878-7878-7878-7878-787878787878"
)

func reserveResponseJSON() string {
	return `{"transactionId":"` + reserveTxID + `","denied":false,` +
		`"reservationIds":["` + reserveResID1 + `","` + reserveResID2 + `"]}`
}

func newTestReservationsFacade(t *testing.T, srv *httptest.Server) *reservationsFacade {
	t.Helper()
	return newReservationsFacade(newTestTracerClient(t, srv))
}

func validReserveInput() *models.ReserveInput {
	return models.NewReserveInput(
		reserveTxID,
		valRequestID,
		decimal.RequireFromString(bigMoney),
		"USD",
		"2026-01-01T00:00:00Z",
	)
}

// TestReservationsFacade_Reserve201 is the load-bearing raw-gate guard. The
// server returns 201 on a new reservation and the generated CreateReservationResp
// parser is status-exact, so the facade MUST route the write through the raw call
// + 2xx gate rather than depend on one exact success status. Also proves money is
// sent as a quoted decimal string (never a float).
func TestReservationsFacade_Reserve201(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(reserveResponseJSON()))
	}))
	defer srv.Close()

	resp, err := newTestReservationsFacade(t, srv).Reserve(context.Background(), validReserveInput())
	if err != nil {
		t.Fatalf("Reserve @201: %v", err)
	}
	if method != http.MethodPost || path != "/v1/reservations" {
		t.Fatalf("reserve req = %s %s, want POST /v1/reservations", method, path)
	}
	if resp == nil || resp.TransactionID != reserveTxID || resp.Denied {
		t.Fatalf("Reserve @201 returned %+v", resp)
	}
	if len(resp.ReservationIDs) != 2 || resp.ReservationIDs[0] != reserveResID1 {
		t.Fatalf("Reserve reservationIds = %v, want 2 ids", resp.ReservationIDs)
	}
	// bigMoney exceeds float64's exact range; a quoted-string body proves the
	// money never round-tripped through binary floating point.
	if !strings.Contains(body, `"amount":"`+bigMoney+`"`) {
		t.Fatalf("request body amount not a quoted decimal string: %q", body)
	}
}

// TestReservationsFacade_ReserveDenied proves the DENY verdict wires through: a
// 201 body with denied:true and an empty reservationIds decodes to Denied==true
// with zero handles. Guards the `denied` json tag — a silent typo would decode a
// REFUSED transaction as Denied==false (the ledger would believe capacity was
// held when it was not), the core protection footgun. The happy path
// (denied:false) is covered by Reserve201.
func TestReservationsFacade_ReserveDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"transactionId":"` + reserveTxID + `","denied":true,"reservationIds":[]}`))
	}))
	defer srv.Close()

	resp, err := newTestReservationsFacade(t, srv).Reserve(context.Background(), validReserveInput())
	if err != nil {
		t.Fatalf("Reserve denied: %v", err)
	}
	if !resp.Denied {
		t.Fatalf("Denied = false, want true (a refused transaction must never decode as allowed)")
	}
	if len(resp.ReservationIDs) != 0 {
		t.Fatalf("ReservationIDs = %v, want empty on denial", resp.ReservationIDs)
	}
}

// TestReservationsFacade_ByID exercises confirm and release by reservation id.
// Both return 200 + ReservationActionResponse.
func TestReservationsFacade_ByID(t *testing.T) {
	cases := []struct {
		name   string
		call   func(f *reservationsFacade) (*models.ReservationActionResponse, error)
		verb   string
		status string
	}{
		{"confirm", func(f *reservationsFacade) (*models.ReservationActionResponse, error) {
			return f.Confirm(context.Background(), reservationID)
		}, "confirm", "CONFIRMED"},
		{"release", func(f *reservationsFacade) (*models.ReservationActionResponse, error) {
			return f.Release(context.Background(), reservationID)
		}, "release", "RELEASED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var method, path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"reservationId":"` + reservationID + `","status":"` + tc.status + `"}`))
			}))
			defer srv.Close()

			resp, err := tc.call(newTestReservationsFacade(t, srv))
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			wantPath := "/v1/reservations/" + reservationID + "/" + tc.verb
			if method != http.MethodPost || path != wantPath {
				t.Fatalf("%s req = %s %s, want POST %s", tc.name, method, path, wantPath)
			}
			if resp.ReservationID != reservationID || resp.Status != tc.status {
				t.Fatalf("%s returned %+v", tc.name, resp)
			}
		})
	}
}

// TestReservationsFacade_ByTransactionFlippedZero proves flipped==0 is a VALID
// idempotent no-op success — it returns (resp, nil), NEVER an error.
func TestReservationsFacade_ByTransactionFlippedZero(t *testing.T) {
	cases := []struct {
		name   string
		call   func(f *reservationsFacade) (*models.TransactionActionResponse, error)
		verb   string
		status string
	}{
		{"confirm", func(f *reservationsFacade) (*models.TransactionActionResponse, error) {
			return f.ConfirmByTransaction(context.Background(), reserveTxID)
		}, "confirm", "CONFIRMED"},
		{"release", func(f *reservationsFacade) (*models.TransactionActionResponse, error) {
			return f.ReleaseByTransaction(context.Background(), reserveTxID)
		}, "release", "RELEASED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var method, path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"transactionId":"` + reserveTxID + `","status":"` + tc.status + `","flipped":0}`))
			}))
			defer srv.Close()

			resp, err := tc.call(newTestReservationsFacade(t, srv))
			if err != nil {
				t.Fatalf("%s flipped=0 must be a nil-error success: %v", tc.name, err)
			}
			wantPath := "/v1/reservations/transaction/" + reserveTxID + "/" + tc.verb
			if method != http.MethodPost || path != wantPath {
				t.Fatalf("%s req = %s %s, want POST %s", tc.name, method, path, wantPath)
			}
			if resp.TransactionID != reserveTxID || resp.Status != tc.status || resp.Flipped != 0 {
				t.Fatalf("%s returned %+v, want flipped=0 success", tc.name, resp)
			}
		})
	}
}

// TestReservationsFacade_ByTransactionFlippedCount proves a non-zero flipped count
// decodes correctly.
func TestReservationsFacade_ByTransactionFlippedCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionId":"` + reserveTxID + `","status":"CONFIRMED","flipped":3}`))
	}))
	defer srv.Close()

	resp, err := newTestReservationsFacade(t, srv).ConfirmByTransaction(context.Background(), reserveTxID)
	if err != nil {
		t.Fatalf("ConfirmByTransaction: %v", err)
	}
	if resp.Flipped != 3 {
		t.Fatalf("flipped = %d, want 3", resp.Flipped)
	}
}

// TestReservationsFacade_ValidateBeforeWire proves bad input is rejected before
// any round trip (no server contact).
func TestReservationsFacade_ValidateBeforeWire(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	bad := validReserveInput()
	bad.TransactionID = ""
	if _, err := newTestReservationsFacade(t, srv).Reserve(context.Background(), bad); err == nil {
		t.Fatalf("empty transactionId: want validation error before the wire")
	}
	if hit {
		t.Fatalf("validation failures must not contact the server")
	}
}

// TestReservationsFacade_Error maps a non-2xx problem+json into *errors.Error with
// the server request-ID threaded through.
func TestReservationsFacade_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-res-409")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"TRACER-0030","title":"Conflict","status":409}`))
	}))
	defer srv.Close()

	_, err := newTestReservationsFacade(t, srv).Confirm(context.Background(), reservationID)
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0030" || sdkErr.RequestID != "req-res-409" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}
