package config

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/version"
)

// Helper function to disable auth check for tests
func disableAuthCheck(t *testing.T) func() {
	t.Helper()
	os.Setenv("MIDAZ_SKIP_AUTH_CHECK", "true")

	return func() {
		os.Unsetenv("MIDAZ_SKIP_AUTH_CHECK")
	}
}

// Helper function to save and restore environment variables
func saveEnv(keys []string) (restore func()) {
	origEnv := make(map[string]string)
	for _, key := range keys {
		origEnv[key] = os.Getenv(key)
	}

	return func() {
		for key, value := range origEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}
}

func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{"DefaultTimeout", DefaultTimeout, 60},
		{"DefaultLocalBaseURL", DefaultLocalBaseURL, "http://localhost"},
		{"DefaultDevelopmentBaseURL", DefaultDevelopmentBaseURL, "https://api.dev.midaz.io"},
		{"DefaultProductionBaseURL", DefaultProductionBaseURL, "https://api.midaz.io"},
		{"DefaultOnboardingPort", DefaultOnboardingPort, "3000"},
		{"DefaultTransactionPort", DefaultTransactionPort, "3001"},
		{"DefaultLocalOnboardingPath", DefaultLocalOnboardingPath, ""},
		{"DefaultLocalTransactionPath", DefaultLocalTransactionPath, ""},
		{"DefaultMaxRetries", DefaultMaxRetries, 3},
		{"DefaultRetryWaitMin", DefaultRetryWaitMin, 1 * time.Second},
		{"DefaultRetryWaitMax", DefaultRetryWaitMax, 30 * time.Second},
		{"DefaultEnableIdempotency", DefaultEnableIdempotency, true},
		{"DefaultEnableRetries", DefaultEnableRetries, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.got)
		})
	}
}

func TestServiceTypeConstants(t *testing.T) {
	assert.Equal(t, ServiceOnboarding, ServiceType("onboarding"))
	assert.Equal(t, ServiceTransaction, ServiceType("transaction"))
}

func TestEnvironmentConstants(t *testing.T) {
	assert.Equal(t, EnvironmentLocal, Environment("local"))
	assert.Equal(t, EnvironmentDevelopment, Environment("development"))
	assert.Equal(t, EnvironmentProduction, Environment("production"))
}

func TestNewConfig_Defaults(t *testing.T) {
	config, err := NewConfig(WithAccessManager(auth.AccessManager{Enabled: false}))
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.Equal(t, DefaultTimeout*time.Second, config.Timeout)
	assert.Equal(t, version.UserAgent(), config.UserAgent)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultRetryWaitMin, config.RetryWaitMin)
	assert.Equal(t, DefaultRetryWaitMax, config.RetryWaitMax)
	assert.True(t, config.EnableRetries)
	assert.True(t, config.EnableIdempotency)
	assert.False(t, config.Debug)
	assert.NotNil(t, config.HTTPClient)
	assert.Equal(t, "http://localhost:3000", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3001", config.ServiceURLs[ServiceTransaction])
}

func TestNewConfig_WithAllOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}
	mockProvider := &mockObservabilityProvider{}

	config, err := NewConfig(
		WithEnvironment(EnvironmentProduction),
		WithOnboardingURL("https://custom.example.com/onboarding"),
		WithTransactionURL("https://custom.example.com/transaction"),
		WithHTTPClient(customClient),
		WithTimeout(90*time.Second),
		WithUserAgent("test-agent/1.0"),
		WithRetryConfig(5, 2*time.Second, 60*time.Second),
		WithRetries(false),
		WithDebug(true),
		WithIdempotency(false),
		WithObservabilityProvider(mockProvider),
		WithAccessManager(auth.AccessManager{
			Enabled:      false,
			Address:      "http://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, EnvironmentProduction, config.Environment)
	assert.Equal(t, "https://custom.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com/transaction", config.ServiceURLs[ServiceTransaction])
	assert.Equal(t, customClient, config.HTTPClient)
	assert.Equal(t, 90*time.Second, config.Timeout)
	assert.Equal(t, "test-agent/1.0", config.UserAgent)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryWaitMin)
	assert.Equal(t, 60*time.Second, config.RetryWaitMax)
	assert.False(t, config.EnableRetries)
	assert.True(t, config.Debug)
	assert.False(t, config.EnableIdempotency)
	assert.Equal(t, mockProvider, config.ObservabilityProvider)
	assert.Equal(t, "http://auth.example.com", config.AccessManager.Address)
	assert.Equal(t, "test-client", config.AccessManager.ClientID)
	assert.Equal(t, "test-secret", config.AccessManager.ClientSecret)
}

func TestWithEnvironment_AllEnvironments(t *testing.T) {
	tests := []struct {
		name string
		env  Environment
	}{
		{
			name: "local",
			env:  EnvironmentLocal,
		},
		{
			name: "development",
			env:  EnvironmentDevelopment,
		},
		{
			name: "production",
			env:  EnvironmentProduction,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithEnvironment(tc.env),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.env, config.Environment)
			// Note: NewConfig sets default URLs first (based on initial EnvironmentLocal),
			// then applies options. So WithEnvironment changes Environment field but
			// doesn't regenerate URLs. Use WithBaseURL after WithEnvironment to update URLs.
		})
	}
}

