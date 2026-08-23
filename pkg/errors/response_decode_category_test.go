package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A DECODE FAILURE MUST NEVER ANSWER YES TO IsInternalError.
//
// The two carry opposite money-path facts. IsInternalError is documented as an
// upstream blip -- the request did not take effect -- and IsBootstrapError
// lists it as exactly that. A response-decode failure proves the request WAS
// sent and the server DID answer, so the outcome is unknown and replaying it
// can post the same transaction twice.
//
// This guard exists because the two disagreed once already: the doc on
// CodeResponseDecode said "deliberately distinct from CodeInternal" while the
// constructor stamped CategoryInternal onto the error, so every caller asking
// "was this an internal error?" was told yes.
func TestResponseDecodeErrorIsNotAnInternalError(t *testing.T) {
	t.Parallel()

	err := NewResponseDecodeError("ListTransactions", http.StatusOK, errors.New("unexpected EOF"))
	require.Error(t, err)

	assert.True(t, IsResponseDecodeError(err),
		"the caller cannot tell an unreadable answer from anything else")
	assert.False(t, IsInternalError(err),
		"a decode failure answering yes here tells the caller nothing happened upstream -- "+
			"it did, and replaying the operation can pay twice")
	assert.Equal(t, CategoryResponseDecode, err.Category)
}

// The categoriser has its own fallback to CategoryInternal, so moving the
// constructor off CategoryInternal is not enough on its own: without an
// explicit check the decode error falls through and is re-labelled internal.
func TestGetErrorCategoryKeepsAResponseDecodeErrorOutOfInternal(t *testing.T) {
	t.Parallel()

	err := NewResponseDecodeError("CreateTransaction", http.StatusCreated, errors.New("invalid character"))

	assert.Equal(t, CategoryResponseDecode, GetErrorCategory(err))
	assert.NotEqual(t, CategoryInternal, GetErrorCategory(err))
}

// Wrapping must not lose the distinction -- callers see these errors wrapped by
// the facade layer, never bare.
func TestResponseDecodeSurvivesWrapping(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("listing transactions: %w",
		NewResponseDecodeError("ListTransactions", http.StatusOK, errors.New("unexpected EOF")))

	assert.True(t, IsResponseDecodeError(wrapped))
	assert.False(t, IsInternalError(wrapped))
}
