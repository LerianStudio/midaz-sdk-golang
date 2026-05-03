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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/security"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/version"
)

// ServiceType represents a type of service in the Midaz API ecosystem.
type ServiceType string

// Service types constants define the available Midaz services.
const (
	// ServiceOnboarding represents the Onboarding service.
	ServiceOnboarding ServiceType = "onboarding"

	// ServiceTransaction represents the Transaction service.
	ServiceTransaction ServiceType = "transaction"

	// ServiceCRM represents the CRM service.
	ServiceCRM ServiceType = "crm"
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
	// Default timeout for HTTP requests in seconds
	DefaultTimeout = 60

	// Default URLs for each environment
	DefaultLocalLedgerBaseURL       = "http://localhost:3002"
	DefaultLocalCRMBaseURL          = "http://localhost:4003"
	DefaultDevelopmentLedgerBaseURL = "https://api.dev.midaz.io"
	DefaultProductionLedgerBaseURL  = "https://api.midaz.io"
	DefaultLedgerAPIVersionPath     = "/v1"

	// Default retry configuration
	DefaultMaxRetries   = 3
	DefaultMinRetryWait = 1 * time.Second
	DefaultRetryWaitMax = 30 * time.Second

	// Default feature flags
	DefaultEnableIdempotency = true
	DefaultEnableRetries     = true
)

