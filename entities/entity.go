// Package entities provides the service interfaces for interacting with the
// Midaz API resources — organizations, ledgers, accounts, assets, balances,
// portfolios, segments, transactions, and CRM resources.
//
// The package entry point is [Entity], constructed via [NewEntityWithConfig]
// (or [NewEntityWithConfigContext] for explicit context propagation). Each
// service is exposed as an interface on Entity (for example Entity.Accounts,
// Entity.Transactions), letting callers depend on the interface and mock it
// in tests.
//
// All HTTP traffic flows through a shared [HTTPClient] that handles auth
// injection, retries, idempotency keys, and observability hooks. Service
// implementations share a small embedded helper (serviceEntity) so that
// per-service files stay focused on per-service business logic.
package entities

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/security"
)

// Config is an interface for accessing configuration values.
// This allows us to use the config package without creating a direct dependency.
type Config interface {
	// GetHTTPClient returns the HTTP client to use for requests.
	GetHTTPClient() *http.Client

	// GetBaseURLs returns the map of service names to base URLs.
	GetBaseURLs() map[string]string

	// GetObservabilityProvider returns the observability provider.
	GetObservabilityProvider() observability.Provider

	// GetPluginAuth returns the plugin authentication configuration.
	GetPluginAuth() auth.AccessManager
}

// insecureHTTPConfig is an OPTIONAL extension of [Config] that exposes
// the data-plane insecure-HTTP opt-in. The entities layer uses a type
// assertion against this interface so adding the method to [Config] does
// not silently break callers that supply their own Config implementation
// (e.g. test fixtures). When the assertion fails, the SDK defaults to
// strict mode (no insecure HTTP), which matches the v3 default behavior.
type insecureHTTPConfig interface {
	GetAllowInsecureHTTP() bool
}

// configAllowsInsecureHTTP reads the optional insecure-HTTP flag from a
// Config implementation. Returns false for nil or for implementations
// that do not satisfy [insecureHTTPConfig], preserving the strict default.
func configAllowsInsecureHTTP(config Config) bool {
	if ext, ok := config.(insecureHTTPConfig); ok {
		return ext.GetAllowInsecureHTTP()
	}

	return false
}

// tracerAPIKeyConfig is an OPTIONAL extension of [Config] exposing the Tracer
// plane's X-API-Key. Read via a type assertion (same pattern as
// [insecureHTTPConfig]) so test-fixture Config implementations that predate
// the two-plane remodel keep compiling; a missing method means "no API key",
// i.e. the Tracer shares the Ledger Bearer.
type tracerAPIKeyConfig interface {
	GetTracerAPIKey() string
}

// configTracerAPIKey reads the optional Tracer X-API-Key from a Config
// implementation. Empty for nil or for implementations that do not satisfy
// [tracerAPIKeyConfig].
func configTracerAPIKey(config Config) string {
	if ext, ok := config.(tracerAPIKeyConfig); ok {
		return ext.GetTracerAPIKey()
	}

	return ""
}

// enableIdempotencyConfig is an OPTIONAL extension of [Config] exposing whether
// automatic idempotency-key generation is enabled. Read via a type assertion
// (same pattern as [tracerAPIKeyConfig]); a Config that does not satisfy it is
// treated as ENABLED — parity with the legacy default (DefaultEnableIdempotency
// == true) so test fixtures and pre-gate Config implementations keep auto-gen.
type enableIdempotencyConfig interface {
	GetEnableIdempotency() bool
}

// configEnableIdempotency reads the optional idempotency gate from a Config
// implementation, defaulting to true (enabled) when the method is absent.
func configEnableIdempotency(config Config) bool {
	if ext, ok := config.(enableIdempotencyConfig); ok {
		return ext.GetEnableIdempotency()
	}

	return true
}

