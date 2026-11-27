package midaz

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
// listener, waits for the accept loop to return so no connection can still be
// on its way into the list, closes every accepted connection, then waits for
// every handler to return.
func rawTCPServer(t *testing.T, handle func(net.Conn)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var (
		acceptWG  sync.WaitGroup
		handlerWG sync.WaitGroup
		mu        sync.Mutex
		conns     []net.Conn
	)

	acceptWG.Add(1)

	go func() {
		defer acceptWG.Done()

		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()

			handlerWG.Add(1)

			go func() {
				defer handlerWG.Done()
				handle(conn)
			}()
		}
	}()

	t.Cleanup(func() {
		_ = ln.Close()
		acceptWG.Wait()

		mu.Lock()
		for _, conn := range conns {
			_ = conn.Close()
		}
		mu.Unlock()

		handlerWG.Wait()
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

	t.Run("timeout-de-cabecalho-de-resposta", func(t *testing.T) {
		// A real http.Transport.ResponseHeaderTimeout against the silent server,
		// with the client timeout left long. net/http's response-header timeout
		// error matches context.DeadlineExceeded through its Is method, so the
		// classifier reports a timeout.
		transport := &http.Transport{ResponseHeaderTimeout: 200 * time.Millisecond}
		t.Cleanup(transport.CloseIdleConnections)

		c := newFacadeTransportClient(t, rawTCPServer(t, silentHandler),
			WithHTTPClient(&http.Client{Transport: transport, Timeout: 30 * time.Second}))

		_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)

		require.True(t, sdkerrors.IsTimeoutError(err), "got %v", err)
		assert.False(t, sdkerrors.IsInternalError(err))
		assert.False(t, sdkerrors.IsNetworkError(err))
		requireTransportDiagnostics(t, err, "V2.Ledgers.Get", http.StatusGatewayTimeout)
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
		requireTransportDiagnostics(t, err, "V2.Transactions.CreateDirect", http.StatusServiceUnavailable)
	})

	t.Run("queda-durante-a-leitura-do-corpo", func(t *testing.T) {
		// The server resets only once the client holds the status and headers,
		// so the failure is the body read, not the wait for a response.
		headersSeen := make(chan struct{})
		done := make(chan struct{})

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

		// Registered after the server, so it runs first: a run that never sees
		// the headers releases the handler instead of hanging the server cleanup.
		t.Cleanup(func() { close(done) })

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
		requireTransportDiagnostics(t, err, "V2.Ledgers.Get", http.StatusServiceUnavailable)
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

// TestFacadeTransport_SendAttempts is tentativas-de-envio-inalteradas: with
// WithMaxRetries(2) against a closed port, the number of wire attempts a read and
// a write make is the one the SDK made before the facade classified transport
// failures: 1 + 2 retries for both, measured on the previous SDK with this same
// test. The classification runs after the retry round tripper returns, so it
// cannot change the count.
func TestFacadeTransport_SendAttempts(t *testing.T) {
	ctx := context.Background()

	calls := []struct {
		name         string
		wantAttempts int
		call         func(*Client) error
	}{
		{"read V2.Ledgers.Get", 3, func(c *Client) error {
			_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)
			return err
		}},
		{"write V2.Transactions.CreateDirect", 3, func(c *Client) error {
			_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())
			return err
		}},
	}

	for _, tt := range calls {
		t.Run("tentativas-de-envio-inalteradas/"+tt.name, func(t *testing.T) {
			var recorder *recordingTransport

			c, err := New(
				WithConfig(createTestConfig(t)),
				WithLedgerURL(closedPortURL(t)),
				WithTracerURL(closedPortURL(t)+"/v1"),
				WithRetryOptions(
					retry.WithMaxRetries(2),
					retry.WithInitialDelay(time.Millisecond),
					retry.WithMaxDelay(2*time.Millisecond),
				),
				WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
					recorder = &recordingTransport{base: base}
					return recorder
				}),
			)
			require.NoError(t, err)

			require.Error(t, tt.call(c))
			require.NotNil(t, recorder)
			assert.Len(t, recorder.requests(), tt.wantAttempts)
		})
	}
}

