package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEntity implements the entityWithAuth interface for testing
type mockEntity struct {
	httpClient *http.Client
	authToken  string
	services   bool
}

func (m *mockEntity) GetHTTPClient() *http.Client {
	return m.httpClient
}

func (m *mockEntity) SetAuthToken(token string) {
	m.authToken = token
}

func (m *mockEntity) InitServices() {
	m.services = true
}

//nolint:revive // cognitive-complexity: table-driven test pattern
func TestWithPluginAuth(t *testing.T) {
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
			expectError:    false,
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
			name: "AuthServiceError",
			pluginAuth: AccessManager{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "invalid-client-id",
				ClientSecret: "invalid-client-secret",
			},
			mockResponse:   nil,
			mockStatusCode: http.StatusUnauthorized,
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
					assert.Equal(t, "/login/oauth/access_token", r.URL.Path)

					// Verify headers
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					// Set response status code
					w.WriteHeader(tt.mockStatusCode)

					// If we have a mock response, return it
					if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
						_ = json.NewEncoder(w).Encode(tt.mockResponse)
					} else if tt.mockStatusCode == http.StatusUnauthorized {
						// Simulate an auth error
						_, _ = w.Write([]byte(`{"code":"AUT-1004","message":"The provided 'clientId' or 'clientSecret' is incorrect.","title":"Invalid Client"}`))
					}
				}))
				defer server.Close()

				// Override the address to use the test server
				tt.pluginAuth.Address = server.URL
			}

			// Create a mock entity
			mockEntity := &mockEntity{
				httpClient: &http.Client{},
			}

			// Call the function under test
			err := WithAccessManager(tt.pluginAuth)(mockEntity)

			// Check the results
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedToken, mockEntity.authToken)

				// If plugin auth is enabled and successful, services should be initialized
				if tt.pluginAuth.Enabled && tt.expectedToken != "" {
					assert.True(t, mockEntity.services)
				}
			}
		})
	}
}

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
					assert.Equal(t, "/login/oauth/access_token", r.URL.Path)

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
