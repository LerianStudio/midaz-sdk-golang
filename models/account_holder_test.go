package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const holderTestUUID = "550e8400-e29b-41d4-a716-446655440002"

func newHolderTestAccountInput() *CreateAccountInput {
	return NewCreateAccountInput("Borrower position", "BRL", "deposit")
}

// The holder link has to reach the wire under the key the server reads, or the
// account is created owned by nobody and nothing says so.
func TestCreateAccountInputHolderIDReachesTheWireAsHolderId(t *testing.T) {
	t.Parallel()

	input := newHolderTestAccountInput().WithHolderID(holderTestUUID)

	require.NotNil(t, input.HolderID)
	assert.Equal(t, holderTestUUID, *input.HolderID)

	encoded, err := json.Marshal(input)
	require.NoError(t, err)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))

	assert.Equal(t, holderTestUUID, wire["holderId"],
		"the server reads the ownership link from holderId")
}

// An account created with no holder must send no holder key at all, rather than
// an explicit null the server would have to interpret.
func TestCreateAccountInputOmitsHolderIdWhenUnset(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(newHolderTestAccountInput())
	require.NoError(t, err)

	var wire map[string]any
	require.NoError(t, json.Unmarshal(encoded, &wire))

	assert.NotContains(t, wire, "holderId",
		"an account created with no holder sends no holder key at all")
}

func TestCreateAccountInputValidatesTheHolderID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		holderID string
		wantErr  bool
	}{
		{name: "a well-formed holder id is accepted", holderID: holderTestUUID},
		{name: "a malformed holder id is refused", holderID: "not-a-uuid", wantErr: true},
		{name: "an empty holder id is refused", holderID: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newHolderTestAccountInput().WithHolderID(tt.holderID).Validate()

			if !tt.wantErr {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "holderId",
				"the caller must learn which field is wrong")
		})
	}
}

// WithHolderID keeps the contract its sibling setters keep: a nil receiver
// returns nil rather than panicking mid-chain.
func TestCreateAccountInputWithHolderIDOnNilReceiverReturnsNil(t *testing.T) {
	t.Parallel()

	var input *CreateAccountInput

	assert.Nil(t, input.WithHolderID(holderTestUUID))
}
