// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
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

// canceledTimeoutError is a net.Error whose Timeout() is true and that also
// wraps context.Canceled, to pin that cancellation wins over timeout.
type canceledTimeoutError struct{}

func (canceledTimeoutError) Error() string   { return "canceled while waiting" }
func (canceledTimeoutError) Timeout() bool   { return true }
func (canceledTimeoutError) Temporary() bool { return false }
func (canceledTimeoutError) Unwrap() error   { return context.Canceled }

// refusedDialError is the shape http.Client.Do returns for a refused
// connection: *url.Error over *net.OpError over *os.SyscallError over the errno.
func refusedDialError() error {
	return &url.Error{Op: "Get", URL: "http://127.0.0.1:1/x", Err: &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: os.NewSyscallError("connect", syscall.ECONNREFUSED),
	}}
}

// TestFacadeTransportError pins the rule transportErr applies to a transport
// failure: network and timeout come back as the classifier builds them, and
// everything else keeps the internal error the facade returned before.
func TestFacadeTransportError(t *testing.T) {
	const operation = "V2.Ledgers.Get"

	network := func(name string, newCause func() error, alsoIs error) facadeTransportErrorCase {
		return facadeTransportErrorCase{
			name: name, newCause: newCause, alsoIs: alsoIs,
			wantCategory: sdkerrors.CategoryNetwork,
			wantStatus:   http.StatusServiceUnavailable,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		}
	}
	timeout := func(name string, newCause func() error) facadeTransportErrorCase {
		return facadeTransportErrorCase{
			name: name, newCause: newCause,
			wantCategory: sdkerrors.CategoryTimeout,
			wantStatus:   http.StatusGatewayTimeout,
			wantSource:   sdkerrors.ErrorSourceTransport,
			wantSent:     true,
		}
	}
	internal := func(name string, newCause func() error) facadeTransportErrorCase {
		return facadeTransportErrorCase{
			name: name, newCause: newCause,
			wantCategory: sdkerrors.CategoryInternal,
			wantStatus:   http.StatusInternalServerError,
			wantSource:   sdkerrors.ErrorSourceSDK,
			wantSent:     false,
		}
	}

	userinfoCanceled := internal("url error with userinfo wrapping context canceled stays internal", func() error {
		return &url.Error{Op: "Get", URL: "http://user:secret@host/x", Err: context.Canceled}
	})
	// The classifier strips the userinfo from the *url.Error in place before it
	// classifies, so the internal fallback renders the stripped URL, while the
	// pre-helper facade rendered it verbatim (redaction does not mask it).
	userinfoCanceled.wantMessage = `internal error during V2.Ledgers.Get: internal error: Get "http://host/x": context canceled`
	userinfoCanceled.wantPreviousMessage = `internal error during V2.Ledgers.Get: internal error: Get "http://user:secret@host/x": context canceled`

	tests := []facadeTransportErrorCase{
		network("connection refused is network", func() error { return syscall.ECONNREFUSED }, nil),
		network("connection reset is network", func() error { return syscall.ECONNRESET }, nil),
		network("net.OpError is network", func() error {
			return &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
		}, nil),
		network("net.DNSError is network", func() error {
			return &net.DNSError{Err: "no such host", Name: "ledger.invalid", IsNotFound: true}
		}, nil),
		network("url error over a refused dial is network", refusedDialError, syscall.ECONNREFUSED),
		network("wrapped connection refused is network", func() error {
			return fmt.Errorf("x: %w", syscall.ECONNREFUSED)
		}, syscall.ECONNREFUSED),
		timeout("context deadline exceeded is timeout", func() error { return context.DeadlineExceeded }),
		timeout("net.Error with Timeout true is timeout", func() error { return timeoutNetError{} }),
		internal("context canceled stays internal", func() error { return context.Canceled }),
		internal("wrapped context canceled stays internal", func() error {
			return fmt.Errorf("x: %w", context.Canceled)
		}),
		internal("url error over context canceled stays internal", func() error {
			return &url.Error{Op: "Get", URL: "http://127.0.0.1:1/x", Err: context.Canceled}
		}),
		internal("timeout that wraps context canceled stays internal", func() error { return canceledTimeoutError{} }),
		internal("joined canceled and connection refused stays internal", func() error {
			return stderrors.Join(context.Canceled, syscall.ECONNREFUSED)
		}),
		internal("no response stays internal", func() error { return errNoResponse }),
		internal("plain error stays internal", func() error { return stderrors.New("boom") }),
		internal("already typed network error stays internal", func() error {
			return sdkerrors.NewNetworkError("transport", syscall.ECONNREFUSED)
		}),
		internal("already typed timeout error stays internal", func() error {
			return sdkerrors.NewTimeoutError("transport", "", context.DeadlineExceeded)
		}),
		internal("wrapped typed nil error stays internal", func() error {
			return fmt.Errorf("x: %w", (*sdkerrors.Error)(nil))
		}),
		userinfoCanceled,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := tt.newCause()
			err := transportErr(operation, cause)

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			assert.Equal(t, tt.wantCategory, sdkErr.Category)
			assert.Equal(t, tt.wantStatus, sdkErr.StatusCode)
			assert.Equal(t, tt.wantSource, sdkErr.Source)
			assert.Equal(t, tt.wantSent, sdkErr.HTTPRequestSent)
			assert.False(t, sdkErr.HTTPResponseReceived)
			assert.Equal(t, operation, sdkErr.Operation)
			assert.True(t, sdkErr.Retryable())
			require.ErrorIs(t, err, cause)

			if tt.alsoIs != nil {
				require.ErrorIs(t, err, tt.alsoIs)
			}

			if tt.wantCategory != sdkerrors.CategoryInternal {
				return
			}

			// The previous rendering is computed on a fresh cause, so an in-place
			// change the helper makes to its own cause cannot hide a difference.
			previous := sdkerrors.NewInternalError(operation, tt.newCause()).Error()
			if tt.wantMessage == "" {
				assert.Equal(t, previous, err.Error(),
					"a failure that is neither network nor timeout must render as before")
			} else {
				assert.Equal(t, tt.wantMessage, err.Error())
				assert.Equal(t, tt.wantPreviousMessage, previous)
			}

			assert.False(t, sdkerrors.IsCancellationError(err))
			assert.False(t, sdkerrors.IsNetworkError(err))
			assert.False(t, sdkerrors.IsTimeoutError(err))
		})
	}

	t.Run("nil stays nil", func(t *testing.T) {
		require.NoError(t, transportErr(operation, nil))
	})
}

type facadeTransportErrorCase struct {
	name                string
	newCause            func() error
	alsoIs              error
	wantCategory        sdkerrors.ErrorCategory
	wantStatus          int
	wantSource          sdkerrors.ErrorSource
	wantSent            bool
	wantMessage         string
	wantPreviousMessage string
}
