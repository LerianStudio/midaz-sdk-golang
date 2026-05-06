package entities

//go:generate mockgen -source=account_types.go -destination=mocks/mock_account_types.go -package=mocks AccountTypesService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// AccountTypesService defines the interface for account type-related operations.
// It provides methods to create, read, update, and delete account types within a ledger.
type AccountTypesService interface {
	// ListAccountTypes retrieves a paginated list of account types for a ledger with optional filters.
	// The organizationID and ledgerID parameters specify which organization and ledger to query.
	// The opts parameter can be used to specify pagination, sorting, and filtering options.
	// Returns a ListResponse containing the account types and pagination information, or an error if the operation fails.
	ListAccountTypes(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) (*models.ListResponse[models.AccountType], error)

	ListAccountTypesAll(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[models.AccountType, error]

	ListAccountTypesPages(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[*models.ListResponse[models.AccountType], error]

	// GetAccountType retrieves a specific account type by its ID.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to retrieve.
	// Returns the account type if found, or an error if the operation fails or the account type doesn't exist.
	GetAccountType(ctx context.Context, organizationID, ledgerID, id string) (*models.AccountType, error)

	// CreateAccountType creates a new account type in the specified ledger.
	//
	// This method creates a new account type that can be used as a template for creating accounts.
	// Account types define the characteristics and behavior of accounts within the ledger.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger. Must be a valid organization ID.
	//   - ledgerID: The ID of the ledger where the account type will be created. Must be a valid ledger ID.
	//   - input: The account type details, including name, keyValue, and optional fields.
	//     Required fields in the input are:
	//     - Name: The human-readable name of the account type (max 256 characters)
	//     - KeyValue: Unique identifier within the organization/ledger (max 100 characters)
	//
	// Returns:
	//   - *models.AccountType: The created account type if successful, containing the account type ID,
	//     timestamps, and other properties.
	//   - error: An error if the operation fails. Possible errors include:
	//     - Invalid input (missing required fields)
	//     - Authentication failure (invalid auth token)
	//     - Authorization failure (insufficient permissions)
	//     - Resource not found (invalid organization or ledger ID)
	//     - Conflict (keyValue already exists)
	//     - Network or server errors
	//
	// Example - Creating a basic cash account type:
	//
	//	// Create a cash account type
	//	accountType, err := accountTypesService.CreateAccountType(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    &models.CreateAccountTypeInput{
	//	        Name: "Cash Account",
	//	        KeyValue: "CASH",
	//	        Description: &description,
	//	        Metadata: map[string]any{
	//	            "category": "liquid_assets",
	//	            "risk_level": "low",
	//	        },
	//	    },
	//	)
	//
	//	if err != nil {
	//	    // Handle error
	//	    return err
	//	}
	//
	//	// Use the account type
	//	fmt.Printf("Account type created: %s (keyValue: %s)\n", accountType.ID, accountType.KeyValue)
	CreateAccountType(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error)

	// UpdateAccountType updates an existing account type.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to update.
	// The input parameter contains the account type details to update, such as name or description.
	// Note that the keyValue field cannot be updated after creation.
	// Returns the updated account type, or an error if the operation fails.
	UpdateAccountType(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error)

	// DeleteAccountType deletes an account type.
	// The organizationID and ledgerID parameters specify which organization and ledger the account type belongs to.
	// The id parameter is the unique identifier of the account type to delete.
	// Note that account types that are in use by existing accounts cannot be deleted.
	// Returns an error if the operation fails.
	DeleteAccountType(ctx context.Context, organizationID, ledgerID, id string) error
}

// accountTypesEntity implements the AccountTypesService interface.
// It handles the communication with the Midaz API for account type-related operations.
type accountTypesEntity struct {
	serviceEntity
}

// newAccountTypesEntity wires the AccountTypesService backed by the shared HTTP transport.
// Internal: invoked by Entity.initServices; callers should reach the service via Client.AccountTypes.
func newAccountTypesEntity(client *http.Client, authToken string, baseURLs map[string]string) AccountTypesService {
	return &accountTypesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
}

// buildURL constructs the URL for account type operations.
func (e *accountTypesEntity) buildURL(organizationID, ledgerID, accountTypeID string) string {
	baseURL := e.baseURLs["onboarding"]

	if accountTypeID == "" {
		return fmt.Sprintf("%s/organizations/%s/ledgers/%s/account-types", baseURL, pathSegment(organizationID), pathSegment(ledgerID))
	}

	return fmt.Sprintf("%s/organizations/%s/ledgers/%s/account-types/%s", baseURL, pathSegment(organizationID), pathSegment(ledgerID), pathSegment(accountTypeID))
}

// ListAccountTypes lists account types for a ledger with optional filters.
func (e *accountTypesEntity) ListAccountTypes(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) (*models.ListResponse[models.AccountType], error) {
	const operation = "ListAccountTypes"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	url := e.buildURL(organizationID, ledgerID, "")

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

	var response models.ListResponse[models.AccountType]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ListAccountTypesAll yields every accounttype matching the request, transparently advancing pagination.
func (e *accountTypesEntity) ListAccountTypesAll(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[models.AccountType, error] {
	return flattenPages(e.ListAccountTypesPages(ctx, organizationID, ledgerID, opts))
}

// ListAccountTypesPages yields one full *ListResponse[AccountType] per page.
func (e *accountTypesEntity) ListAccountTypesPages(ctx context.Context, organizationID, ledgerID string, opts models.AccountTypesListOpts) iter.Seq2[*models.ListResponse[models.AccountType], error] {
	return func(yield func(*models.ListResponse[models.AccountType], error) bool) {
		current := opts
		if current.Page <= 0 {
			current.Page = 1
		}

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListAccountTypes(ctx, organizationID, ledgerID, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			if !page.Pagination.HasMore() {
				return
			}

			current.Page++
		}
	}
}

// GetAccountType gets an account type by ID.
func (e *accountTypesEntity) GetAccountType(ctx context.Context, organizationID, ledgerID, id string) (*models.AccountType, error) {
	const operation = "GetAccountType"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// CreateAccountType creates a new account type.
func (e *accountTypesEntity) CreateAccountType(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	const operation = "CreateAccountType"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account type validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// UpdateAccountType updates an existing account type.
func (e *accountTypesEntity) UpdateAccountType(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	const operation = "UpdateAccountType"

	if organizationID == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return nil, errors.NewMissingParameterError(operation, "id")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	// Validate input
	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "account type validation failed", err)
	}

	url := e.buildURL(organizationID, ledgerID, id)

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var accountType models.AccountType
	if err := e.httpClient.sendRequest(req, &accountType); err != nil {
		return nil, err
	}

	return &accountType, nil
}

// DeleteAccountType deletes an account type.
func (e *accountTypesEntity) DeleteAccountType(ctx context.Context, organizationID, ledgerID, id string) error {
	const operation = "DeleteAccountType"

	if organizationID == "" {
		return errors.NewMissingParameterError(operation, "organizationID")
	}

	if ledgerID == "" {
		return errors.NewMissingParameterError(operation, "ledgerID")
	}

	if id == "" {
		return errors.NewMissingParameterError(operation, "id")
	}

	url := e.buildURL(organizationID, ledgerID, id)

	req, err := newRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.NewInternalError(operation, err)
	}

	return e.httpClient.sendRequest(req, nil)
}
