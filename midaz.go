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
//	org, err := c.Organizations.Get(ctx, "org-id")
//
// # Authentication
//
// v4 requires exactly one auth source at construction time:
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
// Tenant scope is derived from Access Manager/JWT claims. The SDK does not
// expose or send X-Tenant-ID. Use separate Access Manager credentials or token
// context when calls must run under a different tenant scope.
// See docs/multi-tenancy.md.
//
// # Logging and observability
//
// Inject a *slog.Logger via [WithLogger]. Wire OpenTelemetry via
// [WithObservabilityProvider] or [WithObservabilityOptions]. The SDK is
// silent by default (slog.DiscardHandler). See docs/logging.md and the
// observability examples.
//
// # Pagination
//
// Every paginated entity List* method returns one page. ListAll yields
// iter.Seq2[T, error] for full-collection iteration; ListPages yields page
// envelopes with metadata. MetadataIndexes is intentionally non-paginated.
// Page-based and cursor-based endpoints are distinguished at the type system:
// wrong-shape opts don't compile. See docs/pagination.md.
//
// # Errors
//
// Every error returned by SDK code is a *[github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors.Error]
// with structured Category, Code, Operation, and Resource fields. Use
// errors.Is, errors.As, and the typed predicates (IsNotFoundError,
// IsValidationError, IsNetworkError, IsAuthError, etc.). The Retryable()
// method on *Error is the canonical retry-policy source.
//
// # Idempotency and retries
//
// Auto-idempotency is on by default; the SDK emits an X-Idempotency
// header per unsafe request. Override per-call via
// [github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx.WithIdempotencyKey]
// (caller-supplied key) or
// [github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx.WithoutAutoIdempotency]
// (suppression). Disable globally via [WithIdempotency](false). Unsafe
// requests retry only when X-Idempotency is present; caller-supplied and
// SDK-generated keys both satisfy that gate. Retries follow the default
// exponential-backoff policy on 5xx + 408/425/429 + transport errors; customize via [WithRetryOptions] /
// [WithCustomRetryPolicy] / [WithoutRetries]. See examples/06-idempotency
// and examples/07-retries.
//
// # Examples
//
// See examples/ for runnable demos. Start with examples/01-hello-world
// for the minimum-viable shape; examples/03-end-to-end walks the full
// resource hierarchy.
package midaz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/version"
)

// Version is the current version of the SDK. It mirrors the single source of
// truth in pkg/version so midaz.Version and version.Version never drift.
const Version = version.Version

