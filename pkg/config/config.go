// Package config provides configuration management for the Midaz SDK.
//
// This package centralizes all configuration options for the SDK, including:
// - API endpoints and authentication
// - HTTP client settings like timeouts and retries
// - Feature flags and behavior controls
//
// It uses the functional options pattern for flexible, type-safe configuration.
package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
)

// ServiceType represents a type of service in the Midaz API ecosystem.
type ServiceType string

// Service types constants define the available Midaz services.
const (
	// ServiceOnboarding represents the Onboarding service.
	ServiceOnboarding ServiceType = "onboarding"

	// ServiceTransaction represents the Transaction service.
	ServiceTransaction ServiceType = "transaction"
)

// Environment represents a deployment environment for the Midaz API.
type Environment string

// Environment constants define the available Midaz environments.
const (
	// EnvironmentLocal represents a local development environment.
	EnvironmentLocal Environment = "local"

	// EnvironmentDevelopment represents a development/staging environment.
	EnvironmentDevelopment Environment = "development"

	// EnvironmentProduction represents the production environment.
	EnvironmentProduction Environment = "production"
)

// Default configuration values
const (
	// Default user agent for HTTP requests
	DefaultUserAgent = "midaz-go-sdk/1.0.0"

	// Default timeout for HTTP requests in seconds
	DefaultTimeout = 60

	// Default URLs for each environment
	DefaultLocalBaseURL         = "http://localhost"
	DefaultDevelopmentBaseURL   = "https://api.dev.midaz.io"
	DefaultProductionBaseURL    = "https://api.midaz.io"
	DefaultOnboardingPort       = "3000"
	DefaultTransactionPort      = "3001"
	DefaultLocalOnboardingPath  = "/v1"
	DefaultLocalTransactionPath = "/v1"

	// Default retry configuration
	DefaultMaxRetries   = 3
	DefaultRetryWaitMin = 1 * time.Second
	DefaultRetryWaitMax = 30 * time.Second

	// Default feature flags
	DefaultEnableIdempotency = true
	DefaultEnableRetries     = true
)

// Config holds the configuration for the Midaz SDK.
// It centralizes all settings needed to interact with the Midaz API.
type Config struct {

	// AccessManager configuration for authentication
	AccessManager auth.AccessManager

	// Environment specifies which Midaz environment to connect to.
	// This affects the default URLs used if not explicitly overridden.
	Environment Environment

	// ServiceURLs maps service types to their base URLs.
	// These take precedence over Environment-based URLs.
	ServiceURLs map[ServiceType]string

	// HTTPClient is the HTTP client to use for requests.
	// If nil, a default client will be created with the configured timeout.
	HTTPClient *http.Client

	// Timeout is the timeout for HTTP requests.
	Timeout time.Duration

	// UserAgent is the user agent string sent in HTTP requests.
	UserAgent string

	// Retry configuration for failed requests
	MaxRetries    int
	RetryWaitMin  time.Duration
	RetryWaitMax  time.Duration
	EnableRetries bool

	// Debug enables verbose logging of requests and responses.
	Debug bool

	// ObservabilityProvider for tracing, metrics, and logging.
	ObservabilityProvider observability.Provider

	// EnableIdempotency enables automatic generation of idempotency keys.
	EnableIdempotency bool
}

// Option is a function that configures a Config.
// It's the core of the functional options pattern used throughout the SDK.
type Option func(*Config) error

// WithEnvironment sets the environment for the Config.
// This determines the default URLs used for services if not explicitly overridden.
//
// Parameters:
//   - env: The environment to use (EnvironmentLocal, EnvironmentDevelopment, EnvironmentProduction)
//
// Returns:
//   - Option: A function that sets the environment on a Config
func WithEnvironment(env Environment) Option {
	return func(c *Config) error {
		c.Environment = env
		return nil
	}
}

