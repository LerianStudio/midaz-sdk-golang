package models

// TransactionStatusCode is the canonical Midaz ledger transaction status.
// Values mirror github.com/LerianStudio/midaz/v3/pkg/constant (server source of
// truth); the server's filter handler documents this exact 5-value set
// (Enums(CREATED, APPROVED, PENDING, CANCELED, NOTED)). There is no REJECTED,
// COMPLETED, or FAILED status anywhere in the server contract.
//
// This is a distinct concept from the account/resource Status* constants below
// (ACTIVE/INACTIVE/PENDING/CLOSED). Note the *Pending near-collision: a
// transaction can be PENDING (awaiting commit), and an account can be PENDING
// (awaiting activation) — they are unrelated state machines.
//
// Transaction lifecycle (server: transaction_state_handlers.go):
//   - create(pending:true)  → PENDING  (operations not yet applied to balances)
//   - create(pending:false) → APPROVED (operations applied immediately)
//   - annotation create     → NOTED    (metadata-only, no balance impact)
//   - commit  requires PENDING → APPROVED
//   - cancel  requires PENDING → CANCELED
//   - revert  requires APPROVED → creates a NEW child reversal transaction
//     (CREATED → APPROVED); the original transaction's status is NEVER mutated.
//     A re-attempt is rejected by the parent-transaction guard (see
//     errors.IsRevertAlreadyExistsError).
//
// Status.Code on the wire is a plain string for JSON compatibility; compare
// against these constants via the string conversion:
//
//	if tx.Status.Code == string(models.TransactionStatusApproved) {
//	    fmt.Println("transaction applied to balances")
//	}
//
//	switch tx.Status.Code {
//	case string(models.TransactionStatusPending):
//	    // awaiting commit or cancel
//	case string(models.TransactionStatusApproved):
//	    // applied to balances
//	case string(models.TransactionStatusCanceled):
//	    // terminal: cancelled before commit
//	case string(models.TransactionStatusNoted):
//	    // metadata-only annotation
//	}
type TransactionStatusCode string

const (
	// TransactionStatusCreated is the initial status of a child reversal
	// transaction before it is approved. Not observed for ordinary
	// create/commit flows.
	TransactionStatusCreated TransactionStatusCode = "CREATED"

	// TransactionStatusPending is a transaction created with pending:true.
	// Its operations are not yet applied to balances; it awaits an explicit
	// commit (→ APPROVED) or cancel (→ CANCELED).
	TransactionStatusPending TransactionStatusCode = "PENDING"

	// TransactionStatusApproved is a transaction whose operations have been
	// applied to account balances. This is the terminal success state for
	// both immediate (pending:false) and committed transactions.
	TransactionStatusApproved TransactionStatusCode = "APPROVED"

	// TransactionStatusCanceled is a transaction that was cancelled before
	// commit. Only PENDING transactions can be cancelled; APPROVED
	// transactions are reversed via a child reversal transaction, not cancelled.
	TransactionStatusCanceled TransactionStatusCode = "CANCELED"

	// TransactionStatusNoted is an annotation transaction: metadata-only,
	// with no impact on account balances.
	TransactionStatusNoted TransactionStatusCode = "NOTED"
)

// Account status constants define the possible states of an account in the Midaz system.
// These constants are used throughout the SDK to represent account statuses in a consistent way.
//
// This resource status set is distinct from the transaction TransactionStatusCode
// set above. The shared "PENDING" spelling is coincidental: an account PENDING
// means "awaiting activation/approval", whereas a transaction PENDING means
// "awaiting commit". Do not compare across the two vocabularies.
const (
	// StatusActive represents an active resource that can be used normally
	// Active accounts can participate in transactions as both source and destination.
	StatusActive = "ACTIVE"

	// StatusInactive represents a temporarily inactive resource
	// Inactive accounts cannot participate in new transactions but can be reactivated.
	StatusInactive = "INACTIVE"

	// StatusPending represents a resource awaiting activation or approval
	// Pending accounts are in the process of being set up or approved.
	StatusPending = "PENDING"

	// StatusClosed represents a permanently closed resource
	// Closed accounts cannot participate in new transactions and cannot be reopened.
	StatusClosed = "CLOSED"
)

// SortDirection represents the direction for sorting results in list operations.
// This type is used to ensure consistent sort direction values across the SDK.
type SortDirection string

const (
	// SortAscending indicates ascending sort order (A→Z, 0→9)
	SortAscending SortDirection = "asc"

	// SortDescending indicates descending sort order (Z→A, 9→0)
	SortDescending SortDirection = "desc"
)

// PaginationDefaults contains default values for pagination parameters.
// These constants define the standard default behavior for list operations.
const (
	// DefaultLimit is the default number of items to return per page
	DefaultLimit = 10

	// MaxLimit is the maximum number of items that can be requested per page
	MaxLimit = 100

	// DefaultOffset is the default starting position for pagination
	DefaultOffset = 0

	// DefaultPage is the default page number for backward compatibility
	DefaultPage = 1

	// DefaultSortDirection is the default sort direction.
	DefaultSortDirection = string(SortAscending)
)

// QueryParamNames contains the names of query parameters used for API requests.
// These constants ensure consistent parameter naming across all SDK operations.
const (
	// QueryParamLimit is the query parameter name for limit
	QueryParamLimit = "limit"

	// QueryParamOffset is the unsupported legacy query parameter name for offset.
	// Deprecated: current Midaz list endpoints use page-based pagination on the wire;
	// ListOptions.ToQueryParams never emits this parameter.
	QueryParamOffset = "offset"

	// QueryParamPage is the query parameter name for page (backward compatibility)
	QueryParamPage = "page"

	// QueryParamCursor is the query parameter name for cursor
	QueryParamCursor = "cursor"

	// QueryParamOrderDirection is the query parameter name for sort direction.
	QueryParamOrderDirection = "sort_order"

	// QueryParamStartDate is the query parameter name for start date.
	QueryParamStartDate = "start_date"

	// QueryParamEndDate is the query parameter name for end date.
	QueryParamEndDate = "end_date"
)