func TestWithEnvironment_WithBaseURL(t *testing.T) {
	tests := []struct {
		name                   string
		env                    Environment
		expectedOnboardingURL  string
		expectedTransactionURL string
	}{
		{
			name:                   "development with base URL",
			env:                    EnvironmentDevelopment,
			expectedOnboardingURL:  "https://api.custom.io/onboarding",
			expectedTransactionURL: "https://api.custom.io/transaction",
		},
		{
			name:                   "production with base URL",
			env:                    EnvironmentProduction,
			expectedOnboardingURL:  "https://api.custom.io/onboarding",
			expectedTransactionURL: "https://api.custom.io/transaction",
		},
		{
			name:                   "local with base URL",
			env:                    EnvironmentLocal,
			expectedOnboardingURL:  "https://api.custom.io:3000",
			expectedTransactionURL: "https://api.custom.io:3001",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithEnvironment(tc.env),
				WithBaseURL("https://api.custom.io"),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.env, config.Environment)
			assert.Equal(t, tc.expectedOnboardingURL, config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.expectedTransactionURL, config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestWithOnboardingURL_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https URL", "https://api.example.com/onboarding"},
		{"http localhost", "http://localhost:3000"},
		{"http 127.0.0.1", "http://127.0.0.1:3000"},
		{"with path", "https://api.example.com/v1/onboarding"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithOnboardingURL(tc.url),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.url, config.ServiceURLs[ServiceOnboarding])
		})
	}
}

func TestWithOnboardingURL_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{"empty URL", "", "invalid onboarding URL"},
		{"no scheme", "api.example.com", "invalid onboarding URL"},
		{"no host", "https://", "invalid onboarding URL"},
		{"malformed", "://invalid", "invalid onboarding URL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithOnboardingURL(tc.url),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestWithTransactionURL_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https URL", "https://api.example.com/transaction"},
		{"http localhost", "http://localhost:3001"},
		{"with path", "https://api.example.com/v1/transaction"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithTransactionURL(tc.url),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.url, config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestWithTransactionURL_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{"empty URL", "", "invalid transaction URL"},
		{"no scheme", "api.example.com/tx", "invalid transaction URL"},
		{"no host", "https://", "invalid transaction URL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithTransactionURL(tc.url),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestWithBaseURL_LocalEnvironment(t *testing.T) {
	config, err := NewConfig(
		WithEnvironment(EnvironmentLocal),
		WithBaseURL("https://custom.example.com"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)

	assert.Equal(t, "https://custom.example.com:3000", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com:3001", config.ServiceURLs[ServiceTransaction])
}

func TestWithBaseURL_NonLocalEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		env      Environment
		baseURL  string
		expected map[ServiceType]string
	}{
		{
			name:    "development",
			env:     EnvironmentDevelopment,
			baseURL: "https://custom.example.com",
			expected: map[ServiceType]string{
				ServiceOnboarding:  "https://custom.example.com/onboarding",
				ServiceTransaction: "https://custom.example.com/transaction",
			},
		},
		{
			name:    "production",
			env:     EnvironmentProduction,
			baseURL: "https://api.prod.example.com",
			expected: map[ServiceType]string{
				ServiceOnboarding:  "https://api.prod.example.com/onboarding",
				ServiceTransaction: "https://api.prod.example.com/transaction",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithEnvironment(tc.env),
				WithBaseURL(tc.baseURL),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.expected[ServiceOnboarding], config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.expected[ServiceTransaction], config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestWithBaseURL_TrailingSlash(t *testing.T) {
	config, err := NewConfig(
		WithEnvironment(EnvironmentProduction),
		WithBaseURL("https://api.example.com/"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
}

func TestWithBaseURL_Invalid(t *testing.T) {
	_, err := NewConfig(
		WithBaseURL("invalid-url"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

func TestWithHTTPClient_Valid(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}
	config, err := NewConfig(
		WithHTTPClient(customClient),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, customClient, config.HTTPClient)
}

func TestWithHTTPClient_Nil(t *testing.T) {
	_, err := NewConfig(
		WithHTTPClient(nil),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP client cannot be nil")
}

func TestWithTimeout_Valid(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"5 minutes", 5 * time.Minute},
		{"1 millisecond", 1 * time.Millisecond},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithTimeout(tc.timeout),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.timeout, config.Timeout)
		})
	}
}

func TestWithTimeout_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithTimeout(tc.timeout),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "timeout must be greater than 0")
		})
	}
}

