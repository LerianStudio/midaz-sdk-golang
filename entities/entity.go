// Package entities provides access to the Midaz API resources and operations.
// It implements service interfaces for interacting with accounts, assets, ledgers,
// transactions, and other Midaz platform resources.
package entities

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
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

// NewEntity creates a new Entity instance with the provided client configuration.
// This constructor initializes an Entity that provides access to all entity types
// in the Midaz SDK.
//
// Parameters:
//   - client: The HTTP client to use for API requests. Can be configured with custom timeouts
//     and transport options. If nil, a default client will be used.
//   - authToken: The authentication token for API authorization. Must be a valid JWT token
//     issued by the Midaz authentication service.
//   - baseURLs: Map of service names to base URLs. Must include an "onboarding" key with
//     the URL of the onboarding service (e.g., "https://api.midaz.io/v1").
//   - options: Optional configuration options for customizing the entity behavior.
//     These are applied in order after the entity is created.
//
// Returns:
//   - *Entity: A pointer to the newly created Entity, ready to interact with the Midaz API.
//     The Entity provides access to all service interfaces (Accounts, Assets, Ledgers, etc.).
//   - error: An error if the client initialization fails, such as when required parameters
//     are missing or when options cannot be applied.
//
// Example - Basic usage:
//
//	// Create a new entity with default settings
//	entity, err := entities.NewEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create entity: %v", err)
//	}
//
//	// Use the entity to access different services
//	organization, err := entity.Organizations.GetOrganization(
//	    context.Background(),
//	    "org-123",
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to retrieve organization: %v", err)
//	}
//
//	fmt.Printf("Organization: %s\n", organization.LegalName)
//
// Example - With custom options:
//
//	// Create a new entity with debug logging enabled
//	entity, err := entities.NewEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	    entities.WithDebug(true),
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create entity: %v", err)
//	}
//
//	// Create a ledger using the entity
//	ledger, err := entity.Ledgers.CreateLedger(
//	    context.Background(),
//	    "org-123",
//	    models.NewCreateLedgerInput("Main Ledger"),
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to create ledger: %v", err)
//	}
//
//	fmt.Printf("Ledger created: %s\n", ledger.ID)
func NewEntity(client *http.Client, authToken string, baseURLs map[string]string, observabilityProvider observability.Provider, options ...Option) (*Entity, error) {
	// Create a new entity with the provided configuration
	httpClient := NewHTTPClient(client, authToken, observabilityProvider)

	normalizedBaseURLs, err := normalizeBaseURLs(baseURLs)
	if err != nil {
		return nil, err
	}

	entity := &Entity{
		httpClient:    httpClient,
		baseURLs:      normalizedBaseURLs,
		observability: observabilityProvider,
	}

	// Apply the provided options
	for _, option := range options {
		if err := option(entity); err != nil {
			return nil, err
		}
	}

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

// NewEntityWithConfig creates a new Entity using a Config object.
// This is a convenience constructor that integrates with the config package.
//
// Parameters:
//   - config: A configuration object from the config package. Must have AuthToken
//     and service URLs properly configured.
//   - options: Optional configuration options for customizing the entity behavior.
//
// Returns:
//   - *Entity: A pointer to the newly created Entity.
//   - error: An error if initialization fails.
func NewEntityWithConfig(config Config, options ...Option) (*Entity, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Check if plugin auth is enabled
	var authToken string

	pluginAuth := config.GetPluginAuth()

	if pluginAuth.Enabled {
		// Get a token from the plugin auth service
		token, err := auth.GetTokenFromAccessManager(context.Background(), pluginAuth, config.GetHTTPClient())
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

	entity := &Entity{
		httpClient:    httpClient,
		baseURLs:      normalizedBaseURLs,
		observability: config.GetObservabilityProvider(),
	}

	// Apply any additional options
	for _, option := range options {
		if err := option(entity); err != nil {
			return nil, err
		}
	}

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

// initServices initializes the service interfaces for the entity.
func (e *Entity) initServices() {
	// Create the service interfaces
	e.Transactions = NewTransactionsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Accounts = NewAccountsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.AccountTypes = NewAccountTypesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Assets = NewAssetsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.AssetRates = NewAssetRatesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Balances = NewBalancesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Holders = NewHoldersEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Aliases = NewAliasesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Ledgers = NewLedgersEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.MetadataIndexes = NewMetadataIndexesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Operations = NewOperationsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.OperationRoutes = NewOperationRoutesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Organizations = NewOrganizationsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Portfolios = NewPortfoliosEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.Segments = NewSegmentsEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)
	e.TransactionRoutes = NewTransactionRoutesEntity(e.httpClient.client, e.httpClient.authToken, e.baseURLs)

	// Each NewXxxEntity constructor creates a fresh HTTPClient around the shared
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
	tid := e.httpClient.tenantID
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
	e.initServices()
}

// GetEntityHTTPClient returns the custom HTTP client used by the entity.
// This allows for configuration of the HTTP client after the entity is created.
//
// Returns:
//   - *HTTPClient: The HTTP client used by the entity for API requests.
func (e *Entity) GetEntityHTTPClient() *HTTPClient {
	return e.httpClient
}

// GetHTTPClient returns the standard HTTP client used by the entity.
// This is required for the plugin auth interface.
//
// Returns:
//   - *http.Client: The standard HTTP client used by the entity for API requests.
func (e *Entity) GetHTTPClient() *http.Client {
	return e.httpClient.client
}

// GetObservabilityProvider returns the observability provider used by the entity.
//
// Returns:
//   - observability.Provider: The observability provider used by the entity.
func (e *Entity) GetObservabilityProvider() observability.Provider {
	return e.observability
}

// SetHTTPClient sets the HTTP client for the entity.
// This allows for replacing the HTTP client after the entity is created.
// The tenant ID configured on the entity is preserved across the replacement.
//
// Parameters:
//   - client: The HTTP client to use for API requests.
func (e *Entity) SetHTTPClient(client *http.Client) {
	if client == nil {
		return
	}

	// Preserve tenant ID across HTTP client replacement
	savedTenantID := e.httpClient.tenantID

	// Create a new HTTP client with the same auth token and observability
	e.httpClient = NewHTTPClient(client, e.httpClient.authToken, e.observability)
	e.httpClient.tenantID = savedTenantID

	// Re-initialize services with the new HTTP client
	e.initServices()
}

// SetAuthToken sets the authentication token for the entity.
// This is required for the plugin auth interface.
//
// Parameters:
//   - token: The authentication token to use for API requests.
func (e *Entity) SetAuthToken(token string) {
	if token != "" {
		// Set the token directly on the HTTP client
		e.httpClient.authToken = token
		e.propagateHTTPClientConfiguration()
	}
}

// New creates a new Entity with the provided base URL and options.
// This is a simplified version of NewEntity that takes a single base URL and
// applies default values for other settings.
//
// Parameters:
//   - baseURL: The base URL for all API requests.
//   - options: Optional configuration options for the entity.
//
// Returns:
//   - *Entity: A pointer to the newly created Entity.
//   - error: An error if initialization fails.
func New(baseURL string, options ...Option) (*Entity, error) {
	if baseURL == "" {
		return nil, errors.New("base URL cannot be empty")
	}

	// Create a map with both service URLs pointing to the same base URL
	baseURLs := map[string]string{
		"onboarding":  baseURL,
		"transaction": baseURL,
		"crm":         baseURL,
	}

	// Create a default HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create a new HTTP client
	httpClient := NewHTTPClient(client, "", nil)

	// Create a new entity with the provided base URL
	entity := &Entity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
	}

	// Apply any options
	for _, option := range options {
		if err := option(entity); err != nil {
			return nil, err
		}
	}

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

// NewWithServiceURLs creates a new Entity with separate URLs for each service.
// This is the preferred method when different services have different URLs.
//
// Parameters:
//   - serviceURLs: Map of service names to base URLs. Must include both "onboarding"
//     and "transaction" keys with the respective service URLs.
//   - options: Optional configuration options for the entity.
//
// Returns:
//   - *Entity: A pointer to the newly created Entity.
//   - error: An error if initialization fails.
func NewWithServiceURLs(serviceURLs map[string]string, options ...Option) (*Entity, error) {
	// Validate required service URLs
	if serviceURLs == nil {
		return nil, errors.New("service URLs map cannot be nil")
	}

	if _, ok := serviceURLs["onboarding"]; !ok {
		return nil, errors.New("missing onboarding URL in service URLs map")
	}

	if _, ok := serviceURLs["transaction"]; !ok {
		return nil, errors.New("missing transaction URL in service URLs map")
	}

	normalizedBaseURLs, err := normalizeBaseURLs(serviceURLs)
	if err != nil {
		return nil, err
	}

	// Create a default HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create a new HTTP client
	httpClient := NewHTTPClient(client, "", nil)

	// Create a new entity with the provided service URLs
	entity := &Entity{
		httpClient: httpClient,
		baseURLs:   normalizedBaseURLs,
	}

	// Apply any options
	for _, option := range options {
		if err := option(entity); err != nil {
			return nil, err
		}
	}

	// Initialize service interfaces
	entity.initServices()

	return entity, nil
}

func copyBaseURLs(baseURLs map[string]string) map[string]string {
	if baseURLs == nil {
		return nil
	}

	normalized := make(map[string]string, len(baseURLs)+1)
	for service, serviceURL := range baseURLs {
		normalized[service] = serviceURL
	}

	return normalized
}

func normalizeBaseURLs(baseURLs map[string]string) (map[string]string, error) {
	normalized := copyBaseURLs(baseURLs)
	if normalized == nil {
		return nil, errors.New("service URLs map cannot be nil")
	}

	if strings.TrimSpace(normalized["crm"]) == "" {
		crmURL := strings.TrimSpace(normalized["onboarding"])
		if crmURL == "" {
			crmURL = strings.TrimSpace(os.Getenv("MIDAZ_CRM_URL"))
		}

		if crmURL == "" {
			return nil, errors.New("missing crm URL: provide 'crm' key in service URLs map or set MIDAZ_CRM_URL environment variable")
		}

		normalized["crm"] = crmURL
	}

	return normalized, nil
}
