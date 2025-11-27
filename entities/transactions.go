package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
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

	// RevertTransaction reverts a committed transaction.
	// The orgID and ledgerID parameters specify which organization and ledger the transaction belongs to.
	// The transactionID parameter is the unique identifier of the transaction to revert.
	// Returns the reverted transaction, or an error if the operation fails.
	RevertTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error)

	// CommitTransaction commits a pending transaction.
	// The orgID and ledgerID parameters specify which organization and ledger the transaction belongs to.
	// The transactionID parameter is the unique identifier of the transaction to commit.
	// Returns the committed transaction, or an error if the operation fails.
	CommitTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error)

	// CancelTransaction cancels a pending transaction.
	// The orgID and ledgerID parameters specify which organization and ledger the transaction belongs to.
	// The transactionID parameter is the unique identifier of the transaction to cancel.
	// Returns an error if the operation fails.
	CancelTransaction(ctx context.Context, orgID, ledgerID, transactionID string) error

	// CreateInflowTransaction creates an inflow transaction (funds entering the system).
	// Inflow transactions have no source - they represent deposits or funding operations.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// Returns the created transaction, or an error if the operation fails.
	CreateInflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error)

	// CreateOutflowTransaction creates an outflow transaction (funds leaving the system).
	// Outflow transactions have no destination - they represent withdrawals or payout operations.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// Returns the created transaction, or an error if the operation fails.
	CreateOutflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error)

	// CreateAnnotationTransaction creates an annotation transaction (no balance changes).
	// Annotation transactions are used for adding metadata/notes to the ledger without affecting balances.
	// The orgID and ledgerID parameters specify which organization and ledger to create the transaction in.
	// Returns the created transaction, or an error if the operation fails.
	CreateAnnotationTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error)
}

// transactionsEntity implements the TransactionsService interface.
// It handles the communication with the Midaz API for transaction-related operations.
type transactionsEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
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

	// Check if we're using the debug flag from the environment
	if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue {
		httpClient.debug = true
	}

	return &transactionsEntity{
		httpClient: httpClient,
		baseURLs:   baseURLs,
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
func (e *transactionsEntity) CreateTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	const operation = "CreateTransaction"

	// Validate input parameters
	if err := e.validateCreateTransactionInput(operation, orgID, ledgerID, input); err != nil {
		return nil, err
	}

	// Send request to API
	responseMap, err := e.sendCreateTransactionRequest(ctx, orgID, ledgerID, input)
	if err != nil {
		return nil, err
	}

	// Convert response to transaction model
	return e.parseTransactionResponse(responseMap), nil
}

// validateCreateTransactionInput validates all input parameters for CreateTransaction
func (e *transactionsEntity) validateCreateTransactionInput(operation, orgID, ledgerID string, input *models.CreateTransactionInput) error {
	if input == nil {
		return errors.NewMissingParameterError(operation, "input")
	}

	if orgID == "" {
		return errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledger ID")
	}

	if err := input.Validate(); err != nil {
		return errors.NewValidationError(operation, "transaction validation failed", err)
	}

	if input.Send == nil && len(input.Operations) == 0 {
		return errors.NewValidationError(operation, "transaction must have at least one operation", nil)
	}

	return nil
}

