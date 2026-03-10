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
