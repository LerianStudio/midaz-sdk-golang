package generator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAccountsService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error)
}

func (m *mockAccountsService) CreateAccount(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}

	return &models.Account{ID: "acc-123", Name: input.Name}, nil
}

func (*mockAccountsService) GetAccount(_ context.Context, _, _, _ string) (*models.Account, error) {
	return nil, errors.New("mock: GetAccount not implemented")
}

func (*mockAccountsService) ListAccounts(_ context.Context, _, _ string, _ *models.ListOptions) (*models.ListResponse[models.Account], error) {
	return nil, errors.New("mock: ListAccounts not implemented")
}

func (*mockAccountsService) UpdateAccount(_ context.Context, _, _, _ string, _ *models.UpdateAccountInput) (*models.Account, error) {
	return nil, errors.New("mock: UpdateAccount not implemented")
}

func (*mockAccountsService) DeleteAccount(_ context.Context, _, _, _ string) error {
	return nil
}

func (*mockAccountsService) GetAccountBalance(_ context.Context, _, _, _ string) (*models.Balance, error) {
	return nil, errors.New("mock: GetAccountBalance not implemented")
}

func (*mockAccountsService) GetAccountByAlias(_ context.Context, _, _, _ string) (*models.Account, error) {
	return nil, errors.New("mock: GetAccountByAlias not implemented")
}

func (*mockAccountsService) GetAccountByAliasPath(_ context.Context, _, _, _ string) (*models.Account, error) {
	return nil, errors.New("mock: GetAccountByAliasPath not implemented")
}

func (*mockAccountsService) GetAccountsMetricsCount(_ context.Context, _, _ string) (*models.MetricsCount, error) {
	return nil, errors.New("mock: GetAccountsMetricsCount not implemented")
}

func (*mockAccountsService) GetExternalAccount(_ context.Context, _, _, _ string) (*models.Account, error) {
	return nil, errors.New("mock: GetExternalAccount not implemented")
}

func (*mockAccountsService) GetExternalAccountBalance(_ context.Context, _, _, _ string) (*models.Balance, error) {
	return nil, errors.New("mock: GetExternalAccountBalance not implemented")
}

func (*mockAccountsService) GetBalance(_ context.Context, _, _, _ string) (*models.Balance, error) {
	return nil, errors.New("mock: GetBalance not implemented")
}

func TestNewAccountGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewAccountGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewAccountGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestAccountGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewAccountGenerator(nil, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAccountGenerator_Generate_NilAccountsService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAccountGenerator_Generate_EmptyOrgID(t *testing.T) {
	mockSvc := &mockAccountsService{}
	e := &entities.Entity{
		Accounts: mockSvc,
	}
	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	_, err := gen.Generate(context.Background(), "", "ledger-123", "USD", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization and ledger IDs are required")
}

func TestAccountGenerator_Generate_EmptyLedgerID(t *testing.T) {
	mockSvc := &mockAccountsService{}
	e := &entities.Entity{
		Accounts: mockSvc,
	}
	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	_, err := gen.Generate(context.Background(), "org-123", "", "USD", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization and ledger IDs are required")
}

func TestAccountGenerator_Generate_EmptyAssetCode(t *testing.T) {
	mockSvc := &mockAccountsService{}
	e := &entities.Entity{
		Accounts: mockSvc,
	}
	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "asset code is required")
}

func TestAccountGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			return &models.Account{
				ID:   "acc-success",
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name:   "Checking Account",
		Type:   "deposit",
		Status: models.NewStatus(models.StatusActive),
		Metadata: map[string]any{
			"owner": "John Doe",
		},
	}

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.Equal(t, "acc-success", result.ID)
	assert.Equal(t, "Checking Account", result.Name)
}

func TestAccountGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, _ *models.CreateAccountInput) (*models.Account, error) {
			return nil, errors.New("account creation failed")
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Test Account",
		Type: "deposit",
	}

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "account creation failed")
}

