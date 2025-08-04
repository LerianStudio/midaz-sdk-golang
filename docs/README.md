# Midaz Go SDK Documentation

This documentation provides comprehensive information about the Midaz Go SDK, including API references, usage guides, and examples.

## Key Documentation

- [Development GODOC Documentation](./docs/godoc/github.com/LerianStudio/midaz-sdk-golang/v2/index.md) - Static documentation for development

- [Environment Variables](./environment.md) - Complete guide to environment variable configuration
- [Error Handling](./errors.md) - Comprehensive guide to error handling in the SDK
- [Architecture](./architecture.md) - Overview of the SDK architecture
- [Examples](./examples.md) - Detailed explanation of the example applications
- [Pagination](./pagination.md) - Guide to using the pagination features

## API Reference Documentation

- [External API Mapping](./mapping/external_apis.md) - Mapping between SDK interfaces and external APIs
- [Internal API Mapping](./mapping/internal_apis.md) - Mapping between SDK interfaces and internal APIs

## Package Structure

The Midaz Go SDK is organized into the following main packages:

- **Root Package**: Main entry point for the SDK, provides the Client type and configuration options
- **Entities**: High-level entity-based API for working with Midaz resources
- **Models**: Data models representing Midaz resources
- **Pkg**: Utility packages for various SDK functionalities

## API Documentation

The SDK provides detailed API documentation for all packages. You can browse this documentation in two ways:

1. **Interactive Documentation** (requires godoc):
   ```bash
   make godoc
   ```
   Then visit http://localhost:6060/pkg/github.com/LerianStudio/midaz-sdk-golang/v2/

2. **Static Documentation**:
   The static documentation is generated in the `docs/godoc` directory as Markdown files. You can browse this documentation directly.
   ```bash
   make godoc-static
   ```
   
   Generated API documentation files:
   - [Main Package](./godoc/index.txt)
   - [Entities Package](./godoc/entities/index.txt)
   - [Models Package](./godoc/models/index.txt)
   - [Config Package](./godoc/pkg/config/index.txt)
   - [Concurrent Package](./godoc/pkg/concurrent/index.txt)
   - [Access Manager Package](./godoc/pkg/access-manager/index.txt)
   - [Observability Package](./godoc/pkg/observability/index.txt)
   - [Pagination Package](./godoc/pkg/pagination/index.txt)
   - [Validation Package](./godoc/pkg/validation/index.txt)
   - [Validation Core Package](./godoc/pkg/validation/core/index.txt)
   - [Errors Package](./godoc/pkg/errors/index.txt)
   - [Format Package](./godoc/pkg/format/index.txt)
   - [Retry Package](./godoc/pkg/retry/index.txt)
   - [Performance Package](./godoc/pkg/performance/index.txt)

## Key Packages

### Root Package

The root package provides the main entry point for the SDK. The key types are:

- **Client**: The main client for interacting with the Midaz API
- **Option**: Functional options for configuring the client

### Entities Package

The entities package provides high-level entity-based APIs for working with Midaz resources:

- **Organizations**: Methods for managing organizations
- **Ledgers**: Methods for managing ledgers
- **Accounts**: Methods for managing accounts
- **Transactions**: Methods for creating and managing transactions
- **Assets**: Methods for managing assets
- **Portfolios**: Methods for managing portfolios
- **Segments**: Methods for managing segments

### Models Package

The models package provides data models representing Midaz resources:

- **Organization**: Represents an organization in the Midaz system
- **Ledger**: Represents a ledger in the Midaz system
- **Account**: Represents an account in the Midaz system
- **Transaction**: Represents a transaction in the Midaz system
- **Asset**: Represents an asset in the Midaz system
- **Portfolio**: Represents a portfolio in the Midaz system
- **Segment**: Represents a segment in the Midaz system

### Utility Packages

The SDK includes several utility packages in the `pkg` directory:

- **config**: Configuration utilities for the SDK
- **concurrent**: Utilities for concurrent operations
- **access-manager**: Plugin-based authentication for integrating with external identity providers
- **observability**: Observability tools for tracing, metrics, and logging
- **pagination**: Utilities for paginated API requests
- **validation**: Validation utilities for SDK models
- **errors**: Error handling utilities
- **format**: Formatting utilities
- **retry**: Retry mechanisms for API requests
- **performance**: Performance optimization utilities

## Examples

For detailed examples of how to use the SDK, refer to the `examples` directory in the SDK repository.