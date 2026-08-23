// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A response the SDK received and could not decode is a money-path fact of its
// own: the request WAS sent and the server DID answer, so the caller must never
// be told otherwise, and the status it answered with is not an error status.
func TestNewResponseDecodeError_ReportsRequestSentAndResponseReceived(t *testing.T) {
	t.Parallel()

	cause := &json.SyntaxError{}
	err := sdkerrors.NewResponseDecodeError("Transactions.CreateJSON", http.StatusCreated, cause)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsResponseDecodeError(err), "the dedicated predicate must recognise it")
	assert.True(t, sdkerrors.HTTPRequestSent(err), "the request reached the server")
	assert.True(t, sdkerrors.HTTPResponseReceived(err), "the server answered")

	_, upstream := sdkerrors.ActualHTTPStatus(err)
	assert.False(t, upstream, "an unreadable body is not an upstream error status")

	assert.Contains(t, err.Error(), "201", "the status the server answered with stays visible to operators")
	assert.True(t, errors.Is(err, cause) || errors.As(err, &cause), "the decode cause stays in the chain")
}

// The predicate must not fire for the SDK's other internal failures — a marshal
// failure before the request is the opposite fact (nothing was sent).
func TestIsResponseDecodeError_DoesNotMatchOtherInternalErrors(t *testing.T) {
	t.Parallel()

	assert.False(t, sdkerrors.IsResponseDecodeError(nil))
	assert.False(t, sdkerrors.IsResponseDecodeError(errors.New("boom")))
	assert.False(t, sdkerrors.IsResponseDecodeError(sdkerrors.NewInternalError("Transactions.CreateJSON", errors.New("marshal"))))
}
