// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	validationID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	valRequestID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	valAccountID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	valLimitID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	valMatchedID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	valSegmentID  = "11111111-2222-3333-4444-555555555555"
	valPortfolioX = "66666666-7777-8888-9999-000000000000"
	// bigMoney is larger than any float64 can hold exactly — its round trip
	// proves money never touches binary floating point.
	bigMoney       = "79228162514264337593543950335.75"
	limitAmountStr = "50000000000000000000.123456789"
	usageStr       = "12345678901234567890.987654321"
	attemptStr     = "1000000000000000000.5"
)

// limitUsageJSON is a canonical LimitUsageDetail wire body with the money triple
// as quoted strings (swaggertype:"string" on the server).
func limitUsageJSON() string {
	return `{"limitId":"` + valLimitID + `","limitAmount":"` + limitAmountStr + `",` +
		`"currentUsage":"` + usageStr + `","attemptedAmount":"` + attemptStr + `",` +
		`"exceeded":true,"period":"DAILY","scope":"account"}`
}

// validationResponseJSON is a canonical ValidationResponse (the POST verdict).
func validationResponseJSON(decision string) string {
	return `{"decision":"` + decision + `","reason":"matched",` +
		`"matchedRuleIds":["` + valMatchedID + `"],"evaluatedRuleIds":["` + valMatchedID + `"],` +
		`"limitUsageDetails":[` + limitUsageJSON() + `],` +
		`"processingTimeMs":2.5,"totalRulesLoaded":3,"truncated":false,` +
		`"validationId":"` + validationID + `","requestId":"` + valRequestID + `",` +
		`"evaluatedAt":"2026-01-01T00:00:00Z"}`
}

// transactionValidationJSON is a canonical stored record (the Get body).
func transactionValidationJSON() string {
	return `{"validationId":"` + validationID + `","requestId":"` + valRequestID + `",` +
		`"decision":"DENY","reason":"limit exceeded","amount":"` + bigMoney + `","asset":"USD",` +
		`"transactionType":"PIX",` +
		`"account":{"accountId":"` + valAccountID + `","status":"ACTIVE","type":"deposit"},` +
		`"matchedRuleIds":["` + valMatchedID + `"],"evaluatedRuleIds":["` + valMatchedID + `"],` +
		`"limitUsageDetails":[` + limitUsageJSON() + `],` +
		`"processingTimeMs":2.5,"totalRulesLoaded":3,"truncated":false,` +
		`"transactionTimestamp":"2026-01-01T00:00:00Z","createdAt":"2026-01-01T00:00:00Z"}`
}

// validationSummaryJSON is a canonical list item.
func validationSummaryJSON(id string) string {
	return `{"validationId":"` + id + `","accountId":"` + valAccountID + `",` +
		`"segmentId":"` + valSegmentID + `","portfolioId":"` + valPortfolioX + `",` +
		`"amount":"` + bigMoney + `","asset":"USD","decision":"ALLOW","reason":"ok",` +
		`"transactionType":"CARD","matchedRuleIds":["` + valMatchedID + `"],` +
		`"exceededLimitIds":["` + valLimitID + `"],"processingTimeMs":1.1,` +
		`"createdAt":"2026-01-01T00:00:00Z"}`
}

func validInput() *models.ValidateTransactionInput {
	return models.NewValidateTransactionInput(
		valRequestID,
		decimal.RequireFromString(bigMoney),
		"USD",
		"2026-01-01T00:00:00Z",
		models.AccountContext{AccountID: valAccountID, Status: "ACTIVE", Type: "deposit"},
	)
}