// TestFacadeTransport_CleanClose is fechamento-limpo-sem-resposta-inalterado: a
// server that reads the request and closes with a FIN, no reset and no
// response, leaves a cause the classifier does not recognize as network, so the
// facade keeps the internal error it returned before.
func TestFacadeTransport_CleanClose(t *testing.T) {
	baseURL := rawTCPServer(t, func(conn net.Conn) {
		_ = readFullRequest(conn)
		_ = conn.Close()
	})

	c := newFacadeTransportClient(t, baseURL)

	_, err := c.V2.Ledgers.Get(context.Background(), transportTestOrgID, transportTestLedgerID)

	require.True(t, sdkerrors.IsInternalError(err), "got %v", err)
	assert.False(t, sdkerrors.IsNetworkError(err))
	assert.False(t, sdkerrors.IsTimeoutError(err))
	assert.False(t, sdkerrors.HTTPRequestSent(err))

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, http.StatusInternalServerError, sdkErr.StatusCode)
	assert.Equal(t, sdkerrors.ErrorSourceSDK, sdkErr.Source)
}

// TestFacadeTransport_ClosedPort is the closed-port scenarios through a client
// built by New: the real dial and the *url.Error shape http.Client returns, on
// a write, a delete, a count, the /v1 facade and the Tracer plane.
func TestFacadeTransport_ClosedPort(t *testing.T) {
	ctx := context.Background()

	calls := []struct {
		name string
		call func(*Client) error
	}{
		{"porta-fechada-escrita-v2/V2.Transactions.CreateDirect", func(c *Client) error {
			_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())
			return err
		}},
		{"porta-fechada-delete-e-count/V2.Ledgers.Delete", func(c *Client) error {
			return c.V2.Ledgers.Delete(ctx, transportTestOrgID, transportTestLedgerID)
		}},
		{"porta-fechada-delete-e-count/V2.Ledgers.Count", func(c *Client) error {
			_, err := c.V2.Ledgers.Count(ctx, transportTestOrgID)
			return err
		}},
		{"porta-fechada-fachada-v1/V1.Transactions.Get", func(c *Client) error {
			_, err := c.V1.Transactions.Get(ctx, transportTestOrgID, transportTestLedgerID, transportTestLedgerID)
			return err
		}},
		{"porta-fechada-fachada-v1/V1.Transactions.Commit", func(c *Client) error {
			_, err := c.V1.Transactions.Commit(ctx, transportTestOrgID, transportTestLedgerID, transportTestLedgerID)
			return err
		}},
		{"porta-fechada-plano-tracer/Rules.List", func(c *Client) error {
			_, err := c.Rules.List(ctx, models.RulesListOpts{})
			return err
		}},
	}

	for _, tt := range calls {
		t.Run(tt.name, func(t *testing.T) {
			// Both planes point at closed ports, so the Tracer row fails on the
			// Tracer URL and every other row on the Ledger URL.
			c, err := New(
				WithConfig(createTestConfig(t)),
				WithLedgerURL(closedPortURL(t)),
				WithTracerURL(closedPortURL(t)+"/v1"),
				WithRetryOptions(retry.WithMaxRetries(0)),
			)
			require.NoError(t, err)

			err = tt.call(c)

			require.True(t, sdkerrors.IsNetworkError(err), "got %v", err)
			assert.False(t, sdkerrors.IsInternalError(err))
			assert.False(t, sdkerrors.IsTimeoutError(err))
			assert.ErrorIs(t, err, syscall.ECONNREFUSED)
		})
	}
}

// TestFacadeTransport_Cancellation is cancelamento-inalterado: a caller that
// cancels its context while the request waits on a silent server still gets the
// internal error the facade returned before, not a cancellation error.
func TestFacadeTransport_Cancellation(t *testing.T) {
	arrived := make(chan struct{})

	var once sync.Once

	baseURL := rawTCPServer(t, func(conn net.Conn) {
		if readFullRequest(conn) == nil {
			once.Do(func() { close(arrived) })
		}

		_, _ = io.Copy(io.Discard, conn)
	})

	c := newFacadeTransportClient(t, baseURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan struct{})
	defer close(stop)

	go func() {
		select {
		case <-arrived:
			cancel()
		case <-stop:
		}
	}()

	_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())

	require.True(t, sdkerrors.IsInternalError(err), "got %v", err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, sdkerrors.IsCancellationError(err))
	assert.False(t, sdkerrors.IsNetworkError(err))
	assert.False(t, sdkerrors.IsTimeoutError(err))
}

