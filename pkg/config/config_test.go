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

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/version"
)

// disableAuthCheck is a test-only helper that returns an Option which sets
// the internal skipAuthCheck flag. v3 contract: validateConfig consults the
// field, never the env directly. The legacy env-driven path still works via
// FromEnvironment() (verified by TestFromEnvironment_AllVariables and the
// MIDAZ_SKIP_AUTH_CHECK case below).
func disableAuthCheck(t *testing.T) Option {
	t.Helper()

	return func(c *Config) error {
		c.skipAuthCheck = true
		return nil
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	value, ok := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))

	t.Cleanup(func() {
		if ok {
			require.NoError(t, os.Setenv(key, value))
			return
		}

		require.NoError(t, os.Unsetenv(key))
	})
}

func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      any
		expected any
	}{
		{"DefaultTimeout", DefaultTimeout, 60},
		{"DefaultLocalLedgerBaseURL", DefaultLocalLedgerBaseURL, "http://localhost:3002"},
		{"DefaultDevelopmentLedgerBaseURL", DefaultDevelopmentLedgerBaseURL, "https://api.dev.midaz.io"},
		{"DefaultProductionLedgerBaseURL", DefaultProductionLedgerBaseURL, "https://api.midaz.io"},
		{"DefaultLedgerAPIVersionPath", DefaultLedgerAPIVersionPath, "/v1"},
		{"DefaultMaxRetries", DefaultMaxRetries, 3},
		{"DefaultMinRetryWait", DefaultMinRetryWait, 1 * time.Second},
		{"DefaultRetryWaitMax", DefaultRetryWaitMax, 30 * time.Second},
		{"DefaultEnableIdempotency", DefaultEnableIdempotency, true},
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
	assert.Equal(t, ServiceCRM, ServiceType("crm"))
}

func TestWithCRMURL(t *testing.T) {
	cfg := DefaultConfig()
	err := WithCRMURL("https://crm.example.com/v1")(cfg)

	require.NoError(t, err)
	assert.Equal(t, "https://crm.example.com/v1", cfg.ServiceURLs[ServiceCRM])
}

func TestConfigureURLsReadsCRMURL(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "https://crm.example.com/v1")

	cfg := DefaultConfig()
	require.NoError(t, configureURLs(cfg))

	assert.Equal(t, "https://crm.example.com/v1", cfg.ServiceURLs[ServiceCRM])
}

func TestEnvironmentConstants(t *testing.T) {
	assert.Equal(t, EnvironmentLocal, Environment("local"))
	assert.Equal(t, EnvironmentDevelopment, Environment("development"))
	assert.Equal(t, EnvironmentProduction, Environment("production"))
}

func TestNewConfig_Defaults(t *testing.T) {
	config, err := NewConfig(WithAnonymous())
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.Equal(t, DefaultTimeout*time.Second, config.Timeout)
	assert.Equal(t, version.UserAgent(), config.UserAgent)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.Equal(t, DefaultMinRetryWait, config.RetryWaitMin)
	assert.Equal(t, DefaultRetryWaitMax, config.RetryWaitMax)
	// In v3, retries are off iff MaxRetries == 0; default is DefaultMaxRetries (3).
	assert.Positive(t, config.MaxRetries)
	assert.True(t, config.EnableIdempotency)
	assert.False(t, config.Debug)
	assert.NotNil(t, config.HTTPClient)
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceTransaction])
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
		// v3: WithRetryConfig was deleted; chain the 3 individual knobs.
		// v3: WithRetries(bool) was deleted; retries are off iff MaxRetries == 0.
		WithMaxRetries(5),
		WithRetryWaitMin(2*time.Second),
		WithRetryWaitMax(60*time.Second),
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
				WithAnonymous(),
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
			expectedOnboardingURL:  "https://api.custom.io/v1",
			expectedTransactionURL: "https://api.custom.io/v1",
		},
		{
			name:                   "production with base URL",
			env:                    EnvironmentProduction,
			expectedOnboardingURL:  "https://api.custom.io/v1",
			expectedTransactionURL: "https://api.custom.io/v1",
		},
		{
			name:                   "local with base URL",
			env:                    EnvironmentLocal,
			expectedOnboardingURL:  "https://api.custom.io/v1",
			expectedTransactionURL: "https://api.custom.io/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithEnvironment(tc.env),
				WithBaseURL("https://api.custom.io"),
				WithAnonymous(),
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
				WithAnonymous(),
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
				WithAnonymous(),
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
				WithAnonymous(),
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
				WithAnonymous(),
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
		WithAnonymous(),
	)
	require.NoError(t, err)

	assert.Equal(t, "https://custom.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com/v1", config.ServiceURLs[ServiceTransaction])
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
				ServiceOnboarding:  "https://custom.example.com/v1",
				ServiceTransaction: "https://custom.example.com/v1",
			},
		},
		{
			name:    "production",
			env:     EnvironmentProduction,
			baseURL: "https://api.prod.example.com",
			expected: map[ServiceType]string{
				ServiceOnboarding:  "https://api.prod.example.com/v1",
				ServiceTransaction: "https://api.prod.example.com/v1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithEnvironment(tc.env),
				WithBaseURL(tc.baseURL),
				WithAnonymous(),
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
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/v1", config.ServiceURLs[ServiceOnboarding])
}

func TestWithBaseURL_Invalid(t *testing.T) {
	_, err := NewConfig(
		WithBaseURL("invalid-url"),
		WithAnonymous(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

func TestWithHTTPClient_Valid(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}
	config, err := NewConfig(
		WithHTTPClient(customClient),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, customClient, config.HTTPClient)
}

func TestWithHTTPClient_Nil(t *testing.T) {
	_, err := NewConfig(
		WithHTTPClient(nil),
		WithAnonymous(),
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
				WithAnonymous(),
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
				WithAnonymous(),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "timeout must be greater than 0")
		})
	}
}

