// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBillingPackage_ReadRoundTrip proves the read model mirrors the generated
// FeeBillingPackage and that every money-adjacent field (feeAmount, pricing-tier
// unitPrice, discount-tier percentage) rides the wire as a JSON string and
// survives unchanged. The value 0.333333333333333333 is unrepresentable in
// float64 and would visibly drift through a float hop.
func TestBillingPackage_ReadRoundTrip(t *testing.T) {
	const precise = "0.333333333333333333"
	wire := `{
		"id":"pkg-1","organizationId":"org-1","ledgerId":"led-1","label":"Vol",
		"type":"volume","enable":true,
		"eventFilter":{"transactionRoute":"route-1","status":"APPROVED"},
		"pricingModel":"tiered",
		"tiers":[{"minQuantity":0,"maxQuantity":100,"unitPrice":"` + precise + `"},{"minQuantity":101,"unitPrice":"2.00"}],
		"freeQuota":10,
		"discountTiers":[{"minQuantity":1000,"discountPercentage":"` + precise + `"}],
		"countMode":"perRoute","assetCode":"BRL",
		"debitAccountAlias":"@d","creditAccountAlias":"@c",
		"feeAmount":"` + precise + `",
		"maintenanceCreditAccount":"@m",
		"accountTarget":{"segmentId":"seg-1","portfolioId":"port-1","aliases":["a1","a2"]},
		"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"
	}`

	var bp BillingPackage
	if err := json.Unmarshal([]byte(wire), &bp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if bp.ID != "pkg-1" || bp.Label != "Vol" || bp.Type != "volume" {
		t.Fatalf("scalar fields = %+v", bp)
	}
	if bp.FeeAmount == nil || *bp.FeeAmount != precise {
		t.Fatalf("FeeAmount = %v, want %q (no float hop)", bp.FeeAmount, precise)
	}
	if bp.Tiers == nil || len(*bp.Tiers) != 2 {
		t.Fatalf("Tiers = %+v", bp.Tiers)
	}
	if (*bp.Tiers)[0].UnitPrice != precise {
		t.Fatalf("Tiers[0].UnitPrice = %q, want %q", (*bp.Tiers)[0].UnitPrice, precise)
	}
	if (*bp.Tiers)[0].MaxQuantity == nil || *(*bp.Tiers)[0].MaxQuantity != 100 {
		t.Fatalf("Tiers[0].MaxQuantity = %v, want 100", (*bp.Tiers)[0].MaxQuantity)
	}
	if bp.DiscountTiers == nil || (*bp.DiscountTiers)[0].DiscountPercentage != precise {
		t.Fatalf("DiscountTiers[0].DiscountPercentage = %+v, want %q", bp.DiscountTiers, precise)
	}
	if bp.EventFilter == nil || bp.EventFilter.TransactionRoute != "route-1" || bp.EventFilter.Status != "APPROVED" {
		t.Fatalf("EventFilter = %+v", bp.EventFilter)
	}
	if bp.AccountTarget == nil || bp.AccountTarget.SegmentID == nil || *bp.AccountTarget.SegmentID != "seg-1" {
		t.Fatalf("AccountTarget = %+v", bp.AccountTarget)
	}
	if bp.AccountTarget.Aliases == nil || len(*bp.AccountTarget.Aliases) != 2 {
		t.Fatalf("AccountTarget.Aliases = %+v", bp.AccountTarget)
	}
}

// TestCreateBillingPackageInput_VolumeWire proves the volume create input
// serializes byte-for-byte with the server model.BillingPackage DTO: money
// fields as strings, server-owned fields (id/organizationId/timestamps) absent.
func TestCreateBillingPackageInput_VolumeWire(t *testing.T) {
	input := NewCreateVolumeBillingPackageInput("Vol", "led-1", "BRL", "@d", "@c").
		WithEventFilter("route-1", "APPROVED").
		WithPricingModel("tiered").
		WithPricingTiers(
			BillingPricingTier{MinQuantity: 0, MaxQuantity: int64Ptr(100), UnitPrice: "1.50"},
			BillingPricingTier{MinQuantity: 101, UnitPrice: "2.00"},
		).
		WithEnable(true)

	b, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)

	for _, want := range []string{
		`"label":"Vol"`, `"ledgerId":"led-1"`, `"type":"volume"`,
		`"assetCode":"BRL"`, `"debitAccountAlias":"@d"`, `"creditAccountAlias":"@c"`,
		`"pricingModel":"tiered"`, `"unitPrice":"1.50"`, `"unitPrice":"2.00"`,
		`"transactionRoute":"route-1"`, `"status":"APPROVED"`, `"enable":true`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("wire = %s\nmissing %s", got, want)
		}
	}
	for _, bad := range []string{`"id":`, `"organizationId":`, `"createdAt":`, `"updatedAt":`, `"deletedAt":`, `"feeAmount":`} {
		if strings.Contains(got, bad) {
			t.Fatalf("wire = %s\nmust not contain %s", got, bad)
		}
	}
}

