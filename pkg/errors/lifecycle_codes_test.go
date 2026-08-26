package errors_test

import (
	stderrors "errors"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// errWithAPICode builds a realistic *Error carrying the given raw API code,
// going through the HTTP-response constructor production uses.
func errWithAPICode(apiCode string) error {
	return sdkerrors.ErrorFromHTTPResponse(http.StatusConflict, "req-1", "lifecycle error", apiCode, "transaction", "tx-1")
}

func TestIsRevertAlreadyExistsError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "0087 revert already exists", err: errWithAPICode(sdkerrors.APICodeRevertAlreadyExists), want: true},
		{name: "0088 already a revert", err: errWithAPICode(sdkerrors.APICodeAlreadyARevert), want: true},
		{name: "0099 status precondition", err: errWithAPICode(sdkerrors.APICodeStatusPreconditionFailed), want: false},
		{name: "0089 cannot revert", err: errWithAPICode(sdkerrors.APICodeCannotRevert), want: false},
		{name: "unrelated api code", err: errWithAPICode("0042"), want: false},
		{name: "no api code", err: errWithAPICode(""), want: false},
		{name: "plain error", err: stderrors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sdkerrors.IsRevertAlreadyExistsError(tt.err))
		})
	}
}

func TestIsStatusPreconditionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "0099 status precondition", err: errWithAPICode(sdkerrors.APICodeStatusPreconditionFailed), want: true},
		{name: "0087 revert already exists", err: errWithAPICode(sdkerrors.APICodeRevertAlreadyExists), want: false},
		{name: "plain error", err: stderrors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sdkerrors.IsStatusPreconditionError(tt.err))
		})
	}
}

func TestIsCannotRevertError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "0089 cannot revert", err: errWithAPICode(sdkerrors.APICodeCannotRevert), want: true},
		{name: "0087 revert already exists", err: errWithAPICode(sdkerrors.APICodeRevertAlreadyExists), want: false},
		{name: "plain error", err: stderrors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sdkerrors.IsCannotRevertError(tt.err))
		})
	}
}

// TestLifecycleAPICodeConstants pins the constant string values to the server
// contract (github.com/LerianStudio/midaz/v3/pkg/constant). If the server set
// drifts, this guards against silent divergence.
func TestLifecycleAPICodeConstants(t *testing.T) {
	assert.Equal(t, "0021", sdkerrors.APICodeParentTransactionIDNotFound)
	assert.Equal(t, "0087", sdkerrors.APICodeRevertAlreadyExists)
	assert.Equal(t, "0088", sdkerrors.APICodeAlreadyARevert)
	assert.Equal(t, "0089", sdkerrors.APICodeCannotRevert)
	assert.Equal(t, "0090", sdkerrors.APICodeAmbiguousRevert)
	assert.Equal(t, "0091", sdkerrors.APICodeParentIDSameID)
	assert.Equal(t, "0099", sdkerrors.APICodeStatusPreconditionFailed)
	assert.Equal(t, "0165", sdkerrors.APICodeRevertOnlyBidirectional)
}
