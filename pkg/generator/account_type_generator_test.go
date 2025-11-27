package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAccountTypesService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error)
}

func (m *mockAccountTypesService) CreateAccountType(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}
	return &models.AccountType{ID: uuid.New(), Name: input.Name, KeyValue: input.KeyValue}, nil
}

func (m *mockAccountTypesService) GetAccountType(ctx context.Context, orgID, ledgerID, id string) (*models.AccountType, error) {
	return nil, nil
}

func (m *mockAccountTypesService) ListAccountTypes(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.AccountType], error) {
	return nil, nil
}

func (m *mockAccountTypesService) UpdateAccountType(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	return nil, nil
}

func (m *mockAccountTypesService) DeleteAccountType(ctx context.Context, orgID, ledgerID, id string) error {
	return nil
}

func (m *mockAccountTypesService) GetAccountTypesMetricsCount(ctx context.Context, orgID, ledgerID string) (*models.MetricsCount, error) {
	return nil, nil
}

func TestNewAccountTypeGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewAccountTypeGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewAccountTypeGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestAccountTypeGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewAccountTypeGenerator(nil, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Checking", AccountTypeKeyChecking, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAccountTypeGenerator_Generate_NilAccountTypesService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewAccountTypeGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Checking", AccountTypeKeyChecking, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAccountTypeGenerator_Generate_Success(t *testing.T) {
	testID := uuid.New()
	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			return &models.AccountType{
				ID:       testID,
				Name:     input.Name,
				KeyValue: input.KeyValue,
			}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)
	metadata := map[string]any{
		"category":  "deposit",
		"overdraft": false,
	}

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Checking", AccountTypeKeyChecking, metadata)
	require.NoError(t, err)
	assert.Equal(t, testID, result.ID)
	assert.Equal(t, "Checking", result.Name)
	assert.Equal(t, AccountTypeKeyChecking, result.KeyValue)
}

func TestAccountTypeGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			return nil, errors.New("account type creation failed")
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Checking", AccountTypeKeyChecking, nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "account type creation failed")
}

func TestAccountTypeGenerator_Generate_NilMetadata(t *testing.T) {
	var capturedInput *models.CreateAccountTypeInput

	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			capturedInput = input
			return &models.AccountType{ID: uuid.New()}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test", "TEST", nil)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
}

func TestAccountTypeGenerator_GenerateDefaults_NilEntity(t *testing.T) {
	gen := NewAccountTypeGenerator(nil, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	assert.Error(t, err)
	assert.Empty(t, results)
}

func TestAccountTypeGenerator_GenerateDefaults_Success(t *testing.T) {
	callCount := 0
	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			callCount++
			return &models.AccountType{
				ID:       uuid.New(),
				Name:     input.Name,
				KeyValue: input.KeyValue,
			}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	require.NoError(t, err)
	assert.Len(t, results, 7)
}

func TestAccountTypeGenerator_GenerateDefaults_PartialFailure(t *testing.T) {
	callCount := 0
	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			callCount++
			if callCount == 3 || callCount == 5 {
				return nil, errors.New("partial failure")
			}
			return &models.AccountType{
				ID:       uuid.New(),
				Name:     input.Name,
				KeyValue: input.KeyValue,
			}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "partial failure")
	assert.Len(t, results, 5)
}

func TestAccountTypeGenerator_GenerateDefaults_AllFailure(t *testing.T) {
	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			return nil, errors.New("all failed")
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	assert.Error(t, err)
	assert.Empty(t, results)
}

func TestAccountTypeGenerator_GenerateDefaults_VerifyInput(t *testing.T) {
	var createdTypes []struct {
		Name     string
		KeyValue string
		Meta     map[string]any
	}

	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			createdTypes = append(createdTypes, struct {
				Name     string
				KeyValue string
				Meta     map[string]any
			}{
				Name:     input.Name,
				KeyValue: input.KeyValue,
				Meta:     input.Metadata,
			})
			return &models.AccountType{ID: uuid.New(), Name: input.Name, KeyValue: input.KeyValue}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	_, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	require.NoError(t, err)

	assert.Len(t, createdTypes, 7)

	expectedDefaults := []struct {
		Name     string
		KeyValue string
	}{
		{"Checking", AccountTypeKeyChecking},
		{"Savings", AccountTypeKeySavings},
		{"Credit Card", AccountTypeKeyCreditCard},
		{"Expense", AccountTypeKeyExpense},
		{"Revenue", AccountTypeKeyRevenue},
		{"Liability", AccountTypeKeyLiability},
		{"Equity", AccountTypeKeyEquity},
	}

	for i, expected := range expectedDefaults {
		assert.Equal(t, expected.Name, createdTypes[i].Name)
		assert.Equal(t, expected.KeyValue, createdTypes[i].KeyValue)
		assert.NotNil(t, createdTypes[i].Meta)
	}
}

func TestAccountTypeGenerator_GenerateDefaults_Metadata(t *testing.T) {
	var metadataList []map[string]any

	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			metadataList = append(metadataList, input.Metadata)
			return &models.AccountType{ID: uuid.New()}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	_, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	require.NoError(t, err)

	assert.Equal(t, "deposit", metadataList[0]["category"])
	assert.Equal(t, false, metadataList[0]["overdraft"])

	assert.Equal(t, "savings", metadataList[1]["category"])
	assert.Equal(t, true, metadataList[1]["interest"])

	assert.Equal(t, "credit", metadataList[2]["category"])
	assert.Equal(t, true, metadataList[2]["limit_supported"])

	assert.Equal(t, "expense", metadataList[3]["category"])
	assert.Equal(t, "revenue", metadataList[4]["category"])
	assert.Equal(t, "liability", metadataList[5]["category"])
	assert.Equal(t, "equity", metadataList[6]["category"])
}

func TestAccountTypeConstants(t *testing.T) {
	assert.Equal(t, "CHECKING", AccountTypeKeyChecking)
	assert.Equal(t, "SAVINGS", AccountTypeKeySavings)
	assert.Equal(t, "CREDIT_CARD", AccountTypeKeyCreditCard)
	assert.Equal(t, "EXPENSE", AccountTypeKeyExpense)
	assert.Equal(t, "REVENUE", AccountTypeKeyRevenue)
	assert.Equal(t, "LIABILITY", AccountTypeKeyLiability)
	assert.Equal(t, "EQUITY", AccountTypeKeyEquity)
}

func TestAccountTypeGenerator_Generate_VerifyIDs(t *testing.T) {
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockAccountTypesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID
			return &models.AccountType{ID: uuid.New()}, nil
		},
	}

	e := &entities.Entity{
		AccountTypes: mockSvc,
	}

	gen := NewAccountTypeGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "test-org", "test-ledger", "Test", "TEST", nil)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
}
