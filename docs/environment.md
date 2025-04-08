# Environment Variables in the Midaz Go SDK

The Midaz Go SDK provides comprehensive environment variable support for configuration, allowing you to customize the SDK's behavior without changing code. This document explains all available environment variables and their usage.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Environment Configuration](#environment-configuration)
- [API URLs](#api-urls)
- [HTTP Configuration](#http-configuration)
- [Retry Configuration](#retry-configuration)
- [Feature Flags](#feature-flags)
- [Observability Configuration](#observability-configuration)
- [Testing Configuration](#testing-configuration)
- [Example Configuration](#example-configuration)

## Overview

Environment variables provide a flexible way to configure the SDK for different environments and use cases. You can set these variables in your operating system environment or in a `.env` file at the root of your project.

To use a `.env` file, either:

1. Load it manually using a package like `godotenv`:
   ```go
   import "github.com/joho/godotenv"

   func init() {
       godotenv.Load() // Loads .env file from the current directory
   }
   ```

2. Or rely on the SDK's examples and tools that automatically look for a `.env` file.

## Authentication

| Variable | Purpose | Default | Required |
|----------|---------|---------|----------|
| `MIDAZ_AUTH_TOKEN` | Authentication token for the Midaz API | None | Yes (except for testing) |

Example:
```
MIDAZ_AUTH_TOKEN=midaz-auth-token-123456
```

## Environment Configuration

| Variable | Purpose | Default | Options |
|----------|---------|---------|---------|
| `MIDAZ_ENVIRONMENT` | Sets which Midaz environment to connect to | `local` | `local`, `development`, `production` |

Example:
```
MIDAZ_ENVIRONMENT=development
```

## SDK Configuration

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_USER_AGENT` | User agent string for API requests | `midaz-go-sdk/1.0.0` | Used for request identification |

Example:
```
MIDAZ_USER_AGENT=MyApp/1.2.3 (Midaz-Go-SDK/1.0.0)
```

## API URLs

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_BASE_URL` | Base URL for all services | Depends on environment | Used if specific URLs not provided |
| `MIDAZ_ONBOARDING_URL` | URL for the Onboarding API | `http://localhost:3000/v1` (local) | Overrides base URL |
| `MIDAZ_TRANSACTION_URL` | URL for the Transaction API | `http://localhost:3001/v1` (local) | Overrides base URL |

The URLs take precedence in this order:
1. Specific URL (`MIDAZ_ONBOARDING_URL`, `MIDAZ_TRANSACTION_URL`)
2. Base URL with service path (`MIDAZ_BASE_URL/service`)
3. Environment-based default URLs

Example for local development:
```
MIDAZ_BASE_URL=http://localhost
MIDAZ_ONBOARDING_URL=http://localhost:3000/v1
MIDAZ_TRANSACTION_URL=http://localhost:3001/v1
```

Example for custom deployment:
```
MIDAZ_BASE_URL=https://midaz.example.com
```

## HTTP Configuration

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_TIMEOUT` | Timeout in seconds for HTTP requests | `60` | Controls request timeouts |
| `MIDAZ_DEBUG` | Enable debug mode with verbose logging | `false` | Set to "true" for detailed logs |

Example:
```
MIDAZ_TIMEOUT=30
MIDAZ_DEBUG=true
```

## Retry Configuration

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_MAX_RETRIES` | Maximum number of retry attempts | `3` | Controls retry attempts for failed requests |
| `MIDAZ_ENABLE_RETRIES` | Enable retry mechanism | `true` | Set to "false" to disable retries |
| `MIDAZ_RETRY_WAIT_MIN` | Minimum wait time between retries (ms) | `1000` | Initial delay before first retry |
| `MIDAZ_RETRY_WAIT_MAX` | Maximum wait time between retries (ms) | `30000` | Maximum delay between retries |

Example:
```
MIDAZ_MAX_RETRIES=5
MIDAZ_ENABLE_RETRIES=true
MIDAZ_RETRY_WAIT_MIN=500
MIDAZ_RETRY_WAIT_MAX=10000
```

## Feature Flags

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_IDEMPOTENCY` | Enable automatic idempotency key generation | `true` | Controls idempotency for API requests |

Example:
```
MIDAZ_IDEMPOTENCY=true
```

## Observability Configuration

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_OTEL_ENDPOINT` | OpenTelemetry collector endpoint | None | For sending traces/metrics |
| `MIDAZ_LOG_LEVEL` | Logging level | `info` | `debug`, `info`, `warn`, `error` |

Observability is primarily configured through code using the SDK's options:

```go
provider, err := observability.New(ctx,
  observability.WithServiceName("my-service"),
  observability.WithEnvironment("production"),
  observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, logging
)

client, err := client.New(
  client.WithObservabilityProvider(provider),
  // Other options...
)
```

## Testing Configuration

| Variable | Purpose | Default | Notes |
|----------|---------|---------|-------|
| `MIDAZ_SKIP_AUTH_CHECK` | Skip auth token validation | `false` | For testing only |

Example:
```
MIDAZ_SKIP_AUTH_CHECK=true
```

## Example Configuration

Here's a complete example of a `.env` file with all available configuration options:

```
# Authentication
MIDAZ_AUTH_TOKEN=midaz-auth-token-123456

# Environment configuration
MIDAZ_ENVIRONMENT=local

# SDK configuration
MIDAZ_USER_AGENT=MyApp/1.0.0 (Midaz-Go-SDK/1.0.0)

# API URLs
MIDAZ_BASE_URL=http://localhost
MIDAZ_ONBOARDING_URL=http://localhost:3000/v1
MIDAZ_TRANSACTION_URL=http://localhost:3001/v1

# HTTP configuration
MIDAZ_TIMEOUT=30
MIDAZ_DEBUG=true

# Retry configuration
MIDAZ_MAX_RETRIES=3
MIDAZ_ENABLE_RETRIES=true
MIDAZ_RETRY_WAIT_MIN=1000
MIDAZ_RETRY_WAIT_MAX=30000

# Feature flags
MIDAZ_IDEMPOTENCY=true

# Testing configuration (for development only)
MIDAZ_SKIP_AUTH_CHECK=false

# Observability configuration
MIDAZ_OTEL_ENDPOINT=http://localhost:4318
MIDAZ_LOG_LEVEL=debug
```