// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/auth"
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

// accessManagerTokenError is the shape auth.GetTokenFromAccessManager returns
// when the token request fails on the wire.
func accessManagerTokenError(cause error) error {
	return &auth.AccessManagerTokenRequestError{
		Operation:       "access_manager.token_request",
		Phase:           "token_fetch",
		EndpointScheme:  "http",
		EndpointHost:    "127.0.0.1:1",
		EndpointPath:    "/v1/login/oauth/access_token",
		HTTPRequestSent: true,
		Err:             fmt.Errorf("failed to connect to plugin auth service: %w", cause),
	}
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
		internal("access manager token fetch refused stays internal", func() error {
			return accessManagerTokenError(refusedDialError())
		}),
		internal("access manager token fetch deadline stays internal", func() error {
			return accessManagerTokenError(&url.Error{Op: "Post", URL: "http://127.0.0.1:1/v1/login/oauth/access_token", Err: context.DeadlineExceeded})
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

// newFailingPlanes builds the real plane chain (retry RT -> auth RT ->
// generated clients) over a transport that fails every request with cause, with
// retries off so each call makes exactly one attempt.
func newFailingPlanes(t *testing.T, cause error) *PlaneClients {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: "http://ledger.test",
		tracerURL: "http://tracer.test/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   &http.Client{Transport: transportErrorRoundTripper{err: cause}},
		retryOptions: planeTestRetryOptions(),
	})
	require.NoError(t, err)

	return planes
}

// TestFacadeTransportError_ResponseShapes drives one facade call per response
// shape, one per remaining swapped site and one per Tracer facade over a transport that refuses the connection
// or times out, and requires the classified error, not the internal one
// (porta-fechada-leitura-v2, porta-fechada-escrita-v2,
// porta-fechada-delete-e-count, porta-fechada-fachada-v1,
// porta-fechada-plano-tracer at the facade level).
func TestFacadeTransportError_ResponseShapes(t *testing.T) {
	const (
		orgID    = "11111111-1111-1111-1111-111111111111"
		ledgerID = "22222222-2222-2222-2222-222222222222"
		itemID   = "33333333-3333-3333-3333-333333333333"
	)

	ctx := context.Background()

	shapes := []struct {
		name      string
		operation string
		call      func(*PlaneClients) error
	}{
		{"writeJSON", "Ledgers.Create", func(p *PlaneClients) error {
			_, err := newLedgersFacade(p.Ledger, true).Create(ctx, orgID, &models.CreateLedgerInput{Name: "Treasury"})
			return err
		}},
		{"readOne", "Ledgers.Get", func(p *PlaneClients) error {
			_, err := newLedgersFacade(p.Ledger, true).Get(ctx, orgID, ledgerID)
			return err
		}},
		{"readList", "Ledgers.List", func(p *PlaneClients) error {
			_, err := newLedgersFacade(p.Ledger, true).List(ctx, orgID, models.LedgersListOpts{})
			return err
		}},
		{"readSlice", "MetadataIndexes.List", func(p *PlaneClients) error {
			_, err := newMetadataIndexesFacade(p.Ledger, true).List(ctx, "")
			return err
		}},
		{"deleteResource", "Ledgers.Delete", func(p *PlaneClients) error {
			return newLedgersFacade(p.Ledger, true).Delete(ctx, orgID, ledgerID)
		}},
		{"readCount", "Count", func(p *PlaneClients) error {
			_, err := newLedgersFacade(p.Ledger, true).Count(ctx, orgID)
			return err
		}},
		{"lifecycle v1", "Transactions.Commit", func(p *PlaneClients) error {
			_, err := newTransactionsFacade(p.Ledger, true).Commit(ctx, orgID, ledgerID, itemID)
			return err
		}},
		{"revert v1", "Transactions.Revert", func(p *PlaneClients) error {
			_, err := newTransactionsFacade(p.Ledger, true).Revert(ctx, orgID, ledgerID, itemID)
			return err
		}},
		{"cancel v1", "Transactions.Cancel", func(p *PlaneClients) error {
			_, err := newTransactionsFacade(p.Ledger, true).Cancel(ctx, orgID, ledgerID, itemID)
			return err
		}},
		{"lifecycle v2", "V2.Transactions.Commit", func(p *PlaneClients) error {
			_, err := newTransactionsV2Facade(p.Ledger, true).Commit(ctx, orgID, ledgerID, itemID)
			return err
		}},
		{"cancel v2", "V2.Transactions.Cancel", func(p *PlaneClients) error {
			_, err := newTransactionsV2Facade(p.Ledger, true).Cancel(ctx, orgID, ledgerID, itemID)
			return err
		}},
		{"tracer rules", "Rules.List", func(p *PlaneClients) error {
			_, err := newRulesFacade(p.Tracer, true).List(ctx, models.RulesListOpts{})
			return err
		}},
		{"tracer rules delete", "Rules.Delete", func(p *PlaneClients) error {
			return newRulesFacade(p.Tracer, true).Delete(ctx, itemID)
		}},
		{"tracer rules transition", "Rules.Activate", func(p *PlaneClients) error {
			_, err := newRulesFacade(p.Tracer, true).Activate(ctx, itemID)
			return err
		}},
		{"tracer limits", "Limits.List", func(p *PlaneClients) error {
			_, err := newLimitsFacade(p.Tracer, true).List(ctx, models.LimitsListOpts{})
			return err
		}},
		{"tracer limits delete", "Limits.Delete", func(p *PlaneClients) error {
			return newLimitsFacade(p.Tracer, true).Delete(ctx, itemID)
		}},
		{"tracer limits transition", "Limits.Activate", func(p *PlaneClients) error {
			_, err := newLimitsFacade(p.Tracer, true).Activate(ctx, itemID)
			return err
		}},
		{"tracer validations", "Validations.List", func(p *PlaneClients) error {
			_, err := newValidationsFacade(p.Tracer).List(ctx, models.ValidationsListOpts{})
			return err
		}},
		{"tracer audit events", "AuditEvents.List", func(p *PlaneClients) error {
			_, err := newAuditEventsFacade(p.Tracer).List(ctx, models.AuditEventRecordsListOpts{})
			return err
		}},
		{"tracer reservations", "Reservations.Confirm", func(p *PlaneClients) error {
			_, err := newReservationsFacade(p.Tracer).Confirm(ctx, itemID)
			return err
		}},
	}

	failures := []struct {
		name       string
		newCause   func() error
		predicate  func(error) bool
		cause      error
		wantStatus int
	}{
		{"connection refused", func() error {
			return &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
		}, sdkerrors.IsNetworkError, syscall.ECONNREFUSED, http.StatusServiceUnavailable},
		// The deadline is the cause a client timeout or a context deadline leaves
		// in the chain. A bare net.Error timeout from the transport does not reach
		// the classifier as one: the retry engine wraps it and *url.Error.Timeout
		// only asserts its direct cause.
		{"timeout", func() error { return context.DeadlineExceeded }, sdkerrors.IsTimeoutError, context.DeadlineExceeded, http.StatusGatewayTimeout},
	}

	for _, shape := range shapes {
		for _, failure := range failures {
			t.Run(shape.name+"/"+failure.name, func(t *testing.T) {
				err := shape.call(newFailingPlanes(t, failure.newCause()))

				require.Error(t, err)
				assert.True(t, failure.predicate(err), "got %v", err)
				assert.False(t, sdkerrors.IsInternalError(err), "got %v", err)
				require.ErrorIs(t, err, failure.cause)

				var sdkErr *sdkerrors.Error
				require.ErrorAs(t, err, &sdkErr)
				assert.Equal(t, shape.operation, sdkErr.Operation)
				assert.Equal(t, sdkerrors.ErrorSourceTransport, sdkErr.Source)
				assert.True(t, sdkErr.HTTPRequestSent)
				assert.False(t, sdkErr.HTTPResponseReceived)
				assert.Equal(t, failure.wantStatus, sdkErr.StatusCode)
			})
		}
	}
}

// TestFacadeTransportError_AccessManagerTokenFailure is
// falha-do-access-manager-inalterada: when the real Access Manager token fetch
// is refused or times out, the facade request never leaves, and the facade
// returns the same internal error it returned before transport classification.
func TestFacadeTransportError_AccessManagerTokenFailure(t *testing.T) {
	const (
		orgID    = "11111111-1111-1111-1111-111111111111"
		ledgerID = "22222222-2222-2222-2222-222222222222"
	)

	var ledgerHits atomic.Int32

	ledger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ledgerHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ledger.Close)

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	closedURL := "http://" + closed.Addr().String()
	require.NoError(t, closed.Close())

	// The silent Access Manager holds each token request until the client gives
	// up or the test ends. release is registered after the server, so it runs
	// first and unblocks any handler before the server's Close waits on it.
	release := make(chan struct{})
	silent := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(silent.Close)
	t.Cleanup(func() { close(release) })

	fetches := []struct {
		name        string
		address     string
		tokenClient *http.Client
	}{
		{"refused", closedURL, &http.Client{}},
		{"timeout", silent.URL, &http.Client{Timeout: 200 * time.Millisecond}},
	}

	for _, fetch := range fetches {
		accessManager := auth.AccessManager{
			Enabled:           true,
			Address:           fetch.address,
			ClientID:          "facade-transport-client",
			ClientSecret:      "facade-transport-secret",
			AllowInsecureHTTP: true,
		}

		planes, err := newPlaneClients(planeClientsConfig{
			ledgerURL: ledger.URL,
			tracerURL: ledger.URL + "/v1",
			auth: authRoundTripperConfig{
				tokenProvider: func(ctx context.Context) (string, error) {
					return auth.GetTokenFromAccessManager(ctx, accessManager, fetch.tokenClient)
				},
			},
			httpClient:   ledger.Client(),
			retryOptions: planeTestRetryOptions(),
		})
		require.NoError(t, err)

		calls := []struct {
			name      string
			operation string
			call      func() error
		}{
			{"read", "Ledgers.Get", func() error {
				_, err := newLedgersFacade(planes.Ledger, true).Get(context.Background(), orgID, ledgerID)
				return err
			}},
			{"write", "Ledgers.Create", func() error {
				_, err := newLedgersFacade(planes.Ledger, true).Create(context.Background(), orgID, &models.CreateLedgerInput{Name: "Treasury"})
				return err
			}},
		}

		for _, call := range calls {
			t.Run("falha-do-access-manager-inalterada/"+fetch.name+"/"+call.name, func(t *testing.T) {
				err := call.call()

				require.True(t, sdkerrors.IsInternalError(err), "got %v", err)
				assert.False(t, sdkerrors.IsNetworkError(err))
				assert.False(t, sdkerrors.IsTimeoutError(err))

				var tokenErr *auth.AccessManagerTokenRequestError
				require.ErrorAs(t, err, &tokenErr)

				var sdkErr *sdkerrors.Error
				require.ErrorAs(t, err, &sdkErr)
				assert.Equal(t, http.StatusInternalServerError, sdkErr.StatusCode)
				assert.Equal(t, sdkerrors.ErrorSourceSDK, sdkErr.Source)
				assert.False(t, sdkErr.HTTPRequestSent)
				assert.Equal(t, call.operation, sdkErr.Operation)
			})
		}
	}

	assert.Zero(t, ledgerHits.Load(), "no facade request may reach the ledger when the token fetch fails")
}
