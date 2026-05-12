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
	// Table-driven consolidation of the redirect-policy cases. Each row
	// describes the previous request's shape (method, URL, headers,
	// optional body or replay factory), the next request's shape, the
	// optional opt-in for insecure HTTP, and the expected error
	// substring ("" means "must succeed"). The chain-length case is
	// captured via a dedicated builder because it needs to inflate the
	// via slice past maxRedirects.

	tests := []validateRedirectCase{
		{
			name:            "limits redirect chain length",
			previousMethod:  http.MethodGet,
			previousURL:     "https://api.example.com/v1/accounts",
			nextMethod:      http.MethodGet,
			nextURL:         "https://api.example.com/v1/accounts",
			viaInflateTo:    maxRedirects,
			wantErrContains: "stopped after 10 redirects",
		},
		{
			name:            "rejects cross-origin unsafe method without body",
			previousMethod:  http.MethodPost,
			previousURL:     "https://auth.example.com/v1/login/oauth/access_token",
			nextMethod:      http.MethodPost,
			nextURL:         "https://evil.example.net/v1/login/oauth/access_token",
			wantErrContains: "authenticated redirect",
		},
		{
			name:            "rejects cross-origin safe method with body",
			previousMethod:  http.MethodGet,
			previousURL:     "https://auth.example.com/v1/login/oauth/access_token",
			previousBody:    strings.NewReader(`{"clientSecret":"raw"}`),
			nextMethod:      http.MethodGet,
			nextURL:         "https://evil.example.net/v1/login/oauth/access_token",
			wantErrContains: "authenticated redirect",
		},
		{
			name:           "rejects cross-origin safe method with replay factory",
			previousMethod: http.MethodGet,
			previousURL:    "https://auth.example.com/v1/login/oauth/access_token",
			previousGetBody: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(`{"clientSecret":"raw"}`)), nil
			},
			nextMethod:      http.MethodGet,
			nextURL:         "https://evil.example.net/v1/login/oauth/access_token",
			wantErrContains: "authenticated redirect",
		},
		{
			// Organization IDs are Midaz resource identifiers, not tenant
			// identifiers. A safe-method cross-origin redirect carrying only
			// X-Organization-ID must not be treated as authenticated replay.
			name:           "allows cross-origin organization header on safe GET",
			previousMethod: http.MethodGet,
			previousURL:    "https://api.example.com/v1/accounts",
			previousHeaders: map[string]string{
				"X-Organization-ID": "org-7",
			},
			nextMethod: http.MethodGet,
			nextURL:    "https://other.example.net/v1/accounts",
		},
		{
			name:           "rejects cross-origin tenant header on safe GET",
			previousMethod: http.MethodGet,
			previousURL:    "https://api.example.com/v1/accounts",
			previousHeaders: map[string]string{
				"X-Tenant-ID": "tenant-1",
			},
			nextMethod:      http.MethodGet,
			nextURL:         "https://other.example.net/v1/accounts",
			wantErrContains: "authenticated redirect",
		},
		{
			// Defense in depth: tenant header alone is fine, but
			// Authorization in the same request still blocks the
			// cross-origin redirect.
			name:           "rejects cross-origin tenant header when paired with credential",
			previousMethod: http.MethodGet,
			previousURL:    "https://api.example.com/v1/accounts",
			previousHeaders: map[string]string{
				"X-Tenant-ID":   "tenant-1",
				"Authorization": "Bearer raw-token",
			},
			nextMethod:      http.MethodGet,
			nextURL:         "https://evil.example.net/v1/accounts",
			wantErrContains: "authenticated redirect",
		},
		{
			name:           "allows same-origin unsafe redirect",
			previousMethod: http.MethodPost,
			previousURL:    "https://auth.example.com/v1/login/oauth/access_token",
			previousBody:   strings.NewReader(`{"clientSecret":"raw"}`),
			nextMethod:     http.MethodPost,
			nextURL:        "https://auth.example.com/v1/login/oauth/access_token",
		},
		{
			name:              "allows opted-in insecure same-origin redirect",
			previousMethod:    http.MethodPost,
			previousURL:       "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000/v1/login/oauth/access_token",
			previousBody:      strings.NewReader(`{"clientSecret":"raw"}`),
			nextMethod:        http.MethodPost,
			nextURL:           "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000/v1/login/oauth/access_token",
			allowInsecureHTTP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runValidateRedirectCase(t, tt)
		})
	}
}

// validateRedirectCase is the shape used by [runValidateRedirectCase]
// and the test table in [TestValidateRedirect]. The struct lives at
// package scope so the test-driver helper can take it by value and
// keep cognitive complexity down inside the loop body.
type validateRedirectCase struct {
	name              string
	previousMethod    string
	previousURL       string
	previousHeaders   map[string]string
	previousBody      io.Reader
	previousGetBody   func() (io.ReadCloser, error)
	nextMethod        string
	nextURL           string
	viaInflateTo      int  // >0 → inflate via slice to that length to trip the cap
	allowInsecureHTTP bool // true → ValidateRedirectWithInsecureHTTP
	wantErrContains   string
}

