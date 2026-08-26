// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"testing"
)

// TestBillingCalculateResponse_MoneyRoundTrip locks the money third rail: the
// server marshals TotalNetAmount as a JSON string (decimal.Decimal with
// swaggertype:"string"). Decoding into the model and re-encoding must preserve
// the exact string — a value like 0.333333333333333333 is unrepresentable in
// float64 and would drift on any float hop.
func TestBillingCalculateResponse_MoneyRoundTrip(t *testing.T) {
	const precise = "0.333333333333333333"
	wire := `{"results":[{"billingPackageId":"pkg-1","billingPackageLabel":"L","billingType":"volume",` +
		`"period":"2026-01","totalAccounts":500,"totalCharged":480,"totalSkipped":20,` +
		`"totalNetAmount":"` + precise + `","transactionPayload":{"k":"v"}}],` +
		`"summary":{"totalResults":1,"totalVolume":1,"totalMaintenance":0,"totalNetAmount":"` + precise + `"}}`

	var resp BillingCalculateResponse
	if err := json.Unmarshal([]byte(wire), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("Results len = %d, want 1", len(resp.Results))
	}
	r0 := resp.Results[0]
	if r0.TotalNetAmount != precise {
		t.Fatalf("result TotalNetAmount = %q, want %q", r0.TotalNetAmount, precise)
	}
	if r0.TotalAccounts != 500 || r0.TotalCharged != 480 || r0.TotalSkipped != 20 {
		t.Fatalf("counts = %+v", r0)
	}
	if resp.Summary.TotalNetAmount != precise {
		t.Fatalf("summary TotalNetAmount = %q, want %q", resp.Summary.TotalNetAmount, precise)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The exact string must survive the round trip with no precision loss.
	if !json.Valid(out) || !containsSub(string(out), `"totalNetAmount":"`+precise+`"`) {
		t.Fatalf("re-marshaled = %s, want exact %q preserved", out, precise)
	}
}

// TestBillingCalculateInput_Validate covers the required-field guards.
func TestBillingCalculateInput_Validate(t *testing.T) {
	if err := (&BillingCalculateInput{LedgerID: "l", Period: "2026-01"}).Validate(); err != nil {
		t.Fatalf("valid input errored: %v", err)
	}
	if err := (&BillingCalculateInput{Period: "2026-01"}).Validate(); err == nil {
		t.Fatal("missing ledgerId must fail")
	}
	if err := (&BillingCalculateInput{LedgerID: "l"}).Validate(); err == nil {
		t.Fatal("missing period must fail")
	}
	if err := (*BillingCalculateInput)(nil).Validate(); err == nil {
		t.Fatal("nil input must fail")
	}
	// Type is optional but closed-set when present: empty calculates all types.
	if err := (&BillingCalculateInput{LedgerID: "l", Period: "2026-01", Type: BillingPackageTypeVolume}).Validate(); err != nil {
		t.Fatalf("valid volume type errored: %v", err)
	}
	if err := (&BillingCalculateInput{LedgerID: "l", Period: "2026-01", Type: BillingPackageTypeMaintenance}).Validate(); err != nil {
		t.Fatalf("valid maintenance type errored: %v", err)
	}
	if err := (&BillingCalculateInput{LedgerID: "l", Period: "2026-01", Type: ""}).Validate(); err != nil {
		t.Fatalf("empty type must pass (calculates all): %v", err)
	}
	if err := (&BillingCalculateInput{LedgerID: "l", Period: "2026-01", Type: "bogus"}).Validate(); err == nil {
		t.Fatal("invalid type must fail")
	}
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
