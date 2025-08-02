package entities

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestNewAccountTypesEntity(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}
	authToken := "test-token"
	baseURLs := map[string]string{"onboarding": "https://api.midaz.io"}

	entity := NewAccountTypesEntity(client, authToken, baseURLs)

	assert.NotNil(t, entity)
	assert.IsType(t, &accountTypesEntity{}, entity)

	accountTypesEntity := entity.(*accountTypesEntity)
	assert.Equal(t, baseURLs, accountTypesEntity.baseURLs)
	assert.NotNil(t, accountTypesEntity.httpClient)
}

func TestAccountTypesEntity_buildURL(t *testing.T) {
	baseURLs := map[string]string{"onboarding": "https://api.midaz.io"}
	entity := &accountTypesEntity{baseURLs: baseURLs}

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		accountTypeID  string
		expected       string
	}{
		{
			name:           "List URL without account type ID",
			organizationID: "org-123",
			ledgerID:       "ledger-456",
			accountTypeID:  "",
			expected:       "https://api.midaz.io/organizations/org-123/ledgers/ledger-456/account-types",
		},
		{
			name:           "Single account type URL with account type ID",
			organizationID: "org-123",
			ledgerID:       "ledger-456",
			accountTypeID:  "at-789",
			expected:       "https://api.midaz.io/organizations/org-123/ledgers/ledger-456/account-types/at-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := entity.buildURL(tt.organizationID, tt.ledgerID, tt.accountTypeID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccountTypesEntity_ListAccountTypes_ValidationErrors(t *testing.T) {
	entity := &accountTypesEntity{}

	ctx := context.Background()

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		expectedError  string
	}{
		{
			name:           "Missing organization ID",
			organizationID: "",
			ledgerID:       "ledger-123",
			expectedError:  "organizationID",
		},
		{
			name:           "Missing ledger ID",
			organizationID: "org-123",
			ledgerID:       "",
			expectedError:  "ledgerID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.ListAccountTypes(ctx, tt.organizationID, tt.ledgerID, nil)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			var missingParamErr *errors.Error

			assert.ErrorAs(t, err, &missingParamErr)
		})
	}
}

func TestAccountTypesEntity_GetAccountType_ValidationErrors(t *testing.T) {
	entity := &accountTypesEntity{}

	ctx := context.Background()

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		id             string
		expectedError  string
	}{
		{
			name:           "Missing organization ID",
			organizationID: "",
			ledgerID:       "ledger-123",
			id:             "at-456",
			expectedError:  "organizationID",
		},
		{
			name:           "Missing ledger ID",
			organizationID: "org-123",
			ledgerID:       "",
			id:             "at-456",
			expectedError:  "ledgerID",
		},
		{
			name:           "Missing account type ID",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			id:             "",
			expectedError:  "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.GetAccountType(ctx, tt.organizationID, tt.ledgerID, tt.id)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			var missingParamErr *errors.Error
			assert.ErrorAs(t, err, &missingParamErr)
		})
	}
}

func TestAccountTypesEntity_CreateAccountType_ValidationErrors(t *testing.T) {
	entity := &accountTypesEntity{}

	ctx := context.Background()

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		input          *models.CreateAccountTypeInput
		expectedError  string
	}{
		{
			name:           "Missing organization ID",
			organizationID: "",
			ledgerID:       "ledger-123",
			input: models.NewCreateAccountTypeInput("Test Account Type", "TEST"),
			expectedError: "organizationID",
		},
		{
			name:           "Missing ledger ID",
			organizationID: "org-123",
			ledgerID:       "",
			input: models.NewCreateAccountTypeInput("Test Account Type", "TEST"),
			expectedError: "ledgerID",
		},
		{
			name:           "Missing input",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			input:          nil,
			expectedError:  "input",
		},
		{
			name:           "Invalid input - missing name",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			input: models.NewCreateAccountTypeInput("", "TEST"),
			expectedError: "validation failed",
		},
		{
			name:           "Invalid input - missing keyValue",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			input: models.NewCreateAccountTypeInput("Test Account Type", ""),
			expectedError: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.CreateAccountType(ctx, tt.organizationID, tt.ledgerID, tt.input)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestAccountTypesEntity_UpdateAccountType_ValidationErrors(t *testing.T) {
	entity := &accountTypesEntity{}

	ctx := context.Background()

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		id             string
		input          *models.UpdateAccountTypeInput
		expectedError  string
	}{
		{
			name:           "Missing organization ID",
			organizationID: "",
			ledgerID:       "ledger-123",
			id:             "at-456",
			input: models.NewUpdateAccountTypeInput().WithName("Updated Name"),
			expectedError: "organizationID",
		},
		{
			name:           "Missing ledger ID",
			organizationID: "org-123",
			ledgerID:       "",
			id:             "at-456",
			input: models.NewUpdateAccountTypeInput().WithName("Updated Name"),
			expectedError: "ledgerID",
		},
		{
			name:           "Missing account type ID",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			id:             "",
			input: models.NewUpdateAccountTypeInput().WithName("Updated Name"),
			expectedError: "id",
		},
		{
			name:           "Missing input",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			id:             "at-456",
			input:          nil,
			expectedError:  "input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.UpdateAccountType(ctx, tt.organizationID, tt.ledgerID, tt.id, tt.input)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestAccountTypesEntity_DeleteAccountType_ValidationErrors(t *testing.T) {
	entity := &accountTypesEntity{}

	ctx := context.Background()

	tests := []struct {
		name           string
		organizationID string
		ledgerID       string
		id             string
		expectedError  string
	}{
		{
			name:           "Missing organization ID",
			organizationID: "",
			ledgerID:       "ledger-123",
			id:             "at-456",
			expectedError:  "organizationID",
		},
		{
			name:           "Missing ledger ID",
			organizationID: "org-123",
			ledgerID:       "",
			id:             "at-456",
			expectedError:  "ledgerID",
		},
		{
			name:           "Missing account type ID",
			organizationID: "org-123",
			ledgerID:       "ledger-123",
			id:             "",
			expectedError:  "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := entity.DeleteAccountType(ctx, tt.organizationID, tt.ledgerID, tt.id)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			var missingParamErr *errors.Error
			assert.ErrorAs(t, err, &missingParamErr)
		})
	}
}

func TestAccountTypesEntity_ErrorCompilation(t *testing.T) {
	// Just to make sure the code compiles with the error package
	err := errors.NewValidationError("test", "test error", nil)
	assert.NotNil(t, err)
}

