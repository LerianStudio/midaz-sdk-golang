# Midaz Go SDK public API map

This map documents the recommended public SDK surface that consumers should use. It is not a replacement for generated Go docs. The root module is `github.com/LerianStudio/midaz-sdk-golang/v2`; the Go package name is `client`.

## Root package

### Constructor

- `client.New(options ...client.Option) (*client.Client, error)` - Creates an SDK client.

### Client options

- `client.WithConfig(*config.Config)` - Uses a pre-built SDK configuration.
- `client.WithBaseURL(string)` - Sets a shared base URL and derives Ledger/CRM service URLs.
- `client.WithOnboardingURL(string)` - Sets the onboarding alias URL.
- `client.WithTransactionURL(string)` - Sets the transaction alias URL.
- `client.WithCRMURL(string)` - Sets the CRM service URL.
- `client.WithEnvironment(config.Environment)` - Uses a named environment preset.
- `client.WithHTTPClient(*http.Client)` - Supplies a custom HTTP client.
- `client.WithTimeout(time.Duration)` - Sets request timeout.
- `client.WithRetries(maxRetries int, initialDelay, maxDelay time.Duration)` - Configures retry behavior.
- `client.WithCustomRetryPolicy(*retry.Options)` - Uses a custom retry policy.
- `client.DisableRetries()` - Disables client retries.
- `client.WithDebug(bool)` - Enables or disables debug logging.
- `client.WithTenantID(string)` - Sets a default tenant ID on entity clients.
- `client.WithContext(context.Context)` - Sets the client base context.
- `client.WithObservability(tracing, metrics, logging bool)` - Enables SDK-created observability components.
- `client.WithObservabilityOptions(...observability.Option)` - Configures SDK-created observability.
- `client.WithObservabilityProvider(observability.Provider)` - Uses an existing observability provider.
- `client.WithCollectorEndpoint(string)` - Configures the OTLP collector endpoint for SDK-created observability.
- `client.UseAllAPIs()` - Enables all currently supported API surfaces.
- `client.UseEntityAPI()` - Enables the entity API surface.
- `client.UseEntity()` - Compatibility alias for `UseEntityAPI()`.

### Client fields and methods

- `Client.Entity` - Entity service access point. Initialized only when entity APIs are enabled.
- `Client.GetVersion() string` - Returns the SDK version.
- `Client.Shutdown(context.Context) error` - Releases observability resources.
- `Client.Trace(name string, fn func(context.Context) error) error` - Runs a function inside a span when observability is enabled.
- `Client.Logger() observability.Logger` - Returns the configured logger.
- `Client.GetObservabilityProvider() observability.Provider` - Returns the active observability provider.
- `Client.GetMetricsCollector() *observability.MetricsCollector` - Returns the metrics collector when available.
- `Client.GetContext() context.Context` - Returns the client base context.
- `Client.GetConfiguration() *config.Config` - Returns the current configuration.
- `Client.GetConfig() *config.Config` - Compatibility alias for `GetConfiguration`.

## Configuration package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config`.

### Constructors and options

- `config.DefaultConfig() *config.Config`
- `config.NewConfig(options ...config.Option) (*config.Config, error)`
- `config.NewLocalConfig(options ...config.Option) (*config.Config, error)`
- `config.FromEnvironment() config.Option`
- `config.WithEnvironment(config.Environment) config.Option`
- `config.WithBaseURL(string) config.Option`
- `config.WithOnboardingURL(string) config.Option`
- `config.WithTransactionURL(string) config.Option`
- `config.WithCRMURL(string) config.Option`
- `config.WithAccessManager(auth.AccessManager) config.Option`
- `config.WithHTTPClient(*http.Client) config.Option`
- `config.WithTimeout(time.Duration) config.Option`
- `config.WithUserAgent(string) config.Option`
- `config.WithRetryConfig(maxRetries int, minWait, maxWait time.Duration) config.Option`
- `config.WithMaxRetries(int) config.Option`
- `config.WithRetryWaitMin(time.Duration) config.Option`
- `config.WithRetryWaitMax(time.Duration) config.Option`
- `config.WithRetries(bool) config.Option`
- `config.WithDebug(bool) config.Option`
- `config.WithTenantID(string) config.Option`
- `config.WithIdempotency(bool) config.Option`
- `config.WithObservabilityProvider(observability.Provider) config.Option`

### Environment variables read by `config.FromEnvironment`

