// Package retry implements the retry-with-exponential-backoff policy
// used by the Midaz Go SDK and exposes the same machinery for consumer
// code that wants to apply identical policies to non-SDK calls.
//
// Two layers live here:
//
//   - The generic [Do] entry point, which wraps any error-returning
//     function in a configurable retry policy. Use this for retrying
//     non-HTTP work like database calls or RPC clients.
//   - The HTTP-specific entry points ([DoHTTPRequest], [DoHTTP]),
//     which understand HTTP semantics — retryable status codes (5xx,
//     408, 425, 429), Retry-After headers, and the 'safe to retry'
//     question for unsafe methods.
//
// # Default policy
//
//   - 3 retry attempts (4 total tries).
//   - 100ms initial delay, 10s max delay, 2.0x backoff factor.
//   - 0.25 jitter factor to prevent thundering-herd retries.
//   - Retryable on transport errors (DNS, connection-refused, timeouts)
//     and HTTP 408, 425, 429, 500, 502, 503, 504.
//
// # Quickstart
//
//	err := retry.Do(ctx, func() error {
//	    return doSomething()
//	},
//	    retry.WithMaxRetries(5),
//	    retry.WithBackoffFactor(3.0),
//	)
//
// # Two-layer surface
//
// The SDK's client-construction options expose this package via:
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithRetryOptions] —
//     forward retry.Option values into the SDK's HTTP retry policy.
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithCustomRetryPolicy] —
//     replace the policy with an arbitrary predicate.
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithoutRetries] —
//     disable retries entirely.
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithRetryOptions]
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithCustomRetryPolicy]
//   - [github.com/LerianStudio/midaz-sdk-golang/v6.WithoutRetries]
//   - examples/07-retries — runnable demo
package retry
