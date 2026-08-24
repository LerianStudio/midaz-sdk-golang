package entities

import (
	"net/http"
)

// serviceEntity is the embeddable base for every entity service implementation.
//
// All 15 services (accounts, account_types, asset_rates, assets,
// balances, holders, ledgers, metadata_indexes, operations, operation_routes,
// organizations, portfolios, segments, transactions, transaction_routes)
// share the same two fields. Embedding `serviceEntity` removes per-file
// duplication.
//
// In production every Entity hands the SAME [*HTTPClient] pointer to all 15
// services (see [Entity.initServices]). Sharing the client at this level is
// essential: it is the bus that carries mutable auth-token state, the
// singleflight token-refresh group, the customRetryPolicy, and the
// observability fields. Fifteen separate clients would each refresh tokens
// independently on a 401 burst and ignore mid-lifetime [*HTTPClient].SetX
// calls made on the parent Entity.
//
// Note: we embed by value because the struct is constructed once and never
// replaced; the embedded *HTTPClient is the pointer that holds the mutable
// state.
type serviceEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// entityHTTPClient returns the embedded *HTTPClient. Promoted automatically
// to every embedding entity. Production callers go through
// [Entity.GetEntityHTTPClient] — this helper exists for internal use and
// for the package-private direct test constructors.
func (e *serviceEntity) entityHTTPClient() *HTTPClient {
	if e == nil {
		return nil
	}

	return e.httpClient
}

// newSharedServiceEntity wraps a pre-built *HTTPClient inside the embeddable
// serviceEntity. Used by [Entity.initServices] to hand the SAME parent
// [*HTTPClient] to every service entity, so all 15 services share one
// auth-token cache, one singleflight token-refresh group, one
// customRetryPolicy, and one observability surface.
func newSharedServiceEntity(httpClient *HTTPClient, baseURLs map[string]string) serviceEntity {
	return serviceEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
	}
}

// newServiceEntity is the test-friendly constructor used by the per-service
// newXxxEntity functions when a service is built in isolation (i.e., outside
// the parent Entity). Production code always routes through
// [Entity.initServices] which uses [newSharedServiceEntity] so all 15
// services share one client.
//
// Tests that call newXxxEntity directly construct a fresh *HTTPClient here.
// That is intentional: an isolated test service has no parent Entity to
// share state with, so spinning up a per-test client is the simplest path.
// Production wiring is unaffected.
func newServiceEntity(client *http.Client, authToken string, baseURLs map[string]string) serviceEntity {
	return newSharedServiceEntity(NewHTTPClient(client, authToken, nil), baseURLs)
}
