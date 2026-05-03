package security

import (
	"net/http"
	"net/url"
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
			errContain: "insecure HTTP is only allowed for localhost targets",
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
