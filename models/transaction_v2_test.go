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

// crossLedgerDirectGroupJSON is the 201 the Midaz ledger served to a direct create
// whose debit and credit legs named two ledgers of one organization. It is kept
// byte for byte, extra fields included (order, fees, per-transaction groupId),
// so the decode is pinned against what the server really sends.
const crossLedgerDirectGroupJSON = `{"groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","transactions":[{"id":"01a2193f-9542-7ebb-94b2-4193580ca9c3","groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","description":"probe 9271","status":{"code":"CREATED","description":"CREATED"},"amount":"10","assetCode":"BRL","debit":["@alice"],"credit":["@external/BRL"],"ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013686Z","deletedAt":null,"operations":[{"id":"01a2193f-9566-7c10-b8d5-3db52aeecc29","transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","description":"probe 9271","type":"DEBIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"balanceAfter":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e84-70ee-92e6-fe2ec819a38b","accountAlias":"@alice","balanceKey":"default","balanceId":"01a2193f-6e93-79a1-a512-80e5f0d1d841","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013839Z","deletedAt":null,"metadata":null},{"id":"01a2193f-9566-7c13-aa31-dcd5afed82a8","transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","description":"probe 9271","type":"CREDIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"100","onHold":"0","version":1,"overdraftUsed":"0"},"balanceAfter":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e34-7a8c-9235-fc2988f68047","accountAlias":"@external/BRL","balanceKey":"default","balanceId":"01a2193f-6e4a-73e4-b01b-768e60623f63","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"credit","createdAt":"2026-11-26T19:05:27.107013548Z","updatedAt":"2026-11-26T19:05:27.107013839Z","deletedAt":null,"metadata":null}],"order":1,"fees":{"transactionId":"01a2193f-9542-7ebb-94b2-4193580ca9c3","recorded":false,"available":true,"items":[]}},{"id":"01a2193f-9543-703d-8df0-70a1341b5fab","groupId":"01a2193f-9540-799f-8c22-3a988eef1aff","description":"probe 9271","status":{"code":"CREATED","description":"CREATED"},"amount":"10","assetCode":"BRL","debit":["@external/BRL"],"credit":["@bob"],"ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107066968Z","deletedAt":null,"operations":[{"id":"01a2193f-9566-7c2a-8a84-caba3300971f","transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","description":"probe 9271","type":"DEBIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"10","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e67-7d00-896e-5f34df0b065b","accountAlias":"@external/BRL","balanceKey":"default","balanceId":"01a2193f-6e6b-7e0e-9a77-f732a31ae9bb","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107067063Z","deletedAt":null,"metadata":null},{"id":"01a2193f-9566-7c2e-a923-01a83bd89040","transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","description":"probe 9271","type":"CREDIT","assetCode":"BRL","amount":{"value":"10"},"balance":{"available":"0","onHold":"0","version":0,"overdraftUsed":"0"},"balanceAfter":{"available":"10","onHold":"0","version":1,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6eab-7b0e-9920-f573ee27b927","accountAlias":"@bob","balanceKey":"default","balanceId":"01a2193f-6eb6-7226-8565-513257b19816","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d26-721b-b604-464fa60c7e43","balanceAffected":true,"direction":"credit","createdAt":"2026-11-26T19:05:27.107066874Z","updatedAt":"2026-11-26T19:05:27.107067063Z","deletedAt":null,"metadata":null}],"order":2,"fees":{"transactionId":"01a2193f-9543-703d-8df0-70a1341b5fab","recorded":false,"available":true,"items":[]}}]}`