// TestFacadeTransport_KnownLimitTimeoutNotRecognized pins a KNOWN LIMIT, not the
// desired behavior: a timeout the transport reports only through Timeout(),
// with no context.DeadlineExceeded in its chain, does not come out of the
// facade as a timeout error.
//
// The mechanism: the retry engine wraps the last attempt's error
// ("retry: retries exhausted: ...: %w"), and http.Client wraps that in a
// *url.Error. *url.Error.Timeout() only type-asserts its DIRECT cause, which is
// now the retry wrapper, so it reports false; the classifier takes the first
// net.Error in the chain, that *url.Error, and never sees the inner Timeout().
//
// A client timeout, a context deadline and the response-header timeout are
// unaffected: all three leave something that matches context.DeadlineExceeded
// in the chain (see timeout-do-cliente, deadline-do-contexto and
// timeout-de-cabecalho-de-resposta).
func TestFacadeTransport_KnownLimitTimeoutNotRecognized(t *testing.T) {
	// An https plane URL on the silent server, so the TLS handshake
	// never completes and http.Transport.TLSHandshakeTimeout fires. net/http
	// reports it as "net/http: TLS handshake timeout", an error with Timeout()
	// true and no Is method, so nothing in the chain matches
	// context.DeadlineExceeded. The client timeout is left long.
	t.Run("known-limit/tls-handshake-timeout-comes-out-internal-not-timeout", func(t *testing.T) {
		baseURL := strings.Replace(rawTCPServer(t, silentHandler), "http://", "https://", 1)

		transport := &http.Transport{TLSHandshakeTimeout: 200 * time.Millisecond}
		t.Cleanup(transport.CloseIdleConnections)

		c := newFacadeTransportClient(t, baseURL, WithHTTPClient(&http.Client{Transport: transport, Timeout: 30 * time.Second}))

		_, err := c.V2.Ledgers.Get(context.Background(), transportTestOrgID, transportTestLedgerID)

		require.True(t, sdkerrors.IsInternalError(err), "got %v", err)
		assert.False(t, sdkerrors.IsTimeoutError(err))
		assert.False(t, sdkerrors.IsNetworkError(err))
	})
}

// accessManagerSimulator stands in for the Access Manager in the token-path
// scenarios. answer decides every token request; a held request is released
// when the client gives up or, at cleanup, before the server closes.
func accessManagerSimulator(t *testing.T, answer func(w http.ResponseWriter, r *http.Request, release <-chan struct{})) string {
	t.Helper()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, release)
	}))

	// Registered in this order so the release runs first (LIFO) and a held
	// handler returns before Close waits on it.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	return srv.URL
}

// silentAccessManager holds every token request.
func silentAccessManager(_ http.ResponseWriter, r *http.Request, release <-chan struct{}) {
	select {
	case <-r.Context().Done():
	case <-release:
	}
}

// newAccessManagerFacadeClient builds a client through the public path:
// midaz.New with WithAccessManager. The credential comes from a
// CredentialSource, so midaz.New does not fetch a token while it builds the
// client (it only does that for a static ClientID/ClientSecret); construction
// therefore cannot be starved by the short timeouts below, and every facade
// call fetches a token. It returns the client and the number of requests the
// ledger server received.
func newAccessManagerFacadeClient(t *testing.T, accessManagerURL string, opts ...Option) (*Client, *atomic.Int32) {
	t.Helper()

	var ledgerHits atomic.Int32

	ledger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ledgerHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ledger.Close)

	all := append([]Option{
		WithLedgerURL(ledger.URL),
		WithTracerURL(ledger.URL + "/v1"),
		WithRetryOptions(retry.WithMaxRetries(0)),
		WithAccessManager(AccessManager{
			Enabled: true,
			Address: accessManagerURL,
			CredentialSource: CredentialSourceFunc(func(context.Context) (string, string, error) {
				return "facade-transport-client", "facade-transport-secret", nil
			}),
			AllowInsecureHTTP: true,
		}),
	}, opts...)

	c, err := New(all...)
	require.NoError(t, err)

	return c, &ledgerHits
}

// accessManagerFacadeCalls is one read and one write.
var accessManagerFacadeCalls = []struct {
	name      string
	operation string
	call      func(context.Context, *Client) error
}{
	{"read", "V2.Ledgers.Get", func(ctx context.Context, c *Client) error {
		_, err := c.V2.Ledgers.Get(ctx, transportTestOrgID, transportTestLedgerID)
		return err
	}},
	{"write", "V2.Transactions.CreateDirect", func(ctx context.Context, c *Client) error {
		_, err := c.V2.Transactions.CreateDirect(ctx, transportTestOrgID, transportTestLedgerID, facadeTransportInput())
		return err
	}},
}

