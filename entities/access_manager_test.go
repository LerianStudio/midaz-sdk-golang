package entities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConfig implements the Config interface for testing
type mockPluginAuthConfig struct {
	httpClient    *http.Client
	baseURLs      map[string]string
	pluginAuth    auth.AccessManager
	observability observability.Provider
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

func (m *mockPluginAuthConfig) GetPluginAuth() auth.AccessManager {
	return m.pluginAuth
}

// entityPluginAuthTestCase holds test data for entity plugin auth tests.
type entityPluginAuthTestCase struct {
	name           string
	pluginAuth     auth.AccessManager
	mockResponse   *auth.TokenResponse
	mockStatusCode int
	expectError    bool
}

func TestEntityWithPluginAuth(t *testing.T) {
	tests := []entityPluginAuthTestCase{
		{
			name: "Success",
			pluginAuth: auth.AccessManager{
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
			name: "PluginAuthError",
			pluginAuth: auth.AccessManager{
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
			runEntityPluginAuthTest(t, tt)
		})
	}
}

// runEntityPluginAuthTest executes a single entity plugin auth test case.
func runEntityPluginAuthTest(t *testing.T, tt entityPluginAuthTestCase) {
	t.Helper()

	server := createPluginAuthMockServer(t, &tt)
	if server != nil {
		defer server.Close()

		tt.pluginAuth.Address = server.URL
	}

	mockConfig := createMockPluginAuthConfig(tt.pluginAuth)
	entity, err := NewEntityWithConfig(mockConfig)

	assertEntityPluginAuthResult(t, tt, entity, err)
}

// createPluginAuthMockServer creates a mock server for plugin auth testing.
func createPluginAuthMockServer(t *testing.T, tt *entityPluginAuthTestCase) *httptest.Server {
	t.Helper()

	if !tt.pluginAuth.Enabled || tt.pluginAuth.Address == "" {
		return nil
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/login/oauth/access_token", r.URL.Path)

		w.WriteHeader(tt.mockStatusCode)
		writePluginAuthMockResponse(w, tt)
	}))
}

// writePluginAuthMockResponse writes the appropriate response based on test case.
func writePluginAuthMockResponse(w http.ResponseWriter, tt *entityPluginAuthTestCase) {
	if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
		if err := json.NewEncoder(w).Encode(tt.mockResponse); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	if tt.mockStatusCode == http.StatusUnauthorized {
		_, _ = w.Write([]byte(`{"code":"AUT-1004","message":"The provided 'clientId' or 'clientSecret' is incorrect.","title":"Invalid Client"}`))
	}
}

// createMockPluginAuthConfig creates a mock config for testing.
func createMockPluginAuthConfig(pluginAuth auth.AccessManager) *mockPluginAuthConfig {
	return &mockPluginAuthConfig{
		httpClient: &http.Client{},
		baseURLs: map[string]string{
			"onboarding":  "http://localhost:3000/v1",
			"transaction": "http://localhost:3001/v1",
		},
		pluginAuth: pluginAuth,
	}
}

// assertEntityPluginAuthResult asserts the expected result of entity creation.
func assertEntityPluginAuthResult(t *testing.T, tt entityPluginAuthTestCase, entity *Entity, err error) {
	t.Helper()

	if tt.expectError {
		require.Error(t, err)
		assert.Nil(t, entity)

		return
	}

	require.NoError(t, err)
	assert.NotNil(t, entity)

	if tt.pluginAuth.Enabled && tt.mockStatusCode == http.StatusOK {
		assert.Equal(t, "test-access-token", entity.httpClient.authToken)
	}
}

// pluginAuthOptionTestCase holds test data for plugin auth option tests.
type pluginAuthOptionTestCase struct {
	name           string
	pluginAuth     auth.AccessManager
	mockResponse   *auth.TokenResponse
	mockStatusCode int
	expectError    bool
	expectedToken  string
}

func TestWithPluginAuthOption(t *testing.T) {
	tests := []pluginAuthOptionTestCase{
		{
			name: "SuccessfulPluginAuth",
			pluginAuth: auth.AccessManager{
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
			pluginAuth: auth.AccessManager{
				Enabled: false,
			},
			mockResponse:   nil,
			mockStatusCode: 0,
			expectError:    false,
			expectedToken:  "",
		},
		{
			name: "MissingAddress",
			pluginAuth: auth.AccessManager{
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
			runPluginAuthOptionTest(t, tt)
		})
	}
}

// runPluginAuthOptionTest executes a single plugin auth option test case.
func runPluginAuthOptionTest(t *testing.T, tt pluginAuthOptionTestCase) {
	t.Helper()

	server := createPluginAuthOptionMockServer(t, &tt)
	if server != nil {
		defer server.Close()

		tt.pluginAuth.Address = server.URL
	}

	entity := createTestEntity()
	err := WithPluginAuth(tt.pluginAuth)(entity)

	assertPluginAuthOptionResult(t, tt, entity, err)
}

// createPluginAuthOptionMockServer creates a mock server for plugin auth option testing.
func createPluginAuthOptionMockServer(t *testing.T, tt *pluginAuthOptionTestCase) *httptest.Server {
	t.Helper()

	if !tt.pluginAuth.Enabled || tt.pluginAuth.Address == "" {
		return nil
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/login/oauth/access_token", r.URL.Path)

		w.WriteHeader(tt.mockStatusCode)

		if tt.mockResponse != nil && tt.mockStatusCode == http.StatusOK {
			_ = json.NewEncoder(w).Encode(tt.mockResponse)
		}
	}))
}

// createTestEntity creates a basic entity for testing.
func createTestEntity() *Entity {
	entity := &Entity{
		httpClient: NewHTTPClient(&http.Client{}, "", nil),
		baseURLs: map[string]string{
			"onboarding":  "http://localhost:3000/v1",
			"transaction": "http://localhost:3001/v1",
		},
	}
	entity.initServices()

	return entity
}

// assertPluginAuthOptionResult asserts the expected result of plugin auth option.
func assertPluginAuthOptionResult(t *testing.T, tt pluginAuthOptionTestCase, entity *Entity, err error) {
	t.Helper()

	if tt.expectError {
		require.Error(t, err)
		return
	}

	require.NoError(t, err)

	if tt.pluginAuth.Enabled && tt.expectedToken != "" {
		assert.Equal(t, tt.expectedToken, entity.httpClient.authToken)
	}
}
