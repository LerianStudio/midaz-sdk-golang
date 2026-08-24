// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func validV2Leg(alias string) TransactionV2Leg {
	return TransactionV2Leg{
		Alias:          alias,
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		LedgerID:       "22222222-2222-2222-2222-222222222222",
		Amount:         "100",
	}
}

func validV2Input() *CreateTransactionV2Input {
	return &CreateTransactionV2Input{
		Asset:   "USD",
		Amount:  "100",
		Debits:  []TransactionV2Leg{validV2Leg("@src")},
		Credits: []TransactionV2Leg{validV2Leg("@dst")},
	}
}

// TestCreateTransactionV2Input_Validate covers the local refusals, each named by
// the obligation it enforces on the server side.
func TestCreateTransactionV2Input_Validate(t *testing.T) {
	tests := []struct {
		name      string
		mut       func(*CreateTransactionV2Input)
		wantField string
	}{
		{"valid", func(*CreateTransactionV2Input) {}, ""},
		{"no asset", func(in *CreateTransactionV2Input) { in.Asset = "" }, "asset"},
		{"no amount", func(in *CreateTransactionV2Input) { in.Amount = "" }, "amount"},
		{"negative amount", func(in *CreateTransactionV2Input) { in.Amount = "-1" }, "amount"},
		{"zero amount", func(in *CreateTransactionV2Input) { in.Amount = "0" }, "amount"},
		{"no debit side", func(in *CreateTransactionV2Input) { in.Debits = nil }, "debits"},
		{"no credit side", func(in *CreateTransactionV2Input) { in.Credits = nil }, "credits"},
		{
			"leg with no alias",
			func(in *CreateTransactionV2Input) { in.Debits[0].Alias = "" },
			"debits[0]",
		},
		{
			"leg with no scope",
			func(in *CreateTransactionV2Input) { in.Credits[0].LedgerID = "" },
			"credits[0]",
		},
		{
			"leg with both amount and share",
			func(in *CreateTransactionV2Input) { in.Debits[0].Share = &TransactionV2Share{Percentage: 50} },
			"debits[0]",
		},
		{
			"leg with neither amount nor share",
			func(in *CreateTransactionV2Input) { in.Debits[0].Amount = "" },
			"debits[0]",
		},
		{
			"share percentage of zero moves nothing",
			func(in *CreateTransactionV2Input) {
				in.Debits[0].Amount = ""
				in.Debits[0].Share = &TransactionV2Share{Percentage: 0}
			},
			"debits[0]",
		},
		{
			"share percentage above 100",
			func(in *CreateTransactionV2Input) {
				in.Debits[0].Amount = ""
				in.Debits[0].Share = &TransactionV2Share{Percentage: 101}
			},
			"debits[0]",
		},
		{
			// Zero on the narrowing factor means "do not narrow", which is the
			// only reading that lets the field be omitted at all.
			"share with a zero narrowing factor is valid",
			func(in *CreateTransactionV2Input) {
				in.Debits[0].Amount = ""
				in.Debits[0].Share = &TransactionV2Share{Percentage: 60, PercentageOfPercentage: 0}
			},
			"",
		},
		{
			"more than 500 legs on one side",
			func(in *CreateTransactionV2Input) {
				legs := make([]TransactionV2Leg, 501)
				for i := range legs {
					legs[i] = validV2Leg("@src")
				}

				in.Debits = legs
			},
			"debits",
		},
		{
			"nested metadata",
			func(in *CreateTransactionV2Input) {
				in.Metadata = map[string]any{"nested": map[string]any{"a": 1}}
			},
			"metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validV2Input()
			tt.mut(input)

			err := input.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("Validate() = nil, want a failure naming %q", tt.wantField)
			}

			if !strings.Contains(err.Error(), tt.wantField) {
				t.Fatalf("Validate() = %v, want it to name %q", err, tt.wantField)
			}
		})
	}
}