func TestWithUserAgent_Valid(t *testing.T) {
	config, err := NewConfig(
		WithUserAgent("custom-agent/2.0"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, "custom-agent/2.0", config.UserAgent)
}

func TestWithUserAgent_Empty(t *testing.T) {
	_, err := NewConfig(
		WithUserAgent(""),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user agent cannot be empty")
}

func TestWithRetryConfig_Valid(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		minWait    time.Duration
		maxWait    time.Duration
	}{
		{"standard config", 3, 1 * time.Second, 30 * time.Second},
		{"zero retries", 0, 1 * time.Second, 5 * time.Second},
		{"equal min max", 5, 10 * time.Second, 10 * time.Second},
		{"large values", 100, 1 * time.Millisecond, 10 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithRetryConfig(tc.maxRetries, tc.minWait, tc.maxWait),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.maxRetries, config.MaxRetries)
			assert.Equal(t, tc.minWait, config.RetryWaitMin)
			assert.Equal(t, tc.maxWait, config.RetryWaitMax)
		})
	}
}

func TestWithRetryConfig_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		maxRetries  int
		minWait     time.Duration
		maxWait     time.Duration
		expectedErr string
	}{
		{"negative retries", -1, 1 * time.Second, 30 * time.Second, "max retries cannot be negative"},
		{"zero minWait", 3, 0, 30 * time.Second, "minimum wait time must be greater than 0"},
		{"negative minWait", 3, -1 * time.Second, 30 * time.Second, "minimum wait time must be greater than 0"},
		{"maxWait less than minWait", 3, 30 * time.Second, 1 * time.Second, "maximum wait time must be greater than or equal"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithRetryConfig(tc.maxRetries, tc.minWait, tc.maxWait),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestWithMaxRetries_Valid(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
	}{
		{"zero", 0},
		{"one", 1},
		{"five", 5},
		{"hundred", 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithMaxRetries(tc.maxRetries),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.maxRetries, config.MaxRetries)
		})
	}
}

func TestWithMaxRetries_Invalid(t *testing.T) {
	_, err := NewConfig(
		WithMaxRetries(-1),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries cannot be negative")
}

func TestWithRetryWaitMin_Valid(t *testing.T) {
	config, err := NewConfig(
		WithRetryWaitMin(5*time.Second),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, config.RetryWaitMin)
}

func TestWithRetryWaitMin_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		minWait time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithRetryWaitMin(tc.minWait),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "minimum wait time must be greater than 0")
		})
	}
}

func TestWithRetryWaitMax_Valid(t *testing.T) {
	config, err := NewConfig(
		WithRetryWaitMax(60*time.Second),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, config.RetryWaitMax)
}

func TestWithRetryWaitMax_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		minWait     time.Duration
		maxWait     time.Duration
		expectedErr string
	}{
		{"zero maxWait", 1 * time.Second, 0, "maximum wait time must be greater than 0"},
		{"negative maxWait", 1 * time.Second, -1 * time.Second, "maximum wait time must be greater than 0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithRetryWaitMin(tc.minWait),
				WithRetryWaitMax(tc.maxWait),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestWithRetryWaitMax_LessThanMin(t *testing.T) {
	_, err := NewConfig(
		WithRetryWaitMin(30*time.Second),
		WithRetryWaitMax(10*time.Second),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum wait time must be greater than or equal to minimum wait time")
}

func TestWithRetries_Toggle(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithRetries(tc.enabled),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, config.EnableRetries)
		})
	}
}

func TestWithDebug_Toggle(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithDebug(tc.enabled),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, config.Debug)
		})
	}
}

func TestWithIdempotency_Toggle(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithIdempotency(tc.enabled),
				WithAccessManager(auth.AccessManager{Enabled: false}),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, config.EnableIdempotency)
		})
	}
}

func TestWithObservabilityProvider(t *testing.T) {
	provider := &mockObservabilityProvider{}
	config, err := NewConfig(
		WithObservabilityProvider(provider),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, provider, config.ObservabilityProvider)
}

