// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/validation/core"
)

// TransactionsListOpts is the typed options struct for the transaction list
// surface: V1.Transactions.List and the All / Pages iterators.
//
// Embeds CursorListOpts for the shared cursor/sort/date-range fields;
// attaches a TransactionsFilters sub-struct carrying the filter fields the
// transactions endpoints accept (see TransactionsFilters for which endpoint
// honors which).
//
// TransactionsListOpts is a value type. Concurrent-safe by construction.
//
// IMPORTANT: This is a CURSOR-PAGINATED endpoint. The struct does NOT
// expose Page or Offset fields. Iterate by passing back the NextCursor
// from the previous response's Pagination. v3 compile-time prevents
// the v2 footgun where setting WithPage on a cursor endpoint silently
// dropped the value and emitted a stderr warning (audit finding 5.5).
type TransactionsListOpts struct {
	CursorListOpts

	// Filters narrows the result set. Zero value means no narrowing.
	Filters TransactionsFilters
}

// TransactionsFilters is the typed filter set for the transactions endpoints. It
// replaces the v2 mega-struct ListOptions and its 30+ fluent setters that mostly
// no-op'd here (audit finding 5.12), but the ledger honors different subsets per
// endpoint, so treat the field docs below as the contract:
//
//   - List, BOTH surfaces: the metadata predicate and the inherited date range
//     are the only narrowing the ledger applies. Setting any other field here is
//     REFUSED with a validation error naming it, because the ledger would return
//     every transaction and a caller reads that as a narrowed result. /v1 and
//     /v2 behave identically because they are literally the same server
//     function: transaction_handler.go:500 and
//     transaction_v2_mirror_handler.go:148 both call handler.getAllTransactions.
//   - Count, BOTH surfaces: Status and Route, plus the date range
//     (countTransactionsByFilters declares both filters). Count with NO date
//     range counts TODAY only — see the Count methods.
//
// Narrow client-side, carry the identifier in metadata, or use Count for Status
// and Route, rather than relying on the refused fields here.
type TransactionsFilters struct {
	// AssetCode narrows by asset code (e.g. "USD").
	//
	// REFUSED on List (both surfaces); not honored by any endpoint. The server
	// parses asset_code and then drops it: ToCursorPagination
	// (pkg/net/http/httputils.go:533-539) returns only limit, cursor, sort_order
	// and the date range, and that is the only value the repository receives.
	AssetCode string

	// Status narrows by transaction status (e.g. "APPROVED").
	// Valid values mirror TransactionStatusCode:
	// CREATED, PENDING, APPROVED, CANCELED, NOTED.
	//
	// Honored by Count. REFUSED on List (both surfaces): parsed by the server
	// and dropped by ToCursorPagination, same as AssetCode.
	Status string

	// Reference narrows by external transaction reference.
	//
	// REFUSED on List (both surfaces); not honored by any endpoint. The server's
	// query switch (httputils.go:150-252) has no case for it, so it is never
	// even parsed.
	Reference string

	// DestinationAccount narrows to transactions targeting a specific account.
	//
	// REFUSED on List (both surfaces); never parsed by the server, same as
	// Reference.
	DestinationAccount string

	// SourceAccount narrows to transactions originating from a specific account.
	//
	// REFUSED on List (both surfaces); never parsed by the server, same as
	// Reference.
	SourceAccount string

	// Route narrows by transaction route name (e.g. "cashin", "cashout").
	//
	// Honored by Count. REFUSED on List (both surfaces): the server's query
	// switch matches route_id and route_code, never a bare route.
	Route string

	// MetadataKey and MetadataValue filter transactions by a single metadata
	// field, rendered on the wire as `metadata.<MetadataKey>=<MetadataValue>`.
	// This is the ONLY content filter the List endpoint honors, which is why
	// correlation identifiers belong in metadata (see
	// CreateTransactionInput.Metadata and models/correlation).
	// The ledger honors ONE metadata predicate per request (it does not
	// AND-combine multiple metadata keys), so this is a single pair by design.
	// Both must be set together; MetadataKey must obey the storage-layer key
	// rules (non-empty, <=100 chars, no '.' and no leading '$').
	//
	// PERFORMANCE: transaction metadata keys are NOT indexed by default on the
	// ledger (only entity_id is). For a hot correlation key such as
	// "transferId", have an operator create the index via the Midaz admin
	// CreateIndex API to avoid a Mongo collection scan at scale.
	MetadataKey   string
	MetadataValue string
}

// Validate enforces SDK-side preconditions on TransactionsListOpts.
func (o TransactionsListOpts) Validate() error {
	if err := ValidateCursorListOpts("TransactionsListOpts.Validate", o.CursorListOpts); err != nil {
		return err
	}

	return validateMetadataFilter(o.Filters.MetadataKey, o.Filters.MetadataValue)
}

// validateMetadataFilter enforces that the metadata list-filter is either
// fully unset or a complete key/value pair, and that the key obeys the
// storage-layer metadata rules (reused from core.ValidateMetadata: non-empty,
// <=100 chars, no '.' and no leading '$', value within type/length bounds).
func validateMetadataFilter(key, value string) error {
	if key == "" && value == "" {
		return nil
	}

	if key == "" || value == "" {
		return errors.New("TransactionsListOpts.Validate: metadata filter requires both MetadataKey and MetadataValue")
	}

	if err := core.ValidateMetadata(map[string]any{key: value}); err != nil {
		return fmt.Errorf("TransactionsListOpts.Validate: invalid metadata filter: %w", err)
	}

	return nil
}

// ToQueryParams renders an TransactionsListOpts into the wire query map.
//
// Exported because the entity layer in package entities calls it.
// Treat as an internal contract; not part of the user-facing API.
func (o TransactionsListOpts) ToQueryParams() map[string]string {
	params := CursorQueryParams(o.CursorListOpts)

	if o.Filters.AssetCode != "" {
		params["asset_code"] = o.Filters.AssetCode
	}

	if o.Filters.Status != "" {
		params["status"] = o.Filters.Status
	}

	if o.Filters.Reference != "" {
		params["reference"] = o.Filters.Reference
	}

	if o.Filters.DestinationAccount != "" {
		params["destination_account"] = o.Filters.DestinationAccount
	}

	if o.Filters.SourceAccount != "" {
		params["source_account"] = o.Filters.SourceAccount
	}

	if o.Filters.Route != "" {
		params["route"] = o.Filters.Route
	}

	if o.Filters.MetadataKey != "" && o.Filters.MetadataValue != "" {
		params["metadata."+o.Filters.MetadataKey] = o.Filters.MetadataValue
	}

	return params
}
