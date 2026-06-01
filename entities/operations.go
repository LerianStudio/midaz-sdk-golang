package entities

//go:generate mockgen -source=operations.go -destination=mocks/mock_operations.go -package=mocks OperationsService

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// OperationsService defines the interface for operation-related operations.
// It provides methods to list, retrieve, and update operations
// associated with accounts and transactions.
type OperationsService interface {
	// ListOperations retrieves one cursor-paginated page of operations for a specific account.
	//
	// Operations represent the individual accounting entries (debits and credits) that make up
	// transactions in the ledger. This endpoint uses cursor-based pagination; advance pages by
	// reading page.Pagination.NextCursor and assigning it to opts.Cursor. The
	// OperationsListOpts struct has no Page or Offset field by construction — the v2 footgun
	// where setting WithPage on a cursor endpoint silently dropped the value (audit 5.5) is
	// structurally impossible in v3.
	//
	// Parameters:
	//   - ctx: Context for the request, used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger containing the account.
	//   - accountID: The ID of the account to retrieve operations for.
	//   - opts: Typed cursor list options. Limit caps the page size; Filters narrow results.
	//
	// Returns:
	//   - *models.ListResponse[models.Operation]: One page of operations. Use Pagination.NextCursor
	//     for traversal — Total may be unknown on cursor endpoints.
	//   - error: A typed *errors.Error. Validation errors return category validation BEFORE
	//     any HTTP request is sent.
	//
	// Example - One-page fetch with filtering:
	//
	//	opts := models.OperationsListOpts{
	//	    CursorListOpts: models.CursorListOpts{
	//	        Limit:         10,
	//	        SortDirection: models.SortDesc,
	//	    },
	//	    Filters: models.OperationsFilters{Type: "DEBIT", AssetCode: "USD"},
	//	}
	//	page, err := c.Entity.Operations.ListOperations(ctx, "org-123", "ledger-456", "account-789", opts)
	//	if err != nil { return err }
	//	for _, op := range page.Items {
	//	    fmt.Printf("op %s: %s %d %s\n", op.ID, op.Type, op.Amount, op.AssetCode)
	//	}
	//
	// For multi-page traversal, prefer ListOperationsAll (auto cursor advance, range-loop friendly).
	ListOperations(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) (*models.ListResponse[models.Operation], error)

	// ListOperationsAll returns an iter.Seq2 that yields each Operation across every page
	// until the cursor is exhausted or the context is cancelled. Idiomatic v3 iteration:
	//
	//	for op, err := range c.Entity.Operations.ListOperationsAll(ctx, organizationID, ledgerID, accountID, opts) {
	//	    if err != nil { return err }
	//	    process(op)
	//	}
	ListOperationsAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[models.Operation, error]

	// ListOperationsPages returns an iter.Seq2 that yields each *ListResponse page. Use this
	// when you need page-level metadata (Pagination, ItemCount) rather than flattened items.
	ListOperationsPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error]

	// GetOperation retrieves a specific operation by its ID.
	//
	// Operations represent the individual accounting entries (debits and credits) that make up
	// transactions in the ledger. This method retrieves a single operation by its unique identifier.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger containing the account. Must be a valid ledger ID.
	//   - accountID: The ID of the account the operation belongs to. Must be a valid account ID.
	//   - operationID: The unique identifier of the operation to retrieve. Must be a valid operation ID.
	//   - transactionID: The ID of the transaction the operation belongs to. Must be a valid transaction ID.
	//
	// Returns:
	//   - *models.Operation: The operation if found, containing details such as:
	//     - ID: The unique identifier of the operation
	//     - Type: The operation type (debit or credit)
	//     - AccountID: The account affected by the operation
	//     - Amount: The monetary value of the operation
	//     - AssetCode: The currency or asset type involved
	//     - TransactionID: The ID of the transaction this operation belongs to
	//   - error: An error if the operation fails. Possible errors include:
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization, ledger, account, or operation ID)
	//     - Network or server errors
	//
	// Example:
	//
	//	// Retrieve a specific operation
	//	operation, err := operationsService.GetOperation(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    "operation-abc",
	//	    "transaction-xyz",
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to retrieve operation: %v", err)
	//	}

	//
	//	// Process the operation details
	//	fmt.Printf("Operation: %s\n", operation.ID)
	//	fmt.Printf("Type: %s\n", operation.Type)
	//	fmt.Printf("Account: %s\n", operation.AccountID)
	//	fmt.Printf("Transaction: %s\n", operation.TransactionID)
	//
	//	// Calculate the decimal value of the amount
	//	decimalValue := float64(operation.Amount) / math.Pow10(int(operation.Scale))
	//	fmt.Printf("Amount: %.2f %s\n", decimalValue, operation.AssetCode)
	//
	//	// Check if this is a debit or credit operation
	//	if operation.Type == models.OperationTypeDebit {
	//	    fmt.Println("This is a debit operation (funds leaving the account)")
	//	} else if operation.Type == models.OperationTypeCredit {
	//	    fmt.Println("This is a credit operation (funds entering the account)")
	//	}

	GetOperation(ctx context.Context, organizationID, ledgerID, accountID, operationID string) (*models.Operation, error)

	// UpdateTransactionOperation updates an existing operation by transaction scope.
	// The organizationID, ledgerID, and transactionID parameters specify which organization, ledger,
	// and transaction the operation belongs to.
	// The operationID parameter is the unique identifier of the operation to update.
	// The input parameter contains the operation details to update; it must be non-nil
	// and at least one mutable field must be set.
	// Returns the updated operation, or an error if the operation fails.
	UpdateTransactionOperation(ctx context.Context, organizationID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error)
}

