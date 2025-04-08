# Midaz Go SDK Architecture

This document outlines the architecture of the Midaz Go SDK, focusing on abstraction layers and type conversion patterns.

## Abstraction Layers

The SDK is designed with a clean, layered architecture to provide different levels of abstraction for various use cases:

### 1. Models Layer

**Purpose**: Data structures representing API resources with validation logic.

**Responsibilities**:
- Define data structures for all Midaz resources (Account, Asset, Transaction, etc.)
- Provide methods for validation and conversion between formats
- Implement constructors and helper methods for creating and modifying resources
- Handle type conversions between SDK models and backend models (`mmodel`)

**Interface**:
- Model structs like `models.Account`, `models.Transaction`, etc.
- Input structs like `models.CreateAccountInput`, `models.UpdateTransactionInput`, etc.
- List response structs like `models.ListResponse[T]`
- Common types like `models.Status`, `models.Address`, etc.

### 2. Entities Layer

**Purpose**: Low-level API clients for direct service interaction.

**Responsibilities**:
- Provide direct access to the Midaz API endpoints
- Handle HTTP communication, authentication, and error mapping
- Convert between SDK and backend models
- Implement input validation and error handling

**Interface**:
- Service interfaces like `entities.AccountsService`, `entities.TransactionsService`, etc.
- Entity implementations that communicate with the API

### 3. Client Layer

**Purpose**: Top-level entry point for SDK users.

**Responsibilities**:
- Provide a single entry point for all SDK functionality
- Initialize and manage entity instances
- Handle configuration, authentication, and shared resources
- Provide context management and observability features

**Interface**:
- `Client` struct with access to all service interfaces
- Configuration options through functional options pattern
- Context management through `WithContext` method

## Type Conversion Patterns

The SDK includes several patterns and utilities for efficient type conversion:

### 1. Generic Conversion Utilities

The `pkg/conversion` package provides generic utilities for common conversion patterns:

- `MapStruct[T]`: Convert struct to map using reflection
- `UnmapStruct[T]`: Convert map to struct using reflection
- `MapSlice[T, R]`: Map a slice of type T to a slice of type R
- `FilterSlice[T]`: Filter a slice based on a predicate
- `ReduceSlice[T, R]`: Reduce a slice to a single value
- `PtrValue[T]`: Safely get a value from a pointer with default
- `ToPtr[T]`: Create a pointer to a value

### 2. Model Conversion

Models implement conversion methods between SDK models and backend models:

- `FromMmodelX` methods: Convert backend model to SDK model
- `ToMmodelX` methods: Convert SDK model to backend model

These methods use a combination of:
1. Generic `ModelConverter` for automatic field mapping
2. Manual conversion for special cases
3. Fallback to direct field mapping if automatic conversion fails

### 3. Specialized Converters

The SDK includes specialized converters for specific use cases:

- Date/time converters (`ConvertToISODate`, `ConvertToISODateTime`)
- Amount formatting (`FormatAmount`)
- Transaction summary generation (`ConvertTransactionToSummary`)
- Metadata/tag conversion (`ConvertMetadataToTags`, `ConvertTagsToMetadata`)

## Best Practices

### For SDK Developers

1. **Use Generic Utilities**: Use the generic conversion utilities in the `pkg/conversion` package to reduce boilerplate code.

2. **Automatic Model Conversion**: Use `ModelConverter` for simple model-to-model conversions, with manual handling for special cases.

3. **Consistent Patterns**: Follow consistent patterns for API method naming, parameter order, and error handling.

4. **Validation**: Implement validation at the appropriate layer (typically models) to catch errors early.

5. **Minimize Type Conversions**: Avoid unnecessary conversions between types, especially in hot paths.

6. **Testing**: Test conversion methods thoroughly, including edge cases and error conditions.

### For SDK Users

1. **Choose the Right Abstraction**: Use the appropriate abstraction layer for your use case:
   - Entities for direct API access and complex queries
   - Client for a unified entry point and shared configuration

2. **Handle Errors Appropriately**: Use the provided error types and checking functions to handle errors properly.

3. **Use Pagination**: Use pagination for listing resources to avoid large response payloads.

4. **Context Management**: Use contexts for cancellation, timeouts, and tracing.

5. **Observability**: Enable observability features for monitoring and debugging.

## Implementation Notes

### Model Conversion Implementation

Model conversion methods now follow this pattern:

```go
func FromMmodelX(backendModel mmodel.X) X {
    // Try automatic conversion first
    var result X
    if err := conversion.ModelConverter(backendModel, &result); err != nil {
        // Fallback to manual conversion
        return X{
            // Direct field mapping
        }
    }
    
    // Handle special cases not covered by automatic conversion
    
    return result
}
```

This pattern provides several benefits:
1. Reduces boilerplate code for field-by-field mapping
2. Handles type conversions automatically
3. Provides a fallback mechanism for complex cases
4. Is easier to maintain as models evolve

### Transaction Mapping

Transactions use a more complex mapping pattern due to their structure:

1. **Map-Based Conversion**: Transactions are converted to maps first, then to the target struct.
2. **DSL Conversion**: Transaction DSL inputs are converted through specialized methods.
3. **Operation Mapping**: Operations within transactions require special handling for debits and credits.

## Future Improvements

1. **Enhanced Generic Converters**: Extend the generic converters to handle more complex cases like nested structs and maps.

2. **Code Generation**: Consider using code generation for repetitive conversion code.

3. **Performance Optimizations**: Add caching for frequently used conversions.

4. **Enhanced Validation**: Improve validation error messages with field-level details.

5. **Structured Logging**: Add structured logging for conversion operations to aid debugging.