- `MIDAZ_ENVIRONMENT`
- `MIDAZ_BASE_URL`
- `MIDAZ_ONBOARDING_URL`
- `MIDAZ_TRANSACTION_URL`
- `MIDAZ_CRM_URL`
- `MIDAZ_USER_AGENT`
- `MIDAZ_TIMEOUT`
- `MIDAZ_DEBUG`
- `MIDAZ_MAX_RETRIES`
- `MIDAZ_IDEMPOTENCY`
- `PLUGIN_AUTH_ENABLED`
- `PLUGIN_AUTH_ADDRESS`
- `MIDAZ_CLIENT_ID`
- `MIDAZ_CLIENT_SECRET`

## Access Manager package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager` with alias `auth`.

- `auth.AccessManager` - Plugin authentication configuration with `Enabled`, `Address`, `ClientID`, and `ClientSecret`.
- `config.WithAccessManager(auth.AccessManager)` - Attaches Access Manager configuration to SDK config.

## Entity package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/entities`. Consumers usually access services through `c.Entity`.

### Entity access point

- `Entity.Accounts`
- `Entity.AccountTypes`
- `Entity.Assets`
- `Entity.AssetRates`
- `Entity.Balances`
- `Entity.Holders`
- `Entity.Aliases`
- `Entity.Ledgers`
- `Entity.MetadataIndexes`
- `Entity.Operations`
- `Entity.OperationRoutes`
- `Entity.Organizations`
- `Entity.Portfolios`
- `Entity.Segments`
- `Entity.Transactions`
- `Entity.TransactionRoutes`

### OrganizationsService

- `ListOrganizations(ctx, opts)`
- `GetOrganization(ctx, id)`
- `CreateOrganization(ctx, input)`
- `UpdateOrganization(ctx, id, input)`
- `DeleteOrganization(ctx, id)`

### LedgersService

- `ListLedgers(ctx, organizationID, opts)`
- `GetLedger(ctx, organizationID, id)`
- `CreateLedger(ctx, organizationID, input)`
- `UpdateLedger(ctx, organizationID, id, input)`
- `DeleteLedger(ctx, organizationID, id)`

### AccountsService

- `ListAccounts(ctx, organizationID, ledgerID, opts)`
- `GetAccount(ctx, organizationID, ledgerID, id)`
- `GetAccountByAlias(ctx, organizationID, ledgerID, alias)`
- `CreateAccount(ctx, organizationID, ledgerID, input)`
- `UpdateAccount(ctx, organizationID, ledgerID, id, input)`
- `DeleteAccount(ctx, organizationID, ledgerID, id)`
- `GetBalance(ctx, organizationID, ledgerID, accountID)`
- `GetAccountsMetricsCount(ctx, organizationID, ledgerID)`
- `GetExternalAccount(ctx, organizationID, ledgerID, assetCode)`
- `GetExternalAccountBalance(ctx, organizationID, ledgerID, assetCode)`
- `GetAccountByAliasPath(ctx, organizationID, ledgerID, alias)`

### AccountTypesService

- `ListAccountTypes(ctx, organizationID, ledgerID, opts)`
- `GetAccountType(ctx, organizationID, ledgerID, id)`
- `CreateAccountType(ctx, organizationID, ledgerID, input)`
- `UpdateAccountType(ctx, organizationID, ledgerID, id, input)`
- `DeleteAccountType(ctx, organizationID, ledgerID, id)`
- `GetAccountTypesMetricsCount(ctx, organizationID, ledgerID)`

### AssetsService

- `ListAssets(ctx, organizationID, ledgerID, opts)`
- `GetAsset(ctx, organizationID, ledgerID, id)`
- `CreateAsset(ctx, organizationID, ledgerID, input)`
- `UpdateAsset(ctx, organizationID, ledgerID, id, input)`
- `DeleteAsset(ctx, organizationID, ledgerID, id)`

### AssetRatesService

- `CreateOrUpdateAssetRate(ctx, organizationID, ledgerID, input)`
- `GetAssetRate(ctx, organizationID, ledgerID, externalID)`
- `ListAssetRatesByAssetCode(ctx, organizationID, ledgerID, assetCode, opts)`

### BalancesService

- `ListBalances(ctx, orgID, ledgerID, opts)`
- `ListAccountBalances(ctx, orgID, ledgerID, accountID, opts)`
- `GetBalance(ctx, orgID, ledgerID, balanceID)`
- `GetBalanceHistory(ctx, orgID, ledgerID, balanceID, date)`
- `UpdateBalance(ctx, orgID, ledgerID, balanceID, input)`
- `DeleteBalance(ctx, orgID, ledgerID, balanceID)`
- `CreateBalance(ctx, orgID, ledgerID, accountID, input)`
- `ListBalancesByAccountAlias(ctx, orgID, ledgerID, alias, opts)`
- `ListBalancesByExternalCode(ctx, orgID, ledgerID, code, opts)`
- `GetAccountBalancesHistory(ctx, orgID, ledgerID, accountID, date)`

### PortfoliosService

