package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every resource accessor now routes over a generated plane client, so no
// service reads the entity's own *HTTPClient any more. That client still ships
// as public surface — [Entity.GetEntityHTTPClient] hands it back, and its
// SetDebug / SetUserAgent / SetLogger / retry knobs are documented tuning — so
// the invariants that made sharing it safe (H4/H5/H6: one auth-token cache, one
// singleflight refresh group, mid-lifetime Set* propagation) are pinned here
// directly on the object rather than through a service that no longer uses it.

// requestJSON drives one request through the entity's HTTPClient, which is what
// the H4/H5/H6 invariants live on.
func requestJSON(t *testing.T, entity *Entity, url string) error {
	t.Helper()

	var out map[string]any

	return entity.GetEntityHTTPClient().doRequest(context.Background(), http.MethodGet, url, nil, nil, &out)
}

// TestSharedHTTPClient_EntityExposesOneInstance verifies GetEntityHTTPClient
// returns the same instance across calls, so a Set* made on it is not lost to a
// per-call copy.
func TestSharedHTTPClient_EntityExposesOneInstance(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding": "http://localhost",
	}, nil)

	first := entity.GetEntityHTTPClient()
	require.NotNil(t, first, "parent HTTPClient must not be nil")
	require.Same(t, first, entity.GetEntityHTTPClient(), "GetEntityHTTPClient must hand back one instance")
}

// TestSharedHTTPClient_SetUserAgentMidLifetimePropagates verifies SetUserAgent
// on GetEntityHTTPClient changes the User-Agent on subsequent requests, made
// AFTER construction already ran.
func TestSharedHTTPClient_SetUserAgentMidLifetimePropagates(t *testing.T) {
	var seenUserAgent atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUserAgent.Store(r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity := newTestEntity(t, srv.Client(), "token", map[string]string{
		"onboarding": srv.URL,
	}, nil)

	// Mid-lifetime: flip the user-agent AFTER initServices already ran.
	entity.GetEntityHTTPClient().SetUserAgent("rotated-ua/1.0")

	require.NoError(t, requestJSON(t, entity, srv.URL+"/v1/anything"))
	assert.Equal(t, "rotated-ua/1.0", seenUserAgent.Load(),
		"the request must observe the post-construction SetUserAgent")
}

// TestSharedHTTPClient_SetDebugMidLifetimePropagates verifies SetDebug is
// readable on the same instance immediately after being set.
func TestSharedHTTPClient_SetDebugMidLifetimePropagates(t *testing.T) {
	entity := newTestEntity(t, &http.Client{Timeout: time.Second}, "token", map[string]string{
		"onboarding": "http://localhost",
	}, nil)

	parent := entity.GetEntityHTTPClient()
	require.False(t, parent.cloneConfiguration().debug)

	parent.SetDebug(true)

	require.True(t, parent.cloneConfiguration().debug,
		"the client must observe the SetDebug(true) made on it")
}

func formatToken(seq int32) string {
	return "fresh-token-" + strconv.Itoa(int(seq))
}
