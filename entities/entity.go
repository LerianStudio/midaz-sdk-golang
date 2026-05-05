// Package entities provides access to the Midaz API resources and operations.
// It implements service interfaces for interacting with accounts, assets, ledgers,
// transactions, and other Midaz platform resources.
package entities

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/internal/reflectutil"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/security"
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

	// GetTenantID returns the default tenant ID applied to every request
	// made by entity HTTP clients. Per-request overrides via
	// [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithRequestTenantID]
	// take precedence. An empty value disables the X-Tenant-ID header.
	GetTenantID() string
}

// Entity provides a centralized access point to all entity types in the Midaz SDK.
// It acts as a factory for creating specific entity interfaces for different resource types
// and operations.
type Entity struct {
	// HTTP client configuration
	httpClient *HTTPClient
	baseURLs   map[string]string

	// Observability provider for tracing, metrics, and logging
	observability observability.Provider

	// Service interfaces for different resource types
	Accounts          AccountsService
	AccountTypes      AccountTypesService
	Assets            AssetsService
	AssetRates        AssetRatesService
	Balances          BalancesService
	Holders           HoldersService
	Aliases           AliasesService
	Ledgers           LedgersService
	MetadataIndexes   MetadataIndexesService
	Operations        OperationsService
	OperationRoutes   OperationRoutesService
	Organizations     OrganizationsService
	Portfolios        PortfoliosService
	Segments          SegmentsService
	Transactions      TransactionsService
	TransactionRoutes TransactionRoutesService
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

		token, err := provider(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get token from plugin auth service: %w", err)
		}
		// Use the token from the plugin auth service
		authToken = token
	}

	// Create a new entity with the provided configuration
	httpClient := NewHTTPClient(config.GetHTTPClient(), authToken, config.GetObservabilityProvider())

	normalizedBaseURLs, err := normalizeBaseURLs(config.GetBaseURLs())
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(normalizedBaseURLs["transaction"]) == "" {
		normalizedBaseURLs["transaction"] = normalizedBaseURLs["onboarding"]
	}

	if strings.TrimSpace(normalizedBaseURLs["crm"]) == "" {
		normalizedBaseURLs["crm"] = normalizedBaseURLs["onboarding"]
	}

	entity := &Entity{
		httpClient:    httpClient,
		baseURLs:      normalizedBaseURLs,
		observability: config.GetObservabilityProvider(),
	}
	// Seed the default tenant ID from Config so the midaz package's
	// client-level WithTenantID option propagates straight through to the
	// HTTP client. Per-request sdkctx.WithRequestTenantID overrides still
	// win at request time.
	if tid := strings.TrimSpace(config.GetTenantID()); tid != "" {
		entity.httpClient.setTenantIDLocked(tid)
	}

	if pluginAuth.Enabled {
		entity.httpClient.setAuthTokenProvider(
			func(ctx context.Context) (string, error) {
				return auth.GetTokenFromAccessManager(ctx, pluginAuth, config.GetHTTPClient())
			},
			func() { auth.InvalidateAccessManagerToken(pluginAuth) },
		)
	}

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

// initServices initializes the service interfaces for the entity.
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

	// Create the service interfaces
	e.Transactions = newTransactionsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Accounts = newAccountsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.AccountTypes = newAccountTypesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Assets = newAssetsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.AssetRates = newAssetRatesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Balances = newBalancesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Holders = newHoldersEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Aliases = newAliasesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Ledgers = newLedgersEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.MetadataIndexes = newMetadataIndexesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Operations = newOperationsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.OperationRoutes = newOperationRoutesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Organizations = newOrganizationsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Portfolios = newPortfoliosEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Segments = newSegmentsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.TransactionRoutes = newTransactionRoutesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)

	// Each newXxxEntity constructor creates a fresh HTTPClient around the shared
	// transport. Copy the parent entity configuration across after construction.
	e.propagateHTTPClientConfiguration()
}

// tenantSetter is implemented by service entities that can receive a tenant ID.
// This decouples propagateTenantID from knowing every concrete service type.
type tenantSetter interface {
	setDefaultTenantID(tenantID string)
}

type httpClientConfigurator interface {
	entityHTTPClient() *HTTPClient
}

// propagateTenantID copies the entity-level tenant ID to all service entity HTTP clients.
// It iterates over service fields and calls the tenantSetter interface rather than
// hard-coding each concrete type, so adding new services cannot silently break propagation.
func (e *Entity) propagateTenantID() {
	tid := e.httpClient.GetTenantID()
	if tid == "" {
		return
	}

	services := []any{
		e.Accounts, e.AccountTypes, e.Assets, e.AssetRates,
		e.Aliases, e.Balances, e.Holders, e.Ledgers, e.MetadataIndexes, e.Operations, e.OperationRoutes,
		e.Organizations, e.Portfolios, e.Segments,
		e.Transactions, e.TransactionRoutes,
	}

	for _, svc := range services {
		if ts, ok := svc.(tenantSetter); ok {
			ts.setDefaultTenantID(tid)
		}
	}
}

func (e *Entity) propagateHTTPClientConfiguration() {
	e.propagateTenantID()

	services := []any{
		e.Accounts, e.AccountTypes, e.Assets, e.AssetRates,
		e.Aliases, e.Balances, e.Holders, e.Ledgers, e.MetadataIndexes, e.Operations, e.OperationRoutes,
		e.Organizations, e.Portfolios, e.Segments,
		e.Transactions, e.TransactionRoutes,
	}

	for _, svc := range services {
		if configurator, ok := svc.(httpClientConfigurator); ok {
			configurator.entityHTTPClient().applyConfigurationFrom(e.httpClient)
		}
	}
}

func (e *accountsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *accountTypesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *assetsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *assetRatesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *balancesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *holdersEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *aliasesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *ledgersEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *metadataIndexesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *operationsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *operationRoutesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *organizationsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *portfoliosEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *segmentsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *transactionsEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

func (e *transactionRoutesEntity) entityHTTPClient() *HTTPClient {
	return e.httpClient
}

// InitServices initializes the service interfaces for the entity.
// This is an exported version of initServices required for the plugin auth interface.
func (e *Entity) InitServices() {
	if e == nil {
		return
	}

	e.initServices()
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

// SetHTTPClient sets the HTTP client for the entity.
// This allows for replacing the HTTP client after the entity is created.
// The tenant ID configured on the entity is preserved across the replacement.
//
// Parameters:
//   - client: The HTTP client to use for API requests.
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
	if e == nil || e.httpClient == nil {
		return errors.New("entity HTTP client cannot be nil")
	}

	if provider == nil {
		return nil
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

func normalizeBaseURLs(baseURLs map[string]string) (map[string]string, error) {
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
		normalizedURL, err := normalizeServiceURL(serviceURL)
		if err != nil {
			return nil, fmt.Errorf("invalid %s URL: %w", service, err)
		}

		normalized[service] = normalizedURL
	}

	return normalized, nil
}

func normalizeServiceURL(rawURL string) (string, error) {
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

	if err := security.ValidateOutboundRequest(&http.Request{URL: parsedURL}); err != nil {
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