- `ListPortfolios(ctx, organizationID, ledgerID, opts)`
- `GetPortfolio(ctx, organizationID, ledgerID, id)`
- `CreatePortfolio(ctx, organizationID, ledgerID, input)`
- `UpdatePortfolio(ctx, organizationID, ledgerID, id, input)`
- `DeletePortfolio(ctx, organizationID, ledgerID, id)`

### SegmentsService

- `ListSegments(ctx, organizationID, ledgerID, opts)`
- `GetSegment(ctx, organizationID, ledgerID, id)`
- `CreateSegment(ctx, organizationID, ledgerID, input)`
- `UpdateSegment(ctx, organizationID, ledgerID, id, input)`
- `DeleteSegment(ctx, organizationID, ledgerID, id)`

### OperationsService

- `ListOperations(ctx, orgID, ledgerID, accountID, opts)`
- `GetOperation(ctx, orgID, ledgerID, accountID, operationID)`
- `UpdateTransactionOperation(ctx, orgID, ledgerID, transactionID, operationID, input)`
- `UpdateOperation(ctx, orgID, ledgerID, accountID, operationID, input)` - Deprecated compatibility method.

### OperationRoutesService

- `ListOperationRoutes(ctx, organizationID, ledgerID, opts)`
- `GetOperationRoute(ctx, organizationID, ledgerID, operationRouteID)`
- `CreateOperationRoute(ctx, organizationID, ledgerID, input)`
- `UpdateOperationRoute(ctx, organizationID, ledgerID, operationRouteID, input)`
- `DeleteOperationRoute(ctx, organizationID, ledgerID, operationRouteID)`

### TransactionRoutesService

- `ListTransactionRoutes(ctx, organizationID, ledgerID, opts)`
- `GetTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID)`
- `CreateTransactionRoute(ctx, organizationID, ledgerID, input)`
- `UpdateTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID, input)`
- `DeleteTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID)`

### TransactionsService

- `CreateTransaction(ctx, orgID, ledgerID, input)`
- `CreateTransactionWithDSL(ctx, orgID, ledgerID, input)`
- `CreateTransactionWithDSLFile(ctx, orgID, ledgerID, dslContent)`
- `GetTransaction(ctx, orgID, ledgerID, transactionID)`
- `ListTransactions(ctx, orgID, ledgerID, opts)`
- `GetTransactionsMetricsCount(ctx, orgID, ledgerID, opts)`
- `UpdateTransaction(ctx, orgID, ledgerID, transactionID, input)`
- `RevertTransaction(ctx, orgID, ledgerID, transactionID)`
- `CommitTransaction(ctx, orgID, ledgerID, transactionID)`
- `CancelTransaction(ctx, orgID, ledgerID, transactionID)`
- `CancelTransactionWithResponse(ctx, orgID, ledgerID, transactionID)`
- `CreateInflowTransaction(ctx, orgID, ledgerID, input)`
- `CreateOutflowTransaction(ctx, orgID, ledgerID, input)`
- `CreateAnnotationTransaction(ctx, orgID, ledgerID, input)`

### MetadataIndexesService

- `ListMetadataIndexes(ctx, entityName)`
- `CreateMetadataIndex(ctx, entityName, input)`
- `DeleteMetadataIndex(ctx, entityName, metadataKey)`

### CRM services

CRM services use the CRM base URL and set the organization through the `X-Organization-Id` header.

#### HoldersService

- `ListHolders(ctx, organizationID, opts)`
- `CreateHolder(ctx, organizationID, input)`
- `GetHolder(ctx, organizationID, holderID, includeDeleted)`
- `UpdateHolder(ctx, organizationID, holderID, input)`
- `DeleteHolder(ctx, organizationID, holderID, hardDelete)`

#### AliasesService

- `ListAliases(ctx, organizationID, opts)`
- `CreateAlias(ctx, organizationID, holderID, input)`
- `GetAlias(ctx, organizationID, holderID, aliasID, includeDeleted)`
- `UpdateAlias(ctx, organizationID, holderID, aliasID, input)`
- `DeleteAlias(ctx, organizationID, holderID, aliasID, hardDelete)`
- `DeleteRelatedParty(ctx, organizationID, holderID, aliasID, relatedPartyID)`

## Models package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/models`.

### List and pagination

