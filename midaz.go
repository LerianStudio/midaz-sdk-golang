// Package client provides a client for the Midaz API.
// It is the top-level entry point for interacting with the SDK.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

// Version is the current version of the SDK.
// This is automatically updated during the release process.
const Version = "1.1.0-beta.2"

// Client is the main entry point for using the Midaz API.
// It provides access to all API services, connection management,
// authentication, rate limiting, and retry handling.
type Client struct {
	// Configuration
	config *config.Config
	ctx    context.Context

	// Optional API interfaces
	Entity *entities.Entity

	// API interface flags
	useEntity bool

	// tenantID is the default tenant identifier sent as X-Tenant-ID on every request.
	// Per-request overrides via entities.WithTenantID(ctx, id) take precedence.
	// This remains an optional compatibility header; authenticated claims are the
	// primary tenant source of truth in the reference Midaz path.
	tenantID string
	// tenantIDSet tracks whether WithTenantID was explicitly called, allowing
	// an empty value to override the config/env default.
	tenantIDSet bool

	// Observability provider
	observability     observability.Provider
	metrics           *observability.MetricsCollector
	customRetryPolicy func(*http.Response, error) bool
}

// New creates a new Midaz client with the provided options.
func New(options ...Option) (*Client, error) {
	// Create a new client with default settings
	c := &Client{
		ctx: context.Background(), // Default context that can be overridden with WithContext
	}

	// Initialize default observability provider (disabled by default)
	obsProvider, err := observability.New(context.Background(),
		observability.WithServiceName("midaz-go-sdk"),
		observability.WithComponentEnabled(false, false, false), // All disabled by default
	)
	if err != nil {
		return nil, err
	}

	c.observability = obsProvider

	// Create default configuration
	c.config = config.DefaultConfig()

	// Apply all options
	for _, option := range options {
		if option == nil {
			return nil, errors.New("option cannot be nil")
		}

		if err := option(c); err != nil {
			return nil, fmt.Errorf("error applying option: %w", err)
		}
	}

	// Create API interfaces if enabled
	if c.useEntity {
		if err := c.setupEntity(); err != nil {
			return nil, fmt.Errorf("error setting up Entity API: %w", err)
		}
	}

	return c, nil
}

// Option is a functional option for configuring the client.
type Option func(*Client) error

// setupEntity creates the Entity API interface.
func (c *Client) setupEntity() error {
	// Get service URLs from config
	serviceURLs := c.config.GetBaseURLs()

	// Verify we have the required service URLs
	if _, ok := serviceURLs["onboarding"]; !ok {
		return errors.New("missing onboarding URL in config")
	}

	if _, ok := serviceURLs["transaction"]; !ok {
		return errors.New("missing transaction URL in config")
	}

	if err := config.WithObservabilityProvider(c.observability)(c.config); err != nil {
		return fmt.Errorf("failed to configure observability provider: %w", err)
	}

	// Create the entity API with service-specific URLs
	options := []entities.Option{entities.WithObservability(c.observability)}

	// Propagate tenant ID to the entity layer if configured.
	// Client-level tenantID takes precedence over config-level TenantID.
	// When tenantIDSet is true, the client override wins even if empty
	// (allowing explicit clearing of the config/env default).
	var tenantID string
	if c.tenantIDSet {
		tenantID = c.tenantID
	} else {
		tenantID = c.config.TenantID
	}

	if tenantID != "" {
		options = append(options, entities.WithDefaultTenantID(tenantID))
	}

	options = append(options,
		entities.WithDebug(c.config.Debug),
		entities.WithUserAgent(c.config.UserAgent),
	)

	entity, err := entities.NewEntityWithConfig(c.config, options...)
	if err != nil {
		return err
	}

	httpClient := entity.GetEntityHTTPClient()
	httpClient.SetEnableIdempotency(c.config.EnableIdempotency)
	httpClient.WithRetryOptions(
		retry.WithMaxRetries(c.config.MaxRetries),
		retry.WithInitialDelay(c.config.RetryWaitMin),
		retry.WithMaxDelay(c.config.RetryWaitMax),
	)

	if !c.config.EnableRetries {
		httpClient.WithRetryOption(retry.WithMaxRetries(0))
	}

	if c.customRetryPolicy != nil {
		httpClient.SetCustomRetryPolicy(c.customRetryPolicy)
	}

	entity.InitServices()

	c.Entity = entity

	return nil
}

// WithBaseURL sets the base URL for API requests.
//
// Parameters:
//   - baseURL: The base URL for API requests (e.g. "https://api.midaz.io").
//
// Returns:
//   - Option: A function that sets the base URL on the Client
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		// Validate URL
		_, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}

		// Apply to config
		return config.WithBaseURL(baseURL)(c.config)
	}
}