// WithOnboardingURL sets the base URL for the Onboarding API.
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - url: The base URL for the Onboarding API
//
// Returns:
//   - Option: A function that sets the Onboarding URL on a Config
//   - May return an error if the URL is invalid
func WithOnboardingURL(url string) Option {
	return func(c *Config) error {
		// Validate URL
		if _, err := parseURL(url); err != nil {
			return fmt.Errorf("invalid onboarding URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}
		c.ServiceURLs[ServiceOnboarding] = url

		return nil
	}
}

// WithTransactionURL sets the base URL for the Transaction API.
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - url: The base URL for the Transaction API
//
// Returns:
//   - Option: A function that sets the Transaction URL on a Config
//   - May return an error if the URL is invalid
func WithTransactionURL(url string) Option {
	return func(c *Config) error {
		// Validate URL
		if _, err := parseURL(url); err != nil {
			return fmt.Errorf("invalid transaction URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}
		c.ServiceURLs[ServiceTransaction] = url
		return nil
	}
}

// WithBaseURL sets a common base URL that will be used for all services.
// Service-specific ports and paths will be automatically added.
// This is useful for connecting to custom deployments.
//
// Parameters:
//   - baseURL: The base URL (e.g., "http://example.com")
//
// Returns:
//   - Option: A function that sets all service URLs derived from the base URL
//   - May return an error if the URL is invalid
func WithBaseURL(baseURL string) Option {
	return func(c *Config) error {
		// Validate the base URL
		if _, err := parseURL(baseURL); err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}

		// Remove trailing slash if present
		baseURL = strings.TrimSuffix(baseURL, "/")

		// Initialize the map if needed
		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		// Set the URLs for each service
		if c.Environment == EnvironmentLocal {
			c.ServiceURLs[ServiceOnboarding] = fmt.Sprintf("%s:%s%s", baseURL, DefaultOnboardingPort, DefaultLocalOnboardingPath)
			c.ServiceURLs[ServiceTransaction] = fmt.Sprintf("%s:%s%s", baseURL, DefaultTransactionPort, DefaultLocalTransactionPath)
		} else {
			c.ServiceURLs[ServiceOnboarding] = fmt.Sprintf("%s/onboarding", baseURL)
			c.ServiceURLs[ServiceTransaction] = fmt.Sprintf("%s/transaction", baseURL)
		}

		return nil
	}
}

// WithHTTPClient sets a custom HTTP client for the Config.
// This allows for advanced customization of the HTTP client behavior.
//
// Parameters:
//   - client: The HTTP client to use
//
// Returns:
//   - Option: A function that sets the HTTP client on a Config
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) error {
		if client == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}
		c.HTTPClient = client
		return nil
	}
}

// WithTimeout sets the timeout duration for HTTP requests.
//
// Parameters:
//   - timeout: The timeout duration
//
// Returns:
//   - Option: A function that sets the timeout on a Config
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be greater than 0")
		}
		c.Timeout = timeout
		return nil
	}
}

// WithUserAgent sets the user agent for HTTP requests.
//
// Parameters:
//   - userAgent: The user agent string
//
// Returns:
//   - Option: A function that sets the user agent on a Config
func WithUserAgent(userAgent string) Option {
	return func(c *Config) error {
		if userAgent == "" {
			return fmt.Errorf("user agent cannot be empty")
		}
		c.UserAgent = userAgent
		return nil
	}
}

// WithRetryConfig sets the retry configuration for HTTP requests.
//
// Parameters:
//   - maxRetries: The maximum number of retry attempts
//   - minWait: The minimum wait time between retries
//   - maxWait: The maximum wait time between retries
//
// Returns:
//   - Option: A function that sets the retry configuration on a Config
func WithRetryConfig(maxRetries int, minWait, maxWait time.Duration) Option {
	return func(c *Config) error {
		if maxRetries < 0 {
			return fmt.Errorf("max retries cannot be negative")
		}
		if minWait <= 0 {
			return fmt.Errorf("minimum wait time must be greater than 0")
		}
		if maxWait < minWait {
			return fmt.Errorf("maximum wait time must be greater than or equal to minimum wait time")
		}

		c.MaxRetries = maxRetries
		c.RetryWaitMin = minWait
		c.RetryWaitMax = maxWait
		return nil
	}
}

// WithRetries enables or disables retry functionality.
//
// Parameters:
//   - enable: Whether to enable retries
//
// Returns:
//   - Option: A function that sets the retry flag on a Config
func WithRetries(enable bool) Option {
	return func(c *Config) error {
		c.EnableRetries = enable
		return nil
	}
}

// WithDebug enables or disables debug mode.
// In debug mode, the SDK logs detailed information about requests and responses.
//
// Parameters:
//   - enable: Whether to enable debug mode
//
// Returns:
//   - Option: A function that sets the debug flag on a Config
func WithDebug(enable bool) Option {
	return func(c *Config) error {
		c.Debug = enable
		return nil
	}
}

