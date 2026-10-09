package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	transportTestOrgID    = "0190a7c6-1234-7abc-8def-0123456789ab"
	transportTestLedgerID = "0190a7c6-5678-7abc-8def-0123456789ab"
	transportTestToken    = "transport-test-token"
)

// recordedRequest is what the caller-installed round tripper observed.
type recordedRequest struct {
	path          string
	authorization string
}

// recordingTransport is the caller's round tripper: it records every request it
// sees and delegates to the base the SDK handed it.
type recordingTransport struct {
	base http.RoundTripper

	mu   sync.Mutex
	seen []recordedRequest
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.seen = append(r.seen, recordedRequest{path: req.URL.Path, authorization: req.Header.Get("Authorization")})
	r.mu.Unlock()

	return r.base.RoundTrip(req)
}

func (r *recordingTransport) requests() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]recordedRequest(nil), r.seen...)
}

// transportTestServer serves the Access Manager token endpoint and answers every
// other path with an empty page. The first request to failOncePath answers 503
// so the SDK retry chain has to replay it.
func transportTestServer(t *testing.T, failOncePath string) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu     sync.Mutex
		paths  []string
		failed bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		failNow := r.URL.Path == failOncePath && !failed
		if failNow {
			failed = true
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v1/login/oauth/access_token" {
			_, _ = w.Write([]byte(`{"accessToken":"` + transportTestToken + `","expiresAt":"` +
				time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`))

			return
		}

		if failNow {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"about:blank","title":"unavailable","status":503}`))

			return
		}

		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return append([]string(nil), paths...)
	}
}

// TestWithHTTPTransport_SeesEveryRequestOfEveryServiceClient proves the option
// reaches every HTTP request the SDK makes — the Access Manager token exchange,
// every Ledger-plane service and the Tracer plane — while the SDK's own
// behaviour around it is preserved: the Bearer header is already on the request
// (auth sits above the caller's transport), a 503 is replayed through the caller's
// transport (retry sits above it), the base handed to the wrapper is the SDK's
// own tuned pooled transport, and WithTimeout still bounds the client in either
// option order.
func TestWithHTTPTransport_SeesEveryRequestOfEveryServiceClient(t *testing.T) {
	for _, order := range []string{"transport-before-timeout", "transport-after-timeout"} {
		t.Run(order, func(t *testing.T) {
			retriedPath := "/v1/organizations/" + transportTestOrgID + "/ledgers"
			srv, serverPaths := transportTestServer(t, retriedPath)

			var (
				recorder *recordingTransport
				gotBase  http.RoundTripper
			)

			transportOpt := WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
				gotBase = base
				recorder = &recordingTransport{base: base}

				return recorder
			})

			opts := []Option{
				WithEnvironment(config.EnvironmentLocal),
				WithAccessManager(AccessManager{Address: srv.URL, ClientID: "id-" + order, ClientSecret: "secret"}),
				WithLedgerURL(srv.URL),
				WithTracerURL(srv.URL + "/v1"),
				WithRetryOptions(retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(2*time.Millisecond)),
			}
			if order == "transport-before-timeout" {
				opts = append(opts, transportOpt, WithTimeout(7*time.Second))
			} else {
				opts = append(opts, WithTimeout(7*time.Second), transportOpt)
			}

			c, err := New(opts...)
			require.NoError(t, err)
			require.NotNil(t, recorder, "the wrapper must be invoked at construction")

			sdkTransport, ok := gotBase.(*http.Transport)
			require.True(t, ok, "base must be the SDK's own pooled transport, got %T", gotBase)
			assert.Equal(t, 30*time.Second, sdkTransport.ResponseHeaderTimeout,
				"base must keep the SDK default transport tuning")
			assert.Equal(t, 7*time.Second, c.GetConfig().HTTPClient.Timeout, "WithTimeout must still bound the client")

			ctx := context.Background()

			_, err = c.V1.Organizations.List(ctx, models.OrganizationsListOpts{})
			require.NoError(t, err)
			_, err = c.V1.Ledgers.List(ctx, transportTestOrgID, models.LedgersListOpts{})
			require.NoError(t, err)
			_, err = c.V1.Accounts.List(ctx, transportTestOrgID, transportTestLedgerID, models.AccountsListOpts{})
			require.NoError(t, err)
			_, err = c.V1.Transactions.List(ctx, transportTestOrgID, transportTestLedgerID, models.TransactionsListOpts{})
			require.NoError(t, err)
			_, err = c.V1.Balances.ListBalances(ctx, transportTestOrgID, transportTestLedgerID, models.BalancesListOpts{})
			require.NoError(t, err)
			_, err = c.Rules.List(ctx, models.RulesListOpts{})
			require.NoError(t, err)

			seen := recorder.requests()

			seenPaths := make([]string, 0, len(seen))
			for _, r := range seen {
				seenPaths = append(seenPaths, r.path)
			}

			assert.Equal(t, serverPaths(), seenPaths,
				"every request the server received must have crossed the caller's transport, in order")

			wantData := []string{
				"/v1/organizations",
				retriedPath,
				"/v1/organizations/" + transportTestOrgID + "/ledgers/" + transportTestLedgerID + "/accounts",
				"/v1/organizations/" + transportTestOrgID + "/ledgers/" + transportTestLedgerID + "/transactions",
				"/v1/organizations/" + transportTestOrgID + "/ledgers/" + transportTestLedgerID + "/balances",
				"/v1/rules",
			}

			attempts := map[string]int{}
			tokenExchanges := 0

			for _, r := range seen {
				if r.path == "/v1/login/oauth/access_token" {
					tokenExchanges++

					continue
				}

				attempts[r.path]++
				assert.Equal(t, "Bearer "+transportTestToken, r.authorization,
					"auth must already be applied when the caller's transport sees %s", r.path)
			}

			assert.Positive(t, tokenExchanges, "the Access Manager token exchange must cross the caller's transport")

			for _, p := range wantData {
				assert.Contains(t, attempts, p, "service request %s must cross the caller's transport", p)
			}

			assert.Equal(t, 2, attempts[retriedPath], "the retried request must cross the caller's transport on every attempt")
			assert.Len(t, attempts, len(wantData), "no unexpected service request")
		})
	}
}