// WithTimeout sets the request timeout for API requests.
//
// Parameters:
//   - timeout: The timeout duration for requests.
//
// Returns:
//   - Option: A function that sets the timeout on the Client
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		// Apply to config
		return config.WithTimeout(timeout)(c.config)
	}
}

// WithUserAgent sets the user agent for API requests.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		return config.WithUserAgent(userAgent)(c.config)
	}
}

// WithRetries configures the retry policy for failed requests.
//
// Parameters:
//   - maxRetries: The maximum number of retry attempts.
//   - minBackoff: The minimum backoff duration between retries.
//   - maxBackoff: The maximum backoff duration between retries.
//
// Returns:
//   - Option: A function that configures the retry policy on the Client
func WithRetries(maxRetries int, minBackoff, maxBackoff time.Duration) Option {
	return func(c *Client) error {
		// Apply to config
		if err := config.WithRetries(true)(c.config); err != nil {
			return err
		}

		if err := config.WithMaxRetries(maxRetries)(c.config); err != nil {
			return err
		}

		if err := config.WithRetryWaitMin(minBackoff)(c.config); err != nil {
			return err
		}

		return config.WithRetryWaitMax(maxBackoff)(c.config)
	}
}

// WithCustomRetryPolicy sets a custom retry policy for the client.
// This allows for more fine-grained control over when to retry requests.
//
// Parameters:
//   - shouldRetry: A function that decides whether to retry a request based on response and error
//
// Returns:
//   - Option: A function that sets the retry policy on the Client
func WithCustomRetryPolicy(shouldRetry func(*http.Response, error) bool) Option {
	return func(c *Client) error {
		c.customRetryPolicy = shouldRetry

		if c.Entity != nil {
			httpClient := c.Entity.GetEntityHTTPClient()
			if httpClient != nil {
				httpClient.SetCustomRetryPolicy(shouldRetry)
			}
		}

		return nil
	}
}

// DisableRetries disables the retry mechanism.
// This is useful for testing or when you want to handle retries yourself.
//
// Returns:
//   - Option: A function that disables retries on the Client
func DisableRetries() Option {
	return func(c *Client) error {
		// Apply the retry disable to the config
		return config.WithRetries(false)(c.config)
	}
}

// WithObservabilityOptions configures observability for the client with custom options.
//
// Parameters:
//   - options: The observability options to apply to the provider
//
// Returns:
//   - Option: A function that configures observability for the Client with custom options
func WithObservabilityOptions(options ...observability.Option) Option {
	return func(c *Client) error {
		// Create the provider with custom options
		provider, err := observability.New(c.ctx, options...)
		if err != nil {
			return err
		}

		// Set the provider on the client
		c.observability = provider

		// Initialize metrics collector if needed
		if provider.IsEnabled() {
			c.metrics, err = observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}
		}

		// Update the context with the provider
		c.ctx = observability.WithProvider(c.ctx, provider)

		// Note: HTTP client configuration is handled during entity creation

		return nil
	}
}

// WithObservability enables or disables observability features (tracing, metrics, logging).
// This allows for monitoring and debugging of SDK operations.
//
// Parameters:
//   - enableTracing: Whether to enable distributed tracing
//   - enableMetrics: Whether to enable metrics collection
//   - enableLogging: Whether to enable structured logging
//
// Returns:
//   - Option: A function that configures observability for the Client
func WithObservability(enableTracing, enableMetrics, enableLogging bool) Option {
	return func(c *Client) error {
		// Create the provider with functional options
		provider, err := observability.New(c.ctx,
			observability.WithServiceName("midaz-go-sdk"),
			observability.WithServiceVersion(Version),
			observability.WithEnvironment(string(c.config.Environment)),
			observability.WithComponentEnabled(enableTracing, enableMetrics, enableLogging),
		)
		if err != nil {
			return err
		}

		// Set the provider on the client
		c.observability = provider

		// Initialize metrics collector if needed
		if enableMetrics {
			c.metrics, err = observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}
		}

		// Update the context with the provider
		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}
}

// WithObservabilityProvider sets a custom observability provider for the client.
// This is useful when you want to share an observability provider across multiple clients.
//
// Parameters:
//   - provider: The observability provider to use
//
// Returns:
//   - Option: A function that sets the observability provider on the Client
func WithObservabilityProvider(provider observability.Provider) Option {
	return func(c *Client) error {
		if provider == nil {
			return nil
		}

		if reflectutil.IsTypedNil(provider) {
			return errors.New("observability provider cannot be nil")
		}

		// Set the provider on the client
		c.observability = provider

		// Initialize metrics collector if needed
		if provider.IsEnabled() {
			var err error

			c.metrics, err = observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}
		}

		// Update the context with the provider
		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}
}

