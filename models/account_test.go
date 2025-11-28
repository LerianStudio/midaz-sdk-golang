package models

import (
	"strings"
	"testing"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCreateAccountInput tests the factory function for CreateAccountInput
func TestNewCreateAccountInput(t *testing.T) {
	tests := []struct {
		name        string
		accountName string
		assetCode   string
		accountType string
	}{
		{
			name:        "valid input with basic fields",
			accountName: "Savings Account",
			assetCode:   "USD",
			accountType: "deposit",
		},
		{
			name:        "valid input with different asset",
			accountName: "Investment Portfolio",
			assetCode:   "EUR",
			accountType: "savings",
		},
		{
			name:        "valid input with loans type",
			accountName: "Business Loan",
			assetCode:   "BRL",
			accountType: "loans",
		},
		{
			name:        "valid input with marketplace type",
			accountName: "Marketplace Wallet",
			assetCode:   "USD",
			accountType: "marketplace",
		},
		{
			name:        "valid input with creditCard type",
			accountName: "Credit Card Account",
			assetCode:   "USD",
			accountType: "creditCard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput(tt.accountName, tt.assetCode, tt.accountType)

			assert.NotNil(t, input)
			assert.Equal(t, tt.accountName, input.Name)
			assert.Equal(t, tt.assetCode, input.AssetCode)
			assert.Equal(t, tt.accountType, input.Type)
			assert.Equal(t, "ACTIVE", input.Status.Code)
			assert.Nil(t, input.ParentAccountID)
			assert.Nil(t, input.EntityID)
			assert.Nil(t, input.PortfolioID)
			assert.Nil(t, input.SegmentID)
			assert.Nil(t, input.Alias)
			assert.Nil(t, input.Metadata)
		})
	}
}

// TestCreateAccountInputValidate tests the Validate method for CreateAccountInput
func TestCreateAccountInputValidate(t *testing.T) {
	tests := []struct {
		name        string
		input       *CreateAccountInput
		expectError bool
		errContains string
	}{
		{
			name: "valid input",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Status:    NewStatus("ACTIVE"),
			},
			expectError: false,
		},
		{
			name: "empty name",
			input: &CreateAccountInput{
				Name:      "",
				AssetCode: "USD",
				Type:      "deposit",
			},
			expectError: true,
			errContains: "name is required",
		},
		{
			name: "name exceeds max length",
			input: &CreateAccountInput{
				Name:      strings.Repeat("a", 257),
				AssetCode: "USD",
				Type:      "deposit",
			},
			expectError: true,
			errContains: "name must be at most 256 characters",
		},
		{
			name: "name at max length",
			input: &CreateAccountInput{
				Name:      strings.Repeat("a", 256),
				AssetCode: "USD",
				Type:      "deposit",
			},
			expectError: false,
		},
		{
			name: "empty asset code",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "",
				Type:      "deposit",
			},
			expectError: true,
			errContains: "asset code is required",
		},
		{
			name: "empty type",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "",
			},
			expectError: true,
			errContains: "account type is required",
		},
		{
			name: "invalid account type",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "invalid_type",
			},
			expectError: true,
			errContains: "invalid account type",
		},
		{
			name: "valid alias",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr("valid_alias-123"),
			},
			expectError: false,
		},
		{
			name: "invalid alias with special characters",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr("invalid@alias!"),
			},
			expectError: true,
			errContains: "invalid account alias format",
		},
		{
			name: "alias exceeds max length",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr(strings.Repeat("a", 51)),
			},
			expectError: true,
			errContains: "invalid account alias format",
		},
		{
			name: "alias at max length",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr(strings.Repeat("a", 50)),
			},
			expectError: false,
		},
		{
			name: "empty alias pointer",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr(""),
			},
			expectError: false,
		},
		{
			name: "alias with spaces",
			input: &CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "deposit",
				Alias:     stringPtr("alias with spaces"),
			},
			expectError: true,
			errContains: "invalid account alias format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.expectError {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCreateAccountInputWithParentAccountID tests the WithParentAccountID builder method