// crossLedgerHoldGroupJSON is the 201 the same server served to a hold create
// with the same two ledgers: one PENDING transaction, the part the hold reserved.
const crossLedgerHoldGroupJSON = `{"groupId":"01a2193f-95db-7225-ae05-ec34be8f8cec","transactions":[{"id":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","groupId":"01a2193f-95db-7225-ae05-ec34be8f8cec","description":"probe 9271","status":{"code":"PENDING","description":"PENDING"},"amount":"7","assetCode":"BRL","debit":["@alice"],"credit":["@external/BRL"],"ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","feesSkipped":false,"tracerSkipped":false,"createdAt":"2026-11-26T19:05:27.267881135Z","updatedAt":"2026-11-26T19:05:27.267881568Z","deletedAt":null,"operations":[{"id":"01a2193f-95ec-781c-aed3-4763b49c91c5","transactionId":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","description":"probe 9271","type":"ON_HOLD","assetCode":"BRL","amount":{"value":"7"},"balance":{"available":"90","onHold":"0","version":2,"overdraftUsed":"0"},"balanceAfter":{"available":"83","onHold":"7","version":3,"overdraftUsed":"0"},"status":{"code":"","description":null},"accountId":"01a2193f-6e84-70ee-92e6-fe2ec819a38b","accountAlias":"@alice","balanceKey":"default","balanceId":"01a2193f-6e93-79a1-a512-80e5f0d1d841","organizationId":"01a2193f-4ce6-753d-8673-2712abdbfe5b","ledgerId":"01a2193f-4d0c-7ba5-bb53-42d79cf1701d","balanceAffected":true,"direction":"debit","createdAt":"2026-11-26T19:05:27.267881135Z","updatedAt":"2026-11-26T19:05:27.267881928Z","deletedAt":null,"metadata":null}],"order":1,"fees":{"transactionId":"01a2193f-95e3-7bd4-8e69-bfd96d929c35","recorded":false,"available":true,"items":[]}}]}`

// TestCrossLedgerTransactionGroupV2_DecodesServedGroups pins the decode of the
// two cross-ledger group shapes the server serves: a direct group with one
// CREATED transaction per ledger, and a hold group with only the PENDING part.
// Every expectation is read off the served JSON, so a field that stops decoding
// fails here instead of surfacing as an empty value in a caller.
func TestCrossLedgerTransactionGroupV2_DecodesServedGroups(t *testing.T) {
	const (
		ledgerA = "01a2193f-4d0c-7ba5-bb53-42d79cf1701d"
		ledgerB = "01a2193f-4d26-721b-b604-464fa60c7e43"
	)

	tests := []struct {
		name    string
		body    string
		groupID string
		want    []servedTransaction
	}{
		{
			name:    "direct group",
			body:    crossLedgerDirectGroupJSON,
			groupID: "01a2193f-9540-799f-8c22-3a988eef1aff",
			want: []servedTransaction{
				{id: "01a2193f-9542-7ebb-94b2-4193580ca9c3", ledgerID: ledgerA, status: "CREATED"},
				{id: "01a2193f-9543-703d-8df0-70a1341b5fab", ledgerID: ledgerB, status: "CREATED"},
			},
		},
		{
			name:    "hold group",
			body:    crossLedgerHoldGroupJSON,
			groupID: "01a2193f-95db-7225-ae05-ec34be8f8cec",
			want: []servedTransaction{
				{id: "01a2193f-95e3-7bd4-8e69-bfd96d929c35", ledgerID: ledgerA, status: "PENDING"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var group CrossLedgerTransactionGroupV2
			if err := json.Unmarshal([]byte(tt.body), &group); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if group.GroupID != tt.groupID {
				t.Fatalf("GroupID = %q, want %q", group.GroupID, tt.groupID)
			}

			if len(group.Transactions) != len(tt.want) {
				t.Fatalf("len(Transactions) = %d, want %d", len(group.Transactions), len(tt.want))
			}

			for i, want := range tt.want {
				checkServedTransaction(t, i, group.Transactions[i], want)
			}
		})
	}
}

// servedTransaction is what one transaction of a served group must decode to.
type servedTransaction struct {
	id       string
	ledgerID string
	status   string
}

// checkServedTransaction reports every field of the transaction at index i
// that differs from the served one.
func checkServedTransaction(t *testing.T, i int, got TransactionV2, want servedTransaction) {
	t.Helper()

	if got.ID != want.id {
		t.Errorf("Transactions[%d].ID = %q, want %q (served order)", i, got.ID, want.id)
	}

	if got.LedgerID != want.ledgerID {
		t.Errorf("Transactions[%d].LedgerID = %q, want %q", i, got.LedgerID, want.ledgerID)
	}

	if got.Status.Code != want.status {
		t.Errorf("Transactions[%d].Status.Code = %q, want %q", i, got.Status.Code, want.status)
	}
}
