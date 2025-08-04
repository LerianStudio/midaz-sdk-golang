![banner](image/midaz-banner.png)

<div align="center">

[![Latest Release](https://img.shields.io/github/v/release/LerianStudio/midaz-sdk-golang?include_prereleases)](https://github.com/LerianStudio/midaz-sdk-golang/releases)
[![Go Report](https://goreportcard.com/badge/github.com/lerianstudio/midaz-sdk-golang)](https://goreportcard.com/report/github.com/lerianstudio/midaz-sdk-golang)
[![Discord](https://img.shields.io/badge/Discord-Lerian%20Studio-%237289da.svg?logo=discord)](https://discord.gg/DnhqKwkGv3)
[![Go Version](https://img.shields.io/github/go-mod/go-version/LerianStudio/midaz-sdk-golang)](https://golang.org/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE.md)

</div>

# Midaz Go SDK

A comprehensive Go client for the Midaz financial ledger API. This SDK provides a powerful and flexible way to interact with the Midaz platform, enabling developers to build robust financial applications with ease.

## Features

- **Comprehensive API Coverage**: Complete support for all Midaz API endpoints, including organizations, ledgers, accounts, transactions, portfolios, segments, and assets.
- **Functional Options Pattern**: Flexible configuration with type-safe, chainable options.
- **Plugin-based Authentication**: Secure authentication through the Access Manager for seamless integration with identity providers.
- **Robust Error Handling**: Detailed error information with field-level validation errors and helpful suggestions.
- **Concurrency Support**: Built-in utilities for parallel processing, batching, and rate limiting.
- **Observability**: Integrated tracing, metrics, and logging capabilities.
- **Pagination**: Generic pagination utilities that support both offset and cursor-based pagination.
- **Retry Mechanism**: Configurable retry mechanism with exponential backoff for resilient API interactions.
- **Environment Support**: Seamless swclient.**WithAccessManager**(**"**your-auth-token**"**),itching between local, development, and production environments.
- **Idiomatic Go Design**: Follows Go best practices for a natural fit in your Go applications.

## Documentation

For comprehensive documentation including API references, usage guides, and examples, see the [SDK Documentation](docs/README.md).

## Installation

```bash
go get github.com/LerianStudio/midaz-sdk-golang
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

func main() {
	// Configure plugin access manager
	AccessManager := auth.AccessManager{
		Enabled:      true,
		Address:      "https://your-auth-service.com",
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
	}

	// Create a configuration with plugin access manager
	cfg, err := config.NewConfig(
		config.WithAccessManager(AccessManager),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	// Create a client
	c, err := client.New(
		client.WithConfig(cfg),
		client.WithEnvironment(config.EnvironmentProduction),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Create an organization
	ctx := context.Background()
	org, err := c.Entity.Organizations.CreateOrganization(
		ctx,
		&models.CreateOrganizationInput{
			LegalName:       "Example Corporation",
			LegalDocument:   "123456789",
			DoingBusinessAs: "Example Inc.",
			Address: models.Address{
				Line1:   "123 Main St",
				City:    "New York",
				State:   "NY",
				ZipCode: "10001",
				Country: "US",
			},
		},
	)
	if err != nil {
		log.Fatalf("Failed to create organization: %v", err)
	}

	fmt.Printf("Organization created: %s\n", org.ID)
}
```

## Client Configuration

The SDK uses the functional options pattern for flexible configuration:

```go
// Basic configuration with plugin auth
pluginAuth := auth.PluginAuth{
	Enabled:      true,
	Address:      "https://your-auth-service.com",
	ClientID:     "your-client-id",
	ClientSecret: "your-client-secret",
}

cfg, err := config.NewConfig(
	config.WithPluginAuth(pluginAuth),
)
if err != nil {
	log.Fatalf("Failed to create config: %v", err)
}

client, err := client.New(
	client.WithConfig(cfg),
	client.UseAllAPIs(),
)

// Environment-specific configuration
pluginAuth := auth.PluginAuth{
	Enabled:      true,
	Address:      "https://your-auth-service.com",
	ClientID:     "your-client-id",
	ClientSecret: "your-client-secret",
}

cfg, err := config.NewConfig(
	config.WithPluginAuth(pluginAuth),
)
if err != nil {
	log.Fatalf("Failed to create config: %v", err)
}

client, err := client.New(
	client.WithConfig(cfg),
	client.WithEnvironment(config.EnvironmentProduction),
	client.UseAllAPIs(),
)

// Advanced configuration
pluginAuth := auth.PluginAuth{
	Enabled:      true,
	Address:      "https://your-auth-service.com",
	ClientID:     "your-client-id",
	ClientSecret: "your-client-secret",
}

cfg, err := config.NewConfig(
	config.WithPluginAuth(pluginAuth),
)
if err != nil {
	log.Fatalf("Failed to create config: %v", err)
}

client, err := client.New(
	client.WithConfig(cfg),
	client.WithTimeout(30 * time.Second),
	client.WithRetries(3, 100*time.Millisecond, 1*time.Second),
	client.WithObservability(true, true, true), // Enable tracing, metrics, and logging
	client.UseAllAPIs(),
)

// Plugin-based authentication configuration
AccessManager := auth.AccessManager{
	Enabled:      true,
	Address:      "https://your-auth-service.com",
	ClientID:     "your-client-id",
	ClientSecret: "your-client-secret",
}

cfg, err := config.NewConfig(
	config.WithPluginAuth(pluginAuth),
)
if err != nil {
	log.Fatalf("Failed to create config: %v", err)
}

client, err := client.New(
	client.WithConfig(cfg),
	client.UseAllAPIs(),
)
```

## SDK Architecture

The Midaz Go SDK is organized into three main components:

- **Client**: The top-level entry point that provides access to all API services.
- **Entities**: Service interfaces for interacting with Midaz resources.
- **Models**: Data structures representing Midaz resources.
- **Utility Packages**: Helper packages for configuration, concurrency, observability, access management, etc.

### Models

The `models` package defines the data structures used by the SDK:

- **Account**: Represents an account for tracking assets and balances.
- **Asset**: Represents a type of value that can be tracked and transferred.
- **Balance**: Represents the current state of an account's holdings.
- **Ledger**: Represents a collection of accounts and transactions.
- **Organization**: Represents a business entity that owns ledgers and accounts.
- **Portfolio**: Represents a collection of accounts for grouping and management.
- **Segment**: Represents a categorization unit for more granular organization.
- **Transaction**: Represents a financial event with operations (debits and credits).
- **Operation**: Represents an individual accounting entry within a transaction.

## Working with Entities

The SDK provides high-level access to all Midaz entities through the `entities` package. This package implements service interfaces for interacting with Midaz resources and operations, providing a clean, entity-based API:

- **Entity**: A centralized access point to all entity types, acting as a factory for the service interfaces.
- **AccountsService**: Methods for managing accounts and their balances.
- **AssetsService**: Methods for managing asset definitions.
- **BalancesService**: Methods for retrieving and managing account balances.
- **LedgersService**: Methods for creating and managing ledgers within organizations.
- **OperationsService**: Methods for working with transaction operations.
- **OrganizationsService**: Methods for creating and managing organizations.
- **PortfoliosService**: Methods for managing portfolios for account grouping.
- **SegmentsService**: Methods for managing segments for account categorization.
- **TransactionsService**: Methods for creating and managing financial transactions.

### Organizations

```go
// Create an organization
org, err := client.Entity.Organizations.CreateOrganization(ctx, &models.CreateOrganizationInput{
	LegalName:       "Example Organization",
	LegalDocument:   "123456789",
	DoingBusinessAs: "Example",
})

// Get an organization
org, err := client.Entity.Organizations.GetOrganization(ctx, "org-id")

// List organizations
orgs, err := client.Entity.Organizations.ListOrganizations(ctx, nil)
```

### Ledgers

```go
// Create a ledger
ledger, err := client.Entity.Ledgers.CreateLedger(ctx, "org-id", &models.CreateLedgerInput{
	Name: "Main Ledger",
	Metadata: map[string]any{
		"description": "Primary ledger for tracking all accounts",
	},
})

// List ledgers
ledgers, err := client.Entity.Ledgers.ListLedgers(ctx, "org-id", nil)
```

### Accounts

```go
// Create an account
account, err := client.Entity.Accounts.CreateAccount(ctx, "org-id", "ledger-id", &models.CreateAccountInput{
	Name:      "Customer Account",
	AssetCode: "USD",
	Type:      "customer",
})

// Get account balance
balance, err := client.Entity.Accounts.GetBalance(ctx, "org-id", "ledger-id", "account-id")
```

## Access Manager

The Access Manager provides a plugin-based authentication mechanism that allows you to integrate with external identity providers. This feature eliminates the need to hardcode authentication tokens in your application, enhancing security and flexibility.

### Configuration

To use the Access Manager, you need to configure it with the address of your authentication service and your client credentials:

```go
// Import the access manager package
import (
    auth "github.com/LerianStudio/midaz-sdk-golang/pkg/access-manager"
    "github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

// Configure plugin auth
AccessManager := auth.AccessManager{
    Enabled:      true,
    Address:      "https://your-auth-service.com",
    ClientID:     "your-client-id",
    ClientSecret: "your-client-secret",
}

// Create a configuration with plugin auth
cfg, err := config.NewConfig(
    config.WithAccessManager(AccessManager),
)
if err != nil {
    log.Fatalf("Failed to create config: %v", err)
}

// Create a client with the configuration
client, err := client.New(
    client.WithConfig(cfg),
    client.UseAllAPIs(),
)
```

### Environment Variables

You can also configure the Access Manager using environment variables:

```
PLUGIN_AUTH_ENABLED=true
PLUGIN_AUTH_ADDRESS=https://your-auth-service.com
MIDAZ_CLIENT_ID=your-client-id
MIDAZ_CLIENT_SECRET=your-client-secret
```

Then load them in your application:

```go
AccessManagerEnabled := os.Getenv("PLUGIN_AUTH_ENABLED") == "true"
AccessManagerAddress := os.Getenv("PLUGIN_AUTH_ADDRESS")
clientID := os.Getenv("MIDAZ_CLIENT_ID")
clientSecret := os.Getenv("MIDAZ_CLIENT_SECRET")

AccessManager := auth.AccessManager{
    Enabled:      AccessManagerEnabled,
    Address:      AccessManagerAddress,
    ClientID:     clientID,
    ClientSecret: clientSecret,
}
```

### How It Works

When plugin-based authentication is enabled, the SDK will:

1. Make a request to your authentication service using the provided client credentials
2. Retrieve an authentication token
3. Use this token for all subsequent API calls to the Midaz platform
4. Handle token refresh automatically when needed

This approach provides several benefits:

- **Security**: No hardcoded tokens in your application code
- **Flexibility**: Easily switch between different authentication providers
- **Centralized Management**: Manage all your authentication settings in one place
- **Automatic Token Refresh**: Tokens are automatically refreshed when they expire

### Transactions

```go
// Create a transaction using DSL
tx, err := client.Entity.Transactions.CreateTransactionWithDSL(ctx, "org-id", "ledger-id", &models.TransactionDSLInput{
	Description: "Payment from customer to merchant",
	Send: &models.DSLSend{
		Asset: "USD",
		Value: 10000, // $100.00
		Scale: 2,
		Source: &models.DSLSource{
			From: []models.DSLFromTo{
				{
					Account: "customer-account-id",
					Amount: &models.DSLAmount{
						Asset: "USD",
						Value: 10000,
						Scale: 2,
					},
				},
			},
		},
		Distribute: &models.DSLDistribute{
			To: []models.DSLFromTo{
				{
					Account: "merchant-account-id",
					Amount: &models.DSLAmount{
						Asset: "USD",
						Value: 10000,
						Scale: 2,
					},
				},
			},
		},
	},
})
```

## Utility Packages

The SDK includes several utility packages in the `pkg` directory that provide powerful functionality for working with the Midaz API:

- **config**: Configuration management for the SDK, including environment-based configuration and service URL mapping.
- **concurrent**: Utilities for concurrent operations, including worker pools, batching, and rate limiting.
- **observability**: Tracing, metrics, and logging capabilities for monitoring and debugging SDK operations.
- **pagination**: Generic pagination utilities for working with paginated API responses.
- **validation**: Validation utilities for ensuring data integrity and providing helpful error messages.
- **errors**: Structured error handling with field-level validation errors and error classification.
- **format**: Formatting utilities for dates, times, and other data types.
- **retry**: Configurable retry mechanism with exponential backoff for resilient API interactions.
- **performance**: Performance optimization utilities for batch operations and other high-performance scenarios.

## Advanced Features

### Pagination

The SDK provides powerful pagination utilities:

```go
// Create a paginator for accounts
paginator := client.Entity.Accounts.GetAccountPaginator(ctx, "org-id", "ledger-id", &models.ListOptions{
	Limit: 10,
})

// Iterate through all pages
for paginator.HasNext() {
	accounts, err := paginator.Next()
	if err != nil {
		// Handle error
	}

	for _, account := range accounts.Items {
		// Process each account
	}
}
```

### Concurrency Utilities

Process items in parallel with concurrency utilities:

```go
// Process accounts in parallel
results := concurrent.WorkerPool(
	ctx,
	accountIDs,
	func(ctx context.Context, accountID string) (*models.Account, error) {
		// Fetch account details
		return client.Entity.Accounts.GetAccount(ctx, "org-id", "ledger-id", accountID)
	},
	concurrent.WithWorkers(5), // Use 5 workers
)

// Process items in batches
batchResults := concurrent.Batch(
	ctx,
	transactionIDs,
	10, // Process 10 items per batch
	func(ctx context.Context, batch []string) ([]string, error) {
		// Process the batch and return results
		return processedIDs, nil
	},
	concurrent.WithWorkers(3), // Process 3 batches concurrently
)
```

### Observability

Enable detailed observability for monitoring and debugging:

```go
// Create a client with observability enabled
client, err := client.New(
	client.WithObservability(true, true, true), // Enable tracing, metrics, and logging
	client.UseAllAPIs(),
)

// Trace an operation
err = client.Trace("create-organization", func(ctx context.Context) error {
	_, err := client.Entity.Organizations.CreateOrganization(ctx, input)
	return err
})
```

## Environment Variables

The SDK can be configured using environment variables:

- `MIDAZ_AUTH_TOKEN`: Authentication token
- `MIDAZ_ENVIRONMENT`: Environment (local, development, production)
- `MIDAZ_ONBOARDING_URL`: Override for the onboarding service URL
- `MIDAZ_TRANSACTION_URL`: Override for the transaction service URL
- `MIDAZ_DEBUG`: Enable debug mode (true/false)
- `MIDAZ_MAX_RETRIES`: Maximum number of retry attempts

## Documentation

Detailed documentation is available:

- [API Reference](docs/README.md): Complete API documentation
- [Examples](docs/examples.md): Usage examples for common operations
- [Package Overview](docs/godoc/index.txt): Go package documentation

To generate documentation:

```bash
# Start an interactive documentation server
make godoc

# Generate static documentation
make docs
```

## Examples

For more detailed examples, see the [examples directory](examples/):

- [Configuration Examples](examples/configuration-examples/main.go): Various ways to configure the client
- [Context Example](examples/context-example/main.go): Using context for timeouts and cancellation
- [Concurrency Example](examples/concurrency-example/main.go): Parallel processing and batching
- [Retry Example](examples/retry-example/main.go): Custom retry configurations
- [Observability Example](examples/observability-example/main.go): Tracing, metrics, and logging
- [Complete Workflow](examples/workflow-with-entities/main.go): End-to-end workflow example

## Testing

Run the test suite:

```bash
make test
```

Generate test coverage report:

```bash
make coverage
```

## Best Practices

### Error Handling

The SDK provides rich error information. Always check errors and use the error helpers to extract details:

```go
// Create an account
account, err := client.Entity.Accounts.CreateAccount(ctx, "org-id", "ledger-id", input)
if err != nil {
    // Check error type
    switch {
    case errors.IsValidationError(err):
        // Handle validation error
        fmt.Println("Validation error:", err)
    
        // Get field-level errors
        if fieldErrs := errors.GetFieldErrors(err); len(fieldErrs) > 0 {
            for _, fieldErr := range fieldErrs {
                fmt.Printf("Field %s: %s\n", fieldErr.Field, fieldErr.Message)
            }
        }
    case errors.IsNotFoundError(err):
        // Handle not found error
        fmt.Println("Resource not found:", err)
    case errors.IsAuthenticationError(err):
        // Handle authentication error
        fmt.Println("Authentication error:", err)
    case errors.IsNetworkError(err):
        // Handle network error
        fmt.Println("Network error:", err)
    default:
        // Handle other errors
        fmt.Println("Error:", err)
    }
    return
}
```

### Context Usage

Always use contexts for cancellation and timeouts:

```go
// Create a context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Use the context for API calls
account, err := client.Entity.Accounts.GetAccount(ctx, "org-id", "ledger-id", "account-id")
```

### Resource Management

Always clean up resources when you're done:

```go
// Create a client
client, err := client.New(/* options */)
if err != nil {
    log.Fatal(err)
}
defer client.Shutdown(context.Background())

// Use the client...
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Apache License, Version 2.0 - see the [LICENSE.md](LICENSE.md) file for details.

Copyright 2025 Lerian Studio
