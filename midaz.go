// Package midaz is the entry point for the Midaz Go SDK.
//
// # Quickstart (Access Manager auth)
//
//	c, err := midaz.New(
//	    midaz.WithEnvironment(midaz.EnvironmentProduction),
//	    midaz.WithAccessManager(midaz.AccessManager{
//	        Address:      "https://auth.midaz.io",
//	        ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
//	        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
//	    }),
//	)
//	if err != nil { return err }
//	defer c.Shutdown(ctx)
//
//	org, err := c.Organizations.GetOrganization(ctx, "org-id")
//
// # Authentication
//
// v3 requires exactly one auth source at construction time:
//   - [WithAccessManager] — production-shape OAuth via the Lerian
//     Access Manager. Recommended for any non-local stack.
//   - [WithAnonymous] — opt out of authentication. Suitable only for
//     a local Midaz stack with auth disabled.
//
// Calling [New] with neither returns a typed configuration error.
// See docs/auth.md for the full walkthrough.
//
// # Multi-tenancy
//
// Set a default tenant via [WithTenantID]; override per-request via
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithRequestTenantID].
// See docs/multi-tenancy.md.
//
// # Logging and observability
//
// Inject a *slog.Logger via [WithLogger]. Wire OpenTelemetry via
// [WithObservabilityProvider] or [WithObservabilityOptions]. The SDK is
// silent by default (slog.DiscardHandler). See docs/logging.md and
// docs/tracing.md.
//
// # Pagination
//
// Every List* method returns one page. ListAll yields iter.Seq2[T, error]
// for full-collection iteration; ListPages yields page envelopes with
// metadata. Page-based and cursor-based endpoints are distinguished at
// the type system — wrong-shape opts don't compile. See docs/pagination.md.
//
// # Errors
//
// Every error returned by SDK code is a *[github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors.Error]
// with structured Category, Code, Operation, and Resource fields. Use
// errors.Is, errors.As, and the typed predicates (IsNotFoundError,
// IsValidationError, IsNetworkError, IsAuthError, etc.). The Retryable()
// method on *Error is the canonical retry-policy source.
//
// # Idempotency and retries
//
// Auto-idempotency is on by default; the SDK emits an X-Idempotency
// header per unsafe request. Override per-call via
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey]
// (caller-supplied key) or
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithoutAutoIdempotency]
// (suppression). Disable globally via [WithIdempotency](false). Retries
// follow the default exponential-backoff policy on 5xx + 408/425/429 +
// transport errors; customize via [WithRetryOptions] /
// [WithCustomRetryPolicy] / [WithoutRetries]. See examples/06-idempotency
// and examples/07-retries.
//
// # Examples
//
// See examples/ for runnable demos. Start with examples/01-hello-world
// for the minimum-viable shape; examples/03-end-to-end walks the full
// resource hierarchy. See docs/v3-dx-plan.md for the v3 design rationale.
package midaz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
)

// Version is the current version of the SDK.
// This is automatically updated during the release process.
const Version = "1.1.0-beta.2"

// Client is the main entry point for using the Midaz API.
// It provides access to all API services, connection management,
// authentication, rate limiting, and retry handling.
//
// All services are exposed as promoted fields via the embedded *entities.Entity.
// In v3, prefer c.Accounts.X over c.Entity.Accounts.X — they refer to the same
// instance, but the shorter form is the canonical idiom. The embedded Entity
// pointer remains accessible as c.Entity for back-compat during the v2 → v3
// migration window.
type Client struct {
	// Configuration
	config *config.Config
	ctx    context.Context

	// Embedded Entity. Promoted fields expose every service directly on Client:
	//   c.Accounts, c.Transactions, c.Ledgers, c.Organizations, etc.
	// The embedded pointer is also accessible as c.Entity for back-compat.
	*entities.Entity

	// tenantID is the default tenant identifier sent as X-Tenant-ID on every request.
	// Per-request overrides via sdkctx.WithRequestTenantID(ctx, id) take precedence.
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

	// retryOpts is the user-supplied retry.Option chain accumulated by
	// WithRetryOptions calls. Threaded onto the entity HTTPClient AFTER the
	// config-derived seeds (MaxRetries / RetryWaitMin / RetryWaitMax) so the
	// last write wins per the override-on-conflict contract.
	retryOpts []retry.Option

	// logger is the canonical structured logger for the SDK. Always non-nil
	// after New(). Default: slog.New(slog.DiscardHandler) (silent). Configure
	// via WithLogger(...). When Config.Debug is true and WithLogger was not
	// called, midaz.New() installs a stderr text handler at debug level —
	// user-supplied loggers always win over the MIDAZ_DEBUG-driven default.
	logger *slog.Logger

	// loggerSet tracks whether WithLogger was explicitly called. Used to
	// decide whether the Config.Debug-driven default handler should replace
	// the discard default at construction time.
	loggerSet bool

	// slowCallThreshold is the duration above which a successful API call
	// emits a Warn-level structured log line. Zero (default) means no
	// slow-call warnings. Configure via WithSlowCallThreshold(...).
	slowCallThreshold time.Duration
}

