// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/shopspring/decimal"
)

// highPrecision is a 18-fractional-digit value that a float64 cannot hold
// exactly — the money-path canary.
const highPrecision = "100.333333333333333333"

// TestLimit_MaxAmountMoneyRoundTrip is the MONEY-PATH red. MaxAmount is a
// shopspring/decimal.Decimal, never a float. A high-precision value must survive
// marshal (CreateLimitInput → quoted-string wire) and unmarshal (wire → Limit)
// with zero loss. A float64 field would silently truncate to ~15-16 digits.
func TestLimit_MaxAmountMoneyRoundTrip(t *testing.T) {
	want := decimal.RequireFromString(highPrecision)

	// Marshal path: the create body must carry the exact quoted string.
	in := NewCreateLimitInput("daily-cap", LimitTypeDaily, want, "USD").
		WithScope(Scope{TransactionType: strPtrLimitTest("PIX")})

	body, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal CreateLimitInput: %v", err)
	}
	if !strings.Contains(string(body), `"maxAmount":"`+highPrecision+`"`) {
		t.Fatalf("create body = %s, want maxAmount quoted string %q with no loss", body, highPrecision)
	}

	// Unmarshal path: the server echo decodes back into the decimal field.
	var lim Limit
	if err := json.Unmarshal([]byte(`{"limitId":"x","name":"daily-cap","limitType":"DAILY",`+
		`"maxAmount":"`+highPrecision+`","asset":"USD","status":"DRAFT",`+
		`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`), &lim); err != nil {
		t.Fatalf("unmarshal Limit: %v", err)
	}
	if !lim.MaxAmount.Equal(want) {
		t.Fatalf("MaxAmount = %s, want %s (decimal must not lose precision)", lim.MaxAmount, want)
	}
}

// TestUsageSnapshot_MoneyDecimalRatioFloat proves usage money fields are decimal
// and UtilizationPercent is a plain float ratio.
func TestUsageSnapshot_MoneyDecimalRatioFloat(t *testing.T) {
	var snap UsageSnapshot
	if err := json.Unmarshal([]byte(`{"limitId":"L1","currentUsage":"`+highPrecision+`",`+
		`"limitAmount":"200.5","utilizationPercent":50.04,"nearLimit":true}`), &snap); err != nil {
		t.Fatalf("unmarshal UsageSnapshot: %v", err)
	}
	if !snap.CurrentUsage.Equal(decimal.RequireFromString(highPrecision)) {
		t.Fatalf("CurrentUsage = %s, want %s", snap.CurrentUsage, highPrecision)
	}
	if !snap.LimitAmount.Equal(decimal.RequireFromString("200.5")) {
		t.Fatalf("LimitAmount = %s, want 200.5", snap.LimitAmount)
	}
	if snap.UtilizationPercent != 50.04 || !snap.NearLimit {
		t.Fatalf("snapshot = %+v", snap)
	}
}

// TestUpdateLimitInput_OmitsImmutableFields is the immutable-field red. The
// server rejects an update body containing limitType or asset with a 400. The
// PATCH body MUST never carry either key regardless of which fields are set.
func TestUpdateLimitInput_OmitsImmutableFields(t *testing.T) {
	up := NewUpdateLimitInput().
		WithName("renamed").
		WithMaxAmount(decimal.RequireFromString("999.99")).
		WithScopes([]Scope{{TransactionType: strPtrLimitTest("PIX")}})

	body, err := json.Marshal(up)
	if err != nil {
		t.Fatalf("marshal UpdateLimitInput: %v", err)
	}
	s := string(body)
	if strings.Contains(s, "limitType") {
		t.Fatalf("update body = %s, must NOT contain limitType (immutable)", s)
	}
	if strings.Contains(s, "asset") {
		t.Fatalf("update body = %s, must NOT contain asset (immutable)", s)
	}
	if !strings.Contains(s, `"name"`) || !strings.Contains(s, `"maxAmount"`) {
		t.Fatalf("update body = %s, want the set fields present", s)
	}
}

// TestUpdateLimitInput_EmptyRejected proves a no-op PATCH is rejected before the
// wire (mirrors the server IsEmpty → ErrNothingToUpdate probe).
func TestUpdateLimitInput_EmptyRejected(t *testing.T) {
	if err := NewUpdateLimitInput().Validate(); err == nil {
		t.Fatalf("empty update payload must be rejected")
	}
}

// TestCreateLimitInput_Validate covers the closed-enum + money + asset +
// scope-count preconditions.
func TestCreateLimitInput_Validate(t *testing.T) {
	good := Scope{TransactionType: strPtrLimitTest("PIX")}
	pos := decimal.RequireFromString("100")

	tests := []struct {
		name    string
		input   *CreateLimitInput
		wantErr bool
	}{
		{"valid", NewCreateLimitInput("cap", LimitTypeDaily, pos, "USD").WithScope(good), false},
		{"bad limit type", NewCreateLimitInput("cap", "HOURLY", pos, "USD").WithScope(good), true},
		{"zero max amount", NewCreateLimitInput("cap", LimitTypeDaily, decimal.Zero, "USD").WithScope(good), true},
		{"negative max amount", NewCreateLimitInput("cap", LimitTypeDaily, decimal.RequireFromString("-1"), "USD").WithScope(good), true},
		{"two-char asset", NewCreateLimitInput("cap", LimitTypeDaily, pos, "US").WithScope(good), true},
		{"no scopes", NewCreateLimitInput("cap", LimitTypeDaily, pos, "USD"), true},
		{"empty name", NewCreateLimitInput("  ", LimitTypeDaily, pos, "USD").WithScope(good), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestLimitsListOpts_RejectsDateFilter is the silent-drop red. The generated
// ListLimitsParams carries NO start_date/end_date slot, so a well-formed date
// range set on LimitsListOpts would pass the shared cursor validation and then
// be SILENTLY DROPPED at param-mapping time — the server returns the FULL
// unfiltered set. Validate MUST reject any date filter loudly with a typed
// validation error, while the base cursor checks (limit bounds, sort
// direction) keep behaving.
func TestLimitsListOpts_RejectsDateFilter(t *testing.T) {
	tests := []struct {
		name    string
		opts    LimitsListOpts
		wantErr bool
	}{
		{"no dates is valid", LimitsListOpts{}, false},
		{"valid non-date opts pass", LimitsListOpts{CursorListOpts: CursorListOpts{Limit: 50, SortDirection: SortDescending}}, false},
		{"end date rejected", LimitsListOpts{CursorListOpts: CursorListOpts{EndDate: "2026-01-31"}}, true},
		{"start date rejected", LimitsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01"}}, true},
		{"both dates rejected", LimitsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-01-31"}}, true},
		{"limit over max still rejected", LimitsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}}, true},
		{"bad sort direction still rejected", LimitsListOpts{CursorListOpts: CursorListOpts{SortDirection: "weird"}}, true},
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
			if !errors.As(err, &sdkErr) {
				t.Fatalf("error type = %T, want *errors.Error", err)
			}
			if sdkErr.Code != sdkerrors.CodeValidation {
				t.Fatalf("error code = %q, want %q", sdkErr.Code, sdkerrors.CodeValidation)
			}
		})
	}
}

func strPtrLimitTest(s string) *string { return &s }
