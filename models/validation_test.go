// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/shopspring/decimal"
)

// validationMoney is an 18-fractional-digit value a float64 cannot hold exactly.
const validationMoney = "100.333333333333333333"

func testAccount() AccountContext {
	return AccountContext{AccountID: "acct-1", Status: "ACTIVE", Type: "deposit"}
}

// TestValidateTransactionInput_MoneyIsQuotedString is the MONEY-PATH red: Amount
// is a shopspring/decimal.Decimal, and the request body must carry it as a quoted
// string (swaggertype:"string" on the server) with zero precision loss. A float64
// field would truncate to ~15-16 digits and emit an unquoted number.
func TestValidateTransactionInput_MoneyIsQuotedString(t *testing.T) {
	in := NewValidateTransactionInput("req-1", decimal.RequireFromString(validationMoney), "USD", "2026-01-01T00:00:00Z", testAccount())

	body, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal ValidateTransactionInput: %v", err)
	}
	if !strings.Contains(string(body), `"amount":"`+validationMoney+`"`) {
		t.Fatalf("body = %s, want amount quoted string %q with no loss", body, validationMoney)
	}
}

// TestLimitUsageDetail_MoneyTripleRoundTrip proves the money triple decodes into
// exact decimals and the outcome flags decode alongside.
func TestLimitUsageDetail_MoneyTripleRoundTrip(t *testing.T) {
	var d LimitUsageDetail
	if err := json.Unmarshal([]byte(`{"limitId":"L1","limitAmount":"`+validationMoney+`",`+
		`"currentUsage":"200.5","attemptedAmount":"0.000000001","exceeded":true,`+
		`"period":"DAILY","scope":"account"}`), &d); err != nil {
		t.Fatalf("unmarshal LimitUsageDetail: %v", err)
	}
	if d.LimitAmount.String() != validationMoney {
		t.Fatalf("LimitAmount = %s, want %s", d.LimitAmount, validationMoney)
	}
	if !d.CurrentUsage.Equal(decimal.RequireFromString("200.5")) || !d.AttemptedAmount.Equal(decimal.RequireFromString("0.000000001")) {
		t.Fatalf("triple lost precision: %+v", d)
	}
	if !d.Exceeded {
		t.Fatalf("Exceeded = false, want true")
	}
}

// TestValidateTransactionInput_Builders proves the fluent builders set every
// optional context and field, and that the optional contexts marshal under their
// wire keys while unset ones stay absent.
func TestValidateTransactionInput_Builders(t *testing.T) {
	in := NewValidateTransactionInput("req-1", decimal.NewFromInt(10), "USD", "2026-01-01T00:00:00Z", testAccount()).
		WithSegment(SegmentContext{SegmentID: "seg-1"}).
		WithPortfolio(PortfolioContext{PortfolioID: "pf-1"}).
		WithMerchant(MerchantContext{MerchantID: "m-1", Category: "5411", Country: "BR", Name: "Padaria"}).
		WithTransactionType("PIX").
		WithSubType("INSTANT").
		WithMetadata(map[string]any{"channel": "mobile"})

	if in.Segment == nil || in.Portfolio == nil || in.Merchant == nil ||
		in.TransactionType == nil || in.SubType == nil || in.Metadata == nil {
		t.Fatalf("builders left an optional field unset: %+v", in)
	}

	body, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(body)
	for _, key := range []string{`"segmentId":"seg-1"`, `"portfolioId":"pf-1"`, `"merchantId":"m-1"`, `"transactionType":"PIX"`, `"subType":"INSTANT"`, `"channel":"mobile"`} {
		if !strings.Contains(s, key) {
			t.Fatalf("body missing %s: %s", key, s)
		}
	}

	// A bare input omits the optional contexts entirely.
	bare, err := json.Marshal(NewValidateTransactionInput("req-2", decimal.NewFromInt(1), "USD", "2026-01-01T00:00:00Z", testAccount()))
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}
	if strings.Contains(string(bare), "segment") || strings.Contains(string(bare), "merchant") {
		t.Fatalf("bare input leaked an unset optional context: %s", bare)
	}
}