// Client is the main entry point for using the Midaz API.
// It provides access to all API services, connection management,
// authentication, rate limiting, and retry handling.
//
// All services are exposed as promoted fields via the embedded *entities.Entity.
// In v4, prefer c.Accounts.X over c.Entity.Accounts.X — they refer to the same
// instance, but the shorter form is the canonical idiom. The embedded Entity
// pointer remains accessible as c.Entity for back-compat during the v2 → v4
// migration window.
//
// Client wraps a small subset of Entity methods (SetObservability,
// GetObservabilityProvider) so the Client view of state never drifts from the
// Entity view — the Entity is the single source of truth post-construction.
type Client struct {
	// Configuration
	config *config.Config
	ctx    context.Context

	// configMutated tracks whether any config-mutating option has run.
	// WithConfig errors loudly if invoked after another option has already
	// mutated c.config — see WithConfig godoc for the rationale.
	configMutated bool

	// Embedded Entity. Promoted fields expose every service directly on Client:
	//   c.Accounts, c.Transactions, c.Ledgers, c.Organizations, etc.
	// The embedded pointer is also accessible as c.Entity for back-compat.
	*entities.Entity

	// pendingObservability is the observability provider accumulated by
	// option-chain calls (WithObservabilityOptions, WithObservabilityProvider,
	// or the disabled default installed by New). It is the staging buffer
	// used during construction. Post-construction, the embedded *Entity is the
	// single source of truth — GetObservabilityProvider reads from it, and
	// SetObservability delegates writes to it.
	pendingObservability observability.Provider
	metrics              *observability.MetricsCollector
	customRetryPolicy    func(*http.Response, error) bool

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

// Option is a functional option for configuring the client.
type Option func(*Client) error

// New creates a new Midaz client with the provided options.
//
// New validates configuration eagerly. If any required field is missing or any
// option fails, it returns a typed error so callers can distinguish setup
// mistakes from runtime API failures. The "naked SDK" footgun where
// c.Entity could be nil after construction is gone in v4 — every service is
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
//
//   - *Client: A fully-initialized client. All service fields (c.Accounts,
//     c.Transactions, etc.) are non-nil and ready for API calls.
//
//   - error: A *errors.Error with a category appropriate to the failure
//     class. The classification space is:
//
//   - Local validation failures (missing fields, invalid URLs, conflicting
//     options) → Category=CategoryConfiguration. Detect with
//     [pkg/errors.IsConfigurationError].
//
//   - Upstream Access Manager HTTP responses → preserve the actual status
//     code: 401 → [pkg/errors.IsAuthenticationError],
//     403 → [pkg/errors.IsAuthorizationError],
//     429 → [pkg/errors.IsRateLimitError],
//     5xx → [pkg/errors.IsInternalError].
//
//   - Pre-response network failures (DNS, conn-refused, TLS handshake) →
//     Category=CategoryNetwork. Detect with [pkg/errors.IsNetworkError].
//
// [pkg/errors.IsAuthError] only matches the 401/403 upstream branches; it
// is NOT a general bootstrap-failure predicate. Use
// [pkg/errors.IsBootstrapError] (introduced in this release) to detect
// the union of all bootstrap-failure categories in one call.
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

	c.pendingObservability = obsProvider

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

	// Audit-trail the insecure-HTTP escape hatch at construction time so
	// the override is visible in deployment logs. The flag is a deliberate
	// security gate disable; emitting a Warn line makes it impossible to
	// flip it on without it showing up in centralised logging.
	if c.config.AccessManager.Enabled && c.config.AccessManager.AllowInsecureHTTP {
		c.logger.Warn(
			"Access Manager configured with insecure HTTP. Only valid for trusted in-cluster networks. Production deployments must use HTTPS.",
			slog.String("sdk.name", "midaz-go-sdk"),
			slog.String("sdk.component", "bootstrap"),
			slog.String("operation", operation),
		)
	}

	// The DATA-PLANE insecure-HTTP flag (WithAllowInsecureHTTP /
	// MIDAZ_ALLOW_INSECURE_HTTP) disables the same gate for the Ledger and
	// Tracer plane URLs. The Bearer token and idempotency keys transit the data
	// plane, so it earns the same auditable Warn as the Access Manager flag.
	if c.config.AllowInsecureHTTP {
		c.logger.Warn(
			"Data plane (Ledger/Tracer) configured with insecure HTTP. Only valid for trusted in-cluster networks. Production deployments must use HTTPS.",
			slog.String("sdk.name", "midaz-go-sdk"),
			slog.String("sdk.component", "bootstrap"),
			slog.String("operation", operation),
		)
	}

	// Always initialize the Entity surface. The "naked SDK" footgun
	// (c.Entity == nil after New) is gone in v4.
	if err := c.setupEntity(); err != nil {
		c.logBootstrapSetupFailure(err)
		return nil, classifyBootstrapSetupError(operation, err)
	}

	return c, nil
}

// resolveLogger applies the v4 logger-priority rule:
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

// isAccessManagerTokenFetchError reports whether err originates from the
// Access Manager token fetch performed during entity construction.
func isAccessManagerTokenFetchError(err error) bool {
	return auth.IsAccessManagerTokenFetchError(err)
}

func isLocalAccessManagerBootstrapFailure(err error) bool {
	var tokenErr *auth.AccessManagerTokenRequestError
	if !errors.As(err, &tokenErr) {
		return false
	}

	return tokenErr.AccessManagerLocalValidationFailed() || !tokenErr.AccessManagerHTTPRequestSent()
}