// TestCreateBillingPackageInput_MaintenanceWire proves the maintenance create
// input carries feeAmount as a string and the account target.
func TestCreateBillingPackageInput_MaintenanceWire(t *testing.T) {
	const precise = "0.333333333333333333"
	input := NewCreateMaintenanceBillingPackageInput("Maint", "led-1", "BRL", precise, "@m").
		WithAccountTarget(BillingAccountTarget{Aliases: strSlicePtr("a1", "a2")}).
		WithEnable(false)

	b, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"type":"maintenance"`) {
		t.Fatalf("wire = %s, want maintenance type", got)
	}
	if !strings.Contains(got, `"feeAmount":"`+precise+`"`) {
		t.Fatalf("wire = %s, feeAmount must be the exact string %q (no float hop)", got, precise)
	}
	if !strings.Contains(got, `"maintenanceCreditAccount":"@m"`) {
		t.Fatalf("wire = %s, want maintenanceCreditAccount", got)
	}
	if !strings.Contains(got, `"aliases":["a1","a2"]`) {
		t.Fatalf("wire = %s, want account target aliases", got)
	}
	if !strings.Contains(got, `"enable":false`) {
		t.Fatalf("wire = %s, enable=false must be emitted (required pointer)", got)
	}
}

// TestCreateBillingPackageInput_Validate enforces the SDK trust-boundary gate:
// required common fields, valid type, and type-discriminated required fields.
func TestCreateBillingPackageInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateBillingPackageInput
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", &CreateBillingPackageInput{}, true},
		{
			"volume-missing-tiers",
			NewCreateVolumeBillingPackageInput("V", "led-1", "BRL", "@d", "@c").
				WithEventFilter("r", "APPROVED").WithPricingModel("tiered").WithEnable(true),
			true,
		},
		{
			"volume-ok",
			NewCreateVolumeBillingPackageInput("V", "led-1", "BRL", "@d", "@c").
				WithEventFilter("r", "APPROVED").WithPricingModel("tiered").
				WithPricingTiers(BillingPricingTier{MinQuantity: 0, UnitPrice: "1.00"}).WithEnable(true),
			false,
		},
		{
			"maintenance-missing-feeamount",
			NewCreateMaintenanceBillingPackageInput("M", "led-1", "BRL", "", "@m").
				WithAccountTarget(BillingAccountTarget{Aliases: strSlicePtr("a")}).WithEnable(true),
			true,
		},
		{
			"maintenance-ok",
			NewCreateMaintenanceBillingPackageInput("M", "led-1", "BRL", "50.00", "@m").
				WithAccountTarget(BillingAccountTarget{Aliases: strSlicePtr("a")}).WithEnable(true),
			false,
		},
		{
			"enable-required",
			NewCreateVolumeBillingPackageInput("V", "led-1", "BRL", "@d", "@c").
				WithEventFilter("r", "APPROVED").WithPricingModel("tiered").
				WithPricingTiers(BillingPricingTier{MinQuantity: 0, UnitPrice: "1.00"}),
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestUpdateBillingPackageInput_Wire proves PATCH omit-unset: only set fields
// serialize, matching the server BillingPackageUpdate DTO (label/description/enable).
func TestUpdateBillingPackageInput_Wire(t *testing.T) {
	b, err := json.Marshal(NewUpdateBillingPackageInput().WithEnable(false))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, `"enable":false`) {
		t.Fatalf("wire = %s, want enable:false", got)
	}
	if strings.Contains(got, "label") || strings.Contains(got, "description") {
		t.Fatalf("wire = %s, unset fields must be omitted", got)
	}
}

// TestUpdateBillingPackageInput_Validate rejects an empty PATCH and a blank label.
func TestUpdateBillingPackageInput_Validate(t *testing.T) {
	if err := NewUpdateBillingPackageInput().Validate(); err == nil {
		t.Fatal("empty PATCH must fail validation")
	}
	if err := NewUpdateBillingPackageInput().WithLabel("   ").Validate(); err == nil {
		t.Fatal("blank label must fail validation")
	}
	if err := NewUpdateBillingPackageInput().WithLabel("New").Validate(); err != nil {
		t.Fatalf("valid label update: %v", err)
	}
}

func int64Ptr(v int64) *int64 { return &v }
func strSlicePtr(s ...string) *[]string {
	clone := append([]string(nil), s...)
	return &clone
}
