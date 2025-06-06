package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

// TransactionsService defines the interface for transaction-related operations.
// It provides methods to create, read, update, and commit transactions
// within a ledger and organization. The implementation handles all the complexity
// of converting between SDK models and backend data formats, allowing SDK users
// to work with a clean, self-contained API.
type TransactionsService interface {
	// CreateTransaction creates a new transaction using the standard format.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// The input parameter contains the transaction details such as entries, metadata, and external ID.
	// Returns the created transaction, or an error if the operation fails.
	CreateTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error)

	// CreateTransactionWithDSL creates a new transaction using the DSL format.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// The input parameter contains the transaction DSL script and optional metadata.
	// Returns the created transaction, or an error if the operation fails.
	CreateTransactionWithDSL(ctx context.Context, orgID, ledgerID string, input *models.TransactionDSLInput) (*models.Transaction, error)

	// CreateTransactionWithDSLFile creates a new transaction using a DSL file.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// The dslContent parameter contains the raw DSL file content as bytes.
	// Returns the created transaction, or an error if the operation fails.
	CreateTransactionWithDSLFile(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error)

	// GetTransaction retrieves a specific transaction by its ID.
	// The orgID and ledgerID parameters specify which organization and ledger the transaction belongs to.
	// The transactionID parameter is the unique identifier of the transaction to retrieve.
	// Returns the transaction if found, or an error if the operation fails or the transaction doesn't exist.
	GetTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error)

	// ListTransactions retrieves a paginated list of transactions for a ledger with optional filters.
	// The orgID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the transactions and pagination information, or an error if the operation fails.
	ListTransactions(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Transaction], error)

	// UpdateTransaction updates an existing transaction.
	// The orgID and ledgerID parameters specify which organization and ledger the transaction belongs to.
	// The transactionID parameter is the unique identifier of the transaction to update.
	// The input parameter contains the transaction details to update, which can be of various types.
	// Returns the updated transaction, or an error if the operation fails.
	UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input any) (*models.Transaction, error)
}

// TransactionsEntity implements the TransactionsService interface.
// It handles the communication with the Midaz API for transaction-related operations.
type TransactionsEntity struct {
	HTTPClient *HTTPClient
	BaseURLs   map[string]string
}

// NewTransactionsEntity creates a new transactions entity.
//
// Parameters:
//   - client: The HTTP client used for API requests. Can be configured with custom timeouts
//     and transport options. If nil, a default client will be used.
//   - authToken: The authentication token for API authorization. Must be a valid JWT token
//     issued by the Midaz authentication service.
//   - baseURLs: Map of service names to base URLs. Must include a "transaction" key with
//     the URL of the transaction service (e.g., "https://api.midaz.io/v1").
//
// Returns:
//   - TransactionsService: An implementation of the TransactionsService interface that provides
//     methods for creating, retrieving, and managing transactions.
func NewTransactionsEntity(client *http.Client, authToken string, baseURLs map[string]string) TransactionsService {
	httpClient := NewHTTPClient(client, authToken, nil)

	return &TransactionsEntity{
		HTTPClient: httpClient,
		BaseURLs:   baseURLs,
	}
}