// TestWithHTTPTransport_ComposesWithCallerHTTPClient proves that, in either
// option order, the wrapper receives the caller-supplied client's transport as
// its base and the caller's own *http.Client is never mutated.
func TestWithHTTPTransport_ComposesWithCallerHTTPClient(t *testing.T) {
	for _, order := range []string{"client-first", "transport-first"} {
		t.Run(order, func(t *testing.T) {
			callerBase := &recordingTransport{base: http.DefaultTransport}
			callerClient := &http.Client{Transport: callerBase, Timeout: 3 * time.Second}

			var gotBase http.RoundTripper

			transportOpt := WithHTTPTransport(func(base http.RoundTripper) http.RoundTripper {
				gotBase = base

				return &recordingTransport{base: base}
			})

			opts := []Option{WithConfig(createTestConfig(t))}
			if order == "client-first" {
				opts = append(opts, WithHTTPClient(callerClient), transportOpt)
			} else {
				opts = append(opts, transportOpt, WithHTTPClient(callerClient))
			}

			c, err := New(opts...)
			require.NoError(t, err)

			assert.Same(t, callerBase, gotBase, "the wrapper must wrap the caller client's transport")
			assert.Same(t, callerBase, callerClient.Transport, "the caller's client must not be mutated")
			assert.NotSame(t, callerClient, c.GetConfig().HTTPClient, "the SDK must install a copy, not the caller's client")
			assert.IsType(t, &recordingTransport{}, c.GetConfig().HTTPClient.Transport)
			assert.NotSame(t, callerBase, c.GetConfig().HTTPClient.Transport)
			assert.Equal(t, 3*time.Second, c.GetConfig().HTTPClient.Timeout, "the caller client's timeout must be kept")
		})
	}
}

func TestWithHTTPTransport_RejectsNilWrapperAndNilTransport(t *testing.T) {
	_, err := New(WithConfig(createTestConfig(t)), WithHTTPTransport(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option at index 1")

	_, err = New(WithConfig(createTestConfig(t)), WithHTTPTransport(func(http.RoundTripper) http.RoundTripper { return nil }))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "option at index 1")
}