// New creates a new Midaz client with the provided options.
//
// New validates configuration eagerly. If any required field is missing or any
// option fails, it returns a typed configuration error
// (see [pkg/errors.IsConfigurationError]) so callers can distinguish setup
// mistakes from runtime API failures. The "naked SDK" footgun where
// c.Entity could be nil after construction is gone in v3 — every service is
// initialized and ready to use upon successful return.
//
// Default observability provider: New always installs a default
// [observability.Provider] with all three OTel components (tracing, metrics,
// logging) DISABLED. [Client.GetObservabilityProvider] therefore always
// returns a non-nil provider, even if no With*Observability option was
// passed. This default is REPLACED wholesale by the first call to
// [WithObservabilityOptions] or [WithObservabilityProvider] in the option
// chain — there is no merge step. See those options' godoc for replacement
// semantics.
//
// Default logger: New installs a silent [slog.Logger] backed by
// [slog.DiscardHandler] unless [WithLogger] was passed. When [WithLogger]
// is absent and [Config.Debug] is true (e.g. via MIDAZ_DEBUG=true with
// [config.FromEnvironment]), the default is upgraded to a stderr text
// handler at debug level. User-supplied loggers via [WithLogger] always
// win over both defaults.
//
// Returns:
//   - *Client: A fully-initialized client. All service fields (c.Accounts,
//     c.Transactions, etc.) are non-nil and ready for API calls.
//   - error: A *errors.Error with Category=CategoryConfiguration when New
//     cannot construct a usable client. Use errors.Is(err, errors.ErrConfiguration)
//     or errors.IsConfigurationError(err) to check.
//
// See also:
//   - [WithAccessManager], [WithAnonymous] — required auth source.
//   - [WithEnvironment] — pin a deployment environment.
//   - [WithLogger] — wire a *slog.Logger.
//   - [Client.Shutdown] — graceful teardown (call via defer).
//   - examples/01-hello-world — the smallest possible demo.
//   - examples/03-end-to-end — full resource hierarchy walk.
func New(options ...Option) (*Client, error) {
	const operation = "midaz.New"

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
		return nil, sdkerrors.NewConfigurationError(operation, "failed to initialize observability provider", err)
	}

	c.observability = obsProvider

	// Create default configuration
	c.config = config.DefaultConfig()

	// Apply all options. Index nil options in the error message so callers
	// can identify exactly which option in their list is the culprit.
	for i, option := range options {
		if option == nil {
			return nil, sdkerrors.NewConfigurationError(
				operation,
				fmt.Sprintf("option at index %d is nil", i),
				nil,
			)
		}

		if err := option(c); err != nil {
			return nil, sdkerrors.NewConfigurationError(
				operation,
				fmt.Sprintf("option at index %d failed to apply", i),
				err,
			)
		}
	}

	// Eager validation: enforce that the merged config has every required
	// field populated. This catches misconfigurations at New() time instead
	// of letting them surface as cryptic 401/connection-refused errors on
	// the first API call.
	if err := c.config.Validate(); err != nil {
		return nil, sdkerrors.NewConfigurationError(operation, "invalid configuration", err)
	}

	// Resolve the default logger. WithLogger always wins; otherwise the
	// MIDAZ_DEBUG-driven path installs a stderr text handler at debug
	// level (only when Config.Debug=true and FromEnvironment opted in);
	// otherwise the SDK is silent (discard handler).
	c.logger = resolveLogger(c.logger, c.loggerSet, c.config.Debug)

	// Always initialize the Entity surface. The "naked SDK" footgun
	// (c.Entity == nil after New) is gone in v3.
	if err := c.setupEntity(); err != nil {
		return nil, sdkerrors.NewConfigurationError(operation, "failed to initialize entity API", err)
	}

	return c, nil
}

