# Error Handling in the Midaz Go SDK

The Midaz Go SDK provides a comprehensive error handling system that helps you understand and respond to errors that occur during API interactions. This document explains the error handling architecture and best practices for working with errors.

## Table of Contents

- [Error Types](#error-types)
- [Error Details](#error-details)
- [Error Checking Functions](#error-checking-functions)
- [Error Handling Patterns](#error-handling-patterns)
- [Retry Mechanism](#retry-mechanism)
- [Best Practices](#best-practices)
- [Example Usage](#example-usage)

## Error Types

The SDK provides several specialized error types to represent different failure categories:

| Error Type | Description | Common Causes |
|------------|-------------|--------------|
| `ValidationError` | Input validation failed | Invalid field values, missing required fields |
| `AuthenticationError` | Authentication failed | Invalid or expired auth token |
| `AuthorizationError` | Unauthorized access | Insufficient permissions |
| `ResourceNotFoundError` | Resource not found | Invalid ID, deleted resource |
| `ConflictError` | Resource conflict | Duplicate resource, version conflict |
| `RateLimitError` | Rate limit exceeded | Too many requests |
| `ServiceError` | Backend service error | Internal server error, service unavailable |
| `NetworkError` | Network or transport error | Connection issues, timeouts |
| `ParseError` | Response parsing error | Invalid response format, schema mismatch |
| `SDKError` | SDK internal error | Configuration errors, programming errors |

These error types all implement the standard Go `error` interface and can be accessed from the `pkg/errors` package:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
```

## Error Details

Each error includes detailed information to help with troubleshooting:

| Field | Description | Example |
|-------|-------------|---------|
| `Type` | Error type identifier | `"validation_error"` |
| `Code` | Specific error code | `"invalid_input"` |
| `Message` | Human-readable error message | `"Validation failed: invalid account ID"` |
| `StatusCode` | HTTP status code (when applicable) | `400` |
| `Resource` | Resource type related to the error | `"account"` |
| `ResourceID` | ID of the resource (when applicable) | `"acc_123"` |
| `Details` | Additional error details | `{"field": "id", "reason": "must be UUID"}` |
| `RequestID` | Request ID for support (when available) | `"req_abc123"` |

You can access these details using the `ErrorDetails` interface:

```go
if details, ok := err.(errors.ErrorDetails); ok {
    fmt.Printf("Error type: %s, code: %s, message: %s\n", 
        details.ErrorType(), details.ErrorCode(), details.ErrorMessage())
}
```

## Error Checking Functions

The SDK provides helper functions to check for specific error types:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"

// Check for specific error types
if errors.IsValidationError(err) {
    // Handle validation error
}

if errors.IsResourceNotFoundError(err) {
    // Handle not found error
}

if errors.IsRateLimitError(err) {
    // Handle rate limit error
    retry := errors.GetRetryAfter(err)
    fmt.Printf("Rate limit exceeded. Retry after %v\n", retry)
}

// Check for specific HTTP status codes
if errors.IsStatusCode(err, 400) {
    // Handle 400 Bad Request
}
```

## Error Handling Patterns

### Basic Error Handling

```go
org, err := client.Entity.Organizations.CreateOrganization(ctx, input)
if err != nil {
    if errors.IsValidationError(err) {
        // Handle validation error
        fmt.Printf("Validation error: %s\n", err)
        return err
    }
    
    if errors.IsAuthenticationError(err) {
        // Handle authentication error
        fmt.Println("Authentication failed. Please check your credentials.")
        return err
    }
    
    // Handle other errors
    fmt.Printf("Error creating organization: %s\n", err)
    return err
}

// Use the organization
fmt.Printf("Organization created with ID: %s\n", org.ID)
```

### Getting Field-Level Validation Errors

For validation errors, you can access field-specific error details:

```go
org, err := client.Entity.Organizations.CreateOrganization(ctx, input)
if err != nil {
    if errors.IsValidationError(err) {
        if fieldErrs, ok := errors.GetFieldErrors(err); ok {
            for field, fieldErr := range fieldErrs {
                fmt.Printf("Field '%s' error: %s\n", field, fieldErr.Message)
                
                // Get suggestions if available
                if len(fieldErr.Suggestions) > 0 {
                    fmt.Printf("Suggestions: %v\n", fieldErr.Suggestions)
                }
            }
        }
        return err
    }
    
    // Handle other errors
    return err
}
```

### Handling Retryable Errors

```go
const maxRetries = 3

var org *models.Organization
var err error

for attempt := 0; attempt < maxRetries; attempt++ {
    org, err = client.Entity.Organizations.CreateOrganization(ctx, input)
    if err == nil {
        break
    }
    
    if errors.IsRetryableError(err) {
        delay := errors.GetRetryDelay(err, attempt)
        fmt.Printf("Retryable error occurred, retrying in %v...\n", delay)
        time.Sleep(delay)
        continue
    }
    
    // Non-retryable error, exit the loop
    break
}

if err != nil {
    // Handle the error
    return err
}

// Use the organization
fmt.Printf("Organization created with ID: %s\n", org.ID)
```

## Retry Mechanism

The SDK includes a built-in retry mechanism for handling transient errors. By default, the SDK will automatically retry:

1. Temporary network errors
2. Rate limit errors (with appropriate backoff)
3. Server errors (5xx status codes)
4. Specific API errors marked as retryable

You can configure the retry behavior using:

```go
// Through client options
client, err := client.New(
    client.WithRetries(3, 500*time.Millisecond, 5*time.Second),
    // Other options...
)

// Or through environment variables
// MIDAZ_MAX_RETRIES=3
// MIDAZ_RETRY_WAIT_MIN=500
// MIDAZ_RETRY_WAIT_MAX=5000
// MIDAZ_ENABLE_RETRIES=true
```

## Best Practices

1. **Always Check Errors**: Always check for errors and handle them appropriately.

2. **Use Type-Specific Handling**: Use the error checking functions to provide type-specific error handling.

3. **Provide Meaningful Error Messages**: When propagating errors, add context to help with troubleshooting.

4. **Log Error Details**: Log the full error details for debugging and monitoring.

5. **Handle Retryable Errors**: Use the retry mechanism for transient errors.

6. **Check Field Errors**: For validation errors, check field-specific errors to provide better user feedback.

7. **Be Careful with Parsing Errors**: When getting error details, check that the type assertion is successful.

## Example Usage

### Complete Error Handling Example

```go
func CreateAccount(ctx context.Context, client *client.Client, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
    // Validate input
    if err := input.Validate(); err != nil {
        return nil, fmt.Errorf("invalid input: %w", err)
    }
    
    // Create the account
    account, err := client.Entity.Accounts.CreateAccount(ctx, orgID, ledgerID, input)
    if err != nil {
        // Check specific error types
        switch {
        case errors.IsValidationError(err):
            // Handle validation error
            if fieldErrs, ok := errors.GetFieldErrors(err); ok {
                for field, fieldErr := range fieldErrs {
                    log.Printf("Field '%s' error: %s\n", field, fieldErr.Message)
                }
            }
            return nil, fmt.Errorf("validation error creating account: %w", err)
            
        case errors.IsAuthenticationError(err):
            // Handle authentication error
            log.Println("Authentication failed. Please check your credentials.")
            return nil, fmt.Errorf("authentication error creating account: %w", err)
            
        case errors.IsResourceNotFoundError(err):
            // Handle not found error (e.g., ledger not found)
            log.Printf("Resource not found: %v\n", err)
            return nil, fmt.Errorf("resource not found creating account: %w", err)
            
        case errors.IsRateLimitError(err):
            // Handle rate limit error
            retry := errors.GetRetryAfter(err)
            log.Printf("Rate limit exceeded. Retry after %v\n", retry)
            return nil, fmt.Errorf("rate limit exceeded creating account: %w", err)
            
        default:
            // Handle other errors
            log.Printf("Error creating account: %v\n", err)
            return nil, fmt.Errorf("error creating account: %w", err)
        }
    }
    
    log.Printf("Account created with ID: %s\n", account.ID)
    return account, nil
}
```

### Implementing Retry Logic

```go
func CreateAccountWithRetry(ctx context.Context, client *client.Client, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
    const maxRetries = 3
    var account *models.Account
    var err error
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        account, err = client.Entity.Accounts.CreateAccount(ctx, orgID, ledgerID, input)
        if err == nil {
            return account, nil
        }
        
        if !errors.IsRetryableError(err) || attempt == maxRetries-1 {
            break
        }
        
        delay := errors.GetRetryDelay(err, attempt)
        log.Printf("Retryable error occurred (attempt %d/%d), retrying in %v: %v\n", 
            attempt+1, maxRetries, delay, err)
        time.Sleep(delay)
    }
    
    if err != nil {
        return nil, fmt.Errorf("failed to create account after %d attempts: %w", maxRetries, err)
    }
    
    return account, nil
}
```

### Using the Enhanced Error Details

```go
func GetErrorDetails(err error) {
    if details, ok := err.(errors.ErrorDetails); ok {
        fmt.Printf("Error type: %s\n", details.ErrorType())
        fmt.Printf("Error code: %s\n", details.ErrorCode())
        fmt.Printf("Error message: %s\n", details.ErrorMessage())
        
        if details.StatusCode() > 0 {
            fmt.Printf("HTTP status: %d\n", details.StatusCode())
        }
        
        if details.Resource() != "" {
            fmt.Printf("Resource: %s\n", details.Resource())
            if details.ResourceID() != "" {
                fmt.Printf("Resource ID: %s\n", details.ResourceID())
            }
        }
        
        if details.RequestID() != "" {
            fmt.Printf("Request ID: %s\n", details.RequestID())
        }
        
        if errorDetails := details.Details(); errorDetails != nil {
            fmt.Printf("Additional details: %v\n", errorDetails)
        }
    } else {
        fmt.Printf("Generic error: %v\n", err)
    }
}
```