func classifyBootstrapSetupError(operation string, err error) *sdkerrors.Error {
	// Classify by error shape: local Access Manager request validation is
	// configuration, while an upstream Access Manager HTTP response keeps its
	// original HTTP status/source diagnostics. This prevents bootstrap-time
	// 429/5xx responses from being collapsed into synthetic SDK 401s.
	if isLocalAccessManagerBootstrapFailure(err) {
		return sdkerrors.NewConfigurationError(operation, fmt.Sprintf("invalid Access Manager bootstrap request: %v", err), err)
	}

	if upstreamErr := newAccessManagerUpstreamBootstrapError(operation, err); upstreamErr != nil {
		return upstreamErr
	}

	if isAccessManagerTokenFetchError(err) {
		return sdkerrors.NewNetworkError(
			operation,
			fmt.Errorf("failed to obtain Access Manager token during client construction: %w", err),
		)
	}

	return sdkerrors.NewConfigurationError(operation, "failed to initialize entity API", err)
}

func newAccessManagerUpstreamBootstrapError(operation string, err error) *sdkerrors.Error {
	var tokenErr *auth.AccessManagerTokenRequestError
	if !errors.As(err, &tokenErr) {
		return nil
	}

	if !tokenErr.AccessManagerHTTPRequestSent() || tokenErr.StatusCode() <= 0 {
		return nil
	}

	return sdkerrors.NewUpstreamHTTPError(
		operation,
		"Access Manager token request failed during client construction",
		tokenErr.StatusCode(),
		err,
	)
}

func (c *Client) logBootstrapSetupFailure(err error) {
	logger := c.Logger()

	attrs := []any{
		"sdk.name", "midaz-go-sdk",
		"sdk.component", "bootstrap",
		"operation", "midaz.New",
	}

	var tokenErr *auth.AccessManagerTokenRequestError
	if errors.As(err, &tokenErr) {
		attrs = append(attrs,
			"failure.phase", tokenErr.AccessManagerPhase(),
			"auth.scheme", tokenErr.AccessManagerEndpointScheme(),
			"auth.host", tokenErr.AccessManagerEndpointHost(),
			"auth.path", tokenErr.AccessManagerEndpointPath(),
			"httpRequestSent", tokenErr.AccessManagerHTTPRequestSent(),
			"localValidationFailed", tokenErr.AccessManagerLocalValidationFailed(),
		)

		if reason := tokenErr.AccessManagerValidationReason(); reason != "" {
			attrs = append(attrs, "validationReason", reason)
		}
	} else {
		attrs = append(attrs,
			"failure.phase", "entity_setup",
			"httpRequestSent", false,
			"localValidationFailed", false,
		)
	}

	logger.Error("midaz bootstrap setupEntity failed", attrs...)
}

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

	if err := config.WithObservabilityProvider(c.pendingObservability)(c.config); err != nil {
		return fmt.Errorf("failed to configure observability provider: %w", err)
	}

	// Retry chain construction: config seeds first, user overrides last.
	// Override-on-conflict semantics — see [WithRetryOptions] godoc.
	// MaxRetries == 0 (set via [WithoutRetries] or pkg/config.WithMaxRetries(0))
	// flows through naturally; no separate enable flag is consulted.
	//
	// Built BEFORE Entity construction and resolved ONCE so the plane retry
	// round tripper and the legacy *HTTPClient derive from the same seed. The
	// plane clients are built inside NewEntityWithConfigContext — before the
	// legacy WithRetryOptions/SetCustomRetryPolicy calls below — so the
	// resolved policy must be threaded through construction, otherwise plane
	// money-writes would silently ignore WithRetryOptions/WithCustomRetryPolicy.
	retryChain := append(
		[]retry.Option{
			retry.WithMaxRetries(c.config.MaxRetries),
			retry.WithInitialDelay(c.config.RetryWaitMin),
			retry.WithMaxDelay(c.config.RetryWaitMax),
		},
		c.retryOpts...,
	)

	resolvedRetry, err := resolveRetryOptions(retryChain)
	if err != nil {
		return fmt.Errorf("failed to resolve retry options: %w", err)
	}

	// Construct the Entity from the resolved Config. NewEntityWithConfig
	// runs initServices() internally during construction, seeding every
	// per-service HTTPClient with the entity-level snapshot. The config is
	// wrapped so the plane-client builder can read the resolved retry policy
	// (see [entities.planeRetryConfig]).
	entityConfig := planeRetryConfigWrapper{
		Config:            c.config,
		planeRetryOptions: resolvedRetry,
		planeCustomRetry:  c.customRetryPolicy,
	}

	entity, err := entities.NewEntityWithConfigContext(c.ctx, entityConfig)
	if err != nil {
		return err
	}

	if err := entity.SetObservability(c.pendingObservability); err != nil {
		return fmt.Errorf("failed to install observability provider: %w", err)
	}

	httpClient := entity.GetEntityHTTPClient()
	httpClient.SetDebug(c.config.Debug)
	httpClient.SetUserAgent(c.config.UserAgent)
	httpClient.SetEnableIdempotency(c.config.EnableIdempotency)
	httpClient.SetExposeErrorBody(c.config.ExposeErrorBody)

	// Legacy per-service *HTTPClient path (unchanged): the SAME retryChain
	// feeds WithRetryOptions so the two paths resolve from one seed.
	if err := httpClient.WithRetryOptions(retryChain...); err != nil {
		return fmt.Errorf("failed to configure retry options: %w", err)
	}

	if c.customRetryPolicy != nil {
		httpClient.SetCustomRetryPolicy(c.customRetryPolicy)
	}

	// Push the resolved logger and slow-call threshold into the entity-level
	// HTTPClient. With the v4 per-service HTTPClient consolidation, every
	// service shares this same *HTTPClient instance — there's no per-service
	// snapshot to refresh, so the mutation propagates immediately to every
	// service's next request. The v2-era double-init pattern
	// (NewEntityWithConfig → setters → InitServices) is gone.
	httpClient.SetLogger(c.logger)
	httpClient.SetSlowCallThreshold(c.slowCallThreshold)

	c.Entity = entity

	return nil
}

