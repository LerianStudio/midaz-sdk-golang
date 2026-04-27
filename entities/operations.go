package entities

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// OperationsService defines the interface for operation-related operations.
// It provides methods to list, retrieve, and update operations
// associated with accounts and transactions.
type OperationsService interface {
	// ListOperations retrieves a paginated list of operations for a specific account.
	//
	// Operations represent the individual accounting entries (debits and credits) that make up
	// transactions in the ledger. This method allows you to retrieve all operations for a
	// specific account, with optional filtering and pagination controls.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger containing the account. Must be a valid ledger ID.
	//   - accountID: The ID of the account to retrieve operations for. Must be a valid account ID.
	//   - opts: Optional pagination and filtering options:
	//     - Page: The page number to retrieve (1-based indexing)
	//     - Limit: The maximum number of items per page
	//     - Filter: Criteria to filter operations by (e.g., by transaction ID or asset code)
	//     - Sort: Sorting options for the results
	//     If nil, default pagination settings will be used.
	//
	// Returns:
	//   - *models.ListResponse[models.Operation]: A paginated list of operations, including:
	//     - Items: The array of operation objects for the current page
	//     - Page: The current page number
	//     - Limit: The maximum number of items per page
	//     - Total: The total number of operations matching the filter criteria
	//   - error: An error if the operation fails. Possible errors include:
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization, ledger, or account ID)
	//     - Network or server errors
	//
	// Example - Basic usage:
	//
	//	// List operations with default pagination
	//	operations, err := operationsService.ListOperations(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "account-789",
	//	    nil, // Use default pagination
	//	)
	//
	//	if err != nil {
	//	    log.Fatalf("Failed to list operations: %v", err)
	//	}

	//
	//	// Process the operations
	//	fmt.Printf("Retrieved %d operations (page %d of %d)\n",
	//	    len(operations.Items), operations.Page, operations.TotalPages)
	//
	//	for _, op := range operations.Items {
	//	    fmt.Printf("Operation: %s, Type: %s, Amount: %d %s\n",
	//	        op.ID, op.Type, op.Amount, op.AssetCode)
	//	}

	//
	// Example - With pagination and filtering:
	//
	//	// Create pagination options with filtering
	//	opts := &models.ListOptions{
	//	    Page: 1,
	//	    Limit: 10,
	//	    Filter: map[string]any{
	//	        "type": "debit", // Only show debit operations
	//	        "assetCode": "USD", // Only show USD operations
	//	    },
	//	    Sort: []string{"createdAt:desc"}, // Sort by creation time (newest first)
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

// NewOperationsEntity creates a new operations entity.
//
// Parameters:
//   - client: The HTTP client used for API requests. Can be configured with custom timeouts
//     and transport options. If nil, a default client will be used.
//   - authToken: The authentication token for API authorization. Must be a valid JWT token
//     issued by the Midaz authentication service.
//   - baseURLs: Map of service names to base URLs. Must include an "onboarding" key with
//     the URL of the onboarding service (e.g., "https://api.midaz.io/v1").
//
// Returns:
//   - OperationsService: An implementation of the OperationsService interface that provides
//     methods for retrieving, updating, and managing transaction operations.
//
// Example:
//
//	// Create an operations entity with default HTTP client
//	operationsEntity := entities.NewOperationsEntity(
//	    &http.Client{Timeout: 30 * time.Second},
//	    "your-auth-token",
//	    map[string]string{"onboarding": "https://api.midaz.io/v1"},
//	)
//
//	// Use the entity to retrieve operations for an account
//	operations, err := operationsEntity.ListOperations(
//	    context.Background(),
//	    "org-123",
//	    "ledger-456",
//	    "account-789",
//	    nil, // No pagination options
//	)
//
//	if err != nil {
//	    log.Fatalf("Failed to retrieve operations: %v", err)
//	}
//
//	fmt.Printf("Retrieved %d operations\n", len(operations.Items))
func NewOperationsEntity(client *http.Client, authToken string, baseURLs map[string]string) OperationsService {
	// Create a new HTTP client with the shared implementation
	httpClient := NewHTTPClient(client, authToken, nil)

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &operationsEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	// Add query parameters if provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range opts.ToQueryParams() {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Operation]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var operationModel models.Operation
	if err := e.httpClient.sendRequest(req, &operationModel); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &operationModel, nil
}

// UpdateOperation rejects the former account-scoped update path so callers do not
// silently route an accountID into Midaz's transaction-scoped endpoint.
// Deprecated: use UpdateTransactionOperation with a transactionID.
func (*operationsEntity) UpdateOperation(_ context.Context, _, _, _, _ string, _ any) (*models.Operation, error) {
	return nil, errors.NewValidationError("UpdateOperation", "operation updates are transaction-scoped", stderrors.New("use UpdateTransactionOperation(ctx, orgID, ledgerID, transactionID, operationID, input)"))
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

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	url := fmt.Sprintf("%s/organizations/%s/ledgers/%s/transactions/%s/operations/%s", e.baseURLs["transaction"], pathSegment(orgID), pathSegment(ledgerID), pathSegment(transactionID), pathSegment(operationID))

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var operationModel models.Operation
	if err := e.httpClient.sendRequest(req, &operationModel); err != nil {
		// HTTPClient.DoRequest already returns proper error types
		return nil, err
	}

	return &operationModel, nil
}

// buildURL builds the URL for operations API calls using the account-based endpoint.
func (e *operationsEntity) buildURL(orgID, ledgerID, accountID, operationID string) string {
	base := e.baseURLs["transaction"]

	// Ensure the base URL doesn't end with a trailing slash
	base = strings.TrimSuffix(base, "/")

	if operationID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/operations", base, pathSegment(orgID), pathSegment(ledgerID), pathSegment(accountID))
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/accounts/%s/operations/%s", base, pathSegment(orgID), pathSegment(ledgerID), pathSegment(accountID), pathSegment(operationID))
}