// sendCreateTransactionRequest sends the transaction creation request
func (e *transactionsEntity) sendCreateTransactionRequest(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (map[string]any, error) {
	txMap := input.ToLibTransaction()

	var responseMap map[string]any
	if err := e.httpClient.doRequest(ctx, http.MethodPost, e.buildURL(orgID, ledgerID, "/json"), nil, txMap, &responseMap); err != nil {
		return nil, err
	}

	return responseMap, nil
}

// parseTransactionResponse converts response map to Transaction model
func (e *transactionsEntity) parseTransactionResponse(responseMap map[string]any) *models.Transaction {
	transaction := &models.Transaction{
		ID:          getString(responseMap, "id"),
		Description: getString(responseMap, "description"),
		AssetCode:   getString(responseMap, "assetCode"),
	}

	e.setTransactionAmount(transaction, responseMap)
	e.setTransactionIDs(transaction, responseMap)
	e.setTransactionArrays(transaction, responseMap)
	e.setTransactionStatus(transaction, responseMap)
	e.setTransactionTimestamps(transaction, responseMap)
	e.setTransactionMetadata(transaction, responseMap)

	return transaction
}

// setTransactionAmount sets the amount field from various response formats
func (e *transactionsEntity) setTransactionAmount(transaction *models.Transaction, responseMap map[string]any) {
	if amount, ok := responseMap["amount"].(string); ok {
		transaction.Amount = amount
	} else if amount, ok := responseMap["amount"].(float64); ok {
		transaction.Amount = fmt.Sprintf("%.2f", amount)
	}
}

// setTransactionIDs sets organization and ledger IDs and other fields
func (e *transactionsEntity) setTransactionIDs(transaction *models.Transaction, responseMap map[string]any) {
	transaction.OrganizationID = getString(responseMap, "organizationId")
	transaction.LedgerID = getString(responseMap, "ledgerId")
	transaction.Route = getString(responseMap, "route")
	transaction.ChartOfAccountsGroupName = getString(responseMap, "chartOfAccountsGroupName")

	if pending, ok := responseMap["pending"].(bool); ok {
		transaction.Pending = pending
	}
}

// setTransactionArrays sets source and destination arrays
func (e *transactionsEntity) setTransactionArrays(transaction *models.Transaction, responseMap map[string]any) {
	transaction.Source = e.parseStringArray(responseMap, "source")
	transaction.Destination = e.parseStringArray(responseMap, "destination")
}

// parseStringArray converts any array to string array
func (e *transactionsEntity) parseStringArray(responseMap map[string]any, key string) []string {
	if array, ok := responseMap[key].([]any); ok {
		result := make([]string, len(array))

		for i, v := range array {
			if s, ok := v.(string); ok {
				result[i] = s
			}
		}

		return result
	}

	return nil
}

// setTransactionStatus sets the status from response map
func (e *transactionsEntity) setTransactionStatus(transaction *models.Transaction, responseMap map[string]any) {
	statusMap, ok := responseMap["status"].(map[string]any)
	if !ok {
		return
	}

	status := models.Status{
		Code: getString(statusMap, "code"),
	}

	if descStr := getString(statusMap, "description"); descStr != "" {
		desc := descStr
		status.Description = &desc
	}

	transaction.Status = status
}

// setTransactionTimestamps sets created and updated timestamps
func (e *transactionsEntity) setTransactionTimestamps(transaction *models.Transaction, responseMap map[string]any) {
	if createdAt, err := time.Parse(time.RFC3339, getString(responseMap, "createdAt")); err == nil {
		transaction.CreatedAt = createdAt
	}

	if updatedAt, err := time.Parse(time.RFC3339, getString(responseMap, "updatedAt")); err == nil {
		transaction.UpdatedAt = updatedAt
	}
}

// setTransactionMetadata sets the metadata from response map
func (e *transactionsEntity) setTransactionMetadata(transaction *models.Transaction, responseMap map[string]any) {
	if metadata, ok := responseMap["metadata"].(map[string]any); ok {
		transaction.Metadata = metadata
	}
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
func (e *transactionsEntity) CreateTransactionWithDSL(ctx context.Context, orgID, ledgerID string, input *models.TransactionDSLInput) (*models.Transaction, error) {
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
	// Use the strongly-typed converter to include send/source/distribute, share, rate, etc.
	transactionMap := input.ToTransactionMap()

	// Use the correct endpoint for DSL transactions
	url := e.buildURL(orgID, ledgerID, "/dsl")

	body, err := json.Marshal(transactionMap)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to marshal request body: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// CreateTransactionWithDSLFile creates a new transaction using a DSL file.
func (e *transactionsEntity) CreateTransactionWithDSLFile(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organization ID cannot be empty")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, fmt.Errorf("ledger ID cannot be empty")
	}

	// Validate DSL payload before sending
	if err := validateDSLContent(dslContent); err != nil {
		return nil, err
	}

	// Use DSL endpoint with raw body payload
	url := e.buildURL(orgID, ledgerID, "/dsl")

	headers := map[string]string{"Content-Type": "text/plain"}

	var transaction models.Transaction
	if err := e.httpClient.doRawRequest(ctx, http.MethodPost, url, headers, dslContent, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

func validateDSLContent(dslContent []byte) error {
	if len(bytes.TrimSpace(dslContent)) == 0 {
		return fmt.Errorf("DSL content is required")
	}

	if !utf8.Valid(dslContent) {
		return fmt.Errorf("DSL content must be valid UTF-8")
	}

	content := strings.ToLower(string(dslContent))
	if !strings.Contains(content, "send") || !strings.Contains(content, "distribute") {
		return fmt.Errorf("DSL content missing required sections")
	}

	return nil
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
func (e *transactionsEntity) GetTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
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
	url := e.buildURL(orgID, ledgerID, fmt.Sprintf("/%s", transactionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
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
func (e *transactionsEntity) ListTransactions(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Transaction], error) {
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
	url := e.buildURL(orgID, ledgerID, "")

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
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateTransaction updates an existing transaction.
func (e *transactionsEntity) UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input any) (*models.Transaction, error) {
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
	url := e.buildURL(orgID, ledgerID, fmt.Sprintf("/%s", transactionID))

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to marshal request body: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// RevertTransaction reverts a committed transaction.
func (e *transactionsEntity) RevertTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "RevertTransaction"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, errors.NewMissingParameterError(operation, "transaction ID")
	}

	url := e.buildURL(orgID, ledgerID, fmt.Sprintf("/%s/revert", transactionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// CommitTransaction commits a pending transaction.
func (e *transactionsEntity) CommitTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "CommitTransaction"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, errors.NewMissingParameterError(operation, "transaction ID")
	}

	url := e.buildURL(orgID, ledgerID, fmt.Sprintf("/%s/commit", transactionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	return &transaction, nil
}

// CancelTransaction cancels a pending transaction.
func (e *transactionsEntity) CancelTransaction(ctx context.Context, orgID, ledgerID, transactionID string) error {
	const operation = "CancelTransaction"

	if orgID == "" {
		return errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return errors.NewMissingParameterError(operation, "transaction ID")
	}

	url := e.buildURL(orgID, ledgerID, fmt.Sprintf("/%s/cancel", transactionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	if err := e.httpClient.sendRequest(req, nil); err != nil {
		return err
	}

	return nil
}

// CreateInflowTransaction creates an inflow transaction (funds entering the system).
func (e *transactionsEntity) CreateInflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	const operation = "CreateInflowTransaction"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid input", err)
	}

	url := e.buildURL(orgID, ledgerID, "/inflow")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	var result map[string]any
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return e.parseTransactionResponse(result), nil
}

// CreateOutflowTransaction creates an outflow transaction (funds leaving the system).
func (e *transactionsEntity) CreateOutflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error) {
	const operation = "CreateOutflowTransaction"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid input", err)
	}

	url := e.buildURL(orgID, ledgerID, "/outflow")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	var result map[string]any
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return e.parseTransactionResponse(result), nil
}

// CreateAnnotationTransaction creates an annotation transaction (no balance changes).
func (e *transactionsEntity) CreateAnnotationTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error) {
	const operation = "CreateAnnotationTransaction"

	if orgID == "" {
		return nil, errors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid input", err)
	}

	url := e.buildURL(orgID, ledgerID, "/annotation")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	var result map[string]any
	if err := e.httpClient.sendRequest(req, &result); err != nil {
		return nil, err
	}

	return e.parseTransactionResponse(result), nil
}

// buildURL builds the URL for transactions API calls with the specified suffix.
func (e *transactionsEntity) buildURL(orgID, ledgerID, suffix string) string {
	base := e.baseURLs["transaction"]
	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/transactions%s", base, orgID, ledgerID, suffix)
}

// getString safely extracts a string value from a map
func getString(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}

	return ""
}
