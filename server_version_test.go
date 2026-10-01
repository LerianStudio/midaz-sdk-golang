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

// TestServerVersion pins that every 2xx body reaches the parser over a GET on
// exactly /version, with a nil error even when the version is unknown.
func TestServerVersion(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ServerVersion
		mode FeeMode
	}{
		{
			name: "v4.1 buildinfo v1",
			body: `{"schemaVersion":"v1","service":"midaz-ledger","version":"4.1.3","revision":"b6506518f"}`,
			want: ServerVersion{Raw: "4.1.3", Major: 4, Minor: 1, Patch: 3, Known: true, Source: serverversion.SourceBuildInfoV1},
			mode: FeeModeNative,
		},
		{
			name: "v4.0 legacy shape",
			body: `{"version":"v4.0.7","requestDate":"2026-09-30T12:00:00Z","commit":"abc","dirty":false}`,
			want: ServerVersion{Raw: "v4.0.7", Major: 4, Patch: 7, Known: true, Source: serverversion.SourceLegacy},
			mode: FeeModeNative,
		},
		{
			name: "v3 legacy shape",
			body: `{"version":"v3.8.0","requestDate":"2026-09-30T12:00:00Z"}`,
			want: ServerVersion{Raw: "v3.8.0", Major: 3, Minor: 8, Known: true, Source: serverversion.SourceLegacy},
			mode: FeeModeLegacy,
		},
		{
			name: "placeholder 0.0.0",
			body: `{"version":"0.0.0","requestDate":"2026-09-30T12:00:00Z"}`,
			want: ServerVersion{Raw: "0.0.0", Source: serverversion.SourceLegacy},
			mode: FeeModeLegacy,
		},
		{
			name: "dev placeholder",
			body: `{"schemaVersion":"v1","version":"dev"}`,
			want: ServerVersion{Raw: "dev", Source: serverversion.SourceBuildInfoV1},
			mode: FeeModeLegacy,
		},
		{
			name: "unknown schemaVersion",
			body: `{"schemaVersion":"v2","version":"5.0.0"}`,
			want: ServerVersion{Source: serverversion.SourceUnavailable},
			mode: FeeModeLegacy,
		},
		{
			name: "proxy html page",
			body: `<html><body>Welcome to nginx!</body></html>`,
			want: ServerVersion{Source: serverversion.SourceUnavailable},
			mode: FeeModeLegacy,
		},
		{
			name: "1 MiB body is cut at the read limit",
			body: `{"version":"4.1.0","pad":"` + strings.Repeat("x", 1<<20) + `"}`,
			want: ServerVersion{Source: serverversion.SourceUnavailable},
			mode: FeeModeLegacy,
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
			assert.Equal(t, tt.mode, ResolveFeeMode(got))
			assert.Equal(t, http.MethodGet, gotMethod)
			assert.Equal(t, "/version", gotPath, "the route is unversioned; never /v1/version")
		})
	}
}

// TestServerVersionFailures pins that every failure is Unavailable, hence
// legacy, and carries a classified error the caller can log.
func TestServerVersionFailures(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	tests := []struct {
		name      string
		ledgerURL func(t *testing.T) string
		ctx       func(t *testing.T) context.Context
		check     func(t *testing.T, err error)
	}{
		{
			name:      "404 when the server does not mount /version",
			ledgerURL: statusServer(http.StatusNotFound),
			check:     wantStatus(http.StatusNotFound),
		},
		{
			name:      "500",
			ledgerURL: statusServer(http.StatusInternalServerError),
			check:     wantStatus(http.StatusInternalServerError),
		},
		{
			name: "deadline while the server hangs",
			ledgerURL: func(t *testing.T) string {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
					<-r.Context().Done()
				}))
				t.Cleanup(srv.Close)

				return srv.URL
			},
			ctx: func(t *testing.T) context.Context {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				t.Cleanup(cancel)

				return ctx
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				assert.True(t, sdkerrors.IsTimeoutError(err), "want timeout, got %v", err)
			},
		},
		{
			name:      "cancelled context",
			ledgerURL: statusServer(http.StatusOK),
			ctx: func(t *testing.T) context.Context {
				t.Helper()

				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			},
			check: func(t *testing.T, err error) {
				t.Helper()
				assert.True(t, sdkerrors.IsCancellationError(err), "want cancellation, got %v", err)
			},
		},
		{
			name:      "connection refused",
			ledgerURL: func(*testing.T) string { return closed.URL },
			check: func(t *testing.T, err error) {
				t.Helper()
				assert.True(t, sdkerrors.IsNetworkError(err), "want network error, got %v", err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx(t)
			}

			got, err := newServerVersionClient(t, tt.ledgerURL(t)).ServerVersion(ctx)

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

func statusServer(status int) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"version":"4.1.3"}`))
		}))
		t.Cleanup(srv.Close)

		return srv.URL
	}
}

func wantStatus(status int) func(t *testing.T, err error) {
	return func(t *testing.T, err error) {
		t.Helper()

		got, ok := sdkerrors.ActualHTTPStatus(err)
		assert.True(t, ok, "want the upstream status preserved, got %v", err)
		assert.Equal(t, status, got)
	}
}