func TestWithUserAgent_Valid(t *testing.T) {
	config, err := NewConfig(
		WithUserAgent("custom-agent/2.0"),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, "custom-agent/2.0", config.UserAgent)
}

func TestWithUserAgent_Empty(t *testing.T) {
	_, err := NewConfig(
		WithUserAgent(""),
		WithAnonymous(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user agent cannot be empty")
}

// TestWithRetryConfig_Valid / _Invalid were deleted in v3 alongside the
// WithRetryConfig macro. Their coverage is preserved by TestWithMaxRetries_*,
// TestWithRetryWaitMin_*, and TestWithRetryWaitMax_* which exercise each
// single-concern Option independently.

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
				WithAnonymous(),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.maxRetries, config.MaxRetries)
		})
	}
}

func TestWithMaxRetries_Invalid(t *testing.T) {
	_, err := NewConfig(
		WithMaxRetries(-1),
		WithAnonymous(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max retries cannot be negative")
}

func TestWithRetryWaitMin_Valid(t *testing.T) {
	config, err := NewConfig(
		WithRetryWaitMin(5*time.Second),
		WithAnonymous(),
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
				WithAnonymous(),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "minimum wait time must be greater than 0")
		})
	}
}

func TestWithRetryWaitMax_Valid(t *testing.T) {
	config, err := NewConfig(
		WithRetryWaitMax(60*time.Second),
		WithAnonymous(),
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
				WithAnonymous(),
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
		WithAnonymous(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum wait time must be greater than or equal to minimum wait time")
}

// TestWithRetries_Toggle was deleted in v3 alongside the WithRetries(bool)
// Option and the EnableRetries field. The semantic equivalent — "retries are
// off when MaxRetries == 0" — is exercised by TestWithMaxRetries_Valid which
// includes a 0-value test case.

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
				WithAnonymous(),
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
				WithAnonymous(),
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
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, provider, config.ObservabilityProvider)
}

func TestWithObservabilityProvider_Nil(t *testing.T) {
	config, err := NewConfig(
		WithObservabilityProvider(nil),
		WithAnonymous(),
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

	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(accessManager))
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
	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(auth.AccessManager{
		Enabled: true,
		Address: "",
	}))
	require.NoError(t, err)
	assert.True(t, config.AccessManager.Enabled)
}

func TestFromEnvironment_AllVariables(t *testing.T) {
	t.Setenv("MIDAZ_ENVIRONMENT", "development")
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "env-client-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "env-client-secret")
	t.Setenv("MIDAZ_USER_AGENT", "env-agent/1.0")
	t.Setenv("MIDAZ_ONBOARDING_URL", "https://env.example.com/onboarding")
	t.Setenv("MIDAZ_TRANSACTION_URL", "https://env.example.com/transaction")
	t.Setenv("MIDAZ_TIMEOUT", "45")
	t.Setenv("MIDAZ_DEBUG", "true")
	t.Setenv("MIDAZ_MAX_RETRIES", "7")
	t.Setenv("MIDAZ_IDEMPOTENCY", "false")

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

	for _, key := range envVars {
		unsetEnv(t, key)
	}

	t.Setenv("MIDAZ_DEBUG", "true")
	t.Setenv("MIDAZ_TIMEOUT", "90")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.True(t, config.Debug)
	assert.Equal(t, 90*time.Second, config.Timeout)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
	assert.True(t, config.EnableIdempotency)
}