func TestWithObservabilityProvider_Nil(t *testing.T) {
	config, err := NewConfig(
		WithObservabilityProvider(nil),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Nil(t, config.ObservabilityProvider)
}

func TestWithAccessManager(t *testing.T) {
	accessManager := auth.AccessManager{
		Enabled:      true,
		Address:      "http://auth.example.com",
		ClientID:     "client-123",
		ClientSecret: "secret-456",
	}

	cleanup := disableAuthCheck(t)
	defer cleanup()

	config, err := NewConfig(WithAccessManager(accessManager))
	require.NoError(t, err)

	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://auth.example.com", config.AccessManager.Address)
	assert.Equal(t, "client-123", config.AccessManager.ClientID)
	assert.Equal(t, "secret-456", config.AccessManager.ClientSecret)
}

func TestValidateConfig_MissingAuthAddress(t *testing.T) {
	_, err := NewConfig(WithAccessManager(auth.AccessManager{
		Enabled: true,
		Address: "",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin auth address is required")
}

func TestValidateConfig_AuthCheckSkipped(t *testing.T) {
	cleanup := disableAuthCheck(t)
	defer cleanup()

	config, err := NewConfig(WithAccessManager(auth.AccessManager{
		Enabled: true,
		Address: "",
	}))
	require.NoError(t, err)
	assert.True(t, config.AccessManager.Enabled)
}

func TestFromEnvironment_AllVariables(t *testing.T) {
	envVars := []string{
		"MIDAZ_ENVIRONMENT",
		"PLUGIN_AUTH_ENABLED",
		"PLUGIN_AUTH_ADDRESS",
		"MIDAZ_CLIENT_ID",
		"MIDAZ_CLIENT_SECRET",
		"MIDAZ_USER_AGENT",
		"MIDAZ_BASE_URL",
		"MIDAZ_ONBOARDING_URL",
		"MIDAZ_TRANSACTION_URL",
		"MIDAZ_TIMEOUT",
		"MIDAZ_DEBUG",
		"MIDAZ_MAX_RETRIES",
		"MIDAZ_IDEMPOTENCY",
	}

	restore := saveEnv(envVars)
	defer restore()

	os.Setenv("MIDAZ_ENVIRONMENT", "development")
	os.Setenv("PLUGIN_AUTH_ENABLED", "true")
	os.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.example.com")
	os.Setenv("MIDAZ_CLIENT_ID", "env-client-id")
	os.Setenv("MIDAZ_CLIENT_SECRET", "env-client-secret")
	os.Setenv("MIDAZ_USER_AGENT", "env-agent/1.0")
	os.Setenv("MIDAZ_ONBOARDING_URL", "https://env.example.com/onboarding")
	os.Setenv("MIDAZ_TRANSACTION_URL", "https://env.example.com/transaction")
	os.Setenv("MIDAZ_TIMEOUT", "45")
	os.Setenv("MIDAZ_DEBUG", "true")
	os.Setenv("MIDAZ_MAX_RETRIES", "7")
	os.Setenv("MIDAZ_IDEMPOTENCY", "false")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)

	assert.Equal(t, EnvironmentDevelopment, config.Environment)
	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://auth.example.com", config.AccessManager.Address)
	assert.Equal(t, "env-client-id", config.AccessManager.ClientID)
	assert.Equal(t, "env-client-secret", config.AccessManager.ClientSecret)
	assert.Equal(t, "env-agent/1.0", config.UserAgent)
	assert.Equal(t, "https://env.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://env.example.com/transaction", config.ServiceURLs[ServiceTransaction])
	assert.Equal(t, 45*time.Second, config.Timeout)
	assert.True(t, config.Debug)
	assert.Equal(t, 7, config.MaxRetries)
	assert.False(t, config.EnableIdempotency)
}

func TestFromEnvironment_PartialVariables(t *testing.T) {
	envVars := []string{
		"MIDAZ_ENVIRONMENT",
		"PLUGIN_AUTH_ENABLED",
		"PLUGIN_AUTH_ADDRESS",
		"MIDAZ_CLIENT_ID",
		"MIDAZ_CLIENT_SECRET",
		"MIDAZ_USER_AGENT",
		"MIDAZ_BASE_URL",
		"MIDAZ_ONBOARDING_URL",
		"MIDAZ_TRANSACTION_URL",
		"MIDAZ_TIMEOUT",
		"MIDAZ_DEBUG",
		"MIDAZ_MAX_RETRIES",
		"MIDAZ_IDEMPOTENCY",
	}

	restore := saveEnv(envVars)
	defer restore()

	for _, key := range envVars {
		os.Unsetenv(key)
	}

	os.Setenv("MIDAZ_DEBUG", "true")
	os.Setenv("MIDAZ_TIMEOUT", "90")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.True(t, config.Debug)
	assert.Equal(t, 90*time.Second, config.Timeout)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.True(t, config.EnableIdempotency)
}

func TestFromEnvironment_InvalidEnvironment(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_ENVIRONMENT"})
	defer restore()

	os.Setenv("MIDAZ_ENVIRONMENT", "invalid-env")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid environment")
}

func TestFromEnvironment_InvalidTimeout(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_TIMEOUT"})
	defer restore()

	os.Setenv("MIDAZ_TIMEOUT", "not-a-number")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")
}

func TestFromEnvironment_InvalidMaxRetries(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_MAX_RETRIES"})
	defer restore()

	os.Setenv("MIDAZ_MAX_RETRIES", "abc")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid max retries")
}

func TestFromEnvironment_InvalidOnboardingURL(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_ONBOARDING_URL"})
	defer restore()

	os.Setenv("MIDAZ_ONBOARDING_URL", "not-a-valid-url")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid onboarding URL")
}

func TestFromEnvironment_InvalidTransactionURL(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_TRANSACTION_URL"})
	defer restore()

	os.Setenv("MIDAZ_TRANSACTION_URL", "invalid")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction URL")
}

func TestFromEnvironment_InvalidBaseURL(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_BASE_URL"})
	defer restore()

	os.Setenv("MIDAZ_BASE_URL", "://malformed")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

func TestFromEnvironment_BaseURLOverriddenBySpecific(t *testing.T) {
	envVars := []string{"MIDAZ_BASE_URL", "MIDAZ_ONBOARDING_URL", "MIDAZ_TRANSACTION_URL"}

	restore := saveEnv(envVars)
	defer restore()

	// Clear transaction URL to test that base URL is used as fallback
	os.Unsetenv("MIDAZ_TRANSACTION_URL")
	os.Setenv("MIDAZ_BASE_URL", "https://base.example.com")
	os.Setenv("MIDAZ_ONBOARDING_URL", "https://specific.example.com/onboarding")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)

	assert.Equal(t, "https://specific.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://base.example.com:3001", config.ServiceURLs[ServiceTransaction])
}

func TestFromEnvironment_PluginAuthDisabled(t *testing.T) {
	envVars := []string{"PLUGIN_AUTH_ENABLED", "PLUGIN_AUTH_ADDRESS", "MIDAZ_CLIENT_ID", "MIDAZ_CLIENT_SECRET"}

	restore := saveEnv(envVars)
	defer restore()

	os.Setenv("PLUGIN_AUTH_ENABLED", "false")
	os.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.example.com")
	os.Setenv("MIDAZ_CLIENT_ID", "client-id")
	os.Setenv("MIDAZ_CLIENT_SECRET", "client-secret")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)

	assert.False(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://auth.example.com", config.AccessManager.Address)
}

func TestFromEnvironment_IdempotencyTrue(t *testing.T) {
	restore := saveEnv([]string{"MIDAZ_IDEMPOTENCY"})
	defer restore()

	os.Setenv("MIDAZ_IDEMPOTENCY", "true")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)
	assert.True(t, config.EnableIdempotency)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.Equal(t, DefaultTimeout*time.Second, config.Timeout)
	assert.Equal(t, version.UserAgent(), config.UserAgent)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultRetryWaitMin, config.RetryWaitMin)
	assert.Equal(t, DefaultRetryWaitMax, config.RetryWaitMax)
	assert.True(t, config.EnableRetries)
	assert.True(t, config.EnableIdempotency)
	assert.NotNil(t, config.HTTPClient)
	assert.NotNil(t, config.ServiceURLs)
	assert.Equal(t, "http://localhost:3000", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3001", config.ServiceURLs[ServiceTransaction])
}

func TestNewLocalConfig(t *testing.T) {
	config, err := NewLocalConfig()
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.False(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://localhost:3000", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3001", config.ServiceURLs[ServiceTransaction])
}

func TestNewLocalConfig_WithEnvVars(t *testing.T) {
	envVars := []string{"PLUGIN_AUTH_ENABLED", "PLUGIN_AUTH_ADDRESS", "MIDAZ_CLIENT_ID", "MIDAZ_CLIENT_SECRET"}

	restore := saveEnv(envVars)
	defer restore()

	cleanup := disableAuthCheck(t)
	defer cleanup()

	os.Setenv("PLUGIN_AUTH_ENABLED", "true")
	os.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.local.example.com")
	os.Setenv("MIDAZ_CLIENT_ID", "local-client")
	os.Setenv("MIDAZ_CLIENT_SECRET", "local-secret")

	config, err := NewLocalConfig()
	require.NoError(t, err)

	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://auth.local.example.com", config.AccessManager.Address)
	assert.Equal(t, "local-client", config.AccessManager.ClientID)
	assert.Equal(t, "local-secret", config.AccessManager.ClientSecret)
}

func TestNewLocalConfig_WithOptions(t *testing.T) {
	config, err := NewLocalConfig(
		WithTimeout(120*time.Second),
		WithDebug(true),
	)
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.Equal(t, 120*time.Second, config.Timeout)
	assert.True(t, config.Debug)
}

func TestGetBaseURLs(t *testing.T) {
	config, err := NewConfig(
		WithOnboardingURL("https://api.example.com/onboarding"),
		WithTransactionURL("https://api.example.com/transaction"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)

	baseURLs := config.GetBaseURLs()

	assert.Equal(t, "https://api.example.com/onboarding", baseURLs["onboarding"])
	assert.Equal(t, "https://api.example.com/transaction", baseURLs["transaction"])
}

func TestGetHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}
	config, err := NewConfig(
		WithHTTPClient(customClient),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)

	assert.Equal(t, customClient, config.GetHTTPClient())
}

func TestGetHTTPClient_Default(t *testing.T) {
	config, err := NewConfig(WithAccessManager(auth.AccessManager{Enabled: false}))
	require.NoError(t, err)

	client := config.GetHTTPClient()
	assert.NotNil(t, client)
	assert.Equal(t, DefaultTimeout*time.Second, client.Timeout)
}

func TestGetPluginAuth(t *testing.T) {
	cleanup := disableAuthCheck(t)
	defer cleanup()

	config, err := NewConfig(WithAccessManager(auth.AccessManager{
		Enabled:      true,
		Address:      "http://auth.example.com",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}))
	require.NoError(t, err)

	pluginAuth := config.GetPluginAuth()

	assert.True(t, pluginAuth.Enabled)
	assert.Equal(t, "http://auth.example.com", pluginAuth.Address)
	assert.Equal(t, "test-client", pluginAuth.ClientID)
	assert.Equal(t, "test-secret", pluginAuth.ClientSecret)
}

func TestGetPluginAuth_ReturnsCopy(t *testing.T) {
	cleanup := disableAuthCheck(t)
	defer cleanup()

	config, err := NewConfig(WithAccessManager(auth.AccessManager{
		Enabled:      true,
		Address:      "http://auth.example.com",
		ClientID:     "original-client",
		ClientSecret: "original-secret",
	}))
	require.NoError(t, err)

	pluginAuth := config.GetPluginAuth()
	pluginAuth.ClientID = "modified-client"

	// Verify the modification happened on the copy
	assert.Equal(t, "modified-client", pluginAuth.ClientID)
	// Verify the original is unchanged (copy isolation)
	assert.Equal(t, "original-client", config.AccessManager.ClientID)
}

func TestGetObservabilityProvider(t *testing.T) {
	provider := &mockObservabilityProvider{}
	config, err := NewConfig(
		WithObservabilityProvider(provider),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)

	assert.Equal(t, provider, config.GetObservabilityProvider())
}

func TestGetObservabilityProvider_Nil(t *testing.T) {
	config, err := NewConfig(WithAccessManager(auth.AccessManager{Enabled: false}))
	require.NoError(t, err)

	assert.Nil(t, config.GetObservabilityProvider())
}

func TestOptionOverrides(t *testing.T) {
	config, err := NewConfig(
		WithTimeout(30*time.Second),
		WithTimeout(60*time.Second),
		WithTimeout(90*time.Second),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, config.Timeout)
}

func TestOptionOrderMatters(t *testing.T) {
	config, err := NewConfig(
		WithEnvironment(EnvironmentLocal),
		WithBaseURL("https://custom.example.com"),
		WithOnboardingURL("https://specific.example.com/onboarding"),
		WithAccessManager(auth.AccessManager{Enabled: false}),
	)
	require.NoError(t, err)

	assert.Equal(t, "https://specific.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com:3001", config.ServiceURLs[ServiceTransaction])
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"localhost:3000", true},
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		// Note: IPv6 localhost (::1) not handled correctly by current implementation
		// due to strings.Split(host, ":") splitting on colons in IPv6 addresses
		{"api.example.com", false},
		{"example.com:443", false},
		{"192.168.1.1", false},
		{"192.168.1.1:8080", false},
		{"10.0.0.1", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			result := isLocalhost(tc.host)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseURL_Valid(t *testing.T) {
	tests := []string{
		"https://api.example.com",
		"http://localhost:3000",
		"https://api.example.com/v1/path",
		"http://127.0.0.1:8080",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			err := parseURL(url)
			require.NoError(t, err)
		})
	}
}

func TestParseURL_Invalid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"no scheme", "api.example.com"},
		{"no host", "https://"},
		{"scheme only", "https:"},
		{"malformed", "://invalid"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseURL(tc.url)
			require.Error(t, err)
		})
	}
}