// TestValidationsFacade_Evaluate200and201 is the load-bearing raw-gate red. The
// server returns 200 on an idempotent replay and 201 on a new verdict, but the
// generated ValidateTransactionResp parser only fills JSON200 on an exact 200.
// A WithResponse-based Evaluate drops the 201 body. The facade MUST route the
// write through the raw call + 2xx success gate so BOTH statuses decode, WITHOUT
// surfacing the 200/201 distinction to the caller.
func TestValidationsFacade_Evaluate200and201(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var method, path, body string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				b, _ := io.ReadAll(r.Body)
				body = string(b)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(validationResponseJSON("DENY")))
			}))
			defer srv.Close()

			resp, err := newTestValidationsFacade(t, srv).Evaluate(context.Background(), validInput())
			if err != nil {
				t.Fatalf("Evaluate @%d: %v", status, err)
			}
			if method != http.MethodPost || path != "/v1/validations" {
				t.Fatalf("evaluate req = %s %s, want POST /v1/validations", method, path)
			}
			if resp == nil || resp.Decision != "DENY" || resp.ValidationID != validationID {
				t.Fatalf("Evaluate @%d returned %+v", status, resp)
			}
			// Money is a quoted string on the wire (never a JSON number).
			if !strings.Contains(body, `"amount":"`+bigMoney+`"`) {
				t.Fatalf("request body amount not a quoted decimal string: %q", body)
			}
		})
	}
}

// TestValidationsFacade_EvaluateMoneyDecimal proves the verdict's
// LimitUsageDetails money triple (limitAmount/currentUsage/attemptedAmount)
// decodes with EXACT decimal precision on the POST /v1/validations path — each
// value exceeds float64's exact range, so a float path would corrupt it. The
// Get path is covered by TestValidationsFacade_MoneyDecimal; this guards the
// Evaluate response decode.
func TestValidationsFacade_EvaluateMoneyDecimal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(validationResponseJSON("DENY")))
	}))
	defer srv.Close()

	resp, err := newTestValidationsFacade(t, srv).Evaluate(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(resp.LimitUsageDetails) != 1 {
		t.Fatalf("LimitUsageDetails = %d, want 1", len(resp.LimitUsageDetails))
	}
	d := resp.LimitUsageDetails[0]
	if d.LimitAmount.String() != limitAmountStr || d.CurrentUsage.String() != usageStr || d.AttemptedAmount.String() != attemptStr {
		t.Fatalf("money triple lost precision: got {%s, %s, %s}", d.LimitAmount, d.CurrentUsage, d.AttemptedAmount)
	}
}

// TestValidationsFacade_MoneyDecimal proves the Amount and the LimitUsageDetail
// triple round-trip as exact decimals with no float loss. bigMoney exceeds
// float64's exact range, so an accidental float path would corrupt it.
func TestValidationsFacade_MoneyDecimal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(transactionValidationJSON()))
	}))
	defer srv.Close()

	rec, err := newTestValidationsFacade(t, srv).Get(context.Background(), validationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !rec.Amount.Equal(decimal.RequireFromString(bigMoney)) || rec.Amount.String() != bigMoney {
		t.Fatalf("Amount = %s, want exact %s", rec.Amount, bigMoney)
	}
	if len(rec.LimitUsageDetails) != 1 {
		t.Fatalf("LimitUsageDetails = %d, want 1", len(rec.LimitUsageDetails))
	}
	d := rec.LimitUsageDetails[0]
	if d.LimitAmount.String() != limitAmountStr || d.CurrentUsage.String() != usageStr || d.AttemptedAmount.String() != attemptStr {
		t.Fatalf("limit triple lost precision: got {%s, %s, %s}", d.LimitAmount, d.CurrentUsage, d.AttemptedAmount)
	}
}

