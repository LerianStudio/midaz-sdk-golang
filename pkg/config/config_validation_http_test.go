package config

import (
	"net/http"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestNewDefaultHTTPClientRejectsSensitiveCrossOriginRedirect(t *testing.T) {
	client := NewDefaultHTTPClient(time.Second)

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("X-Idempotency", "raw-idempotency-key")

	next, err := http.NewRequest(http.MethodGet, "https://evil.example.net/v1/accounts", nil)
	require.NoError(t, err)

	err = client.CheckRedirect(next, []*http.Request{previous})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authenticated redirect")
}

func TestWithHTTPClientInstallsRedirectGuardWhenMissing(t *testing.T) {
	callerClient := &http.Client{}
	config, err := NewConfig(WithAnonymous(), WithHTTPClient(callerClient))
	require.NoError(t, err)
	require.NotNil(t, config.HTTPClient.CheckRedirect)
	require.Nil(t, callerClient.CheckRedirect, "caller-owned client must not be mutated in place")

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("X-API-Key", "raw-api-key")

	next, err := http.NewRequest(http.MethodGet, "https://evil.example.net/v1/accounts", nil)
	require.NoError(t, err)

	err = config.HTTPClient.CheckRedirect(next, []*http.Request{previous})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authenticated redirect")
}

func TestWithHTTPClientComposesExplicitRedirectPolicy(t *testing.T) {
	var called bool
	callerClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			called = true
			return assert.AnError
		},
	}
	config, err := NewConfig(WithAnonymous(), WithHTTPClient(callerClient))
	require.NoError(t, err)

	require.NotSame(t, callerClient, config.HTTPClient)
	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	next, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts?page=2", nil)
	require.NoError(t, err)

	require.ErrorIs(t, config.HTTPClient.CheckRedirect(next, []*http.Request{previous}), assert.AnError)
	assert.True(t, called)
}

func TestWithHTTPClientBlocksCrossOriginBeforeExplicitRedirectPolicy(t *testing.T) {
	var called bool
	callerClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			called = true
			return nil
		},
	}
	config, err := NewConfig(WithAnonymous(), WithHTTPClient(callerClient))
	require.NoError(t, err)

	previous, err := http.NewRequest(http.MethodGet, "https://api.example.com/v1/accounts", nil)
	require.NoError(t, err)
	previous.Header.Set("X-API-Key", "raw-api-key")
	next, err := http.NewRequest(http.MethodGet, "https://evil.example.net/v1/accounts", nil)
	require.NoError(t, err)

	err = config.HTTPClient.CheckRedirect(next, []*http.Request{previous})
	require.ErrorIs(t, err, security.ErrAuthenticatedRedirect)
	assert.False(t, called, "SDK guard must reject before caller redirect policy runs")
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
