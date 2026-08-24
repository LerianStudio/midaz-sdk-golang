// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"net/http"
	"reflect"
	"testing"
)

// TestEntity_PlaneFacadeAccessorsWired proves that every service a caller can
// reach is actually built by construction.
//
// The version-group test next door pins which accessors EXIST; this one pins
// that each of them is non-nil after a normal NewEntityWithConfig (which builds
// the plane clients and runs initServices). Those are different failures: an
// accessor can be declared on V1Services or V2Services, pass the membership
// test, and still be left nil by a missed line in newV1Services — which a
// caller only discovers as a nil dereference at the first call.
//
// The two groups are walked by reflection rather than listed, so a service
// added to a group without a matching constructor line fails here with no test
// edit. The flat Tracer accessors stay listed by hand: they live directly on
// Entity next to non-service fields, so there is no struct to walk.
func TestEntity_PlaneFacadeAccessorsWired(t *testing.T) {
	baseURLs := map[string]string{
		"onboarding": "https://api.example.com/onboarding",
		"tracer":     "https://api.example.com/tracer/v1",
	}

	entity, err := NewEntityWithConfig(&mockPluginAuthConfig{httpClient: http.DefaultClient, baseURLs: baseURLs})
	if err != nil {
		t.Fatalf("NewEntityWithConfig: %v", err)
	}

	for _, group := range []struct {
		name  string
		value reflect.Value
	}{
		{"V1", reflect.ValueOf(entity.V1)},
		{"V2", reflect.ValueOf(entity.V2)},
	} {
		typ := group.value.Type()
		if typ.NumField() == 0 {
			t.Fatalf("%s has no members — the reflection walk would assert nothing", group.name)
		}

		for i := range typ.NumField() {
			if group.value.Field(i).IsNil() {
				t.Errorf("%s.%s is nil after construction — declared but never wired in new%sServices",
					group.name, typ.Field(i).Name, group.name)
			}
		}
	}

	// Each `entity.X == nil` is evaluated against the concrete facade pointer
	// type (not an any-wrapped typed nil), so the comparison is sound.
	tracerAccessors := []struct {
		name  string
		isNil bool
	}{
		{"Rules", entity.Rules == nil},
		{"Limits", entity.Limits == nil},
		{"Validations", entity.Validations == nil},
		{"Reservations", entity.Reservations == nil},
		{"AuditEvents", entity.AuditEvents == nil},
	}
	for _, a := range tracerAccessors {
		if a.isNil {
			t.Errorf("tracer accessor %s is nil — initServices skipped it", a.name)
		}
	}
}