// TestValidationsFacade_ListFlatEnvelope is the load-bearing envelope red. The
// tracer serializes the list as {transactionValidations:[...],nextCursor}. A
// straight json.Unmarshal into models.ListResponse reads Items only from the
// "items" key and yields EMPTY Items. The facade MUST map the domain-keyed
// envelope (correct key: transactionValidations) so Items is non-empty.
func TestValidationsFacade_ListFlatEnvelope(t *testing.T) {
	body := `{"transactionValidations":[` + validationSummaryJSON(validationID) + `],"hasMore":false,"nextCursor":""}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	page, err := newTestValidationsFacade(t, srv).List(context.Background(), models.ValidationsListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("List Items = %d, want 1 (flat {transactionValidations:[...]} must map to Items)", len(page.Items))
	}
	item := page.Items[0]
	if item.ValidationID != validationID || item.Decision != "ALLOW" {
		t.Fatalf("List Items[0] = %+v", item)
	}
	if !item.Amount.Equal(decimal.RequireFromString(bigMoney)) {
		t.Fatalf("list item Amount = %s, want %s", item.Amount, bigMoney)
	}
}

// TestValidationsFacade_ListParamMapping proves every field listValidationsParams
// maps reaches the wire under the correct query key. Guards against a copy-paste
// mis-map (e.g. AccountID written to matched_rule_id) silently returning the wrong
// protection records. Dates are RFC3339 (the tracer server strict-parses them so).
func TestValidationsFacade_ListParamMapping(t *testing.T) {
	var q url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionValidations":[],"nextCursor":""}`))
	}))
	defer srv.Close()

	opts := models.ValidationsListOpts{
		CursorListOpts: models.CursorListOpts{
			Limit:         25,
			Cursor:        "cur-1",
			SortDirection: models.SortDescending,
			StartDate:     "2026-01-01T00:00:00Z",
			EndDate:       "2026-01-31T23:59:59Z",
		},
		SortBy: "processing_time_ms",
		Filters: models.ValidationsFilters{
			Decision:        "DENY",
			AccountID:       valAccountID,
			MatchedRuleID:   valMatchedID,
			ExceededLimitID: valLimitID,
			SegmentID:       valSegmentID,
			PortfolioID:     valPortfolioX,
			TransactionType: "CARD",
		},
	}

	if _, err := newTestValidationsFacade(t, srv).List(context.Background(), opts); err != nil {
		t.Fatalf("List: %v", err)
	}

	for key, want := range map[string]string{
		"limit":             "25",
		"cursor":            "cur-1",
		"sort_order":        "desc",
		"sort_by":           "processing_time_ms",
		"start_date":        "2026-01-01T00:00:00Z",
		"end_date":          "2026-01-31T23:59:59Z",
		"decision":          "DENY",
		"account_id":        valAccountID,
		"matched_rule_id":   valMatchedID,
		"exceeded_limit_id": valLimitID,
		"segment_id":        valSegmentID,
		"portfolio_id":      valPortfolioX,
		"transaction_type":  "CARD",
	} {
		if got := q.Get(key); got != want {
			t.Fatalf("query %s = %q, want %q", key, got, want)
		}
	}
}

// TestValidationsFacade_PagesCursorStop chains two cursor pages, stops on an
// empty nextCursor, and asserts the cursor advances across exactly two requests.
func TestValidationsFacade_PagesCursorStop(t *testing.T) {
	page1 := `{"transactionValidations":[` + validationSummaryJSON("11111111-1111-1111-1111-111111111111") + `],"hasMore":true,"nextCursor":"c2"}`
	page2 := `{"transactionValidations":[` + validationSummaryJSON("22222222-2222-2222-2222-222222222222") + `],"hasMore":false,"nextCursor":""}`

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

	all, err := CollectAll(newTestValidationsFacade(t, srv).ListAll(context.Background(), models.ValidationsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("collected %d validations, want 2", len(all))
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", cursors)
	}
}

// TestValidationsFacade_PagesCtxCancel proves a cancelled context terminates
// iteration with the context error before any request.
func TestValidationsFacade_PagesCtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionValidations":[],"nextCursor":""}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectAll(newTestValidationsFacade(t, srv).ListAll(ctx, models.ValidationsListOpts{}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestValidationsFacade_Get decodes the full stored record, including the
// account context and the decision.
func TestValidationsFacade_Get(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(transactionValidationJSON()))
	}))
	defer srv.Close()

	rec, err := newTestValidationsFacade(t, srv).Get(context.Background(), validationID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if method != http.MethodGet || path != "/v1/validations/"+validationID {
		t.Fatalf("get req = %s %s, want GET /v1/validations/%s", method, path, validationID)
	}
	if rec.ValidationID != validationID || rec.Decision != "DENY" || rec.Account.AccountID != valAccountID {
		t.Fatalf("Get returned %+v", rec)
	}
}