// CreateTransaction creates a new transaction using the standard format.
//
// This method creates a transaction using the standard format, which involves specifying
// a list of entries (debits and credits) that make up the transaction. Each entry specifies
// an account, direction (debit or credit), amount, and asset code.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger where the transaction will be created. Must be a valid ledger ID.
//   - input: The transaction details, including entries, description, metadata, and other properties.
//     The input must contain at least one entry, and the transaction must be balanced
//     (total debits must equal total credits for each asset).
//
// Returns:
//   - *models.Transaction: The created transaction if successful, containing the transaction ID,
//     status, entries, and other properties.
//   - error: An error if the operation fails. Possible errors include:
//   - Invalid input (missing required fields, unbalanced transaction)
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization or ledger ID)
//   - Network or server errors
func (e *TransactionsEntity) CreateTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	// Operation name for error context
	const operation = "CreateTransaction"

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate required parameters
	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	// Validate the transaction input
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "transaction validation failed", err)
	}

	// If using Send structure, we don't need to check for operations
	if input.Send == nil && len(input.Operations) == 0 {
		return nil, errors.NewValidationError(operation, "transaction must have at least one operation", nil)
	}

	// Convert the input to the format expected by the backend
	txMap := input.ToLibTransaction()

	// Create the request
	req, err := e.HTTPClient.NewRequest("POST", e.buildURLWithSuffix(orgID, ledgerID, "/json"), txMap)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	// Use a map to parse the response to handle potential schema differences
	var responseMap map[string]interface{}
	if err := e.HTTPClient.sendRequest(req, &responseMap); err != nil {
		return nil, err
	}

	// Convert the map response to a Transaction model
	transaction := &models.Transaction{
		ID:          getString(responseMap, "id"),
		Description: getString(responseMap, "description"),
		AssetCode:   getString(responseMap, "assetCode"),
	}

	// Handle amount fields
	if amount, ok := responseMap["amount"].(float64); ok {
		transaction.Amount = int64(amount)
	}
	if scale, ok := responseMap["amountScale"].(float64); ok {
		transaction.Scale = int64(scale)
	}

	// Handle organization and ledger IDs
	transaction.OrganizationID = getString(responseMap, "organizationId")
	transaction.LedgerID = getString(responseMap, "ledgerId")

	// Handle status
	statusMap, ok := responseMap["status"].(map[string]interface{})
	if ok {
		status := models.Status{
			Code: getString(statusMap, "code"),
		}

		// Handle description as a pointer
		if descStr := getString(statusMap, "description"); descStr != "" {
			desc := descStr // Create a copy to get a stable address
			status.Description = &desc
		}

		transaction.Status = status
	}

	// Handle timestamps
	if createdAt, err := time.Parse(time.RFC3339, getString(responseMap, "createdAt")); err == nil {
		transaction.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, getString(responseMap, "updatedAt")); err == nil {
		transaction.UpdatedAt = updatedAt
	}

	// Handle metadata
	if metadata, ok := responseMap["metadata"].(map[string]interface{}); ok {
		transaction.Metadata = metadata
	}

	return transaction, nil
}

// CreateTransactionWithDSL creates a new transaction using the DSL format.
//
// This method creates a transaction using the Domain-Specific Language (DSL) format,
// which provides a more flexible way to define complex transactions. The DSL format
// allows for more advanced transaction logic, including conditional operations and
// multi-step transactions.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger where the transaction will be created. Must be a valid ledger ID.
//   - input: The transaction DSL input, including the DSL script and optional metadata.
//     The DSL script must follow the Midaz transaction DSL syntax and must define a balanced
//     transaction (total debits must equal total credits for each asset).
//
// Returns:
//   - *models.Transaction: The created transaction if successful, containing the transaction ID,
//     status, operations, and other properties.
//   - error: An error if the operation fails. Possible errors include:
//   - Invalid DSL script (syntax errors, unbalanced transaction)
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization or ledger ID)
//   - Network or server errors
func (e *TransactionsEntity) CreateTransactionWithDSL(ctx context.Context, orgID, ledgerID string, input *models.TransactionDSLInput) (*models.Transaction, error) {
	// Operation name for error context
	const operation = "CreateTransactionWithDSL"

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate required parameters
	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	// Convert the DSL input to map format before sending to API
	transactionMap := map[string]any{
		"description": input.Description,
		"metadata":    input.Metadata,
	}

	// Use the correct endpoint for DSL transactions
	url := e.buildURLWithSuffix(orgID, ledgerID, "/dsl")

	body, err := json.Marshal(transactionMap)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to marshal request body: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.HTTPClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// CreateTransactionWithDSLFile creates a new transaction using a DSL file.
func (e *TransactionsEntity) CreateTransactionWithDSLFile(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organization ID cannot be empty")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, fmt.Errorf("ledger ID cannot be empty")
	}

	// Validate required parameters
	if len(dslContent) == 0 {
		return nil, fmt.Errorf("DSL content is required")
	}

	// Use the correct endpoint for DSL file transactions
	url := e.buildURLWithSuffix(orgID, ledgerID, "/dsl/file")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(dslContent))
	if err != nil {
		return nil, fmt.Errorf("internal error: %w", err)
	}

	var transaction models.Transaction
	if err := e.HTTPClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// GetTransaction retrieves a specific transaction by its ID.
