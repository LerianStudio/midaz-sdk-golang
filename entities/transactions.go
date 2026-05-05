package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
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

	// GetTransactionsMetricsCount retrieves the count of transactions that match the supplied filters.
	GetTransactionsMetricsCount(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.MetricsCount, error)

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

	// CancelTransactionWithResponse cancels a pending transaction and returns the cancelled transaction.
	CancelTransactionWithResponse(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error)

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

func (e *transactionsEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.SetTenantID(tenantID)
}

// newTransactionsEntity creates a new transactions entity.
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
func newTransactionsEntity(client *http.Client, authToken string, baseURLs map[string]string) TransactionsService {
	httpClient := NewHTTPClient(client, authToken, nil)

	return &transactionsEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
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
		e.httpClient.emitBusinessError(ctx, businessEventTransactionCreated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID}, err)

		return nil, err
	}

	// Convert response to transaction model
	transaction := e.parseTransactionResponse(responseMap)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionCreated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return transaction, nil
}

// validateCreateTransactionInput validates all input parameters for CreateTransaction
func (*transactionsEntity) validateCreateTransactionInput(operation, orgID, ledgerID string, input *models.CreateTransactionInput) error {
	if input == nil {
		return sdkerrors.NewMissingParameterError(operation, "input")
	}

	if orgID == "" {
		return sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if err := input.Validate(); err != nil {
		return sdkerrors.NewValidationError(operation, "transaction validation failed", err)
	}

	return nil
}

// sendCreateTransactionRequest sends the transaction creation request
func (e *transactionsEntity) sendCreateTransactionRequest(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (map[string]any, error) {
	txMap := input.ToLibTransaction()

	var responseMap map[string]any

	headers := map[string]string{}
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		headers["X-Idempotency"] = key
		headers[internalCallerIdempotencyHeader] = BoolTrue
	}

	if err := e.httpClient.doRequest(ctx, http.MethodPost, e.buildURL(orgID, ledgerID, "/json"), headers, txMap, &responseMap); err != nil {
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
	e.setTransactionOperations(transaction, responseMap)
	e.normalizeTransaction(transaction)

	return transaction
}

// setTransactionAmount sets the amount field from various response formats
func (*transactionsEntity) setTransactionAmount(transaction *models.Transaction, responseMap map[string]any) {
	if amount, ok := responseMap["amount"]; ok {
		transaction.Amount = models.DecimalStringFromAny(amount)
	}
}

// setTransactionIDs sets organization and ledger IDs and other fields
func (*transactionsEntity) setTransactionIDs(transaction *models.Transaction, responseMap map[string]any) {
	transaction.OrganizationID = getString(responseMap, "organizationId")
	transaction.LedgerID = getString(responseMap, "ledgerId")
	transaction.Route = getString(responseMap, "route")
	transaction.RouteID = getString(responseMap, "routeId")
	transaction.ParentTransactionID = getString(responseMap, "parentTransactionId")
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
func (*transactionsEntity) parseStringArray(responseMap map[string]any, key string) []string {
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
func (*transactionsEntity) setTransactionStatus(transaction *models.Transaction, responseMap map[string]any) {
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
func (*transactionsEntity) setTransactionTimestamps(transaction *models.Transaction, responseMap map[string]any) {
	if createdAt, err := time.Parse(time.RFC3339, getString(responseMap, "createdAt")); err == nil {
		transaction.CreatedAt = createdAt
	}

	if updatedAt, err := time.Parse(time.RFC3339, getString(responseMap, "updatedAt")); err == nil {
		transaction.UpdatedAt = updatedAt
	}

	if deletedAt := getString(responseMap, "deletedAt"); deletedAt != "" {
		if parsedDeletedAt, err := time.Parse(time.RFC3339, deletedAt); err == nil {
			transaction.DeletedAt = &parsedDeletedAt
		}
	}
}

// setTransactionMetadata sets the metadata from response map
func (*transactionsEntity) setTransactionMetadata(transaction *models.Transaction, responseMap map[string]any) {
	if metadata, ok := responseMap["metadata"].(map[string]any); ok {
		transaction.Metadata = metadata
	}
}

func (*transactionsEntity) normalizeTransaction(transaction *models.Transaction) {
	if transaction == nil {
		return
	}

	if transaction.Source == nil {
		transaction.Source = []string{}
	}

	if transaction.Destination == nil {
		transaction.Destination = []string{}
	}

	if transaction.Operations == nil {
		transaction.Operations = []models.Operation{}
	}

	if transaction.Metadata == nil {
		transaction.Metadata = map[string]any{}
	}

	for i := range transaction.Operations {
		if transaction.Operations[i].Metadata == nil {
			transaction.Operations[i].Metadata = map[string]any{}
		}
	}
}

func (e *transactionsEntity) normalizeTransactionListResponse(response *models.ListResponse[models.Transaction]) {
	if response == nil {
		return
	}

	if response.Items == nil {
		response.Items = []models.Transaction{}
	}

	for i := range response.Items {
		e.normalizeTransaction(&response.Items[i])
	}
}

func (e *transactionsEntity) setTransactionOperations(transaction *models.Transaction, responseMap map[string]any) {
	operations, ok := responseMap["operations"].([]any)
	if !ok {
		return
	}

	parsedOperations := make([]models.Operation, 0, len(operations))
	for _, item := range operations {
		operationMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		parsedOperations = append(parsedOperations, e.parseOperation(operationMap))
	}

	transaction.Operations = parsedOperations
}

func (*transactionsEntity) parseOperation(operationMap map[string]any) models.Operation {
	operation := models.Operation{
		ID:               getString(operationMap, "id"),
		TransactionID:    getString(operationMap, "transactionId"),
		Description:      getString(operationMap, "description"),
		Type:             getString(operationMap, "type"),
		AssetCode:        getString(operationMap, "assetCode"),
		ChartOfAccounts:  getString(operationMap, "chartOfAccounts"),
		Amount:           operationAmountFromMap(operationMap),
		Balance:          operationBalanceFromMap(operationMap, "balance"),
		BalanceAfter:     operationBalanceFromMap(operationMap, "balanceAfter"),
		Status:           operationStatusFromMap(operationMap),
		AccountID:        getString(operationMap, "accountId"),
		AccountAlias:     getString(operationMap, "accountAlias"),
		BalanceID:        getString(operationMap, "balanceId"),
		BalanceKey:       getString(operationMap, "balanceKey"),
		OrganizationID:   getString(operationMap, "organizationId"),
		LedgerID:         getString(operationMap, "ledgerId"),
		Route:            getString(operationMap, "route"),
		RouteID:          getString(operationMap, "routeId"),
		RouteCode:        getString(operationMap, "routeCode"),
		RouteDescription: getString(operationMap, "routeDescription"),
		Direction:        getString(operationMap, "direction"),
	}
	if metadata, ok := operationMap["metadata"].(map[string]any); ok {
		operation.Metadata = metadata
	} else {
		operation.Metadata = map[string]any{}
	}

	if balanceAffected, ok := boolFromAny(operationMap["balanceAffected"]); ok {
		operation.BalanceAffected = &balanceAffected
	}

	return operation
}

func operationAmountFromMap(operationMap map[string]any) models.Amount {
	amountMap, ok := operationMap["amount"].(map[string]any)
	if !ok {
		return models.Amount{}
	}

	return models.Amount{Value: decimalPtrFromAny(amountMap["value"])}
}

func operationBalanceFromMap(operationMap map[string]any, key string) models.OperationBalance {
	balanceMap, ok := operationMap[key].(map[string]any)
	if !ok {
		return models.OperationBalance{}
	}

	return models.OperationBalance{
		Available: decimalPtrFromAny(balanceMap["available"]),
		OnHold:    decimalPtrFromAny(balanceMap["onHold"]),
	}
}

func operationStatusFromMap(operationMap map[string]any) models.Status {
	statusMap, ok := operationMap["status"].(map[string]any)
	if !ok {
		return models.Status{}
	}

	return models.Status{Code: getString(statusMap, "code")}
}

func decimalPtrFromAny(value any) *decimal.Decimal {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		parsed, err := decimal.NewFromString(strings.TrimSpace(v))
		if err != nil {
			return nil
		}

		return &parsed
	case json.Number:
		parsed, err := decimal.NewFromString(v.String())
		if err != nil {
			return nil
		}

		return &parsed
	case int:
		parsed := decimal.NewFromInt(int64(v))
		return &parsed
	case int64:
		parsed := decimal.NewFromInt(v)
		return &parsed
	case float64:
		parsed := decimal.NewFromFloat(v)
		return &parsed
	default:
		return nil
	}
}

func boolFromAny(value any) (result bool, ok bool) {
	if v, ok := value.(bool); ok {
		return v, true
	}

	return false, false
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
		return nil, sdkerrors.NewMissingParameterError(operation, "input")
	}

	// Validate required parameters
	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	dslContent, err := input.RenderDSL()
	if err != nil {
		return nil, sdkerrors.NewValidationError(operation, "failed to render DSL input", err)
	}

	transaction, err := e.CreateTransactionWithDSLFile(ctx, orgID, ledgerID, dslContent)
	if err != nil {
		return nil, err
	}

	return transaction, nil
}

// CreateTransactionWithDSLFile creates a new transaction using a DSL file.
func (e *transactionsEntity) CreateTransactionWithDSLFile(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
	if orgID == "" {
		return nil, errors.New("organization ID cannot be empty")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, errors.New("ledger ID cannot be empty")
	}

	// Validate DSL payload before sending
	if err := validateDSLContent(dslContent); err != nil {
		return nil, err
	}

	if int64(len(dslContent)) > maxHTTPRequestBodyBytes {
		return nil, sdkerrors.NewValidationError("CreateTransactionWithDSLFile", "DSL content exceeds maximum request size", nil)
	}

	// Use DSL endpoint with raw body payload
	endpointURL := e.buildURL(orgID, ledgerID, "/dsl")

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("transaction", "transaction.dsl")
	if err != nil {
		return nil, sdkerrors.NewInternalError("CreateTransactionWithDSLFile", fmt.Errorf("failed to create multipart body: %w", err))
	}

	if _, err := part.Write(dslContent); err != nil {
		return nil, sdkerrors.NewInternalError("CreateTransactionWithDSLFile", fmt.Errorf("failed to write multipart body: %w", err))
	}

	if err := writer.Close(); err != nil {
		return nil, sdkerrors.NewInternalError("CreateTransactionWithDSLFile", fmt.Errorf("failed to finalize multipart body: %w", err))
	}

	headers := map[string]string{"Content-Type": writer.FormDataContentType()}

	var responseMap map[string]any
	if err := e.httpClient.doRawRequest(ctx, http.MethodPost, endpointURL, headers, body.Bytes(), &responseMap); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventTransactionCreated, map[string]any{"operation": "CreateTransactionWithDSLFile", "organizationId": orgID, "ledgerId": ledgerID}, err)

		return nil, err
	}

	transaction := e.parseTransactionResponse(responseMap)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionCreated, map[string]any{"operation": "CreateTransactionWithDSLFile", "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return transaction, nil
}

func validateDSLContent(dslContent []byte) error {
	if len(bytes.TrimSpace(dslContent)) == 0 {
		return errors.New("DSL content is required")
	}

	if !utf8.Valid(dslContent) {
		return errors.New("DSL content must be valid UTF-8")
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
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	// Validate required parameters
	if transactionID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "transaction ID")
	}

	// Build the URL for the transaction
	endpointURL := e.buildTransactionURL(orgID, ledgerID, transactionID)

	req, err := newRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		return nil, err
	}

	e.normalizeTransaction(&transaction)

	return &transaction, nil
}