- `models.NewListOptions()`
- `(*models.ListOptions).WithLimit(int)`
- `(*models.ListOptions).WithOffset(int)` - Compatibility input; query serialization uses `page`.
- `(*models.ListOptions).WithPage(int)`
- `(*models.ListOptions).WithCursor(string)`
- `(*models.ListOptions).WithOrderBy(string)` - Stored for compatibility; not sent by common query serialization.
- `(*models.ListOptions).WithOrderDirection(models.SortDirection)` - Sent as `sort_order`.
- `(*models.ListOptions).WithFilter(key, value string)`
- `(*models.ListOptions).WithFilters(map[string]string)`
- `(*models.ListOptions).WithDateRange(startDate, endDate string)`
- `(*models.ListOptions).WithAdditionalParam(key, value string)`
- `(*models.ListOptions).WithIncludeDeleted(bool)`
- `(*models.ListOptions).WithHolderID(string)`
- `(*models.ListOptions).WithExternalID(string)`
- `(*models.ListOptions).WithDocument(string)`
- `(*models.ListOptions).WithAccountID(string)`
- `(*models.ListOptions).WithLedgerID(string)`
- `(*models.ListOptions).WithParticipantDocument(string)`
- `(*models.ListOptions).WithRelatedPartyDocument(string)`
- `(*models.ListOptions).ToQueryParams()`
- `(*models.Pagination).HasNextPage()`
- `(*models.Pagination).NextPageOptions()`
- `(*models.Pagination).HasPrevPage()`
- `(*models.Pagination).PrevPageOptions()`
- `(*models.Pagination).CurrentPage()`
- `(*models.Pagination).TotalPages()`

### Common input builders

- `models.NewCreateOrganizationInput(legalName, legalDocument)`
- `models.NewUpdateOrganizationInput()`
- `models.NewCreateAssetInput(name, code)`
- `models.NewUpdateAssetInput()`
- `models.NewCreateTransactionInput(assetCode, amount)`
- `models.NewCreateAccountInput(name, assetCode, accountType)`
- `models.NewAssetRateListOptions()`

## Errors package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors`.

- Core type: `*errors.Error`.
- Sentinel errors: `ErrValidation`, `ErrAuthentication`, `ErrPermission`, `ErrNotFound`, `ErrAlreadyExists`, `ErrIdempotency`, `ErrRateLimit`, `ErrTimeout`, `ErrCancellation`, `ErrInternal`, `ErrInsufficientBalance`, `ErrAccountEligibility`, `ErrAssetMismatch`.
- Checkers: `IsValidationError`, `IsNotFoundError`, `IsAuthenticationError`, `IsAuthorizationError`, `IsPermissionError`, `IsConflictError`, `IsAlreadyExistsError`, `IsRateLimitError`, `IsTimeoutError`, `IsNetworkError`, `IsCancellationError`, `IsInternalError`, `IsInsufficientBalanceError`, `IsAccountEligibilityError`, `IsAssetMismatchError`, `IsIdempotencyError`.
- Accessors: `GetErrorCategory`, `GetStatusCode`, `GetErrorCode`, `GetErrorDetails`, `GetTransactionErrorContext`.
- Constructors: `NewValidationError`, `NewInvalidInputError`, `NewNotFoundError`, `NewAuthenticationError`, `NewAuthorizationError`, `NewConflictError`, `NewRateLimitError`, `NewTimeoutError`, `NewInternalError`.

## Observability package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability`.

- `observability.New(ctx, opts...)`
- `observability.WithServiceName(string)`
- `observability.WithServiceVersion(string)`
- `observability.WithEnvironment(string)`
- `observability.WithCollectorEndpoint(string)` - OTLP gRPC endpoint in `host:port` form.
- `observability.WithSDKVersion(string)`
- `observability.WithLogLevel(observability.LogLevel)`
- `observability.WithTraceSampleRate(float64)`
- `observability.WithComponentEnabled(tracing, metrics, logging bool)`
- `observability.WithAttributes(...attribute.KeyValue)`
- `observability.WithPropagationHeaders(...string)`
- `observability.WithRegisterGlobally(bool)`
- `observability.WithProvider(context.Context, observability.Provider)`
- `observability.WithDevelopmentDefaults()`
- `observability.WithProductionDefaults()`
- `observability.ExtractContext(ctx, headers)`
- `observability.InjectContext(ctx, headers)`
- `observability.TraceID(ctx)`
- `observability.SpanID(ctx)`
- `observability.RecordMetric(ctx, provider, name, value, attrs...)`
- `observability.RecordDuration(ctx, provider, name, start, attrs...)`

## Retry package

Use `github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry`.

- `retry.Do(ctx, fn, opts...)`
- `retry.DoWithContext(ctx, fn)`
- `retry.IsRetryableError(err, options)`
- `retry.WithMaxRetries(int)`
- `retry.WithInitialDelay(time.Duration)`
- `retry.WithMaxDelay(time.Duration)`
- `retry.WithBackoffFactor(float64)`
- `retry.WithJitterFactor(float64)`
- `retry.WithRetryableHTTPCodes([]int)`
- `retry.WithHighReliability()`
- `retry.WithNoRetry()`