// TestFacadeTransport_AccessManagerTokenFailure drives a failing Access Manager
// token fetch through the public path (midaz.New with WithAccessManager).
//
// falha-do-access-manager-inalterada: the token request is refused, or the
// Access Manager answers 500 or with an unreadable token. The typed
// *auth.AccessManagerTokenRequestError reaches the facade, which keeps the
// internal error: the ledger request never left.
//
// prazo-estourado-durante-o-token: the Access Manager holds the token request
// until the client timeout (WithTimeout) or the caller's context deadline
// expires. An expired deadline is a timeout wherever the time was spent, so the
// facade reports a timeout although the ledger request was never sent. Which
// error carries the deadline depends on a timer race (the token request's own
// deadline wrapped in the typed token error, net/http's text-only
// Client.Timeout error, or a bare context error); every form matches
// context.DeadlineExceeded, so the outcome does not depend on the race.
func TestFacadeTransport_AccessManagerTokenFailure(t *testing.T) {
	internalCases := []struct {
		name   string
		answer func(t *testing.T) string
	}{
		{"refused", func(t *testing.T) string { return closedPortURL(t) }},
		{"bad-answer-500", func(t *testing.T) string {
			return accessManagerSimulator(t, func(w http.ResponseWriter, _ *http.Request, _ <-chan struct{}) {
				w.WriteHeader(http.StatusInternalServerError)
			})
		}},
		{"bad-answer-unreadable-token", func(t *testing.T) string {
			return accessManagerSimulator(t, func(w http.ResponseWriter, _ *http.Request, _ <-chan struct{}) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("not a token"))
			})
		}},
	}

	for _, tc := range internalCases {
		for _, call := range accessManagerFacadeCalls {
			t.Run("falha-do-access-manager-inalterada/"+tc.name+"/"+call.name, func(t *testing.T) {
				c, ledgerHits := newAccessManagerFacadeClient(t, tc.answer(t))

				err := call.call(context.Background(), c)

				require.True(t, sdkerrors.IsInternalError(err), "got %v", err)
				assert.False(t, sdkerrors.IsNetworkError(err))
				assert.False(t, sdkerrors.IsTimeoutError(err))

				var sdkErr *sdkerrors.Error
				require.ErrorAs(t, err, &sdkErr)
				assert.Equal(t, http.StatusInternalServerError, sdkErr.StatusCode)
				assert.Equal(t, sdkerrors.ErrorSourceSDK, sdkErr.Source)
				assert.False(t, sdkErr.HTTPRequestSent)
				assert.Zero(t, ledgerHits.Load())
			})
		}
	}

	deadlineCases := []struct {
		name    string
		options []Option
		ctx     func(t *testing.T) context.Context
	}{
		{"client-timeout", []Option{WithTimeout(300 * time.Millisecond)}, func(*testing.T) context.Context { return context.Background() }},
		{"context-deadline", []Option{WithTimeout(30 * time.Second)}, func(t *testing.T) context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			t.Cleanup(cancel)

			return ctx
		}},
	}

	for _, tc := range deadlineCases {
		for _, call := range accessManagerFacadeCalls {
			t.Run("prazo-estourado-durante-o-token/"+tc.name+"/"+call.name, func(t *testing.T) {
				c, ledgerHits := newAccessManagerFacadeClient(t, accessManagerSimulator(t, silentAccessManager), tc.options...)

				err := call.call(tc.ctx(t), c)

				require.True(t, sdkerrors.IsTimeoutError(err), "got %v", err)
				assert.False(t, sdkerrors.IsInternalError(err))
				assert.False(t, sdkerrors.IsNetworkError(err))

				var sdkErr *sdkerrors.Error
				require.ErrorAs(t, err, &sdkErr)
				assert.Equal(t, http.StatusGatewayTimeout, sdkErr.StatusCode)
				assert.Equal(t, sdkerrors.ErrorSourceTransport, sdkErr.Source)
				assert.True(t, sdkErr.HTTPRequestSent)
				assert.Equal(t, call.operation, sdkErr.Operation)
				assert.Zero(t, ledgerHits.Load())
			})
		}
	}
}
