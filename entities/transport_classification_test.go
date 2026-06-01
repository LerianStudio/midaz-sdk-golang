// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"net/http"
	"syscall"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transportErrorRoundTripper struct {
	err error
}

func (r transportErrorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, r.err
}

// TestHTTPClient_TransportError_TypedNetwork is the 8D regression
// test. It exercises the audit's CRITICAL acceptance criterion:
//
//	IsNetworkError(err) returns true for deterministic transport failures.
//
// Before 8D: the SDK transport returned a bare fmt.Errorf-wrapped
// *net.OpError. IsNetworkError(err) returned false because the
// predicate looked for *Error{Category: CategoryNetwork}. The typed
// system was bypassed entirely.
//
// After 8D: handleRequestExecutionError funnels every transport
// failure through ClassifyTransportError, which produces a typed
// *errors.Error with the proper Category. IsNetworkError(err) now
// returns true.
func TestHTTPClient_TransportError_TypedNetwork(t *testing.T) {
	httpClient := &http.Client{Transport: transportErrorRoundTripper{err: syscall.ECONNREFUSED}}
	c := NewHTTPClient(httpClient, "test-token", nil)

	var result map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, "https://api.example.test/healthz",
		map[string]string{}, nil, &result)

	require.Error(t, err, "synthetic transport failure must fail")

	// The error must be classifiable. Either the network category
	// (conn-refused) or the timeout category (deadline) is acceptable —
	// the point is that it's NOT CategoryInternal and the typed
	// predicates work.
	require.True(t,
		sdkerrors.IsNetworkError(err) || sdkerrors.IsTimeoutError(err),
		"expected typed network/timeout error, got: %v", err)

	// Verify the underlying cause is preserved for errors.Unwrap walks.
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr,
		"transport error must be a *errors.Error")
	require.Error(t, sdkErr.Err,
		"underlying transport cause must be preserved")
}

// TestHTTPClient_TransportError_NoCustomRetryablePrefix verifies
// audit 8.3: the 'custom retryable: ' internal-wrapper prefix MUST
// NOT appear in user-facing error strings. Constructs an HTTPClient
// with a custom retry policy so the retryableCustomPolicyError path
// is exercised, then asserts the final rendered string is clean.
func TestHTTPClient_TransportError_NoCustomRetryablePrefix(t *testing.T) {
	httpClient := &http.Client{Transport: transportErrorRoundTripper{err: syscall.ECONNREFUSED}}
	c := NewHTTPClient(httpClient, "test-token", nil)

	// Force the custom retry policy path by configuring one that
	// always says 'retry'. The dial will still fail; the wrapper
	// will fire; we verify its rendered string is clean.
	c.SetCustomRetryPolicy(func(_ *http.Response, _ error) bool { return true })

	var result map[string]any
	err := c.doRequest(context.Background(), http.MethodGet, "https://api.example.test/healthz",
		map[string]string{}, nil, &result)

	require.Error(t, err)

	msg := err.Error()
	assert.NotContains(t, msg, "custom retryable:",
		"audit 8.3: 'custom retryable:' prefix must not leak; got: %q", msg)
}

// TestRetryableCustomPolicyError_NoPrefix is the unit-level proof
// that the wrapper itself produces clean output. Independent of the
// transport, this asserts the rendering contract directly.
func TestRetryableCustomPolicyError_NoPrefix(t *testing.T) {
	wrapper := retryableCustomPolicyError{
		err: errors.New("dial tcp: connection refused"),
	}

	msg := wrapper.Error()
	assert.Equal(t, "dial tcp: connection refused", msg,
		"audit 8.3: wrapper must render only the underlying error, no prefix")

	// And Unwrap must walk through.
	require.ErrorIs(t, wrapper, wrapper.err)
}
