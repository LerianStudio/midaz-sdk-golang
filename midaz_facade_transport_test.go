package midaz

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// facadeTransportInput is a valid /v2 create body: the scenarios below fail on
// the wire, never in input validation.
func facadeTransportInput() *models.CreateTransactionV2Input {
	return &models.CreateTransactionV2Input{
		Asset:   "USD",
		Amount:  "10",
		Debits:  []models.TransactionV2Leg{{Alias: "@src", Amount: "10"}},
		Credits: []models.TransactionV2Leg{{Alias: "@dst", Amount: "10"}},
	}
}

// closedPortURL returns the base URL of a local port nobody listens on.
func closedPortURL(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return "http://" + addr
}

// rawTCPServer accepts connections and hands each to handle. Cleanup closes the
// listener and every accepted connection, then waits for the accept loop and
// every handler to return.
func rawTCPServer(t *testing.T, handle func(net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		conns []net.Conn
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()

			wg.Add(1)

			go func() {
				defer wg.Done()
				handle(conn)
			}()
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()

		mu.Lock()
		for _, conn := range conns {
			_ = conn.Close()
		}
		mu.Unlock()

		wg.Wait()
	})

	return "http://" + ln.Addr().String()
}

// readFullRequest reads one request, body included, off conn.
func readFullRequest(conn net.Conn) error {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return err
	}

	_, err = io.Copy(io.Discard, req.Body)

	return err
}

// resetConn closes conn with SO_LINGER 0, so the peer gets a RST, not a FIN.
func resetConn(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}

	_ = conn.Close()
}

// silentHandler reads the request and never answers; it returns once the
// connection is closed by either side.
func silentHandler(conn net.Conn) {
	_ = readFullRequest(conn)
	_, _ = io.Copy(io.Discard, conn)
}

// newFacadeTransportClient builds a real client over both planes at baseURL with
// retries off, plus any extra options.
func newFacadeTransportClient(t *testing.T, baseURL string, opts ...Option) *Client {
	t.Helper()

	all := append([]Option{
		WithConfig(createTestConfig(t)),
		WithLedgerURL(baseURL),
		WithTracerURL(baseURL + "/v1"),
		WithRetryOptions(retry.WithMaxRetries(0)),
	}, opts...)

	c, err := New(all...)
	require.NoError(t, err)

	return c
}

// requireTransportDiagnostics checks the fields every classified transport
// failure carries.
func requireTransportDiagnostics(t *testing.T, err error, operation string, status int) {
	t.Helper()

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, status, sdkErr.StatusCode)
	assert.Equal(t, sdkerrors.ErrorSourceTransport, sdkErr.Source)
	assert.True(t, sdkErr.HTTPRequestSent)
	assert.False(t, sdkErr.HTTPResponseReceived)
	assert.Equal(t, operation, sdkErr.Operation)
	assert.True(t, sdkErr.Retryable())
}

// TestFacadeTransport_RealClient drives the facade through a client built by New
// against local servers that fail at the transport, and requires a network or
// timeout error where the facade used to report an internal one.
func TestFacadeTransport_RealClient(t *testing.T) {
	ctx := context.Background()

	t.Run("diagnostico-de-falha-de-rede/causa-original-preservada", func(t *testing.T) {
		c := newFacadeTransportClient(t, closedPortURL(t))

		_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)

		require.True(t, sdkerrors.IsNetworkError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
		assert.False(t, sdkerrors.IsTimeoutError(err))
		assert.ErrorIs(t, err, syscall.ECONNREFUSED)
		requireTransportDiagnostics(t, err, "V2.Ledgers.Get", http.StatusServiceUnavailable)
	})

	t.Run("timeout-do-cliente/diagnostico-de-timeout", func(t *testing.T) {
		c := newFacadeTransportClient(t, rawTCPServer(t, silentHandler), WithTimeout(200*time.Millisecond))

		_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())

		require.True(t, sdkerrors.IsTimeoutError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
		assert.False(t, sdkerrors.IsNetworkError(err))
		requireTransportDiagnostics(t, err, "V2.Transactions.CreateDirect", http.StatusGatewayTimeout)
	})

	t.Run("deadline-do-contexto", func(t *testing.T) {
		c := newFacadeTransportClient(t, rawTCPServer(t, silentHandler), WithTimeout(30*time.Second))

		deadlineCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()

		_, err := c.V2.Transactions.CreateDirect(deadlineCtx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())

		require.True(t, sdkerrors.IsTimeoutError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
		assert.False(t, sdkerrors.IsNetworkError(err))
	})

	t.Run("reset-depois-do-envio", func(t *testing.T) {
		c := newFacadeTransportClient(t, rawTCPServer(t, func(conn net.Conn) {
			_ = readFullRequest(conn)
			resetConn(conn)
		}))

		_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())

		require.True(t, sdkerrors.IsNetworkError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
		assert.True(t, sdkerrors.HTTPRequestSent(err))
	})

	t.Run("queda-durante-a-leitura-do-corpo", func(t *testing.T) {
		// The server resets only once the client holds the status and headers,
		// so the failure is the body read, not the wait for a response.
		headersSeen := make(chan struct{})
		done := make(chan struct{})
		t.Cleanup(func() { close(done) })

		baseURL := rawTCPServer(t, func(conn net.Conn) {
			if readFullRequest(conn) != nil {
				return
			}

			_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{\"id\":")

			select {
			case <-headersSeen:
			case <-done:
			}

			resetConn(conn)
		})

		var once sync.Once

		c := newFacadeTransportClient(t, baseURL, WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
			return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				resp, err := base.RoundTrip(req)
				if resp != nil {
					once.Do(func() { close(headersSeen) })
				}

				return resp, err
			})
		}))

		_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)

		require.True(t, sdkerrors.IsNetworkError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
	})

	t.Run("retentativas-esgotadas-mantem-classificacao", func(t *testing.T) {
		c := newFacadeTransportClient(t, closedPortURL(t), WithRetryOptions(
			retry.WithMaxRetries(2),
			retry.WithInitialDelay(time.Millisecond),
			retry.WithMaxDelay(2*time.Millisecond),
		))

		_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)

		require.True(t, sdkerrors.IsNetworkError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
	})
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
