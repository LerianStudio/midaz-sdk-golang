# Midaz Go SDK Internal API Map

This document provides a comprehensive overview of the internal APIs used by the Midaz Go SDK, organized by package and purpose. These APIs are not intended for direct use by SDK consumers but are documented here for SDK maintainers and contributors.

## Table of Contents

- [I. Client Package](#i-client-package-midazclient)
- [II. Resource Clients](#ii-resource-clients-midazclient)
- [III. Models Package](#iii-models-package-midazmodels)
- [IV. Error Handling](#iv-error-handling-midazerrors)
- [V. Implementation Patterns](#v-implementation-patterns)

## I. Client Package (`midaz/client`)

The client package provides the foundation for all API interactions, handling HTTP requests, authentication, and error processing.

### HTTP Client

- `httpClient` - Handles all HTTP requests to the Midaz API
  ```go
  type httpClient struct {
      baseURL       string
      authToken     string
      httpClient    *http.Client
      debug         bool
      userAgent     string
      retryOptions  *retry.Options
      jsonPool      *performance.JSONPool
      metrics       observability.MetricsProvider
      observability observability.Provider
  }
  ```

  - `Do(ctx context.Context, req *http.Request) (*http.Response, error)` - Executes an HTTP request with retries and error handling
    ```go
    // Implementation pattern
    func (c *httpClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
        // Add authentication headers
        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
        req.Header.Set("User-Agent", c.userAgent)
        
        // Execute request with retry logic
        var resp *http.Response
        var err error
        for attempt := 0; attempt < maxRetries; attempt++ {
            resp, err = c.httpClient.Do(req.WithContext(ctx))
            if err == nil || !isRetryableError(err) {
                break
            }
            time.Sleep(backoffDuration(attempt))
        }
        
        // Handle response errors
        if err != nil {
            return nil, err
        }
        
        if resp.StatusCode >= 400 {
            return resp, handleErrorResponse(resp)
        }
        
        return resp, nil
    }
    ```
  
  - `Get(ctx context.Context, path string, params url.Values) (*http.Response, error)` - Performs an HTTP GET request
  - `Post(ctx context.Context, path string, body interface{}) (*http.Response, error)` - Performs an HTTP POST request
  - `Put(ctx context.Context, path string, body interface{}) (*http.Response, error)` - Performs an HTTP PUT request
  - `Delete(ctx context.Context, path string) (*http.Response, error)` - Performs an HTTP DELETE request
  - `Patch(ctx context.Context, path string, body interface{}) (*http.Response, error)` - Performs an HTTP PATCH request

### API Client

- `apiClient` - Base client for all API operations
  ```go
  type apiClient struct {
      httpClient      *httpClient
      serviceURLs     map[string]string // Map of service names to URLs
      timeout         time.Duration
      debug           bool
      userAgent       string
      observability   observability.Provider
  }
  ```

  - `SetAuthToken(token string)` - Sets the authentication token
    ```go
    func (c *apiClient) SetAuthToken(token string) {
        c.httpClient.authToken = token
    }
    ```
  
  - `SetServiceURLs(serviceURLs map[string]string)` - Sets the service-specific URLs
    ```go
    func (c *apiClient) SetServiceURLs(serviceURLs map[string]string) {
        c.serviceURLs = serviceURLs
    }
    ```
  
  - `SetTimeout(seconds int)` - Sets the request timeout
  - `SetDebug(debug bool)` - Enables or disables debug mode
  - `SetUserAgent(userAgent string)` - Sets the user agent string used for API requests

## II. Resource Clients (`midaz/client`)

These clients handle specific resource operations. Each client encapsulates the API operations for a specific resource type.

### Organization Client

- `organizationClient` - Handles organization-related API operations
  ```go
  type organizationClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Organization], error)` - Lists organizations
    ```go
    // Implementation pattern
    func (c *organizationClient) List(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Organization], error) {
        path := "/organizations"
        params := url.Values{}
        
        // Add pagination parameters
        if opts != nil {
            if opts.Page > 0 {
                params.Set("page", strconv.Itoa(opts.Page))
            }
            if opts.Limit > 0 {
                params.Set("limit", strconv.Itoa(opts.Limit))
            }
            // Add other filter parameters
        }
        
        resp, err := c.apiClient.httpClient.Get(ctx, path, params)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        
        var result models.ListResponse[models.Organization]
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, err
        }
        
        return &result, nil
    }
    ```
  
  - `Get(ctx context.Context, id string) (*models.Organization, error)` - Gets an organization by ID
  - `Create(ctx context.Context, input *models.CreateOrganizationInput) (*models.Organization, error)` - Creates a new organization
  - `Update(ctx context.Context, id string, input *models.UpdateOrganizationInput) (*models.Organization, error)` - Updates an organization
  - `Delete(ctx context.Context, id string) error` - Deletes an organization

### Ledger Client

- `ledgerClient` - Handles ledger-related API operations
  ```go
  type ledgerClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID string, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error)` - Lists ledgers for an organization
  - `Get(ctx context.Context, organizationID, id string) (*models.Ledger, error)` - Gets a ledger by ID
  - `Create(ctx context.Context, organizationID string, input *models.CreateLedgerInput) (*models.Ledger, error)` - Creates a new ledger
  - `Update(ctx context.Context, organizationID, id string, input *models.UpdateLedgerInput) (*models.Ledger, error)` - Updates a ledger
  - `Delete(ctx context.Context, organizationID, id string) error` - Deletes a ledger

### Account Client

- `accountClient` - Handles account-related API operations
  ```go
  type accountClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Account], error)` - Lists accounts for a ledger
  - `Get(ctx context.Context, organizationID, ledgerID, id string) (*models.Account, error)` - Gets an account by ID
  - `GetByAlias(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error)` - Gets an account by alias
  - `Create(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error)` - Creates a new account
  - `Update(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountInput) (*models.Account, error)` - Updates an account
  - `Delete(ctx context.Context, organizationID, ledgerID, id string) error` - Deletes an account

### Asset Client

- `assetClient` - Handles asset-related API operations
  ```go
  type assetClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Asset], error)` - Lists assets for a ledger
  - `Get(ctx context.Context, organizationID, ledgerID, id string) (*models.Asset, error)` - Gets an asset by ID
  - `Create(ctx context.Context, organizationID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error)` - Creates a new asset
  - `Update(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error)` - Updates an asset
  - `Delete(ctx context.Context, organizationID, ledgerID, id string) error` - Deletes an asset


### Portfolio Client

- `portfolioClient` - Handles portfolio-related API operations
  ```go
  type portfolioClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Portfolio], error)` - Lists portfolios for a ledger
  - `Get(ctx context.Context, organizationID, ledgerID, id string) (*models.Portfolio, error)` - Gets a portfolio by ID
  - `Create(ctx context.Context, organizationID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error)` - Creates a new portfolio
  - `Update(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdatePortfolioInput) (*models.Portfolio, error)` - Updates a portfolio
  - `Delete(ctx context.Context, organizationID, ledgerID, id string) error` - Deletes a portfolio

### Segment Client

- `segmentClient` - Handles segment-related API operations
  ```go
  type segmentClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID, portfolioID string, opts *models.ListOptions) (*models.ListResponse[models.Segment], error)` - Lists segments for a portfolio
  - `Get(ctx context.Context, organizationID, ledgerID, portfolioID, id string) (*models.Segment, error)` - Gets a segment by ID
  - `Create(ctx context.Context, organizationID, ledgerID, portfolioID string, input *models.CreateSegmentInput) (*models.Segment, error)` - Creates a new segment
  - `Update(ctx context.Context, organizationID, ledgerID, portfolioID, id string, input *models.UpdateSegmentInput) (*models.Segment, error)` - Updates a segment
  - `Delete(ctx context.Context, organizationID, ledgerID, portfolioID, id string) error` - Deletes a segment

### Transaction Client

- `transactionClient` - Handles transaction-related API operations
  ```go
  type transactionClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Transaction], error)` - Lists transactions for a ledger
  - `Get(ctx context.Context, organizationID, ledgerID, id string) (*models.Transaction, error)` - Gets a transaction by ID
  - `Create(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error)` - Creates a new transaction
    ```go
    // Implementation pattern
    func (c *transactionClient) Create(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
        path := fmt.Sprintf("/organizations/%s/ledgers/%s/transactions", organizationID, ledgerID)
        
        resp, err := c.apiClient.httpClient.Post(ctx, path, input)
        if err != nil {
            return nil, err
        }
        defer resp.Body.Close()
        
        var result models.Transaction
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return nil, err
        }
        
        return &result, nil
    }
    ```
  
  - `Commit(ctx context.Context, organizationID, ledgerID, id string) (*models.Transaction, error)` - Commits a pending transaction
  - `Cancel(ctx context.Context, organizationID, ledgerID, id string) (*models.Transaction, error)` - Cancels a pending transaction

### Balance Client

- `balanceClient` - Handles balance-related API operations
  ```go
  type balanceClient struct {
      apiClient *apiClient
  }
  ```

  - `List(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Balance], error)` - Lists balances for a ledger
  - `Get(ctx context.Context, organizationID, ledgerID, id string) (*models.Balance, error)` - Gets a balance by ID
  - `Update(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateBalanceInput) (*models.Balance, error)` - Updates a balance

## III. Models Package (`midaz/models`)

The models package defines the data structures used throughout the SDK, representing API resources, request inputs, and responses.

### Resource Models

These structs represent the core resources in the Midaz API:

- `Organization` - Represents an organization in the system
  ```go
  type Organization struct {
      ID            string     `json:"id"`
      LegalName     string     `json:"legalName"`
      LegalDocument string     `json:"legalDocument,omitempty"`
      Status        Status     `json:"status"`
      Address       *Address   `json:"address,omitempty"`
      CreatedAt     time.Time  `json:"createdAt"`
      UpdatedAt     time.Time  `json:"updatedAt"`
      DeletedAt     *time.Time `json:"deletedAt,omitempty"`
      Metadata      map[string]any `json:"metadata,omitempty"`
      Tags          []string   `json:"tags,omitempty"`
  }
  ```

- `Ledger` - Represents a ledger in the system
- `Account` - Represents an account in the system
- `Asset` - Represents an asset in the system
- `Portfolio` - Represents a portfolio in the system
- `Segment` - Represents a segment in the system
- `Transaction` - Represents a transaction in the system
- `Operation` - Represents an operation within a transaction
- `Balance` - Represents an account balance

### Input Models

These structs represent the input data for API operations:

- `CreateOrganizationInput` - Input for creating an organization
  ```go
  type CreateOrganizationInput struct {
      LegalName     string      `json:"legalName"`
      LegalDocument string      `json:"legalDocument,omitempty"`
      Status        Status      `json:"status,omitempty"`
      Address       *Address    `json:"address,omitempty"`
      Metadata      map[string]any  `json:"metadata,omitempty"`
      Tags          []string    `json:"tags,omitempty"`
  }
  ```

- `UpdateOrganizationInput` - Input for updating an organization
- `CreateLedgerInput` - Input for creating a ledger
- `UpdateLedgerInput` - Input for updating a ledger
- `CreateAccountInput` - Input for creating an account
- `UpdateAccountInput` - Input for updating an account
- `CreateAssetInput` - Input for creating an asset
- `UpdateAssetInput` - Input for updating an asset
- `CreatePortfolioInput` - Input for creating a portfolio
- `UpdatePortfolioInput` - Input for updating a portfolio
- `CreateSegmentInput` - Input for creating a segment
- `UpdateSegmentInput` - Input for updating a segment
- `CreateTransactionInput` - Input for creating a transaction
- `CreateOperationInput` - Input for creating an operation
- `UpdateBalanceInput` - Input for updating a balance

### Response Models

These structs represent API responses:

- `ListResponse` - Generic paginated response for list operations
  ```go
  type ListResponse[T any] struct {
      Items      []T   `json:"items"`
      Page       int   `json:"page"`
      Limit      int   `json:"limit"`
      TotalItems int   `json:"totalItems"`
      TotalPages int   `json:"totalPages"`
  }
  ```

- `ListOptions` - Options for list operations
  ```go
  type ListOptions struct {
      Page    int               `json:"page,omitempty"`
      Limit   int               `json:"limit,omitempty"`
      Filters map[string]string `json:"filters,omitempty"`
      Sort    map[string]string `json:"sort,omitempty"`
  }
  ```

### Common Types and Constants

- `Status` - Enum for resource status
  ```go
  type Status string

  const (
      StatusActive   Status = "active"
      StatusInactive Status = "inactive"
      StatusPending  Status = "pending"
  )
  ```

- `Address` - Struct for address information
- Transaction status constants
- Operation type constants
- Asset type constants

## IV. Error Handling (`midaz/errors`)

The errors package provides standardized error types and utilities for handling errors in a consistent way.

### Error Types

- `MidazError` - Custom error type with additional context
  ```go
  type MidazError struct {
      Code      string
      Message   string
      Err       error
      Resource  string
      RequestID string
  }
  
  // Implements the error interface
  func (e *MidazError) Error() string {
      if e.Resource != "" {
          return fmt.Sprintf("%s: %s (resource: %s)", e.Code, e.Message, e.Resource)
      }
      return fmt.Sprintf("%s: %s", e.Code, e.Message)
  }
  
  // Implements the Unwrap interface for errors.Is and errors.As
  func (e *MidazError) Unwrap() error {
      return e.Err
  }
  ```

### Standard Error Codes

- `ErrNotFound` - Returned when a resource is not found
- `ErrValidation` - Returned when a request fails validation
- `ErrTimeout` - Returned when a request times out
- `ErrAuthentication` - Returned when authentication fails
- `ErrPermission` - Returned when the user does not have permission
- `ErrRateLimit` - Returned when the API rate limit is exceeded
- `ErrInternal` - Returned when an unexpected error occurs

### Transaction-Specific Errors

- `ErrAccountEligibility` - Returned when accounts are not eligible for a transaction
- `ErrAssetMismatch` - Returned when accounts have different asset types
- `ErrInsufficientBalance` - Returned when a transaction would result in a negative balance

### Error Handling Utilities

- `NewError()` - Creates a new MidazError with the given code and error
- `NewErrorf()` - Creates a new MidazError with the given code and formatted message
- `APIErrorToError()` - Converts an internal API error type to a public error

### Error Type Checking

- `IsNotFoundError()` - Checks if the error is a not found error
- `IsValidationError()` - Checks if the error is a validation error
- `IsAccountEligibilityError()` - Checks if the error is related to account eligibility
- `IsInsufficientBalanceError()` - Checks if the error is an insufficient balance error
- `IsAssetMismatchError()` - Checks if the error is related to asset mismatch
- `IsTimeoutError()` - Checks if the error is related to timeout
- `IsAuthenticationError()` - Checks if the error is related to authentication
- `IsPermissionError()` - Checks if the error is related to permissions
- `IsRateLimitError()` - Checks if the error is related to rate limiting
- `IsInternalError()` - Checks if the error is an internal error

### Transaction Error Utilities

- `FormatTransactionError()` - Produces a standardized error message for transaction errors
- `CategorizeTransactionError()` - Provides the error category as a string
- `GetTransactionErrorContext()` - Returns detailed context information for transaction errors
- `IsTransactionRetryable()` - Determines if a transaction error can be safely retried

## V. Implementation Patterns

This section describes common implementation patterns used throughout the SDK.

### Environment Variables

The SDK uses environment variables for configuration, allowing users to customize behavior without changing code:

```go
// In config/config.go
func configFromEnvironment() Config {
    c := Config{
        AuthToken:       os.Getenv("MIDAZ_AUTH_TOKEN"),
        UserAgent:       "Midaz-Go-SDK/" + Version, // Uses version constant
        Environment:     os.Getenv("MIDAZ_ENVIRONMENT"),
    }
    
    // Override defaults with environment variables if provided
    if onboardingURL := os.Getenv("MIDAZ_ONBOARDING_URL"); onboardingURL != "" {
        c.OnboardingURL = onboardingURL
    }
    
    if transactionURL := os.Getenv("MIDAZ_TRANSACTION_URL"); transactionURL != "" {
        c.TransactionURL = transactionURL
    }
    
    if userAgent := os.Getenv("MIDAZ_USER_AGENT"); userAgent != "" {
        c.UserAgent = userAgent
    }
    
    return c
}
```

### Service URL Management

The SDK supports multi-service architecture by mapping service names to URLs:

```go
// Map of service names to URLs
serviceURLs := map[string]string{
    "onboarding":  "https://onboarding.api.midaz.com",
    "transaction": "https://transaction.api.midaz.com",
}

// Getting the correct URL for a specific service
func (e *Entity) getServiceURL(service string) string {
    if url, ok := e.baseURLs[service]; ok {
        return url
    }
    // Fall back to default URL if service-specific URL not found
    return e.baseURL
}
```

### HTTP Client Configuration

The HTTP client includes detailed debugging and customizable user agent support:

```go
// Creating an HTTP client with debug logging and custom user agent
client := &HTTPClient{
    client:     optimizedClient,
    authToken:  authToken,
    userAgent:  getUserAgent(), // Gets from environment or uses default
    debug:      debug,
    // Additional fields omitted for brevity
}

// Debug logging for requests and responses
if c.debug {
    c.debugLog("Request URL: %s %s", method, requestURL)
    c.debugLog("Request headers: %v", req.Header)
    c.debugLog("Request body: %s", string(bodyBytes))
}

// After the request
if c.debug {
    c.debugLog("Response from: %s %s", method, requestURL)
    c.debugLog("Response status: %d", resp.StatusCode)
    c.debugLog("Response headers: %v", resp.Header)
    c.debugLog("Response body: %s", string(responseBody))
}
```

### Interface-Based Design

The SDK uses interfaces to define the public API, with private implementations:

```go
// Public interface
type SomeService interface {
    List(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.SomeResource], error)
    Get(ctx context.Context, id string) (*models.SomeResource, error)
    Create(ctx context.Context, input *models.CreateSomeResourceInput) (*models.SomeResource, error)
    Update(ctx context.Context, id string, input *models.UpdateSomeResourceInput) (*models.SomeResource, error)
    Delete(ctx context.Context, id string) error
}

// Private implementation
type someServiceImpl struct {
    client *client.Client
}

// Constructor that returns the interface
func NewSomeService(client *client.Client) SomeService {
    return &someServiceImpl{
        client: client,
    }
}

// Implementation methods
func (s *someServiceImpl) List(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.SomeResource], error) {
    // Implementation
}

// ... other methods
```

This pattern allows for:
- Clean separation between public API and implementation details
- Easier testing through interface mocking
- Future extensibility without breaking changes

### Error Wrapping

The SDK uses error wrapping to preserve context:

```go
func (c *someClient) Get(ctx context.Context, id string) (*models.SomeResource, error) {
    resp, err := c.httpClient.Get(ctx, fmt.Sprintf("/resources/%s", id), nil)
    if err != nil {
        return nil, fmt.Errorf("failed to get resource: %w", err)
    }
    
    // Process response
}
```

This pattern allows:
- Preserving the original error for inspection with `errors.Is` and `errors.As`
- Adding context to errors for better debugging
- Standardized error handling throughout the SDK

### Pagination Handling

The SDK uses a consistent pattern for handling paginated results:

```go
func (c *someClient) List(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.SomeResource], error) {
    path := "/resources"
    params := url.Values{}
    
    // Add pagination parameters
    if opts != nil {
        if opts.Page > 0 {
            params.Set("page", strconv.Itoa(opts.Page))
        }
        if opts.Limit > 0 {
            params.Set("limit", strconv.Itoa(opts.Limit))
        }
        
        // Add filters
        for key, value := range opts.Filters {
            params.Set(key, value)
        }
        
        // Add sorting
        for key, value := range opts.Sort {
            params.Set(fmt.Sprintf("sort[%s]", key), value)
        }
    }
    
    // Execute request
    resp, err := c.httpClient.Get(ctx, path, params)
    if err != nil {
        return nil, err
    }
    
    // Parse response
    var result models.ListResponse[models.SomeResource]
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

This pattern provides:
- Consistent pagination across all list operations
- Support for filtering and sorting
- Clear separation of concerns between pagination, filtering, and API calls