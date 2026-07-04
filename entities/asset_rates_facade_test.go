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
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	assetRatesOrgID    = "11111111-1111-1111-1111-111111111111"
	assetRatesLedgerID = "22222222-2222-2222-2222-222222222222"
)

func assetRatesBase() string {
	return "/v1/organizations/" + assetRatesOrgID + "/ledgers/" + assetRatesLedgerID + "/asset-rates"
}

// TestAssetRatesFacade_CreateOrUpdate is the money-path assert. The upsert is a
// PUT to .../asset-rates (no separate Update endpoint), and the rate/scale must
// ride the wire as JSON integers in the int+scale fixed-point shape (rate:525,
// scale:2 == 5.25), byte-for-byte with the legacy json.Marshal(input). The
// returned AssetRate.Rate must round-trip that fixed-point integer with NO
// truncation: 525 stays 525 (not 5, not 5.25 collapsed to a float that loses
// precision), scale stays 2.
func TestAssetRatesFacade_CreateOrUpdate(t *testing.T) {
	var m, p, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Server echoes the fixed-point shape back verbatim.
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","from":"USD","to":"BRL","rate":525,"scale":2,"source":"Central Bank"}`))
	}))
	defer srv.Close()

	rate, err := newTestAssetRatesFacade(t, srv).CreateOrUpdateAssetRate(context.Background(), assetRatesOrgID, assetRatesLedgerID,
		models.NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2).WithSource("Central Bank"))
	if err != nil {
		t.Fatalf("CreateOrUpdateAssetRate: %v", err)
	}

	// Upsert is a PUT to the collection path — never POST, never a per-id path.
	if m != http.MethodPut || p != assetRatesBase() {
		t.Fatalf("create req = %s %s, want PUT %s", m, p, assetRatesBase())
	}

	// Money-path wire assert: rate and scale on the wire are JSON integers in the
	// fixed-point shape, matching the legacy json.Marshal of CreateAssetRateInput.
	var sent struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Rate  int    `json:"rate"`
		Scale int    `json:"scale"`
	}
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("sent body not decodable: %v (body=%q)", err, body)
	}
	if sent.From != "USD" || sent.To != "BRL" || sent.Rate != 525 || sent.Scale != 2 {
		t.Fatalf("sent body = %+v, want from=USD to=BRL rate=525 scale=2 (fixed-point int shape)", sent)
	}
	if strings.Contains(body, `"rate":5.25`) {
		t.Fatalf("rate serialized as a float, want fixed-point integer 525: %q", body)
	}

	// Money-path round-trip assert: the decoded rate is the fixed-point integer
	// 525 (no truncation to 5, no float collapse), and scale is 2.
	if rate.Rate == nil || !rate.Rate.Equal(decimal.NewFromInt(525)) {
		t.Fatalf("decoded rate = %v, want exactly 525 (fixed-point, no truncation)", rate.Rate)
	}
	if rate.Scale == nil || *rate.Scale != 2 {
		t.Fatalf("decoded scale = %v, want 2", rate.Scale)
	}
}

// TestAssetRatesFacade_GetByExternalID round-trips a read by external ID over
// the generated client, asserting verb+path match the legacy wire
// (GET .../asset-rates/{externalId}).
func TestAssetRatesFacade_GetByExternalID(t *testing.T) {
	const externalID = "44444444-4444-4444-4444-444444444444"

	var m, p string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","externalId":"` + externalID + `","from":"USD","to":"BRL","rate":525,"scale":2}`))
	}))
	defer srv.Close()

	rate, err := newTestAssetRatesFacade(t, srv).GetAssetRate(context.Background(), assetRatesOrgID, assetRatesLedgerID, externalID)
	if err != nil {
		t.Fatalf("GetAssetRate: %v", err)
	}
	if m != http.MethodGet || p != assetRatesBase()+"/"+externalID {
		t.Fatalf("get req = %s %s, want GET %s/%s", m, p, assetRatesBase(), externalID)
	}
	if rate.ExternalID != externalID {
		t.Fatalf("GetAssetRate returned %+v", rate)
	}
	if rate.Rate == nil || !rate.Rate.Equal(decimal.NewFromInt(525)) {
		t.Fatalf("decoded rate = %v, want 525 (no truncation on read)", rate.Rate)
	}
}