// resolveLogger applies the v3 logger-priority rule:
//
//  1. If WithLogger was explicitly called (loggerSet=true), use that logger
//     (even if it's nil — caller asked for silence).
//  2. Else if Config.Debug is true (typically via FromEnvironment loading
//     MIDAZ_DEBUG=true), install a stderr text handler at debug level.
//  3. Else: discard handler (silent default).
//
// Exposed for tests that exercise the priority rule without going through New().
func resolveLogger(explicit *slog.Logger, explicitSet bool, debugFromConfig bool) *slog.Logger {
	if explicitSet {
		if explicit == nil {
			return slog.New(slog.DiscardHandler)
		}

		return explicit
	}

	if debugFromConfig {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	return slog.New(slog.DiscardHandler)
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

	// Reconcile the tenant ID before constructing the Entity. The client-level
	// override (set via WithTenantID) wins over the config/env default; we
	// mutate the config copy so entities.NewEntityWithConfig sees one
	// consistent source of truth via Config.GetTenantID().
	if c.tenantIDSet {
		c.config.TenantID = c.tenantID
	}

	// Construct the Entity from the resolved Config. Post-construction tuning
	// (observability, debug, user-agent, idempotency, retries, logger,
	// slow-call threshold, custom retry policy) flows through dedicated
	// setters below — Batch 6B retired the entities.Option indirection.
	entity, err := entities.NewEntityWithConfig(c.config)
	if err != nil {
		return err
	}

	if err := entity.SetObservability(c.observability); err != nil {
		return fmt.Errorf("failed to install observability provider: %w", err)
	}

	httpClient := entity.GetEntityHTTPClient()
	httpClient.SetDebug(c.config.Debug)
	httpClient.SetUserAgent(c.config.UserAgent)
	httpClient.SetEnableIdempotency(c.config.EnableIdempotency)

	// Retry chain construction: config seeds first, user overrides last.
	// Override-on-conflict semantics — see [WithRetryOptions] godoc.
	// MaxRetries == 0 (set via [WithoutRetries] or pkg/config.WithMaxRetries(0))
	// flows through naturally; no separate enable flag is consulted.
	retryChain := append(
		[]retry.Option{
			retry.WithMaxRetries(c.config.MaxRetries),
			retry.WithInitialDelay(c.config.RetryWaitMin),
			retry.WithMaxDelay(c.config.RetryWaitMax),
		},
		c.retryOpts...,
	)
	httpClient.WithRetryOptions(retryChain...)

	if c.customRetryPolicy != nil {
		httpClient.SetCustomRetryPolicy(c.customRetryPolicy)
	}

	// Push the resolved logger and slow-call threshold into the entity
	// HTTP client BEFORE InitServices, so per-service HTTP clients
	// inherit the values via the snapshot/applyConfigurationFrom path.
	httpClient.SetLogger(c.logger)
	httpClient.SetSlowCallThreshold(c.slowCallThreshold)

	entity.InitServices()

	c.Entity = entity

	return nil
}

// WithBaseURL sets the base URL for API requests.
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithBaseURL], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// Parameters:
//   - baseURL: The base URL for API requests (e.g. "https://api.midaz.io").
//
// Returns:
//   - Option: A function that sets the base URL on the Client
//
// See also:
//   - [WithEnvironment] — preferred for production stacks.
//   - [WithOnboardingURL], [WithTransactionURL], [WithCRMURL] — per-service overrides.
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithTimeout], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithUserAgent], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		return config.WithUserAgent(userAgent)(c.config)
	}
}