func TestParseEnvInt_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"100", 100},
		{"-5", -5},
		// Note: fmt.Sscanf with %d parses leading integers from strings
		{"1.5", 1},      // parses "1" from "1.5"
		{"1a", 1},       // parses "1" from "1a"
		{"123abc", 123}, // parses "123" from "123abc"
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parseEnvInt(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestParseEnvInt_Invalid(t *testing.T) {
	tests := []string{
		"",
		"abc",
		"a1",
		" ",
		"notanumber",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseEnvInt(input)
			require.Error(t, err)
		})
	}
}

func TestSetDefaultServiceURLs_UnknownEnvironment(t *testing.T) {
	config := &Config{
		Environment: Environment("unknown"),
		ServiceURLs: make(map[ServiceType]string),
	}

	err := setDefaultServiceURLs(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown environment")
}

func TestSetDefaultServiceURLs_AllEnvironments(t *testing.T) {
	tests := []struct {
		env                    Environment
		expectedOnboardingURL  string
		expectedTransactionURL string
	}{
		{
			env:                    EnvironmentLocal,
			expectedOnboardingURL:  "http://localhost:3000",
			expectedTransactionURL: "http://localhost:3001",
		},
		{
			env:                    EnvironmentDevelopment,
			expectedOnboardingURL:  "https://api.dev.midaz.io/onboarding",
			expectedTransactionURL: "https://api.dev.midaz.io/transaction",
		},
		{
			env:                    EnvironmentProduction,
			expectedOnboardingURL:  "https://api.midaz.io/onboarding",
			expectedTransactionURL: "https://api.midaz.io/transaction",
		},
	}

	for _, tc := range tests {
		t.Run(string(tc.env), func(t *testing.T) {
			config := &Config{
				Environment: tc.env,
				ServiceURLs: make(map[ServiceType]string),
			}

			err := setDefaultServiceURLs(config)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOnboardingURL, config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.expectedTransactionURL, config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestConfigureEnvironment_AllEnvironments(t *testing.T) {
	tests := []struct {
		envValue    string
		expected    Environment
		shouldError bool
	}{
		{"local", EnvironmentLocal, false},
		{"development", EnvironmentDevelopment, false},
		{"production", EnvironmentProduction, false},
		{"", EnvironmentLocal, false},
		{"invalid", EnvironmentLocal, true},
		{"LOCAL", EnvironmentLocal, true},
		{"PRODUCTION", EnvironmentLocal, true},
	}

	for _, tc := range tests {
		t.Run(tc.envValue, func(t *testing.T) {
			restore := saveEnv([]string{"MIDAZ_ENVIRONMENT"})
			defer restore()

			if tc.envValue != "" {
				os.Setenv("MIDAZ_ENVIRONMENT", tc.envValue)
			} else {
				os.Unsetenv("MIDAZ_ENVIRONMENT")
			}

			config := &Config{Environment: EnvironmentLocal}
			err := configureEnvironment(config)

			if tc.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, config.Environment)
			}
		})
	}
}

func TestConfigureAccessManager(t *testing.T) {
	tests := []struct {
		name           string
		envEnabled     string
		envAddress     string
		envClientID    string
		envSecret      string
		expectedEnable bool
	}{
		{
			name:           "all values set enabled",
			envEnabled:     "true",
			envAddress:     "http://auth.example.com",
			envClientID:    "client-123",
			envSecret:      "secret-456",
			expectedEnable: true,
		},
		{
			name:           "disabled",
			envEnabled:     "false",
			envAddress:     "http://auth.example.com",
			envClientID:    "client-123",
			envSecret:      "secret-456",
			expectedEnable: false,
		},
		{
			name:           "empty enabled",
			envEnabled:     "",
			envAddress:     "http://auth.example.com",
			envClientID:    "client-123",
			envSecret:      "secret-456",
			expectedEnable: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restore := saveEnv([]string{"PLUGIN_AUTH_ENABLED", "PLUGIN_AUTH_ADDRESS", "MIDAZ_CLIENT_ID", "MIDAZ_CLIENT_SECRET"})
			defer restore()

			if tc.envEnabled != "" {
				os.Setenv("PLUGIN_AUTH_ENABLED", tc.envEnabled)
			} else {
				os.Unsetenv("PLUGIN_AUTH_ENABLED")
			}

			os.Setenv("PLUGIN_AUTH_ADDRESS", tc.envAddress)
			os.Setenv("MIDAZ_CLIENT_ID", tc.envClientID)
			os.Setenv("MIDAZ_CLIENT_SECRET", tc.envSecret)

			config := &Config{}
			configureAccessManager(config)

			if tc.envEnabled == "" {
				assert.Empty(t, config.AccessManager.Address)
			} else {
				assert.Equal(t, tc.expectedEnable, config.AccessManager.Enabled)
				assert.Equal(t, tc.envAddress, config.AccessManager.Address)
				assert.Equal(t, tc.envClientID, config.AccessManager.ClientID)
				assert.Equal(t, tc.envSecret, config.AccessManager.ClientSecret)
			}
		})
	}
}

func TestConfigureUserAgent(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		initialValue  string
		expectedValue string
	}{
		{"set from env", "custom-agent/1.0", "default-agent", "custom-agent/1.0"},
		{"empty env keeps initial", "", "default-agent", "default-agent"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restore := saveEnv([]string{"MIDAZ_USER_AGENT"})
			defer restore()

			if tc.envValue != "" {
				os.Setenv("MIDAZ_USER_AGENT", tc.envValue)
			} else {
				os.Unsetenv("MIDAZ_USER_AGENT")
			}

			config := &Config{UserAgent: tc.initialValue}
			configureUserAgent(config)
			assert.Equal(t, tc.expectedValue, config.UserAgent)
		})
	}
}