// planeRetryConfig is an OPTIONAL extension of [Config] carrying the effective
// retry policy for the plane money-path, resolved ONCE by the caller
// (midaz.Client) so the plane retry round tripper and the legacy *HTTPClient
// agree on the effective values. It exists because the caller's retry option
// chain and custom policy live on the midaz.Client — assembled AFTER Entity
// construction — and are not reachable through the base [Config] methods. Read
// via a type assertion (same pattern as [tracerAPIKeyConfig]); a Config that
// does not satisfy it falls back to retry.DefaultOptions() + nil policy.
type planeRetryConfig interface {
	GetPlaneRetryOptions() *retry.Options
	GetPlaneCustomRetryPolicy() func(*http.Response, error) bool
}

// configPlaneRetry reads the optional plane retry policy from a Config
// implementation. Returns retry.DefaultOptions() + nil policy for nil or for
// implementations that do not satisfy [planeRetryConfig], or when the exposed
// options are nil.
func configPlaneRetry(config Config) (retry.Options, func(*http.Response, error) bool) {
	ext, ok := config.(planeRetryConfig)
	if !ok {
		return *retry.DefaultOptions(), nil
	}

	opts := ext.GetPlaneRetryOptions()
	if opts == nil {
		return *retry.DefaultOptions(), nil
	}

	//nolint:bodyclose // returns a retry-policy func (which has *http.Response in its signature), not an HTTP response.
	return *opts, ext.GetPlaneCustomRetryPolicy()
}

// Entity provides a centralized access point to all entity types in the Midaz SDK.
// It acts as a factory for creating specific entity interfaces for different resource types
// and operations.
type Entity struct {
	// HTTP client configuration
	httpClient *HTTPClient
	baseURLs   map[string]string

	// planes holds the two generated, typed plane clients (Ledger + Tracer).
	// They are the low-level surface the hand-written facade migrates onto in
	// Phases 2-4; during the transition the legacy per-service *HTTPClient
	// above still serves the 3 legacy services (Balances, Operations, Aliases).
	// Nil only when construction never reached the plane-client build step.
	planes *PlaneClients

	// Observability provider for tracing, metrics, and logging
	observability observability.Provider

	// enableIdempotency gates auto-generated X-Idempotency keys on the wired
	// plane write-facades (parity with the legacy SetEnableIdempotency gate).
	// Resolved once at construction from the Config; threaded into each write
	// facade's constructor by initServices.
	enableIdempotency bool

	// Ledger-plane resource accessors. Epic 5.3 swap: these 13 now route to the
	// concrete plane facades (*xFacade) over e.planes.Ledger, not the legacy
	// per-service interfaces. Balances/Operations/Aliases stay legacy (no facade
	// exists yet — Epic 5.4 gap resolution).
	Accounts          *accountsFacade
	AccountTypes      *accountTypesFacade
	Assets            *assetsFacade
	AssetRates        *assetRatesFacade
	Balances          BalancesService
	Holders           *holdersFacade
	Aliases           AliasesService
	Ledgers           *ledgersFacade
	MetadataIndexes   *metadataIndexesFacade
	Operations        OperationsService
	OperationRoutes   *operationRoutesFacade
	Organizations     *organizationsFacade
	Portfolios        *portfoliosFacade
	Segments          *segmentsFacade
	Transactions      *transactionsFacade
	TransactionRoutes *transactionRoutesFacade

	// Plane-native facades (Phases 3-4). Additive accessors over the typed
	// generated plane clients; they coexist with the resource accessors above
	// (13 already plane facades, plus the legacy Balances/Operations/Aliases
	// trio) until the Phase 5 cutover repoints the remaining trio too. Reached
	// fluently via client.X.Method (promoted through the embedded *Entity). Nil
	// when the Entity was built without plane clients.
	Rules               *rulesFacade
	Limits              *limitsFacade
	Validations         *validationsFacade
	Reservations        *reservationsFacade
	AuditEvents         *auditEventsFacade
	ProtectionAudit     *auditFacade
	Encryption          *encryptionFacade
	Instruments         *instrumentsFacade
	Composition         *compositionFacade
	FeePackages         *feePackagesFacade
	FeeEstimates        *feeEstimateFacade
	BillingPackages     *billingPackagesFacade
	BillingCalculations *billingCalculateFacade
}

