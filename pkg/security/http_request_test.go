package security

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOutboundRequest(t *testing.T) {
	tests := []struct {
		name       string
		req        *http.Request
		errContain string
	}{
		{
			name:       "NilRequest",
			req:        nil,
			errContain: "http request cannot be nil",
		},
		{
			name:       "NilURL",
			req:        &http.Request{},
			errContain: "http request URL cannot be nil",
		},
		{
			name: "MissingHost",
			req: &http.Request{
				URL: &url.URL{Scheme: "https"},
			},
			errContain: "http request URL must include host",
		},
		{
			name: "UnsupportedScheme",
			req: &http.Request{
				URL: &url.URL{Scheme: "ftp", Host: "example.com"},
			},
			errContain: "unsupported URL scheme",
		},
		{
			name: "ValidHTTPSMixedCase",
			req: &http.Request{
				URL: &url.URL{Scheme: "HTTPS", Host: "8.8.8.8"},
			},
			errContain: "",
		},
		{
			name: "ValidHTTPS",
			req: &http.Request{
				URL: &url.URL{Scheme: "https", Host: "api.example.com"},
			},
			errContain: "",
		},
		{
			name: "ValidHTTP",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "localhost:8080"},
			},
			errContain: "",
		},
		{
			name: "InsecureRemoteHTTP",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "api.example.com"},
			},
			errContain: "insecure HTTP is only allowed for localhost targets",
		},
		{
			// RFC 6761 §6.3: any *.localhost name is reserved for loopback.
			// Docker Compose network aliases (e.g. mock-midaz.localhost) rely
			// on this to expose dev-stack services to the SDK over plain HTTP.
			name: "ValidHTTPDotLocalhost",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "mock-midaz.localhost:3001"},
			},
			errContain: "",
		},
		{
			// Suffix match must reject hostnames where ".localhost" is a label
			// inside a longer registrable name (DNS-rebinding-style abuse).
			name: "InsecureLocalhostSuffixSpoof",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "localhost.attacker.com"},
			},
			errContain: "insecure HTTP is only allowed for localhost targets",
		},
		{
			name: "ValidHTTPIPv6LoopbackBracketed",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "[::1]:8080"},
			},
			errContain: "",
		},
		{
			name: "ValidHTTPIPv4LoopbackAlias",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "127.1:8080"},
			},
			errContain: "",
		},
		{
			name: "ValidHTTPTrailingDotLocalhost",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "localhost.:8080"},
			},
			errContain: "",
		},
		{
			name: "RejectHTTPMetadataAddress",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "169.254.169.254"},
			},
			errContain: "insecure HTTP is only allowed for localhost targets",
		},
		{
			name: "RejectHTTPPrivateAddress",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", Host: "10.0.0.1"},
			},
			errContain: "insecure HTTP is only allowed for localhost targets",
		},
		{
			name: "RejectHTTPUserinfoHostConfusion",
			req: &http.Request{
				URL: &url.URL{Scheme: "http", User: url.User("localhost"), Host: "api.example.com"},
			},
			errContain: "URL must not include user information",
		},
		{
			name: "AllowHTTPSPrivateAddressCompatibility",
			req: &http.Request{
				URL: &url.URL{Scheme: "https", Host: "10.0.0.1"},
			},
			errContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutboundRequest(tt.req)

			if tt.errContain == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContain)
		})
	}
}

func TestValidateRedirect(t *testing.T) {
	t.Run("limits redirect chain length", func(t *testing.T) {
		const contractMaxRedirects = 10

		req := newRedirectRequest(t, http.MethodGet, "https://api.example.com/v1/accounts", nil)
		via := make([]*http.Request, contractMaxRedirects)
		for i := range via {
			via[i] = newRedirectRequest(t, http.MethodGet, "https://api.example.com/v1/accounts", nil)
		}

		err := ValidateRedirect(req, via)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stopped after 10 redirects")
	})

	t.Run("rejects cross-origin unsafe method without body", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodPost, "https://auth.example.com/v1/login/oauth/access_token", nil)
		next := newRedirectRequest(t, http.MethodPost, "https://evil.example.net/v1/login/oauth/access_token", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})

	t.Run("rejects cross-origin safe method with body", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodGet, "https://auth.example.com/v1/login/oauth/access_token", strings.NewReader(`{"clientSecret":"raw"}`))
		next := newRedirectRequest(t, http.MethodGet, "https://evil.example.net/v1/login/oauth/access_token", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})

	t.Run("rejects cross-origin safe method with replay factory", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodGet, "https://auth.example.com/v1/login/oauth/access_token", nil)
		previous.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(`{"clientSecret":"raw"}`)), nil
		}
		next := newRedirectRequest(t, http.MethodGet, "https://evil.example.net/v1/login/oauth/access_token", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})

	t.Run("rejects cross-origin tenant header", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodGet, "https://api.example.com/v1/accounts", nil)
		previous.Header.Set("X-Tenant-ID", "tenant-1")
		next := newRedirectRequest(t, http.MethodGet, "https://evil.example.net/v1/accounts", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})

	t.Run("allows same-origin unsafe redirect", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodPost, "https://auth.example.com/v1/login/oauth/access_token", strings.NewReader(`{"clientSecret":"raw"}`))
		next := newRedirectRequest(t, http.MethodPost, "https://auth.example.com/v1/login/oauth/access_token", nil)

		require.NoError(t, ValidateRedirect(next, []*http.Request{previous}))
	})

	t.Run("allows opted-in insecure same-origin redirect", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodPost, "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000/v1/login/oauth/access_token", strings.NewReader(`{"clientSecret":"raw"}`))
		next := newRedirectRequest(t, http.MethodPost, "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000/v1/login/oauth/access_token", nil)

		require.NoError(t, ValidateRedirectWithInsecureHTTP(next, []*http.Request{previous}, true))
	})
}

func newRedirectRequest(t *testing.T, method, rawURL string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, rawURL, body)
	require.NoError(t, err)

	return req
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{name: "Localhost", hostname: "localhost", want: true},
		{name: "IPv4Loopback", hostname: "127.0.0.1", want: true},
		{name: "IPv4LoopbackAlias", hostname: "127.1", want: true},
		{name: "IPv4LoopbackAlternate", hostname: "127.0.1.1", want: true},
		{name: "IPv6Loopback", hostname: "::1", want: true},
		{name: "TrailingDotLocalhost", hostname: "localhost.", want: true},
		{name: "TrailingDotLocalhostSubdomain", hostname: "mock-midaz.localhost.", want: true},
		{name: "UpperCaseLocalhost", hostname: "LOCALHOST", want: true},
		{name: "DotLocalhostSubdomain", hostname: "mock-midaz.localhost", want: true},
		{name: "DotLocalhostMultiLabel", hostname: "foo.bar.localhost", want: true},
		{name: "LocalhostSuffixSpoof", hostname: "localhost.attacker.com", want: false},
		{name: "NotLocalhost", hostname: "notlocalhost", want: false},
		{name: "BareDevName", hostname: "mock-midaz", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isLocalhost(tt.hostname))
		})
	}
}