// WithObservabilityProvider sets the observability provider.
//
// Parameters:
//   - provider: The observability provider to use
//
// Returns:
//   - Option: A function that sets the observability provider on a Config
func WithObservabilityProvider(provider observability.Provider) Option {
	return func(c *Config) error {
		c.ObservabilityProvider = provider
		return nil
	}
}

// WithIdempotency enables or disables automatic idempotency key generation.
//
// Parameters:
//   - enable: Whether to enable idempotency key generation
//
// Returns:
//   - Option: A function that sets the idempotency flag on a Config
func WithIdempotency(enable bool) Option {
	return func(c *Config) error {
		c.EnableIdempotency = enable
		return nil
	}
}

// WithAccessManager sets the plugin-based authentication configuration.
//
// Parameters:
//   - AccessManager: The plugin authentication configuration
//
// Returns:
//   - Option: A function that sets the plugin authentication on a Config
func WithAccessManager(AccessManager auth.AccessManager) Option {
	return func(c *Config) error {
		c.AccessManager = AccessManager
		return nil
	}
}

// FromEnvironment loads configuration from environment variables.
// This allows for configuration without code changes.
//
// Environment variables:
// - MIDAZ_ENVIRONMENT: The environment to use (local, development, production)
// - MIDAZ_AUTH_TOKEN: The authentication token
// - MIDAZ_USER_AGENT: The user agent string to use for HTTP requests
// - MIDAZ_ONBOARDING_URL: The URL for the Onboarding API
// - MIDAZ_TRANSACTION_URL: The URL for the Transaction API
// - MIDAZ_BASE_URL: The base URL for all services
// - MIDAZ_TIMEOUT: The timeout in seconds for HTTP requests
// - MIDAZ_DEBUG: Enable debug mode (true/false)
// - MIDAZ_MAX_RETRIES: Maximum number of retries
// - MIDAZ_IDEMPOTENCY: Enable idempotency (true/false)
//
// Returns:
//   - Option: A function that sets configuration from environment variables
func FromEnvironment() Option {
	return func(c *Config) error {
		// Load environment variables
		if env := os.Getenv("MIDAZ_ENVIRONMENT"); env != "" {
			switch Environment(env) {
			case EnvironmentLocal:
				c.Environment = EnvironmentLocal
			case EnvironmentDevelopment:
				c.Environment = EnvironmentDevelopment
			case EnvironmentProduction:
				c.Environment = EnvironmentProduction
			default:
				return fmt.Errorf("invalid environment: %s", env)
			}
		}

		if enable := os.Getenv("PLUGIN_AUTH_ENABLED"); enable != "" {
			c.AccessManager.Address = os.Getenv("PLUGIN_AUTH_ADDRESS")
			c.AccessManager.ClientID = os.Getenv("MIDAZ_CLIENT_ID")
			c.AccessManager.ClientSecret = os.Getenv("MIDAZ_CLIENT_SECRET")
			c.AccessManager.Enabled = enable == "true"
		}

		// Set user agent from environment if available
		if userAgent := os.Getenv("MIDAZ_USER_AGENT"); userAgent != "" {
			c.UserAgent = userAgent
		}

		// URLs take precedence in this order: specific URL > base URL > environment default
		if baseURL := os.Getenv("MIDAZ_BASE_URL"); baseURL != "" {
			if err := WithBaseURL(baseURL)(c); err != nil {
				return err
			}
		}

		// Specific URLs override base URL
		if url := os.Getenv("MIDAZ_ONBOARDING_URL"); url != "" {
			if err := WithOnboardingURL(url)(c); err != nil {
				return err
			}
		}

		if url := os.Getenv("MIDAZ_TRANSACTION_URL"); url != "" {
			if err := WithTransactionURL(url)(c); err != nil {
				return err
			}
		}

		// Other settings
		if debug := os.Getenv("MIDAZ_DEBUG"); debug == "true" {
			c.Debug = true
		}

		if timeout := os.Getenv("MIDAZ_TIMEOUT"); timeout != "" {
			seconds, err := parseEnvInt(timeout)
			if err != nil {
				return fmt.Errorf("invalid timeout: %w", err)
			}
			c.Timeout = time.Duration(seconds) * time.Second
		}

		if retries := os.Getenv("MIDAZ_MAX_RETRIES"); retries != "" {
			maxRetries, err := parseEnvInt(retries)
			if err != nil {
				return fmt.Errorf("invalid max retries: %w", err)
			}
			c.MaxRetries = maxRetries
		}

		if idempotency := os.Getenv("MIDAZ_IDEMPOTENCY"); idempotency != "" {
			c.EnableIdempotency = idempotency == "true"
		}

		return nil
	}
}

