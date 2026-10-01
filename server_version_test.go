package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

func newServerVersionClient(t *testing.T, ledgerURL string) *Client {
	t.Helper()

	c, err := New(WithConfig(createTestConfig(t)), WithLedgerURL(ledgerURL))
	require.NoError(t, err)

	return c
}

// TestServerVersion pins that a /version body reaches the parser over a GET on
// exactly /version, with a nil error even when the version is a placeholder.
func TestServerVersion(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ServerVersion
	}{
		{
			name: "v4.1 buildinfo v1",
			body: `{"schemaVersion":"v1","service":"midaz-ledger","version":"4.1.3","revision":"b6506518f"}`,
			want: ServerVersion{Raw: "4.1.3", Major: 4, Minor: 1, Patch: 3, Known: true, Source: serverversion.SourceBuildInfoV1},
		},
		{
			name: "placeholder version is no error",
			body: `{"version":"0.0.0","requestDate":"2026-09-30T12:00:00Z"}`,
			want: ServerVersion{Raw: "0.0.0", Source: serverversion.SourceLegacy},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := newServerVersionClient(t, srv.URL).ServerVersion(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, http.MethodGet, gotMethod)
			assert.Equal(t, "/version", gotPath, "the route is unversioned; never /v1/version")
		})
	}
}

// TestServerVersionFailures pins that every failure is Unavailable, hence
// legacy, and carries a classified error the caller can log.
func TestServerVersionFailures(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		timeout time.Duration
		check   func(t *testing.T, err error)
	}{
		{
			name: "404 when the server does not mount /version",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"version":"4.1.3"}`))
			},
			check: func(t *testing.T, err error) {
				t.Helper()

				got, ok := sdkerrors.ActualHTTPStatus(err)
				assert.True(t, ok, "want the upstream status preserved, got %v", err)
				assert.Equal(t, http.StatusNotFound, got)
			},
		},
		{
			name:    "2xx proxy page is not a /version response",
			handler: writeBody(`<html><body>Welcome to nginx!</body></html>`),
			check:   wantNotVersionBody,
		},
		{
			name:    "1 MiB body is cut at the read limit",
			handler: writeBody(`{"version":"4.1.0","pad":"` + strings.Repeat("x", 1<<20) + `"}`),
			check:   wantNotVersionBody,
		},
		{
			name: "deadline while the server hangs",
			handler: func(_ http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
			},
			timeout: 50 * time.Millisecond,
			check: func(t *testing.T, err error) {
				t.Helper()
				assert.True(t, sdkerrors.IsTimeoutError(err), "want timeout, got %v", err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			ctx := context.Background()
			if tt.timeout > 0 {
				var cancel context.CancelFunc

				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}

			got, err := newServerVersionClient(t, srv.URL).ServerVersion(ctx)

			require.Error(t, err)
			assert.Equal(t, ServerVersion{Source: serverversion.SourceUnavailable}, got)
			assert.Equal(t, FeeModeLegacy, ResolveFeeMode(got))
			tt.check(t, err)
		})
	}
}

func TestServerVersionNilClient(t *testing.T) {
	var c *Client

	got, err := c.ServerVersion(context.Background())

	assert.True(t, sdkerrors.IsConfigurationError(err), "want configuration error, got %v", err)
	assert.Equal(t, ServerVersion{Source: serverversion.SourceUnavailable}, got)
}

func writeBody(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}
}

func wantNotVersionBody(t *testing.T, err error) {
	t.Helper()

	assert.True(t, sdkerrors.IsResponseDecodeError(err), "want response decode error, got %v", err)
	require.ErrorContains(t, err, "(HTTP 200): body is not a /version response")
}
