# Validation System in Midaz SDK

This document explains the validation architecture in the Midaz Go SDK, focusing on how external dependencies are managed through interfaces.

## Overview

The validation system in Midaz SDK is designed with the following goals:

1. **Maintain consistency** with backend APIs
2. **Decouple from external dependencies** for easier testing and maintenance
3. **Support custom validation logic** for specific use cases
4. **Allow dependency injection** for better testability

## Implementation Details

The validation system is implemented using a dependency-inversion approach where:

1. The `validation` package defines interfaces for validation operations
2. The `adapters` subpackage provides implementations using lib-commons
3. The `testing` subpackage provides mock implementations for testing
4. The Client struct and Entity struct both have a validatorProvider field
5. Validators are propagated from Client -> Entity -> Models

## Architecture

### Key Components

#### 1. Validator Interfaces

The validation system uses interfaces to define validation contracts:

```go
// TypeValidator defines operations for validating asset types
type TypeValidator interface {
    ValidateType(assetType string) error
}

// Other validator interfaces...

// ValidatorProvider combines all validator interfaces
type ValidatorProvider interface {
    TypeValidator
    AccountTypeValidator
    CurrencyValidator
    CountryValidator
}
```

#### 2. Default Implementation

The SDK provides a default implementation using lib-commons:

```go
// LibCommonsValidator implements ValidatorProvider using lib-commons
type LibCommonsValidator struct{}

func (v *LibCommonsValidator) ValidateType(assetType string) error {
    return commons.ValidateType(strings.ToLower(assetType))
}

// Other validation methods...
```

#### 3. Validator Configuration

Validators can be configured at the client level:

```go
// Create a client with a custom validator
client, err := client.New(
    validation.WithValidatorProvider(myCustomValidator),
    // Other options...
)
```

## Usage Examples

### Default Validation

```go
// Create a client with default validation (using lib-commons)
client, err := client.New(
    client.WithHTTPClient(httpClient),
    client.WithEnvironment("development"),
)

// Validate using the default validator
tx := &models.TransactionDSLInput{...}
if err := tx.Validate(); err != nil {
    // Handle validation error
}
```

### Client Configuration with Validator

```go
// Create custom validator
customValidator := &MyCustomValidator{}

// Create client with the custom validator
client, err := client.New(
    client.WithHTTPClient(httpClient),
    client.UseEntity(), // If you want to use entities
    validation.WithValidatorProvider(customValidator),
)

// The validator is automatically used by the client and entities
```

### Custom Validation

```go
// Create a custom validator
type CustomValidator struct{}

func (v *CustomValidator) ValidateType(assetType string) error {
    // Custom validation logic
    return nil
}

// Other validation methods...

// Create a client with the custom validator
client, err := client.New(
    client.WithHTTPClient(httpClient),
    validation.WithValidatorProvider(&CustomValidator{}),
)

// Validate using the custom validator
tx := &models.TransactionDSLInput{...}
if err := tx.ValidateWithProvider(client.GetValidatorProvider()); err != nil {
    // Handle validation error
}
```

## Testing

The interface-based approach makes testing easier:

```go
// Create a mock validator for testing
mockValidator := &testing.MockValidatorProvider{}
mockValidator.ShouldReturnError = true
mockValidator.ErrorMessage = "mock validation error"

// Create a client with the mock validator
client, err := client.New(
    validation.WithValidatorProvider(mockValidator),
)

// Test validation with the mock
tx := &models.TransactionDSLInput{...}
err := tx.ValidateWithProvider(client.GetValidatorProvider())
// Assert err contains "mock validation error"
```

## Integration with Entity API

When using the Entity API, validators are automatically propagated:

```go
// Create a client with a custom validator and enable Entity API
client, err := client.New(
    validation.WithValidatorProvider(customValidator),
    client.UseEntity(),
)

// The Entity API inherits the validator from the client
entityValidator := client.Entity.GetValidatorProvider()

// Use the Entity's validator directly if needed
if err := entityValidator.ValidateType("USD"); err != nil {
    // Handle validation error
}
```

## Benefits of This Approach

1. **Source of Truth**: By default, uses lib-commons for consistency with backend
2. **Decoupling**: No direct dependency on external libraries in the validation interface
3. **Extensibility**: Custom validators can implement domain-specific rules
4. **Testability**: Easy to mock validation for testing
5. **Backward Compatibility**: Maintains API compatibility for existing code
6. **Consistency**: The same validator is used throughout the SDK (client, entities, models)
7. **Configurability**: Validators can be configured at client creation time