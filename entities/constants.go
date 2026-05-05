// Package entities provides direct access to Midaz API services.
package entities

// Service types for API endpoints.
// These constants define the different service types used to identify
// which API endpoint to use when making requests to the Midaz platform.
const (
	// ServiceOnboarding identifies the onboarding service API.
	// This service handles organization, ledger, account, asset, and portfolio management.
	ServiceOnboarding = "onboarding"

	// ServiceTransaction identifies the transaction service API.
	// This service handles transaction creation, retrieval, and management,
	// as well as operations and balances.
	ServiceTransaction = "transaction"
)

// Header names used in HTTP requests.
const (
	// HeaderTenantID is the HTTP header name used to propagate the tenant identifier.
	// This is an optional compatibility header for deployments or gateways that honor
	// explicit tenant headers. In the reference Midaz path, authenticated claims remain
	// the primary tenant source of truth.
	HeaderTenantID = "X-Tenant-ID"

	// HeaderTotalCount is the HTTP header name used by count endpoints.
	HeaderTotalCount = "X-Total-Count"
)

// Common boolean string literal used in HTTP header values for internal
// idempotency negotiation (X-Midaz-Caller-Idempotency, X-Midaz-Auto-Idempotency).
const (
	// BoolTrue represents the string value "true".
	BoolTrue = "true"
)