// parseEnvInt parses an integer from an environment variable.
func parseEnvInt(value string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(value, "%d", &result); err != nil {
		return 0, err
	}

	return result, nil
}

// NewConfig creates a new Config with default values.
// The resulting Config will have sensible defaults for all settings.
// Use options to customize any aspect of the configuration.
//
// Parameters:
//   - options: Zero or more Option functions to customize the Config
//
// Returns:
//   - *Config: A new configuration with the provided options applied
//   - error: An error if any option validation fails
func NewConfig(options ...Option) (*Config, error) {
	// Create a config with default values
	config := &Config{
		AccessManager:     auth.AccessManager{},
		Environment:       EnvironmentLocal,
		ServiceURLs:       make(map[ServiceType]string),
		Timeout:           DefaultTimeout * time.Second,
		UserAgent:         DefaultUserAgent,
		MaxRetries:        DefaultMaxRetries,
		RetryWaitMin:      DefaultRetryWaitMin,
		RetryWaitMax:      DefaultRetryWaitMax,
		EnableRetries:     DefaultEnableRetries,
		EnableIdempotency: DefaultEnableIdempotency,
	}

	// Apply default URLs based on environment
	if err := setDefaultServiceURLs(config); err != nil {
		return nil, err
	}

	// Apply provided options
	for _, option := range options {
		if err := option(config); err != nil {
			return nil, err
		}
	}

	// Create HTTP client if not provided
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// setDefaultServiceURLs sets default URLs based on the environment.
func setDefaultServiceURLs(config *Config) error {
	// Set default URLs based on environment
	switch config.Environment {
	case EnvironmentLocal:
		baseURL := DefaultLocalBaseURL
		config.ServiceURLs[ServiceOnboarding] = fmt.Sprintf("%s:%s%s", baseURL, DefaultOnboardingPort, DefaultLocalOnboardingPath)
		config.ServiceURLs[ServiceTransaction] = fmt.Sprintf("%s:%s%s", baseURL, DefaultTransactionPort, DefaultLocalTransactionPath)
	case EnvironmentDevelopment:
		baseURL := DefaultDevelopmentBaseURL
		config.ServiceURLs[ServiceOnboarding] = fmt.Sprintf("%s/onboarding", baseURL)
		config.ServiceURLs[ServiceTransaction] = fmt.Sprintf("%s/transaction", baseURL)
	case EnvironmentProduction:
		baseURL := DefaultProductionBaseURL
		config.ServiceURLs[ServiceOnboarding] = fmt.Sprintf("%s/onboarding", baseURL)
		config.ServiceURLs[ServiceTransaction] = fmt.Sprintf("%s/transaction", baseURL)
	default:
		return fmt.Errorf("unknown environment: %s", config.Environment)
	}

	return nil
}

// validateConfig ensures that the Config has all required fields.
func validateConfig(config *Config) error {
	// Check that we have URLs for required services
	if _, ok := config.ServiceURLs[ServiceOnboarding]; !ok {
		return fmt.Errorf("onboarding URL is required")
	}
	if _, ok := config.ServiceURLs[ServiceTransaction]; !ok {
		return fmt.Errorf("transaction URL is required")
	}

	// When plugin auth is enabled, we require the plugin auth address
	if config.AccessManager.Enabled && config.AccessManager.Address == "" {
		// But for tests, we'll skip this check
		if os.Getenv("MIDAZ_SKIP_AUTH_CHECK") != "true" {
			return fmt.Errorf("plugin auth address is required")
		}
	}

	return nil
}

// GetBaseURLs converts ServiceURLs to the map format expected by the entity layer.
func (c *Config) GetBaseURLs() map[string]string {
	result := make(map[string]string)
	for service, url := range c.ServiceURLs {
		result[string(service)] = url
	}
	return result
}

// GetHTTPClient returns the HTTP client to use for requests.
func (c *Config) GetHTTPClient() *http.Client {
	return c.HTTPClient
}

// GetPluginAuth returns the plugin authentication configuration.
func (c *Config) GetPluginAuth() auth.AccessManager {
	// Return a copy of the plugin auth configuration
	return auth.AccessManager{
		Address:      c.AccessManager.Address,
		ClientID:     c.AccessManager.ClientID,
		ClientSecret: c.AccessManager.ClientSecret,
		Enabled:      c.AccessManager.Enabled,
	}
}

// GetObservabilityProvider returns the observability provider.
func (c *Config) GetObservabilityProvider() observability.Provider {
	return c.ObservabilityProvider
}

// parseURL validates that a URL is properly formatted.
func parseURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// Require scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("URL must include scheme and host")
	}

	return parsedURL, nil
}

