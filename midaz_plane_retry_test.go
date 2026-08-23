package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/stretchr/testify/require"
)

// TestPlaneClientHonorsThreadedMaxRetries proves the ordering-hazard fix: a
// user-supplied WithRetryOptions(WithMaxRetries(5)) must reach the PLANE retry
// round tripper, not just the legacy *HTTPClient. Because plane clients are
// built inside NewEntityWithConfigContext (before midaz.go applies retry
// options to the legacy client), the effective policy must be threaded through
// construction. A persistent 503 on a plane GET (Rules.List → Tracer plane)
// must therefore be attempted 1 + 5 = 6 times.
func TestPlaneClientHonorsThreadedMaxRetries(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"unavailable","status":503}`))
	}))
	defer srv.Close()

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithLedgerURL(srv.URL+"/v1"),
		WithTracerURL(srv.URL+"/v1"),
		WithRetryOptions(
			retry.WithMaxRetries(5),
			retry.WithInitialDelay(time.Millisecond),
			retry.WithMaxDelay(2*time.Millisecond),
		),
	)
	require.NoError(t, err)

	// Rules.List is a GET (idempotent) on the Tracer plane; it retries freely.
	_, _ = c.Rules.List(context.Background(), models.RulesListOpts{})

	if got := atomic.LoadInt32(&count); got != 6 {
		t.Fatalf("plane attempts = %d, want 6 (WithMaxRetries(5) must reach the plane retry RT)", got)
	}
}

// TestPlaneClientHonorsThreadedCustomPolicy proves the custom retry policy is
// threaded onto the plane retry RT. A 418 is outside the default retryable set,
// so it is retried ONLY if the caller's WithCustomRetryPolicy reached the plane
// RT. Without threading the plane RT's customPolicy is nil and the 418 is a
// single attempt.
func TestPlaneClientHonorsThreadedCustomPolicy(t *testing.T) {
	var count int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"teapot","status":418}`))
	}))
	defer srv.Close()

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithLedgerURL(srv.URL+"/v1"),
		WithTracerURL(srv.URL+"/v1"),
		WithRetryOptions(
			retry.WithMaxRetries(2),
			retry.WithInitialDelay(time.Millisecond),
			retry.WithMaxDelay(2*time.Millisecond),
		),
		WithCustomRetryPolicy(func(resp *http.Response, _ error) bool {
			return resp != nil && resp.StatusCode == http.StatusTeapot
		}),
	)
	require.NoError(t, err)

	_, _ = c.Rules.List(context.Background(), models.RulesListOpts{})

	if got := atomic.LoadInt32(&count); got != 3 {
		t.Fatalf("plane attempts = %d, want 3 (custom policy must reach the plane retry RT: 1 + 2 retries on 418)", got)
	}
}
