// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors_test

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyTransportError_NilPasses covers the nil-safe contract.
func TestClassifyTransportError_NilPasses(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("op", nil)
	assert.NoError(t, err)
}

// TestClassifyTransportError_Idempotent covers the contract that
// classifying an already-typed *Error returns the same error
// unchanged. The transport layer can call ClassifyTransportError
// at every boundary without nesting wrappers.
func TestClassifyTransportError_Idempotent(t *testing.T) {
	original := sdkerrors.NewNetworkError("accounts.Create", errors.New("boom"))
	classified := sdkerrors.ClassifyTransportError("accounts.Create", original)

	assert.Same(t, original, classified,
		"already-typed *Error must pass through unchanged")
}

// TestClassifyTransportError_ContextCanceled covers Rule 1.
func TestClassifyTransportError_ContextCanceled(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("accounts.Create", context.Canceled)
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, sdkerrors.CategoryCancellation, sdkErr.Category)
	assert.Equal(t, sdkerrors.CodeCancellation, sdkErr.Code)
	assert.Equal(t, "accounts.Create", sdkErr.Operation)
	require.ErrorIs(t, err, context.Canceled)
}

// TestClassifyTransportError_ContextDeadlineExceeded covers Rule 2.
func TestClassifyTransportError_ContextDeadlineExceeded(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("accounts.Get", context.DeadlineExceeded)
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, sdkerrors.CategoryTimeout, sdkErr.Category)
	assert.Equal(t, sdkerrors.CodeTimeout, sdkErr.Code)
	assert.True(t, sdkerrors.IsTimeoutError(err))
}

// TestClassifyTransportError_NetTimeout covers Rule 3 — a *net.OpError
// whose Timeout() returns true is classified as CategoryTimeout, not
// CategoryNetwork.
func TestClassifyTransportError_NetTimeout(t *testing.T) {
	timeoutErr := &timeoutNetError{msg: "i/o timeout"}
	err := sdkerrors.ClassifyTransportError("transactions.Create", timeoutErr)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsTimeoutError(err),
		"net.Error with Timeout()==true must classify as CategoryTimeout")
}

// TestClassifyTransportError_DNSFailure covers Rule 5.
func TestClassifyTransportError_DNSFailure(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "nope.invalid"}
	err := sdkerrors.ClassifyTransportError("transactions.List", dnsErr)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err),
		"DNS lookup failure must classify as CategoryNetwork")

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, "transactions.List", sdkErr.Operation)
	require.ErrorIs(t, err, dnsErr,
		"underlying cause must be walkable via errors.Unwrap")
}

// TestClassifyTransportError_ConnRefused covers the syscall path
// (Rule 7) — an EPIPE / ECONNREFUSED reaches the typed predicate.
func TestClassifyTransportError_ConnRefused(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("ledgers.Create", syscall.ECONNREFUSED)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err),
		"ECONNREFUSED must classify as CategoryNetwork")
}

// TestClassifyTransportError_OpError covers Rule 4 — generic
// *net.OpError without a timeout flag.
func TestClassifyTransportError_OpError(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	err := sdkerrors.ClassifyTransportError("accounts.List", opErr)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err))
}

// TestClassifyTransportError_FallbackInternal covers the default —
// anything not matched by the rule set is CategoryInternal.
func TestClassifyTransportError_FallbackInternal(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("unknown.Op", errors.New("totally unrelated boom"))
	require.Error(t, err)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, sdkerrors.CategoryInternal, sdkErr.Category)
}

// TestClassifyTransportError_SubstringFallback covers the last-resort
// substring path for transports that synthesize string-only errors.
func TestClassifyTransportError_SubstringFallback(t *testing.T) {
	// A naked errors.New that contains the "no such host" fragment.
	err := sdkerrors.ClassifyTransportError("op", errors.New("dial tcp: lookup foo.invalid: no such host"))

	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err))
}

// TestClassifyTransportError_ErrorsIsWalksThrough verifies that
// classified errors play nicely with errors.Is for sentinel matching.
func TestClassifyTransportError_ErrorsIsWalksThrough(t *testing.T) {
	err := sdkerrors.ClassifyTransportError("op", context.Canceled)
	require.ErrorIs(t, err, sdkerrors.ErrCancellation,
		"classified error must match the typed sentinel")
}

// TestClassifyTransportError_DeterministicConnRefused covers the audit
// acceptance criterion without depending on live localhost socket behavior.
func TestClassifyTransportError_DeterministicConnRefused(t *testing.T) {
	classified := sdkerrors.ClassifyTransportError("readyz.Check", syscall.ECONNREFUSED)
	require.Error(t, classified)

	assert.True(t, sdkerrors.IsNetworkError(classified),
		"expected network/timeout, got %s: %v",
		errorCategory(t, classified), classified)
}

// errorCategory extracts the category label from an *Error for
// readable test failure messages.
func errorCategory(t *testing.T, err error) sdkerrors.ErrorCategory {
	t.Helper()

	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) {
		return sdkErr.Category
	}

	return ""
}

// timeoutNetError is a minimal net.Error implementation whose
// Timeout() returns true. Used to exercise Rule 3 without needing a
// real socket dial in unit tests.
type timeoutNetError struct {
	msg string
}

func (e *timeoutNetError) Error() string { return e.msg }
func (*timeoutNetError) Timeout() bool   { return true }
func (*timeoutNetError) Temporary() bool { return false }