// Boolean string values for environment variable comparison.
const (
	// boolTrue represents the string value "true" for boolean environment variables.
	boolTrue = "true"
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

	// TenantID is the default tenant identifier sent as X-Tenant-ID on every request.
	// It can be set via the WithTenantID option.
	// Per-request overrides via entities.WithTenantID(ctx, id) take precedence. This
	// is an optional compatibility header and may be ignored by deployments that derive
	// tenant scope from authenticated claims.
	TenantID string

	// tenantIDSet tracks whether WithTenantID was explicitly called, allowing
	// an empty value to clear any environment-provided default.
	tenantIDSet bool

	baseURLSet        bool
	onboardingURLSet  bool
	transactionURLSet bool
	crmURLSet         bool
	httpClientOwned   bool
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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.Environment = env
		if !c.baseURLSet {
			if err := setDefaultServiceURLs(c); err != nil {
				return err
			}
		}

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
func WithOnboardingURL(onboardingURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		// Validate URL
		if err := parseURL(onboardingURL); err != nil {
			return fmt.Errorf("invalid onboarding URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		c.ServiceURLs[ServiceOnboarding] = strings.TrimRight(onboardingURL, "/")
		c.onboardingURLSet = true

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
func WithTransactionURL(transactionURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		// Validate URL
		if err := parseURL(transactionURL); err != nil {
			return fmt.Errorf("invalid transaction URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		c.ServiceURLs[ServiceTransaction] = strings.TrimRight(transactionURL, "/")
		c.transactionURLSet = true

		return nil
	}
}

// WithCRMURL sets the base URL for the CRM API.
func WithCRMURL(crmURL string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if err := parseURL(crmURL); err != nil {
			return fmt.Errorf("invalid crm URL: %w", err)
		}

		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		c.ServiceURLs[ServiceCRM] = strings.TrimRight(crmURL, "/")
		c.crmURLSet = true

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		// Validate the base URL
		if err := parseURL(baseURL); err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}

		// Remove trailing slash if present
		baseURL = strings.TrimSuffix(baseURL, "/")

		// Initialize the map if needed
		if c.ServiceURLs == nil {
			c.ServiceURLs = make(map[ServiceType]string)
		}

		ledgerURL, err := buildLedgerServiceURL(baseURL)
		if err != nil {
			return fmt.Errorf("invalid ledger base URL: %w", err)
		}

		crmURL, err := buildCRMServiceURL(baseURL)
		if err != nil {
			return fmt.Errorf("invalid crm base URL: %w", err)
		}

		if !c.onboardingURLSet {
			c.ServiceURLs[ServiceOnboarding] = ledgerURL
		}

		if !c.transactionURLSet {
			c.ServiceURLs[ServiceTransaction] = ledgerURL
		}

		if !c.crmURLSet {
			c.ServiceURLs[ServiceCRM] = crmURL
		}

		c.baseURLSet = true

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if client == nil {
			return errors.New("HTTP client cannot be nil")
		}

		c.HTTPClient = client
		c.httpClientOwned = false

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if timeout <= 0 {
			return errors.New("timeout must be greater than 0")
		}

		c.Timeout = timeout
		if c.HTTPClient != nil && c.httpClientOwned {
			c.HTTPClient.Timeout = timeout
		}

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if userAgent == "" {
			return errors.New("user agent cannot be empty")
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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if maxRetries < 0 {
			return errors.New("max retries cannot be negative")
		}

		if minWait <= 0 {
			return errors.New("minimum wait time must be greater than 0")
		}

		if maxWait < minWait {
			return errors.New("maximum wait time must be greater than or equal to minimum wait time")
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
		if c == nil {
			return errors.New("config cannot be nil")
		}

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if isTypedNil(provider) {
			return errors.New("observability provider cannot be nil")
		}

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.EnableIdempotency = enable

		return nil
	}
}

// WithTenantID sets the default tenant ID for all API requests.
// The tenant ID is sent as the X-Tenant-ID header on every request.
// Per-request overrides via entities.WithTenantID(ctx, tenantID) take precedence
// over this configuration-level default. This header is best-effort compatibility
// metadata rather than the sole tenant source of truth for the reference Midaz path.
//
// Parameters:
//   - tenantID: The tenant identifier to use
//
// Returns:
//   - Option: A function that sets the tenant ID on a Config
func WithTenantID(tenantID string) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.TenantID = strings.TrimSpace(tenantID)
		c.tenantIDSet = true

		return nil
	}
}

// WithAccessManager sets the plugin-based authentication configuration.
//
// Parameters:
//   - accessManager: The plugin authentication configuration
//
// Returns:
//   - Option: A function that sets the plugin authentication on a Config
func WithAccessManager(accessManager auth.AccessManager) Option {
	return func(c *Config) error {
		if c == nil {
			return errors.New("config cannot be nil")
		}

		c.AccessManager = accessManager

		return nil
	}
}

// FromEnvironment loads configuration from environment variables.
// This allows for configuration without code changes.
//
// Environment variables:
// - MIDAZ_ENVIRONMENT: The environment to use (local, development, production)
// - PLUGIN_AUTH_ENABLED: Enable access manager authentication (true/false)
// - PLUGIN_AUTH_ADDRESS: The address of the access manager service
// - MIDAZ_CLIENT_ID: The client ID for authentication
// - MIDAZ_CLIENT_SECRET: The client secret for authentication
// - MIDAZ_USER_AGENT: The user agent string to use for HTTP requests
// - MIDAZ_ONBOARDING_URL: The URL for the Onboarding API
// - MIDAZ_TRANSACTION_URL: The URL for the Transaction API
// - MIDAZ_CRM_URL: The URL for the CRM API
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
		if err := configureEnvironment(c); err != nil {
			return err
		}

		configureAccessManager(c)
		configureUserAgent(c)

		if err := configureURLs(c); err != nil {
			return err
		}

		if err := configureTimeoutAndRetries(c); err != nil {
			return err
		}

		configureOptionalSettings(c)

		return nil
	}
}

// configureEnvironment sets the environment from environment variables
func configureEnvironment(c *Config) error {
	env := os.Getenv("MIDAZ_ENVIRONMENT")
	if env == "" {
		return nil
	}

	switch Environment(env) {
	case EnvironmentLocal:
		return WithEnvironment(EnvironmentLocal)(c)
	case EnvironmentDevelopment:
		return WithEnvironment(EnvironmentDevelopment)(c)
	case EnvironmentProduction:
		return WithEnvironment(EnvironmentProduction)(c)
	default:
		return fmt.Errorf("invalid environment: %s", env)
	}
}

// configureAccessManager sets up access manager configuration from environment
func configureAccessManager(c *Config) {
	if enable := os.Getenv("PLUGIN_AUTH_ENABLED"); enable != "" {
		c.AccessManager.Address = os.Getenv("PLUGIN_AUTH_ADDRESS")
		c.AccessManager.ClientID = os.Getenv("MIDAZ_CLIENT_ID")
		c.AccessManager.ClientSecret = os.Getenv("MIDAZ_CLIENT_SECRET")
		c.AccessManager.Enabled = enable == boolTrue
	}
}

// configureUserAgent sets user agent from environment if available
func configureUserAgent(c *Config) {
	if userAgent := os.Getenv("MIDAZ_USER_AGENT"); userAgent != "" {
		c.UserAgent = userAgent
	}
}

// configureURLs sets up URL configuration from environment variables
func configureURLs(c *Config) error {
	// URLs take precedence in this order: specific URL > base URL > environment default
	if baseURL := os.Getenv("MIDAZ_BASE_URL"); baseURL != "" {
		if err := WithBaseURL(baseURL)(c); err != nil {
			return err
		}
	}

	return configureSpecificURLs(c)
}

// configureSpecificURLs sets specific service URLs that override base URL
func configureSpecificURLs(c *Config) error {
	if onboardingURL := os.Getenv("MIDAZ_ONBOARDING_URL"); onboardingURL != "" {
		if err := WithOnboardingURL(onboardingURL)(c); err != nil {
			return err
		}
	}

	if transactionURL := os.Getenv("MIDAZ_TRANSACTION_URL"); transactionURL != "" {
		if err := WithTransactionURL(transactionURL)(c); err != nil {
			return err
		}
	}

	if crmURL := os.Getenv("MIDAZ_CRM_URL"); crmURL != "" {
		if err := WithCRMURL(crmURL)(c); err != nil {
			return err
		}
	}

	return nil
}

// configureTimeoutAndRetries sets timeout and retry configuration from environment
func configureTimeoutAndRetries(c *Config) error {
	if err := configureTimeout(c); err != nil {
		return err
	}

	return configureRetries(c)
}

// configureTimeout sets timeout from environment variable
func configureTimeout(c *Config) error {
	timeout := os.Getenv("MIDAZ_TIMEOUT")
	if timeout == "" {
		return nil
	}

	seconds, err := parseEnvInt(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	return WithTimeout(time.Duration(seconds) * time.Second)(c)
}

// configureRetries sets max retries from environment variable
func configureRetries(c *Config) error {
	retries := os.Getenv("MIDAZ_MAX_RETRIES")
	if retries == "" {
		return nil
	}

	maxRetries, err := parseEnvInt(retries)
	if err != nil {
		return fmt.Errorf("invalid max retries: %w", err)
	}

	return WithMaxRetries(maxRetries)(c)
}

// configureOptionalSettings sets optional boolean settings from environment
func configureOptionalSettings(c *Config) {
	if debug := os.Getenv("MIDAZ_DEBUG"); debug == boolTrue {
		c.Debug = true
	}

	if idempotency := os.Getenv("MIDAZ_IDEMPOTENCY"); idempotency != "" {
		c.EnableIdempotency = idempotency == boolTrue
	}
}

// parseEnvInt parses an integer from an environment variable.
func parseEnvInt(value string) (int, error) {
	result, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
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
		UserAgent:         version.UserAgent(),
		MaxRetries:        DefaultMaxRetries,
		RetryWaitMin:      DefaultMinRetryWait,
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
		if option == nil {
			return nil, errors.New("option cannot be nil")
		}

		if err := option(config); err != nil {
			return nil, err
		}
	}

	// Create HTTP client if not provided
	if config.HTTPClient == nil {
		config.HTTPClient = NewDefaultHTTPClient(config.Timeout)
		config.httpClientOwned = true
	}

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// setDefaultServiceURLs sets default URLs based on the environment.
func setDefaultServiceURLs(config *Config) error {
	ensureServiceURLMap(config)

	serviceURLs, err := defaultServiceURLsForEnvironment(config.Environment)
	if err != nil {
		return err
	}

	applyDefaultServiceURLs(config, serviceURLs)

	return nil
}

func ensureServiceURLMap(config *Config) {
	if config.ServiceURLs == nil {
		config.ServiceURLs = make(map[ServiceType]string)
	}
}

type defaultServiceURLs struct {
	ledgerURL string
	crmURL    string
}

func defaultServiceURLsForEnvironment(environment Environment) (defaultServiceURLs, error) {
	switch environment {
	case EnvironmentLocal:
		return defaultLocalServiceURLs()
	case EnvironmentDevelopment:
		return defaultLedgerBackedServiceURLs(DefaultDevelopmentLedgerBaseURL)
	case EnvironmentProduction:
		return defaultLedgerBackedServiceURLs(DefaultProductionLedgerBaseURL)
	default:
		return defaultServiceURLs{}, fmt.Errorf("unknown environment: %s", environment)
	}
}

func defaultLocalServiceURLs() (defaultServiceURLs, error) {
	ledgerURL, err := buildLedgerServiceURL(DefaultLocalLedgerBaseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	crmURL, err := buildCRMServiceURL(DefaultLocalCRMBaseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	return defaultServiceURLs{ledgerURL: ledgerURL, crmURL: crmURL}, nil
}

func defaultLedgerBackedServiceURLs(baseURL string) (defaultServiceURLs, error) {
	ledgerURL, err := buildLedgerServiceURL(baseURL)
	if err != nil {
		return defaultServiceURLs{}, err
	}

	return defaultServiceURLs{ledgerURL: ledgerURL, crmURL: ledgerURL}, nil
}

func applyDefaultServiceURLs(config *Config, serviceURLs defaultServiceURLs) {
	if !config.onboardingURLSet {
		config.ServiceURLs[ServiceOnboarding] = serviceURLs.ledgerURL
	}

	if !config.transactionURLSet {
		config.ServiceURLs[ServiceTransaction] = serviceURLs.ledgerURL
	}

	if !config.crmURLSet {
		config.ServiceURLs[ServiceCRM] = serviceURLs.crmURL
	}
}

// validateConfig ensures that the Config has all required fields.
func validateConfig(config *Config) error {
	// Check that we have URLs for required services
	if _, ok := config.ServiceURLs[ServiceOnboarding]; !ok {
		return errors.New("onboarding URL is required")
	}

	if _, ok := config.ServiceURLs[ServiceTransaction]; !ok {
		return errors.New("transaction URL is required")
	}

	// When plugin auth is enabled, we require the plugin auth address
	if config.AccessManager.Enabled && config.AccessManager.Address == "" {
		// But for tests, we'll skip this check
		if os.Getenv("MIDAZ_SKIP_AUTH_CHECK") != boolTrue {
			return errors.New("plugin auth address is required")
		}
	}

	return nil
}

// GetBaseURLs converts ServiceURLs to the map format expected by the entity layer.
func (c *Config) GetBaseURLs() map[string]string {
	result := make(map[string]string)
	for service, serviceURL := range c.ServiceURLs {
		result[string(service)] = serviceURL
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

// Clone returns an independent copy of the configuration.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	cloned := *c
	if c.ServiceURLs != nil {
		cloned.ServiceURLs = make(map[ServiceType]string, len(c.ServiceURLs))
		for service, serviceURL := range c.ServiceURLs {
			cloned.ServiceURLs[service] = serviceURL
		}
	}

	return &cloned
}

// parseURL validates that a URL is properly formatted.
// It also warns (via stderr) if using HTTP instead of HTTPS for non-localhost URLs.
func parseURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	// Require scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("URL must include scheme and host")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}

	if parsedURL.User != nil {
		return errors.New("URL must not include user information")
	}

	if parsedURL.Scheme == "http" && !isLocalhost(parsedURL.Host) {
		return fmt.Errorf("insecure HTTP is only allowed for localhost targets: %s", parsedURL.Host)
	}

	return nil
}

// NewDefaultHTTPClient returns an SDK-owned HTTP client with a conservative pooled transport.
func NewDefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return security.ValidateOutboundRequest(req)
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

func isTypedNil(value any) bool {
	if value == nil {
		return false
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func buildLedgerServiceURL(baseURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}

	if isLocalhost(parsedURL.Host) && parsedURL.Port() == "" {
		parsedURL.Host = withPort(parsedURL.Hostname(), "3002")
	}

	return ensureAPIVersionPath(parsedURL), nil
}

func buildCRMServiceURL(baseURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSuffix(baseURL, "/"))
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}

	if isLocalhost(parsedURL.Host) && parsedURL.Port() == "" {
		parsedURL.Host = withPort(parsedURL.Hostname(), "4003")
	}

	return ensureAPIVersionPath(parsedURL), nil
}

func ensureAPIVersionPath(parsedURL *url.URL) string {
	cleanPath := strings.TrimSuffix(parsedURL.Path, "/")
	if cleanPath == "" {
		parsedURL.Path = DefaultLedgerAPIVersionPath
		return parsedURL.String()
	}

	if cleanPath == DefaultLedgerAPIVersionPath {
		parsedURL.Path = cleanPath
		return parsedURL.String()
	}

	parsedURL.Path = cleanPath + DefaultLedgerAPIVersionPath

	return parsedURL.String()
}

func withPort(hostname, port string) string {
	if strings.Contains(hostname, ":") {
		return "[" + hostname + "]:" + port
	}

	return hostname + ":" + port
}

// isLocalhost checks if the host is a localhost address (for development use).
func isLocalhost(host string) bool {
	hostname := host
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		hostname = splitHost
	}

	hostname = strings.Trim(hostname, "[]")
	if hostname == "localhost" {
		return true
	}
	// RFC 6761 §6.3: ".localhost" is a reserved special-use TLD that resolvers
	// must treat as loopback. Used by Docker Compose aliases and dev tooling.
	if strings.HasSuffix(hostname, ".localhost") {
		return true
	}

	ip := net.ParseIP(hostname)

	return ip != nil && ip.IsLoopback()
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
		UserAgent:         version.UserAgent(),
		MaxRetries:        DefaultMaxRetries,
		RetryWaitMin:      DefaultMinRetryWait,
		RetryWaitMax:      DefaultRetryWaitMax,
		EnableRetries:     DefaultEnableRetries,
		EnableIdempotency: DefaultEnableIdempotency,
	}

	// Apply default URLs based on environment.
	// Error is safely ignored because DefaultConfig always uses EnvironmentLocal
	// which is a valid, known environment that will never return an error.
	_ = setDefaultServiceURLs(config) //nolint:errcheck // EnvironmentLocal is hardcoded above and always valid

	// Create HTTP client
	config.HTTPClient = NewDefaultHTTPClient(config.Timeout)
	config.httpClientOwned = true

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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if maxRetries < 0 {
			return errors.New("max retries cannot be negative")
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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if waitTime <= 0 {
			return errors.New("minimum wait time must be greater than 0")
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
		if c == nil {
			return errors.New("config cannot be nil")
		}

		if waitTime <= 0 {
			return errors.New("maximum wait time must be greater than 0")
		}

		if waitTime < c.RetryWaitMin {
			return errors.New("maximum wait time must be greater than or equal to minimum wait time")
		}

		c.RetryWaitMax = waitTime

		return nil
	}
}

// NewLocalConfig creates a Config for local development.
// This is a convenience function for quickly setting up a local configuration.
//
// Parameters:
//   - options: Additional options to apply after local defaults and Access Manager environment values
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
		pluginAuthEnabled = enabled == boolTrue || enabled == "1"
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
