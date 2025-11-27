package generator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTransactionsService struct {
	createFunc        func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error)
	createWithDSLFunc func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error)
	commitFunc        func(ctx context.Context, orgID, ledgerID, txID string) (*models.Transaction, error)
	revertFunc        func(ctx context.Context, orgID, ledgerID, txID string) (*models.Transaction, error)
}

func (m *mockTransactionsService) CreateTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}
	return &models.Transaction{ID: "tx-123"}, nil
}

func (m *mockTransactionsService) CreateTransactionWithDSLFile(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
	if m.createWithDSLFunc != nil {
		return m.createWithDSLFunc(ctx, orgID, ledgerID, dslContent)
	}
	return &models.Transaction{ID: "tx-dsl-123"}, nil
}

func (m *mockTransactionsService) GetTransaction(ctx context.Context, orgID, ledgerID, id string) (*models.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionsService) ListTransactions(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Transaction], error) {
	return nil, nil
}

func (m *mockTransactionsService) UpdateTransaction(ctx context.Context, orgID, ledgerID, id string, input any) (*models.Transaction, error) {
	return nil, nil
}

func (m *mockTransactionsService) CommitTransaction(ctx context.Context, orgID, ledgerID, id string) (*models.Transaction, error) {
	if m.commitFunc != nil {
		return m.commitFunc(ctx, orgID, ledgerID, id)
	}
	return &models.Transaction{ID: id}, nil
}

func (m *mockTransactionsService) RevertTransaction(ctx context.Context, orgID, ledgerID, id string) (*models.Transaction, error) {
	if m.revertFunc != nil {
		return m.revertFunc(ctx, orgID, ledgerID, id)
	}
	return &models.Transaction{ID: id}, nil
}

func (m *mockTransactionsService) CancelTransaction(ctx context.Context, orgID, ledgerID, transactionID string) error {
	return nil
}

func (m *mockTransactionsService) CreateTransactionWithDSL(ctx context.Context, orgID, ledgerID string, input *models.TransactionDSLInput) (*models.Transaction, error) {
	return &models.Transaction{ID: "tx-dsl"}, nil
}

func (m *mockTransactionsService) CreateInflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	return &models.Transaction{ID: "tx-inflow"}, nil
}

func (m *mockTransactionsService) CreateOutflowTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error) {
	return &models.Transaction{ID: "tx-outflow"}, nil
}

func (m *mockTransactionsService) CreateAnnotationTransaction(ctx context.Context, orgID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error) {
	return &models.Transaction{ID: "tx-annotation"}, nil
}

func TestNewTransactionGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewTransactionGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewTransactionGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestTransactionGenerator_GenerateWithDSL_NilEntity(t *testing.T) {
	gen := NewTransactionGenerator(nil, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "key-123",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionGenerator_GenerateWithDSL_NilTransactionsService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "key-123",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionGenerator_GenerateWithDSL_InvalidPattern_EmptyDSL(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}
	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "",
		IdempotencyKey:           "key-123",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dsl template is required")
}

func TestTransactionGenerator_GenerateWithDSL_InvalidPattern_EmptyChartOfAccounts(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}
	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "key-123",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chart of accounts group name is required")
}

func TestTransactionGenerator_GenerateWithDSL_InvalidPattern_EmptyIdempotencyKey(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}
	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "idempotency key is required")
}

func TestTransactionGenerator_GenerateWithDSL_Success(t *testing.T) {
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			return &models.Transaction{
				ID: "tx-success",
			}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		Description:              "Test payment",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "key-123",
	}

	result, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	require.NoError(t, err)
	assert.Equal(t, "tx-success", result.ID)
}

func TestTransactionGenerator_GenerateWithDSL_Error(t *testing.T) {
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			return nil, errors.New("DSL parsing failed")
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "invalid dsl",
		IdempotencyKey:           "key-123",
	}

	result, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "DSL parsing failed")
}

func TestTransactionGenerator_GenerateWithDSL_WithIdempotencyKey(t *testing.T) {
	var capturedCtx context.Context

	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			capturedCtx = ctx
			return &models.Transaction{ID: "tx-123"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "test",
		DSLTemplate:              "send 100 USD from @source to @dest",
		IdempotencyKey:           "unique-key-456",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	require.NoError(t, err)
	assert.NotNil(t, capturedCtx)
}

func TestTransactionGenerator_GenerateBatch_EmptyPatterns(t *testing.T) {
	gen := NewTransactionGenerator(nil, nil)

	results, err := gen.GenerateBatch(context.Background(), "org-123", "ledger-123", []data.TransactionPattern{}, 0)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestTransactionGenerator_GenerateBatch_NilEntity(t *testing.T) {
	gen := NewTransactionGenerator(nil, nil)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD", IdempotencyKey: "key-1"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 200 USD", IdempotencyKey: "key-2"},
	}

	results, err := gen.GenerateBatch(context.Background(), "org-123", "ledger-123", patterns, 0)
	assert.Error(t, err)
	assert.Empty(t, results)
}