// operationsEntity implements the OperationsService interface.
// It handles the communication with the Midaz API for operation-related operations.
type operationsEntity struct {
	serviceEntity
}

// newOperationsEntity wires the OperationsService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Operations.
func newOperationsEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationsService {
	return &operationsEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// ListOperations lists operations for an account with optional filters.
func (e *operationsEntity) ListOperations(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) (*models.ListResponse[models.Operation], error) {
	const operation = "ListOperations"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	url := e.buildURL(organizationID, ledgerID, accountID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.ToQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Operation]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	normalizeOperationListResponse(&response)

	return &response, nil
}

// ListOperationsAll yields every operation matching the request,
// transparently advancing pagination via the server-issued NextCursor.
func (e *operationsEntity) ListOperationsAll(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[models.Operation, error] {
	return flattenPages(e.ListOperationsPages(ctx, organizationID, ledgerID, accountID, opts))
}

// ListOperationsPages yields one full *ListResponse[Operation] per page,
// transparently advancing pagination via the server-issued NextCursor.
func (e *operationsEntity) ListOperationsPages(ctx context.Context, organizationID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error] {
	ctx = requestContext(ctx)

	return func(yield func(*models.ListResponse[models.Operation], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListOperations(ctx, organizationID, ledgerID, accountID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			next := page.Pagination.NextCursor
			if next == "" {
				return
			}

			current.Cursor = next
		}
	}
}

// GetOperation retrieves an operation by its ID.
//
// Operations represent the individual accounting entries (debits and credits) that make up
// transactions in the ledger. This method retrieves a single operation by its unique identifier.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger containing the account. Must be a valid ledger ID.
//   - accountID: The ID of the account the operation belongs to. Must be a valid account ID.
//   - operationID: The unique identifier of the operation to retrieve. Must be a valid operation ID.
//   - transactionID: The ID of the transaction the operation belongs to. Must be a valid transaction ID.
//
// Returns:
//   - *models.Operation: The operation if found, containing details such as:
//   - ID: The unique identifier of the operation
//   - Type: The operation type (debit or credit)
//   - AccountID: The account affected by the operation
//   - Amount: The monetary value of the operation
//   - AssetCode: The currency or asset type involved
//   - TransactionID: The ID of the transaction this operation belongs to
//   - error: An error if the operation fails. Possible errors include:
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization, ledger, account, or operation ID)
//   - Network or server errors
//
// Example:
//
//	// Retrieve a specific operation
//	operation, err := operationsService.GetOperation(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    "account-789",
//	    "operation-abc",
//	    "transaction-xyz",
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to retrieve operation: %v", err)
//	}
//
//	// Process the operation details
//	fmt.Printf("Operation: %s\n", operation.ID)
//	fmt.Printf("Type: %s\n", operation.Type)
//	fmt.Printf("Account: %s\n", operation.AccountID)
//	fmt.Printf("Transaction: %s\n", operation.TransactionID)
//
//	// Calculate the decimal value of the amount
//	decimalValue := float64(operation.Amount) / math.Pow10(int(operation.Scale))
//	fmt.Printf("Amount: %.2f %s\n", decimalValue, operation.AssetCode)
//
//	// Check if this is a debit or credit operation
//	if operation.Type == models.OperationTypeDebit {
//	    fmt.Println("This is a debit operation (funds leaving the account)")
//	} else if operation.Type == models.OperationTypeCredit {
//	    fmt.Println("This is a credit operation (funds entering the account)")
//	}
func (e *operationsEntity) GetOperation(ctx context.Context, organizationID, ledgerID, accountID, operationID string) (*models.Operation, error) {
	const operation = "GetOperation"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	if operationID == "" {
		return nil, errors.NewMissingParameterError(operation, "operationID")
	}

	// Always use the account-based endpoint for GET operations
	url := e.buildURL(organizationID, ledgerID, accountID, operationID)

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var operationModel models.Operation
	if err := e.httpClient.sendRequest(req, &operationModel); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	normalizeOperation(&operationModel)

	return &operationModel, nil
}

// UpdateTransactionOperation updates an operation.
func (e *operationsEntity) UpdateTransactionOperation(ctx context.Context, organizationID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	const operation = "UpdateTransactionOperation"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if transactionID == "" {
		return nil, errors.NewMissingParameterError(operation, "transactionID")
	}

	if operationID == "" {
		return nil, errors.NewMissingParameterError(operation, "operationID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "update validation failed", err)
	}

	url := buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "transactions", transactionID, "operations", operationID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var operationModel models.Operation
	if err := e.httpClient.sendRequest(req, &operationModel); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		e.httpClient.emitBusinessError(ctx, businessEventOperationUpdated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldTransactionID: transactionID, businessFieldOperationID: operationID}, err)

		return nil, err
	}

	normalizeOperation(&operationModel)
	e.httpClient.emitBusinessEvent(ctx, businessEventOperationUpdated, map[string]any{businessFieldOperation: operation, businessFieldOrganizationID: organizationID, businessFieldLedgerID: ledgerID, businessFieldTransactionID: transactionID, businessFieldOperationID: operationModel.ID, businessFieldStatus: operationModel.Status.Code})

	return &operationModel, nil
}

// buildURL builds the URL for operations API calls using the account-based endpoint.
func (e *operationsEntity) buildURL(organizationID, ledgerID, accountID, operationID string) string {
	if operationID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "accounts", accountID, "operations")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "accounts", accountID, "operations", operationID)
}

func normalizeOperationListResponse(response *models.ListResponse[models.Operation]) {
	if response.Items == nil {
		response.Items = []models.Operation{}
	}

	for i := range response.Items {
		normalizeOperation(&response.Items[i])
	}
}

// normalizeOperation reserves a place for any future server-shape
// normalization that the SDK should apply to operations on the way out.
//
// Historically this helper rewrote a server-returned nil Metadata into an
// empty map. We stopped doing that — the wire shape is the wire shape, and
// callers who want a guaranteed-non-nil view should use the
// (*Operation).MetadataOrEmpty accessor.
func normalizeOperation(_ *models.Operation) {
}
