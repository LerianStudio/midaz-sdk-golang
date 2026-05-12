package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestGetTokenFromPluginAuth(t *testing.T) {
	tests := []struct {
		name           string
		pluginAuth     AccessManager
		mockResponse   *TokenResponse
		mockStatusCode int
		expectError    bool
		expectedToken  string
	}{
		{
			name: "Success",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse: &TokenResponse{
				AccessToken:  "test-access-token",
				TokenType:    "Bearer",
				RefreshToken: "test-refresh-token",
				ExpiresAt:    "2025-05-17T00:00:00Z",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedToken:  "test-access-token",
		},
		{
			name: "PluginAuthDisabled",
			pluginAuth: AccessManager{
				Enabled: false,
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			expectError:    true,
			expectedToken:  "",
		},
		{
			name: "MissingAddress",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			expectError:    true,
			expectedToken:  "",
		},
		{
			name: "EmptyAccessToken",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse: &TokenResponse{
				AccessToken:  "",
				TokenType:    "Bearer",
				RefreshToken: "test-refresh-token",
				ExpiresAt:    "2025-05-17T00:00:00Z",
			},
			mockStatusCode: http.StatusOK,
			expectError:    true,
			expectedToken:  "",
		},
		{
			name: "InvalidResponse",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse:   nil,
			mockStatusCode: http.StatusOK, // Status OK but invalid JSON response
			expectError:    true,
			expectedToken:  "",
		},
		{
			name: "ServerError",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse:   nil,
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
			expectedToken:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server to simulate the auth service
			var server *httptest.Server

			if tt.pluginAuth.Enabled && tt.pluginAuth.Address != "" {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify request method and path
					assert.Equal(t, http.MethodPost, r.Method)
					assert.Equal(t, "/v1/login/oauth/access_token", r.URL.Path)

					// Verify headers
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					// Read and verify the request body
					var payload map[string]string

					err := json.NewDecoder(r.Body).Decode(&payload)
					assert.NoError(t, err)

					assert.Equal(t, "client_credentials", payload["grantType"])
					assert.Equal(t, tt.pluginAuth.ClientID, payload["clientId"])
					assert.Equal(t, tt.pluginAuth.ClientSecret, payload["clientSecret"])

					// Set response status code
					w.WriteHeader(tt.mockStatusCode)

					// If we have a mock response, return it
					if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
						_ = json.NewEncoder(w).Encode(tt.mockResponse)
					} else if tt.mockStatusCode == http.StatusInternalServerError {
						_, _ = w.Write([]byte(`{"code":"SRV-5000","message":"Internal server error","title":"Server Error"}`))
					} else if tt.mockStatusCode == http.StatusOK && tt.mockResponse == nil {
						// Invalid JSON response
						_, _ = w.Write([]byte(`{invalid-json`))
					}
				}))
				defer server.Close()

				// Override the address to use the test server
				tt.pluginAuth.Address = server.URL
			}

			// Call the function under test
			token, err := GetTokenFromAccessManager(context.Background(), tt.pluginAuth, &http.Client{})

			// Check the results
			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestGetTokenFromAccessManager_ErrorDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"code":"SRV-5000","message":"Internal server error","clientSecret":"leaked"}`,
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       `{invalid-json`,
		},
		{
			name:       "empty token",
			statusCode: http.StatusOK,
			body:       `{"accessToken":"","refreshToken":"raw-refresh"}`,
		},
		{
			name:       "oversized body",
			statusCode: http.StatusOK,
			body:       strings.Repeat("a", int(maxAccessManagerResponseBodyBytes)+1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, err := w.Write([]byte(tt.body))
				assert.NoError(t, err)
			}))
			defer srv.Close()

			mgr := AccessManager{
				Enabled:      true,
				Address:      srv.URL,
				ClientID:     "diag-client",
				ClientSecret: "diag-secret",
			}
			InvalidateAccessManagerToken(mgr)

			token, err := GetTokenFromAccessManager(context.Background(), mgr, srv.Client())

			require.Error(t, err)
			assert.Empty(t, token)

			var tokenErr *AccessManagerTokenRequestError
			require.ErrorAs(t, err, &tokenErr)
			assert.Equal(t, accessManagerTokenRequestOperation, tokenErr.Operation)
			assert.Equal(t, accessManagerTokenFetchPhase, tokenErr.AccessManagerPhase())
			assert.Equal(t, "http", tokenErr.AccessManagerEndpointScheme())
			assert.Equal(t, strings.TrimPrefix(srv.URL, "http://"), tokenErr.AccessManagerEndpointHost())
			assert.Equal(t, accessManagerOAuthLoginPath, tokenErr.AccessManagerEndpointPath())
			assert.True(t, tokenErr.AccessManagerHTTPRequestSent())
			assert.False(t, tokenErr.AccessManagerLocalValidationFailed())
			assert.Empty(t, tokenErr.AccessManagerValidationReason())
			assert.Equal(t, tt.statusCode, tokenErr.StatusCode())
			require.Error(t, errors.Unwrap(tokenErr))

			rendered := err.Error()
			assert.NotContains(t, rendered, "diag-secret")
			assert.NotContains(t, rendered, "raw-refresh")
			assert.NotContains(t, rendered, "leaked")
			assert.Contains(t, rendered, "httpRequestSent=true")
			assert.Contains(t, rendered, "localValidationFailed=false")
		})
	}
}

type typedNilAccessManagerCause struct{}

func (*typedNilAccessManagerCause) Error() string { return "typed nil should not render" }

func TestAccessManagerTokenRequestErrorErrorRedactsAndHandlesTypedNilCause(t *testing.T) {
	t.Run("redacts sensitive inner error values", func(t *testing.T) {
		err := &AccessManagerTokenRequestError{
			Operation:       accessManagerTokenRequestOperation,
			Phase:           accessManagerTokenFetchPhase,
			EndpointScheme:  "https",
			EndpointHost:    "auth.example.com",
			EndpointPath:    accessManagerOAuthLoginPath,
			HTTPRequestSent: true,
			StatusCodeValue: http.StatusInternalServerError,
			Err:             errors.New("access_token=secret-token client_secret=super-secret password=hunter2"),
		}

		rendered := err.Error()
		assert.Contains(t, rendered, "statusCode=500")
		assert.NotContains(t, rendered, "secret-token")
		assert.NotContains(t, rendered, "super-secret")
		assert.NotContains(t, rendered, "hunter2")
		assert.Contains(t, rendered, "[REDACTED]")
	})

	t.Run("typed nil inner error does not panic or render", func(t *testing.T) {
		var typedNil *typedNilAccessManagerCause
		err := &AccessManagerTokenRequestError{
			Operation: accessManagerTokenRequestOperation,
			Phase:     accessManagerTokenFetchPhase,
			Err:       typedNil,
		}

		require.NotPanics(t, func() { _ = err.Error() })
		assert.NotContains(t, err.Error(), "typed nil should not render")
	})

	t.Run("Unwrap exposes the inner cause for errors.Is and errors.As", func(t *testing.T) {
		sentinel := errors.New("sentinel cause")
		err := &AccessManagerTokenRequestError{
			Operation: accessManagerTokenRequestOperation,
			Phase:     accessManagerTokenFetchPhase,
			Err:       sentinel,
		}

		assert.Same(t, sentinel, errors.Unwrap(err),
			"Unwrap must return the exact inner sentinel pointer")
		assert.ErrorIs(t, err, sentinel,
			"errors.Is must walk through Unwrap to match the sentinel")
	})

	t.Run("Unwrap on nil receiver is safe and returns nil", func(t *testing.T) {
		var err *AccessManagerTokenRequestError
		assert.NoError(t, err.Unwrap())
	})
}
