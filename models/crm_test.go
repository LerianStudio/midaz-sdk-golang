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

func TestUpdateAliasInput_JSONOmitEmpty(t *testing.T) {
	data, err := json.Marshal(&UpdateAliasInput{Metadata: map[string]any{"risk": "low"}})
	require.NoError(t, err)

	assert.JSONEq(t, `{"metadata":{"risk":"low"}}`, string(data))
}

func TestUpdateAliasInput_JSONExplicitNullFields(t *testing.T) {
	data, err := json.Marshal((&UpdateAliasInput{}).WithNullFields("regulatoryFields"))
	require.NoError(t, err)

	assert.JSONEq(t, `{"regulatoryFields":null}`, string(data))
}

func TestUpdateAliasInput_RejectsEmptyPayload(t *testing.T) {
	input := NewUpdateAliasInput()

	require.ErrorContains(t, input.Validate(), "empty update payload not allowed")

	_, err := json.Marshal(input)
	require.ErrorContains(t, err, "empty update payload not allowed")
}

func TestCreateHolderInput_Validate(t *testing.T) {
	holderType := "NATURAL_PERSON"
	input := &CreateHolderInput{
		Type:     &holderType,
		Name:     "Jane Doe",
		Document: "12345678900",
		Metadata: map[string]any{"tier": "gold"},
	}

	require.NoError(t, input.Validate())
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

func TestCreateAliasInput_Validate(t *testing.T) {
	input := &CreateAliasInput{
		LedgerID:  "ledger-123",
		AccountID: "account-123",
		RelatedParties: []*RelatedParty{{
			Document:  "12345678900",
			Name:      "Jane Doe",
			Role:      "PRIMARY_HOLDER",
			StartDate: "2026-01-01",
		}},
	}

	require.NoError(t, input.Validate())
}

func TestCreateAliasInput_ValidateRelatedPartyDates(t *testing.T) {
	endDate := "2025-12-31"
	input := &CreateAliasInput{
		LedgerID:  "ledger-123",
		AccountID: "account-123",
		RelatedParties: []*RelatedParty{{
			Document:  "12345678900",
			Name:      "Jane Doe",
			Role:      "PRIMARY_HOLDER",
			StartDate: "2026-01-01",
			EndDate:   &endDate,
		}},
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endDate must not be before startDate")
}