// ListTransactions retrieves a paginated list of transactions for a ledger with optional filters.
//
// This method fetches a list of transactions from the specified organization and ledger,
// with support for cursor pagination, sorting, and filtering.
//
// Parameters:
//   - ctx: Context for the request, which can be used for cancellation and timeout.
//   - orgID: The ID of the organization that owns the ledger. Must be a valid organization ID.
//   - ledgerID: The ID of the ledger to query. Must be a valid ledger ID.
//   - opts: Optional parameters for pagination, sorting, and filtering. Can be nil for default behavior.
//     Supported options include:
//   - Cursor: The cursor returned by the previous transaction list response
//   - Limit: The number of items per page (default is 20)
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
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	// Validate required parameters
	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	// Build the URL for the transactions
	endpointURL := e.buildURL(orgID, ledgerID, "")

	req, err := newRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	// Add query parameters if options are provided
	if opts != nil {
		q := req.URL.Query()

		for key, value := range transactionListQueryParams(opts) {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.Transaction]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	e.normalizeTransactionListResponse(&response)

	return &response, nil
}

func transactionListQueryParams(opts *models.ListOptions) map[string]string {
	return cursorListQueryParams(opts)
}

// GetTransactionsMetricsCount retrieves the count of transactions that match the supplied filters.
func (e *transactionsEntity) GetTransactionsMetricsCount(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.MetricsCount, error) {
	const operation = "GetTransactionsMetricsCount"

	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	endpointURL := e.buildMetricsURL(orgID, ledgerID)

	req, err := newRequestWithContext(ctx, http.MethodHead, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	if opts != nil {
		q := req.URL.Query()
		for key, value := range transactionMetricsCountQueryParams(opts) {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	count, err := e.httpClient.doCountRequest(ctx, http.MethodHead, req.URL.String(), nil)
	if err != nil {
		return nil, err
	}

	return &models.MetricsCount{TransactionsCount: count}, nil
}

func transactionMetricsCountQueryParams(opts *models.ListOptions) map[string]string {
	params := map[string]string{}
	if opts == nil {
		return params
	}

	allowed := map[string]struct{}{
		"route":      {},
		"status":     {},
		"start_date": {},
		"end_date":   {},
	}

	if opts.StartDate != "" {
		params["start_date"] = opts.StartDate
	}

	if opts.EndDate != "" {
		params["end_date"] = opts.EndDate
	}

	for key, value := range opts.Filters {
		if _, ok := allowed[key]; ok && value != "" {
			params[key] = value
		}
	}

	for key, value := range opts.AdditionalParams {
		if _, ok := allowed[key]; ok && value != "" {
			params[key] = value
		}
	}

	return params
}

// UpdateTransaction updates an existing transaction.
func (e *transactionsEntity) UpdateTransaction(ctx context.Context, orgID, ledgerID, transactionID string, input any) (*models.Transaction, error) {
	// Operation name for error context
	const operation = "UpdateTransaction"

	// Validate required parameters
	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "transaction ID")
	}

	if err := validateUpdatePayload(operation, input, "*models.UpdateTransactionInput"); err != nil {
		return nil, err
	}

	// Build the URL for the transaction
	endpointURL := e.buildTransactionURL(orgID, ledgerID, transactionID)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, fmt.Errorf("failed to marshal request body: %w", err))
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, endpointURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, fmt.Errorf("failed to create request: %w", err))
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventTransactionUpdated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID}, err)

		return nil, err
	}

	e.normalizeTransaction(&transaction)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionUpdated, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return &transaction, nil
}