func TestCreateAccountInputWithParentAccountID(t *testing.T) {
	tests := []struct {
		name            string
		parentAccountID string
	}{
		{
			name:            "valid UUID",
			parentAccountID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:            "another valid UUID",
			parentAccountID: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		},
		{
			name:            "empty string",
			parentAccountID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithParentAccountID(tt.parentAccountID)

			assert.Same(t, input, result)
			assert.NotNil(t, input.ParentAccountID)
			assert.Equal(t, tt.parentAccountID, *input.ParentAccountID)
		})
	}
}

// TestCreateAccountInputWithEntityID tests the WithEntityID builder method
func TestCreateAccountInputWithEntityID(t *testing.T) {
	tests := []struct {
		name     string
		entityID string
	}{
		{
			name:     "valid entity ID",
			entityID: "entity-12345",
		},
		{
			name:     "UUID entity ID",
			entityID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "empty string",
			entityID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithEntityID(tt.entityID)

			assert.Same(t, input, result)
			assert.NotNil(t, input.EntityID)
			assert.Equal(t, tt.entityID, *input.EntityID)
		})
	}
}

// TestCreateAccountInputWithPortfolioID tests the WithPortfolioID builder method
func TestCreateAccountInputWithPortfolioID(t *testing.T) {
	tests := []struct {
		name        string
		portfolioID string
	}{
		{
			name:        "valid UUID",
			portfolioID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:        "empty string",
			portfolioID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithPortfolioID(tt.portfolioID)

			assert.Same(t, input, result)
			assert.NotNil(t, input.PortfolioID)
			assert.Equal(t, tt.portfolioID, *input.PortfolioID)
		})
	}
}

// TestCreateAccountInputWithSegmentID tests the WithSegmentID builder method
func TestCreateAccountInputWithSegmentID(t *testing.T) {
	tests := []struct {
		name      string
		segmentID string
	}{
		{
			name:      "valid UUID",
			segmentID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "empty string",
			segmentID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithSegmentID(tt.segmentID)

			assert.Same(t, input, result)
			assert.NotNil(t, input.SegmentID)
			assert.Equal(t, tt.segmentID, *input.SegmentID)
		})
	}
}

// TestCreateAccountInputWithStatus tests the WithStatus builder method
func TestCreateAccountInputWithStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode string
	}{
		{
			name:       "ACTIVE status",
			statusCode: "ACTIVE",
		},
		{
			name:       "INACTIVE status",
			statusCode: "INACTIVE",
		},
		{
			name:       "CLOSED status",
			statusCode: "CLOSED",
		},
		{
			name:       "PENDING status",
			statusCode: "PENDING",
		},
		{
			name:       "BLOCKED status",
			statusCode: "BLOCKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			status := NewStatus(tt.statusCode)
			result := input.WithStatus(status)

			assert.Same(t, input, result)
			assert.Equal(t, tt.statusCode, input.Status.Code)
		})
	}
}

// TestCreateAccountInputWithAlias tests the WithAlias builder method
func TestCreateAccountInputWithAlias(t *testing.T) {
	tests := []struct {
		name  string
		alias string
	}{
		{
			name:  "simple alias",
			alias: "my_account",
		},
		{
			name:  "alphanumeric alias",
			alias: "account123",
		},
		{
			name:  "alias with hyphens",
			alias: "user-account-1",
		},
		{
			name:  "alias with underscores",
			alias: "user_account_1",
		},
		{
			name:  "empty alias",
			alias: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithAlias(tt.alias)

			assert.Same(t, input, result)
			assert.NotNil(t, input.Alias)
			assert.Equal(t, tt.alias, *input.Alias)
		})
	}
}

