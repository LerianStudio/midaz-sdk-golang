package midaz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The actor relay through the full client (midaz.New). Twin of
// midaz_trace_propagation_test.go. Subtest names carry the scenario names of
// specs/ledger-actor-relay/spec.md so each one can be traced to its test.

const (
	// actorTestHeader is the header name the relay writes; the SDK keeps its
	// own constant unexported on purpose.
	actorTestHeader = "X-Lerian-Actor"

	// Obviously fake partner tokens.
	actorTestTokenA = "tok-A"
	actorTestTokenB = "tok-B"

	actorTestTokenPath = "/v1/login/oauth/access_token"
)

// actorTestCtxKey keys the fake partner token in a test context.
type actorTestCtxKey struct{}

// actorTestSource is the consumer's context reader handed to WithActorRelay.
func actorTestSource(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(actorTestCtxKey{}).(string)

	return token, ok
}

func actorTestContext(token string) context.Context {
	return context.WithValue(context.Background(), actorTestCtxKey{}, token)
}

// actorTestRequest is what the fake server received.
type actorTestRequest struct {
	method string
	host   string
	path   string
	actor  []string
	body   string
}

// actorTestServer serves Ledger, Tracer and the Access Manager token exchange
// from one host and records every request. When unauthorizedOnce is set, the
// first non-token request answers 401 so the auth round tripper refreshes the
// access token and replays.
type actorTestServer struct {
	srv *httptest.Server

	mu   sync.Mutex
	seen []actorTestRequest
}

