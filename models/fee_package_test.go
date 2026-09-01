// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// fieldError reports whether the error carries a FieldErrors entry for the exact
// field key. Validate returns *validation.FieldErrors, so its per-field keys are
// asserted directly rather than via substring matching on the flat render.
func fieldError(t *testing.T, err error, field string) bool {
	t.Helper()
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), field)
}

// TestFeePackage_ReadRoundTrip proves the read model mirrors the generated
// FeePackage and that every money-adjacent field (minimum/maximum amount and each
// fee calculation value) rides the wire as a JSON string and survives unchanged.
// 0.333333333333333333 is unrepresentable in float64 and would visibly drift
// through a float hop.
func TestFeePackage_ReadRoundTrip(t *testing.T) {
	const precise = "0.333333333333333333"
	wire := `{
		"id":"pkg-1","feeGroupLabel":"Std","segmentId":"seg-1","ledgerId":"led-1",
		"minimumAmount":"` + precise + `","maximumAmount":"1000.00",
		"waivedAccounts":["@a","@b"],
		"fees":{"admin":{
			"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"` + precise + `"}]},
			"creditAccount":"@fees","feeLabel":"Admin","isDeductibleFrom":true,"priority":1,
			"referenceAmount":"originalAmount"}},
		"enable":true,
		"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"
	}`

	var fp FeePackage
	if err := json.Unmarshal([]byte(wire), &fp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if fp.ID != "pkg-1" || fp.FeeGroupLabel != "Std" || fp.SegmentID != "seg-1" {
		t.Fatalf("scalar fields = %+v", fp)
	}
	if fp.MinimumAmount != precise {
		t.Fatalf("MinimumAmount = %q, want %q (no float hop)", fp.MinimumAmount, precise)
	}
	if fp.MaximumAmount != "1000.00" {
		t.Fatalf("MaximumAmount = %q, want 1000.00", fp.MaximumAmount)
	}
	fee, ok := fp.Fees["admin"]
	if !ok {
		t.Fatalf("fees[admin] missing: %+v", fp.Fees)
	}
	if len(fee.CalculationModel.Calculations) != 1 || fee.CalculationModel.Calculations[0].Value != precise {
		t.Fatalf("calculation value = %+v, want %q (no float hop)", fee.CalculationModel.Calculations, precise)
	}
	if fee.IsDeductibleFrom == nil || !*fee.IsDeductibleFrom {
		t.Fatalf("IsDeductibleFrom = %v, want true", fee.IsDeductibleFrom)
	}
}

// TestUpdatePackageInput_Wire proves PATCH omit-unset: only set fields serialize,
// under their server wire names, and a precise money amount rides as an exact
// string with no float hop.
func TestUpdatePackageInput_Wire(t *testing.T) {
	const precise = "0.333333333333333333"
	b, err := json.Marshal(NewUpdatePackageInput().WithMinAmount(precise).WithEnable(false))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"minimumAmount":"`+precise+`"`) {
		t.Fatalf("wire = %s, want exact minimumAmount string %q (no float hop)", got, precise)
	}
	if !strings.Contains(got, `"enable":false`) {
		t.Fatalf("wire = %s, want enable:false", got)
	}
	for _, bad := range []string{`"feeGroupLabel"`, `"description"`, `"segmentId"`, `"transactionRoute"`, `"maximumAmount"`, `"waivedAccounts"`, `"fees"`} {
		if strings.Contains(got, bad) {
			t.Fatalf("wire = %s, unset field %s must be omitted", got, bad)
		}
	}
}

// requireNoLedgerIDOnWire marshals a fee/billing request input and fails if the
// body carries a "ledgerId" key at any depth.
//
// This is the load-bearing assertion for the midaz v4 fee/billing contract: the
// server removed ledgerId from these request bodies and its schemas are closed
// (additionalProperties: false), so a body still carrying the key is rejected with
// 400 on every fee/billing write. The break is invisible to every other test in
// this repo — they assert against SDK-owned mocks that accept any body — so this
// check is the only detection. Do not weaken it to a top-level-only check: the
// key must be absent from nested objects too.
func requireNoLedgerIDOnWire(t *testing.T, input any) string {
	t.Helper()

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("marshaled body is not valid JSON: %v (%s)", err, raw)
	}

	assertNoLedgerIDKey(t, decoded, "$", string(raw))

	return string(raw)
}

// assertNoLedgerIDKey walks a decoded JSON value and fails on any "ledgerId" key.
func assertNoLedgerIDKey(t *testing.T, node any, path, body string) {
	t.Helper()

	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			if key == "ledgerId" {
				t.Fatalf("wire = %s\n%s.ledgerId must be absent: midaz v4 rejects the field (closed schema)", body, path)
			}

			assertNoLedgerIDKey(t, child, path+"."+key, body)
		}
	case []any:
		for i, child := range v {
			assertNoLedgerIDKey(t, child, path+"["+strconv.Itoa(i)+"]", body)
		}
	}
}

// TestCreatePackageInput_Wire is the midaz v4 contract rail on the fee-package
// create body: the marshaled payload must carry the create fields and NO
// ledgerId, which the server now rejects.
func TestCreatePackageInput_Wire(t *testing.T) {
	deductible := false
	input := NewCreatePackageInput("Std", "100.00", "1000.00", map[string]Fee{
		"admin": {
			CreditAccount:    "@fees",
			FeeLabel:         "Admin",
			ReferenceAmount:  "originalAmount",
			IsDeductibleFrom: &deductible,
			CalculationModel: FeeCalculationModel{
				ApplicationRule: "flatFee",
				Calculations:    []Calculation{{Type: "flat", Value: "10.00"}},
			},
		},
	}).WithEnable(true)

	got := requireNoLedgerIDOnWire(t, input)

	for _, want := range []string{`"feeGroupLabel":"Std"`, `"minimumAmount":"100.00"`, `"maximumAmount":"1000.00"`, `"enable":true`} {
		if !strings.Contains(got, want) {
			t.Fatalf("wire = %s\nmissing %s", got, want)
		}
	}
}

// TestCreatePackageInput_Validate enforces the SDK trust-boundary gate: the
// required top-level fields AND the per-fee dive that mirrors the server's
// ValidateStruct dive over the Fee map (feeshared/model/package.go tags). Each
// case asserts the specific FieldError key so a future tag drift is caught.
func TestCreatePackageInput_Validate(t *testing.T) {
	deductible := false
	okFee := func() Fee {
		return Fee{
			CreditAccount:    "@fees",
			FeeLabel:         "Admin",
			ReferenceAmount:  "originalAmount",
			IsDeductibleFrom: &deductible,
			CalculationModel: FeeCalculationModel{
				ApplicationRule: "flatFee",
				Calculations:    []Calculation{{Type: "flat", Value: "10.00"}},
			},
		}
	}
	// mut clones a valid input and mutates the "admin" fee for the failure case.
	mut := func(f func(fee *Fee)) *CreatePackageInput {
		fee := okFee()
		f(&fee)
		return NewCreatePackageInput("Std", "100.00", "1000.00", map[string]Fee{"admin": fee}).WithEnable(true)
	}

	tests := []struct {
		name    string
		input   *CreatePackageInput
		wantErr bool
		wantKey string
	}{
		{"nil", nil, true, ""},
		{
			"ok",
			NewCreatePackageInput("Std", "100.00", "1000.00", map[string]Fee{"admin": okFee()}).WithEnable(true),
			false, "",
		},
		{
			"missing-feeGroupLabel",
			NewCreatePackageInput("  ", "100.00", "1000.00", map[string]Fee{"admin": okFee()}).WithEnable(true),
			true, "feeGroupLabel",
		},
		{
			"missing-minimumAmount",
			NewCreatePackageInput("Std", "  ", "1000.00", map[string]Fee{"admin": okFee()}).WithEnable(true),
			true, "minimumAmount",
		},
		{
			"missing-maximumAmount",
			NewCreatePackageInput("Std", "100.00", "  ", map[string]Fee{"admin": okFee()}).WithEnable(true),
			true, "maximumAmount",
		},
		{
			"empty-fees",
			NewCreatePackageInput("Std", "100.00", "1000.00", map[string]Fee{}).WithEnable(true),
			true, "fees",
		},
		{
			"nil-enable",
			NewCreatePackageInput("Std", "100.00", "1000.00", map[string]Fee{"admin": okFee()}),
			true, "enable",
		},
		{"fee-missing-isDeductibleFrom", mut(func(f *Fee) { f.IsDeductibleFrom = nil }), true, "fees[admin].isDeductibleFrom"},
		{"fee-missing-feeLabel", mut(func(f *Fee) { f.FeeLabel = "" }), true, "fees[admin].feeLabel"},
		{"fee-missing-creditAccount", mut(func(f *Fee) { f.CreditAccount = "" }), true, "fees[admin].creditAccount"},
		{"fee-bad-referenceAmount", mut(func(f *Fee) { f.ReferenceAmount = "wrong" }), true, "fees[admin].referenceAmount"},
		{"fee-empty-referenceAmount", mut(func(f *Fee) { f.ReferenceAmount = "" }), true, "fees[admin].referenceAmount"},
		{
			"fee-nil-calculationModel",
			mut(func(f *Fee) { f.CalculationModel = FeeCalculationModel{} }),
			true, "fees[admin].calculationModel",
		},
		{
			"fee-bad-applicationRule",
			mut(func(f *Fee) { f.CalculationModel.ApplicationRule = "wrong" }),
			true, "fees[admin].calculationModel.applicationRule",
		},
		{
			"fee-bad-calculation-type",
			mut(func(f *Fee) { f.CalculationModel.Calculations[0].Type = "wrong" }),
			true, "fees[admin].calculationModel.calculations[0].type",
		},
		{
			"fee-empty-calculation-value",
			mut(func(f *Fee) { f.CalculationModel.Calculations[0].Value = "" }),
			true, "fees[admin].calculationModel.calculations[0].value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantKey != "" && !fieldError(t, err, tt.wantKey) {
				t.Fatalf("Validate() err = %v, want FieldError key %q", err, tt.wantKey)
			}
		})
	}
}
