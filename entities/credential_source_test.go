package entities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type credCtxKey struct{}

// ctxCredentialSource resolves the credential stored in the request context.
func ctxCredentialSource(ctx context.Context) (string, string, error) {
	id, ok := ctx.Value(credCtxKey{}).(string)
	if !ok {
		return "", "", errors.New("no credential in context")
	}

	return id, "secret-" + id, nil
}

// newTokenServer issues "tok-<clientID>-<n>", n counting that client's exchanges.
func newTokenServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()

	var (
		hits   int32
		mu     sync.Mutex
		counts = map[string]int{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)

		var body struct {
			ClientID string `json:"clientId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		counts[body.ClientID]++
		n := counts[body.ClientID]
		mu.Unlock()

		_ = json.NewEncoder(w).Encode(auth.TokenResponse{
			AccessToken: fmt.Sprintf("tok-%s-%d", body.ClientID, n),
			TokenType:   "Bearer",
			ExpiresAt:   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		})
	}))
	t.Cleanup(srv.Close)

	return srv, &hits
}

func TestCredentialSource_NoExchangeAtNew(t *testing.T) {
	am, hits := newTokenServer(t)

	cfg := createMockPluginAuthConfig(auth.AccessManager{
		Enabled: true, Address: am.URL, CredentialSource: ctxCredentialSource,
	})

	entity, err := NewEntityWithConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, entity)
	assert.Zero(t, atomic.LoadInt32(hits))
}

// A concurrent 401 burst from two credentials must replay each credential's own
// fresh token: the data server only accepts a token's second generation.
func TestCredentialSource_Concurrent401BurstKeepsCredentialsApart(t *testing.T) {
	am, _ := newTokenServer(t)

	var (
		mu    sync.Mutex
		wrong []string
	)

	data := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		who := r.Header.Get("X-Who")

		if !strings.HasPrefix(token, "tok-"+who+"-") {
			mu.Lock()
			wrong = append(wrong, who+" got "+token)
			mu.Unlock()
		}

		if !strings.HasSuffix(token, "-1") {
			w.WriteHeader(http.StatusOK)
			return
		}

		time.Sleep(20 * time.Millisecond) // keep the 401 burst overlapping
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer data.Close()

	cfg := createMockPluginAuthConfig(auth.AccessManager{
		Enabled: true, Address: am.URL, CredentialSource: ctxCredentialSource,
	})
	rt := newAuthRefreshRoundTripper(http.DefaultTransport, accessManagerAuthConfig(cfg, cfg.pluginAuth))

	var wg sync.WaitGroup

	for _, who := range []string{"a", "b"} {
		for range 10 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				ctx := context.WithValue(context.Background(), credCtxKey{}, who)
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, data.URL, nil)
				req.Header.Set("X-Who", who)

				resp, err := rt.RoundTrip(req)
				if assert.NoError(t, err) {
					resp.Body.Close()
				}
			}()
		}
	}

	wg.Wait()
	assert.Empty(t, wrong)
}

func TestCredentialSource_ErrorIsNotRetried(t *testing.T) {
	var calls int32

	source := func(context.Context) (string, string, error) {
		atomic.AddInt32(&calls, 1)
		return "", "", errors.New("no credential")
	}

	cfg := createMockPluginAuthConfig(auth.AccessManager{
		Enabled: true, Address: "http://127.0.0.1:1", CredentialSource: source,
	})
	authRT := newAuthRefreshRoundTripper(http.DefaultTransport, accessManagerAuthConfig(cfg, cfg.pluginAuth))
	opts := retry.DefaultOptions()
	opts.MaxRetries = 3
	opts.InitialDelay = time.Millisecond
	opts.MaxDelay = time.Millisecond

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:1", nil)
	resp, err := newRetryRoundTripper(authRT, *opts, nil).RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credential")
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))
}