// resolveRetryOptions folds an option chain onto retry.DefaultOptions() using
// the same apply mechanism as retry.Do and HTTPClient.WithRetryOptions, so the
// plane retry round tripper and the legacy *HTTPClient resolve identical
// effective options from one seed. It is the single conversion point from
// []retry.Option to a concrete retry.Options for plane-client construction.
func resolveRetryOptions(opts []retry.Option) (*retry.Options, error) {
	resolved := retry.DefaultOptions()

	for i, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("retry option at index %d cannot be nil", i)
		}

		if err := opt(resolved); err != nil {
			return nil, fmt.Errorf("retry option at index %d failed: %w", i, err)
		}
	}

	return resolved, nil
}

// planeRetryConfigWrapper adapts *config.Config into an entities.Config that
// ALSO exposes the effective plane retry policy (entities.planeRetryConfig).
// The embedded *config.Config promotes every base and optional Config method
// (GetHTTPClient, GetBaseURLs, GetObservabilityProvider, GetPluginAuth,
// GetTracerAPIKey, GetAllowInsecureHTTP); this wrapper adds only the resolved
// retry policy, which lives on the midaz.Client rather than on config.Config.
type planeRetryConfigWrapper struct {
	*config.Config

	planeRetryOptions *retry.Options
	planeCustomRetry  func(*http.Response, error) bool
}

// GetPlaneRetryOptions returns the effective plane retry policy resolved at
// construction. Never nil in practice (setupEntity always resolves it).
func (w planeRetryConfigWrapper) GetPlaneRetryOptions() *retry.Options {
	return w.planeRetryOptions
}

