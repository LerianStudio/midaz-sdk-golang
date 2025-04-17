package entities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	auth "github.com/LerianStudio/midaz-sdk-golang/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/stretchr/testify/assert"
)

// mockConfig implements the Config interface for testing
type mockPluginAuthConfig struct {
	httpClient     *http.Client
	baseURLs       map[string]string
	pluginAuth     auth.PluginAuth
	observability  observability.Provider
}

func (m *mockPluginAuthConfig) GetHTTPClient() *http.Client {
	return m.httpClient
}

func (m *mockPluginAuthConfig) GetBaseURLs() map[string]string {
	return m.baseURLs
}

func (m *mockPluginAuthConfig) GetObservabilityProvider() observability.Provider {
	return m.observability
}

func (m *mockPluginAuthConfig) GetPluginAuth() auth.PluginAuth {
	return m.pluginAuth
}

func TestEntityWithPluginAuth(t *testing.T) {
	tests := []struct {
		name           string
		pluginAuth     auth.PluginAuth
		mockResponse   *auth.TokenResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "SuccessfulPluginAuth",
			pluginAuth: auth.PluginAuth{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse: &auth.TokenResponse{
				AccessToken:  "test-access-token",
				TokenType:    "Bearer",
				RefreshToken: "test-refresh-token",
				ExpiresAt:    "2025-05-17T00:00:00Z",
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "PluginAuthDisabled",
			pluginAuth: auth.PluginAuth{
				Enabled: false,
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			expectError:    false,
		},
		{
			name: "PluginAuthError",
			pluginAuth: auth.PluginAuth{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "invalid-client-id",
				ClientSecret: "invalid-client-secret",
			},
			mockResponse:   nil,
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
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
					
					// Set response status code
					w.WriteHeader(tt.mockStatusCode)
					
					// If we have a mock response, return it
					if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
						json.NewEncoder(w).Encode(tt.mockResponse)
					} else if tt.mockStatusCode == http.StatusUnauthorized {
						// Simulate an auth error
						w.Write([]byte(`{"code":"AUT-1004","message":"The provided 'clientId' or 'clientSecret' is incorrect.","title":"Invalid Client"}`))
					}
				}))
				defer server.Close()
				
				// Override the address to use the test server
				tt.pluginAuth.Address = server.URL
			}
			
			// Create a mock config
			mockConfig := &mockPluginAuthConfig{
				httpClient: &http.Client{},
				baseURLs: map[string]string{
					"onboarding":  "http://localhost:3000/v1",
					"transaction": "http://localhost:3001/v1",
				},
				pluginAuth: tt.pluginAuth,
			}
			
			// Create an entity with the mock config
			entity, err := NewEntityWithConfig(mockConfig)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, entity)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, entity)
				
				// If plugin auth is enabled and successful, the auth token should be set
				if tt.pluginAuth.Enabled && tt.mockStatusCode == http.StatusOK {
					assert.Equal(t, "test-access-token", entity.httpClient.authToken)
				}
			}
		})
	}
}

func TestWithPluginAuthOption(t *testing.T) {
	tests := []struct {
		name           string
		pluginAuth     auth.PluginAuth
		mockResponse   *auth.TokenResponse
		mockStatusCode int
		expectError    bool
		expectedToken  string
	}{
		{
			name: "SuccessfulPluginAuth",
			pluginAuth: auth.PluginAuth{
				Enabled:      true,
				Address:      "http://localhost:4000",
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			mockResponse: &auth.TokenResponse{
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
			pluginAuth: auth.PluginAuth{
				Enabled: false,
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			expectError:    false,
			expectedToken:  "",
		},
		{
			name: "MissingAddress",
			pluginAuth: auth.PluginAuth{
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
					
					// Set response status code
					w.WriteHeader(tt.mockStatusCode)
					
					// If we have a mock response, return it
					if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
						json.NewEncoder(w).Encode(tt.mockResponse)
					}
				}))
				defer server.Close()
				
				// Override the address to use the test server
				tt.pluginAuth.Address = server.URL
			}
			
			// Create a basic entity
			entity := &Entity{
				httpClient: NewHTTPClient(&http.Client{}, "", nil),
				baseURLs: map[string]string{
					"onboarding":  "http://localhost:3000/v1",
					"transaction": "http://localhost:3001/v1",
				},
			}
			
			// Initialize services to avoid nil pointers
			entity.initServices()
			
			// Call the function under test
			err := WithPluginAuth(tt.pluginAuth)(entity)
			
			// Check the results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// If plugin auth is enabled and successful, the auth token should be set
				if tt.pluginAuth.Enabled && tt.expectedToken != "" {
					assert.Equal(t, tt.expectedToken, entity.httpClient.authToken)
				}
			}
		})
	}
}