// TestCreateAccountInputWithMetadata tests the WithMetadata builder method
func TestCreateAccountInputWithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name: "simple metadata",
			metadata: map[string]any{
				"key1": "value1",
			},
		},
		{
			name: "complex metadata",
			metadata: map[string]any{
				"customer_id": "cust-123",
				"email":       "test@example.com",
				"priority":    1,
				"active":      true,
			},
		},
		{
			name: "nested metadata",
			metadata: map[string]any{
				"user": map[string]any{
					"name": "John",
					"age":  30,
				},
			},
		},
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit")
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result)
			assert.Equal(t, tt.metadata, input.Metadata)
		})
	}
}

// TestCreateAccountInputToMmodel tests the ToMmodel conversion method
func TestCreateAccountInputToMmodel(t *testing.T) {
	alias := "test_alias"
	parentID := "parent-123"
	entityID := "entity-456"
	portfolioID := "portfolio-789"
	segmentID := "segment-012"

	input := CreateAccountInput{
		Name:            "Test Account",
		ParentAccountID: &parentID,
		EntityID:        &entityID,
		AssetCode:       "USD",
		PortfolioID:     &portfolioID,
		SegmentID:       &segmentID,
		Status:          NewStatus("ACTIVE"),
		Alias:           &alias,
		Type:            "deposit",
		Metadata: map[string]any{
			"key": "value",
		},
	}

	result := input.ToMmodel()

	assert.Equal(t, input.Name, result.Name)
	assert.Equal(t, input.ParentAccountID, result.ParentAccountID)
	assert.Equal(t, input.EntityID, result.EntityID)
	assert.Equal(t, input.AssetCode, result.AssetCode)
	assert.Equal(t, input.PortfolioID, result.PortfolioID)
	assert.Equal(t, input.SegmentID, result.SegmentID)
	assert.Equal(t, input.Status.Code, result.Status.Code)
	assert.Equal(t, input.Alias, result.Alias)
	assert.Equal(t, input.Type, result.Type)
	assert.Equal(t, input.Metadata, result.Metadata)
}

// TestCreateAccountInputBuilderChaining tests chaining multiple builder methods
func TestCreateAccountInputBuilderChaining(t *testing.T) {
	input := NewCreateAccountInput("Chained Account", "EUR", "savings").
		WithParentAccountID("parent-id").
		WithEntityID("entity-id").
		WithPortfolioID("portfolio-id").
		WithSegmentID("segment-id").
		WithStatus(NewStatus("PENDING")).
		WithAlias("chained_alias").
		WithMetadata(map[string]any{"chain": "test"})

	assert.Equal(t, "Chained Account", input.Name)
	assert.Equal(t, "EUR", input.AssetCode)
	assert.Equal(t, "savings", input.Type)
	assert.NotNil(t, input.ParentAccountID)
	assert.Equal(t, "parent-id", *input.ParentAccountID)
	assert.NotNil(t, input.EntityID)
	assert.Equal(t, "entity-id", *input.EntityID)
	assert.NotNil(t, input.PortfolioID)
	assert.Equal(t, "portfolio-id", *input.PortfolioID)
	assert.NotNil(t, input.SegmentID)
	assert.Equal(t, "segment-id", *input.SegmentID)
	assert.Equal(t, "PENDING", input.Status.Code)
	assert.NotNil(t, input.Alias)
	assert.Equal(t, "chained_alias", *input.Alias)
	assert.Equal(t, "test", input.Metadata["chain"])
}

// TestNewUpdateAccountInput tests the factory function for UpdateAccountInput
func TestNewUpdateAccountInput(t *testing.T) {
	input := NewUpdateAccountInput()

	assert.NotNil(t, input)
	assert.Empty(t, input.Name)
	assert.Nil(t, input.SegmentID)
	assert.Nil(t, input.PortfolioID)
	assert.Empty(t, input.Status.Code)
	assert.Nil(t, input.Metadata)
}

