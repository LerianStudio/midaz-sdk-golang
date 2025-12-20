# Resilience Guide

## Overview

The Go SDK includes retry logic, circuit breakers, and connection pooling for reliable API communication.

## Retry Configuration

```go
client, err := sdk.NewClient(
    sdk.WithRetryConfig(retry.Config{
        MaxRetries:    3,
        InitialDelay:  100 * time.Millisecond,
        BackoffFactor: 2.0,
        JitterFactor:  0.25,
        MaxDelay:      10 * time.Second,
    }),
)
```

## Retry Options

| Option | Default | Description |
|--------|---------|-------------|
| `MaxRetries` | 3 | Maximum retry attempts |
| `InitialDelay` | 100ms | First retry delay |
| `BackoffFactor` | 2.0 | Exponential multiplier |
| `JitterFactor` | 0.25 | Randomization factor |
| `MaxDelay` | 10s | Maximum delay cap |

## Presets

```go
// High reliability preset
sdk.WithRetryConfig(retry.HighReliability())
// MaxRetries: 5, InitialDelay: 200ms, BackoffFactor: 2.5

// Fast fail preset
sdk.WithRetryConfig(retry.FastFail())
// MaxRetries: 1, InitialDelay: 50ms
```

See: `pkg/retry/retry.go:66-127`

## Retryable HTTP Codes

The SDK automatically retries on:
- `408` Request Timeout
- `429` Too Many Requests
- `500` Internal Server Error
- `502` Bad Gateway
- `503` Service Unavailable
- `504` Gateway Timeout

## Circuit Breaker

```go
import "github.com/lerianstudio/midaz-sdk-go/v2/pkg/concurrent"

breaker := concurrent.NewCircuitBreaker(concurrent.BreakerConfig{
    Threshold:    5,           // Failures before opening
    Timeout:      30 * time.Second, // Reset timeout
    HalfOpenMax:  2,           // Test requests in half-open
})
```

## Connection Pooling

HTTP client is pre-configured with:
- Keep-alive connections
- Connection reuse
- Configurable pool size

```go
client, err := sdk.NewClient(
    sdk.WithHTTPClient(&http.Client{
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 10,
            IdleConnTimeout:     90 * time.Second,
        },
    }),
)
```

## Idempotency

For safe retries on mutating operations:

```go
ctx := context.WithValue(ctx, "X-Idempotency", "unique-request-id")
result, err := client.Entity.Transactions().Create(ctx, input)
```

## Timeout Configuration

```go
client, err := sdk.NewClient(
    sdk.WithTimeout(30 * time.Second),
)

// Or per-request
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
result, err := client.Entity.Accounts().Get(ctx, orgID, ledgerID, accountID)
```

## Key File References

| Purpose | File |
|---------|------|
| Retry logic | `pkg/retry/retry.go:66-127` |
| Circuit breaker | `pkg/concurrent/circuit_breaker.go` |
| HTTP client | `entities/http.go:45-85` |
