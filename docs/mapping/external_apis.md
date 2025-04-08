# Midaz Go SDK Method Map

This document provides a comprehensive overview of all public methods available in the Midaz Go SDK, organized by package and purpose.

## Table of Contents

- [I. Client Package (`midaz`)](#i-client-package-midaz)
- [II. Entities Package (`midaz/entities`)](#ii-entities-package-midazentities)
- [III. Error Handling Package (`midaz/pkg/errors`)](#iii-error-handling-package-midazpkgerrors)
- [IV. Configuration Package (`midaz/pkg/config`)](#iv-configuration-package-midazpkgconfig)
- [V. Concurrent Package (`midaz/pkg/concurrent`)](#v-concurrent-package-midazpkgconcurrent)
- [VI. Observability Package (`midaz/pkg/observability`)](#vi-observability-package-midazpkgobservability)
- [VII. Pagination Package (`midaz/pkg/pagination`)](#vii-pagination-package-midazpkgpagination)
- [VIII. Retry Package (`midaz/pkg/retry`)](#viii-retry-package-midazpkgretry)
- [IX. Validation Package (`midaz/pkg/validation`)](#ix-validation-package-midazpkgvalidation)
- [X. Models Package (`midaz/models`)](#x-models-package-midazmodels)

## I. Client Package (`midaz`)

The client package provides the main entry point for the Midaz SDK.

### Client

- `New()` - Creates a new Midaz client with the provided options
- `WithAuthToken()` - Sets the authentication token for the client
- `WithOnboardingURL()` - Sets the base URL for the onboarding API
- `WithTransactionURL()` - Sets the base URL for the transaction API
- `WithServiceURLs()` - Sets a map of service-specific URLs for different API services
- `WithHTTPClient()` - Sets a custom HTTP client for the client
- `WithTimeout()` - Sets the timeout for requests made by the client
- `WithDebug()` - Enables or disables debug mode for the client
- `WithUserAgent()` - Sets a custom user agent string for API requests
- `WithObservability()` - Sets the observability provider for metrics, tracing, and logging
- `WithContext()` - Sets the context for the client
- `WithRetryOptions()` - Sets retry options for API requests
- `Client.Entity` - Provides access to the Entity API interface
- `Client.GetVersion()` - Returns the current version of the SDK

## II. Entities Package (`midaz/entities`)

The entities package provides direct access to Midaz API resources.

### Entity

- `NewEntity()` - Creates a new Entity instance that provides access to all service interfaces with a single base URL
- `NewWithServiceURLs()` - Creates a new Entity instance with separate URLs for each service (preferred method)
- `Entity.Accounts` - Returns the AccountsService interface for account operations
- `Entity.Assets` - Returns the AssetsService interface for asset operations
- `Entity.AssetRates` - Returns the AssetRatesService interface for asset rate operations
- `Entity.Balances` - Returns the BalancesService interface for balance operations
- `Entity.Ledgers` - Returns the LedgersService interface for ledger operations
- `Entity.Operations` - Returns the OperationsService interface for operation operations
- `Entity.Organizations` - Returns the OrganizationsService interface for organization operations
- `Entity.Portfolios` - Returns the PortfoliosService interface for portfolio operations
- `Entity.Segments` - Returns the SegmentsService interface for segment operations
- `Entity.Transactions` - Returns the TransactionsService interface for transaction operations

### Accounts Service

- `AccountsService.List()` - Lists accounts for a ledger with pagination
- `AccountsService.Get()` - Gets an account by ID
- `AccountsService.GetByAlias()` - Gets an account by alias
- `AccountsService.Create()` - Creates a new account
- `AccountsService.Update()` - Updates an account
- `AccountsService.Delete()` - Deletes an account

### Assets Service

- `AssetsService.List()` - Lists assets for a ledger with pagination
- `AssetsService.Get()` - Gets an asset by ID
- `AssetsService.Create()` - Creates a new asset
- `AssetsService.Update()` - Updates an asset
- `AssetsService.Delete()` - Deletes an asset

### Asset Rates Service

- `AssetRatesService.List()` - Lists asset rates for a ledger with pagination
- `AssetRatesService.Get()` - Gets an asset rate by ID
- `AssetRatesService.Create()` - Creates a new asset rate
- `AssetRatesService.Update()` - Updates an asset rate
- `AssetRatesService.Delete()` - Deletes an asset rate

### Balances Service

- `BalancesService.List()` - Lists balances for a ledger with pagination
- `BalancesService.ListForAccount()` - Lists balances for a specific account
- `BalancesService.Get()` - Gets a balance by ID
- `BalancesService.Update()` - Updates a balance

### Ledgers Service

- `LedgersService.List()` - Lists ledgers for an organization with pagination
- `LedgersService.Get()` - Gets a ledger by ID
- `LedgersService.Create()` - Creates a new ledger
- `LedgersService.Update()` - Updates a ledger
- `LedgersService.Delete()` - Deletes a ledger

### Operations Service

- `OperationsService.List()` - Lists operations for a ledger with pagination
- `OperationsService.Get()` - Gets an operation by ID

### Organizations Service

- `OrganizationsService.List()` - Lists organizations with pagination
- `OrganizationsService.Get()` - Gets an organization by ID
- `OrganizationsService.Create()` - Creates a new organization
- `OrganizationsService.Update()` - Updates an organization
- `OrganizationsService.Delete()` - Deletes an organization

### Portfolios Service

- `PortfoliosService.List()` - Lists portfolios for a ledger with pagination
- `PortfoliosService.Get()` - Gets a portfolio by ID
- `PortfoliosService.Create()` - Creates a new portfolio
- `PortfoliosService.Update()` - Updates a portfolio
- `PortfoliosService.Delete()` - Deletes a portfolio

### Segments Service

- `SegmentsService.List()` - Lists segments for a portfolio with pagination
- `SegmentsService.Get()` - Gets a segment by ID
- `SegmentsService.Create()` - Creates a new segment
- `SegmentsService.Update()` - Updates a segment
- `SegmentsService.Delete()` - Deletes a segment

### Transactions Service

- `TransactionsService.List()` - Lists transactions for a ledger with pagination
- `TransactionsService.Get()` - Gets a transaction by ID
- `TransactionsService.Create()` - Creates a new transaction
- `TransactionsService.Commit()` - Commits a pending transaction
- `TransactionsService.Cancel()` - Cancels a pending transaction

## III. Error Handling Package (`midaz/pkg/errors`)

The errors package provides standardized error types and utilities for working with errors in the Midaz SDK.

### Error Type

- `Error` - Core error type that contains detailed error information
  - `Category` - General category of the error (e.g., validation, not_found)
  - `Code` - Specific error code
  - `Message` - Human-readable error message
  - `Operation` - Operation that was being performed
  - `Resource` - Type of resource involved
  - `ResourceID` - Identifier of the resource involved
  - `StatusCode` - HTTP status code
  - `RequestID` - API request ID for debugging
  - `Err` - The underlying error

### Error Type Checking

- `IsValidationError(err error) bool` - Checks if the error is a validation error
- `IsNotFoundError(err error) bool` - Checks if the error is a not found error
- `IsAuthenticationError(err error) bool` - Checks if the error is an authentication error
- `IsAuthorizationError(err error) bool` - Checks if the error is an authorization error
- `IsPermissionError(err error) bool` - Checks if the error is a permission error
- `IsConflictError(err error) bool` - Checks if the error is a conflict error
- `IsAlreadyExistsError(err error) bool` - Checks if the error is an already exists error
- `IsRateLimitError(err error) bool` - Checks if the error is a rate limit error
- `IsTimeoutError(err error) bool` - Checks if the error is a timeout error
- `IsNetworkError(err error) bool` - Checks if the error is a network error
- `IsCancellationError(err error) bool` - Checks if the error is a cancellation error
- `IsInternalError(err error) bool` - Checks if the error is an internal error

### Transaction Error Checking

- `IsInsufficientBalanceError(err error) bool` - Checks if the error is an insufficient balance error
- `IsAccountEligibilityError(err error) bool` - Checks if the error is an account eligibility error
- `IsAssetMismatchError(err error) bool` - Checks if the error is an asset mismatch error
- `IsIdempotencyError(err error) bool` - Checks if the error is an idempotency error

### Error Utilities

- `GetErrorCategory(err error) ErrorCategory` - Gets the category of an error
- `GetStatusCode(err error) int` - Gets the HTTP status code for an error
- `FormatErrorForDisplay(err error) string` - Formats an error for display to end users
- `FormatTransactionError(err error, operationType string) string` - Formats a transaction error with operation context
- `CategorizeTransactionError(err error) string` - Returns a string indicating the category of a transaction error
- `ErrorFromHTTPResponse(statusCode int, requestID, message, code, entityType, resourceID string) error` - Creates an error from HTTP response details

### Error Creation

- `NewValidationError(operation, message string, err error) *Error` - Creates a validation error
- `NewInvalidInputError(operation string, err error) *Error` - Creates an invalid input error
- `NewNotFoundError(operation, resource, resourceID string, err error) *Error` - Creates a not found error
- `NewAuthenticationError(operation, message string, err error) *Error` - Creates an authentication error
- `NewAuthorizationError(operation, message string, err error) *Error` - Creates an authorization error
- `NewConflictError(operation, resource, resourceID string, err error) *Error` - Creates a conflict error
- `NewRateLimitError(operation, message string, err error) *Error` - Creates a rate limit error
- `NewTimeoutError(operation, message string, err error) *Error` - Creates a timeout error
- `NewInternalError(operation string, err error) *Error` - Creates an internal error
- `NewInsufficientBalanceError(operation, accountID string, err error) *Error` - Creates an insufficient balance error
- `NewAssetMismatchError(operation, expected, actual string, err error) *Error` - Creates an asset mismatch error
- `NewAccountEligibilityError(operation, accountID string, err error) *Error` - Creates an account eligibility error

## IV. Configuration Package (`midaz/pkg/config`)

The config package provides configuration management for the Midaz SDK.

### Core Functions and Types

- `NewConfig(options ...Option) (*Config, error)` - Creates a new configuration with the provided options
- `DefaultConfig() *Config` - Creates a new configuration with default values
- `NewLocalConfig(authToken string, options ...Option) (*Config, error)` - Creates a configuration for local development

### Configuration Options

- `WithEnvironment(env Environment) Option` - Sets the environment (local, development, production)
- `WithAuthToken(token string) Option` - Sets the authentication token
- `WithOnboardingURL(url string) Option` - Sets the base URL for the Onboarding API
- `WithTransactionURL(url string) Option` - Sets the base URL for the Transaction API
- `WithBaseURL(baseURL string) Option` - Sets a common base URL for all services
- `WithHTTPClient(client *http.Client) Option` - Sets a custom HTTP client
- `WithTimeout(timeout time.Duration) Option` - Sets the timeout for HTTP requests
- `WithUserAgent(userAgent string) Option` - Sets the user agent for HTTP requests
- `WithRetryConfig(maxRetries int, minWait, maxWait time.Duration) Option` - Sets the retry configuration
- `WithMaxRetries(maxRetries int) Option` - Sets the maximum number of retries
- `WithRetryWaitMin(waitTime time.Duration) Option` - Sets the minimum wait time between retries
- `WithRetryWaitMax(waitTime time.Duration) Option` - Sets the maximum wait time between retries
- `WithRetries(enable bool) Option` - Enables or disables retries
- `WithDebug(enable bool) Option` - Enables or disables debug mode
- `WithObservabilityProvider(provider observability.Provider) Option` - Sets the observability provider
- `WithIdempotency(enable bool) Option` - Enables or disables automatic idempotency key generation
- `FromEnvironment() Option` - Loads configuration from environment variables

### Config Methods

- `Config.GetBaseURLs() map[string]string` - Returns a map of service names to URLs
- `Config.GetHTTPClient() *http.Client` - Returns the HTTP client
- `Config.GetAuthToken() string` - Returns the authentication token
- `Config.GetObservabilityProvider() observability.Provider` - Returns the observability provider

## V. Concurrent Package (`midaz/pkg/concurrent`)

The concurrent package provides utilities for working with concurrent operations in the Midaz SDK.

### Worker Pool

- `WorkerPool[T, R any](ctx context.Context, items []T, workFn WorkFunc[T, R], opts ...PoolOption) []Result[T, R]` - Creates a pool of workers for parallel processing
- `Batch[T, R any](ctx context.Context, items []T, batchSize int, workFn func(ctx context.Context, batch []T) ([]R, error), opts ...PoolOption) []Result[T, R]` - Processes items in batches using a worker pool
- `ForEach[T any](ctx context.Context, items []T, fn func(ctx context.Context, item T) error, opts ...PoolOption) error` - Executes a function for each item in parallel

### Worker Pool Options

- `WithWorkers(workers int) PoolOption` - Sets the number of worker goroutines
- `WithBufferSize(size int) PoolOption` - Sets the size of the channel buffers
- `WithUnorderedResults() PoolOption` - Configures the pool to return results as they are completed
- `WithRateLimit(operationsPerSecond int) PoolOption` - Sets the maximum number of operations per second
- `WithWaitGroup(wg *sync.WaitGroup) PoolOption` - Creates a worker pool that utilizes an external wait group

### Rate Limiting

- `NewRateLimiter(opsPerSecond int, maxBurst int) *RateLimiter` - Creates a new rate limiter
- `RateLimiter.Wait(ctx context.Context) error` - Blocks until a token is available or the context is cancelled
- `RateLimiter.Stop()` - Stops the rate limiter and releases resources

### Types

- `WorkFunc[T, R any]` - A generic worker function that processes an item and returns a result and error
- `Result[T, R any]` - Holds the result of a processed item along with any error that occurred
- `RateLimiter` - Provides a simple mechanism to limit the rate of operations

## VI. Observability Package (`midaz/pkg/observability`)

The observability package provides utilities for adding observability capabilities to the Midaz SDK, including metrics, logging, and distributed tracing.

### Core Functions and Types

- `New(ctx context.Context, opts ...Option) (Provider, error)` - Creates a new observability provider
- `NewWithConfig(ctx context.Context, config *Config) (Provider, error)` - Creates a provider with the given configuration
- `WithSpan(ctx context.Context, provider Provider, name string, fn func(context.Context) error, opts ...trace.SpanStartOption) error` - Executes a function within a new span
- `RecordMetric(ctx context.Context, provider Provider, name string, value float64, attrs ...attribute.KeyValue)` - Records a metric
- `RecordDuration(ctx context.Context, provider Provider, name string, start time.Time, attrs ...attribute.KeyValue)` - Records a duration metric
- `ExtractContext(ctx context.Context, headers map[string]string) context.Context` - Extracts context from HTTP headers
- `InjectContext(ctx context.Context, headers map[string]string)` - Injects context into HTTP headers

### Provider Interface

- `Provider.Tracer() trace.Tracer` - Returns a tracer for creating spans
- `Provider.Meter() metric.Meter` - Returns a meter for creating metrics
- `Provider.Logger() Logger` - Returns a logger
- `Provider.Shutdown(ctx context.Context) error` - Gracefully shuts down the provider
- `Provider.IsEnabled() bool` - Returns true if observability is enabled

### Configuration Options

- `WithServiceName(name string) Option` - Sets the service name for observability
- `WithServiceVersion(version string) Option` - Sets the service version
- `WithSDKVersion(version string) Option` - Sets the SDK version
- `WithEnvironment(env string) Option` - Sets the environment (production, development, etc.)
- `WithCollectorEndpoint(endpoint string) Option` - Sets the OpenTelemetry collector endpoint
- `WithLogLevel(level LogLevel) Option` - Sets the minimum log level
- `WithLogOutput(output io.Writer) Option` - Sets the writer for logs
- `WithTraceSampleRate(rate float64) Option` - Sets the trace sampling rate
- `WithComponentEnabled(tracing, metrics, logging bool) Option` - Enables/disables components
- `WithAttributes(attrs ...attribute.KeyValue) Option` - Adds attributes to telemetry
- `WithHighTracingSampling() Option` - Sets a high trace sampling rate (0.5)
- `WithFullTracingSampling() Option` - Sets a full trace sampling rate (1.0)
- `WithDevelopmentDefaults() Option` - Sets reasonable defaults for development
- `WithProductionDefaults() Option` - Sets reasonable defaults for production

### Logging Interface

- `Logger.Debug(args ...interface{})` - Logs at debug level
- `Logger.Debugf(format string, args ...interface{})` - Logs formatted message at debug level
- `Logger.Info(args ...interface{})` - Logs at info level
- `Logger.Infof(format string, args ...interface{})` - Logs formatted message at info level
- `Logger.Warn(args ...interface{})` - Logs at warn level
- `Logger.Warnf(format string, args ...interface{})` - Logs formatted message at warn level
- `Logger.Error(args ...interface{})` - Logs at error level
- `Logger.Errorf(format string, args ...interface{})` - Logs formatted message at error level
- `Logger.Fatal(args ...interface{})` - Logs at fatal level and exits
- `Logger.Fatalf(format string, args ...interface{})` - Logs formatted message at fatal level and exits
- `Logger.With(fields map[string]interface{}) Logger` - Returns a logger with added fields
- `Logger.WithContext(ctx trace.SpanContext) Logger` - Returns a logger with context information
- `Logger.WithSpan(span trace.Span) Logger` - Returns a logger with span information

## VII. Pagination Package (`midaz/pkg/pagination`)

The pagination package provides utilities for working with paginated API responses.

### Core Functions and Types

- `NewPaginator[T any](fetcher PageFetcher[T], options ...PaginatorOption) (Paginator[T], error)` - Creates a new paginator
- `NewPaginatorWithDefaults[T any](operationName, entityType string, fetcher PageFetcher[T], initialOptions PageOptions, observer Observer) Paginator[T]` - Creates a paginator with minimal parameters
- `CollectAll[T any](ctx context.Context, operationName, entityType string, fetcher PageFetcher[T], options PageOptions) ([]T, error)` - Creates a paginator and collects all items
- `NewObserver() Observer` - Creates a new pagination observer
- `GetEntityTypeFromURL(url string) string` - Extracts the entity type from a URL

### Paginator Interface

- `Paginator[T].Next(ctx context.Context) bool` - Advances to the next page of results
- `Paginator[T].Items() []T` - Returns the items in the current page
- `Paginator[T].Err() error` - Returns any error that occurred during pagination
- `Paginator[T].PageInfo() PageInfo` - Returns information about the current page
- `Paginator[T].All(ctx context.Context) ([]T, error)` - Retrieves all remaining items
- `Paginator[T].ForEach(ctx context.Context, fn func(item T) error) error` - Iterates through all items
- `Paginator[T].Concurrent(ctx context.Context, workers int, fn func(item T) error) error` - Processes items concurrently

### Paginator Options

- `WithLimit(limit int) PaginatorOption` - Sets the initial page limit
- `WithOffset(offset int) PaginatorOption` - Sets the initial page offset
- `WithCursor(cursor string) PaginatorOption` - Sets the initial cursor
- `WithFilters(filters map[string]string) PaginatorOption` - Sets the initial filters
- `WithPageOptions(options PageOptions) PaginatorOption` - Sets all initial page options
- `WithObserver(observer Observer) PaginatorOption` - Sets the observer for monitoring
- `WithOperationName(operationName string) PaginatorOption` - Sets the operation name
- `WithEntityType(entityType string) PaginatorOption` - Sets the entity type
- `WithWorkerCount(workerCount int) PaginatorOption` - Sets the number of workers
- `WithDefaultLimit(defaultLimit int) PaginatorOption` - Sets the default limit

### Types

- `PageFetcher[T any]` - A function type for fetching a page of results
- `PageOptions` - Options for fetching a page (limit, offset, cursor, filters)
- `PageResult[T any]` - Represents a single page of results
- `PageInfo` - Contains information about the current page
- `Observer` - Interface for observing pagination events
- `Event` - Represents a pagination operation event

## VIII. Retry Package (`midaz/pkg/retry`)

The retry package provides utilities for implementing retry logic with exponential backoff and jitter for resilient operations.

### Core Functions

- `Do(ctx context.Context, fn func() error, opts ...Option) error` - Executes a function with retries
- `DoWithContext(ctx context.Context, fn func() error) error` - Executes a function with retry options from context
- `IsRetryableError(err error, options *Options) bool` - Checks if an error is retryable
- `WithOptionsContext(ctx context.Context, options *Options) context.Context` - Returns a context with retry options
- `GetOptionsFromContext(ctx context.Context) *Options` - Gets retry options from context

### Retry Options

- `WithMaxRetries(maxRetries int) Option` - Sets the maximum number of retry attempts
- `WithInitialDelay(delay time.Duration) Option` - Sets the initial delay before first retry
- `WithMaxDelay(delay time.Duration) Option` - Sets the maximum delay between retries
- `WithBackoffFactor(factor float64) Option` - Sets the factor by which to increase delay
- `WithRetryableErrors(errors []string) Option` - Sets the error strings that trigger retry
- `WithRetryableHTTPCodes(codes []int) Option` - Sets the HTTP status codes that trigger retry
- `WithJitterFactor(factor float64) Option` - Sets the amount of jitter to add to delay
- `WithHighReliability() Option` - Configures options for high reliability
- `WithNoRetry() Option` - Disables retries

### HTTP Retry Functions

- `DoHTTPRequest(ctx context.Context, client *http.Client, req *http.Request, opts ...HTTPOption) (*HTTPResponse, error)` - Performs HTTP request with retries
- `DoHTTPRequestWithContext(ctx context.Context, client *http.Client, req *http.Request) (*HTTPResponse, error)` - Performs HTTP request with options from context
- `DoHTTP(ctx context.Context, client *http.Client, method, url string, body io.Reader, opts ...HTTPOption) (*HTTPResponse, error)` - Simplified HTTP request with retries
- `WithHTTPOptionsContext(ctx context.Context, options *HTTPOptions) context.Context` - Returns context with HTTP retry options
- `GetHTTPOptionsFromContext(ctx context.Context) *HTTPOptions` - Gets HTTP retry options from context

### HTTP Retry Options

- `WithHTTPMaxRetries(maxRetries int) HTTPOption` - Sets the maximum number of retry attempts
- `WithHTTPInitialDelay(delay time.Duration) HTTPOption` - Sets the initial delay before first retry
- `WithHTTPMaxDelay(delay time.Duration) HTTPOption` - Sets the maximum delay between retries
- `WithHTTPBackoffFactor(factor float64) HTTPOption` - Sets the factor by which to increase delay
- `WithHTTPRetryableHTTPCodes(codes []int) HTTPOption` - Sets HTTP status codes for retry
- `WithHTTPRetryableNetworkErrors(errors []string) HTTPOption` - Sets network errors for retry
- `WithHTTPRetryAllServerErrors(retry bool) HTTPOption` - Sets whether to retry all 5xx errors
- `WithHTTPRetryOn4xx(codes []int) HTTPOption` - Sets 4xx status codes to retry
- `WithHTTPPreRetryHook(hook func(ctx context.Context, req *http.Request, resp *HTTPResponse) error) HTTPOption` - Sets hook before retry
- `WithHTTPJitterFactor(factor float64) HTTPOption` - Sets jitter factor for HTTP retries
- `WithHTTPHighReliability() HTTPOption` - Configures HTTP options for high reliability
- `WithHTTPNoRetry() HTTPOption` - Disables HTTP retries

## IX. Validation Package (`midaz/pkg/validation`)

The validation package provides utilities for validating various aspects of Midaz data.

### Core Validation Functions

- `DefaultValidator() *Validator` - Returns a validator with default configuration
- `NewValidator(options ...core.ValidationOption) (*Validator, error)` - Creates a validator with options
- `ValidateTransactionDSL(input TransactionDSLValidator) error` - Validates transaction DSL input
- `ValidateAssetCode(assetCode string) error` - Validates asset code format (3-4 uppercase letters)
- `ValidateAccountAlias(alias string) error` - Validates account alias format
- `ValidateTransactionCode(code string) error` - Validates transaction code format
- `ValidateMetadata(metadata map[string]any) error` - Validates metadata types and structure
- `ValidateDateRange(start, end time.Time) error` - Validates that start is not after end
- `ValidateCreateTransactionInput(input map[string]any) ValidationSummary` - Comprehensive validation
- `ValidateAssetType(assetType string) error` - Validates asset type is supported
- `ValidateAccountType(accountType string) error` - Validates account type is supported
- `ValidateCurrencyCode(code string) error` - Validates ISO 4217 currency code
- `ValidateCountryCode(code string) error` - Validates ISO 3166-1 alpha-2 country code
- `ValidateAddress(address *Address) error` - Validates address structure
- `GetExternalAccountReference(assetCode string) string` - Creates external account reference

### Validation Methods

- `Validator.ValidateMetadata(metadata map[string]any) error` - Validates metadata with configuration
- `Validator.ValidateAddress(address *Address) error` - Validates address with configuration

### Validation Configuration Options

- `core.WithMaxMetadataSize(size int) ValidationOption` - Sets maximum metadata size
- `core.WithMaxStringLength(length int) ValidationOption` - Sets maximum string length
- `core.WithMaxAddressLineLength(length int) ValidationOption` - Sets maximum address line length
- `core.WithMaxZipCodeLength(length int) ValidationOption` - Sets maximum zip code length
- `core.WithMaxCityLength(length int) ValidationOption` - Sets maximum city name length
- `core.WithMaxStateLength(length int) ValidationOption` - Sets maximum state name length
- `core.WithStrictMode(strict bool) ValidationOption` - Enables additional validation checks

### Validation Results

- `ValidationSummary` - Holds results of validation with multiple potential errors
- `ValidationSummary.AddError(err error)` - Adds an error to the validation summary
- `ValidationSummary.GetErrorMessages() []string` - Returns all error messages as strings
- `ValidationSummary.GetErrorSummary() string` - Returns a single string with all errors

## X. Models Package (`midaz/models`)

The models package defines the data structures used throughout the SDK.

### Resource Models

- `Organization` - Represents an organization in the system
- `Ledger` - Represents a ledger in the system
- `Account` - Represents an account in the system
- `Asset` - Represents an asset in the system
- `AssetRate` - Represents an exchange rate between assets
- `Portfolio` - Represents a portfolio in the system
- `Segment` - Represents a segment in the system
- `Transaction` - Represents a transaction in the system
- `Operation` - Represents an operation within a transaction
- `Balance` - Represents an account balance

### Input Models

- `CreateOrganizationInput` - Input for creating an organization
- `UpdateOrganizationInput` - Input for updating an organization
- `CreateLedgerInput` - Input for creating a ledger
- `UpdateLedgerInput` - Input for updating a ledger
- `CreateAccountInput` - Input for creating an account
- `UpdateAccountInput` - Input for updating an account
- `CreateAssetInput` - Input for creating an asset
- `UpdateAssetInput` - Input for updating an asset
- `CreateAssetRateInput` - Input for creating an asset rate
- `UpdateAssetRateInput` - Input for updating an asset rate
- `CreatePortfolioInput` - Input for creating a portfolio
- `UpdatePortfolioInput` - Input for updating a portfolio
- `CreateSegmentInput` - Input for creating a segment
- `UpdateSegmentInput` - Input for updating a segment
- `CreateTransactionInput` - Input for creating a transaction
- `CreateOperationInput` - Input for creating an operation
- `UpdateBalanceInput` - Input for updating a balance

### Response Models

- `ListResponse` - Generic paginated response for list operations
- `ListOptions` - Options for list operations

### Common Types and Constants

- `Status` - Enum for resource status
- `Address` - Struct for address information
- `TransactionStatus` constants - Pending, Committed, Canceled
- `OperationType` constants - Debit, Credit
- `AssetType` constants - Currency, Stock, Crypto