// WithRetryOptions threads pkg/retry tuning knobs onto the entity HTTPClient
// after construction. Use this to override the defaults seeded from
// [pkg/config.WithMaxRetries] / [pkg/config.WithRetryWaitMin] /
// [pkg/config.WithRetryWaitMax], or to set knobs that have no Config
// counterpart (BackoffFactor, JitterFactor, RetryableErrors, RetryableHTTPCodes).
//
// Semantics: override-on-conflict. Config-derived knobs (MaxRetries,
// InitialDelay=RetryWaitMin, MaxDelay=RetryWaitMax) are applied first during
// [setupEntity]; any retry.Option passed here runs afterward and the last
// write wins. Equivalently, the chain is:
//
//	retry.WithMaxRetries(c.config.MaxRetries),     // from Config
//	retry.WithInitialDelay(c.config.RetryWaitMin), // from Config
//	retry.WithMaxDelay(c.config.RetryWaitMax),     // from Config
//	opts...,                                       // user-supplied here
//
// [WithoutRetries] is implemented as a sugar that prepends
// retry.WithMaxRetries(0); pass WithRetryOptions(retry.WithMaxRetries(N))
// after WithoutRetries to re-enable.
//
// Example:
//
//	client, _ := midaz.New(
//	    midaz.WithEnvironment(midaz.EnvironmentLocal),
//	    midaz.WithRetryOptions(
//	        retry.WithMaxRetries(5),
//	        retry.WithJitterFactor(0.4),
//	        retry.WithRetryableHTTPCodes([]int{408, 429, 500, 502, 503, 504}),
//	    ),
//	)
//
// Or use a preset:
//
//	midaz.WithRetryOptions(retry.WithHighReliability())
//
// Returns:
//   - Option: A function that appends the retry options to the Client's pending chain
//
// See also:
//   - [WithCustomRetryPolicy] — replace the policy with an arbitrary predicate.
//   - [WithoutRetries] — disable retries for this client.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry] — option catalog.
//   - examples/07-retries — runnable demo.
func WithRetryOptions(opts ...retry.Option) Option {
	return func(c *Client) error {
		c.retryOpts = append(c.retryOpts, opts...)
		return nil
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
//
// See also:
//   - [WithRetryOptions] — tune the default policy without replacing it.
//   - [WithoutRetries] — disable retries entirely.
//   - examples/07-retries — runnable demo.
func WithCustomRetryPolicy(shouldRetry func(*http.Response, error) bool) Option {
	return func(c *Client) error {
		c.customRetryPolicy = shouldRetry

		if c.Entity != nil {
			httpClient := c.GetEntityHTTPClient()
			if httpClient != nil {
				httpClient.SetCustomRetryPolicy(shouldRetry)
			}
		}

		return nil
	}
}

// WithoutRetries is the canonical off-switch for the retry mechanism.
// It pins MaxRetries to 0 on the Config; downstream HTTP calls execute
// exactly once with no automatic retry on transient failures.
//
// Soft-disable semantics: WithoutRetries simply sets MaxRetries=0. A
// subsequent [WithRetryOptions](retry.WithMaxRetries(N)) in the same
// option chain will re-enable retries with N attempts. Last write wins.
// Use this when you want a default-off posture that test code or callers
// can still override.
//
// Use cases:
//   - Tests that must never retry (combine with no override).
//   - Callers that handle their own retry logic at a higher layer.
//
// Returns:
//   - Option: A function that disables retries on the Client
//
// See also:
//   - [WithRetryOptions], [WithCustomRetryPolicy] — alternatives.
//   - examples/07-retries — runnable demo.
func WithoutRetries() Option {
	return func(c *Client) error {
		return config.WithMaxRetries(0)(c.config)
	}
}

// WithObservabilityOptions builds a fresh observability provider from the
// supplied [observability.Option] chain and installs it on the Client. This
// is the canonical entry point for configuring tracing/metrics/logging via
// the OTel-aligned pkg/observability surface.
//
// Replacement semantics: WithObservabilityOptions REPLACES any provider
// previously installed on this Client — including the default disabled
// provider that [New] installs at construction time (see [New] godoc).
// Subsequent WithObservabilityOptions calls likewise replace. There is no
// composition or merge step; the last call wins. To start from a known set
// of defaults, include [observability.WithDevelopmentDefaults] or
// [observability.WithProductionDefaults] as the first item in the chain.
//
// If the resulting provider IsEnabled, a [observability.MetricsCollector] is
// constructed and made available via [Client.GetMetricsCollector]. The
// Client's context is also updated so [observability.WithProvider] is
// reachable on every per-request context derived from [Client.GetContext].
//
// Use this when you need full control over the provider construction:
// custom service name, custom collector endpoint, sample rate, attributes,
// propagators, log level, log output, or component toggles.
//
// Example — full-tracing dev setup with custom collector:
//
//	client, _ := midaz.New(
//	    midaz.WithObservabilityOptions(
//	        observability.WithServiceName("my-service"),
//	        observability.WithCollectorEndpoint("localhost:4317"),
//	        observability.WithComponentEnabled(true, true, true),
//	        observability.WithFullTracingSampling(),
//	    ),
//	)
//
// Example — install a known set of dev defaults then tweak one knob:
//
//	midaz.WithObservabilityOptions(
//	    observability.WithDevelopmentDefaults(),
//	    observability.WithServiceName("my-service"),
//	)
//
// For sharing a pre-built provider across multiple clients, prefer
// [WithObservabilityProvider] which skips the construction step.
//
// Parameters:
//   - options: The [observability.Option] chain used to build the provider
//
// Returns:
//   - Option: A function that installs the new provider on the Client
//
// See also:
//   - [WithObservabilityProvider] — pass a pre-built provider.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability] — option catalog.
//   - examples/10-observability-otel — runnable demo.
func WithObservabilityOptions(options ...observability.Option) Option {
	return func(c *Client) error {
		// Build the provider from the supplied chain. Any previously
		// installed provider (default-disabled from New, or a prior
		// WithObservabilityOptions / WithObservabilityProvider call) is
		// replaced wholesale — see godoc for replacement semantics.
		provider, err := observability.New(c.ctx, options...)
		if err != nil {
			return err
		}

		c.observability = provider

		if provider.IsEnabled() {
			c.metrics, err = observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}
		}

		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}
}

// WithObservabilityProvider installs a pre-built [observability.Provider]
// on the Client. Use this when you want to share an observability provider
// across multiple Midaz clients (e.g. one provider, many tenant-scoped
// clients) or when the provider was constructed elsewhere in your
// application's bootstrap.
//
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithObservabilityProvider],
// which most callers should not invoke directly. Prefer this option when
// constructing the client via [New].
//
// Replacement semantics: WithObservabilityProvider REPLACES any provider
// previously installed on this Client — including the default disabled
// provider that [New] installs at construction time. See [New] godoc.
//
// Nil handling: a typed-nil [observability.Provider] (e.g. (*Provider)(nil))
// returns an error; a literal nil interface is treated as a no-op and the
// existing provider is preserved.
//
// Parameters:
//   - provider: The pre-built observability provider to install
//
// Returns:
//   - Option: A function that installs the provider on the Client
//
// See also:
//   - [WithObservabilityOptions] — build a provider inline.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability.Provider]
//   - examples/10-observability-otel — runnable demo.
func WithObservabilityProvider(provider observability.Provider) Option {
	return func(c *Client) error {
		if provider == nil {
			return nil
		}

		if reflectutil.IsTypedNil(provider) {
			return errors.New("observability provider cannot be nil")
		}

		// Replace any previously installed provider (default-disabled or
		// otherwise). See godoc for replacement semantics.
		c.observability = provider

		if provider.IsEnabled() {
			var err error

			c.metrics, err = observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}
		}

		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}
}

