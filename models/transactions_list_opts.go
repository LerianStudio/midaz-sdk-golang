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
//     (countTransactionsByFilters declares both filters). Every OTHER field —
//     AssetCode, Reference, SourceAccount, DestinationAccount, and the metadata
//     predicate — is REFUSED here too, for the same reason it is on List: the
//     count endpoint declares no parameter for any of them, so they would be
//     dropped and the caller would read an unnarrowed count as a narrowed one.
//     Count with NO date range counts TODAY only, not the whole ledger: the
//     server defaults an absent start_date to today 00:00:00 UTC and an absent
//     end_date to today 23:59:59 UTC (count_transactions_by_filters.go:63-65 and
//     75-77). To count any other span set StartDate and EndDate below in
//     YYYY-MM-DD, the same format List takes; each names a WHOLE day and both
//     ends are inclusive, so 2026-01-01 through 2026-01-31 covers all of January.
//
// Read together: Status and Route are honoured on Count and refused on List; the
// metadata predicate is honoured on List and refused on Count; AssetCode,
// Reference, SourceAccount and DestinationAccount are refused everywhere.
//
// Narrow client-side, carry the identifier in metadata, or use Count for Status
// and Route, rather than relying on the refused fields here.
type TransactionsFilters struct {
	// AssetCode is REFUSED by every transaction surface — List and Count, on both
	// server versions. It narrows nothing.
	//
	// The server parses asset_code and then drops it: ToCursorPagination
	// (pkg/net/http/httputils.go:533-539) returns only limit, cursor, sort_order
	// and the date range, and that is the only value the repository receives. The
	// count endpoint declares no asset_code parameter at all.
	AssetCode string

	// Status narrows by transaction status (e.g. "APPROVED").
	// Valid values mirror TransactionStatusCode:
	// CREATED, PENDING, APPROVED, CANCELED, NOTED.
	//
	// Honored by Count. REFUSED on List (both surfaces): parsed by the server
	// and dropped by ToCursorPagination, same as AssetCode.
	Status string

	// Reference is REFUSED by every transaction surface — List and Count, on both
	// server versions. It narrows nothing.
	//
	// The server's query switch (httputils.go:150-252) has no case for it, so it
	// is never even parsed. Carry an external reference in metadata instead; the
	// metadata predicate is the one content filter List honours.
	Reference string

	// DestinationAccount is REFUSED by every transaction surface — List and
	// Count, on both server versions. It narrows nothing: never parsed by the
	// server, same as Reference.
	DestinationAccount string

	// SourceAccount is REFUSED by every transaction surface — List and Count, on
	// both server versions. It narrows nothing: never parsed by the server, same
	// as Reference.
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
