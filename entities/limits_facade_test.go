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
	limitID       = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	moneyPrecise  = "100.333333333333333333" // 18 fractional digits — float64 canary
	limitScopeTxn = "PIX"
)

// limitJSON is a canonical server limit body: camelCase, limitId identity,
// maxAmount as a quoted decimal string.
func limitJSON(id, status, maxAmount string) string {
	return `{"limitId":"` + id + `","name":"daily-cap","limitType":"DAILY",` +
		`"maxAmount":"` + maxAmount + `","asset":"USD","status":"` + status + `",` +
		`"scopes":[{"transactionType":"` + limitScopeTxn + `"}],` +
		`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
}

func newCreateLimit() *models.CreateLimitInput {
	return models.NewCreateLimitInput("daily-cap", models.LimitTypeDaily,
		decimal.RequireFromString(moneyPrecise), "USD").
		WithScope(models.Scope{TransactionType: strPtr(limitScopeTxn)})
}

// TestLimitsFacade_Create201Money is the combined 201 + MONEY-PATH guard. The
// server returns 201 and the generated CreateLimitResp parser is status-exact, so
// the facade must gate on any 2xx rather than on one status; and the
// high-precision maxAmount must survive marshal → wire → decode with no loss.
func TestLimitsFacade_Create201Money(t *testing.T) {
	var method, path, reqBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		reqBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated) // 201, not 200
		_, _ = w.Write([]byte(limitJSON(limitID, "DRAFT", moneyPrecise)))
	}))
	defer srv.Close()

	lim, err := newTestLimitsFacade(t, srv).Create(context.Background(), newCreateLimit())
	if err != nil {
		t.Fatalf("Create @201: %v", err)
	}
	if method != http.MethodPost || path != "/v1/limits" {
		t.Fatalf("create req = %s %s, want POST /v1/limits", method, path)
	}
	// Marshal leg: the exact quoted string reached the wire.
	if !strings.Contains(reqBody, `"maxAmount":"`+moneyPrecise+`"`) {
		t.Fatalf("create body = %s, want maxAmount quoted string %q", reqBody, moneyPrecise)
	}
	// Decode leg: 201 decoded into a Limit with the money intact.
	if lim == nil || lim.ID != limitID {
		t.Fatalf("Create @201 returned %+v, want a decoded limit with ID %s", lim, limitID)
	}
	if !lim.MaxAmount.Equal(decimal.RequireFromString(moneyPrecise)) {
		t.Fatalf("MaxAmount = %s, want %s (money must round-trip with no loss)", lim.MaxAmount, moneyPrecise)
	}
}

// TestLimitsFacade_UpdateOmitsImmutable proves the PATCH omit-unset body reaches
// the server as PATCH and never carries the immutable limitType/asset keys.
func TestLimitsFacade_UpdateOmitsImmutable(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(limitJSON(limitID, "DRAFT", "500")))
	}))
	defer srv.Close()

	_, err := newTestLimitsFacade(t, srv).Update(context.Background(), limitID,
		models.NewUpdateLimitInput().WithMaxAmount(decimal.RequireFromString("500")))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if method != http.MethodPatch || path != "/v1/limits/"+limitID {
		t.Fatalf("update req = %s %s, want PATCH /v1/limits/%s", method, path, limitID)
	}
	if strings.Contains(body, "limitType") || strings.Contains(body, "asset") {
		t.Fatalf("update body = %q, must NOT contain immutable limitType/asset", body)
	}
	if !strings.Contains(body, `"maxAmount"`) {
		t.Fatalf("update body = %q, want the set maxAmount field", body)
	}
}

// TestLimitsFacade_ListFlatEnvelope is the load-bearing envelope red. The tracer
// serializes lists as the FLAT {limits:[...],nextCursor} envelope, not
// {items,pagination}. A straight json.Unmarshal into models.ListResponse[Limit]
// reads Items from the "items" key only and yields EMPTY Items.
func TestLimitsFacade_ListFlatEnvelope(t *testing.T) {
	body := `{"limits":[` + limitJSON(limitID, "ACTIVE", "100") + `],"hasMore":false,"nextCursor":""}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	page, err := newTestLimitsFacade(t, srv).List(context.Background(), models.LimitsListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("List Items = %d, want 1 (flat {limits:[...]} envelope must map to Items)", len(page.Items))
	}
	if page.Items[0].ID != limitID || page.Items[0].Status != "ACTIVE" {
		t.Fatalf("List Items[0] = %+v", page.Items[0])
	}
}