// runValidateRedirectCase executes a single redirect-policy test case.
// Extracted from the loop body to keep
// [TestValidateRedirect]'s cognitive complexity under the project's
// revive threshold. Each branch here is straightforward setup; the
// genuine policy assertions are at the bottom.
func runValidateRedirectCase(t *testing.T, tt validateRedirectCase) {
	t.Helper()

	previous := newRedirectRequest(t, tt.previousMethod, tt.previousURL, tt.previousBody)
	for k, v := range tt.previousHeaders {
		previous.Header.Set(k, v)
	}

	if tt.previousGetBody != nil {
		previous.GetBody = tt.previousGetBody
	}

	next := newRedirectRequest(t, tt.nextMethod, tt.nextURL, nil)

	via := buildRedirectVia(t, tt, previous)

	var err error
	if tt.allowInsecureHTTP {
		err = ValidateRedirectWithInsecureHTTP(next, via, true)
	} else {
		err = ValidateRedirect(next, via)
	}

	if tt.wantErrContains == "" {
		require.NoError(t, err)
		return
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), tt.wantErrContains)
	if strings.Contains(tt.wantErrContains, "authenticated redirect") {
		require.ErrorIs(t, err, ErrAuthenticatedRedirect)
	}
}

// buildRedirectVia returns the [previous] request inside a slice, OR
// inflates the slice to viaInflateTo length when the case is
// exercising the redirect-chain cap.
func buildRedirectVia(t *testing.T, tt validateRedirectCase, previous *http.Request) []*http.Request {
	t.Helper()

	if tt.viaInflateTo <= 0 {
		return []*http.Request{previous}
	}

	via := make([]*http.Request, tt.viaInflateTo)
	for i := range via {
		via[i] = newRedirectRequest(t, tt.previousMethod, tt.previousURL, nil)
	}

	return via
}

func newRedirectRequest(t *testing.T, method, rawURL string, body io.Reader) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, rawURL, body)
	require.NoError(t, err)

	return req
}

// TestSameOrigin_FailClosedHostFormVariants pins the documented "strict,
// fail-closed" behaviour of sameOrigin: trailing-dot and
// explicit-default-port host variants compare unequal to the bare form.
// If we ever loosen this, the credential-replay threat model docs in
// sameOrigin's godoc must be updated in lockstep.
func TestSameOrigin_FailClosedHostFormVariants(t *testing.T) {
	t.Run("trailing-dot host counts as cross-origin", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodGet, "https://example.com/v1/accounts", nil)
		previous.Header.Set("Authorization", "Bearer raw-token")
		next := newRedirectRequest(t, http.MethodGet, "https://example.com./v1/accounts", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAuthenticatedRedirect)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})

	t.Run("explicit default HTTPS port counts as cross-origin", func(t *testing.T) {
		previous := newRedirectRequest(t, http.MethodGet, "https://example.com/v1/accounts", nil)
		previous.Header.Set("Authorization", "Bearer raw-token")
		next := newRedirectRequest(t, http.MethodGet, "https://example.com:443/v1/accounts", nil)

		err := ValidateRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAuthenticatedRedirect)
		assert.Contains(t, err.Error(), "authenticated redirect")
	})
}

// TestEnsureRedirectPolicy_SDKGuardRunsBeforeCallerCheckRedirect verifies
// that the SDK redirect guard is invoked unconditionally on every
// redirect — even when the caller installed a permissive CheckRedirect
// that would otherwise allow the cross-origin replay.
func TestEnsureRedirectPolicy_SDKGuardRunsBeforeCallerCheckRedirect(t *testing.T) {
	t.Run("SDK guard rejects cross-origin even when caller would allow", func(t *testing.T) {
		var callerInvoked bool
		caller := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				callerInvoked = true
				return nil // would happily follow
			},
		}

		wrapped := EnsureRedirectPolicy(caller)

		previous := newRedirectRequest(t, http.MethodGet, "https://api.example.com/v1/accounts", nil)
		previous.Header.Set("Authorization", "Bearer raw-token")
		next := newRedirectRequest(t, http.MethodGet, "https://evil.example.net/v1/accounts", nil)

		err := wrapped.CheckRedirect(next, []*http.Request{previous})
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAuthenticatedRedirect)
		assert.Contains(t, err.Error(), "authenticated redirect")
		assert.False(t, callerInvoked, "caller CheckRedirect must NOT run when SDK guard rejects")
	})

	t.Run("caller CheckRedirect runs on same-origin redirects", func(t *testing.T) {
		var callerInvoked bool
		caller := &http.Client{
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				callerInvoked = true
				return nil
			},
		}

		wrapped := EnsureRedirectPolicy(caller)

		previous := newRedirectRequest(t, http.MethodGet, "https://api.example.com/v1/accounts", nil)
		next := newRedirectRequest(t, http.MethodGet, "https://api.example.com/v2/accounts", nil)

		require.NoError(t, wrapped.CheckRedirect(next, []*http.Request{previous}))
		assert.True(t, callerInvoked, "same-origin redirect must reach caller CheckRedirect")
	})
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
