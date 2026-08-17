// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func validEstimateSend() *SendInput {
	return &SendInput{
		Asset: "BRL",
		Value: "100.00",
		Source: &SourceInput{
			From: []FromToInput{{AccountAlias: "@source", Amount: AmountInput{Asset: "BRL", Value: "100.00"}}},
		},
		Distribute: &DistributeInput{
			To: []FromToInput{{AccountAlias: "@dest", Amount: AmountInput{Asset: "BRL", Value: "100.00"}}},
		},
	}
}

// TestFeeEstimateInput_Validate enforces the SDK trust-boundary gate: required
// package and ledger IDs and a valid send leg (the server rejects the same).
func TestFeeEstimateInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *FeeEstimateInput
		wantErr bool
		wantKey string
	}{
		{"nil", nil, true, ""},
		{
			"ok",
			&FeeEstimateInput{PackageID: "pkg-1", LedgerID: "led-1", Transaction: FeeEstimateTransactionInput{Send: validEstimateSend()}},
			false, "",
		},
		{
			"missing-packageId",
			&FeeEstimateInput{LedgerID: "led-1", Transaction: FeeEstimateTransactionInput{Send: validEstimateSend()}},
			true, "packageId",
		},
		{
			"missing-ledgerId",
			&FeeEstimateInput{PackageID: "pkg-1", Transaction: FeeEstimateTransactionInput{Send: validEstimateSend()}},
			true, "ledgerId",
		},
		{
			"nil-send",
			&FeeEstimateInput{PackageID: "pkg-1", LedgerID: "led-1"},
			true, "transaction.send",
		},
		{
			"invalid-send",
			&FeeEstimateInput{PackageID: "pkg-1", LedgerID: "led-1", Transaction: FeeEstimateTransactionInput{Send: &SendInput{}}},
			true, "transaction.send",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantKey != "" && !strings.Contains(err.Error(), tt.wantKey) {
				t.Fatalf("Validate() err = %v, want key %q", err, tt.wantKey)
			}
		})
	}
}

// TestFeeEstimateInput_Wire is the money-string rail on the estimate write path:
// a string send value must marshal as a JSON string, never a bare number. A number
// on the wire would be a float hop that drifts on values like 0.333333333333333333.
func TestFeeEstimateInput_Wire(t *testing.T) {
	const precise = "0.333333333333333333"
	send := validEstimateSend()
	send.Value = precise
	send.Source.From[0].Amount.Value = precise

	input := &FeeEstimateInput{
		PackageID:   "pkg-1",
		LedgerID:    "led-1",
		Transaction: FeeEstimateTransactionInput{Send: send},
	}

	b, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(b)

	if !strings.Contains(got, `"packageId":"pkg-1"`) || !strings.Contains(got, `"ledgerId":"led-1"`) {
		t.Fatalf("wire = %s, want packageId + ledgerId", got)
	}
	// Quoted string, not a bare number. `"value":0.333…` (no quote) would be the
	// float-hop bug this rail guards against.
	if !strings.Contains(got, `"value":"`+precise+`"`) {
		t.Fatalf("wire = %s, want send value as JSON string %q (no bare number)", got, precise)
	}
	if strings.Contains(got, `"value":`+precise) {
		t.Fatalf("wire = %s, send value must not be a bare number", got)
	}
}
