// Package security holds defensive validators that the Midaz Go SDK
// applies to outbound HTTP requests before they leave the process.
//
// The package is intentionally small. It enforces a narrow set of
// transport-level safety properties on every request the SDK issues
// — properties that are cheap to check, hard to violate by accident,
// and that the standard library does not enforce on its own.
//
// # Guarantees
//
// [ValidateOutboundRequest] enforces three rules on outbound HTTP
// requests:
//
//   - URL must not contain a userinfo component (CWE-639). Credentials
//     embedded in URLs leak through logs, proxies, and referrer
//     headers; the SDK refuses to send them.
//   - Scheme must be http or https. Anything else (file://, ftp://,
//     gopher://, …) is rejected outright.
//   - Plain http:// is only allowed for loopback hosts (localhost,
//     127.0.0.0/8 aliases, ::1, the .localhost reserved TLD per
//     RFC 6761 §6.3). Non-loopback http:// targets are rejected, so
//     production traffic cannot accidentally fall back to cleartext.
//
// # Not provided
//
// The SDK does not block https:// requests to RFC1918, link-local,
// or cloud-metadata addresses (e.g. 10.0.0.0/8, 192.168.0.0/16,
// 169.254.169.254). Self-hosted Midaz deployments routinely sit on
// private network ranges and TLS-terminated internal endpoints, so a
// blanket private-IP block here would break legitimate topologies.
//
// Callers that accept SDK target URLs from untrusted input (BaseURL,
// LedgerURL, CRMURL, AccessManager.Address) are responsible for
// validating those URLs against their own allowlist before handing
// them to the SDK if SSRF against internal services is in scope for
// their threat model.
//
// # Public surface
//
//   - [ValidateOutboundRequest] — the validator described above.
//     Invoked automatically by the SDK's HTTP transport on every
//     request; safe to call directly from custom transports that
//     want the same guarantees.
//
// # When to use this package directly
//
// Almost never. The SDK applies [ValidateOutboundRequest] inside its
// HTTP transport. Reach into pkg/security only when authoring a
// custom transport that wraps the SDK and wants to apply the same
// guard to additional URLs (e.g., webhook delivery destinations).
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithBaseURL]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithAccessManager]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors] —
//     errors returned when validation rejects a request
package security