// TestValidateTransactionInput_Validate covers the required-field preconditions:
// RequestID non-empty, Amount > 0, Currency exactly 3, timestamp non-empty,
// Account present.
func TestValidateTransactionInput_Validate(t *testing.T) {
	acct := testAccount()
	pos := decimal.NewFromInt(1)

	tests := []struct {
		name    string
		input   *ValidateTransactionInput
		wantErr bool
	}{
		{"valid", NewValidateTransactionInput("req-1", pos, "USD", "2026-01-01T00:00:00Z", acct), false},
		{"empty requestId", NewValidateTransactionInput("  ", pos, "USD", "2026-01-01T00:00:00Z", acct), true},
		{"zero amount", NewValidateTransactionInput("req-1", decimal.Zero, "USD", "2026-01-01T00:00:00Z", acct), true},
		{"negative amount", NewValidateTransactionInput("req-1", decimal.RequireFromString("-1"), "USD", "2026-01-01T00:00:00Z", acct), true},
		{"two-char currency", NewValidateTransactionInput("req-1", pos, "US", "2026-01-01T00:00:00Z", acct), true},
		{"empty timestamp", NewValidateTransactionInput("req-1", pos, "USD", "  ", acct), true},
		{"missing account", NewValidateTransactionInput("req-1", pos, "USD", "2026-01-01T00:00:00Z", AccountContext{}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.input.Validate() != nil) != tt.wantErr {
				t.Fatalf("Validate() wantErr = %v", tt.wantErr)
			}
		})
	}

	if (&ValidateTransactionInput{}).Validate() == nil {
		t.Fatalf("zero-value input must be rejected")
	}
	var nilInput *ValidateTransactionInput
	if nilInput.Validate() == nil {
		t.Fatalf("nil input must be rejected")
	}
}

// TestValidationsListOpts_DatesMustBeRFC3339 pins the tracer contract: the
// validations-list server strict-parses start_date/end_date as RFC3339
// (midaz components/tracer/.../transaction_validation_handler.go:355,
// time.Parse(time.RFC3339, ...) with a 400 on failure), NOT the ledger plane's
// YYYY-MM-DD. So an RFC3339 range is accepted, a bare YYYY-MM-DD value is
// rejected before the wire (the SDK's ONLY previously-accepted format the server
// would 400), and an inverted RFC3339 range is rejected. Shared limit/sort
// checks still apply.
func TestValidationsListOpts_DatesMustBeRFC3339(t *testing.T) {
	tests := []struct {
		name    string
		opts    ValidationsListOpts
		wantErr bool
	}{
		{"empty is valid", ValidationsListOpts{}, false},
		{"RFC3339 range accepted", ValidationsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01T00:00:00Z", EndDate: "2026-01-31T23:59:59Z"}}, false},
		{"RFC3339 start only accepted", ValidationsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01T00:00:00Z"}}, false},
		{"YYYY-MM-DD start rejected", ValidationsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01"}}, true},
		{"YYYY-MM-DD end rejected", ValidationsListOpts{CursorListOpts: CursorListOpts{EndDate: "2026-01-31"}}, true},
		{"inverted RFC3339 range rejected", ValidationsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-02-01T00:00:00Z", EndDate: "2026-01-01T00:00:00Z"}}, true},
		{"limit over max rejected", ValidationsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}}, true},
		{"bad sort direction rejected", ValidationsListOpts{CursorListOpts: CursorListOpts{SortDirection: "weird"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				return
			}
			var sdkErr *sdkerrors.Error
			if !errors.As(err, &sdkErr) || sdkErr.Code != sdkerrors.CodeValidation {
				t.Fatalf("want *errors.Error code %q, got %v", sdkerrors.CodeValidation, err)
			}
		})
	}
}