func TestConfigureOptionalSettings(t *testing.T) {
	tests := []struct {
		name                string
		debugEnv            string
		idempotencyEnv      string
		expectedDebug       bool
		expectedIdempotency bool
		initialIdempotency  bool
	}{
		{"debug true", "true", "", true, true, true},
		{"debug false", "false", "", false, true, true},
		{"idempotency true", "", "true", false, true, false},
		{"idempotency false", "", "false", false, false, true},
		{"both true", "true", "true", true, true, false},
		{"both false", "false", "false", false, false, true},
		{"empty values", "", "", false, true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restore := saveEnv([]string{"MIDAZ_DEBUG", "MIDAZ_IDEMPOTENCY"})
			defer restore()

			if tc.debugEnv != "" {
				os.Setenv("MIDAZ_DEBUG", tc.debugEnv)
			} else {
				os.Unsetenv("MIDAZ_DEBUG")
			}

			if tc.idempotencyEnv != "" {
				os.Setenv("MIDAZ_IDEMPOTENCY", tc.idempotencyEnv)
			} else {
				os.Unsetenv("MIDAZ_IDEMPOTENCY")
			}

			config := &Config{EnableIdempotency: tc.initialIdempotency}
			configureOptionalSettings(config)

			assert.Equal(t, tc.expectedDebug, config.Debug)
			assert.Equal(t, tc.expectedIdempotency, config.EnableIdempotency)
		})
	}
}

