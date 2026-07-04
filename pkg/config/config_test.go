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

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/version"
)

// disableAuthCheck is a test-only helper that returns an Option which sets
// the internal skipAuthCheck flag. v3 contract: validateConfig consults the
// field, never the env directly.
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
		{"DefaultExposeErrorBody", DefaultExposeErrorBody, false},
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
	assert.Equal(t, ServiceTracer, ServiceType("tracer"))
}

func TestConfigureURLsReadsTracerURL(t *testing.T) {
	t.Setenv("MIDAZ_TRACER_URL", "https://tracer.example.com/v1")

	cfg := DefaultConfig()
	require.NoError(t, configureURLs(cfg))

	assert.Equal(t, "https://tracer.example.com/v1", cfg.ServiceURLs[ServiceTracer])
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
	assert.False(t, config.ExposeErrorBody)
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
		WithLedgerURL("https://custom.example.com/v1"),
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
		WithErrorBodyExposure(false),
		WithObservabilityProvider(mockProvider),
		WithAccessManager(auth.AccessManager{
			Enabled:      false,
			Address:      "https://auth.example.com",
			ClientID:     "test-client",
			ClientSecret: "test-secret",
		}),
	)
	require.NoError(t, err)

	assert.Equal(t, EnvironmentProduction, config.Environment)
	assert.Equal(t, "https://custom.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://custom.example.com/v1", config.ServiceURLs[ServiceTransaction])
	assert.NotSame(t, customClient, config.HTTPClient)
	assert.Equal(t, customClient.Timeout, config.HTTPClient.Timeout)
	assert.NotNil(t, config.HTTPClient.CheckRedirect)
	assert.Equal(t, 90*time.Second, config.Timeout)
	assert.Equal(t, "test-agent/1.0", config.UserAgent)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 2*time.Second, config.RetryWaitMin)
	assert.Equal(t, 60*time.Second, config.RetryWaitMax)
	assert.True(t, config.Debug)
	assert.False(t, config.EnableIdempotency)
	assert.False(t, config.ExposeErrorBody)
	assert.Equal(t, mockProvider, config.ObservabilityProvider)
	assert.Equal(t, "https://auth.example.com", config.AccessManager.Address)
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
		name              string
		env               Environment
		expectedLedgerURL string
	}{
		{
			name:              "development with base URL",
			env:               EnvironmentDevelopment,
			expectedLedgerURL: "https://api.custom.io/v1",
		},
		{
			name:              "production with base URL",
			env:               EnvironmentProduction,
			expectedLedgerURL: "https://api.custom.io/v1",
		},
		{
			name:              "local with base URL",
			env:               EnvironmentLocal,
			expectedLedgerURL: "https://api.custom.io/v1",
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
			assert.Equal(t, tc.expectedLedgerURL, config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.expectedLedgerURL, config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestWithLedgerURL_Valid(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"https URL", "https://api.example.com/v1"},
		{"http localhost", "http://localhost:3000"},
		{"http 127.0.0.1", "http://127.0.0.1:3000"},
		{"with path", "https://api.example.com/v1/ledger"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(
				WithLedgerURL(tc.url),
				WithAnonymous(),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.url, config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.url, config.ServiceURLs[ServiceTransaction])
		})
	}
}

func TestWithLedgerURL_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{"empty URL", "", "invalid ledger URL"},
		{"no scheme", "api.example.com", "invalid ledger URL"},
		{"no host", "https://", "invalid ledger URL"},
		{"malformed", "://invalid", "invalid ledger URL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewConfig(
				WithLedgerURL(tc.url),
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
	assert.NotSame(t, customClient, config.HTTPClient)
	assert.Equal(t, customClient.Timeout, config.HTTPClient.Timeout)
	assert.NotNil(t, config.HTTPClient.CheckRedirect)
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

func TestWithErrorBodyExposure_Toggle(t *testing.T) {
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
				WithErrorBodyExposure(tc.enabled),
				WithAnonymous(),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.enabled, config.ExposeErrorBody)
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
		Address:      "https://auth.example.com",
		ClientID:     "client-123",
		ClientSecret: "secret-456",
	}

	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(accessManager))
	require.NoError(t, err)

	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "https://auth.example.com", config.AccessManager.Address)
	assert.Equal(t, "client-123", config.AccessManager.ClientID)
	assert.Equal(t, "secret-456", config.AccessManager.ClientSecret)
}

