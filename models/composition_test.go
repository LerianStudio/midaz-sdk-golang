package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateHolderAccountInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateHolderAccountInput
		wantErr string
	}{
		{name: "nil", input: nil, wantErr: "input cannot be nil"},
		{name: "assetCode required", input: &CreateHolderAccountInput{Type: "deposit"}, wantErr: "assetCode is required"},
		{name: "type required", input: &CreateHolderAccountInput{AssetCode: "USD"}, wantErr: "type is required"},
		{name: "valid", input: &CreateHolderAccountInput{AssetCode: "USD", Type: "deposit"}, wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestCreateHolderAccountInputJSONTags pins the wire contract byte-for-byte
// against the server pkg/mmodel.CreateHolderAccountInput. holderId MUST NOT
// appear (path-sourced). Instrument fields carry omitempty; the flat account
// fields do not.
func TestCreateHolderAccountInputJSONTags(t *testing.T) {
	input := &CreateHolderAccountInput{
		Name:      "Corporate Checking",
		AssetCode: "USD",
		Type:      "deposit",
		Status:    NewStatus("ACTIVE"),
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	got := string(data)
	assert.Contains(t, got, `"name":"Corporate Checking"`)
	assert.Contains(t, got, `"assetCode":"USD"`)
	assert.Contains(t, got, `"type":"deposit"`)
	// Flat account fields lack omitempty: present even when zero/null.
	assert.Contains(t, got, `"parentAccountId":null`)
	assert.Contains(t, got, `"entityId":null`)
	assert.Contains(t, got, `"portfolioId":null`)
	assert.Contains(t, got, `"segmentId":null`)
	assert.Contains(t, got, `"alias":null`)
	assert.Contains(t, got, `"blocked":null`)
	assert.Contains(t, got, `"status":`)
	// holderId is path-sourced, never on the body.
	assert.NotContains(t, got, "holderId")
	// Instrument fields carry omitempty: absent when unset.
	assert.NotContains(t, got, "skip")
	assert.NotContains(t, got, "bankingDetails")
	assert.NotContains(t, got, "regulatoryFields")
	assert.NotContains(t, got, "relatedParties")
}

func TestCreateHolderAccountInputInstrumentFieldsEmitted(t *testing.T) {
	input := &CreateHolderAccountInput{
		AssetCode:        "USD",
		Type:             "deposit",
		Status:           NewStatus("ACTIVE"),
		Skip:             &AccountSkip{Holder: true},
		BankingDetails:   &BankingDetails{IBAN: strPtr("BR1")},
		RegulatoryFields: &RegulatoryFields{ParticipantDocument: strPtr("PD-1")},
		RelatedParties:   []*RelatedParty{{Document: "D", Name: "N", Role: RelatedPartyRolePrimaryHolder, StartDate: "2026-01-01"}},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	got := string(data)
	assert.Contains(t, got, `"skip":{"holder":true}`)
	assert.Contains(t, got, `"bankingDetails":`)
	assert.Contains(t, got, `"regulatoryFields":`)
	assert.Contains(t, got, `"relatedParties":`)
}

func TestAccountSkipOmitsFalse(t *testing.T) {
	data, err := json.Marshal(&AccountSkip{Holder: false})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(data))
}

// TestHolderAccountResponseNullInstrument confirms the partial-failure shape
// unmarshals with a null instrument (Instrument stays nil) while the typed
// InstrumentError block is populated. The pointer fields preserve the
// null/absent distinction.
func TestHolderAccountResponseNullInstrument(t *testing.T) {
	raw := `{"account":{"id":"33333333-3333-3333-3333-333333333333","name":"Acc"},"instrument":null,"instrumentError":{"status":"FAILED","reason":"X"}}`

	var response HolderAccountResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &response))

	require.NotNil(t, response.Account)
	assert.Equal(t, "33333333-3333-3333-3333-333333333333", response.Account.ID)
	assert.Nil(t, response.Instrument)
	require.NotNil(t, response.InstrumentError)
	assert.Equal(t, "FAILED", response.InstrumentError.Status)
	assert.Equal(t, "X", response.InstrumentError.Reason)
}

func TestHolderAccountResponseSuccessNoError(t *testing.T) {
	raw := `{"account":{"id":"1"},"instrument":{"holderId":"22222222-2222-2222-2222-222222222222","type":"CHECKING","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","deletedAt":null}}`

	var response HolderAccountResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &response))

	require.NotNil(t, response.Instrument)
	assert.Nil(t, response.InstrumentError)
}

func strPtr(s string) *string { return &s }
