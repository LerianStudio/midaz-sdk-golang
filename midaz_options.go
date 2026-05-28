package midaz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
)

// markConfigMutated records that a config-mutating option has run. WithConfig
// inspects this flag to refuse a mid-chain replacement that would silently
// void earlier mutations — see WithConfig godoc.
func (c *Client) markConfigMutated() {
	c.configMutated = true
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
//   - [WithLedgerURL], [WithCRMURL] — per-service overrides.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		// Validate URL
		_, err := url.Parse(baseURL)
		if err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}

		c.markConfigMutated()
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
		c.markConfigMutated()
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
		c.markConfigMutated()
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
//	        retry.WithRetryableHTTPCodes([]int{408, 425, 429, 500, 502, 503, 504}),
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
		if err := validateRetryOptions(opts...); err != nil {
			return err
		}

		c.retryOpts = append(c.retryOpts, opts...)
		return nil
	}
}

func validateRetryOptions(opts ...retry.Option) error {
	retryOpts := retry.DefaultOptions()
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("retry option at index %d cannot be nil", i)
		}

		if err := opt(retryOpts); err != nil {
			return fmt.Errorf("retry option at index %d failed: %w", i, err)
		}
	}

	return nil
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
		c.markConfigMutated()
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

		c.pendingObservability = provider

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
		c.pendingObservability = provider

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
		c.markConfigMutated()
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
// # Ordering rule
//
// WithConfig replaces the entire client configuration in one shot. If any
// other config-mutating option (WithBaseURL, WithUserAgent, WithEnvironment,
// WithDebug, WithIdempotency, WithErrorBodyExposure, WithoutRetries, …) ran before WithConfig in
// the option chain, those mutations are silently voided by the replacement.
// To prevent that footgun, WithConfig errors loudly when invoked after
// another config-mutating option has already run.
//
// Place WithConfig FIRST in the option chain — or omit per-knob options
// entirely and configure the *config.Config directly before passing it in.
// Per-knob options that have NO Config counterpart (WithLogger,
// WithRetryOptions, WithSlowCallThreshold, WithCustomRetryPolicy, …) can
// freely run on either side of WithConfig because they don't touch
// c.config.
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

		if c.configMutated {
			return errors.New("WithConfig must come before any other config-mutating option (WithBaseURL, WithUserAgent, WithEnvironment, WithDebug, WithIdempotency, WithErrorBodyExposure, WithoutRetries, and other config-mutating options …) — placing it later silently voids those mutations")
		}

		c.config = cfg.Clone()
		c.markConfigMutated()

		if provider := c.config.GetObservabilityProvider(); provider != nil && !reflectutil.IsTypedNil(provider) {
			c.pendingObservability = provider
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
		c.markConfigMutated()
		return config.WithHTTPClient(client)(c.config)
	}
}

// WithLedgerURL sets the URL for the Ledger API. The Ledger service serves
// both onboarding and transaction endpoints under the same plane.
//
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithLedgerURL], which most
// callers should not invoke directly. Prefer this option when constructing the
// client via [New].
//
// This overrides any URL derived from the Environment setting.
//
// Parameters:
//   - ledgerURL: The URL for the Ledger API
//
// Returns:
//   - Option: A function that sets the Ledger URL on the Client
func WithLedgerURL(ledgerURL string) Option {
	return func(c *Client) error {
		c.markConfigMutated()
		return config.WithLedgerURL(ledgerURL)(c.config)
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
		c.markConfigMutated()
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
		c.markConfigMutated()
		return config.WithDebug(enabled)(c.config)
	}
}

// WithErrorBodyExposure enables or disables raw upstream error response body
// exposure on SDK errors. When enabled, upstream 4xx/5xx response bodies are
// attached without redaction and only truncated.
func WithErrorBodyExposure(enabled bool) Option {
	return func(c *Client) error {
		c.markConfigMutated()
		return config.WithErrorBodyExposure(enabled)(c.config)
	}
}

// WithIdempotency enables or disables automatic idempotency-key generation
// for unsafe HTTP methods (POST, PUT, PATCH, DELETE). When enabled, the SDK
// attaches an X-Idempotency header derived from a UUID to each unsafe
// request unless [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey] was used to set an explicit
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
		c.markConfigMutated()
		return config.WithIdempotency(enabled)(c.config)
	}
}

// WithAccessManager configures plugin-based authentication via the Lerian
// Access Manager service. The supplied AccessManager must have Address,
// ClientID, and ClientSecret populated; the Enabled field is auto-set to
// true (the act of calling this option is the opt-in).
//
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAccessManager],
// which most callers should not invoke directly. Prefer this option when
// constructing the client via [New].
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
// one clears the other (last write wins). v3 requires exactly one auth source
// at construction time; without either, midaz.New() returns a configuration
// error.
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
		c.markConfigMutated()
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
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAnonymous],
// which most callers should not invoke directly. Prefer this option when
// constructing the client via [New].
//
// Use cases: local development against an unsecured midaz stack, integration
// tests against testcontainers, or read-only inspection where the operator
// has confirmed the target endpoints don't require auth.
//
// WithAnonymous and [WithAccessManager] are mutually exclusive — applying
// one clears the other (last write wins).
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
		c.markConfigMutated()
		return config.WithAnonymous()(c.config)
	}
}

// WithAllowInsecureAccessManagerHTTP opts the client into accepting plain
// http:// Access Manager URLs even for non-loopback hosts. The default is
// strict (HTTPS or loopback only); flipping this true bypasses transport
// security on the credential-bearing token-exchange request.
//
// Two-layer surface: this is the user-facing wrapper. It delegates to
// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAllowInsecureAccessManagerHTTP],
// which most callers should not invoke directly.
//
// SECURITY: this disables a deliberate construction-time security gate.
// Production deployments must leave this off (the default). The flag
// exists for the canonical in-cluster Kubernetes pattern where the Access
// Manager is reached via a Service DNS name such as
// http://plugin-access-manager-auth.midaz-plugins.svc.cluster.local:4000,
// where the transport is already protected by a service mesh or trusted
// network segment.
//
// When this option is set to true, midaz.New emits a Warn-level log line
// at construction so the override is auditable in deployment logs:
//
//	"Access Manager configured with insecure HTTP. Only valid for trusted
//	 in-cluster networks. Production deployments must use HTTPS."
//
// Equivalent env var: MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP=true (consumed
// via [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.FromEnvironment]).
//
// Parameters:
//   - allow: Whether to permit plain http:// for non-loopback Access
//     Manager URLs.
//
// Returns:
//   - Option: A function that wires the flag onto the underlying Config.
//
// See also:
//   - [WithAccessManager] — the auth source this flag relaxes scheme rules for.
func WithAllowInsecureAccessManagerHTTP(allow bool) Option {
	return func(c *Client) error {
		c.markConfigMutated()
		return config.WithAllowInsecureAccessManagerHTTP(allow)(c.config)
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