func TestWithAccessManagerPreservesPriorAllowInsecureHTTP(t *testing.T) {
	config, err := NewConfig(
		WithEnvironment(EnvironmentDevelopment),
		WithAllowInsecureAccessManagerHTTP(true),
		WithAccessManager(auth.AccessManager{
			Address:      "http://auth.internal.example.com",
			ClientID:     "client-123",
			ClientSecret: "secret-456",
		}),
	)
	require.NoError(t, err)

	assert.True(t, config.AccessManager.Enabled)
	assert.True(t, config.AccessManager.AllowInsecureHTTP)
	assert.Equal(t, "http://auth.internal.example.com", config.AccessManager.Address)
}

func TestWithAllowInsecureAccessManagerHTTPFalseAppliedLastDisablesPriorOptIn(t *testing.T) {
	_, err := NewConfig(
		WithEnvironment(EnvironmentDevelopment),
		WithAllowInsecureAccessManagerHTTP(true),
		WithAccessManager(auth.AccessManager{
			Address:      "http://auth.internal.example.com",
			ClientID:     "client-123",
			ClientSecret: "secret-456",
		}),
		WithAllowInsecureAccessManagerHTTP(false),
	)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "invalid plugin auth address")
	assert.Contains(t, err.Error(), "validationReason=insecure_scheme")
}

