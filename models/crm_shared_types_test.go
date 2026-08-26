package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateHolderInput_JSONOmitEmpty(t *testing.T) {
	name := "Jane Updated"
	data, err := json.Marshal(&UpdateHolderInput{Name: &name})
	require.NoError(t, err)

	assert.JSONEq(t, `{"name":"Jane Updated"}`, string(data))
}

func TestUpdateHolderInput_JSONExplicitNullFields(t *testing.T) {
	data, err := json.Marshal((&UpdateHolderInput{}).WithNullFields("contact", "metadata"))
	require.NoError(t, err)

	assert.JSONEq(t, `{"contact":null,"metadata":null}`, string(data))
}

func TestUpdateHolderInput_RejectsEmptyPayload(t *testing.T) {
	// Validate() is the single source of truth for empty-payload rejection;
	// MarshalJSON deliberately does not duplicate this check (audit 7.19).
	// The entity layer is responsible for invoking Validate before marshal.
	input := NewUpdateHolderInput()

	require.ErrorContains(t, input.Validate(), "empty update payload not allowed")
}

func TestUpdateInstrumentInput_JSONOmitEmpty(t *testing.T) {
	data, err := json.Marshal(&UpdateInstrumentInput{Metadata: map[string]any{"risk": "low"}})
	require.NoError(t, err)

	assert.JSONEq(t, `{"metadata":{"risk":"low"}}`, string(data))
}

func TestUpdateInstrumentInput_JSONExplicitNullFields(t *testing.T) {
	data, err := json.Marshal((&UpdateInstrumentInput{}).WithNullFields("regulatoryFields"))
	require.NoError(t, err)

	assert.JSONEq(t, `{"regulatoryFields":null}`, string(data))
}

func TestUpdateInstrumentInput_RejectsEmptyPayload(t *testing.T) {
	// See TestUpdateHolderInput_RejectsEmptyPayload for rationale.
	input := NewUpdateInstrumentInput()

	require.ErrorContains(t, input.Validate(), "empty update payload not allowed")
}

func TestCRMUpdateInputs_ValidateNullFields(t *testing.T) {
	name := "Jane Updated"
	tests := []struct {
		name  string
		input interface {
			Validate() error
		}
		want string
	}{
		{
			name:  "holder unsupported null field",
			input: NewUpdateHolderInput().WithNullFields("unknown"),
			want:  "unsupported null field",
		},
		{
			name:  "holder empty null field",
			input: NewUpdateHolderInput().WithNullFields(""),
			want:  "null field cannot be empty",
		},
		{
			name:  "holder set and clear conflict",
			input: (&UpdateHolderInput{Name: &name}).WithNullFields("name"),
			want:  "cannot be set and cleared",
		},
		{
			name:  "instrument set and clear conflict",
			input: (&UpdateInstrumentInput{Metadata: map[string]any{"risk": "low"}}).WithNullFields("metadata"),
			want:  "cannot be set and cleared",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, tt.input.Validate(), tt.want)
		})
	}
}

func TestCreateHolderInput_Validate(t *testing.T) {
	holderType := HolderTypeNaturalPerson
	input := &CreateHolderInput{
		Type:     &holderType,
		Name:     "Jane Doe",
		Document: "12345678900",
		Metadata: map[string]any{"tier": "gold"},
	}

	require.NoError(t, input.Validate())
}

func TestCRMBuilderNilReceivers(t *testing.T) {
	var (
		createHolder     *CreateHolderInput
		updateHolder     *UpdateHolderInput
		createInstrument *CreateInstrumentInput
		updateInstrument *UpdateInstrumentInput
	)

	assert.Nil(t, createHolder.WithMetadata(map[string]any{"tier": "gold"}))
	assert.Nil(t, updateHolder.WithNullFields("metadata"))
	assert.Nil(t, createInstrument.WithRelatedParties(nil))
	assert.Nil(t, updateInstrument.WithRelatedParties(nil))
}

func TestCreateHolderInput_ValidateRejectsInvalidType(t *testing.T) {
	holderType := "INVALID"
	input := &CreateHolderInput{
		Type:     &holderType,
		Name:     "Jane Doe",
		Document: "12345678900",
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NATURAL_PERSON or LEGAL_PERSON")
}

// The three tests below pin the shared related-party validation the CRM
// instrument resource inherits from the retired alias resource. They exercise
// validateRelatedParties / validateRelatedPartyDates / validateRelatedPartyRole
// through their surviving caller.

func TestCreateInstrumentInput_ValidateRelatedParties(t *testing.T) {
	input := newTestCreateInstrumentInput().WithRelatedParties([]*RelatedParty{{
		Document:  "12345678900",
		Name:      "Jane Doe",
		Role:      RelatedPartyRolePrimaryHolder,
		StartDate: "2026-01-01",
	}})

	require.NoError(t, input.Validate())
}

func TestCreateInstrumentInput_ValidateRelatedPartyDates(t *testing.T) {
	endDate := "2025-12-31"
	input := newTestCreateInstrumentInput().WithRelatedParties([]*RelatedParty{{
		Document:  "12345678900",
		Name:      "Jane Doe",
		Role:      RelatedPartyRolePrimaryHolder,
		StartDate: "2026-01-01",
		EndDate:   &endDate,
	}})

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endDate must not be before startDate")
}

func TestCreateInstrumentInput_ValidateInvalidRelatedPartyRole(t *testing.T) {
	input := newTestCreateInstrumentInput().WithRelatedParties([]*RelatedParty{{
		Document:  "12345678900",
		Name:      "Jane Doe",
		Role:      "INVALID",
		StartDate: "2026-01-01",
	}})

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role must be PRIMARY_HOLDER")
}