// TestLimitsFacade_PagesCursorStop chains two cursor pages and stops on an empty
// nextCursor, asserting the cursor advances and exactly two requests are made.
func TestLimitsFacade_PagesCursorStop(t *testing.T) {
	page1 := `{"limits":[` + limitJSON("11111111-1111-1111-1111-111111111111", "ACTIVE", "100") + `],"hasMore":true,"nextCursor":"c2"}`
	page2 := `{"limits":[` + limitJSON("22222222-2222-2222-2222-222222222222", "ACTIVE", "200") + `],"hasMore":false,"nextCursor":""}`

	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	all, err := CollectAll(newTestLimitsFacade(t, srv).ListAll(context.Background(), models.LimitsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("collected %d limits, want 2", len(all))
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", cursors)
	}
}

// TestLimitsFacade_PagesCtxCancel proves a cancelled context terminates iteration
// with the context error before any request.
func TestLimitsFacade_PagesCtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limits":[],"nextCursor":""}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectAll(newTestLimitsFacade(t, srv).ListAll(ctx, models.LimitsListOpts{}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestLimitsFacade_Delete204 proves delete succeeds on a 204 no-body response
// with nothing to decode.
func TestLimitsFacade_Delete204(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestLimitsFacade(t, srv).Delete(context.Background(), limitID); err != nil {
		t.Fatalf("Delete @204: %v", err)
	}
	if method != http.MethodDelete || path != "/v1/limits/"+limitID {
		t.Fatalf("delete req = %s %s, want DELETE /v1/limits/%s", method, path, limitID)
	}
}

// TestLimitsFacade_Lifecycle proves each body-less state transition POSTs the
// right path and decodes the 200 limit body.
func TestLimitsFacade_Lifecycle(t *testing.T) {
	tests := []struct {
		name     string
		call     func(f *limitsFacade) (*models.Limit, error)
		wantPath string
		status   string
	}{
		{"activate", func(f *limitsFacade) (*models.Limit, error) { return f.Activate(context.Background(), limitID) }, "/v1/limits/" + limitID + "/activate", "ACTIVE"},
		{"deactivate", func(f *limitsFacade) (*models.Limit, error) { return f.Deactivate(context.Background(), limitID) }, "/v1/limits/" + limitID + "/deactivate", "INACTIVE"},
		{"draft", func(f *limitsFacade) (*models.Limit, error) { return f.Draft(context.Background(), limitID) }, "/v1/limits/" + limitID + "/draft", "DRAFT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(limitJSON(limitID, tt.status, "100")))
			}))
			defer srv.Close()

			lim, err := tt.call(newTestLimitsFacade(t, srv))
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if method != http.MethodPost || path != tt.wantPath {
				t.Fatalf("%s req = %s %s, want POST %s", tt.name, method, path, tt.wantPath)
			}
			if lim == nil || lim.Status != tt.status {
				t.Fatalf("%s returned %+v, want status %s", tt.name, lim, tt.status)
			}
		})
	}
}

// TestLimitsFacade_GetUsage proves the usage snapshot decodes with decimal money
// fields and a float display ratio.
func TestLimitsFacade_GetUsage(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limitId":"` + limitID + `","currentUsage":"` + moneyPrecise + `",` +
			`"limitAmount":"200.5","utilizationPercent":50.04,"nearLimit":true}`))
	}))
	defer srv.Close()

	snap, err := newTestLimitsFacade(t, srv).GetUsage(context.Background(), limitID)
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if method != http.MethodGet || path != "/v1/limits/"+limitID+"/usage" {
		t.Fatalf("usage req = %s %s, want GET /v1/limits/%s/usage", method, path, limitID)
	}
	if !snap.CurrentUsage.Equal(decimal.RequireFromString(moneyPrecise)) {
		t.Fatalf("CurrentUsage = %s, want %s (decimal money)", snap.CurrentUsage, moneyPrecise)
	}
	if snap.UtilizationPercent != 50.04 || !snap.NearLimit {
		t.Fatalf("snapshot = %+v", snap)
	}
}

// TestLimitsFacade_Error maps a non-2xx problem+json into *errors.Error with the
// server request-ID threaded through.
func TestLimitsFacade_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-limit-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0042","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestLimitsFacade(t, srv).Get(context.Background(), limitID)
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0042" || sdkErr.RequestID != "req-limit-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestLimitsFacade_ListError maps a non-2xx problem+json from the LIST endpoint
// into *errors.Error with the APICode and server request-ID extracted. This
// exercises List's own DecodeProblemJSON branch — distinct from the decodeOne
// path Get uses in TestLimitsFacade_Error.
func TestLimitsFacade_ListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-limit-list-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0044","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestLimitsFacade(t, srv).List(context.Background(), models.LimitsListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0044" || sdkErr.RequestID != "req-limit-list-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// The class is a RESPONSE DECODE error, not an internal one: the server
// answered and the SDK could not read the answer, which is a different
// fact from "the SDK is broken" and is what a caller needs in order to
// decide whether to reconcile.
// TestLimitsFacade_ListMalformedBody proves a 200 whose body is not valid JSON
// for the flat {limits:[...]} envelope surfaces as a typed response-decode error
// rather than an empty page or a panic.
func TestLimitsFacade_ListMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limits": not-valid-json`))
	}))
	defer srv.Close()

	_, err := newTestLimitsFacade(t, srv).List(context.Background(), models.LimitsListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.Code != sdkerrors.CodeResponseDecode {
		t.Fatalf("error code = %q, want %q (malformed body must be a response-decode error)", sdkErr.Code, sdkerrors.CodeResponseDecode)
	}
}

// TestLimitsFacade_ValidateBeforeWire proves bad input is rejected before any
// round trip (no server contact).
func TestLimitsFacade_ValidateBeforeWire(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	f := newTestLimitsFacade(t, srv)

	// Missing scopes (limits require 1-100).
	if _, err := f.Create(context.Background(),
		models.NewCreateLimitInput("cap", models.LimitTypeDaily, decimal.RequireFromString("100"), "USD")); err == nil {
		t.Fatalf("missing scopes should be rejected before the wire")
	}
	// Zero max amount.
	if _, err := f.Create(context.Background(),
		models.NewCreateLimitInput("cap", models.LimitTypeDaily, decimal.Zero, "USD").
			WithScope(models.Scope{TransactionType: strPtr(limitScopeTxn)})); err == nil {
		t.Fatalf("zero max amount should be rejected before the wire")
	}
	if hit {
		t.Fatalf("validation failures must not contact the server")
	}
}

func newTestLimitsFacade(t *testing.T, srv *httptest.Server) *limitsFacade {
	t.Helper()
	return newLimitsFacade(newTestTracerClient(t, srv), true)
}
