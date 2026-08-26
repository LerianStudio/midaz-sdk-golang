// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import "testing"

// TestAuditEventRecordsListOpts_DatesMustBeRFC3339 pins the tracer contract: the
// audit-events list server strict-parses start_date/end_date as RFC3339
// (midaz components/tracer/.../audit_event_validation.go:296, time.Parse(time.RFC3339, ...)),
// NOT the ledger plane's YYYY-MM-DD. So an RFC3339 range is accepted, a bare
// YYYY-MM-DD value is rejected before the wire, and an inverted RFC3339 range is
// rejected. Shared limit/sort checks still apply. This is the same validator
// (ValidateCursorListOptsRFC3339Dates) the validations list uses.
func TestAuditEventRecordsListOpts_DatesMustBeRFC3339(t *testing.T) {
	tests := []struct {
		name    string
		opts    AuditEventRecordsListOpts
		wantErr bool
	}{
		{"empty is valid", AuditEventRecordsListOpts{}, false},
		{"RFC3339 range accepted", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01T00:00:00Z", EndDate: "2026-01-31T23:59:59Z"}}, false},
		{"RFC3339 start only accepted", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01T00:00:00Z"}}, false},
		{"YYYY-MM-DD start rejected", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-01-01"}}, true},
		{"YYYY-MM-DD end rejected", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{EndDate: "2026-01-31"}}, true},
		{"inverted RFC3339 range rejected", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{StartDate: "2026-02-01T00:00:00Z", EndDate: "2026-01-01T00:00:00Z"}}, true},
		{"limit over max rejected", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{Limit: MaxLimit + 1}}, true},
		{"bad sort direction rejected", AuditEventRecordsListOpts{CursorListOpts: CursorListOpts{SortDirection: "weird"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.opts.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
