// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
)

// TestAuditEventsListDecodes covers case (a): the flat server envelope
// (top-level items + next_cursor + prev_cursor + limit + organization_id)
// decodes into ListResponse[AuditEvent] with items populated, next_cursor
// landing in Pagination, and the ignored organization_id causing no error.
func TestAuditEventsListDecodes(t *testing.T) {
	body := `{
		"organization_id":"11111111-1111-1111-1111-111111111111",
		"items":[
			{"id":"e1","action":"provision","actor":"svc","outcome":"success","reason":"init","from_status":"none","to_status":"active","timestamp":"2026-01-01T00:00:00Z","request_id":"req-1"}
		],
		"limit":20,
		"next_cursor":"c2",
		"prev_cursor":"c0"
	}`

	var list ListResponse[AuditEvent]
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(list.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(list.Items))
	}

	got := list.Items[0]
	if got.ID != "e1" || got.Action != "provision" || got.Actor != "svc" ||
		got.Outcome != "success" || got.Reason != "init" ||
		got.FromStatus != "none" || got.ToStatus != "active" ||
		got.Timestamp != "2026-01-01T00:00:00Z" || got.RequestID != "req-1" {
		t.Fatalf("AuditEvent decoded wrong: %+v", got)
	}

	if list.Pagination.NextCursor != "c2" {
		t.Fatalf("NextCursor = %q, want c2", list.Pagination.NextCursor)
	}
	if list.Pagination.PrevCursor != "c0" {
		t.Fatalf("PrevCursor = %q, want c0", list.Pagination.PrevCursor)
	}
	if list.Pagination.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", list.Pagination.Limit)
	}
	if list.Pagination.ItemCount != 1 {
		t.Fatalf("ItemCount = %d, want 1", list.Pagination.ItemCount)
	}
}

// TestAuditEventsListOpts_Validate covers case (d): Validate rejects an outcome
// outside the restricted enum and accepts the members plus empty.
func TestAuditEventsListOpts_Validate(t *testing.T) {
	rejected := []string{"conflict", "not_found"}
	for _, outcome := range rejected {
		opts := AuditEventsListOpts{Outcome: outcome}
		err := opts.Validate()
		if err == nil {
			t.Fatalf("Validate(outcome=%q) = nil, want a FieldError", outcome)
		}

		var fe *validation.FieldErrors
		if !errors.As(err, &fe) {
			t.Fatalf("Validate(outcome=%q) error type = %T, want *validation.FieldErrors", outcome, err)
		}
		found := false
		for _, item := range fe.Errs() {
			if item.Field == "outcome" {
				found = true
			}
		}
		if !found {
			t.Fatalf("Validate(outcome=%q) had no outcome FieldError: %v", outcome, err)
		}
	}

	accepted := []string{"", "success", "failure", "already_exists"}
	for _, outcome := range accepted {
		if err := (AuditEventsListOpts{Outcome: outcome}).Validate(); err != nil {
			t.Fatalf("Validate(outcome=%q) = %v, want nil", outcome, err)
		}
	}
}