func TestValidateConfig_MissingAuthAddress(t *testing.T) {
	_, err := NewConfig(WithAccessManager(auth.AccessManager{
		Enabled: true,
		Address: "",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin auth address is required")
}

func TestValidateConfig_AccessManagerAddressValidatedBeforeTokenFetch(t *testing.T) {
	_, err := NewConfig(WithEnvironment(EnvironmentDevelopment), WithAccessManager(auth.AccessManager{
		Enabled:      true,
		Address:      "http://auth.internal.example.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret-value",
	}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid plugin auth address")
	assert.Contains(t, err.Error(), "operation=access_manager.token_request")
	assert.Contains(t, err.Error(), "phase=token_fetch")
	assert.Contains(t, err.Error(), "endpoint=http://auth.internal.example.com/v1/login/oauth/access_token")
	assert.Contains(t, err.Error(), "httpRequestSent=false")
	assert.Contains(t, err.Error(), "localValidationFailed=true")
	assert.Contains(t, err.Error(), "validationReason=insecure_scheme")
	assert.NotContains(t, err.Error(), "super-secret-value")
}

func TestValidateConfig_RejectsInsecureAccessManagerHTTPInProduction(t *testing.T) {
	_, err := NewConfig(
		WithEnvironment(EnvironmentProduction),
		WithAccessManager(auth.AccessManager{
			Address:      "http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000",
			ClientID:     "client-id",
			ClientSecret: "super-secret-value",
		}),
		WithAllowInsecureAccessManagerHTTP(true),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin auth insecure HTTP is not allowed in production")
	assert.NotContains(t, err.Error(), "super-secret-value")
}

func TestValidateConfig_AuthCheckSkipped(t *testing.T) {
	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(auth.AccessManager{
		Enabled: true,
		Address: "",
	}))
	require.NoError(t, err)
	assert.True(t, config.AccessManager.Enabled)
}

func TestFromEnvironment_TwoPlanes(t *testing.T) {
	t.Setenv("MIDAZ_LEDGER_URL", "https://ledger.example.com/v1")
	t.Setenv("MIDAZ_TRACER_URL", "https://tracer.example.com/v1")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	// Ledger plane feeds both onboarding and transaction internal routes.
	assert.Equal(t, "https://ledger.example.com/v1", config.LedgerURL)
	assert.Equal(t, "https://ledger.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://ledger.example.com/v1", config.ServiceURLs[ServiceTransaction])

	// Tracer plane is its own explicit URL.
	assert.Equal(t, "https://tracer.example.com/v1", config.TracerURL)
	assert.Equal(t, "https://tracer.example.com/v1", config.ServiceURLs[ServiceTracer])

	// No X-API-Key configured: tracer shares the ledger Bearer (empty key).
	assert.Empty(t, config.TracerAPIKey)
}

func TestFromEnvironment_TracerAPIKey(t *testing.T) {
	t.Setenv("MIDAZ_LEDGER_URL", "https://ledger.example.com/v1")
	t.Setenv("MIDAZ_TRACER_URL", "https://tracer.example.com/v1")
	t.Setenv("MIDAZ_TRACER_API_KEY", "trk-secret")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	assert.Equal(t, "trk-secret", config.TracerAPIKey)
}

func TestWithTracerURL(t *testing.T) {
	cfg := DefaultConfig()
	require.NoError(t, WithTracerURL("https://tracer.example.com/v1")(cfg))

	assert.Equal(t, "https://tracer.example.com/v1", cfg.TracerURL)
	assert.Equal(t, "https://tracer.example.com/v1", cfg.ServiceURLs[ServiceTracer])
}

func TestWithTracerAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	require.NoError(t, WithTracerAPIKey("trk-secret")(cfg))

	assert.Equal(t, "trk-secret", cfg.TracerAPIKey)
}

func TestWithBaseURL_FansOutToBothPlanes(t *testing.T) {
	cfg, err := NewConfig(WithBaseURL("https://api.example.com"), WithAnonymous())
	require.NoError(t, err)

	// A single base URL seeds both the ledger and tracer planes under /v1.
	assert.Equal(t, "https://api.example.com/v1", cfg.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://api.example.com/v1", cfg.ServiceURLs[ServiceTransaction])
	assert.Equal(t, "https://api.example.com/v1", cfg.ServiceURLs[ServiceTracer])
}

func TestFromEnvironment_AllVariables(t *testing.T) {
	t.Setenv("MIDAZ_ENVIRONMENT", "development")
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "env-client-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "env-client-secret")
	t.Setenv("MIDAZ_LEDGER_URL", "https://env.example.com/v1")
	t.Setenv("MIDAZ_TIMEOUT", "45")
	t.Setenv("MIDAZ_DEBUG", "true")
	t.Setenv("MIDAZ_MAX_RETRIES", "7")
	t.Setenv("MIDAZ_IDEMPOTENCY", "false")
	t.Setenv("MIDAZ_ERROR_EXPOSE_BODY", "false")

	config, err := NewConfig(FromEnvironment())
	require.NoError(t, err)

	assert.Equal(t, EnvironmentDevelopment, config.Environment)
	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "https://auth.example.com", config.AccessManager.Address)
	assert.Equal(t, "env-client-id", config.AccessManager.ClientID)
	assert.Equal(t, "env-client-secret", config.AccessManager.ClientSecret)
	assert.Equal(t, "https://env.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://env.example.com/v1", config.ServiceURLs[ServiceTransaction])
	assert.Equal(t, 45*time.Second, config.Timeout)
	assert.True(t, config.Debug)
	assert.Equal(t, 7, config.MaxRetries)
	assert.False(t, config.EnableIdempotency)
	assert.False(t, config.ExposeErrorBody)
}

func TestFromEnvironment_PartialVariables(t *testing.T) {
	envVars := []string{
		"MIDAZ_ENVIRONMENT",
		"PLUGIN_AUTH_ENABLED",
		"PLUGIN_AUTH_ADDRESS",
		"MIDAZ_CLIENT_ID",
		"MIDAZ_CLIENT_SECRET",
		"MIDAZ_BASE_URL",
		"MIDAZ_LEDGER_URL",
		"MIDAZ_TIMEOUT",
		"MIDAZ_DEBUG",
		"MIDAZ_MAX_RETRIES",
		"MIDAZ_IDEMPOTENCY",
		"MIDAZ_ERROR_EXPOSE_BODY",
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
	assert.False(t, config.ExposeErrorBody)
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

func TestFromEnvironment_InvalidLedgerURL(t *testing.T) {
	t.Setenv("MIDAZ_LEDGER_URL", "not-a-valid-url")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ledger URL")
}

func TestFromEnvironment_InvalidBaseURL(t *testing.T) {
	t.Setenv("MIDAZ_BASE_URL", "://malformed")

	_, err := NewConfig(FromEnvironment())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base URL")
}

func TestFromEnvironment_BaseURLOverriddenBySpecific(t *testing.T) {
	// MIDAZ_LEDGER_URL overrides the base-URL-derived ledger plane for both
	// onboarding and transaction routes.
	t.Setenv("MIDAZ_BASE_URL", "https://base.example.com")
	t.Setenv("MIDAZ_LEDGER_URL", "https://specific.example.com/v1")

	config, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.NoError(t, err)

	assert.Equal(t, "https://specific.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://specific.example.com/v1", config.ServiceURLs[ServiceTransaction])
}

func TestFromEnvironment_PluginAuthDisabled(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "false")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.example.com")
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
	assert.Equal(t, "https://auth.example.com", config.AccessManager.Address)
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
	assert.False(t, config.ExposeErrorBody)
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
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.local.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "local-client")
	t.Setenv("MIDAZ_CLIENT_SECRET", "local-secret")

	config, err := NewLocalConfig()
	require.NoError(t, err)

	assert.True(t, config.AccessManager.Enabled)
	assert.Equal(t, "https://auth.local.example.com", config.AccessManager.Address)
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
		WithLedgerURL("https://api.example.com/v1"),
		WithAnonymous(),
	)
	require.NoError(t, err)

	baseURLs := config.GetBaseURLs()

	assert.Equal(t, "https://api.example.com/v1", baseURLs["onboarding"])
	assert.Equal(t, "https://api.example.com/v1", baseURLs["transaction"])
}

func TestGetHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}
	config, err := NewConfig(
		WithHTTPClient(customClient),
		WithAnonymous(),
	)
	require.NoError(t, err)

	client := config.GetHTTPClient()
	assert.NotSame(t, customClient, client)
	assert.Equal(t, customClient.Timeout, client.Timeout)
	assert.NotNil(t, client.CheckRedirect)
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
		Enabled:           true,
		Address:           "https://auth.example.com",
		ClientID:          "test-client",
		ClientSecret:      "test-secret",
		AllowInsecureHTTP: true,
	}))
	require.NoError(t, err)

	pluginAuth := config.GetPluginAuth()

	assert.True(t, pluginAuth.Enabled)
	assert.Equal(t, "https://auth.example.com", pluginAuth.Address)
	assert.Equal(t, "test-client", pluginAuth.ClientID)
	assert.Equal(t, "test-secret", pluginAuth.ClientSecret)
	assert.True(t, pluginAuth.AllowInsecureHTTP)
}