func newActorTestServer(t *testing.T, unauthorizedOnce bool) *actorTestServer {
	t.Helper()

	s := &actorTestServer{}

	var answered401 atomic.Bool

	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		s.mu.Lock()
		s.seen = append(s.seen, actorTestRequest{
			method: r.Method, host: r.Host, path: r.URL.Path,
			actor: r.Header.Values(actorTestHeader), body: string(body),
		})
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == actorTestTokenPath {
			_, _ = w.Write([]byte(`{"accessToken":"` + transportTestToken + `","expiresAt":"` +
				time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`))

			return
		}

		if unauthorizedOnce && answered401.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"AUT-1004","title":"Unauthorized","message":"expired"}`))

			return
		}

		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"` + transportTestOrgID + `","legalName":"Acme","legalDocument":"123"}`))

			return
		}

		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	t.Cleanup(s.srv.Close)

	return s
}

func (s *actorTestServer) requests() []actorTestRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]actorTestRequest(nil), s.seen...)
}

func (s *actorTestServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seen = nil
}

// actorTestClient builds a client on the fake server with a fixed Access
// Manager credential. clientID keeps the process-wide token cache apart
// between subtests.
func actorTestClient(t *testing.T, s *actorTestServer, clientID string, opts ...Option) *Client {
	t.Helper()

	all := append([]Option{
		WithEnvironment(config.EnvironmentLocal),
		WithAccessManager(AccessManager{Address: s.srv.URL, ClientID: clientID, ClientSecret: "secret"}),
		WithLedgerURL(s.srv.URL),
		WithTracerURL(s.srv.URL + "/v1"),
		WithRetryOptions(retry.WithMaxRetries(0)),
	}, opts...)

	c, err := New(all...)
	require.NoError(t, err)

	return c
}

func createOrgInput() *models.CreateOrganizationInput {
	return models.NewCreateOrganizationInput("Acme", "123")
}

// TestActorRelay_LedgerPlane covers the relay on every Ledger surface.
func TestActorRelay_LedgerPlane(t *testing.T) {
	t.Run("relay-em-leitura-v2", func(t *testing.T) {
		s := newActorTestServer(t, false)
		c := actorTestClient(t, s, "actor-read-v2", WithActorRelay(actorTestSource))
		s.reset()

		_, err := c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		got := s.requests()
		require.Len(t, got, 1)
		assert.Equal(t, "/v2/organizations", got[0].path)
		assert.Equal(t, []string{actorTestTokenA}, got[0].actor)
	})

	t.Run("relay-em-escrita-v2", func(t *testing.T) {
		withSrv := newActorTestServer(t, false)
		withOption := actorTestClient(t, withSrv, "actor-write-v2-on", WithActorRelay(actorTestSource))
		withSrv.reset()

		withoutSrv := newActorTestServer(t, false)
		withoutOption := actorTestClient(t, withoutSrv, "actor-write-v2-off")
		withoutSrv.reset()

		ctx := actorTestContext(actorTestTokenA)

		_, err := withOption.V2.Organizations.Create(ctx, createOrgInput())
		require.NoError(t, err)
		_, err = withoutOption.V2.Organizations.Create(ctx, createOrgInput())
		require.NoError(t, err)

		got, want := withSrv.requests(), withoutSrv.requests()
		require.Len(t, got, 1)
		require.Len(t, want, 1)
		assert.Equal(t, http.MethodPost, got[0].method)
		assert.Equal(t, "/v2/organizations", got[0].path)
		assert.Equal(t, []string{actorTestTokenA}, got[0].actor)
		assert.Empty(t, want[0].actor)
		assert.JSONEq(t, want[0].body, got[0].body, "the JSON body must be the one sent without the option")
	})

	t.Run("relay-na-fachada-v1", func(t *testing.T) {
		s := newActorTestServer(t, false)
		c := actorTestClient(t, s, "actor-v1", WithActorRelay(actorTestSource))
		s.reset()

		ctx := actorTestContext(actorTestTokenA)

		_, err := c.V1.Organizations.List(ctx, models.OrganizationsListOpts{})
		require.NoError(t, err)
		_, err = c.V1.Organizations.Create(ctx, createOrgInput())
		require.NoError(t, err)

		got := s.requests()
		require.Len(t, got, 2)

		for _, req := range got {
			assert.Equal(t, "/v1/organizations", req.path)
			assert.Equal(t, []string{actorTestTokenA}, req.actor, "%s must carry the actor", req.method)
		}
	})

	t.Run("relay-na-url-default", func(t *testing.T) {
		var (
			mu   sync.Mutex
			seen []*http.Request
		)

		// Self-answering transport: nothing leaves the process.
		answer := WithHTTPTransport(func(http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				seen = append(seen, req.Clone(req.Context()))
				mu.Unlock()

				return jsonResponse(req, http.StatusOK, `{"items":[],"limit":10}`), nil
			})
		})

		c, err := New(WithEnvironment(config.EnvironmentLocal), WithAnonymous(), answer, WithActorRelay(actorTestSource))
		require.NoError(t, err)

		_, err = c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()

		require.Len(t, seen, 1)
		assert.Equal(t, "localhost:3002", seen[0].URL.Host, "the request must go to the local default Ledger URL")
		assert.Equal(t, "/v2/organizations", seen[0].URL.Path)
		assert.Equal(t, []string{actorTestTokenA}, seen[0].Header.Values(actorTestHeader))
	})
}

// TestActorRelay_OtherDestinations proves the actor reaches the Ledger plane
// only, with every destination on one host.
func TestActorRelay_OtherDestinations(t *testing.T) {
	t.Run("access-manager-nao-recebe-credencial-fixa", func(t *testing.T) {
		// The 401 forces a token exchange inside the call, with the call's
		// context, so the exchange is not only the one midaz.New makes.
		s := newActorTestServer(t, true)
		c := actorTestClient(t, s, "actor-am-fixed", WithActorRelay(actorTestSource))
		s.reset()

		_, err := c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		assertActorOnlyOnLedger(t, s.requests(), true)
	})

	t.Run("access-manager-nao-recebe-credencial-por-requisicao", func(t *testing.T) {
		s := newActorTestServer(t, false)

		c, err := New(
			WithEnvironment(config.EnvironmentLocal),
			WithAccessManager(AccessManager{
				Address: s.srv.URL,
				CredentialSource: CredentialSourceFunc(func(context.Context) (string, string, error) {
					return "actor-am-per-request", "secret", nil
				}),
			}),
			WithLedgerURL(s.srv.URL),
			WithTracerURL(s.srv.URL+"/v1"),
			WithRetryOptions(retry.WithMaxRetries(0)),
			WithActorRelay(actorTestSource),
		)
		require.NoError(t, err)
		s.reset()

		_, err = c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		assertActorOnlyOnLedger(t, s.requests(), true)
	})

	t.Run("mesmo-host-para-ledger-tracer-e-access-manager", func(t *testing.T) {
		s := newActorTestServer(t, true)
		c := actorTestClient(t, s, "actor-same-host", WithActorRelay(actorTestSource))
		s.reset()

		ctx := actorTestContext(actorTestTokenA)

		_, err := c.V2.Organizations.List(ctx, models.OrganizationsListOpts{})
		require.NoError(t, err)
		_, err = c.Rules.List(ctx, models.RulesListOpts{})
		require.NoError(t, err)

		got := s.requests()

		hosts := map[string]bool{}
		for _, req := range got {
			hosts[req.host] = true
		}

		require.Len(t, hosts, 1, "Ledger, Tracer and Access Manager must share one host")
		assertActorOnlyOnLedger(t, got, true)

		var tracerSeen bool

		for _, req := range got {
			if req.path == "/v1/rules" {
				tracerSeen = true

				assert.Empty(t, req.actor, "the Tracer must not receive the actor")
			}
		}

		assert.True(t, tracerSeen, "the Tracer call must have reached the server")
	})

	t.Run("relay-mantido-no-replay-de-401", func(t *testing.T) {
		s := newActorTestServer(t, true)
		c := actorTestClient(t, s, "actor-401-replay", WithActorRelay(actorTestSource))
		s.reset()

		_, err := c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		got := s.requests()
		require.Len(t, got, 3, "401, token refresh, replay")
		assert.Equal(t, "/v2/organizations", got[0].path)
		assert.Equal(t, actorTestTokenPath, got[1].path)
		assert.Equal(t, "/v2/organizations", got[2].path)
		assert.Equal(t, []string{actorTestTokenA}, got[0].actor, "the first attempt must carry the actor")
		assert.Empty(t, got[1].actor, "the token refresh must not carry the actor")
		assert.Equal(t, []string{actorTestTokenA}, got[2].actor, "the replay must carry the actor")
	})
}

// assertActorOnlyOnLedger checks that every token exchange lacks the actor and
// every Ledger request carries tok-A. wantExchange requires at least one
// exchange among the requests.
func assertActorOnlyOnLedger(t *testing.T, got []actorTestRequest, wantExchange bool) {
	t.Helper()

	var exchanges, ledger int

	for _, req := range got {
		switch {
		case req.path == actorTestTokenPath:
			exchanges++

			assert.Empty(t, req.actor, "the token exchange must not carry the actor")
		case strings.HasPrefix(req.path, "/v2/") || req.path == "/v1/organizations":
			ledger++

			assert.Equal(t, []string{actorTestTokenA}, req.actor, "Ledger %s %s must carry the actor", req.method, req.path)
		}
	}

	if wantExchange {
		assert.Positive(t, exchanges, "a token exchange must have happened during the call")
	}

	assert.Positive(t, ledger, "a Ledger request must have reached the server")
}

// TestActorRelay_PerCallAndCoexistence covers per-call resolution and the
// coexistence with a relay the consumer already does in WithHTTPTransport.
func TestActorRelay_PerCallAndCoexistence(t *testing.T) {
	t.Run("token-por-chamada", func(t *testing.T) {
		s := newActorTestServer(t, false)
		c := actorTestClient(t, s, "actor-per-call", WithActorRelay(actorTestSource))
		s.reset()

		const callsPerToken = 25

		tokens := []string{actorTestTokenA, actorTestTokenB}
		start := make(chan struct{})
		errs := make(chan error, callsPerToken*len(tokens))

		var wg sync.WaitGroup

		for i := range callsPerToken * len(tokens) {
			token := tokens[i%len(tokens)]

			wg.Add(1)

			go func() {
				defer wg.Done()

				<-start

				// The organization id names the token, so the server side can
				// tell which context each request came from.
				_, err := c.V2.Organizations.Get(actorTestContext(token), "org-"+token)
				errs <- err
			}()
		}

		close(start)
		wg.Wait()
		close(errs)

		for err := range errs {
			require.NoError(t, err)
		}

		got := s.requests()
		require.Len(t, got, callsPerToken*len(tokens))

		for _, req := range got {
			token := strings.TrimPrefix(req.path, "/v2/organizations/org-")
			assert.Equal(t, []string{token}, req.actor, "request %s must carry its own context's token", req.path)
		}
	})

	t.Run("cola-existente-continua-funcionando", func(t *testing.T) {
		s := newActorTestServer(t, false)
		c := actorTestClient(t, s, "actor-glue-only", WithHTTPTransport(actorGlue))
		s.reset()

		_, err := c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		got := s.requests()
		require.Len(t, got, 1)
		assert.Equal(t, []string{actorTestTokenA}, got[0].actor, "the consumer's own relay must keep working")
	})

	t.Run("cola-existente-junto-com-a-opcao", func(t *testing.T) {
		s := newActorTestServer(t, false)
		c := actorTestClient(t, s, "actor-glue-and-option", WithHTTPTransport(actorGlue), WithActorRelay(actorTestSource))
		s.reset()

		_, err := c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.NoError(t, err)

		got := s.requests()
		require.Len(t, got, 1)
		assert.Equal(t, []string{actorTestTokenA}, got[0].actor, "the header must appear once, with that token")
	})
}

// actorGlue is the relay a plugin writes today over WithHTTPTransport: it sets
// the header from the request context on every request it sees.
func actorGlue(base http.RoundTripper) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if token, ok := actorTestSource(req.Context()); ok && req.URL.Path != actorTestTokenPath {
			req = req.Clone(req.Context())
			req.Header.Set(actorTestHeader, token)
		}

		return base.RoundTrip(req)
	})
}

// TestActorRelay_TokenProtection covers error text and redirects.
func TestActorRelay_TokenProtection(t *testing.T) {
	t.Run("token-fora-da-mensagem-de-erro", func(t *testing.T) {
		t.Run("error response", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":"AUT-1019","title":"Partner Not Allowed","message":"the partner this request was made on behalf of is not allowed to perform it"}`))
			}))
			t.Cleanup(srv.Close)

			c, err := New(WithConfig(createTestConfig(t)), WithLedgerURL(srv.URL), WithTracerURL(srv.URL+"/v1"),
				WithActorRelay(actorTestSource))
			require.NoError(t, err)

			_, err = c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
			require.Error(t, err)
			assert.NotContains(t, err.Error(), actorTestTokenA)
		})

		t.Run("transport failure", func(t *testing.T) {
			failing := WithHTTPTransport(func(http.RoundTripper) http.RoundTripper {
				return roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial tcp: connection refused")
				})
			})

			c, err := New(WithConfig(createTestConfig(t)), failing, WithRetryOptions(retry.WithMaxRetries(0)),
				WithActorRelay(actorTestSource))
			require.NoError(t, err)

			_, err = c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
			require.Error(t, err)
			assert.NotContains(t, err.Error(), actorTestTokenA)
		})

		t.Run("token invalid for a header", func(t *testing.T) {
			var reached atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				reached.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			c, err := New(WithConfig(createTestConfig(t)), WithLedgerURL(srv.URL), WithTracerURL(srv.URL+"/v1"),
				WithRetryOptions(retry.WithMaxRetries(0)), WithActorRelay(actorTestSource))
			require.NoError(t, err)

			const invalidToken = "tok\nA"

			_, err = c.V2.Organizations.List(actorTestContext(invalidToken), models.OrganizationsListOpts{})
			require.Error(t, err, "net/http refuses an invalid header value before sending")
			t.Logf("measured error: %q", err.Error())
			assert.NotContains(t, err.Error(), invalidToken)
			assert.NotContains(t, err.Error(), "tok", "no part of the token may appear in the error")
			assert.Zero(t, reached.Load(), "the request must not reach the server")
		})
	})

	t.Run("redirect-para-outra-origem-recusado", func(t *testing.T) {
		var otherHits atomic.Int32

		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			otherHits.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(other.Close)

		ledger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
		}))
		t.Cleanup(ledger.Close)

		c, err := New(WithConfig(createTestConfig(t)), WithLedgerURL(ledger.URL), WithTracerURL(ledger.URL+"/v1"),
			WithRetryOptions(retry.WithMaxRetries(0)), WithActorRelay(actorTestSource))
		require.NoError(t, err)

		_, err = c.V2.Organizations.List(actorTestContext(actorTestTokenA), models.OrganizationsListOpts{})
		require.Error(t, err, "the call must fail without following the redirect")
		assert.NotContains(t, err.Error(), actorTestTokenA)
		assert.Zero(t, otherHits.Load(), "the other origin must receive no request")
	})
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