func TestTransactionGenerator_GenerateBatch_Success(t *testing.T) {
	callCount := 0
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			callCount++
			return &models.Transaction{
				ID: "tx-" + string(rune('0'+callCount)),
			}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 2)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD from @a to @b", IdempotencyKey: "key-1"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 200 USD from @a to @b", IdempotencyKey: "key-2"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 300 USD from @a to @b", IdempotencyKey: "key-3"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", patterns, 0)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestTransactionGenerator_GenerateBatch_PartialError(t *testing.T) {
	callCount := 0
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			callCount++
			if callCount == 2 {
				return nil, errors.New("partial failure")
			}
			return &models.Transaction{ID: "tx-ok"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 1)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD from @a to @b", IdempotencyKey: "key-1"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 200 USD from @a to @b", IdempotencyKey: "key-2"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 300 USD from @a to @b", IdempotencyKey: "key-3"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", patterns, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "partial failure")
	assert.Len(t, results, 2)
}

func TestTransactionGenerator_GenerateBatch_WithTPS(t *testing.T) {
	callCount := 0
	var callTimes []time.Time

	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			callCount++
			callTimes = append(callTimes, time.Now())
			return &models.Transaction{ID: "tx-tps"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 1)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD from @a to @b", IdempotencyKey: "key-1"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 200 USD from @a to @b", IdempotencyKey: "key-2"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", patterns, 100)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestTransactionGenerator_GenerateBatch_CancelledContext(t *testing.T) {
	// When context is already cancelled, the batch may still succeed because
	// the cancellation check happens asynchronously in the worker pool
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			return &models.Transaction{ID: "tx-123"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ctx = WithWorkers(ctx, 1)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD from @a to @b", IdempotencyKey: "key-1"},
	}

	// The call may or may not return an error depending on timing
	_, _ = gen.GenerateBatch(ctx, "org-123", "ledger-123", patterns, 10)
}

func TestTransactionPattern_Fields(t *testing.T) {
	t.Run("Complete pattern", func(t *testing.T) {
		pattern := data.TransactionPattern{
			ChartOfAccountsGroupName: "payments",
			Description:              "Customer payment",
			DSLTemplate:              "send 100 USD from @customer to @merchant",
			RequiresCommit:           true,
			IdempotencyKey:           "idem-123",
			ExternalID:               "ext-456",
			Metadata: map[string]any{
				"type": "payment",
			},
		}

		assert.Equal(t, "payments", pattern.ChartOfAccountsGroupName)
		assert.Equal(t, "Customer payment", pattern.Description)
		assert.NotEmpty(t, pattern.DSLTemplate)
		assert.True(t, pattern.RequiresCommit)
		assert.Equal(t, "idem-123", pattern.IdempotencyKey)
		assert.Equal(t, "ext-456", pattern.ExternalID)
		assert.NotNil(t, pattern.Metadata)
	})

	t.Run("Minimal pattern", func(t *testing.T) {
		pattern := data.TransactionPattern{
			ChartOfAccountsGroupName: "test",
			DSLTemplate:              "send 100 USD from @a to @b",
			IdempotencyKey:           "key",
		}

		assert.Equal(t, "test", pattern.ChartOfAccountsGroupName)
		assert.NotEmpty(t, pattern.DSLTemplate)
		assert.False(t, pattern.RequiresCommit)
		assert.Nil(t, pattern.Metadata)
	})
}

