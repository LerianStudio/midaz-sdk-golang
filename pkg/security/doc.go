// Package security holds defensive validators that the Midaz Go SDK
// applies to inputs before they cross trust boundaries — most
// importantly, the URL allowlist that prevents Server-Side Request
// Forgery (SSRF) when a caller-supplied address feeds into an HTTP
// request.
//
// The package is intentionally small. The big idea: every config-time
// URL the SDK accepts (BaseURL, OnboardingURL, TransactionURL, CRMURL,
// AccessManager.Address) goes through the same allowlist check before
// the SDK opens a socket. Loopback, RFC1918, and other typical
// SSRF-bait addresses are rejected unless explicitly enabled for
// local-development work.
//
// # Public surface
//
//   - [ValidateAddress] — host-and-scheme guard for raw URL strings.
//     Returns a typed configuration error rejecting invalid input
//     before it hits the HTTP transport.
//   - SSRF guard helpers used by the HTTP transport itself.
//
// # When to use this package directly
//
// Almost never. The SDK applies these checks at config-validation
// time. Reach into pkg/security only when authoring a custom
// transport that wraps the SDK and wants to apply the same guards
// to additional URLs (e.g., webhook delivery destinations).
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithBaseURL]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithAccessManager]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors] —
//     errors returned when SSRF-protection rejects a URL
package security