// TestValidationsFacade_ValidateBeforeWire proves bad input is rejected before
// any round trip (no server contact).
func TestValidationsFacade_ValidateBeforeWire(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	f := newTestValidationsFacade(t, srv)
	acct := models.AccountContext{AccountID: valAccountID, Status: "ACTIVE", Type: "deposit"}

	bad := []struct {
		name  string
		input *models.ValidateTransactionInput
	}{
		{"zero amount", models.NewValidateTransactionInput(valRequestID, decimal.Zero, "USD", "2026-01-01T00:00:00Z", acct)},
		{"negative amount", models.NewValidateTransactionInput(valRequestID, decimal.RequireFromString("-1"), "USD", "2026-01-01T00:00:00Z", acct)},
		{"bad asset", models.NewValidateTransactionInput(valRequestID, decimal.NewFromInt(1), "US", "2026-01-01T00:00:00Z", acct)},
		{"empty requestId", models.NewValidateTransactionInput("  ", decimal.NewFromInt(1), "USD", "2026-01-01T00:00:00Z", acct)},
		{"missing account", models.NewValidateTransactionInput(valRequestID, decimal.NewFromInt(1), "USD", "2026-01-01T00:00:00Z", models.AccountContext{})},
	}
	for _, tt := range bad {
		if _, err := f.Evaluate(context.Background(), tt.input); err == nil {
			t.Fatalf("%s: want validation error before the wire", tt.name)
		}
	}
	if hit {
		t.Fatalf("validation failures must not contact the server")
	}
}

// TestValidationsFacade_Error maps a non-2xx problem+json from Get into
// *errors.Error with the server request-ID threaded through.
func TestValidationsFacade_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-val-404")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"TRACER-0020","title":"Not Found","status":404}`))
	}))
	defer srv.Close()

	_, err := newTestValidationsFacade(t, srv).Get(context.Background(), validationID)
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0020" || sdkErr.RequestID != "req-val-404" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestValidationsFacade_ListError maps a non-2xx problem+json from the LIST
// endpoint into *errors.Error — exercising List's own DecodeProblemJSON branch,
// distinct from the decodeOne path Get uses.
func TestValidationsFacade_ListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-val-list-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0021","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestValidationsFacade(t, srv).List(context.Background(), models.ValidationsListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0021" || sdkErr.RequestID != "req-val-list-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// The class is a RESPONSE DECODE error, not an internal one: the server
// answered and the SDK could not read the answer, which is a different
// fact from "the SDK is broken" and is what a caller needs in order to
// decide whether to reconcile.
// TestValidationsFacade_ListMalformedBody proves a 200 whose body is not valid
// JSON for the flat envelope surfaces as a typed response-decode error.
func TestValidationsFacade_ListMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionValidations": not-json`))
	}))
	defer srv.Close()

	_, err := newTestValidationsFacade(t, srv).List(context.Background(), models.ValidationsListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.Code != sdkerrors.CodeResponseDecode {
		t.Fatalf("error code = %q, want %q", sdkErr.Code, sdkerrors.CodeResponseDecode)
	}
}

func newTestValidationsFacade(t *testing.T, srv *httptest.Server) *validationsFacade {
	t.Helper()
	return newValidationsFacade(newTestTracerClient(t, srv))
}
