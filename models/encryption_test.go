// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/validation"
)

// fieldKeys extracts the per-field keys from a *validation.FieldErrors so tests
// assert the exact keys rather than substring-matching the flat render.
func fieldKeys(t *testing.T, err error) map[string]bool {
	t.Helper()

	var fe *validation.FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("error type = %T, want *validation.FieldErrors", err)
	}

	keys := make(map[string]bool, len(fe.Errors))
	for _, e := range fe.Errors {
		keys[e.Field] = true
	}

	return keys
}

// TestProvisionEncryptionInput_Validate covers case (e): Validate rejects an
// empty Actor and an empty Reason, and the FieldErrors carry the exact per-field
// keys.
func TestProvisionEncryptionInput_Validate(t *testing.T) {
	if err := NewProvisionEncryptionInput("svc", "rotate").Validate(); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	tests := []struct {
		name  string
		input *ProvisionEncryptionInput
		want  []string
	}{
		{"empty actor", &ProvisionEncryptionInput{Actor: "", Reason: "rotate"}, []string{"actor"}},
		{"empty reason", &ProvisionEncryptionInput{Actor: "svc", Reason: ""}, []string{"reason"}},
		{"both empty", &ProvisionEncryptionInput{}, []string{"actor", "reason"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error for %+v", tt.input)
			}

			keys := fieldKeys(t, err)
			for _, want := range tt.want {
				if !keys[want] {
					t.Errorf("missing FieldError key %q; got keys %v", want, keys)
				}
			}
			if len(keys) != len(tt.want) {
				t.Errorf("got keys %v, want exactly %v", keys, tt.want)
			}
		})
	}
}