// DefaultConfig creates a new Config with default values.
// Unlike NewConfig, this doesn't validate required fields, making it suitable for initialization
// before applying options.
//
// Returns:
//   - *Config: A new configuration with default values
func DefaultConfig() *Config {
	// Create a config with default values
	config := &Config{
		Environment:       EnvironmentLocal,
		ServiceURLs:       make(map[ServiceType]string),
		Timeout:           DefaultTimeout * time.Second,
		UserAgent:         DefaultUserAgent,
		MaxRetries:        DefaultMaxRetries,
		RetryWaitMin:      DefaultRetryWaitMin,
		RetryWaitMax:      DefaultRetryWaitMax,
		EnableRetries:     DefaultEnableRetries,
		EnableIdempotency: DefaultEnableIdempotency,
	}

	// Apply default URLs based on environment (ignoring error for default config)
	_ = setDefaultServiceURLs(config)

	// Create HTTP client
	config.HTTPClient = &http.Client{
		Timeout: config.Timeout,
	}

	return config
}

// WithMaxRetries sets the maximum number of retries for HTTP requests.
//
// Parameters:
//   - maxRetries: The maximum number of retry attempts
//
// Returns:
//   - Option: A function that sets the max retries on a Config
func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) error {
		if maxRetries < 0 {
			return fmt.Errorf("max retries cannot be negative")
		}
		c.MaxRetries = maxRetries
		return nil
	}
}

// WithRetryWaitMin sets the minimum wait time between retries.
//
// Parameters:
//   - waitTime: The minimum wait time between retries
//
// Returns:
//   - Option: A function that sets the minimum retry wait time on a Config
func WithRetryWaitMin(waitTime time.Duration) Option {
	return func(c *Config) error {
		if waitTime <= 0 {
			return fmt.Errorf("minimum wait time must be greater than 0")
		}
		c.RetryWaitMin = waitTime

		return nil
	}
}

// WithRetryWaitMax sets the maximum wait time between retries.
//
// Parameters:
//   - waitTime: The maximum wait time between retries
//
// Returns:
//   - Option: A function that sets the maximum retry wait time on a Config
func WithRetryWaitMax(waitTime time.Duration) Option {
	return func(c *Config) error {
		if waitTime <= 0 {
			return fmt.Errorf("maximum wait time must be greater than 0")
		}
		if waitTime < c.RetryWaitMin {
			return fmt.Errorf("maximum wait time must be greater than or equal to minimum wait time")
		}
		c.RetryWaitMax = waitTime
		return nil
	}
}

// NewLocalConfig creates a Config for local development.
// This is a convenience function for quickly setting up a local configuration.
//
// Parameters:
//   - authToken: The authentication token to use (deprecated, use PLUGIN_AUTH_ADDRESS env var instead)
//   - options: Additional options to apply
//
// Returns:
//   - *Config: A configuration for local development
//   - error: An error if configuration fails
func NewLocalConfig(options ...Option) (*Config, error) {
	// Get plugin auth values from environment
	pluginAuthEnabled := false
	pluginAuthAddress := "" // Default to authToken for backward compatibility
	pluginAuthClientID := ""
	pluginAuthClientSecret := ""

	if enabled := os.Getenv("PLUGIN_AUTH_ENABLED"); enabled != "" {
		pluginAuthEnabled = enabled == "true" || enabled == "1"
	}

	if address := os.Getenv("PLUGIN_AUTH_ADDRESS"); address != "" {
		pluginAuthAddress = address
	}

	if clientID := os.Getenv("MIDAZ_CLIENT_ID"); clientID != "" {
		pluginAuthClientID = clientID
	}

	if clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET"); clientSecret != "" {
		pluginAuthClientSecret = clientSecret
	}

	// Start with local environment
	localOptions := append(
		[]Option{
			WithEnvironment(EnvironmentLocal),
			WithAccessManager(auth.AccessManager{
				Enabled:      pluginAuthEnabled,
				Address:      pluginAuthAddress,
				ClientID:     pluginAuthClientID,
				ClientSecret: pluginAuthClientSecret,
			}),
		},
		options...,
	)

	return NewConfig(localOptions...)
}