// WithCollectorEndpoint sets the OTLP collector endpoint for observability.
// This is used to send traces, metrics, and logs to an OpenTelemetry collector.
//
// Parameters:
//   - endpoint: The endpoint for the OpenTelemetry collector
//
// Returns:
//   - Option: A function that sets the collector endpoint on the Client
func WithCollectorEndpoint(endpoint string) Option {
	return func(c *Client) error {
		// Check if there's an existing provider
		current := c.observability
		if current == nil {
			return nil
		}

		// Create the provider with functional options
		provider, err := observability.New(c.ctx,
			observability.WithServiceName("midaz-go-sdk"),
			observability.WithServiceVersion(Version),
			observability.WithEnvironment(string(c.config.Environment)),
			observability.WithCollectorEndpoint(endpoint),
			observability.WithComponentEnabled(true, true, true), // Enable all components
		)
		if err != nil {
			return err
		}

		// Set the provider on the client
		c.observability = provider

		// Initialize metrics collector
		c.metrics, err = observability.NewMetricsCollector(provider)
		if err != nil {
			return err
		}

		// Update the context with the provider
		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}
}

// WithEnvironment sets the environment for the client.
// This is used for configuration options that vary by environment.
//
// Parameters:
//   - env: The environment to use
//
// Returns:
//   - Option: A function that sets the environment on the Client
func WithEnvironment(env config.Environment) Option {
	return func(c *Client) error {
		// Apply to config
		return config.WithEnvironment(env)(c.config)
	}
}

// WithContext sets the client base context used by client-level helpers and
// observability setup. Entity service methods still use the context passed to
// each service call.
//
// Parameters:
//   - ctx: The context to use
//
// Returns:
//   - Option: A function that sets the context on the Client
func WithContext(ctx context.Context) Option {
	return func(c *Client) error {
		if ctx == nil {
			return errors.New("context cannot be nil")
		}

		c.ctx = ctx

		return nil
	}
}

// UseAllAPIs enables all available API interfaces.
// This is a convenience function for enabling all APIs at once.
//
// Returns:
//   - Option: A function that enables all APIs on the Client
func UseAllAPIs() Option {
	return func(c *Client) error {
		c.useEntity = true
		return nil
	}
}

// UseEntityAPI enables the Entity API interface.
// This is the high-level API for working with Midaz entities.
//
// Returns:
//   - Option: A function that enables the Entity API on the Client
func UseEntityAPI() Option {
	return func(c *Client) error {
		c.useEntity = true
		return nil
	}
}

// WithConfig sets a custom configuration for the client.
// This allows for using a pre-configured Config object instead of individual options.
//
// Parameters:
//   - cfg: The configuration to use
//
// Returns:
//   - Option: A function that sets the configuration on the Client
func WithConfig(cfg *config.Config) Option {
	return func(c *Client) error {
		if cfg == nil {
			return errors.New("config cannot be nil")
		}

		c.config = cfg.Clone()
		if provider := c.config.GetObservabilityProvider(); provider != nil && !reflectutil.IsTypedNil(provider) {
			c.observability = provider
			c.ctx = observability.WithProvider(c.ctx, provider)
			if provider.IsEnabled() {
				metrics, err := observability.NewMetricsCollector(provider)
				if err != nil {
					return err
				}

				c.metrics = metrics
			}
		}

		return nil
	}
}

// WithHTTPClient sets a custom HTTP client for the Client.
// This allows for advanced customization of HTTP client behavior.
//
// Parameters:
//   - client: The HTTP client to use
//
// Returns:
//   - Option: A function that sets the HTTP client on the Client
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) error {
		return config.WithHTTPClient(client)(c.config)
	}
}

// WithOnboardingURL sets the URL for the Onboarding API.
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - onboardingURL: The URL for the Onboarding API
//
// Returns:
//   - Option: A function that sets the Onboarding URL on the Client
func WithOnboardingURL(onboardingURL string) Option {
	return func(c *Client) error {
		return config.WithOnboardingURL(onboardingURL)(c.config)
	}
}

// WithTransactionURL sets the URL for the Transaction API.
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - transactionURL: The URL for the Transaction API
//
// Returns:
//   - Option: A function that sets the Transaction URL on the Client
func WithTransactionURL(transactionURL string) Option {
	return func(c *Client) error {
		return config.WithTransactionURL(transactionURL)(c.config)
	}
}

// WithCRMURL sets the URL for the CRM API.
// This overrides any URL derived from the Environment setting.
func WithCRMURL(crmURL string) Option {
	return func(c *Client) error {
		return config.WithCRMURL(crmURL)(c.config)
	}
}

// WithDebug enables or disables debug mode.
// In debug mode, the SDK logs detailed information about requests and responses.
//
// Parameters:
//   - enable: Whether to enable debug mode
//
// Returns:
//   - Option: A function that sets the debug flag on the Client
func WithDebug(enable bool) Option {
	return func(c *Client) error {
		return config.WithDebug(enable)(c.config)
	}
}