// RevertTransaction reverts a committed transaction.
func (e *transactionsEntity) RevertTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "RevertTransaction"

	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "transaction ID")
	}

	endpointURL := e.buildTransactionURL(orgID, ledgerID, transactionID, "revert")

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventTransactionReverted, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID}, err)

		return nil, err
	}

	e.normalizeTransaction(&transaction)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionReverted, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return &transaction, nil
}

// CommitTransaction commits a pending transaction.
func (e *transactionsEntity) CommitTransaction(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "CommitTransaction"

	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "transaction ID")
	}

	endpointURL := e.buildTransactionURL(orgID, ledgerID, transactionID, "commit")

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		e.httpClient.emitBusinessError(ctx, businessEventTransactionCommitted, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID}, err)

		return nil, err
	}

	e.normalizeTransaction(&transaction)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionCommitted, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return &transaction, nil
}

// CancelTransaction cancels a pending transaction.
func (e *transactionsEntity) CancelTransaction(ctx context.Context, orgID, ledgerID, transactionID string) error {
	_, err := e.CancelTransactionWithResponse(ctx, orgID, ledgerID, transactionID)

	return err
}

// CancelTransactionWithResponse cancels a pending transaction and returns the cancelled transaction.
func (e *transactionsEntity) CancelTransactionWithResponse(ctx context.Context, orgID, ledgerID, transactionID string) (*models.Transaction, error) {
	const operation = "CancelTransaction"

	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if transactionID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "transaction ID")
	}

	endpointURL := e.buildTransactionURL(orgID, ledgerID, transactionID, "cancel")

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, nil)
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	var transaction models.Transaction
	if err := e.httpClient.sendRequest(req, &transaction); err != nil {
		if errors.Is(err, errEmptyResponseBody) || errors.Is(err, errNullResponseBody) {
			// The cancel endpoint sometimes returns a 204/empty body. Even
			// in that case we want callers to receive a normalized
			// Transaction value (with Status populated) and the business
			// event to carry the same status field as the non-empty path,
			// so downstream consumers don't see a status-less event for
			// the empty-body case.
			tx := &models.Transaction{ID: transactionID, Status: models.Status{Code: "CANCELED"}}
			e.normalizeTransaction(tx)
			e.httpClient.emitBusinessEvent(ctx, businessEventTransactionCancelled, map[string]any{
				"operation":      operation,
				"organizationId": orgID,
				"ledgerId":       ledgerID,
				"transactionId":  transactionID,
				"status":         tx.Status.Code,
			})

			return tx, nil
		}

		e.httpClient.emitBusinessError(ctx, businessEventTransactionCancelled, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transactionID}, err)

		return nil, err
	}

	e.normalizeTransaction(&transaction)
	e.httpClient.emitBusinessEvent(ctx, businessEventTransactionCancelled, map[string]any{"operation": operation, "organizationId": orgID, "ledgerId": ledgerID, "transactionId": transaction.ID, "status": transaction.Status.Code})

	return &transaction, nil
}

