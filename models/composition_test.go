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

// TestCreateHolderAccountInput_BuilderClonesCollections proves the With*
// builders defensively copy caller-supplied collections: mutating the original
// map or slice (or a related party's pointer fields) after the build must not
// leak into the built input. Mirrors the clone guarantee CreateInstrumentInput
// already makes.
func TestCreateHolderAccountInput_BuilderClonesCollections(t *testing.T) {
	metadata := map[string]any{"tier": "gold"}
	endDate := "2026-12-31"
	parties := []*RelatedParty{{
		Document:  "D",
		Name:      "N",
		Role:      RelatedPartyRolePrimaryHolder,
		StartDate: "2026-01-01",
		EndDate:   &endDate,
	}}

	input := NewCreateHolderAccountInput("USD", "deposit").
		WithMetadata(metadata).
		WithRelatedParties(parties)

	// Mutate the caller's originals AFTER the build; nothing should leak in.
	metadata["tier"] = "bronze"
	metadata["injected"] = true
	parties[0].Name = "mutated"
	*parties[0].EndDate = "1999-01-01"
	parties[0] = nil

	assert.Equal(t, "gold", input.Metadata["tier"], "metadata value must be cloned")
	assert.NotContains(t, input.Metadata, "injected", "metadata keys must be cloned")

	require.Len(t, input.RelatedParties, 1)
	require.NotNil(t, input.RelatedParties[0], "slice element must be cloned")
	assert.Equal(t, "N", input.RelatedParties[0].Name, "party field must be cloned")
	require.NotNil(t, input.RelatedParties[0].EndDate)
	assert.Equal(t, "2026-12-31", *input.RelatedParties[0].EndDate, "party pointer field must be deep-cloned")
}

func strPtr(s string) *string { return &s }