// WithEnvironment sets the environment for the client.
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithEnvironment], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// This is used for configuration options that vary by environment.
//
// Parameters:
//   - env: The environment to use
//
// Returns:
//   - Option: A function that sets the environment on the Client
//
// See also:
//   - [EnvironmentLocal], [EnvironmentDevelopment], [EnvironmentProduction] — the three values.
//   - [WithBaseURL] — for self-hosted stacks not covered by the standard environments.
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithHTTPClient], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithOnboardingURL], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithTransactionURL], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithCRMURL], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// This overrides any URL derived from the Environment setting.
func WithCRMURL(crmURL string) Option {
	return func(c *Client) error {
		return config.WithCRMURL(crmURL)(c.config)
	}
}

// WithDebug enables or disables debug mode.
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithDebug], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// In debug mode, the SDK logs detailed information about requests and responses.
//
// Parameters:
//   - enabled: Whether to enable debug mode
//
// Returns:
//   - Option: A function that sets the debug flag on the Client
func WithDebug(enabled bool) Option {
	return func(c *Client) error {
		return config.WithDebug(enabled)(c.config)
	}
}

// WithIdempotency enables or disables automatic idempotency-key generation
// for unsafe HTTP methods (POST, PUT, PATCH, DELETE). When enabled, the SDK
// attaches an X-Idempotency header derived from a UUID to each unsafe
// request unless [entities.WithIdempotencyKey] was used to set an explicit
// key on the per-request context.
//
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithIdempotency],
// which most callers should not invoke directly. Prefer this option when
// constructing the client via [New].
//
// Default: enabled (Config.EnableIdempotency = DefaultEnableIdempotency = true).
// Disable when you have an upstream gateway that handles idempotency, or
// when running tests that assert exact request bodies.
//
// Parameters:
//   - enabled: Whether to enable automatic idempotency-key generation
//
// Returns:
//   - Option: A function that sets the idempotency flag on the Client
//
// See also:
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey] — caller-supplied key for one request.
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithoutAutoIdempotency] — per-call suppression.
//   - examples/06-idempotency — runnable demo.
func WithIdempotency(enabled bool) Option {
	return func(c *Client) error {
		return config.WithIdempotency(enabled)(c.config)
	}
}

