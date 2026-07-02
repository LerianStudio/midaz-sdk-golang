package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateInstrumentInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateInstrumentInput
		wantErr string
	}{
		{name: "nil", input: nil, wantErr: "input cannot be nil"},
		{name: "type required", input: NewCreateInstrumentInput(""), wantErr: "type is required"},
		{name: "valid", input: NewCreateInstrumentInput("CHECKING"), wantErr: ""},
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

func TestCreateInstrumentInputBuildersOmitUnset(t *testing.T) {
	input := NewCreateInstrumentInput("CHECKING").
		WithDocument("DOC-1").
		WithMetadata(map[string]any{"k": "v"})

	data, err := json.Marshal(input)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"type":"CHECKING"`)
	assert.Contains(t, string(data), `"document":"DOC-1"`)
	assert.NotContains(t, string(data), "bankingDetails")
	assert.NotContains(t, string(data), "regulatoryFields")
	assert.NotContains(t, string(data), "relatedParties")
}

func TestCreateInstrumentInputBuildersNilSafe(t *testing.T) {
	var input *CreateInstrumentInput
	require.Nil(t, input.WithDocument("x"))
	require.Nil(t, input.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, input.WithBankingDetails(&BankingDetails{}))
	require.Nil(t, input.WithRegulatoryFields(&RegulatoryFields{}))
	require.Nil(t, input.WithRelatedParties(nil))
}

func TestUpdateInstrumentInputOmitsUnsetPatchFields(t *testing.T) {
	data, err := json.Marshal(NewUpdateInstrumentInput().WithDocument("DOC-9"))
	require.NoError(t, err)

	assert.JSONEq(t, `{"document":"DOC-9"}`, string(data))
	assert.NotContains(t, string(data), "metadata")
	assert.NotContains(t, string(data), "bankingDetails")
}

func TestUpdateInstrumentInputEmptyPayloadRejected(t *testing.T) {
	require.ErrorContains(t, NewUpdateInstrumentInput().Validate(), "empty update payload")
}

func TestUpdateInstrumentInputNilPtrMarshalsNull(t *testing.T) {
	var input *UpdateInstrumentInput
	data, err := json.Marshal(input)
	require.NoError(t, err)
	assert.JSONEq(t, `null`, string(data))
}

func TestUpdateInstrumentInputNilSafe(t *testing.T) {
	var input *UpdateInstrumentInput
	require.Error(t, input.Validate())
	require.Nil(t, input.WithMetadata(map[string]any{"k": "v"}))
}

func TestInstrumentUnmarshalUsesSharedTypes(t *testing.T) {
	raw := `{
		"id":"11111111-1111-1111-1111-111111111111",
		"holderId":"22222222-2222-2222-2222-222222222222",
		"type":"CHECKING",
		"document":"DOC-1",
		"bankingDetails":{"iban":"BR123"},
		"regulatoryFields":{"participantDocument":"PD-1"},
		"relatedParties":[{"document":"D","name":"N","role":"PRIMARY_HOLDER","startDate":"2026-01-01"}],
		"createdAt":"2026-01-01T00:00:00Z",
		"updatedAt":"2026-01-01T00:00:00Z",
		"deletedAt":null
	}`

	var instrument Instrument
	require.NoError(t, json.Unmarshal([]byte(raw), &instrument))

	require.NotNil(t, instrument.Type)
	assert.Equal(t, "CHECKING", *instrument.Type)
	require.NotNil(t, instrument.BankingDetails)
	require.NotNil(t, instrument.BankingDetails.IBAN)
	assert.Equal(t, "BR123", *instrument.BankingDetails.IBAN)
	require.NotNil(t, instrument.RegulatoryFields)
	require.NotNil(t, instrument.RelatedParties)
	require.Len(t, *instrument.RelatedParties, 1)
	assert.Nil(t, instrument.DeletedAt)
}

func TestInstrumentsListOptsToQueryParams(t *testing.T) {
	opts := InstrumentsListOpts{
		Filters: InstrumentFilters{Type: "CHECKING", IncludeDeleted: true},
	}

	params := opts.ToQueryParams()
	assert.Equal(t, "CHECKING", params["type"])
	assert.Equal(t, "true", params["include_deleted"])
}