// GetPlaneCustomRetryPolicy returns the caller-supplied plane custom retry
// policy, or nil when none was configured.
func (w planeRetryConfigWrapper) GetPlaneCustomRetryPolicy() func(*http.Response, error) bool {
	return w.planeCustomRetry
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

	// Read observability via the canonical Entity-backed accessor so the
	// Client/Entity views never disagree. After New() succeeds this is the
	// same provider that's installed on every per-service HTTPClient.
	if provider := c.GetObservabilityProvider(); provider != nil {
		if err := provider.Shutdown(ctx); err != nil {
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

	provider := c.GetObservabilityProvider()
	if provider == nil || !provider.IsEnabled() {
		return fn(c.ctx)
	}

	return observability.WithSpan(c.ctx, provider, name, fn)
}

// Logger returns the canonical *slog.Logger for this client. The return value
// is always non-nil — when no WithLogger was configured and Config.Debug is
// false, the logger is wired to a discard handler (silent).
//
// Use this to emit application-side log lines that should follow the same
// handler as SDK-internal lines, or to inspect the configured logger in tests.
//
// # Two logger surfaces
//
// The SDK has two distinct logger surfaces and they are not the same handler:
//
//   - Client.Logger() — this method. Returns *slog.Logger. The Go-stdlib
//     idiom. Used by the SDK for retry diagnostics and other internal lines
//     that are not span-correlated. Configured via WithLogger or implicitly by
//     Config.Debug. Has its own handler chain.
//   - Provider.Logger() — accessed via c.GetObservabilityProvider().Logger().
//     Returns the bespoke observability.Logger interface. OTel-correlated:
//     attaches trace_id/span_id when used inside a span via WithSpan(span).
//     Configured by WithObservabilityOptions / WithObservabilityProvider.
//
// The two surfaces are intentionally disjoint: slog is the standard-library
// idiom users already configure for their applications; observability.Logger
// predates slog and serves the OTel-correlated path. Most users want
// Client.Logger() for application code; reach for Provider.Logger() only when
// you need span-aware logging within an SDK call.
//
// In v4 the return type changed from observability.Logger to *slog.Logger.
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
// The provider is read from the embedded *Entity, the single source of truth
// post-construction. Calling [Client.SetObservability] updates both the
// Client's view and the Entity's view atomically; the v2 drift footgun where
// Client and Entity diverged after a promoted SetObservability call is gone.
//
// Returns:
//   - Provider: The observability provider (never nil after [New])
func (c *Client) GetObservabilityProvider() observability.Provider {
	if c == nil {
		return nil
	}

	if c.Entity != nil {
		return c.Entity.GetObservabilityProvider()
	}

	// Pre-Entity (e.g. from inside an Option, before setupEntity has run):
	// fall back to the staging buffer. Post-New() reads always go through
	// the Entity branch above.
	return c.pendingObservability
}

// SetObservability installs an observability provider on this Client and
// propagates the change to the embedded Entity (and every per-service
// HTTPClient). Use this when you need to swap the observability provider
// after [New] has already returned — for example, when a deferred
// configuration loader resolves an OTel exporter that wasn't available at
// construction time.
//
// SetObservability replaces the metrics collector if the new provider reports
// IsEnabled() == true. A nil provider is a no-op.
//
// In v4 this is the canonical post-construction observability mutator. It
// supersedes the v2 pattern where the promoted *Entity.SetObservability was
// the only entry point but Client kept its own duplicate observability field —
// callers occasionally hit the drift footgun where Client.GetObservabilityProvider
// returned the stale Client copy. That field is gone; Entity is the single
// source of truth and this method delegates to it.
//
// Parameters:
//   - provider: The observability provider to install. Nil is a no-op.
//
// Returns:
//   - error: Non-nil only if the metrics collector fails to construct.
func (c *Client) SetObservability(provider observability.Provider) error {
	if c == nil {
		return errors.New("client cannot be nil")
	}

	if provider == nil || reflectutil.IsTypedNil(provider) {
		return nil
	}

	// Pre-Entity (called from within an Option before setupEntity ran):
	// stage on the client buffer; setupEntity will install it on the Entity.
	if c.Entity == nil {
		c.pendingObservability = provider

		if provider.IsEnabled() {
			collector, err := observability.NewMetricsCollector(provider)
			if err != nil {
				return err
			}

			c.metrics = collector
		}

		c.ctx = observability.WithProvider(c.ctx, provider)

		return nil
	}

	// Post-Entity: delegate to Entity (which updates HTTPClient state under
	// lock and re-propagates to every per-service HTTPClient). Then refresh
	// the Client-level metrics collector so GetMetricsCollector keeps in sync.
	if err := c.Entity.SetObservability(provider); err != nil {
		return err
	}

	if provider.IsEnabled() {
		collector, err := observability.NewMetricsCollector(provider)
		if err != nil {
			return err
		}

		c.metrics = collector
	}

	c.ctx = observability.WithProvider(c.ctx, provider)

	return nil
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

// GetVersion returns the current version of the SDK.
// This is useful for debugging and tracking the SDK version in logs and traces.
//
// Returns:
//   - string: The current version of the SDK
func (*Client) GetVersion() string {
	return Version
}