// CreateInflowTransaction creates an inflow transaction (funds entering the system).
func (e *transactionsEntity) CreateInflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	const operation = "CreateInflowTransaction"

	if orgID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, sdkerrors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, sdkerrors.NewValidationError(operation, "invalid input", err)
	}

	endpointURL := e.buildURL(orgID, ledgerID, "/inflow")

	body, err := json.Marshal(input.ToMap())
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
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
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, sdkerrors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, sdkerrors.NewValidationError(operation, "invalid input", err)
	}

	endpointURL := e.buildURL(orgID, ledgerID, "/outflow")

	body, err := json.Marshal(input.ToMap())
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
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
		return nil, sdkerrors.NewMissingParameterError(operation, "organization ID")
	}

	if ledgerID == "" {
		return nil, sdkerrors.NewMissingParameterError(operation, "ledger ID")
	}

	if input == nil {
		return nil, sdkerrors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, sdkerrors.NewValidationError(operation, "invalid input", err)
	}

	endpointURL := e.buildURL(orgID, ledgerID, "/annotation")

	body, err := json.Marshal(input.ToLibTransaction())
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, sdkerrors.NewInternalError(operation, err)
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
	parts := []string{"transactions"}
	if suffix != "" {
		parts = append(parts, strings.Split(strings.TrimPrefix(suffix, "/"), "/")...)
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], orgID, ledgerID, parts...)
}

func (e *transactionsEntity) buildTransactionURL(orgID, ledgerID, transactionID string, parts ...string) string {
	segments := []string{
		"organizations",
		pathSegment(orgID),
		"ledgers",
		pathSegment(ledgerID),
		"transactions",
		pathSegment(transactionID),
	}

	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}

		for _, piece := range strings.Split(part, "/") {
			if piece != "" {
				segments = append(segments, pathSegment(piece))
			}
		}
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(e.baseURLs["transaction"], "/"), strings.Join(segments, "/"))
}

func (e *transactionsEntity) buildMetricsURL(orgID, ledgerID string) string {
	return buildLedgerScopedURL(e.baseURLs["transaction"], orgID, ledgerID, "transactions", "metrics", "count")
}

// getString safely extracts a string value from a map
func getString(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}

	return ""
}