func TestGetPluginAuth_ReturnsCopy(t *testing.T) {
	config, err := NewConfig(disableAuthCheck(t), WithAccessManager(auth.AccessManager{
		Enabled:      true,
		Address:      "https://auth.example.com",
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
		WithLedgerURL("https://specific.example.com/v1"),
		WithAnonymous(),
	)
	require.NoError(t, err)

	// WithLedgerURL applied after WithBaseURL wins for both onboarding and
	// transaction routes.
	assert.Equal(t, "https://specific.example.com/v1", config.ServiceURLs[ServiceOnboarding])
	assert.Equal(t, "https://specific.example.com/v1", config.ServiceURLs[ServiceTransaction])
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

// TestParseURLWithInsecureHTTP exercises the per-call insecure-HTTP
// opt-in. The strict default path is covered by TestParseURL_Valid /
// TestParseURL_Invalid; this matrix asserts:
//
//   - allow=true relaxes ONLY the http-non-localhost guard.
//   - scheme allowlist, userinfo rejection, and missing-scheme/host
//     rejection are NOT relaxed by the flag.
//   - allow=false explicitly still rejects.
//   - HTTPS URLs are unaffected.
func TestParseURLWithInsecureHTTP(t *testing.T) {
	tests := []struct {
		name              string
		url               string
		allowInsecureHTTP bool
		errContain        string
	}{
		{
			name:              "AllowHTTPClusterLocalServiceDNS",
			url:               "http://midaz-ledger.midaz-mt.svc.cluster.local:3000",
			allowInsecureHTTP: true,
			errContain:        "",
		},
		{
			name:              "AllowHTTPPrivateRFC1918",
			url:               "http://10.0.0.5:3000",
			allowInsecureHTTP: true,
			errContain:        "",
		},
		{
			name:              "AllowHTTPLocalhostRegression",
			url:               "http://localhost:3000",
			allowInsecureHTTP: true,
			errContain:        "",
		},
		{
			name:              "AllowHTTPPublicHostInsideClusterMesh",
			url:               "http://api.internal.example.com",
			allowInsecureHTTP: true,
			errContain:        "",
		},
		{
			name:              "RejectFTPSchemeEvenWithAllow",
			url:               "ftp://midaz-ledger.midaz-mt.svc.cluster.local:3000",
			allowInsecureHTTP: true,
			errContain:        "URL scheme must be http or https",
		},
		{
			name:              "RejectUserinfoEvenWithAllow",
			url:               "http://attacker@midaz-ledger.midaz-mt.svc.cluster.local:3000",
			allowInsecureHTTP: true,
			errContain:        "URL must not include user information",
		},
		{
			name:              "RejectMissingHostEvenWithAllow",
			url:               "http://",
			allowInsecureHTTP: true,
			errContain:        "URL must include scheme and host",
		},
		{
			name:              "AllowFalseStillRejectsHTTPNonLocalhost",
			url:               "http://midaz-ledger.midaz-mt.svc.cluster.local:3000",
			allowInsecureHTTP: false,
			errContain:        "insecure HTTP is only allowed for localhost targets",
		},
		{
			name:              "AllowHTTPSPublicWithoutFlag",
			url:               "https://api.example.com",
			allowInsecureHTTP: false,
			errContain:        "",
		},
		{
			name:              "AllowHTTPSPublicWithFlag",
			url:               "https://api.example.com",
			allowInsecureHTTP: true,
			errContain:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseURLWithInsecureHTTP(tc.url, tc.allowInsecureHTTP)

			if tc.errContain == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContain)
		})
	}
}

// TestWithAllowInsecureHTTP_LedgerURL asserts the user-facing wiring at
// the config layer: applying WithAllowInsecureHTTP(true) BEFORE
// WithLedgerURL permits an http://*.svc.cluster.local target that
// would otherwise be rejected by parseURL at option-application time.
// This reproduces the plugin-br-bank-transfer MT-staging failure that
// drove the hotfix.
func TestWithAllowInsecureHTTP_LedgerURL(t *testing.T) {
	const clusterURL = "http://midaz-ledger.midaz-mt.svc.cluster.local:3000"

	t.Run("OptInAcceptsClusterLocalHTTP", func(t *testing.T) {
		cfg, err := NewConfig(
			disableAuthCheck(t),
			WithEnvironment(EnvironmentDevelopment),
			WithAllowInsecureHTTP(true),
			WithLedgerURL(clusterURL),
		)
		require.NoError(t, err)
		assert.True(t, cfg.AllowInsecureHTTP)
		assert.True(t, cfg.GetAllowInsecureHTTP())
		assert.Equal(t, clusterURL, cfg.ServiceURLs[ServiceOnboarding])
		assert.Equal(t, clusterURL, cfg.ServiceURLs[ServiceTransaction])
	})

	t.Run("DefaultRejectsClusterLocalHTTP", func(t *testing.T) {
		_, err := NewConfig(
			disableAuthCheck(t),
			WithEnvironment(EnvironmentDevelopment),
			WithLedgerURL(clusterURL),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insecure HTTP is only allowed for localhost targets")
	})

	t.Run("ExplicitFalseRejectsClusterLocalHTTP", func(t *testing.T) {
		_, err := NewConfig(
			disableAuthCheck(t),
			WithEnvironment(EnvironmentDevelopment),
			WithAllowInsecureHTTP(false),
			WithLedgerURL(clusterURL),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insecure HTTP is only allowed for localhost targets")
	})

	t.Run("OptInAfterURLDoesNotRetroactivelyAllow", func(t *testing.T) {
		// Documents the ordering rule from WithAllowInsecureHTTP's
		// godoc. The URL setter validates against c.AllowInsecureHTTP
		// at the moment it runs, so flipping the flag afterwards
		// cannot rescue a URL that was already rejected.
		_, err := NewConfig(
			disableAuthCheck(t),
			WithEnvironment(EnvironmentDevelopment),
			WithLedgerURL(clusterURL),
			WithAllowInsecureHTTP(true),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insecure HTTP is only allowed for localhost targets")
	})
}

// TestWithAllowInsecureHTTP_TracerURL mirrors the LedgerURL coverage on
// the Tracer URL path so both plane setters are pinned.
func TestWithAllowInsecureHTTP_TracerURL(t *testing.T) {
	const tracerURL = "http://midaz-tracer.midaz-mt.svc.cluster.local:4020"

	cfg, err := NewConfig(
		disableAuthCheck(t),
		WithEnvironment(EnvironmentDevelopment),
		WithAllowInsecureHTTP(true),
		WithLedgerURL("http://midaz-ledger.midaz-mt.svc.cluster.local:3000"),
		WithTracerURL(tracerURL),
	)
	require.NoError(t, err)
	assert.True(t, cfg.AllowInsecureHTTP)
	assert.Equal(t, tracerURL, cfg.ServiceURLs[ServiceTracer])
}

// TestWithAllowInsecureHTTP_BaseURL covers WithBaseURL, which fans out
// to both planes via buildLedgerServiceURL / buildTracerServiceURL —
// each of those internally parses the same base URL.
func TestWithAllowInsecureHTTP_BaseURL(t *testing.T) {
	const baseURL = "http://midaz-api.midaz-mt.svc.cluster.local:3000"

	cfg, err := NewConfig(
		disableAuthCheck(t),
		WithEnvironment(EnvironmentDevelopment),
		WithAllowInsecureHTTP(true),
		WithBaseURL(baseURL),
	)
	require.NoError(t, err)
	assert.True(t, cfg.AllowInsecureHTTP)
	assert.NotEmpty(t, cfg.ServiceURLs[ServiceOnboarding])
	assert.NotEmpty(t, cfg.ServiceURLs[ServiceTracer])
}

// TestValidateConfig_RejectsInsecureHTTPInProduction mirrors the
// existing Access Manager production gate. The data-plane flag is also
// forbidden in production so the in-cluster carve-out cannot be flipped
// on by accident for a public deployment.
func TestValidateConfig_RejectsInsecureHTTPInProduction(t *testing.T) {
	_, err := NewConfig(
		disableAuthCheck(t),
		WithEnvironment(EnvironmentProduction),
		WithAllowInsecureHTTP(true),
		// Use HTTPS here so the URL setter itself accepts the value;
		// the validation gate fires later in validateConfig.
		WithLedgerURL("https://api.midaz.io"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insecure HTTP is not allowed in production")
}

// TestFromEnvironment_AllowInsecureHTTP pins the env-loading path: the
// MIDAZ_ALLOW_INSECURE_HTTP var must be read BEFORE the URL env vars so
// in-cluster http:// targets are accepted automatically.
func TestFromEnvironment_AllowInsecureHTTP(t *testing.T) {
	t.Run("EnabledAcceptsClusterLocalLedger", func(t *testing.T) {
		t.Setenv("MIDAZ_ALLOW_INSECURE_HTTP", "true")
		t.Setenv("MIDAZ_LEDGER_URL", "http://midaz-ledger.midaz-mt.svc.cluster.local:3000")

		cfg, err := NewConfig(disableAuthCheck(t), FromEnvironment())
		require.NoError(t, err)
		assert.True(t, cfg.AllowInsecureHTTP)
		assert.Equal(t, "http://midaz-ledger.midaz-mt.svc.cluster.local:3000", cfg.ServiceURLs[ServiceOnboarding])
	})

	t.Run("UnsetKeepsDefaultStrict", func(t *testing.T) {
		t.Setenv("MIDAZ_LEDGER_URL", "http://midaz-ledger.midaz-mt.svc.cluster.local:3000")

		_, err := NewConfig(disableAuthCheck(t), FromEnvironment())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insecure HTTP is only allowed for localhost targets")
	})

	t.Run("InvalidBoolValueIsRejected", func(t *testing.T) {
		t.Setenv("MIDAZ_ALLOW_INSECURE_HTTP", "yes")

		_, err := NewConfig(disableAuthCheck(t), FromEnvironment())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid MIDAZ_ALLOW_INSECURE_HTTP")
	})
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
		env               Environment
		expectedLedgerURL string
	}{
		{
			env:               EnvironmentLocal,
			expectedLedgerURL: "http://localhost:3002/v1",
		},
		{
			env:               EnvironmentDevelopment,
			expectedLedgerURL: "https://api.dev.midaz.io/v1",
		},
		{
			env:               EnvironmentProduction,
			expectedLedgerURL: "https://api.midaz.io/v1",
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
			assert.Equal(t, tc.expectedLedgerURL, config.ServiceURLs[ServiceOnboarding])
			assert.Equal(t, tc.expectedLedgerURL, config.ServiceURLs[ServiceTransaction])
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
			envAddress:     "https://auth.example.com",
			envClientID:    "client-123",
			envSecret:      "secret-456",
			expectedEnable: true,
		},
		{
			name:           "disabled",
			envEnabled:     "false",
			envAddress:     "https://auth.example.com",
			envClientID:    "client-123",
			envSecret:      "secret-456",
			expectedEnable: false,
		},
		{
			name:           "empty enabled",
			envEnabled:     "",
			envAddress:     "https://auth.example.com",
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
