// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"reflect"
	"testing"
)

// TestVersionGroups_ZeroValueEntityIsGuardable is the reason V1 and V2 are struct
// VALUES on Entity rather than pointers.
//
// A hand-rolled zero-value &Entity{} is legal — Entity and InitServices are both
// exported — and the idiomatic guard against an uninitialized Entity is a nil
// check on the member the caller wants (pkg/generator and pkg/integrity both do
// exactly this). With pointer groups that guard PANICS on the zero value, one
// level deeper than the check can see: e.V1 is nil before e.V1.Accounts is ever
// evaluated. With value groups the members are simply nil, which is what the flat
// accessors used to give.
//
// If someone changes these fields to pointers, this test is what fails.
func TestVersionGroups_ZeroValueEntityIsGuardable(t *testing.T) {
	entity := &Entity{}

	// The guard itself must not panic — that is the whole property under test.
	if entity.V1.Accounts != nil {
		t.Fatal("zero-value Entity: V1.Accounts must be nil")
	}

	if entity.V2.Holders != nil {
		t.Fatal("zero-value Entity: V2.Holders must be nil")
	}

	for _, field := range []string{"V1", "V2"} {
		f, ok := reflect.TypeOf(Entity{}).FieldByName(field)
		if !ok {
			t.Fatalf("Entity has no field %s", field)
		}

		if f.Type.Kind() != reflect.Struct {
			t.Fatalf("Entity.%s is %s, want a struct value: a pointer group makes the "+
				"caller's nil guard panic on a zero-value Entity", field, f.Type.Kind())
		}
	}
}

// TestVersionGroups_MembershipMatchesServerSurface pins which group each resource
// lives in, because that placement is a fact about Midaz and not a preference:
// /v1 answers 404 for every V2Services member, and /v2 dropped asset rates and
// the V1 transaction creation styles.
//
// It also pins the two negative facts that are easy to undo by accident — there
// is no V2 asset-rate accessor, and no V1 accessor for a family Midaz removed
// from /v1.
func TestVersionGroups_MembershipMatchesServerSurface(t *testing.T) {
	tests := []struct {
		group   string
		typ     reflect.Type
		want    []string
		absent  []string
		absentW string
	}{
		{
			group: "V1",
			typ:   reflect.TypeOf(V1Services{}),
			want: []string{
				"Organizations", "Ledgers", "Accounts", "AccountTypes", "Assets",
				"AssetRates", "Balances", "Operations", "Portfolios", "Segments",
				"OperationRoutes", "TransactionRoutes", "Transactions", "MetadataIndexes",
			},
			absent: []string{
				"Holders", "Instruments", "Encryption", "Composition",
				"ProtectionAudit", "BillingPackages", "FeePackages", "FeeEstimates",
				"BillingCalculations",
			},
			absentW: "Midaz removed this family from /v1; a V1 accessor for it would 404",
		},
		{
			group: "V2",
			typ:   reflect.TypeOf(V2Services{}),
			want: []string{
				"Holders", "Instruments", "Encryption", "Composition",
				"ProtectionAudit", "BillingPackages", "FeePackages", "FeeEstimates",
				"BillingCalculations",
			},
			absent:  []string{"AssetRates"},
			absentW: "/v2 dropped asset rates; they are V1-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			for _, name := range tt.want {
				if _, ok := tt.typ.FieldByName(name); !ok {
					t.Errorf("%s is missing %s", tt.group, name)
				}
			}

			for _, name := range tt.absent {
				if _, ok := tt.typ.FieldByName(name); ok {
					t.Errorf("%s must not expose %s: %s", tt.group, name, tt.absentW)
				}
			}

			if got := tt.typ.NumField(); got != len(tt.want) {
				t.Errorf("%s has %d members, want exactly %d — a new accessor needs a "+
					"line in this test naming the server version that serves it",
					tt.group, got, len(tt.want))
			}
		})
	}
}
