package config

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestDefaultValues(t *testing.T) {
	// Test that the default constants have the expected values
	if DefaultTimeout != 60 {
		t.Errorf("Expected DefaultTimeout to be 60, got %d", DefaultTimeout)
	}

	if DefaultLocalBaseURL != "http://localhost" {
		t.Errorf("Expected DefaultLocalBaseURL to be http://localhost, got %s", DefaultLocalBaseURL)
	}

	if DefaultMaxRetries != 3 {
		t.Errorf("Expected DefaultMaxRetries to be 3, got %d", DefaultMaxRetries)
	}

	if DefaultRetryWaitMin != 1*time.Second {
		t.Errorf("Expected DefaultRetryWaitMin to be 1s, got %s", DefaultRetryWaitMin)
	}

	if DefaultRetryWaitMax != 30*time.Second {
		t.Errorf("Expected DefaultRetryWaitMax to be 30s, got %s", DefaultRetryWaitMax)
	}
}

func TestNewConfig(t *testing.T) {
	// Test creating a new config with default values
	config, err := NewConfig(WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Check that default values are set correctly
	onboardingURL := config.ServiceURLs[ServiceOnboarding]
	transactionURL := config.ServiceURLs[ServiceTransaction]

	expectedOnboardingURL := "http://localhost:3000/v1"
	expectedTransactionURL := "http://localhost:3001/v1"

	if onboardingURL != expectedOnboardingURL {
		t.Errorf("Expected onboarding URL to be %s, got %s", expectedOnboardingURL, onboardingURL)
	}

	if transactionURL != expectedTransactionURL {
		t.Errorf("Expected transaction URL to be %s, got %s", expectedTransactionURL, transactionURL)
	}

	if config.Timeout != DefaultTimeout*time.Second {
		t.Errorf("Expected Timeout to be %s, got %s", DefaultTimeout*time.Second, config.Timeout)
	}

	if config.UserAgent != DefaultUserAgent {
		t.Errorf("Expected UserAgent to be %s, got %s", DefaultUserAgent, config.UserAgent)
	}

	if config.MaxRetries != DefaultMaxRetries {
		t.Errorf("Expected MaxRetries to be %d, got %d", DefaultMaxRetries, config.MaxRetries)
	}

	if config.RetryWaitMin != DefaultRetryWaitMin {
		t.Errorf("Expected RetryWaitMin to be %s, got %s", DefaultRetryWaitMin, config.RetryWaitMin)
	}

	if config.RetryWaitMax != DefaultRetryWaitMax {
		t.Errorf("Expected RetryWaitMax to be %s, got %s", DefaultRetryWaitMax, config.RetryWaitMax)
	}

	if config.Debug {
		t.Errorf("Expected Debug to be false, got %t", config.Debug)
	}
}

func TestWithOnboardingURL(t *testing.T) {
	// Test setting a custom onboarding URL
	customURL := "https://api.example.com/onboarding"
	config, err := NewConfig(WithOnboardingURL(customURL), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.ServiceURLs[ServiceOnboarding] != customURL {
		t.Errorf("Expected onboarding URL to be %s, got %s", customURL, config.ServiceURLs[ServiceOnboarding])
	}
}

func TestWithTransactionURL(t *testing.T) {
	// Test setting a custom transaction URL
	customURL := "https://api.example.com/transaction"
	config, err := NewConfig(WithTransactionURL(customURL), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.ServiceURLs[ServiceTransaction] != customURL {
		t.Errorf("Expected transaction URL to be %s, got %s", customURL, config.ServiceURLs[ServiceTransaction])
	}
}

func TestWithBaseURL(t *testing.T) {
	// Test setting a base URL that affects both onboarding and transaction URLs
	baseURL := "https://api.example.com"
	config, err := NewConfig(WithBaseURL(baseURL), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Setting Environment to local for this test to make URL generation predictable
	config.Environment = EnvironmentLocal

	// Set the base URL again to regenerate the URLs
	err = WithBaseURL(baseURL)(config)
	if err != nil {
		t.Fatalf("Failed to set base URL: %v", err)
	}

	expectedOnboardingURL := baseURL + ":" + DefaultOnboardingPort + DefaultLocalOnboardingPath
	expectedTransactionURL := baseURL + ":" + DefaultTransactionPort + DefaultLocalTransactionPath

	if config.ServiceURLs[ServiceOnboarding] != expectedOnboardingURL {
		t.Errorf("Expected onboarding URL to be %s, got %s", expectedOnboardingURL, config.ServiceURLs[ServiceOnboarding])
	}

	if config.ServiceURLs[ServiceTransaction] != expectedTransactionURL {
		t.Errorf("Expected transaction URL to be %s, got %s", expectedTransactionURL, config.ServiceURLs[ServiceTransaction])
	}
}

func TestWithEnvironment(t *testing.T) {
	// Test setting different environments
	environments := []struct {
		env      Environment
		expected Environment
	}{
		{EnvironmentLocal, EnvironmentLocal},
		{EnvironmentDevelopment, EnvironmentDevelopment},
		{EnvironmentProduction, EnvironmentProduction},
	}

	for _, tc := range environments {
		config, err := NewConfig(WithEnvironment(tc.env), WithAuthToken("test-token"))
		if err != nil {
			t.Fatalf("Failed to create config with environment %s: %v", tc.env, err)
		}

		if config.Environment != tc.expected {
			t.Errorf("Expected environment to be %s, got %s", tc.expected, config.Environment)
		}
	}
}

func TestWithAuthToken(t *testing.T) {
	// Test setting an auth token
	token := "test-auth-token"
	config, err := NewConfig(WithAuthToken(token))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.AuthToken != token {
		t.Errorf("Expected AuthToken to be %s, got %s", token, config.AuthToken)
	}
}

func TestWithHTTPClient(t *testing.T) {
	// Test setting a custom HTTP client
	httpClient := &http.Client{
		Timeout: 120 * time.Second,
	}

	config, err := NewConfig(WithHTTPClient(httpClient), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.HTTPClient != httpClient {
		t.Errorf("Expected HTTPClient to be the custom client")
	}
}

func TestWithTimeout(t *testing.T) {
	// Test setting a custom timeout
	timeout := 30 * time.Second
	config, err := NewConfig(WithTimeout(timeout), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.Timeout != timeout {
		t.Errorf("Expected Timeout to be %s, got %s", timeout, config.Timeout)
	}
}

func TestWithUserAgent(t *testing.T) {
	// Test setting a custom user agent
	userAgent := "custom-user-agent/1.0"
	config, err := NewConfig(WithUserAgent(userAgent), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.UserAgent != userAgent {
		t.Errorf("Expected UserAgent to be %s, got %s", userAgent, config.UserAgent)
	}
}

func TestWithRetryConfig(t *testing.T) {
	// Test setting a custom retry config
	maxRetries := 5
	minWait := 2 * time.Second
	maxWait := 60 * time.Second

	config, err := NewConfig(WithRetryConfig(maxRetries, minWait, maxWait), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.MaxRetries != maxRetries {
		t.Errorf("Expected MaxRetries to be %d, got %d", maxRetries, config.MaxRetries)
	}

	if config.RetryWaitMin != minWait {
		t.Errorf("Expected RetryWaitMin to be %s, got %s", minWait, config.RetryWaitMin)
	}

	if config.RetryWaitMax != maxWait {
		t.Errorf("Expected RetryWaitMax to be %s, got %s", maxWait, config.RetryWaitMax)
	}
}

func TestWithRetries(t *testing.T) {
	// Test enabling and disabling retries
	config, err := NewConfig(WithRetries(false), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.EnableRetries {
		t.Errorf("Expected EnableRetries to be false, got true")
	}

	config, err = NewConfig(WithRetries(true), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if !config.EnableRetries {
		t.Errorf("Expected EnableRetries to be true, got false")
	}
}

func TestWithDebug(t *testing.T) {
	// Test enabling debug mode
	config, err := NewConfig(WithDebug(true), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if !config.Debug {
		t.Errorf("Expected Debug to be true, got false")
	}

	// Test disabling debug mode
	config, err = NewConfig(WithDebug(false), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.Debug {
		t.Errorf("Expected Debug to be false, got true")
	}
}

func TestWithIdempotency(t *testing.T) {
	// Test enabling and disabling idempotency
	config, err := NewConfig(WithIdempotency(false), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if config.EnableIdempotency {
		t.Errorf("Expected EnableIdempotency to be false, got true")
	}

	config, err = NewConfig(WithIdempotency(true), WithAuthToken("test-token"))
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	if !config.EnableIdempotency {
		t.Errorf("Expected EnableIdempotency to be true, got false")
	}
}

func TestNewLocalConfig(t *testing.T) {
	// Test creating a local configuration
	token := "test-local-token"
	config, err := NewLocalConfig(token)
	if err != nil {
		t.Fatalf("Failed to create local config: %v", err)
	}

	// Check that local config values are set correctly
	if config.Environment != EnvironmentLocal {
		t.Errorf("Expected Environment to be local, got %s", config.Environment)
	}

	if config.AuthToken != token {
		t.Errorf("Expected AuthToken to be %s, got %s", token, config.AuthToken)
	}
}

func TestFromEnvironment(t *testing.T) {
	// Save the original environment
	origEnv := make(map[string]string)
	for _, key := range []string{
		"MIDAZ_ENVIRONMENT",
		"MIDAZ_AUTH_TOKEN",
		"MIDAZ_ONBOARDING_URL",
		"MIDAZ_TRANSACTION_URL",
		"MIDAZ_BASE_URL",
		"MIDAZ_TIMEOUT",
		"MIDAZ_DEBUG",
		"MIDAZ_MAX_RETRIES",
		"MIDAZ_IDEMPOTENCY",
	} {
		origEnv[key] = os.Getenv(key)
	}

	// Restore the original environment when the test completes
	defer func() {
		for key, value := range origEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set up the test environment
	os.Setenv("MIDAZ_ENVIRONMENT", "development")
	os.Setenv("MIDAZ_AUTH_TOKEN", "env-token")
	os.Setenv("MIDAZ_ONBOARDING_URL", "https://env.example.com/onboarding")
	os.Setenv("MIDAZ_TIMEOUT", "45")
	os.Setenv("MIDAZ_DEBUG", "true")
	os.Setenv("MIDAZ_MAX_RETRIES", "4")
	os.Setenv("MIDAZ_IDEMPOTENCY", "false")

	// Create a config from the environment
	config, err := NewConfig(FromEnvironment())
	if err != nil {
		t.Fatalf("Failed to create config from environment: %v", err)
	}

	// Check that environment values were applied
	if config.Environment != EnvironmentDevelopment {
		t.Errorf("Expected Environment to be development, got %s", config.Environment)
	}

	if config.AuthToken != "env-token" {
		t.Errorf("Expected AuthToken to be env-token, got %s", config.AuthToken)
	}

	if config.ServiceURLs[ServiceOnboarding] != "https://env.example.com/onboarding" {
		t.Errorf("Expected onboarding URL to be https://env.example.com/onboarding, got %s", config.ServiceURLs[ServiceOnboarding])
	}

	if config.Timeout != 45*time.Second {
		t.Errorf("Expected Timeout to be 45s, got %s", config.Timeout)
	}

	if !config.Debug {
		t.Errorf("Expected Debug to be true, got false")
	}

	if config.MaxRetries != 4 {
		t.Errorf("Expected MaxRetries to be 4, got %d", config.MaxRetries)
	}

	if config.EnableIdempotency {
		t.Errorf("Expected EnableIdempotency to be false, got true")
	}
}

func TestMultipleOptions(t *testing.T) {
	// Test applying multiple options at once
	config, err := NewConfig(
		WithOnboardingURL("https://api.example.com/onboarding"),
		WithTransactionURL("https://api.example.com/transaction"),
		WithAuthToken("test-token"),
		WithTimeout(30*time.Second),
		WithUserAgent("custom-agent/1.0"),
		WithRetryConfig(5, 2*time.Second, 60*time.Second),
		WithDebug(true),
		WithIdempotency(false),
	)
	if err != nil {
		t.Fatalf("Failed to create config with multiple options: %v", err)
	}

	// Check that all options were applied correctly
	if config.ServiceURLs[ServiceOnboarding] != "https://api.example.com/onboarding" {
		t.Errorf("Expected onboarding URL to be https://api.example.com/onboarding, got %s", config.ServiceURLs[ServiceOnboarding])
	}

	if config.ServiceURLs[ServiceTransaction] != "https://api.example.com/transaction" {
		t.Errorf("Expected transaction URL to be https://api.example.com/transaction, got %s", config.ServiceURLs[ServiceTransaction])
	}

	if config.AuthToken != "test-token" {
		t.Errorf("Expected AuthToken to be test-token, got %s", config.AuthToken)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected Timeout to be 30s, got %s", config.Timeout)
	}

	if config.UserAgent != "custom-agent/1.0" {
		t.Errorf("Expected UserAgent to be custom-agent/1.0, got %s", config.UserAgent)
	}

	if config.MaxRetries != 5 {
		t.Errorf("Expected MaxRetries to be 5, got %d", config.MaxRetries)
	}

	if config.RetryWaitMin != 2*time.Second {
		t.Errorf("Expected RetryWaitMin to be 2s, got %s", config.RetryWaitMin)
	}

	if config.RetryWaitMax != 60*time.Second {
		t.Errorf("Expected RetryWaitMax to be 60s, got %s", config.RetryWaitMax)
	}

	if !config.Debug {
		t.Errorf("Expected Debug to be true, got false")
	}

	if config.EnableIdempotency {
		t.Errorf("Expected EnableIdempotency to be false, got true")
	}
}

func TestOptionOverrides(t *testing.T) {
	// Test that later options override earlier ones
	config, err := NewConfig(
		WithOnboardingURL("https://api1.example.com"),
		WithOnboardingURL("https://api2.example.com"),
		WithAuthToken("test-token"),
	)
	if err != nil {
		t.Fatalf("Failed to create config with overriding options: %v", err)
	}

	if config.ServiceURLs[ServiceOnboarding] != "https://api2.example.com" {
		t.Errorf("Expected onboarding URL to be https://api2.example.com, got %s", config.ServiceURLs[ServiceOnboarding])
	}

	// Test overriding with base URL
	config, err = NewConfig(
		WithOnboardingURL("https://api.example.com/onboarding"),
		WithBaseURL("https://base.example.com"),
		WithAuthToken("test-token"),
	)
	if err != nil {
		t.Fatalf("Failed to create config with base URL override: %v", err)
	}

	expectedOnboardingURL := "https://base.example.com:3000/v1"
	expectedTransactionURL := "https://base.example.com:3001/v1"

	if config.ServiceURLs[ServiceOnboarding] != expectedOnboardingURL {
		t.Errorf("Expected onboarding URL to be %s, got %s", expectedOnboardingURL, config.ServiceURLs[ServiceOnboarding])
	}

	if config.ServiceURLs[ServiceTransaction] != expectedTransactionURL {
		t.Errorf("Expected transaction URL to be %s, got %s", expectedTransactionURL, config.ServiceURLs[ServiceTransaction])
	}
}

func TestGetterMethods(t *testing.T) {
	// Test the getter methods on Config
	httpClient := &http.Client{Timeout: 60 * time.Second}
	authToken := "test-getter-token"
	onboardingURL := "https://api.example.com/onboarding"
	transactionURL := "https://api.example.com/transaction"

	config, err := NewConfig(
		WithHTTPClient(httpClient),
		WithAuthToken(authToken),
		WithOnboardingURL(onboardingURL),
		WithTransactionURL(transactionURL),
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Test GetHTTPClient
	if config.GetHTTPClient() != httpClient {
		t.Errorf("Expected GetHTTPClient to return the custom client")
	}

	// Test GetAuthToken
	if config.GetAuthToken() != authToken {
		t.Errorf("Expected GetAuthToken to return %s, got %s", authToken, config.GetAuthToken())
	}

	// Test GetBaseURLs
	baseURLs := config.GetBaseURLs()
	if baseURLs["onboarding"] != onboardingURL {
		t.Errorf("Expected baseURLs[\"onboarding\"] to be %s, got %s", onboardingURL, baseURLs["onboarding"])
	}

	if baseURLs["transaction"] != transactionURL {
		t.Errorf("Expected baseURLs[\"transaction\"] to be %s, got %s", transactionURL, baseURLs["transaction"])
	}
}