// WithTenantID sets the default tenant ID for all API requests made through this client.
// The tenant ID is sent as the X-Tenant-ID header on every request.
// Per-request overrides via sdkctx.WithRequestTenantID(ctx, tenantID) take precedence
// over this client-level default. This is an optional compatibility signal for
// deployments that honor the header, not a replacement for tenant resolution from
// authenticated claims.
//
// Parameters:
//   - tenantID: The tenant identifier to use
//
// Returns:
//   - Option: A function that sets the tenant ID on the Client
//
// See also:
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithRequestTenantID] — per-request override.
//   - docs/multi-tenancy.md — full multi-tenant routing contract.
func WithTenantID(tenantID string) Option {
	return func(c *Client) error {
		c.tenantID = strings.TrimSpace(tenantID)
		c.tenantIDSet = true

		return nil
	}
}

// WithAccessManager configures plugin-based authentication via the Lerian
// Access Manager service. The supplied AccessManager must have Address,
// ClientID, and ClientSecret populated; the Enabled field is auto-set to
// true (the act of calling this option is the opt-in).
//
// Example:
//
//	c, err := midaz.New(
//	    midaz.WithEnvironment(midaz.EnvironmentProduction),
//	    midaz.WithAccessManager(midaz.AccessManager{
//	        Address:      "https://auth.midaz.io",
//	        ClientID:     "abc",
//	        ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
//	    }),
//	)
//
// WithAccessManager and [WithAnonymous] are mutually exclusive — applying
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAccessManager], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// one clears the other. v3 requires exactly one auth source at construction
// time; without either, midaz.New() returns a configuration error.
//
// See docs/auth.md for the full setup walkthrough.
//
// Parameters:
//   - am: AccessManager configuration. Address, ClientID, ClientSecret are
//     all required; Enabled is auto-populated.
//
// Returns:
//   - Option: a function that wires AccessManager onto the underlying Config.
//
// See also:
//   - [WithAnonymous] — opt out of authentication (local stacks only).
//   - [AccessManager] — the credential bag.
//   - docs/auth.md — authentication setup walkthrough.
//   - examples/02-auth — runnable demo.
func WithAccessManager(am AccessManager) Option {
	return func(c *Client) error {
		return config.WithAccessManager(am)(c.config)
	}
}

// WithAnonymous explicitly opts the client out of authentication. This is
// the only sanctioned way to construct a client without credentials in v3 —
// without WithAnonymous AND without [WithAccessManager], midaz.New()
// returns a typed configuration error of the form
//
//	"no auth source configured; use WithAccessManager or WithAnonymous"
//
// Use cases: local development against an unsecured midaz stack, integration
// tests against testcontainers, or read-only inspection where the operator
// has confirmed the target endpoints don't require auth.
//
// WithAnonymous and [WithAccessManager] are mutually exclusive — applying
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAnonymous], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// one clears the other.
//
// Returns:
//   - Option: a function that flags the underlying Config as deliberately
//     auth-less so validation accepts it.
//
// See also:
//   - [WithAccessManager] — production-shape OAuth via Lerian Access Manager.
//   - docs/auth.md — when to use anonymous vs authenticated.
func WithAnonymous() Option {
	return func(c *Client) error {
		return config.WithAnonymous()(c.config)
	}
}