func TestAccountGenerator_Generate_WithAlias(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	alias := "my-checking"
	template := data.AccountTemplate{
		Name:  "Checking Account",
		Type:  "deposit",
		Alias: &alias,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountGenerator_Generate_WithParentAccount(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	parentID := "parent-acc-id"
	template := data.AccountTemplate{
		Name:            "Child Account",
		Type:            "deposit",
		ParentAccountID: &parentID,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountGenerator_Generate_WithPortfolioAndSegment(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	portfolioID := "portfolio-123"
	segmentID := "segment-456"
	entityID := "entity-789"
	template := data.AccountTemplate{
		Name:        "Investment Account",
		Type:        "savings",
		PortfolioID: &portfolioID,
		SegmentID:   &segmentID,
		EntityID:    &entityID,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountGenerator_GenerateBatch_EmptyTemplates(t *testing.T) {
	gen := NewAccountGenerator(nil, nil)

	results, err := gen.GenerateBatch(context.Background(), "org-123", "ledger-123", "USD", []data.AccountTemplate{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestAccountGenerator_GenerateBatch_NilEntity(t *testing.T) {
	gen := NewAccountGenerator(nil, nil)

	templates := []data.AccountTemplate{
		{Name: "Account 1", Type: "deposit"},
		{Name: "Account 2", Type: "savings"},
	}

	results, err := gen.GenerateBatch(context.Background(), "org-123", "ledger-123", "USD", templates)
	require.Error(t, err)
	assert.Empty(t, results)
}

func TestAccountGenerator_GenerateBatch_Success(t *testing.T) {
	var callCount atomic.Int32
	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			count := callCount.Add(1)

			return &models.Account{
				ID:   "acc-" + string(rune('0'+count)),
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 2)

	templates := []data.AccountTemplate{
		{Name: "Account 1", Type: "deposit"},
		{Name: "Account 2", Type: "savings"},
		{Name: "Account 3", Type: "creditCard"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", "USD", templates)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestAccountGenerator_GenerateBatch_PartialError(t *testing.T) {
	var callCount atomic.Int32
	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			count := callCount.Add(1)
			if count == 2 {
				return nil, errors.New("partial failure")
			}

			return &models.Account{
				ID:   "acc-ok",
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 1)

	templates := []data.AccountTemplate{
		{Name: "Account 1", Type: "deposit"},
		{Name: "Account 2", Type: "savings"},
		{Name: "Account 3", Type: "expense"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", "USD", templates)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partial failure")
	assert.Len(t, results, 2)
}

func TestMapAccountClass(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"expense type", "expense", "EXPENSE"},
		{"revenue type", "revenue", "REVENUE"},
		{"liability type", "liability", "LIABILITY"},
		{"equity type", "equity", "EQUITY"},
		{"creditCard type", "creditCard", "LIABILITY"},
		{"deposit type", "deposit", "ASSET"},
		{"savings type", "savings", "ASSET"},
		{"unknown type", "unknown", "ASSET"},
		{"empty type", "", "ASSET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAccountClass(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInferAccountTypeKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"deposit type", "deposit", AccountTypeKeyChecking},
		{"marketplace type", "marketplace", AccountTypeKeyChecking},
		{"savings type", "savings", AccountTypeKeySavings},
		{"creditCard type", "creditCard", AccountTypeKeyCreditCard},
		{"expense type", "expense", AccountTypeKeyExpense},
		{"revenue type", "revenue", AccountTypeKeyRevenue},
		{"liability type", "liability", AccountTypeKeyLiability},
		{"equity type", "equity", AccountTypeKeyEquity},
		{"unknown type", "unknown", ""},
		{"empty type", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferAccountTypeKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSupportedAccountTypeKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"CHECKING", AccountTypeKeyChecking, true},
		{"SAVINGS", AccountTypeKeySavings, true},
		{"CREDIT_CARD", AccountTypeKeyCreditCard, true},
		{"EXPENSE", AccountTypeKeyExpense, true},
		{"REVENUE", AccountTypeKeyRevenue, true},
		{"LIABILITY", AccountTypeKeyLiability, true},
		{"EQUITY", AccountTypeKeyEquity, true},
		{"invalid key", "INVALID", false},
		{"empty key", "", false},
		{"lowercase key", "checking", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSupportedAccountTypeKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccountGenerator_Generate_WithAccountTypeKey(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	accountTypeKey := AccountTypeKeyChecking
	template := data.AccountTemplate{
		Name:           "Checking Account",
		Type:           "deposit",
		AccountTypeKey: &accountTypeKey,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.NotNil(t, capturedInput.Metadata)
	assert.Equal(t, AccountTypeKeyChecking, capturedInput.Metadata["account_type_key"])
}

func TestAccountGenerator_Generate_WithInvalidAccountTypeKey(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	invalidKey := "INVALID_KEY"
	template := data.AccountTemplate{
		Name:           "Checking Account",
		Type:           "deposit",
		AccountTypeKey: &invalidKey,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.Equal(t, AccountTypeKeyChecking, capturedInput.Metadata["account_type_key"])
}

func TestAccountGenerator_Generate_InferredAccountTypeKey(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name: "Savings Account",
		Type: "savings",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.NotNil(t, capturedInput.Metadata)
	assert.Equal(t, AccountTypeKeySavings, capturedInput.Metadata["account_type_key"])
}

func TestAccountTemplate_Fields(t *testing.T) {
	t.Run("Complete template", func(t *testing.T) {
		alias := "checking-main"
		parentID := "parent-123"
		portfolioID := "portfolio-456"
		segmentID := "segment-789"
		entityID := "entity-abc"
		accountTypeKey := AccountTypeKeyChecking

		template := data.AccountTemplate{
			Name:            "Main Checking",
			Type:            "deposit",
			Status:          models.NewStatus(models.StatusActive),
			Alias:           &alias,
			ParentAccountID: &parentID,
			PortfolioID:     &portfolioID,
			SegmentID:       &segmentID,
			EntityID:        &entityID,
			AccountTypeKey:  &accountTypeKey,
			Metadata: map[string]any{
				"owner": "John Doe",
			},
		}

		assert.Equal(t, "Main Checking", template.Name)
		assert.Equal(t, "deposit", template.Type)
		assert.NotNil(t, template.Status)
		assert.Equal(t, "checking-main", *template.Alias)
		assert.Equal(t, "parent-123", *template.ParentAccountID)
		assert.Equal(t, "portfolio-456", *template.PortfolioID)
		assert.Equal(t, "segment-789", *template.SegmentID)
		assert.Equal(t, "entity-abc", *template.EntityID)
		assert.Equal(t, AccountTypeKeyChecking, *template.AccountTypeKey)
		assert.NotNil(t, template.Metadata)
		assert.Equal(t, "John Doe", template.Metadata["owner"])
	})

	t.Run("Minimal template", func(t *testing.T) {
		template := data.AccountTemplate{
			Name: "Minimal Account",
			Type: "deposit",
		}

		assert.Equal(t, "Minimal Account", template.Name)
		assert.Equal(t, "deposit", template.Type)
		assert.Nil(t, template.Alias)
		assert.Nil(t, template.ParentAccountID)
	})
}

func TestAccountGenerator_Generate_EmptyAlias(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	emptyAlias := ""
	template := data.AccountTemplate{
		Name:  "Test Account",
		Type:  "deposit",
		Alias: &emptyAlias,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountGenerator_Generate_EmptyParentAccountID(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	emptyParentID := ""
	template := data.AccountTemplate{
		Name:            "Test Account",
		Type:            "deposit",
		ParentAccountID: &emptyParentID,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountGenerator_Generate_NilMetadata(t *testing.T) {
	var capturedInput *models.CreateAccountInput

	mockSvc := &mockAccountsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateAccountInput) (*models.Account, error) {
			capturedInput = input
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	e := &entities.Entity{
		Accounts: mockSvc,
	}

	gen := NewAccountGenerator(e, nil)
	template := data.AccountTemplate{
		Name:     "Test Account",
		Type:     "deposit",
		Metadata: nil,
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "USD", template)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.NotNil(t, capturedInput.Metadata)
}

func TestSupportedAccountTypeKeys(t *testing.T) {
	expectedKeys := []string{
		AccountTypeKeyChecking,
		AccountTypeKeySavings,
		AccountTypeKeyCreditCard,
		AccountTypeKeyExpense,
		AccountTypeKeyRevenue,
		AccountTypeKeyLiability,
		AccountTypeKeyEquity,
	}

	assert.Equal(t, expectedKeys, supportedAccountTypeKeys)
}
