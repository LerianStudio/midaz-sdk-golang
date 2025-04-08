// Package entities provides access to the Midaz API resources and operations.
// It implements service interfaces for interacting with accounts, assets, ledgers,
// transactions, and other Midaz platform resources.
package entities

import (
	"fmt"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
)

// Config is an interface for accessing configuration values.
// This allows us to use the config package without creating a direct dependency.
type Config interface {
	// GetHTTPClient returns the HTTP client to use for requests.
	GetHTTPClient() *http.Client

	// GetAuthToken returns the authentication token.
	GetAuthToken() string

	// GetBaseURLs returns the map of service names to base URLs.
	GetBaseURLs() map[string]string

	// GetObservabilityProvider returns the observability provider.
	GetObservabilityProvider() observability.Provider
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
	Accounts      AccountsService
	Assets        AssetsService
	AssetRates    AssetRatesService
	Balances      BalancesService
	Ledgers       LedgersService
	Operations    OperationsService
	Organizations OrganizationsService
	Portfolios    PortfoliosService
	Segments      SegmentsService
	Transactions  TransactionsService
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
	entity := &Entity{
		httpClient:    NewHTTPClient(client, authToken, observabilityProvider),
		baseURLs:      baseURLs,
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
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create a new entity using values from the config
	entity := &Entity{
		httpClient:    NewHTTPClient(config.GetHTTPClient(), config.GetAuthToken(), config.GetObservabilityProvider()),
		baseURLs:      config.GetBaseURLs(),
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
	// Initialize service interfaces with the entity configuration
	client := e.httpClient.client
	token := e.httpClient.authToken

	// Make sure we have the observability provider correctly set in the HTTP client
	if e.httpClient.observability == nil && e.observability != nil {
		e.httpClient.observability = e.observability

		// Create metrics collector if needed
		if e.observability.IsEnabled() {
			e.httpClient.metrics, _ = observability.NewMetricsCollector(e.observability)
		}
	}

	// Initialize service interfaces with the new HTTP client
	e.Accounts = NewAccountsEntity(client, token, e.baseURLs)
	e.Assets = NewAssetsEntity(client, token, e.baseURLs)
	e.AssetRates = NewAssetRatesEntity(client, token, e.baseURLs)
	e.Balances = NewBalancesEntity(client, token, e.baseURLs)
	e.Ledgers = NewLedgersEntity(client, token, e.baseURLs)
	e.Operations = NewOperationsEntity(client, token, e.baseURLs)
	e.Organizations = NewOrganizationsEntity(client, token, e.baseURLs)
	e.Portfolios = NewPortfoliosEntity(client, token, e.baseURLs)
	e.Segments = NewSegmentsEntity(client, token, e.baseURLs)
	e.Transactions = NewTransactionsEntity(client, token, e.baseURLs)
}

// GetHTTPClient returns the HTTP client used by the entity.
// This allows for configuration of the HTTP client after the entity is created.
//
// Returns:
//   - *HTTPClient: The HTTP client used by the entity for API requests.
func (e *Entity) GetHTTPClient() *HTTPClient {
	return e.httpClient
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
//
// Parameters:
//   - client: The HTTP client to use for API requests.
func (e *Entity) SetHTTPClient(client *http.Client) {
	if client == nil {
		return
	}

	// Create a new HTTP client with the same auth token and observability
	e.httpClient = NewHTTPClient(client, e.httpClient.authToken, e.observability)

	// Re-initialize services with the new HTTP client
	e.initServices()
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
		return nil, fmt.Errorf("base URL cannot be empty")
	}

	// Create a map with both service URLs pointing to the same base URL
	baseURLs := map[string]string{
		"onboarding":  baseURL,
		"transaction": baseURL,
	}

	// Create a default HTTP client
	client := &http.Client{
		Timeout: 30 * 1000000000, // 30 seconds in nanoseconds
	}

	// Create a new entity with default values
	entity := &Entity{
		httpClient:    NewHTTPClient(client, "", nil),
		baseURLs:      baseURLs,
		observability: nil,
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
		return nil, fmt.Errorf("service URLs map cannot be nil")
	}

	if _, ok := serviceURLs["onboarding"]; !ok {
		return nil, fmt.Errorf("missing onboarding URL in service URLs map")
	}

	if _, ok := serviceURLs["transaction"]; !ok {
		return nil, fmt.Errorf("missing transaction URL in service URLs map")
	}

	// Create a default HTTP client
	client := &http.Client{
		Timeout: 30 * 1000000000, // 30 seconds in nanoseconds
	}

	// Create a new entity with default values and service-specific URLs
	entity := &Entity{
		httpClient:    NewHTTPClient(client, "", nil),
		baseURLs:      serviceURLs,
		observability: nil,
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
