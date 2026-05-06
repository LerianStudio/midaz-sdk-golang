// Package entities provides shared service-entity construction helpers.
//
// Track 7F (audit 7.8) — every entity service implementation has the same
// two fields (httpClient + baseURLs) and the same constructor body. The
// helpers in this file consolidate that body to one site so the entities
// stay focused on per-service business logic.
package entities

import (
	"net/http"
)

// serviceEntity is the embeddable base for every entity service implementation.
//
// All ~18 entities (accounts, account_types, aliases, asset_rates, assets,
// balances, holders, ledgers, metadata_indexes, operations, operation_routes,
// organizations, portfolios, segments, transactions, transaction_routes) share
// the same two fields. Embedding `serviceEntity` removes the per-file
// duplication and gives every entity setDefaultTenantID for free.
//
// Note: we do not embed via pointer because the value is constructed once
// and never replaced; the embedded HTTPClient is a pointer that holds the
// mutable state.
type serviceEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

// setDefaultTenantID propagates a default tenant ID into the embedded
// HTTPClient. Promoted automatically to every embedding entity, eliminating
// the 16-copy boilerplate that lived in v2.
func (e *serviceEntity) setDefaultTenantID(tenantID string) {
	if e == nil || e.httpClient == nil {
		return
	}

	e.httpClient.setTenantIDLocked(tenantID)
}

// newServiceEntity builds the shared HTTPClient and prepares the per-service
// base URL map. Used by every entity constructor, replacing 18 duplicated
// copies of the same three lines.
func newServiceEntity(client *http.Client, authToken string, baseURLs map[string]string) serviceEntity {
	return serviceEntity{
		httpClient: NewHTTPClient(client, authToken, nil),
		baseURLs:   prepareServiceBaseURLs(baseURLs),
	}
}
