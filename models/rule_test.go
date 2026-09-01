// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// TestRule_UnmarshalWireShape proves the server wire (ruleId identity key,
// camelCase fields) maps into the SDK struct — ID reads from "ruleId", not "id".
func TestRule_UnmarshalWireShape(t *testing.T) {
	const wire = `{
		"ruleId":"rule-123","name":"block-high-value","expression":"transaction.amount > 1000",
		"action":"DENY","status":"ACTIVE","scopes":[{"transactionType":"PIX"}],
		"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z",
		"activatedAt":"2026-01-02T00:00:00Z"
	}`

	var r Rule
	if err := json.Unmarshal([]byte(wire), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if r.ID != "rule-123" {
		t.Fatalf("ID = %q, want rule-123 (must read the ruleId wire key)", r.ID)
	}
	if r.Action != "DENY" || r.Status != "ACTIVE" {
		t.Fatalf("rule = %+v", r)
	}
	if len(r.Scopes) != 1 || r.Scopes[0].TransactionType == nil || *r.Scopes[0].TransactionType != "PIX" {
		t.Fatalf("scopes = %+v", r.Scopes)
	}
	if r.ActivatedAt == nil {
		t.Fatalf("ActivatedAt should be set")
	}
}

// TestCreateRuleInput_Validate covers the closed action enum plus the required
// non-empty Name/Expression bounds.
func TestCreateRuleInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateRuleInput
		wantErr bool
	}{
		{
			name:    "valid",
			input:   NewCreateRuleInput("r", "amount > 1", RuleActionReview),
			wantErr: false,
		},
		{
			name:    "bad action rejected",
			input:   NewCreateRuleInput("r", "amount > 1", "MAYBE"),
			wantErr: true,
		},
		{
			name:    "empty expression rejected",
			input:   NewCreateRuleInput("r", "   ", RuleActionAllow),
			wantErr: true,
		},
		{
			name:    "empty name rejected",
			input:   NewCreateRuleInput("", "amount > 1", RuleActionAllow),
			wantErr: true,
		},
		{
			name:    "too many scopes rejected",
			input:   NewCreateRuleInput("r", "amount > 1", RuleActionAllow).WithScopes(make([]Scope, maxRuleScopes+1)),
			wantErr: true,
		},
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

// TestCreateRuleInput_ExpressionIsOpaque proves the CEL expression passes through
// verbatim on the wire — the SDK never parses or rewrites it.
func TestCreateRuleInput_ExpressionIsOpaque(t *testing.T) {
	const cel = `transaction.amount > 1000 && account.type in ["CHECKING","SAVINGS"]`

	body, err := json.Marshal(NewCreateRuleInput("r", cel, RuleActionDeny))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var sent struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("unmarshal sent: %v", err)
	}
	if sent.Expression != cel {
		t.Fatalf("expression = %q, want verbatim %q", sent.Expression, cel)
	}
}

// TestUpdateRuleInput_OmitUnset proves the PATCH body carries only fields the
// caller set — an unset field is absent, never a zero value that would clobber
// the server side.
func TestUpdateRuleInput_OmitUnset(t *testing.T) {
	body, err := json.Marshal(NewUpdateRuleInput().WithAction(RuleActionAllow))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(body)
	if !strings.Contains(got, `"action"`) {
		t.Fatalf("update body missing set field action: %s", got)
	}
	for _, absent := range []string{`"name"`, `"expression"`, `"description"`, `"scopes"`} {
		if strings.Contains(got, absent) {
			t.Fatalf("update body should omit unset field %s: %s", absent, got)
		}
	}
}

// TestRulesListOpts_RejectsDateFilter is the silent-drop red. The generated
// ListRulesParams carries NO start_date/end_date slot, so a well-formed date
// range set on RulesListOpts would pass the shared cursor validation and then
// be SILENTLY DROPPED at param-mapping time — the server returns the FULL
// unfiltered set. Validate MUST reject any date filter loudly with a typed
// validation error, while the base cursor checks (limit bounds, sort
// direction) keep behaving.
func TestRulesListOpts_RejectsDateFilter(t *testing.T) {
	tests := []struct {
		name    string
		opts    RulesListOpts
		wantErr bool
	}{
		{"no dates is valid", RulesListOpts{}, false},
		{"valid non-date opts pass", RulesListOpts{CursorListOpts: CursorListOpts{Limit: 50, SortDirection: SortAscending}}, false},
		{"start date rejected", RulesListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01"}}, true},
		{"end date rejected", RulesListOpts{CursorListOpts: CursorListOpts{EndDate: "2026-01-31"}}, true},
		{"both dates rejected", RulesListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01", EndDate: "2026-01-31"}}, true},
		{"limit over max still rejected", RulesListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}}, true},
		{"bad sort direction still rejected", RulesListOpts{CursorListOpts: CursorListOpts{SortDirection: "weird"}}, true},
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

// TestUpdateRuleInput_RejectsNoOp proves an empty PATCH is rejected (mirrors the
// server ErrNothingToUpdate probe).
func TestUpdateRuleInput_RejectsNoOp(t *testing.T) {
	if !NewUpdateRuleInput().isEmpty() {
		t.Fatalf("fresh update input should be empty")
	}
	if err := NewUpdateRuleInput().Validate(); err == nil {
		t.Fatalf("empty PATCH should be rejected")
	}
	if err := NewUpdateRuleInput().WithName("x").Validate(); err != nil {
		t.Fatalf("non-empty PATCH should validate: %v", err)
	}
}