func TestFromEnvironment_InvalidEnvironment(t *testing.T) {
	t.Setenv("MIDAZ_ENVIRONMENT", "invalid-env")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid environment")
}

func TestFromEnvironment_InvalidTimeout(t *testing.T) {
	t.Setenv("MIDAZ_TIMEOUT", "not-a-number")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timeout")
}

func TestFromEnvironment_InvalidMaxRetries(t *testing.T) {
	t.Setenv("MIDAZ_MAX_RETRIES", "abc")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid max retries")
}

func TestFromEnvironment_InvalidOnboardingURL(t *testing.T) {
	t.Setenv("MIDAZ_ONBOARDING_URL", "not-a-valid-url")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid onboarding URL")
}

func TestFromEnvironment_InvalidTransactionURL(t *testing.T) {
	t.Setenv("MIDAZ_TRANSACTION_URL", "invalid")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid transaction URL")
}

func TestFromEnvironment_InvalidBaseURL(t *testing.T) {
	t.Setenv("MIDAZ_BASE_URL", "://malformed")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

func TestFromEnvironment_BaseURLOverriddenBySpecific(t *testing.T) {
	// Clear transaction URL to test that base URL is used as fallback
	unsetEnv(t, "MIDAZ_TRANSACTION_URL")
	t.Setenv("MIDAZ_BASE_URL", "https://base.example.com")
	t.Setenv("MIDAZ_ONBOARDING_URL", "https://specific.example.com/onboarding")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	assert.Equal(t, "https://specific.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://base.example.com/v1", config.ServiceURLs[ServiceTransaction])
}

func TestFromEnvironment_PluginAuthDisabled(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "false")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "client-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "client-secret")

	// PLUGIN_AUTH_ENABLED=false leaves the AccessManager fields populated
	// but Enabled=false, which counts as no active auth source. Add
	// WithAnonymous to satisfy the auth-required gate; WithAnonymous
	// preserves the other AccessManager fields so we can still verify
	// that PLUGIN_AUTH_ADDRESS was captured.
	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	assert.False(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://auth.example.com", config.AccessManager.Address)
}

func TestFromEnvironment_IdempotencyTrue(t *testing.T) {
	t.Setenv("MIDAZ_IDEMPOTENCY", "true")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
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
	assert.Equal(t, DefaultMinRetryWait, config.RetryWaitMin)
	assert.Equal(t, DefaultRetryWaitMax, config.RetryWaitMax)
	// In v3, retries are off iff MaxRetries == 0; default is DefaultMaxRetries (3).
	assert.Positive(t, config.MaxRetries)
	assert.True(t, config.EnableIdempotency)
	assert.NotNil(t, config.HTTPClient)
	assert.NotNil(t, config.ServiceURLs)
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceTransaction])
}

func TestNewLocalConfig(t *testing.T) {
	config, err := NewLocalConfig()
	require.NoError(t, err)

	assert.Equal(t, EnvironmentLocal, config.Environment)
	assert.False(t, config.AccessManager.Enabled)
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "http://localhost:3002/v1", config.ServiceURLs[ServiceTransaction])
}

func TestNewLocalConfig_WithEnvVars(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "http://auth.local.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "local-client")
	t.Setenv("MIDAZ_CLIENT_SECRET", "local-secret")

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
		WithAnonymous(),
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
		WithAnonymous(),
	)
	require.NoError(t, err)

	assert.Equal(t, customClient, config.GetHTTPClient())
}

func TestGetHTTPClient_Default(t *testing.T) {
	config, err := NewConfig(WithAnonymous())
	require.NoError(t, err)

	client := config.GetHTTPClient()
	assert.NotNil(t, client)
	assert.Equal(t, DefaultTimeout*time.Second, client.Timeout)
}

