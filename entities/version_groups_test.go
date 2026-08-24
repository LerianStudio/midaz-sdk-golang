// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"go/ast"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
				"Organizations", "Ledgers", "Accounts", "AccountTypes", "Assets",
				"Balances", "Operations", "Portfolios", "Segments",
				"OperationRoutes", "TransactionRoutes", "Transactions", "MetadataIndexes",
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

// TestV2HasOneSpellingPerEndpoint pins the de-duplication decision the V2 surface
// was built on, because it is the kind of thing a later contributor undoes by
// being helpful.
//
// Three account-scoped endpoints — an account's balances, its operations, its
// balances at an instant — are reachable on /v1 through BOTH V1.Accounts and
// V1.Balances / V1.Operations. Both spellings work and both are tested; they
// exist because they were written at different times, and every fix to one has
// to be remembered on the other. The point-in-time read is the proof that this
// costs something real: the two /v1 spellings enforced DIFFERENT date contracts
// until Epic 2 reconciled them, so the same wire call answered "now" through one
// accessor and a past instant through the other.
//
// V2 has one spelling per endpoint, and this asserts it by ENDPOINT rather than
// by method name. An earlier version listed the method names that must not come
// back — ListBalances, ListOperations, BalancesAtTimestamp — which a helpful
// contributor walks straight past by choosing a different name for the same
// wire call. A generated operation is one HTTP method and one path, so counting
// the V2 facades that reach each one is counting the spellings of an endpoint;
// re-adding an account's balance read to V2.Accounts fails here whatever it is
// called.
func TestV2HasOneSpellingPerEndpoint(t *testing.T) {
	fset := token.NewFileSet()

	// operation -> the V2 facade types that reach it.
	owners := map[string]map[string]bool{}

	for _, file := range parseGoFiles(t, fset, ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			owner, ok := v2FacadeReceiver(fn)
			if !ok {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				op, ok := generatedOperation(sel.Sel.Name)
				if !ok || !strings.HasSuffix(op, "V2") {
					return true
				}

				if owners[op] == nil {
					owners[op] = map[string]bool{}
				}

				owners[op][owner] = true

				return true
			})
		}
	}

	require.NotEmpty(t, owners, "found no V2 operations reached by a V2 facade; the scan is broken, not the code")

	var doubled []string

	for op, types := range owners {
		if len(types) > 1 {
			doubled = append(doubled, op+" is reached by "+strings.Join(sortedKeys(types), " and "))
		}
	}

	sort.Strings(doubled)

	require.Empty(t, doubled,
		"every /v2 endpoint must have exactly one spelling; two facades reaching one operation is the "+
			"/v1 drift this surface was built to avoid:\n  %s",
		strings.Join(doubled, "\n  "))

	t.Logf("one spelling each across %d V2 operations", len(owners))
}

// v2FacadeReceiver returns the V2 facade type a method hangs off, if it is one.
func v2FacadeReceiver(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false
	}

	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}

	ident, ok := recv.(*ast.Ident)
	if !ok || !strings.HasSuffix(ident.Name, "V2Facade") {
		return "", false
	}

	return ident.Name, true
}

// TestV2OwnsTheDeDuplicatedEndpoints is the positive half of
// TestV2HasOneSpellingPerEndpoint: removing a spelling from V2.Accounts is only
// de-duplication if the endpoint is still reachable somewhere. Without this, a
// deletion on both sides would pass the negative test and leave a v2 client
// unable to read an account's balances at all.
func TestV2OwnsTheDeDuplicatedEndpoints(t *testing.T) {
	tests := []struct {
		owner string
		typ   reflect.Type
		want  []string
	}{
		{
			owner: "V2.Balances",
			typ:   reflect.TypeOf(&balancesV2Facade{}),
			want:  []string{"ListAccountBalances", "GetAccountBalancesHistory"},
		},
		{
			owner: "V2.Operations",
			typ:   reflect.TypeOf(&operationsV2Facade{}),
			want:  []string{"ListOperations", "UpdateTransactionOperation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.owner, func(t *testing.T) {
			for _, name := range tt.want {
				if _, ok := tt.typ.MethodByName(name); !ok {
					t.Errorf("%s.%s is missing: V2.Accounts does not spell it either, so the endpoint is unreachable", tt.owner, name)
				}
			}
		})
	}
}