//
// This method fetches a transaction by its unique identifier from the specified organization
// and ledger. It returns the complete transaction details, including all operations,
// metadata, and status information.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger where the transaction exists. Must be a valid ledger ID.
//   - transactionID: The unique identifier of the transaction to retrieve. Must be a valid
//     transaction ID previously returned from a transaction creation method.
//
// Returns:
//   - *models.Transaction: The retrieved transaction if found, containing the transaction ID,
//     status, operations, metadata, and other properties.
//   - error: An error if the operation fails. Possible errors include:
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization, ledger, or transaction ID)
//   - Network or server errors
func (e *TransactionsEntity) GetTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	// Operation name for error context
	const operation = "GetTransaction"

	// Validate required parameters
	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	// Validate required parameters
	if transactionID == "" {
		return nil, errors.NewMissingParameterError(operation, "transaction ID")
	}

	// Build the URL for the transaction
	url := e.buildURLWithSuffix(orgID, ledgerID, fmt.Sprintf("/%s", transactionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.HTTPClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// ListTransactions retrieves a paginated list of transactions for a ledger with optional filters.
//
// This method fetches a list of transactions from the specified organization and ledger,
// with support for pagination, sorting, and filtering. The results are returned as a paginated
// list that includes the total count and links to navigate between pages.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger to query. Must be a valid ledger ID.
//   - opts: Optional parameters for pagination, sorting, and filtering. Can be nil for default behavior.
//     Supported options include:
//   - Page: The page number to retrieve (starting from 1)
//   - PageSize: The number of items per page (default is 20)
//   - Sort: The field to sort by (e.g., "created_at")
//   - Order: The sort order ("asc" or "desc")
//   - Filter: Additional filtering criteria as key-value pairs
//
// Returns:
//   - *models.ListResponse[models.Transaction]: A paginated response containing:
//   - Items: The list of transactions for the current page
//   - Pagination: Metadata about the pagination, including total count and links
//   - error: An error if the operation fails. Possible errors include:
//   - Authentication failure (invalid auth token)
//   - Authorization failure (insufficient permissions)
//   - Resource not found (invalid organization or ledger ID)
//   - Invalid parameters (negative page number, etc.)
//   - Network or server errors
func (e *TransactionsEntity) ListTransactions(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Transaction], error) {
	// Operation name for error context
	const operation = "ListTransactions"

	// Validate required parameters
	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	// Build the URL for the transactions
	url := e.buildURLWithSuffix(orgID, ledgerID, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	// Add query parameters if options are provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range opts.ToQueryParams() {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Transaction]
	if err := e.HTTPClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateTransaction updates an existing transaction.
func (e *TransactionsEntity) UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input any) (*models.Transaction, error) {
	// Operation name for error context
	const operation = "UpdateTransaction"

	// Validate required parameters
	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, errors.NewMissingParameterError(operation, "transaction ID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Build the URL for the transaction
	url := e.buildURLWithSuffix(orgID, ledgerID, fmt.Sprintf("/%s", transactionID))

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to marshal request body: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.HTTPClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// buildURLWithSuffix builds the URL for transactions API calls with the specified suffix.
func (e *TransactionsEntity) buildURLWithSuffix(orgID, ledgerID, suffix string) string {
	base := e.BaseURLs["transaction"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/transactions%s", base, orgID, ledgerID, suffix)
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
