package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Subtest names carrying a spec scenario name trace back to
// specs/ledger-actor-relay/spec.md.

// actorRelayFakeToken is an obviously fake partner token.
const actorRelayFakeToken = "fake-actor-token"

// actorRelayCtxKey keys the fake partner token in a test context.
type actorRelayCtxKey struct{}

// actorRelaySource is the test token source: it reads the token stored under
// actorRelayCtxKey, the way a consumer's context reader would.
func actorRelaySource(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(actorRelayCtxKey{}).(string)

	return token, ok
}

func withActorToken(token string) context.Context {
	return context.WithValue(context.Background(), actorRelayCtxKey{}, token)
}

// actorRelayRequest is what the test server received.
type actorRelayRequest struct {
	method string
	path   string
	header http.Header
	body   string
}

// actorRelayServer records every request. When failFirst is set, the first
// request answers 503 so the plane retry round tripper replays it.
func actorRelayServer(t *testing.T, failFirst bool) (*httptest.Server, func() []actorRelayRequest) {
	t.Helper()

	var (
		mu   sync.Mutex
		seen []actorRelayRequest
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		seen = append(seen, actorRelayRequest{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: string(body)})
		failNow := failFirst && len(seen) == 1
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		if failNow {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"title":"unavailable"}`))

			return
		}

		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	t.Cleanup(srv.Close)

	return srv, func() []actorRelayRequest {
		mu.Lock()
		defer mu.Unlock()

		return append([]actorRelayRequest(nil), seen...)
	}
}

func actorRelayPlanes(t *testing.T, srv *httptest.Server, source func(context.Context) (string, bool), retryOptions retry.Options) *PlaneClients {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:       srv.Client(),
		retryOptions:     retryOptions,
		actorTokenSource: source,
	})
	require.NoError(t, err)

	return planes
}

// ledgerListAndCreate makes one Ledger read and one Ledger write.
func ledgerListAndCreate(t *testing.T, ctx context.Context, planes *PlaneClients) {
	t.Helper()

	limit := "10"
	_, err := planes.Ledger.ListOrganizationsWithResponse(ctx, &genledger.ListOrganizationsParams{Limit: &limit})
	require.NoError(t, err)

	_, err = planes.Ledger.CreateOrganizationWithBodyWithResponse(ctx, "application/json",
		strings.NewReader(`{"legalName":"Acme","legalDocument":"123"}`))
	require.NoError(t, err)
}

func TestPlaneClients_ActorRelay(t *testing.T) {
	t.Run("tracer-nao-recebe", func(t *testing.T) {
		srv, seen := actorRelayServer(t, false)
		planes := actorRelayPlanes(t, srv, actorRelaySource, planeTestRetryOptions())
		ctx := withActorToken(actorRelayFakeToken)

		limit := "10"
		_, err := planes.Ledger.ListOrganizationsWithResponse(ctx, &genledger.ListOrganizationsParams{Limit: &limit})
		require.NoError(t, err)
		_, err = planes.Tracer.ListRulesWithResponse(ctx, nil)
		require.NoError(t, err)

		got := seen()
		require.Len(t, got, 2)
		assert.Equal(t, "/v1/organizations", got[0].path)
		assert.Equal(t, []string{actorRelayFakeToken}, got[0].header.Values(actorHeader), "the Ledger request must carry the actor")
		assert.Equal(t, "/v1/rules", got[1].path)
		assert.Empty(t, got[1].header.Values(actorHeader), "the Tracer request must not carry the actor")
	})

	t.Run("contexto-sem-token-nao-envia", func(t *testing.T) {
		withSrv, withSeen := actorRelayServer(t, false)
		ledgerListAndCreate(t, context.Background(), actorRelayPlanes(t, withSrv, actorRelaySource, planeTestRetryOptions()))

		withoutSrv, withoutSeen := actorRelayServer(t, false)
		ledgerListAndCreate(t, context.Background(), actorRelayPlanes(t, withoutSrv, nil, planeTestRetryOptions()))

		withOption, withoutOption := withSeen(), withoutSeen()
		require.Len(t, withOption, 2)
		require.Len(t, withoutOption, 2)

		for i := range withOption {
			assert.Empty(t, withOption[i].header.Values(actorHeader))
			assert.Equal(t, withoutOption[i].method, withOption[i].method)
			assert.Equal(t, withoutOption[i].path, withOption[i].path)
			assert.Equal(t, withoutOption[i].header, withOption[i].header, "headers must match a client without the option")
			assert.Equal(t, withoutOption[i].body, withOption[i].body, "body must match a client without the option")
		}
	})

	t.Run("token-em-branco-nao-envia", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			token string
		}{
			{name: "empty", token: ""},
			{name: "spaces", token: "   "},
			{name: "tab and newline", token: "\t\n"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				srv, seen := actorRelayServer(t, false)
				ledgerListAndCreate(t, withActorToken(tc.token), actorRelayPlanes(t, srv, actorRelaySource, planeTestRetryOptions()))

				for _, req := range seen() {
					_, present := req.header[actorHeader]
					assert.False(t, present, "%s %s must not carry the actor", req.method, req.path)
				}
			})
		}
	})

	t.Run("token with surrounding spaces is sent trimmed", func(t *testing.T) {
		srv, seen := actorRelayServer(t, false)
		ledgerListAndCreate(t, withActorToken("  "+actorRelayFakeToken+" "), actorRelayPlanes(t, srv, actorRelaySource, planeTestRetryOptions()))

		for _, req := range seen() {
			assert.Equal(t, []string{actorRelayFakeToken}, req.header.Values(actorHeader))
		}
	})

	t.Run("relay-mantido-em-retentativa", func(t *testing.T) {
		srv, seen := actorRelayServer(t, true)
		planes := actorRelayPlanes(t, srv, actorRelaySource, retry.Options{
			MaxRetries:         1,
			InitialDelay:       time.Millisecond,
			MaxDelay:           time.Millisecond,
			BackoffFactor:      2.0,
			RetryableHTTPCodes: []int{http.StatusServiceUnavailable},
		})

		limit := "10"
		resp, err := planes.Ledger.ListOrganizationsWithResponse(withActorToken(actorRelayFakeToken), &genledger.ListOrganizationsParams{Limit: &limit})
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode())

		got := seen()
		require.Len(t, got, 2, "the 503 must be retried once")

		for i, req := range got {
			assert.Equal(t, []string{actorRelayFakeToken}, req.header.Values(actorHeader), "attempt %d must carry the actor", i+1)
		}
	})

	t.Run("sem-opcao-requisicao-inalterada", func(t *testing.T) {
		srv, seen := actorRelayServer(t, false)
		planes := actorRelayPlanes(t, srv, nil, planeTestRetryOptions())

		ledgerClient, ok := planes.Ledger.ClientInterface.(*genledger.Client)
		require.True(t, ok, "ledger client type %T", planes.Ledger.ClientInterface)
		assert.Len(t, ledgerClient.RequestEditors, 1, "without the option only the trace editor is installed")

		ctx := withActorToken(actorRelayFakeToken)
		ledgerListAndCreate(t, ctx, planes)
		_, err := planes.Tracer.ListRulesWithResponse(ctx, nil)
		require.NoError(t, err)

		got := seen()
		require.Len(t, got, 3)

		for _, req := range got {
			_, present := req.header[actorHeader]
			assert.False(t, present, "%s %s must not carry the actor without the option", req.method, req.path)
		}

		// The header set a Ledger read and write carried before this change.
		assert.ElementsMatch(t, []string{"Accept-Encoding", "Authorization", "User-Agent"}, headerNames(got[0].header))
		assert.ElementsMatch(t, []string{"Accept-Encoding", "Authorization", "Content-Length", "Content-Type", "User-Agent"}, headerNames(got[1].header))
		assert.Equal(t, `{"legalName":"Acme","legalDocument":"123"}`, got[1].body)
	})

	t.Run("option installs one more editor on the ledger client only", func(t *testing.T) {
		srv, _ := actorRelayServer(t, false)
		planes := actorRelayPlanes(t, srv, actorRelaySource, planeTestRetryOptions())

		ledgerClient, ok := planes.Ledger.ClientInterface.(*genledger.Client)
		require.True(t, ok)
		assert.Len(t, ledgerClient.RequestEditors, 2, "trace editor plus the actor relay")
	})
}

func headerNames(h http.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}

	return names
}