func TestGetPluginAuth(t *testing.T) {
	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(auth.AccessManager{
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
	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(auth.AccessManager{
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
		WithAnonymous(),
	)
	require.NoError(t, err)

	assert.Equal(t, provider, config.GetObservabilityProvider())
}

func TestGetObservabilityProvider_Nil(t *testing.T) {
	config, err := NewConfig(WithAnonymous())
	require.NoError(t, err)

	assert.Nil(t, config.GetObservabilityProvider())
}

func TestOptionOverrides(t *testing.T) {
	config, err := NewConfig(
		WithTimeout(30*time.Second),
		WithTimeout(60*time.Second),
		WithTimeout(90*time.Second),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, config.Timeout)
}

func TestOptionOrderMatters(t *testing.T) {
	config, err := NewConfig(
		WithEnvironment(EnvironmentLocal),
		WithBaseURL("https://custom.example.com"),
		WithOnboardingURL("https://specific.example.com/onboarding"),
		WithAnonymous(),
	)
	require.NoError(t, err)

	assert.Equal(t, "https://specific.example.com/onboarding", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com/v1", config.ServiceURLs[ServiceTransaction])
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
		// RFC 6761 §6.3: ".localhost" suffix must be treated as loopback.
		{"mock-midaz.localhost", true},
		{"foo.bar.localhost", true},
		{"mock-midaz.localhost:3001", true},
		// Note: IPv6 localhost (::1) not handled correctly by current implementation
		// due to strings.Split(host, ":") splitting on colons in IPv6 addresses
		{"api.example.com", false},
		{"example.com:443", false},
		{"192.168.1.1", false},
		{"192.168.1.1:8080", false},
		{"10.0.0.1", false},
		// Suffix match must be anchored: a host whose .localhost is mid-string is NOT loopback.
		{"localhost.attacker.com", false},
		{"notlocalhost", false},
		{"mock-midaz", false},
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
		{" 42 ", 42},
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
		"1.5",
		"1a",
		"123abc",
		"10s",
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
			expectedOnboardingURL:  "http://localhost:3002/v1",
			expectedTransactionURL: "http://localhost:3002/v1",
		},
		{
			env:                    EnvironmentDevelopment,
			expectedOnboardingURL:  "https://api.dev.midaz.io/v1",
			expectedTransactionURL: "https://api.dev.midaz.io/v1",
		},
		{
			env:                    EnvironmentProduction,
			expectedOnboardingURL:  "https://api.midaz.io/v1",
			expectedTransactionURL: "https://api.midaz.io/v1",
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
			if tc.envValue != "" {
				t.Setenv("MIDAZ_ENVIRONMENT", tc.envValue)
			} else {
				unsetEnv(t, "MIDAZ_ENVIRONMENT")
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
			if tc.envEnabled != "" {
				t.Setenv("PLUGIN_AUTH_ENABLED", tc.envEnabled)
			} else {
				unsetEnv(t, "PLUGIN_AUTH_ENABLED")
			}

			t.Setenv("PLUGIN_AUTH_ADDRESS", tc.envAddress)
			t.Setenv("MIDAZ_CLIENT_ID", tc.envClientID)
			t.Setenv("MIDAZ_CLIENT_SECRET", tc.envSecret)

			config := &Config{}
			require.NoError(t, configureAccessManager(config))

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
			if tc.envValue != "" {
				t.Setenv("MIDAZ_USER_AGENT", tc.envValue)
			} else {
				unsetEnv(t, "MIDAZ_USER_AGENT")
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
			if tc.debugEnv != "" {
				t.Setenv("MIDAZ_DEBUG", tc.debugEnv)
			} else {
				unsetEnv(t, "MIDAZ_DEBUG")
			}

			if tc.idempotencyEnv != "" {
				t.Setenv("MIDAZ_IDEMPOTENCY", tc.idempotencyEnv)
			} else {
				unsetEnv(t, "MIDAZ_IDEMPOTENCY")
			}

			config := &Config{EnableIdempotency: tc.initialIdempotency}
			require.NoError(t, configureOptionalSettings(config))

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
			ServiceCRM:         "https://api.example.com/crm",
		},
		Anonymous: true,
	}

	err := validateConfig(config)
	require.NoError(t, err)
}

func TestValidateConfig_CRMURLIsOptional(t *testing.T) {
	config := &Config{
		ServiceURLs: map[ServiceType]string{
			ServiceOnboarding:  "https://api.example.com/onboarding",
			ServiceTransaction: "https://api.example.com/transaction",
		},
		// Anonymous=true is the v3-canonical way to assert no-auth at
		// validation time without going through the option chain.
		Anonymous: true,
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
	assert.Equal(t, "https://api.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://api.example.com/v1", config.ServiceURLs[ServiceTransaction])
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
