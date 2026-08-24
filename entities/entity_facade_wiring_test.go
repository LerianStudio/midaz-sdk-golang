// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"net/http"
	"testing"
)

// TestEntity_PlaneFacadeAccessorsWired proves the Phase 5.1 additive wiring:
// every net-new plane-native facade accessor is non-nil after a normal
// NewEntityWithConfig construction (which builds e.planes and runs
// initServices). Guards against the `if e.planes != nil` block silently
// skipping an assignment, and against an accessor declared but never wired.
// The 16 legacy accessors are covered by their own suites; this test asserts
// only the additive surface and that the legacy accessors still coexist.
func TestEntity_PlaneFacadeAccessorsWired(t *testing.T) {
	baseURLs := map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
		"tracer":      "https://api.example.com/tracer/v1",
	}

	entity, err := NewEntityWithConfig(&mockPluginAuthConfig{httpClient: http.DefaultClient, baseURLs: baseURLs})
	if err != nil {
		t.Fatalf("NewEntityWithConfig: %v", err)
	}

	// Each `entity.X == nil` is evaluated against the concrete facade pointer
	// type (not an any-wrapped typed nil), so the comparison is sound.
	newAccessors := []struct {
		name  string
		isNil bool
	}{
		{"Rules", entity.Rules == nil},
		{"Limits", entity.Limits == nil},
		{"Validations", entity.Validations == nil},
		{"Reservations", entity.Reservations == nil},
		{"AuditEvents", entity.AuditEvents == nil},
		{"ProtectionAudit", entity.ProtectionAudit == nil},
		{"Encryption", entity.Encryption == nil},
		{"Instruments", entity.Instruments == nil},
		{"Composition", entity.Composition == nil},
		{"FeePackages", entity.FeePackages == nil},
		{"FeeEstimates", entity.FeeEstimates == nil},
		{"BillingPackages", entity.BillingPackages == nil},
		{"BillingCalculations", entity.BillingCalculations == nil},
	}
	for _, a := range newAccessors {
		if a.isNil {
			t.Errorf("accessor %s is nil — Phase 5.1 additive wiring skipped it", a.name)
		}
	}

	// Coexistence: a representative legacy accessor is still wired.
	if entity.Transactions == nil {
		t.Error("legacy accessor Transactions is nil — additive wiring must not disturb coexistence")
	}
}