// TestAssetRatesFacade_ListAndPaginate exercises the cursor List/Pages/All
// trinaldo end-to-end over the .../asset-rates/from/{assetCode} path, chaining
// two cursor pages then stopping on an empty next_cursor. A HasMore()-based stop
// would loop forever on the terminal page; this asserts the cursor-pure stop.
func TestAssetRatesFacade_ListAndPaginate(t *testing.T) {
	const assetCode = "USD"
	page1 := `{"items":[{"id":"33333333-3333-3333-3333-333333333333","from":"USD","to":"BRL","rate":525,"scale":2}],"limit":1,"next_cursor":"c2"}`
	page2 := `{"items":[{"id":"55555555-5555-5555-5555-555555555555","from":"USD","to":"EUR","rate":90,"scale":2}],"limit":1}`

	var seenCursors []string
	var seenPaths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCursors = append(seenCursors, r.URL.Query().Get("cursor"))
		seenPaths = append(seenPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	all, err := CollectAll(newTestAssetRatesFacade(t, srv).ListAssetRatesByAssetCodeAll(context.Background(), assetRatesOrgID, assetRatesLedgerID, assetCode, models.AssetRatesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 || all[0].To != "BRL" || all[1].To != "EUR" {
		t.Fatalf("All = %+v", all)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "" || seenCursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", seenCursors)
	}
	wantPath := assetRatesBase() + "/from/" + assetCode
	if seenPaths[0] != wantPath {
		t.Fatalf("list path = %q, want %q", seenPaths[0], wantPath)
	}
}

// TestAssetRatesFacade_Filters is the per-resource differentiator. The
// cursor/sort/date fields map to native generated param slots; the to[] filter
// also has a native slot that serializes explode=false as a single
// comma-joined param (to=BRL,EUR), byte-identical to the legacy
// ToQueryParams strings.Join.
func TestAssetRatesFacade_Filters(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"limit":7}`))
	}))
	defer srv.Close()

	_, err := newTestAssetRatesFacade(t, srv).ListAssetRatesByAssetCode(context.Background(), assetRatesOrgID, assetRatesLedgerID, "USD", models.AssetRatesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 7, SortDirection: models.SortAscending, StartDate: "2025-01-01", EndDate: "2025-12-31"},
		Filters:        models.AssetRatesFilters{To: []string{"BRL", "EUR"}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := q.Get("limit"); got != "7" {
		t.Fatalf("limit = %q, want 7 (param slot)", got)
	}
	if got := q.Get("sort_order"); got != "asc" {
		t.Fatalf("sort_order = %q, want asc (param slot)", got)
	}
	if got := q.Get("start_date"); got != "2025-01-01" {
		t.Fatalf("start_date = %q, want 2025-01-01 (param slot)", got)
	}
	if got := q.Get("end_date"); got != "2025-12-31" {
		t.Fatalf("end_date = %q, want 2025-12-31 (param slot)", got)
	}
	// to[] rides the native slot; explode=false renders it as one comma-joined
	// param, matching the legacy strings.Join wire.
	if got := q.Get("to"); got != "BRL,EUR" {
		t.Fatalf("to = %q, want comma-joined BRL,EUR (native slot, explode=false)", got)
	}
}

// TestAssetRatesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID on
// the write path.
func TestAssetRatesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-ar-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestAssetRatesFacade(t, srv).CreateOrUpdateAssetRate(context.Background(), assetRatesOrgID, assetRatesLedgerID,
		models.NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-ar-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestAssetRatesFacade_WriteReplaySafe is the money-path 401-replay guard: the
// PUT body must survive the auth round tripper's post-401 replay with the
// fixed-point rate/scale integers intact on the replayed request.
func TestAssetRatesFacade_WriteReplaySafe(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","from":"USD","to":"BRL","rate":525,"scale":2}`))
	}))
	defer srv.Close()

	_, err := newTestAssetRatesFacade(t, srv).CreateOrUpdateAssetRate(context.Background(), assetRatesOrgID, assetRatesLedgerID,
		models.NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2))
	if err != nil {
		t.Fatalf("CreateOrUpdateAssetRate with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"rate":525`) || !strings.Contains(replayed, `"scale":2`) {
		t.Fatalf("replayed body = %q, want full JSON with fixed-point rate/scale intact", replayed)
	}
}

func newTestAssetRatesFacade(t *testing.T, srv *httptest.Server) *assetRatesFacade {
	t.Helper()
	return newAssetRatesFacade(newTestLedgerClient(t, srv), true)
}