// TestCreateTransactionV2Input_WireShape pins the request body the four /v2
// create actions send.
//
// The two things that matter: the idempotency key must NOT be in the body (it is
// a header, and a body field would both leak it into the idempotency hash source
// and fail the server's strict decode), and both leg arrays must always be
// present because the server treats an absent side as a missing field.
func TestCreateTransactionV2Input_WireShape(t *testing.T) {
	input := validV2Input()
	input.IdempotencyKey = "must-not-travel-in-the-body"
	input.Description = "payout"

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, absent := range []string{"idempotencyKey", "IdempotencyKey"} {
		if _, ok := wire[absent]; ok {
			t.Fatalf("%q must not appear in the body: it is the X-Idempotency header", absent)
		}
	}

	if strings.Contains(string(raw), "must-not-travel") {
		t.Fatalf("the idempotency key leaked into the body: %s", raw)
	}

	for _, required := range []string{"asset", "amount", "debits", "credits"} {
		if _, ok := wire[required]; !ok {
			t.Fatalf("%q missing from the body: the server requires it: %s", required, raw)
		}
	}

	// A share leg must omit "amount" entirely rather than send "": the server
	// refuses a leg carrying both value expressions.
	shareInput := validV2Input()
	shareInput.Debits[0].Amount = ""
	shareInput.Debits[0].Share = &TransactionV2Share{Percentage: 100}

	raw, err = json.Marshal(shareInput)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var shareWire struct {
		Debits []map[string]any `json:"debits"`
	}

	if err := json.Unmarshal(raw, &shareWire); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := shareWire.Debits[0]["amount"]; ok {
		t.Fatalf("a share leg must not send an amount key at all: %s", raw)
	}
}

// TestTransactionV2_DropsAndKeepsTheRightFields pins the /v1 ↔ /v2 response
// divergence at the type level.
//
// This is the reason TransactionV2 is a separate type rather than an alias. A
// field /v2 does not serve would decode to its zero value forever and read as
// "empty" rather than "not served" — and route or chartOfAccountsGroupName
// coming back empty on every transaction is the kind of thing a reconciliation
// pipeline notices months later.
func TestTransactionV2_DropsAndKeepsTheRightFields(t *testing.T) {
	tests := []struct {
		typ     reflect.Type
		name    string
		absent  []string
		absentW string
		present []string
	}{
		{
			typ:     reflect.TypeOf(TransactionV2{}),
			name:    "TransactionV2",
			absent:  []string{"ChartOfAccountsGroupName", "Route", "Source", "Destination", "Template", "ExternalID", "Pending"},
			absentW: "/v2 does not serve this field; carrying it would decode as an empty value forever",
			present: []string{"FeesSkipped", "TracerSkipped", "Debit", "Credit"},
		},
		{
			typ:     reflect.TypeOf(OperationV2{}),
			name:    "OperationV2",
			absent:  []string{"ChartOfAccounts", "Route"},
			absentW: "/v2 dropped this from the nested operation shape",
			present: []string{"RouteID", "RouteCode", "RouteDescription", "BalanceAfter"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, name := range tt.absent {
				if _, ok := tt.typ.FieldByName(name); ok {
					t.Errorf("%s must not carry %s: %s", tt.name, name, tt.absentW)
				}
			}

			for _, name := range tt.present {
				if _, ok := tt.typ.FieldByName(name); !ok {
					t.Errorf("%s is missing %s, which /v2 does serve", tt.name, name)
				}
			}
		})
	}
}

// TestUpdateTransactionV2Input_Validate pins that an empty patch is refused. A
// PATCH that changes nothing still costs a round trip and an audit entry, and it
// is far more often a caller bug than an intention.
func TestUpdateTransactionV2Input_Validate(t *testing.T) {
	if err := (&UpdateTransactionV2Input{}).Validate(); err == nil {
		t.Fatal("an empty patch must be refused")
	}

	if err := (&UpdateTransactionV2Input{Description: "corrected"}).Validate(); err != nil {
		t.Fatalf("a description-only patch must be accepted, got %v", err)
	}

	if err := (&UpdateTransactionV2Input{Metadata: map[string]any{"ref": "abc"}}).Validate(); err != nil {
		t.Fatalf("a metadata-only patch must be accepted, got %v", err)
	}

	if err := (&UpdateTransactionV2Input{Metadata: map[string]any{"n": map[string]any{"a": 1}}}).Validate(); err == nil {
		t.Fatal("nested metadata must be refused")
	}
}
