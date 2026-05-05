package entities

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// OperationsService defines the interface for operation-related operations.
// It provides methods to list, retrieve, and update operations
// associated with accounts and transactions.
type OperationsService interface {
	// ListOperations retrieves a cursor-paginated list of operations for a specific account.
	//
	// Operations represent the individual accounting entries (debits and credits) that make up
	// transactions in the ledger. This method allows you to retrieve all operations for a
	// specific account, with optional filtering and cursor pagination controls.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger containing the account. Must be a valid ledger ID.
	//   - accountID: The ID of the account to retrieve operations for. Must be a valid account ID.
	//   - opts: Optional cursor pagination and filtering options. ListOptions.Cursor and
	//     ListOptions.Limit are serialized by cursorListQueryParams; Page and Offset are
	//     ignored for this cursor-only endpoint and are not sent on the wire.
	//
	// Returns:
	//   - *models.ListResponse[models.Operation]: A cursor-paginated list of operations.
	//     Use response pagination cursor fields, not Total/TotalPages, for traversal.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization, ledger, or account ID)
	//     - Network or server errors
	//
	// Example - Basic usage:
	//
	//	// List operations with default cursor pagination
	//	operations, err := operationsService.ListOperations(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    nil, // Use default cursor pagination
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list operations: %v", err)
	//	}

	//
	//	// Process the operations
	//	fmt.Printf("Retrieved %d operations; next cursor: %s\n",
	//	    len(operations.Items), operations.Pagination.NextCursor)
	//
	//	for _, op := range operations.Items {
	//	    fmt.Printf("Operation: %s, Type: %s, Amount: %d %s\n",
	//	        op.ID, op.Type, op.Amount, op.AssetCode)
	//	}

	//
	// Example - With pagination and filtering:
	//
	//	// Create cursor pagination options with filtering
	//	opts := &models.ListOptions{
	//	    Limit: 10,
	//	    Cursor: "next-cursor-from-previous-response",
	//	    Filters: map[string]string{
	//	        "type": "debit", // Only show debit operations
	//	        "assetCode": "USD", // Only show USD operations
	//	    },
	//	    OrderDirection: "desc",
	//	}

	//
	//	// List operations with pagination and filtering
	//	operations, err := operationsService.ListOperations(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    opts,
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list operations: %v", err)
	//	}

	//
	//	// Process the operations
	//	fmt.Printf("Retrieved %d debit operations in USD\n", len(operations.Items))
	ListOperations(ctx context.Context, orgID, ledgerID, accountID string, opts *models.ListOptions) (*models.ListResponse[models.Operation], error)

	// GetOperation retrieves a specific operation by its ID.
	//
	// Operations represent the individual accounting entries (debits and credits) that make up
	// transactions in the ledger. This method retrieves a single operation by its unique identifier.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
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

	GetOperation(ctx context.Context, orgID, ledgerID, accountID, operationID string, transactionID ...string) (*models.Operation, error)

	// UpdateTransactionOperation updates an existing operation by transaction scope.
	// The orgID, ledgerID, and transactionID parameters specify which organization, ledger,
	// and transaction the operation belongs to.
	// The operationID parameter is the unique identifier of the operation to update.
	// The input parameter contains the operation details to update.
	// Returns the updated operation, or an error if the operation fails.
	UpdateTransactionOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input any) (*models.Operation, error)

	// UpdateOperation is retained for source compatibility with the former account-scoped
	// signature. Midaz now updates operations through the transaction-scoped endpoint.
	// Deprecated: use UpdateTransactionOperation with a transactionID.
	UpdateOperation(ctx context.Context, orgID, ledgerID, accountID, operationID string, input any) (*models.Operation, error)
}

// operationsEntity implements the OperationsService interface.
// It handles the communication with the Midaz API for operation-related operations.
type operationsEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

func (e *operationsEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.SetTenantID(tenantID)
}

// newOperationsEntity wires the OperationsService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.Operations.
func newOperationsEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	return &operationsEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
	}
}

// ListOperations lists operations for an account with optional filters.
func (e *operationsEntity) ListOperations(ctx context.Context, orgID, ledgerID, accountID string, opts *models.ListOptions) (*models.ListResponse[models.Operation], error) {
	const operation = "ListOperations"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if accountID == "" {
		return nil, errors.NewMissingParameterError(operation, "accountID")
	}

	url := e.buildURL(orgID, ledgerID, accountID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	// Add query parameters if provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range cursorListQueryParams(opts) {
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

// GetOperation retrieves an operation by its ID.
//
// Operations represent the individual accounting entries (debits and credits) that make up
// transactions in the ledger. This method retrieves a single operation by its unique identifier.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
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
func (e *operationsEntity) GetOperation(ctx context.Context, orgID, ledgerID, accountID, operationID string, _ ...string) (*models.Operation, error) {
	const operation = "GetOperation"

	if orgID == "" {
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
	url := e.buildURL(orgID, ledgerID, accountID, operationID)

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

// UpdateOperation is the deprecated account-scoped update method. It fails
// LOUDLY without performing any network call.
//
// The previous behavior — silently issuing a GET to discover transactionID
// and then re-routing the PATCH to the transaction-scoped endpoint —
// hid a contract change behind two RPCs and made consumers believe the
// account-scoped path still worked. We now refuse the call up front so
// the deprecation surface is immediate and unambiguous.
//
// Deprecated: use UpdateTransactionOperation(ctx, orgID, ledgerID,
// transactionID, operationID, input).
func (*operationsEntity) UpdateOperation(_ context.Context, _, _, _, _ string, _ any) (*models.Operation, error) {
	return nil, errors.NewValidationError(
		"UpdateOperation",
		"the account-scoped operation update path has been removed",
		stderrors.New("use UpdateTransactionOperation(ctx, orgID, ledgerID, transactionID, operationID, input) — the SDK no longer auto-resolves transactionID via a hidden GET"),
	)
}

// UpdateTransactionOperation updates an operation.
func (e *operationsEntity) UpdateTransactionOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input any) (*models.Operation, error) {
	const operation = "UpdateTransactionOperation"

	if orgID == "" {
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

	if err := validateUpdatePayload(operation, input, "*models.UpdateOperationInput"); err != nil {
		return nil, err
	}

	url := buildLedgerScopedURL(e.baseURLs["transaction"], orgID, ledgerID, "transactions", transactionID, "operations", operationID)

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
		e.httpClient.emitBusinessError(ctx, businessEventOperationUpdated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID, "operationId": operationID}, err)

		return nil, err
	}

	normalizeOperation(&operationModel)
	e.httpClient.emitBusinessEvent(ctx, businessEventOperationUpdated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID, "operationId": operationModel.ID, "status": operationModel.Status.Code})

	return &operationModel, nil
}

// buildURL builds the URL for operations API calls using the account-based endpoint.
func (e *operationsEntity) buildURL(orgID, ledgerID, accountID, operationID string) string {
	if operationID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], orgID, ledgerID, "accounts", accountID, "operations")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], orgID, ledgerID, "accounts", accountID, "operations", operationID)
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