// WithTenantID sets the default tenant ID for all API requests made through this client.
// The tenant ID is sent as the X-Tenant-ID header on every request.
// Per-request overrides via entities.WithTenantID(ctx, tenantID) take precedence
// over this client-level default. This is an optional compatibility signal for
// deployments that honor the header, not a replacement for tenant resolution from
// authenticated claims.
//
// Parameters:
//   - tenantID: The tenant identifier to use
//
// Returns:
//   - Option: A function that sets the tenant ID on the Client
func WithTenantID(tenantID string) Option {
	return func(c *Client) error {
		c.tenantID = strings.TrimSpace(tenantID)
		c.tenantIDSet = true

		return nil
	}
}

// UseEntity enables the Entity API interface.
// This is an alias for UseEntityAPI for backward compatibility.
//
// Returns:
//   - Option: A function that enables the Entity API on the Client
func UseEntity() Option {
	return UseEntityAPI()
}

// Shutdown gracefully shuts down the client, releasing any resources.
// This ensures that any pending operations are completed and resources are released.
//
// Parameters:
//   - ctx: The context for the shutdown operation
//
// Returns:
//   - error: An error if the shutdown operation fails
func (c *Client) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}

	// Shutdown observability provider
	if c.observability != nil {
		if err := c.observability.Shutdown(ctx); err != nil {
			return fmt.Errorf("error shutting down observability provider: %w", err)
		}
	}

	return nil
}

// Trace executes the given function within the context of a trace span.
// This is a convenience function for creating a traced operation.
//
// Parameters:
//   - name: The name of the operation
//   - fn: The function to execute within the trace span
//
// Returns:
//   - error: An error if the traced operation fails
func (c *Client) Trace(name string, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("trace callback cannot be nil")
	}

	if c == nil {
		return errors.New("client cannot be nil")
	}

	if c.observability == nil || !c.observability.IsEnabled() {
		return fn(c.ctx)
	}

	return observability.WithSpan(c.ctx, c.observability, name, fn)
}

// Logger returns the logger from the observability provider.
// This is a convenience function for getting the logger.
//
// Returns:
//   - Logger: The logger from the observability provider
func (c *Client) Logger() observability.Logger {
	if c.observability == nil {
		return nil
	}

	return c.observability.Logger()
}

// GetObservabilityProvider returns the observability provider.
// This is useful when you want to use the provider directly.
//
// Returns:
//   - Provider: The observability provider
func (c *Client) GetObservabilityProvider() observability.Provider {
	return c.observability
}

// GetMetricsCollector returns the metrics collector.
// This is useful when you want to record custom metrics.
//
// Returns:
//   - MetricsCollector: The metrics collector
func (c *Client) GetMetricsCollector() *observability.MetricsCollector {
	return c.metrics
}

// GetContext returns the client's context.
// This is useful when you want to use the client's context for other operations.
//
// Returns:
//   - context.Context: The client's context
func (c *Client) GetContext() context.Context {
	return c.ctx
}

// GetConfiguration returns a deep copy of the client's configuration.
//
// Mutating the returned *config.Config does NOT affect the live client.
// The clone is safe to inspect for debugging or to feed into another
// client constructor; it is NOT a handle for runtime tweaks.
//
// The clone can still carry Access Manager credentials, so do not log
// it without redaction.
//
// Returns:
//   - *config.Config: An independent copy of the client configuration.
func (c *Client) GetConfiguration() *config.Config {
	if c == nil || c.config == nil {
		return nil
	}

	return c.config.Clone()
}

// GetConfig returns the client configuration.
// This is an alias for GetConfiguration for backward compatibility.
//
// Returns:
//   - *config.Config: The client configuration
func (c *Client) GetConfig() *config.Config {
	return c.GetConfiguration()
}

// NewAccount constructs a basic account.
func (*Client) NewAccount() *models.Account {
	return &models.Account{}
}

// NewLedger constructs a basic ledger.
func (*Client) NewLedger() *models.Ledger {
	return &models.Ledger{}
}

// NewOrganization constructs a basic organization.
func (*Client) NewOrganization() *models.Organization {
	return &models.Organization{}
}

// NewTransaction constructs a basic transaction.
func (*Client) NewTransaction() *models.Transaction {
	return &models.Transaction{}
}

// NewOperation constructs a basic operation.
func (*Client) NewOperation() *models.Operation {
	return &models.Operation{}
}

// NewAsset constructs a basic asset.
func (*Client) NewAsset() *models.Asset {
	return &models.Asset{}
}

// GetVersion returns the current version of the SDK.
// This is useful for debugging and tracking the SDK version in logs and traces.
//
// Returns:
//   - string: The current version of the SDK
func (*Client) GetVersion() string {
	return Version
}