// WithLogger sets the canonical *slog.Logger for the client. Once configured,
// the SDK emits structured log lines for retry attempts, slow calls, and
// internal warnings through this logger.
//
// The SDK is silent by default (discard handler). Pass WithLogger to opt in.
// When both WithLogger and Config.Debug=true (typically via FromEnvironment)
// are present, WithLogger always wins — the MIDAZ_DEBUG bypass that existed
// in v2 is gone in v3.
//
// Integrations (paired with WithAnonymous for brevity in these snippets;
// production setups would supply WithAccessManager):
//
//	// stdlib slog with JSON to stdout
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	c, _ := midaz.New(midaz.WithLogger(logger), midaz.WithAnonymous())
//
//	// charmbracelet/log
//	import charm "github.com/charmbracelet/log"
//	clog := charm.NewWithOptions(os.Stderr, charm.Options{Level: charm.DebugLevel})
//	c, _ := midaz.New(midaz.WithLogger(slog.New(clog)), midaz.WithAnonymous())
//
//	// zap via slog adapter (Go 1.22+)
//	import "go.uber.org/zap/exp/zapslog"
//	zl, _ := zap.NewProduction()
//	c, _ := midaz.New(
//	    midaz.WithLogger(slog.New(zapslog.NewHandler(zl.Core(), nil))),
//	    midaz.WithAnonymous(),
//	)
//
// Passing nil clears any previously-configured logger and reverts to the
// silent discard default.
//
// Returns:
//   - Option: A function that sets the logger on a Client.
//
// See also:
//   - [WithSlowCallThreshold] — emit a warn-level record for slow calls.
//   - docs/logging.md — logging contract and adapter recipes (zap, zerolog, logrus).
//   - examples/08-logging-slog — runnable demo.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) error {
		c.logger = logger
		c.loggerSet = true

		return nil
	}
}

// WithSlowCallThreshold configures the duration above which a successful API
// call triggers a Warn-level structured log line on the configured logger.
// The line includes operation, http.method, url.path, http.status_code,
// duration_ms, and request_id when available.
//
// Zero (default) disables slow-call warnings.
//
// Negative values are coerced to zero (disabled). Setting a positive
// threshold without WithLogger is harmless — the warning lands on the
// discard handler.
//
// Returns:
//   - Option: A function that sets the slow-call threshold on a Client.
func WithSlowCallThreshold(threshold time.Duration) Option {
	return func(c *Client) error {
		if threshold < 0 {
			threshold = 0
		}

		c.slowCallThreshold = threshold

		return nil
	}
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

// Logger returns the canonical *slog.Logger for this client. The return value
// is always non-nil — when no WithLogger was configured and Config.Debug is
// false, the logger is wired to a discard handler (silent).
//
// Use this to emit application-side log lines that should follow the same
// handler as SDK-internal lines, or to inspect the configured logger in tests.
//
// In v3 the return type changed from observability.Logger to *slog.Logger.
// Code that needs the bespoke observability.Logger interface should reach
// for c.GetObservabilityProvider().Logger() instead.
//
// Returns:
//   - *slog.Logger: A non-nil logger. Discard by default.
func (c *Client) Logger() *slog.Logger {
	if c == nil || c.logger == nil {
		// Defensive: a properly-constructed client always has a non-nil
		// logger after New(). This branch only triggers if the caller
		// constructed a Client struct directly (which is unsupported).
		return slog.New(slog.DiscardHandler)
	}

	return c.logger
}

// GetObservabilityProvider returns the observability provider installed on
// this Client. The return value is never nil for a Client constructed via
// [New]: if no [WithObservabilityOptions] or [WithObservabilityProvider]
// option was passed, [New] installs a default provider with all three OTel
// components (tracing, metrics, logging) disabled. Use
// [observability.Provider.IsEnabled] to distinguish the default-disabled
// provider from a user-installed enabled one.
//
// Returns:
//   - Provider: The observability provider (never nil after [New])
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
