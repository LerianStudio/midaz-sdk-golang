// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"syscall"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// timeoutNetError is a net.Error whose Timeout() is true and whose message
// matches no network fragment, so only the timeout signal can classify it.
type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "deadline reached" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return false }

// TestFacadeTransportError pins the rule transportErr applies to a transport
// failure: network and timeout come back as the classifier builds them, and
// everything else keeps the internal error the facade returned before.
func TestFacadeTransportError(t *testing.T) {
	const operation = "V2.Ledgers.Get"

	alreadyTyped := sdkerrors.NewNetworkError("transport", syscall.ECONNREFUSED)

	tests := []struct {
		name         string
		cause        error
		wantCategory sdkerrors.ErrorCategory
		wantStatus   int
		wantSource   sdkerrors.ErrorSource
		wantSent     bool
	}{
		{
			name:         "connection refused is network",
			cause:        syscall.ECONNREFUSED,
			wantCategory: sdkerrors.CategoryNetwork,
			wantStatus:   http.StatusServiceUnavailable,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "connection reset is network",
			cause:        syscall.ECONNRESET,
			wantCategory: sdkerrors.CategoryNetwork,
			wantStatus:   http.StatusServiceUnavailable,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "net.OpError is network",
			cause:        &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
			wantCategory: sdkerrors.CategoryNetwork,
			wantStatus:   http.StatusServiceUnavailable,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "net.DNSError is network",
			cause:        &net.DNSError{Err: "no such host", Name: "ledger.invalid", IsNotFound: true},
			wantCategory: sdkerrors.CategoryNetwork,
			wantStatus:   http.StatusServiceUnavailable,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "context deadline exceeded is timeout",
			cause:        context.DeadlineExceeded,
			wantCategory: sdkerrors.CategoryTimeout,
			wantStatus:   http.StatusGatewayTimeout,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "net.Error with Timeout true is timeout",
			cause:        timeoutNetError{},
			wantCategory: sdkerrors.CategoryTimeout,
			wantStatus:   http.StatusGatewayTimeout,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		},
		{
			name:         "context canceled stays internal",
			cause:        context.Canceled,
			wantCategory: sdkerrors.CategoryInternal,
			wantStatus:   http.StatusInternalServerError,
			wantSource:   sdkerrors.ErrorSourceSDK,
			wantSent:     false,
		},
		{
			name:         "no response stays internal",
			cause:        errNoResponse,
			wantCategory: sdkerrors.CategoryInternal,
			wantStatus:   http.StatusInternalServerError,
			wantSource:   sdkerrors.ErrorSourceSDK,
			wantSent:     false,
		},
		{
			name:         "plain error stays internal",
			cause:        stderrors.New("boom"),
			wantCategory: sdkerrors.CategoryInternal,
			wantStatus:   http.StatusInternalServerError,
			wantSource:   sdkerrors.ErrorSourceSDK,
			wantSent:     false,
		},
		{
			name:         "already typed error stays internal",
			cause:        alreadyTyped,
			wantCategory: sdkerrors.CategoryInternal,
			wantStatus:   http.StatusInternalServerError,
			wantSource:   sdkerrors.ErrorSourceSDK,
			wantSent:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := transportErr(operation, tt.cause)

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			assert.Equal(t, tt.wantCategory, sdkErr.Category)
			assert.Equal(t, tt.wantStatus, sdkErr.StatusCode)
			assert.Equal(t, tt.wantSource, sdkErr.Source)
			assert.Equal(t, tt.wantSent, sdkErr.HTTPRequestSent)
			assert.False(t, sdkErr.HTTPResponseReceived)
			assert.Equal(t, operation, sdkErr.Operation)
			assert.True(t, sdkErr.Retryable())
			require.ErrorIs(t, err, tt.cause)

			if tt.wantCategory == sdkerrors.CategoryInternal {
				assert.Equal(t, sdkerrors.NewInternalError(operation, tt.cause).Error(), err.Error(),
					"a failure that is neither network nor timeout must render as before")
				assert.False(t, sdkerrors.IsCancellationError(err))
				assert.False(t, sdkerrors.IsNetworkError(err))
				assert.False(t, sdkerrors.IsTimeoutError(err))
			}
		})
	}

	t.Run("nil stays nil", func(t *testing.T) {
		require.NoError(t, transportErr(operation, nil))
	})
}