// TestUpdateAccountInputValidate tests the Validate method for UpdateAccountInput
func TestUpdateAccountInputValidate(t *testing.T) {
	tests := []struct {
		name        string
		input       *UpdateAccountInput
		expectError bool
		errContains string
	}{
		{
			name:        "empty input is valid",
			input:       &UpdateAccountInput{},
			expectError: false,
		},
		{
			name: "valid name",
			input: &UpdateAccountInput{
				Name: "Updated Account",
			},
			expectError: false,
		},
		{
			name: "name exceeds max length",
			input: &UpdateAccountInput{
				Name: strings.Repeat("a", 257),
			},
			expectError: true,
			errContains: "name must be at most 256 characters",
		},
		{
			name: "name at max length",
			input: &UpdateAccountInput{
				Name: strings.Repeat("a", 256),
			},
			expectError: false,
		},
		{
			name: "valid metadata",
			input: &UpdateAccountInput{
				Metadata: map[string]any{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name: "metadata with empty key",
			input: &UpdateAccountInput{
				Metadata: map[string]any{
					"": "value",
				},
			},
			expectError: true,
			errContains: "invalid metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.expectError {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUpdateAccountInputWithName tests the WithName builder method
func TestUpdateAccountInputWithName(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
	}{
		{
			name:      "simple name",
			inputName: "Updated Account",
		},
		{
			name:      "name with spaces",
			inputName: "My Updated Account Name",
		},
		{
			name:      "empty name",
			inputName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAccountInput()
			result := input.WithName(tt.inputName)

			assert.Same(t, input, result)
			assert.Equal(t, tt.inputName, input.Name)
		})
	}
}

// TestUpdateAccountInputWithSegmentID tests the WithSegmentID builder method
func TestUpdateAccountInputWithSegmentID(t *testing.T) {
	input := NewUpdateAccountInput()
	segmentID := "segment-123"
	result := input.WithSegmentID(segmentID)

	assert.Same(t, input, result)
	assert.NotNil(t, input.SegmentID)
	assert.Equal(t, segmentID, *input.SegmentID)
}

// TestUpdateAccountInputWithPortfolioID tests the WithPortfolioID builder method
func TestUpdateAccountInputWithPortfolioID(t *testing.T) {
	input := NewUpdateAccountInput()
	portfolioID := "portfolio-123"
	result := input.WithPortfolioID(portfolioID)

	assert.Same(t, input, result)
	assert.NotNil(t, input.PortfolioID)
	assert.Equal(t, portfolioID, *input.PortfolioID)
}

// TestUpdateAccountInputWithStatus tests the WithStatus builder method
func TestUpdateAccountInputWithStatus(t *testing.T) {
	input := NewUpdateAccountInput()
	status := NewStatus("BLOCKED")
	result := input.WithStatus(status)

	assert.Same(t, input, result)
	assert.Equal(t, "BLOCKED", input.Status.Code)
}

// TestUpdateAccountInputWithMetadata tests the WithMetadata builder method
func TestUpdateAccountInputWithMetadata(t *testing.T) {
	input := NewUpdateAccountInput()
	metadata := map[string]any{
		"updated": true,
		"version": 2,
	}
	result := input.WithMetadata(metadata)

	assert.Same(t, input, result)
	assert.Equal(t, metadata, input.Metadata)
}

// TestUpdateAccountInputToMmodel tests the ToMmodel conversion method
func TestUpdateAccountInputToMmodel(t *testing.T) {
	segmentID := "segment-123"
	portfolioID := "portfolio-456"

	input := UpdateAccountInput{
		Name:        "Updated Account",
		SegmentID:   &segmentID,
		PortfolioID: &portfolioID,
		Status:      NewStatus("ACTIVE"),
		Metadata: map[string]any{
			"key": "value",
		},
	}

	result := input.ToMmodel()

	assert.Equal(t, input.Name, result.Name)
	assert.Equal(t, input.SegmentID, result.SegmentID)
	assert.Equal(t, input.PortfolioID, result.PortfolioID)
	assert.Equal(t, input.Status.Code, result.Status.Code)
	assert.Equal(t, input.Metadata, result.Metadata)
}

// TestUpdateAccountInputBuilderChaining tests chaining multiple builder methods
func TestUpdateAccountInputBuilderChaining(t *testing.T) {
	input := NewUpdateAccountInput().
		WithName("Chained Update").
		WithSegmentID("segment-id").
		WithPortfolioID("portfolio-id").
		WithStatus(NewStatus("CLOSED")).
		WithMetadata(map[string]any{"chain": "update"})

	assert.Equal(t, "Chained Update", input.Name)
	assert.NotNil(t, input.SegmentID)
	assert.Equal(t, "segment-id", *input.SegmentID)
	assert.NotNil(t, input.PortfolioID)
	assert.Equal(t, "portfolio-id", *input.PortfolioID)
	assert.Equal(t, "CLOSED", input.Status.Code)
	assert.Equal(t, "update", input.Metadata["chain"])
}

// TestListAccountInputValidate tests the Validate method for ListAccountInput
func TestListAccountInputValidate(t *testing.T) {
	tests := []struct {
		name        string
		input       *ListAccountInput
		expectError bool
		errContains string
	}{
		{
			name:        "empty input is valid",
			input:       &ListAccountInput{},
			expectError: false,
		},
		{
			name: "valid page and perPage",
			input: &ListAccountInput{
				Page:    1,
				PerPage: 10,
			},
			expectError: false,
		},
		{
			name: "negative page",
			input: &ListAccountInput{
				Page: -1,
			},
			expectError: true,
			errContains: "page number cannot be negative",
		},
		{
			name: "negative perPage",
			input: &ListAccountInput{
				PerPage: -1,
			},
			expectError: true,
			errContains: "perPage cannot be negative",
		},
		{
			name: "perPage exceeds max",
			input: &ListAccountInput{
				PerPage: 101,
			},
			expectError: true,
			errContains: "perPage cannot exceed 100",
		},
		{
			name: "perPage at max",
			input: &ListAccountInput{
				PerPage: 100,
			},
			expectError: false,
		},
		{
			name: "valid filter",
			input: &ListAccountInput{
				Filter: AccountFilter{
					Status: []string{"ACTIVE", "PENDING"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.expectError {
				require.Error(t, err)

				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestAccountsFromMmodel tests the FromMmodel conversion function
func TestAccountsFromMmodel(t *testing.T) {
	mmodelAccounts := mmodel.Accounts{
		Items: []mmodel.Account{
			{
				ID:   "acc-1",
				Name: "Account 1",
			},
			{
				ID:   "acc-2",
				Name: "Account 2",
			},
		},
		Page:  1,
		Limit: 10,
	}

	result := FromMmodel(mmodelAccounts)

	assert.Len(t, result.Items, 2)
	assert.Equal(t, "acc-1", result.Items[0].ID)
	assert.Equal(t, "Account 1", result.Items[0].Name)
	assert.Equal(t, "acc-2", result.Items[1].ID)
	assert.Equal(t, "Account 2", result.Items[1].Name)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.Limit)
}

// TestAccountsFromMmodelEmpty tests FromMmodel with empty accounts
func TestAccountsFromMmodelEmpty(t *testing.T) {
	mmodelAccounts := mmodel.Accounts{
		Items: []mmodel.Account{},
		Page:  1,
		Limit: 10,
	}

	result := FromMmodel(mmodelAccounts)

	assert.Empty(t, result.Items)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.Limit)
}

// TestGetAccountAlias tests the GetAccountAlias helper function
func TestGetAccountAlias(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "account with alias",
			account: Account{
				ID:    "acc-1",
				Alias: stringPtr("my_alias"),
			},
			expected: "my_alias",
		},
		{
			name: "account without alias",
			account: Account{
				ID:    "acc-2",
				Alias: nil,
			},
			expected: "",
		},
		{
			name: "account with empty alias",
			account: Account{
				ID:    "acc-3",
				Alias: stringPtr(""),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAccountAlias(tt.account)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetAccountIdentifier tests the GetAccountIdentifier helper function
func TestGetAccountIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "account with alias returns alias",
			account: Account{
				ID:    "acc-123",
				Alias: stringPtr("my_alias"),
			},
			expected: "my_alias",
		},
		{
			name: "account without alias returns ID",
			account: Account{
				ID:    "acc-456",
				Alias: nil,
			},
			expected: "acc-456",
		},
		{
			name: "account with empty alias returns ID",
			account: Account{
				ID:    "acc-789",
				Alias: stringPtr(""),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAccountIdentifier(tt.account)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAccountFilterStructure tests the AccountFilter structure
func TestAccountFilterStructure(t *testing.T) {
	filter := AccountFilter{
		Status: []string{"ACTIVE", "PENDING", "BLOCKED"},
	}

	assert.Len(t, filter.Status, 3)
	assert.Contains(t, filter.Status, "ACTIVE")
	assert.Contains(t, filter.Status, "PENDING")
	assert.Contains(t, filter.Status, "BLOCKED")
}

// TestAccountFilterEmpty tests an empty AccountFilter
func TestAccountFilterEmpty(t *testing.T) {
	filter := AccountFilter{}

	assert.Nil(t, filter.Status)
}

// TestListAccountResponseStructure tests the ListAccountResponse structure
func TestListAccountResponseStructure(t *testing.T) {
	response := ListAccountResponse{
		Items: []Account{
			{ID: "acc-1", Name: "Account 1"},
			{ID: "acc-2", Name: "Account 2"},
		},
		Total:       100,
		CurrentPage: 1,
		PageSize:    10,
		TotalPages:  10,
	}

	assert.Len(t, response.Items, 2)
	assert.Equal(t, 100, response.Total)
	assert.Equal(t, 1, response.CurrentPage)
	assert.Equal(t, 10, response.PageSize)
	assert.Equal(t, 10, response.TotalPages)
}

// TestAccountsStructure tests the Accounts structure
func TestAccountsStructure(t *testing.T) {
	accounts := Accounts{
		Items: []Account{
			{ID: "acc-1", Name: "Account 1"},
		},
		Page:  2,
		Limit: 25,
	}

	assert.Len(t, accounts.Items, 1)
	assert.Equal(t, 2, accounts.Page)
	assert.Equal(t, 25, accounts.Limit)
}

// TestStatusCodesForAccount tests various status codes used for accounts
func TestStatusCodesForAccount(t *testing.T) {
	statuses := []string{
		"ACTIVE",
		"INACTIVE",
		"CLOSED",
		"PENDING",
		"BLOCKED",
	}

	for _, code := range statuses {
		t.Run(code, func(t *testing.T) {
			status := NewStatus(code)
			assert.Equal(t, code, status.Code)
			assert.Nil(t, status.Description)
		})
	}
}

// TestStatusWithDescription tests status with description
func TestStatusWithDescription(t *testing.T) {
	status := NewStatus("BLOCKED")
	statusWithDesc := WithStatusDescription(status, "Account blocked due to suspicious activity")

	assert.Equal(t, "BLOCKED", statusWithDesc.Code)
	assert.NotNil(t, statusWithDesc.Description)
	assert.Equal(t, "Account blocked due to suspicious activity", *statusWithDesc.Description)
}

// TestValidAccountTypes tests valid account types
func TestValidAccountTypes(t *testing.T) {
	validTypes := []string{
		"deposit",
		"savings",
		"loans",
		"marketplace",
		"creditCard",
	}

	for _, accountType := range validTypes {
		t.Run(accountType, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", accountType)
			err := input.Validate()
			require.NoError(t, err)
		})
	}
}

// TestCreateAccountInputWithAllFields tests creating an input with all fields populated
func TestCreateAccountInputWithAllFields(t *testing.T) {
	parentID := "parent-123"
	entityID := "entity-456"
	portfolioID := "portfolio-789"
	segmentID := "segment-012"
	alias := "full_account"
	description := "Test account"

	input := &CreateAccountInput{
		Name:            "Full Account",
		ParentAccountID: &parentID,
		EntityID:        &entityID,
		AssetCode:       "USD",
		PortfolioID:     &portfolioID,
		SegmentID:       &segmentID,
		Status:          WithStatusDescription(NewStatus("ACTIVE"), description),
		Alias:           &alias,
		Type:            "deposit",
		Metadata: map[string]any{
			"created_by": "test",
			"priority":   1,
		},
	}

	err := input.Validate()
	require.NoError(t, err)

	assert.Equal(t, "Full Account", input.Name)
	assert.Equal(t, "parent-123", *input.ParentAccountID)
	assert.Equal(t, "entity-456", *input.EntityID)
	assert.Equal(t, "USD", input.AssetCode)
	assert.Equal(t, "portfolio-789", *input.PortfolioID)
	assert.Equal(t, "segment-012", *input.SegmentID)
	assert.Equal(t, "ACTIVE", input.Status.Code)
	assert.Equal(t, description, *input.Status.Description)
	assert.Equal(t, "full_account", *input.Alias)
	assert.Equal(t, "deposit", input.Type)
	assert.Equal(t, "test", input.Metadata["created_by"])
	assert.Equal(t, 1, input.Metadata["priority"])
}

// TestAccountAliasValidCharacters tests valid alias character combinations
func TestAccountAliasValidCharacters(t *testing.T) {
	validAliases := []string{
		"a",
		"abc",
		"abc123",
		"ABC123",
		"test_account",
		"test-account",
		"Test_Account-123",
		"a1b2c3",
		strings.Repeat("a", 50),
	}

	for _, alias := range validAliases {
		t.Run(alias, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit").WithAlias(alias)
			err := input.Validate()
			require.NoError(t, err, "alias '%s' should be valid", alias)
		})
	}
}

// TestAccountAliasInvalidCharacters tests invalid alias character combinations
func TestAccountAliasInvalidCharacters(t *testing.T) {
	invalidAliases := []string{
		"test@account",
		"test.account",
		"test account",
		"test!account",
		"test#account",
		"test$account",
		"test%account",
		"test&account",
		"test*account",
		"test(account",
		"test)account",
		"test+account",
		"test=account",
		"test[account",
		"test]account",
		"test{account",
		"test}account",
		"test|account",
		"test\\account",
		"test/account",
		"test?account",
		"test<account",
		"test>account",
		"test,account",
		strings.Repeat("a", 51),
	}

	for _, alias := range invalidAliases {
		t.Run(alias, func(t *testing.T) {
			input := NewCreateAccountInput("Test", "USD", "deposit").WithAlias(alias)
			err := input.Validate()
			require.Error(t, err, "alias '%s' should be invalid", alias)
		})
	}
}

// TestMetadataValidation tests various metadata configurations
func TestMetadataValidation(t *testing.T) {
	tests := []struct {
		name        string
		metadata    map[string]any
		expectError bool
	}{
		{
			name:        "nil metadata",
			metadata:    nil,
			expectError: false,
		},
		{
			name:        "empty metadata",
			metadata:    map[string]any{},
			expectError: false,
		},
		{
			name: "string values",
			metadata: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
			expectError: false,
		},
		{
			name: "integer values",
			metadata: map[string]any{
				"count": 10,
				"total": 100,
			},
			expectError: false,
		},
		{
			name: "float values",
			metadata: map[string]any{
				"rate":   0.05,
				"amount": 100.50,
			},
			expectError: false,
		},
		{
			name: "boolean values",
			metadata: map[string]any{
				"active":   true,
				"verified": false,
			},
			expectError: false,
		},
		{
			name: "nil value",
			metadata: map[string]any{
				"nullable": nil,
			},
			expectError: false,
		},
		{
			name: "mixed values",
			metadata: map[string]any{
				"name":     "test",
				"count":    10,
				"rate":     0.05,
				"active":   true,
				"nullable": nil,
			},
			expectError: false,
		},
		{
			name: "nested map",
			metadata: map[string]any{
				"nested": map[string]any{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name: "array value",
			metadata: map[string]any{
				"tags": []any{"tag1", "tag2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAccountInput().WithMetadata(tt.metadata)
			err := input.Validate()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
