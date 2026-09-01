package entities

// Header names used in HTTP requests.
//
// Service-name string keys ("onboarding", "tracer") are defined
// once in [github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config]; do not
// re-define them here.
const (
	// HeaderTotalCount is the HTTP header name used by count endpoints.
	HeaderTotalCount = "X-Total-Count"

	// tracerAPIVersionPath is the version segment the Tracer plane base URL must
	// carry. The Tracer OpenAPI spec declares servers:[{url: "/v1"}] with
	// unversioned paths, so the version cannot come from the path the way it does
	// on the Ledger plane. Mirrors
	// [github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config.DefaultTracerAPIVersionPath].
	tracerAPIVersionPath = "/v1"
)