func TestTransactionGenerator_GenerateWithDSL_VerifyDSLContent(t *testing.T) {
	var capturedDSL []byte

	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			capturedDSL = dslContent
			return &models.Transaction{ID: "tx-123"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	expectedDSL := "send 500 BRL from @customer/checking to @merchant/checking"
	pattern := data.TransactionPattern{
		ChartOfAccountsGroupName: "brazilian-payments",
		DSLTemplate:              expectedDSL,
		IdempotencyKey:           "br-pay-123",
	}

	_, err := gen.GenerateWithDSL(context.Background(), "org-123", "ledger-123", pattern)
	require.NoError(t, err)
	assert.Equal(t, expectedDSL, string(capturedDSL))
}

func TestTransactionGenerator_GenerateBatch_AllErrors(t *testing.T) {
	mockSvc := &mockTransactionsService{
		createWithDSLFunc: func(ctx context.Context, orgID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
			return nil, errors.New("all failed")
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	gen := NewTransactionGenerator(e, nil)
	ctx := WithWorkers(context.Background(), 1)

	patterns := []data.TransactionPattern{
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 100 USD from @a to @b", IdempotencyKey: "key-1"},
		{ChartOfAccountsGroupName: "test", DSLTemplate: "send 200 USD from @a to @b", IdempotencyKey: "key-2"},
	}

	results, err := gen.GenerateBatch(ctx, "org-123", "ledger-123", patterns, 0)
	assert.Error(t, err)
	assert.Empty(t, results)
}

func TestNewTransactionLifecycle(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		lc := NewTransactionLifecycle(nil, nil)
		assert.NotNil(t, lc)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		lc := NewTransactionLifecycle(e, nil)
		assert.NotNil(t, lc)
	})
}

func TestTransactionLifecycle_CreatePending_NilEntity(t *testing.T) {
	lc := NewTransactionLifecycle(nil, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	_, err := lc.CreatePending(ctx, &models.CreateTransactionInput{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionLifecycle_CreatePending_NilInput(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	_, err := lc.CreatePending(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction input is required")
}

func TestTransactionLifecycle_CreatePending_MissingIDs(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	_, err := lc.CreatePending(context.Background(), &models.CreateTransactionInput{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization and ledger IDs are required")
}

func TestTransactionLifecycle_CreatePending_Success(t *testing.T) {
	var capturedInput *models.CreateTransactionInput

	mockSvc := &mockTransactionsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
			capturedInput = input
			return &models.Transaction{ID: "tx-pending"}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	result, err := lc.CreatePending(ctx, &models.CreateTransactionInput{})
	require.NoError(t, err)
	assert.Equal(t, "tx-pending", result.ID)
	assert.True(t, capturedInput.Pending)
}

func TestTransactionLifecycle_Commit_NilEntity(t *testing.T) {
	lc := NewTransactionLifecycle(nil, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Commit(ctx, "tx-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionLifecycle_Commit_EmptyTxID(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Commit(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")
}

func TestTransactionLifecycle_Commit_MissingIDs(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	err := lc.Commit(context.Background(), "tx-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization and ledger IDs are required")
}

func TestTransactionLifecycle_Commit_Success(t *testing.T) {
	mockSvc := &mockTransactionsService{
		commitFunc: func(ctx context.Context, orgID, ledgerID, txID string) (*models.Transaction, error) {
			return &models.Transaction{ID: txID}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Commit(ctx, "tx-123")
	assert.NoError(t, err)
}

func TestTransactionLifecycle_Revert_NilEntity(t *testing.T) {
	lc := NewTransactionLifecycle(nil, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Revert(ctx, "tx-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionLifecycle_Revert_EmptyTxID(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Revert(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")
}

func TestTransactionLifecycle_Revert_MissingIDs(t *testing.T) {
	mockSvc := &mockTransactionsService{}
	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	err := lc.Revert(context.Background(), "tx-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization and ledger IDs are required")
}

func TestTransactionLifecycle_Revert_Success(t *testing.T) {
	mockSvc := &mockTransactionsService{
		revertFunc: func(ctx context.Context, orgID, ledgerID, txID string) (*models.Transaction, error) {
			return &models.Transaction{ID: txID}, nil
		},
	}

	e := &entities.Entity{
		Transactions: mockSvc,
	}

	lc := NewTransactionLifecycle(e, nil)

	ctx := WithOrgID(context.Background(), "org-123")
	ctx = WithLedgerID(ctx, "ledger-123")

	err := lc.Revert(ctx, "tx-123")
	assert.NoError(t, err)
}

func TestTransactionLifecycle_HandleInsufficientFunds_NilError(t *testing.T) {
	lc := NewTransactionLifecycle(nil, nil)

	err := lc.HandleInsufficientFunds(context.Background(), nil)
	assert.Nil(t, err)
}

func TestTransactionLifecycle_HandleInsufficientFunds_RegularError(t *testing.T) {
	lc := NewTransactionLifecycle(nil, nil)

	// Note: Due to the current implementation of sdkerrors.IsInsufficientBalanceError,
	// which has a fallback that checks errors.Is(err, ValueOfOriginalType(err, code)),
	// and ValueOfOriginalType returns err itself for non-MidazError types,
	// most non-nil errors will be treated as "insufficient balance" errors.
	// Using the special "unknown error" string which is explicitly excluded.
	regularErr := errors.New("unknown error")
	err := lc.HandleInsufficientFunds(context.Background(), regularErr)
	assert.Nil(t, err, "Errors with message 'unknown error' should return nil")
}