// NewEntityWithConfig creates a new Entity using a Config object.
// This is a convenience constructor that integrates with the config package.
//
// Parameters:
//   - config: A configuration object from the config package. Must have
//     auth (Access Manager) and service URLs properly configured.
//
// Returns:
//   - *Entity: A pointer to the newly created Entity.
//   - error: An error if initialization fails.
//
// Post-construction tuning is exposed via dedicated setters:
//   - (*Entity).SetHTTPClient — replace the underlying *http.Client
//   - (*Entity).SetObservability — install / replace the observability provider
//   - (*HTTPClient).SetDebug — flip the debug-log flag
//   - (*HTTPClient).SetUserAgent — override the User-Agent header
//   - (*HTTPClient).SetLogger / SetSlowCallThreshold — observability tuning
func NewEntityWithConfig(config Config) (*Entity, error) {
	return NewEntityWithConfigContext(context.Background(), config)
}

// NewEntityWithConfigContext creates a new Entity using config and uses ctx for
// construction-time I/O such as the initial Access Manager token exchange.
func NewEntityWithConfigContext(ctx context.Context, config Config) (*Entity, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if config == nil || reflectutil.IsTypedNil(config) {
		return nil, errors.New("config cannot be nil")
	}

	// Check if plugin auth is enabled
	var authToken string

	pluginAuth := config.GetPluginAuth()

	if pluginAuth.Enabled {
		// Get a token from the plugin auth service
		provider := func(ctx context.Context) (string, error) {
			return auth.GetTokenFromAccessManager(ctx, pluginAuth, config.GetHTTPClient())
		}

		token, err := provider(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get token from plugin auth service: %w", auth.WrapAccessManagerTokenFetchError(err))
		}
		// Use the token from the plugin auth service
		authToken = token
	}

	// Create a new entity with the provided configuration
	allowInsecureHTTP := configAllowsInsecureHTTP(config)
	httpClient := NewHTTPClient(config.GetHTTPClient(), authToken, config.GetObservabilityProvider())
	httpClient.SetAllowInsecureHTTP(allowInsecureHTTP)

	normalizedBaseURLs, err := normalizeBaseURLs(config.GetBaseURLs(), allowInsecureHTTP)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(normalizedBaseURLs["transaction"]) == "" {
		normalizedBaseURLs["transaction"] = normalizedBaseURLs["onboarding"]
	}

	if strings.TrimSpace(normalizedBaseURLs["crm"]) == "" {
		normalizedBaseURLs["crm"] = normalizedBaseURLs["onboarding"]
	}

	if strings.TrimSpace(normalizedBaseURLs["tracer"]) == "" {
		normalizedBaseURLs["tracer"] = normalizedBaseURLs["onboarding"]
	}

	entity := &Entity{
		httpClient:        httpClient,
		baseURLs:          normalizedBaseURLs,
		observability:     config.GetObservabilityProvider(),
		enableIdempotency: configEnableIdempotency(config),
	}
	if pluginAuth.Enabled {
		entity.httpClient.setAuthTokenProvider(
			func(ctx context.Context) (string, error) {
				return auth.GetTokenFromAccessManager(ctx, pluginAuth, config.GetHTTPClient())
			},
			func() { auth.InvalidateAccessManagerToken(pluginAuth) },
		)
	}

	planes, err := buildPlaneClients(config, pluginAuth, normalizedBaseURLs)
	if err != nil {
		return nil, err
	}

	entity.planes = planes

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

// initServices initializes the service interfaces for the entity.
//
// The 3 legacy service entities (Balances, Operations, Aliases) share the SAME
// parent [*HTTPClient] — passed via [newSharedServiceEntity]. That single
// instance owns the auth-token cache, the singleflight token-refresh group,
// the customRetryPolicy, the observability surface, and the
// userAgent/debug/idempotency knobs. Sharing
// the client matters in three places:
//
//   - Token refresh on 401: when one service refreshes via [HTTPClient.refreshAuthToken]
//     the new token is visible to every other service immediately because
//     they read from the same authToken field under c.mu.
//   - Singleflight dedup: a 401 burst hitting multiple services collapses
//     onto one underlying tokenProvider call, since [HTTPClient.tokenRefreshGroup]
//     is one [singleflight.Group] not three.
//   - Set* propagation: [Entity.GetEntityHTTPClient] returns the same client
//     that every service uses, so SetDebug / SetUserAgent / SetLogger and
//     friends take effect on the next request from any service — no
//     "post-construction propagate" step required.
func (e *Entity) initServices() {
	if e == nil || e.httpClient == nil {
		return
	}

	if e.httpClient.client == nil {
		e.httpClient.client = defaultHTTPClient()
	}

	if e.baseURLs == nil {
		e.baseURLs = map[string]string{}
	}

	// Build the shared base once per service. The *HTTPClient is a pointer,
	// so all 3 legacy services see the same mutable state (auth token, refresh
	// group, customRetryPolicy, etc.); baseURLs is cloned per service via
	// prepareServiceBaseURLs so per-service mutation cannot bleed across
	// services.
	shared := func() serviceEntity {
		return newSharedServiceEntity(e.httpClient, e.baseURLs)
	}

	// Legacy per-service surface. Epic 5.3 repointed the 13 ledger resources to
	// facades (below); only Balances/Operations/Aliases remain legacy-wired
	// (no facade exists yet — Epic 5.4 gap).
	e.Balances = &balancesEntity{serviceEntity: shared()}
	e.Aliases = &aliasesEntity{serviceEntity: shared()}
	e.Operations = &operationsEntity{serviceEntity: shared()}

	// Plane-native facades. The 13 ledger resource accessors (Epic 5.3 swap)
	// join the Phase 3-4 additive facades here — all route over the typed plane
	// clients, not the legacy *HTTPClient. The e.planes != nil guard is
	// defensive: no first-party constructor reaches this with planes == nil
	// (buildPlaneClients either errors or returns a non-nil PlaneClients, and
	// NewEntityWithConfigContext always assigns e.planes before initServices).
	// It exists only so a hand-rolled zero-value &Entity{} — legal because
	// Entity and InitServices are exported — cannot nil-deref the plane clients.
	if e.planes != nil {
		e.Organizations = newOrganizationsFacade(e.planes.Ledger, e.enableIdempotency)
		e.Ledgers = newLedgersFacade(e.planes.Ledger, e.enableIdempotency)
		e.Accounts = newAccountsFacade(e.planes.Ledger, e.enableIdempotency)
		e.Assets = newAssetsFacade(e.planes.Ledger, e.enableIdempotency)
		e.AssetRates = newAssetRatesFacade(e.planes.Ledger, e.enableIdempotency)
		e.Portfolios = newPortfoliosFacade(e.planes.Ledger, e.enableIdempotency)
		e.Segments = newSegmentsFacade(e.planes.Ledger, e.enableIdempotency)
		e.AccountTypes = newAccountTypesFacade(e.planes.Ledger, e.enableIdempotency)
		e.MetadataIndexes = newMetadataIndexesFacade(e.planes.Ledger, e.enableIdempotency)
		e.OperationRoutes = newOperationRoutesFacade(e.planes.Ledger, e.enableIdempotency)
		e.TransactionRoutes = newTransactionRoutesFacade(e.planes.Ledger, e.enableIdempotency)
		e.Holders = newHoldersFacade(e.planes.Ledger, e.enableIdempotency)
		e.Transactions = newTransactionsFacade(e.planes.Ledger, e.enableIdempotency)

		e.Rules = newRulesFacade(e.planes.Tracer, e.enableIdempotency)
		e.Limits = newLimitsFacade(e.planes.Tracer, e.enableIdempotency)
		e.Validations = newValidationsFacade(e.planes.Tracer)
		e.Reservations = newReservationsFacade(e.planes.Tracer)
		e.AuditEvents = newAuditEventsFacade(e.planes.Tracer)
		e.ProtectionAudit = newAuditFacade(e.planes.Ledger)
		e.Encryption = newEncryptionFacade(e.planes.Ledger, e.enableIdempotency)
		e.Instruments = newInstrumentsFacade(e.planes.Ledger, e.enableIdempotency)
		e.Composition = newCompositionFacade(e.planes.Ledger, e.enableIdempotency)
		e.FeePackages = newFeePackagesFacade(e.planes.Ledger, e.enableIdempotency)
		e.FeeEstimates = newFeeEstimateFacade(e.planes.Ledger)
		e.BillingPackages = newBillingPackagesFacade(e.planes.Ledger, e.enableIdempotency)
		e.BillingCalculations = newBillingCalculateFacade(e.planes.Ledger)
	}
}

// InitServices initializes the service interfaces for the entity.
// This is an exported version of initServices required for the plugin auth interface.
func (e *Entity) InitServices() {
	if e == nil {
		return
	}

	e.initServices()
}

// RefreshHTTPConfiguration is a no-op kept for source compatibility.
//
// In v3 every service entity shares the parent Entity's *HTTPClient, so
// SetDebug / SetUserAgent / SetLogger / SetSlowCallThreshold / WithRetryOptions
// / SetCustomRetryPolicy / SetEnableIdempotency made on
// [Entity.GetEntityHTTPClient] take effect on the next request from any
// service immediately. There is no per-service snapshot to refresh.
//
// Deprecated: this method has no observable effect since the per-service
// HTTPClient consolidation. Calls can be removed without behavior change.
func (e *Entity) RefreshHTTPConfiguration() {
	// No-op: per-service HTTPClient state was eliminated. The parent
	// HTTPClient IS the per-service HTTPClient — Set* mutations propagate
	// naturally because there is exactly one instance.
	_ = e
}

// GetEntityHTTPClient returns the custom HTTP client used by the entity.
// This allows for configuration of the HTTP client after the entity is created.
//
// Returns:
//   - *HTTPClient: The HTTP client used by the entity for API requests.
func (e *Entity) GetEntityHTTPClient() *HTTPClient {
	if e == nil {
		return nil
	}

	return e.httpClient
}

// GetHTTPClient returns the standard HTTP client used by the entity.
// This is required for the plugin auth interface.
//
// Returns:
//   - *http.Client: The standard HTTP client used by the entity for API requests.
func (e *Entity) GetHTTPClient() *http.Client {
	if e == nil || e.httpClient == nil {
		return nil
	}

	return e.httpClient.client
}

// GetObservabilityProvider returns the observability provider used by the entity.
//
// Returns:
//   - observability.Provider: The observability provider used by the entity.
func (e *Entity) GetObservabilityProvider() observability.Provider {
	if e == nil {
		return nil
	}

	return e.observability
}

// SetHTTPClient replaces the HTTP client used by the LEGACY per-service surface
// only — Balances, Operations, and Aliases — and preserves the entity's tenant
// ID and auth token across the swap.
//
// LIMITATION: it does NOT re-transport the 18 plane facades (Organizations,
// Ledgers, Accounts, Transactions, Encryption, and the rest). initServices
// rebuilds those facades over the already-constructed e.planes.Ledger /
// e.planes.Tracer clients, whose transport is fixed at construction and is not
// rebuilt here; only the legacy trio picks up the new client. To control the
// transport for the facades/planes, pass config.WithHTTPClient(client) when the
// client is constructed rather than swapping it afterward.
//
// Parameters:
//   - client: The HTTP client to use for API requests (legacy trio only).
func (e *Entity) SetHTTPClient(client *http.Client) {
	if e == nil {
		return
	}

	if client == nil {
		return
	}

	if e.httpClient == nil {
		e.httpClient = NewHTTPClient(client, "", e.observability)
		e.initServices()

		return
	}

	savedConfig := e.httpClient.cloneConfiguration()

	// Create a new HTTP client with the same auth token and observability
	e.httpClient = NewHTTPClient(client, e.httpClient.authToken, e.observability)
	e.httpClient.applyConfigurationSnapshot(savedConfig)

	// Re-initialize services with the new HTTP client
	e.initServices()
}

// SetObservability installs an observability provider on the entity and its
// underlying HTTPClient.
//
// When the provider reports IsEnabled() == true, a fresh
// observability.MetricsCollector is constructed (see observability.NewMetricsCollector)
// and attached atomically alongside the provider, so observers never see
// a new provider with the previous metrics collector or vice versa.
//
// A nil provider is a no-op.
//
// Returns an error only if the metrics collector fails to initialize.
//
// Parameters:
//   - provider: The observability.Provider to install.
//
// Returns:
//   - error: Non-nil only if MetricsCollector construction fails.
func (e *Entity) SetObservability(provider observability.Provider) error {
	if e == nil {
		return errors.New("entity cannot be nil")
	}

	if e.httpClient == nil {
		return errors.New("entity HTTP client cannot be nil")
	}

	if provider == nil || reflectutil.IsTypedNil(provider) {
		return errors.New("observability provider cannot be nil")
	}

	var metrics *observability.MetricsCollector

	if provider.IsEnabled() {
		collector, err := observability.NewMetricsCollector(provider)
		if err != nil {
			return err
		}

		metrics = collector
	}

	e.httpClient.setObservabilityLocked(provider, metrics)
	e.observability = provider

	return nil
}

func normalizeBaseURLs(baseURLs map[string]string, allowInsecureHTTP bool) (map[string]string, error) {
	normalized := maps.Clone(baseURLs)
	if normalized == nil {
		return nil, errors.New("service URLs map cannot be nil")
	}

	onboarding := strings.TrimSpace(normalized["onboarding"])
	if onboarding == "" {
		return nil, errors.New("missing onboarding URL in service URLs map")
	}

	if strings.TrimSpace(normalized["transaction"]) == "" {
		normalized["transaction"] = onboarding
	}

	if strings.TrimSpace(normalized["crm"]) == "" {
		normalized["crm"] = onboarding
	}

	for service, serviceURL := range normalized {
		normalizedURL, err := normalizeServiceURL(serviceURL, allowInsecureHTTP)
		if err != nil {
			return nil, fmt.Errorf("invalid %s URL: %w", service, err)
		}

		normalized[service] = normalizedURL
	}

	return normalized, nil
}

func normalizeServiceURL(rawURL string, allowInsecureHTTP bool) (string, error) {
	parsedURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(rawURL), "/"))
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", errors.New("URL must include scheme and host")
	}

	if parsedURL.User != nil {
		return "", errors.New("URL must not include user information")
	}

	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return "", errors.New("URL must not include query parameters or fragments")
	}

	if err := security.ValidateOutboundRequestWithInsecureHTTP(&http.Request{URL: parsedURL}, allowInsecureHTTP); err != nil {
		return "", err
	}

	cleanPath := strings.TrimRight(parsedURL.Path, "/")
	if cleanPath == "" {
		parsedURL.Path = "/v1"
	} else if cleanPath != "/v1" && !strings.HasSuffix(cleanPath, "/v1") {
		parsedURL.Path = cleanPath + "/v1"
	} else {
		parsedURL.Path = cleanPath
	}

	return parsedURL.String(), nil
}
