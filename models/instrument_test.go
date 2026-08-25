package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testInstrumentLedgerID  = "99999999-9999-9999-9999-999999999999"
	testInstrumentAccountID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

// newTestCreateInstrumentInput builds a payload carrying all four properties the
// server contract requires, which is the baseline every instrument test in this
// package varies from.
func newTestCreateInstrumentInput() *CreateInstrumentInput {
	return NewCreateInstrumentInput(testInstrumentLedgerID, testInstrumentAccountID).
		WithBankingDetails(&BankingDetails{}).
		WithMetadata(map[string]any{"k": "v"})
}

// TestCreateInstrumentInputValidate pins the create contract's four required
// properties, one missing property per case.
func TestCreateInstrumentInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateInstrumentInput
		wantErr string
	}{
		{name: "nil", input: nil, wantErr: "input cannot be nil"},
		{
			name:    "ledger required",
			input:   NewCreateInstrumentInput("", testInstrumentAccountID).WithBankingDetails(&BankingDetails{}).WithMetadata(map[string]any{}),
			wantErr: "ledgerId is required",
		},
		{
			name:    "account required",
			input:   NewCreateInstrumentInput(testInstrumentLedgerID, "").WithBankingDetails(&BankingDetails{}).WithMetadata(map[string]any{}),
			wantErr: "accountId is required",
		},
		{
			name:    "ledger must be a uuid",
			input:   NewCreateInstrumentInput("not-a-uuid", testInstrumentAccountID).WithBankingDetails(&BankingDetails{}).WithMetadata(map[string]any{}),
			wantErr: "ledgerId must be a UUID",
		},
		{
			name:    "account must be a uuid",
			input:   NewCreateInstrumentInput(testInstrumentLedgerID, "not-a-uuid").WithBankingDetails(&BankingDetails{}).WithMetadata(map[string]any{}),
			wantErr: "accountId must be a UUID",
		},
		{
			name:    "banking details required",
			input:   NewCreateInstrumentInput(testInstrumentLedgerID, testInstrumentAccountID).WithMetadata(map[string]any{}),
			wantErr: "bankingDetails is required",
		},
		{
			name:    "metadata required",
			input:   NewCreateInstrumentInput(testInstrumentLedgerID, testInstrumentAccountID).WithBankingDetails(&BankingDetails{}),
			wantErr: "metadata is required",
		},
		{name: "valid", input: newTestCreateInstrumentInput(), wantErr: ""},
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

// TestCreateInstrumentInputMarshalsOnlyContractFields is the anti-regression
// guard on the wire body: the endpoint declares additionalProperties: false, so
// a convenience field added here is not ignored — it fails the request. The
// required four are always emitted; the optional two are omitted when unset.
func TestCreateInstrumentInputMarshalsOnlyContractFields(t *testing.T) {
	data, err := json.Marshal(newTestCreateInstrumentInput())
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"ledgerId":"`+testInstrumentLedgerID+`",
		"accountId":"`+testInstrumentAccountID+`",
		"bankingDetails":{},
		"metadata":{"k":"v"}
	}`, string(data))

	var emitted map[string]any
	require.NoError(t, json.Unmarshal(data, &emitted))

	for _, phantom := range []string{"type", "document"} {
		assert.NotContains(t, emitted, phantom,
			"the create endpoint has no slot for %q; emitting it fails the whole request", phantom)
	}
}

func TestCreateInstrumentInputBuildersNilSafe(t *testing.T) {
	var input *CreateInstrumentInput
	require.Nil(t, input.WithMetadata(map[string]any{"k": "v"}))
	require.Nil(t, input.WithBankingDetails(&BankingDetails{}))
	require.Nil(t, input.WithRegulatoryFields(&RegulatoryFields{}))
	require.Nil(t, input.WithRelatedParties(nil))
}

// newTestUpdateInstrumentInput builds the minimum PATCH body the contract
// accepts: both required properties, nothing else.
func newTestUpdateInstrumentInput() *UpdateInstrumentInput {
	return NewUpdateInstrumentInput().
		WithBankingDetails(&BankingDetails{}).
		WithMetadata(map[string]any{"k": "v"})
}

// TestUpdateInstrumentInputMarshalsOnlyContractFields is the anti-regression
// guard on the PATCH body. The endpoint declares additionalProperties: false,
// so an extra property fails the whole request; the optional two stay out when
// unset.
func TestUpdateInstrumentInputMarshalsOnlyContractFields(t *testing.T) {
	data, err := json.Marshal(newTestUpdateInstrumentInput())
	require.NoError(t, err)

	assert.JSONEq(t, `{"bankingDetails":{},"metadata":{"k":"v"}}`, string(data))

	var emitted map[string]any
	require.NoError(t, json.Unmarshal(data, &emitted))

	assert.NotContains(t, emitted, "document",
		"the update endpoint has no document slot; emitting it fails the whole request")
	assert.NotContains(t, emitted, "regulatoryFields")
	assert.NotContains(t, emitted, "relatedParties")
}

// TestUpdateInstrumentInputRequiresContractFields pins required-on-PATCH. The
// server chose it; the SDK refuses locally instead of letting the caller find
// out as a 422.
func TestUpdateInstrumentInputRequiresContractFields(t *testing.T) {
	tests := []struct {
		name    string
		input   *UpdateInstrumentInput
		wantErr string
	}{
		{
			name:    "banking details required",
			input:   NewUpdateInstrumentInput().WithMetadata(map[string]any{"k": "v"}),
			wantErr: "bankingDetails is required",
		},
		{
			name:    "metadata required",
			input:   NewUpdateInstrumentInput().WithBankingDetails(&BankingDetails{}),
			wantErr: "metadata is required",
		},
		{
			name:    "optional properties alone are not enough",
			input:   NewUpdateInstrumentInput().WithRegulatoryFields(&RegulatoryFields{}),
			wantErr: "is required",
		},
		{name: "both present", input: newTestUpdateInstrumentInput(), wantErr: ""},
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

// TestUpdateInstrumentInputNullFields covers the collision between RFC 7396
// field-clearing and required-on-PATCH: only the two optional properties can be
// cleared, and a caller clearing a required one is told why rather than being
// handed the generic unsupported-field message.
func TestUpdateInstrumentInputNullFields(t *testing.T) {
	clearable := []string{"regulatoryFields", "relatedParties"}
	for _, field := range clearable {
		t.Run("clears "+field, func(t *testing.T) {
			input := newTestUpdateInstrumentInput().WithNullField(field)
			require.NoError(t, input.Validate())

			data, err := json.Marshal(input)
			require.NoError(t, err)
			assert.Contains(t, string(data), `"`+field+`":null`)
		})
	}

	for _, field := range []string{"metadata", "bankingDetails"} {
		t.Run("refuses to clear required "+field, func(t *testing.T) {
			// Set on a fresh payload so the set-and-cleared conflict is not what
			// refuses it: this must fail on the required rule alone.
			input := NewUpdateInstrumentInput().WithNullField(field)
			require.ErrorContains(t, input.Validate(), "required by the update contract and cannot be cleared")

			// MarshalJSON refuses too, so a caller skipping Validate cannot put
			// a null on a required property.
			_, err := json.Marshal(input)
			require.ErrorContains(t, err, "unsupported null field")
		})
	}

	t.Run("refuses a property the contract does not have", func(t *testing.T) {
		input := newTestUpdateInstrumentInput().WithNullField("document")
		require.ErrorContains(t, input.Validate(), `unsupported null field "document"`)
	})

	t.Run("refuses set and cleared in the same request", func(t *testing.T) {
		input := newTestUpdateInstrumentInput().
			WithRegulatoryFields(&RegulatoryFields{}).
			WithNullField("regulatoryFields")
		require.ErrorContains(t, input.Validate(), "cannot be set and cleared in the same request")
	})
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
		CursorListOpts: CursorListOpts{Cursor: "c-1"},
		Filters:        InstrumentFilters{Type: "CHECKING", IncludeDeleted: true},
	}

	params := opts.ToQueryParams()
	assert.Equal(t, "CHECKING", params["type"])
	assert.Equal(t, "true", params["include_deleted"])
	// Cursor renders via CursorQueryParams. Dates are NOT exercised here: this is a
	// NoDates endpoint (see TestHoldersInstrumentsListOpts_RejectDates), so a valid
	// opts never carries StartDate/EndDate.
	assert.Equal(t, "c-1", params["cursor"])
}