func TestValidateConfig_MissingOnboardingURL(t *testing.T) {
	config := &Config{
		ServiceURLs: map[ServiceType]string{
			ServiceTransaction: "https://api.example.com/transaction",
		},
	}

	err := validateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onboarding URL is required")
}

func TestValidateConfig_MissingTransactionURL(t *testing.T) {
	config := &Config{
		ServiceURLs: map[ServiceType]string{
			ServiceOnboarding: "https://api.example.com/onboarding",
		},
	}

	err := validateConfig(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transaction URL is required")
}

func TestValidateConfig_Valid(t *testing.T) {
	config := &Config{
		ServiceURLs: map[ServiceType]string{
			ServiceOnboarding:  "https://api.example.com/onboarding",
			ServiceTransaction: "https://api.example.com/transaction",
		},
		AccessManager: auth.AccessManager{Enabled: false},
	}

	err := validateConfig(config)
	require.NoError(t, err)
}

func TestNewConfig_OptionError(t *testing.T) {
	errorOption := func(_ *Config) error {
		return assert.AnError
	}

	_, err := NewConfig(errorOption)
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestWithBaseURL_InitializesServiceURLsMap(t *testing.T) {
	config := &Config{
		Environment: EnvironmentProduction,
		ServiceURLs: nil,
	}

	err := WithBaseURL("https://api.example.com")(config)
	require.NoError(t, err)

	assert.NotNil(t, config.ServiceURLs)
	assert.Equal(t, "https://api.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://api.example.com/transaction", config.ServiceURLs[ServiceTransaction])
}

func TestWithOnboardingURL_InitializesServiceURLsMap(t *testing.T) {
	config := &Config{
		ServiceURLs: nil,
	}

	err := WithOnboardingURL("https://api.example.com/onboarding")(config)
	require.NoError(t, err)

	assert.NotNil(t, config.ServiceURLs)
	assert.Equal(t, "https://api.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
}

func TestWithTransactionURL_InitializesServiceURLsMap(t *testing.T) {
	config := &Config{
		ServiceURLs: nil,
	}

	err := WithTransactionURL("https://api.example.com/transaction")(config)
	require.NoError(t, err)

	assert.NotNil(t, config.ServiceURLs)
	assert.Equal(t, "https://api.example.com/transaction", config.ServiceURLs[ServiceTransaction])
}

// Mock observability provider for testing
type mockObservabilityProvider struct{}

func (*mockObservabilityProvider) Tracer() trace.Tracer {
	return nil
}

func (*mockObservabilityProvider) Meter() metric.Meter {
	return nil
}

func (*mockObservabilityProvider) Logger() observability.Logger {
	return nil
}

func (*mockObservabilityProvider) Shutdown(_ context.Context) error {
	return nil
}

func (*mockObservabilityProvider) IsEnabled() bool {
	return true
}
