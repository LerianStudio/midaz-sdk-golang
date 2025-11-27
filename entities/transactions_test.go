package entities

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

// ========== Test Data Helpers ==========

func createTestTransaction(id, orgID, ledgerID, assetCode, amount, statusCode string) *models.Transaction {
	return &models.Transaction{
		ID:             id,
		Description:    "Test transaction",
		AssetCode:      assetCode,
		Amount:         amount,
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Route:          "payment",
		Pending:        false,
		Source:         []string{"source-account"},
		Destination:    []string{"dest-account"},
		Status: models.Status{
			Code: statusCode,
		},
		Metadata: map[string]any{
			"key": "value",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestTransactionList(orgID, ledgerID string) *models.ListResponse[models.Transaction] {
	return &models.ListResponse[models.Transaction]{
		Items: []models.Transaction{
			*createTestTransaction("tx-001", orgID, ledgerID, "USD", "100.00", "COMPLETED"),
			*createTestTransaction("tx-002", orgID, ledgerID, "EUR", "200.00", "PENDING"),
		},
		Pagination: models.Pagination{
			Total:  2,
			Limit:  10,
			Offset: 0,
		},
	}
}

func createTestTransactionInput() *models.CreateTransactionInput {
	return &models.CreateTransactionInput{
		AssetCode:   "USD",
		Amount:      "100",
		Description: "Test payment",
		Send: &models.SendInput{
			Asset: "USD",
			Value: "100",
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{Account: "source-account", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{Account: "dest-account", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
				},
			},
		},
	}
}

func createTestDSLInput() *models.TransactionDSLInput {
	return &models.TransactionDSLInput{
		Description: "DSL Transaction",
		Send: &models.DSLSend{
			Asset: "USD",
			Value: "100",
			Source: &models.DSLSource{
				From: []models.DSLFromTo{{Account: "source-account"}},
			},
			Distribute: &models.DSLDistribute{
				To: []models.DSLFromTo{{Account: "dest-account"}},
			},
		},
	}
}

// ========== TestListTransactions ==========

func TestListTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"

	txList := createTestTransactionList(orgID, ledgerID)

	// Test successful list with default options
	mockService.EXPECT().
		ListTransactions(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(txList, nil)

	result, err := mockService.ListTransactions(ctx, orgID, ledgerID, nil)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "tx-001", result.Items[0].ID)
	assert.Equal(t, "USD", result.Items[0].AssetCode)
	assert.Equal(t, "COMPLETED", result.Items[0].Status.Code)

	// Test with pagination options
	paginatedList := &models.ListResponse[models.Transaction]{
		Items: []models.Transaction{
			*createTestTransaction("tx-003", orgID, ledgerID, "GBP", "300.00", "COMPLETED"),
		},
		Pagination: models.Pagination{
			Total:  11,
			Limit:  5,
			Offset: 10,
		},
	}

	mockService.EXPECT().
		ListTransactions(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(paginatedList, nil)

	result, err = mockService.ListTransactions(ctx, orgID, ledgerID, &models.ListOptions{Limit: 5, Offset: 10})
	assert.NoError(t, err)
	assert.Equal(t, 11, result.Pagination.Total)
	assert.Len(t, result.Items, 1)

	// Test empty organization ID validation
	mockService.EXPECT().
		ListTransactions(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.ListTransactions(ctx, "", ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID validation
	mockService.EXPECT().
		ListTransactions(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.ListTransactions(ctx, orgID, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty list response
	emptyList := &models.ListResponse[models.Transaction]{
		Items:      []models.Transaction{},
		Pagination: models.Pagination{Total: 0, Limit: 10, Offset: 0},
	}

	mockService.EXPECT().
		ListTransactions(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(emptyList, nil)

	result, err = mockService.ListTransactions(ctx, orgID, ledgerID, nil)
	assert.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.Equal(t, 0, result.Pagination.Total)

	// Test server error
	mockService.EXPECT().
		ListTransactions(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("internal server error"))

	_, err = mockService.ListTransactions(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal server error")
}

// ========== TestGetTransaction ==========

func TestGetTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	transactionID := "tx-789"

	tx := createTestTransaction(transactionID, orgID, ledgerID, "USD", "100.00", "COMPLETED")

	// Test successful get
	mockService.EXPECT().
		GetTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(tx, nil)

	result, err := mockService.GetTransaction(ctx, orgID, ledgerID, transactionID)
	assert.NoError(t, err)
	assert.Equal(t, transactionID, result.ID)
	assert.Equal(t, "USD", result.AssetCode)
	assert.Equal(t, "100.00", result.Amount)
	assert.Equal(t, "COMPLETED", result.Status.Code)

	// Test empty organization ID
	mockService.EXPECT().
		GetTransaction(gomock.Any(), "", ledgerID, transactionID).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.GetTransaction(ctx, "", ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		GetTransaction(gomock.Any(), orgID, "", transactionID).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.GetTransaction(ctx, orgID, "", transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty transaction ID
	mockService.EXPECT().
		GetTransaction(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("transaction ID is required"))

	_, err = mockService.GetTransaction(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")

	// Test transaction not found
	mockService.EXPECT().
		GetTransaction(gomock.Any(), orgID, ledgerID, "nonexistent-tx").
		Return(nil, fmt.Errorf("transaction not found"))

	_, err = mockService.GetTransaction(ctx, orgID, ledgerID, "nonexistent-tx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========== TestCreateTransaction ==========

func TestCreateTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	input := createTestTransactionInput()

	tx := createTestTransaction("tx-new-001", orgID, ledgerID, "USD", "100", "COMPLETED")

	// Test successful create
	mockService.EXPECT().
		CreateTransaction(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateTransaction(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, "tx-new-001", result.ID)
	assert.Equal(t, "USD", result.AssetCode)

	// Test empty organization ID
	mockService.EXPECT().
		CreateTransaction(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateTransaction(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateTransaction(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateTransaction(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test nil input
	mockService.EXPECT().
		CreateTransaction(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.CreateTransaction(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")

	// Test validation error from server
	mockService.EXPECT().
		CreateTransaction(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("transaction not balanced"))

	_, err = mockService.CreateTransaction(ctx, orgID, ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not balanced")
}

// ========== TestCreateTransactionWithDSL ==========

func TestCreateTransactionWithDSL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	input := createTestDSLInput()

	tx := createTestTransaction("tx-dsl-001", orgID, ledgerID, "USD", "100", "COMPLETED")

	// Test successful create
	mockService.EXPECT().
		CreateTransactionWithDSL(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateTransactionWithDSL(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, "tx-dsl-001", result.ID)

	// Test empty organization ID
	mockService.EXPECT().
		CreateTransactionWithDSL(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateTransactionWithDSL(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateTransactionWithDSL(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateTransactionWithDSL(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test nil input
	mockService.EXPECT().
		CreateTransactionWithDSL(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.CreateTransactionWithDSL(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")
}

// ========== TestCreateTransactionWithDSLFile ==========

func TestCreateTransactionWithDSLFile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	dslContent := []byte("send { asset: USD, value: 100 } distribute { to: [dest-account] }")

	tx := createTestTransaction("tx-dsl-file-001", orgID, ledgerID, "USD", "100", "COMPLETED")

	// Test successful create
	mockService.EXPECT().
		CreateTransactionWithDSLFile(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateTransactionWithDSLFile(ctx, orgID, ledgerID, dslContent)
	assert.NoError(t, err)
	assert.Equal(t, "tx-dsl-file-001", result.ID)

	// Test empty organization ID
	mockService.EXPECT().
		CreateTransactionWithDSLFile(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateTransactionWithDSLFile(ctx, "", ledgerID, dslContent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateTransactionWithDSLFile(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateTransactionWithDSLFile(ctx, orgID, "", dslContent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty DSL content
	mockService.EXPECT().
		CreateTransactionWithDSLFile(gomock.Any(), orgID, ledgerID, []byte("")).
		Return(nil, fmt.Errorf("DSL content is required"))

	_, err = mockService.CreateTransactionWithDSLFile(ctx, orgID, ledgerID, []byte(""))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DSL content is required")
}

// ========== TestUpdateTransaction ==========

func TestUpdateTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	transactionID := "tx-789"
	input := &models.UpdateTransactionInput{
		Description: "Updated description",
		Metadata:    map[string]any{"updated": true},
	}

	tx := createTestTransaction(transactionID, orgID, ledgerID, "USD", "100", "COMPLETED")
	tx.Description = "Updated description"
	tx.Metadata = map[string]any{"updated": true}

	// Test successful update
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), orgID, ledgerID, transactionID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.UpdateTransaction(ctx, orgID, ledgerID, transactionID, input)
	assert.NoError(t, err)
	assert.Equal(t, transactionID, result.ID)
	assert.Equal(t, "Updated description", result.Description)

	// Test empty organization ID
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), "", ledgerID, transactionID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.UpdateTransaction(ctx, "", ledgerID, transactionID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), orgID, "", transactionID, gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.UpdateTransaction(ctx, orgID, "", transactionID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty transaction ID
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), orgID, ledgerID, "", gomock.Any()).
		Return(nil, fmt.Errorf("transaction ID is required"))

	_, err = mockService.UpdateTransaction(ctx, orgID, ledgerID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")

	// Test nil input
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), orgID, ledgerID, transactionID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.UpdateTransaction(ctx, orgID, ledgerID, transactionID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")

	// Test not found error
	mockService.EXPECT().
		UpdateTransaction(gomock.Any(), orgID, ledgerID, "nonexistent-tx", gomock.Any()).
		Return(nil, fmt.Errorf("transaction not found"))

	_, err = mockService.UpdateTransaction(ctx, orgID, ledgerID, "nonexistent-tx", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ========== TestCommitTransaction ==========

func TestCommitTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	transactionID := "tx-789"

	tx := createTestTransaction(transactionID, orgID, ledgerID, "USD", "100", "COMPLETED")
	tx.Pending = false

	// Test successful commit
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(tx, nil)

	result, err := mockService.CommitTransaction(ctx, orgID, ledgerID, transactionID)
	assert.NoError(t, err)
	assert.Equal(t, transactionID, result.ID)
	assert.Equal(t, "COMPLETED", result.Status.Code)
	assert.False(t, result.Pending)

	// Test empty organization ID
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), "", ledgerID, transactionID).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CommitTransaction(ctx, "", ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), orgID, "", transactionID).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CommitTransaction(ctx, orgID, "", transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty transaction ID
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("transaction ID is required"))

	_, err = mockService.CommitTransaction(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")

	// Test not found error
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), orgID, ledgerID, "nonexistent-tx").
		Return(nil, fmt.Errorf("transaction not found"))

	_, err = mockService.CommitTransaction(ctx, orgID, ledgerID, "nonexistent-tx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test already committed error
	mockService.EXPECT().
		CommitTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(nil, fmt.Errorf("transaction already committed"))

	_, err = mockService.CommitTransaction(ctx, orgID, ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already committed")
}

// ========== TestCancelTransaction ==========

func TestCancelTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	transactionID := "tx-789"

	// Test successful cancel
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(nil)

	err := mockService.CancelTransaction(ctx, orgID, ledgerID, transactionID)
	assert.NoError(t, err)

	// Test empty organization ID
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), "", ledgerID, transactionID).
		Return(fmt.Errorf("organization ID is required"))

	err = mockService.CancelTransaction(ctx, "", ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), orgID, "", transactionID).
		Return(fmt.Errorf("ledger ID is required"))

	err = mockService.CancelTransaction(ctx, orgID, "", transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty transaction ID
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), orgID, ledgerID, "").
		Return(fmt.Errorf("transaction ID is required"))

	err = mockService.CancelTransaction(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")

	// Test not found error
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), orgID, ledgerID, "nonexistent-tx").
		Return(fmt.Errorf("transaction not found"))

	err = mockService.CancelTransaction(ctx, orgID, ledgerID, "nonexistent-tx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test already committed error
	mockService.EXPECT().
		CancelTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(fmt.Errorf("cannot cancel committed transaction"))

	err = mockService.CancelTransaction(ctx, orgID, ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel")
}

// ========== TestRevertTransaction ==========

func TestRevertTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"
	transactionID := "tx-789"

	revertTx := createTestTransaction("tx-revert-001", orgID, ledgerID, "USD", "100", "COMPLETED")
	revertTx.Description = "Revert of tx-789"

	// Test successful revert
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(revertTx, nil)

	result, err := mockService.RevertTransaction(ctx, orgID, ledgerID, transactionID)
	assert.NoError(t, err)
	assert.Equal(t, "tx-revert-001", result.ID)
	assert.Contains(t, result.Description, "Revert")

	// Test empty organization ID
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), "", ledgerID, transactionID).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.RevertTransaction(ctx, "", ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), orgID, "", transactionID).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.RevertTransaction(ctx, orgID, "", transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test empty transaction ID
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), orgID, ledgerID, "").
		Return(nil, fmt.Errorf("transaction ID is required"))

	_, err = mockService.RevertTransaction(ctx, orgID, ledgerID, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction ID is required")

	// Test not found error
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), orgID, ledgerID, "nonexistent-tx").
		Return(nil, fmt.Errorf("transaction not found"))

	_, err = mockService.RevertTransaction(ctx, orgID, ledgerID, "nonexistent-tx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test already reverted error
	mockService.EXPECT().
		RevertTransaction(gomock.Any(), orgID, ledgerID, transactionID).
		Return(nil, fmt.Errorf("transaction already reverted"))

	_, err = mockService.RevertTransaction(ctx, orgID, ledgerID, transactionID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already reverted")
}

// ========== TestCreateInflowTransaction ==========

func TestCreateInflowTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"

	input := models.NewCreateInflowInput("USD", "100", &models.DistributeInput{
		To: []models.FromToInput{
			{Account: "dest-account", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		},
	}).WithDescription("Deposit")

	tx := createTestTransaction("tx-inflow-001", orgID, ledgerID, "USD", "100", "COMPLETED")
	tx.Description = "Deposit"

	// Test successful create
	mockService.EXPECT().
		CreateInflowTransaction(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateInflowTransaction(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, "tx-inflow-001", result.ID)
	assert.Equal(t, "Deposit", result.Description)

	// Test empty organization ID
	mockService.EXPECT().
		CreateInflowTransaction(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateInflowTransaction(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateInflowTransaction(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateInflowTransaction(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test nil input
	mockService.EXPECT().
		CreateInflowTransaction(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.CreateInflowTransaction(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")
}

// ========== TestCreateOutflowTransaction ==========

func TestCreateOutflowTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"

	input := models.NewCreateOutflowInput("USD", "100", &models.SourceInput{
		From: []models.FromToInput{
			{Account: "source-account", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
		},
	}).WithDescription("Withdrawal")

	tx := createTestTransaction("tx-outflow-001", orgID, ledgerID, "USD", "100", "COMPLETED")
	tx.Description = "Withdrawal"

	// Test successful create
	mockService.EXPECT().
		CreateOutflowTransaction(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateOutflowTransaction(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, "tx-outflow-001", result.ID)
	assert.Equal(t, "Withdrawal", result.Description)

	// Test empty organization ID
	mockService.EXPECT().
		CreateOutflowTransaction(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateOutflowTransaction(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateOutflowTransaction(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateOutflowTransaction(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test nil input
	mockService.EXPECT().
		CreateOutflowTransaction(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.CreateOutflowTransaction(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")
}

// ========== TestCreateAnnotationTransaction ==========

func TestCreateAnnotationTransaction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockTransactionsService(ctrl)

	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-456"

	input := models.NewCreateAnnotationInput("Annotation note").
		WithCode("ANN-001").
		WithMetadata(map[string]any{"type": "note"})

	tx := createTestTransaction("tx-annotation-001", orgID, ledgerID, "", "0", "COMPLETED")
	tx.Description = "Annotation note"
	tx.Metadata = map[string]any{"type": "note"}

	// Test successful create
	mockService.EXPECT().
		CreateAnnotationTransaction(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(tx, nil)

	result, err := mockService.CreateAnnotationTransaction(ctx, orgID, ledgerID, input)
	assert.NoError(t, err)
	assert.Equal(t, "tx-annotation-001", result.ID)
	assert.Equal(t, "Annotation note", result.Description)

	// Test empty organization ID
	mockService.EXPECT().
		CreateAnnotationTransaction(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, fmt.Errorf("organization ID is required"))

	_, err = mockService.CreateAnnotationTransaction(ctx, "", ledgerID, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test empty ledger ID
	mockService.EXPECT().
		CreateAnnotationTransaction(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, fmt.Errorf("ledger ID is required"))

	_, err = mockService.CreateAnnotationTransaction(ctx, orgID, "", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test nil input
	mockService.EXPECT().
		CreateAnnotationTransaction(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, fmt.Errorf("input is required"))

	_, err = mockService.CreateAnnotationTransaction(ctx, orgID, ledgerID, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is required")

	// Test empty description validation
	invalidInput := &models.CreateAnnotationInput{Description: ""}
	mockService.EXPECT().
		CreateAnnotationTransaction(gomock.Any(), orgID, ledgerID, invalidInput).
		Return(nil, fmt.Errorf("description is required"))

	_, err = mockService.CreateAnnotationTransaction(ctx, orgID, ledgerID, invalidInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "description is required")
}

// ========== TestTransactionInputValidation ==========

func TestTransactionInputValidation(t *testing.T) {
	t.Run("CreateTransactionInput validation", func(t *testing.T) {
		// Test empty amount
		input := &models.CreateTransactionInput{
			AssetCode: "USD",
			Amount:    "",
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "amount")

		// Test zero amount
		input = &models.CreateTransactionInput{
			AssetCode: "USD",
			Amount:    "0",
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "amount")

		// Test empty asset code
		input = &models.CreateTransactionInput{
			AssetCode: "",
			Amount:    "100",
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "assetCode")

		// Test missing operations and send
		input = &models.CreateTransactionInput{
			AssetCode: "USD",
			Amount:    "100",
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operations or send")
	})

	t.Run("UpdateTransactionInput validation", func(t *testing.T) {
		// Test valid input
		input := models.NewUpdateTransactionInput().
			WithDescription("Test description").
			WithMetadata(map[string]any{"key": "value"})
		err := input.Validate()
		assert.NoError(t, err)

		// Test description too long
		longDesc := ""
		for i := 0; i < 300; i++ {
			longDesc += "a"
		}
		input = &models.UpdateTransactionInput{Description: longDesc}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "description")
	})

	t.Run("CreateInflowInput validation", func(t *testing.T) {
		// Test nil send
		input := &models.CreateInflowInput{}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "send is required")

		// Test empty asset
		input = &models.CreateInflowInput{
			Send: &models.SendInflowInput{
				Value: "100",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "asset is required")

		// Test zero value
		input = &models.CreateInflowInput{
			Send: &models.SendInflowInput{
				Asset: "USD",
				Value: "0",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be greater than zero")

		// Test missing distribute
		input = &models.CreateInflowInput{
			Send: &models.SendInflowInput{
				Asset: "USD",
				Value: "100",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "distribute")
	})

	t.Run("CreateOutflowInput validation", func(t *testing.T) {
		// Test nil send
		input := &models.CreateOutflowInput{}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "send is required")

		// Test empty asset
		input = &models.CreateOutflowInput{
			Send: &models.SendOutflowInput{
				Value: "100",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "asset is required")

		// Test zero value
		input = &models.CreateOutflowInput{
			Send: &models.SendOutflowInput{
				Asset: "USD",
				Value: "0",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be greater than zero")

		// Test missing source
		input = &models.CreateOutflowInput{
			Send: &models.SendOutflowInput{
				Asset: "USD",
				Value: "100",
			},
		}
		err = input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source")
	})

	t.Run("CreateAnnotationInput validation", func(t *testing.T) {
		// Test empty description
		input := &models.CreateAnnotationInput{Description: ""}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "description is required")

		// Test valid input
		input = models.NewCreateAnnotationInput("Test annotation")
		err = input.Validate()
		assert.NoError(t, err)
	})
}

// ========== TestTransactionDSLInputValidation ==========

func TestTransactionDSLInputValidation(t *testing.T) {
	t.Run("Valid DSL input", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Description: "Test DSL transaction",
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "100",
				Source: &models.DSLSource{
					From: []models.DSLFromTo{{Account: "source"}},
				},
				Distribute: &models.DSLDistribute{
					To: []models.DSLFromTo{{Account: "dest"}},
				},
			},
		}
		err := input.Validate()
		assert.NoError(t, err)
	})

	t.Run("Nil send", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Description: "Test",
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "send is required")
	})

	t.Run("Empty asset", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Send: &models.DSLSend{
				Value: "100",
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "asset is required")
	})

	t.Run("Zero value", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "0",
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value must be greater than 0")
	})

	t.Run("Missing source", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "100",
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source")
	})

	t.Run("Empty source from", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "100",
				Source: &models.DSLSource{
					From: []models.DSLFromTo{},
				},
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source.from")
	})

	t.Run("Missing distribute", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "100",
				Source: &models.DSLSource{
					From: []models.DSLFromTo{{Account: "source"}},
				},
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "distribute")
	})

	t.Run("Description too long", func(t *testing.T) {
		longDesc := ""
		for i := 0; i < 300; i++ {
			longDesc += "a"
		}
		input := &models.TransactionDSLInput{
			Description: longDesc,
			Send: &models.DSLSend{
				Asset: "USD",
				Value: "100",
				Source: &models.DSLSource{
					From: []models.DSLFromTo{{Account: "source"}},
				},
				Distribute: &models.DSLDistribute{
					To: []models.DSLFromTo{{Account: "dest"}},
				},
			},
		}
		err := input.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "description")
	})
}

// ========== TestTransactionMapConversion ==========

func TestTransactionMapConversion(t *testing.T) {
	t.Run("CreateTransactionInput ToLibTransaction", func(t *testing.T) {
		input := &models.CreateTransactionInput{
			ChartOfAccountsGroupName: "assets",
			Description:              "Test",
			Pending:                  true,
			Route:                    "payment",
			Metadata:                 map[string]any{"key": "value"},
			Send: &models.SendInput{
				Asset: "USD",
				Value: "100",
				Source: &models.SourceInput{
					From: []models.FromToInput{
						{Account: "source", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
					},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{
						{Account: "dest", Amount: models.AmountInput{Asset: "USD", Value: "100"}},
					},
				},
			},
		}

		result := input.ToLibTransaction()
		assert.NotNil(t, result)
		assert.Equal(t, "assets", result["chartOfAccountsGroupName"])
		assert.Equal(t, "Test", result["description"])
		assert.Equal(t, true, result["pending"])
		assert.Equal(t, "payment", result["route"])
		assert.NotNil(t, result["send"])
		assert.NotNil(t, result["metadata"])
	})

	t.Run("TransactionDSLInput ToTransactionMap", func(t *testing.T) {
		input := &models.TransactionDSLInput{
			ChartOfAccountsGroupName: "liabilities",
			Description:              "DSL Test",
			Code:                     "TXN001",
			Pending:                  true,
			Metadata:                 map[string]any{"type": "transfer"},
			Send: &models.DSLSend{
				Asset: "EUR",
				Value: "500",
				Source: &models.DSLSource{
					From: []models.DSLFromTo{{Account: "source-acc"}},
				},
				Distribute: &models.DSLDistribute{
					To: []models.DSLFromTo{{Account: "dest-acc"}},
				},
			},
		}

		result := input.ToTransactionMap()
		assert.NotNil(t, result)
		assert.Equal(t, "liabilities", result["chartOfAccountsGroupName"])
		assert.Equal(t, "DSL Test", result["description"])
		assert.Equal(t, "TXN001", result["code"])
		assert.Equal(t, true, result["pending"])
		assert.NotNil(t, result["send"])
	})

	t.Run("Nil input returns nil", func(t *testing.T) {
		var input *models.CreateTransactionInput
		result := input.ToLibTransaction()
		assert.Nil(t, result)

		var dslInput *models.TransactionDSLInput
		dslResult := dslInput.ToTransactionMap()
		assert.Nil(t, dslResult)
	})
}

// ========== TestTransactionBuilderMethods ==========

func TestTransactionBuilderMethods(t *testing.T) {
	t.Run("CreateTransactionInput builder", func(t *testing.T) {
		input := models.NewCreateTransactionInput("USD", "100").
			WithDescription("Test payment").
			WithMetadata(map[string]any{"key": "value"}).
			WithExternalID("ext-123").
			WithSend(&models.SendInput{
				Asset: "USD",
				Value: "100",
				Source: &models.SourceInput{
					From: []models.FromToInput{{Account: "src", Amount: models.AmountInput{Asset: "USD", Value: "100"}}},
				},
				Distribute: &models.DistributeInput{
					To: []models.FromToInput{{Account: "dst", Amount: models.AmountInput{Asset: "USD", Value: "100"}}},
				},
			})

		assert.Equal(t, "USD", input.AssetCode)
		assert.Equal(t, "100", input.Amount)
		assert.Equal(t, "Test payment", input.Description)
		assert.Equal(t, "ext-123", input.ExternalID)
		assert.NotNil(t, input.Metadata)
		assert.NotNil(t, input.Send)
	})

	t.Run("UpdateTransactionInput builder", func(t *testing.T) {
		input := models.NewUpdateTransactionInput().
			WithDescription("Updated").
			WithMetadata(map[string]any{"updated": true}).
			WithExternalID("ext-456")

		assert.Equal(t, "Updated", input.Description)
		assert.Equal(t, "ext-456", input.ExternalID)
		assert.NotNil(t, input.Metadata)
	})

	t.Run("CreateInflowInput builder", func(t *testing.T) {
		input := models.NewCreateInflowInput("USD", "100", &models.DistributeInput{
			To: []models.FromToInput{{Account: "dest", Amount: models.AmountInput{Asset: "USD", Value: "100"}}},
		}).
			WithDescription("Deposit").
			WithCode("DEP-001").
			WithMetadata(map[string]any{"source": "external"}).
			WithChartOfAccountsGroupName("assets").
			WithRoute("deposit")

		assert.Equal(t, "Deposit", input.Description)
		assert.Equal(t, "DEP-001", input.Code)
		assert.Equal(t, "assets", input.ChartOfAccountsGroupName)
		assert.Equal(t, "deposit", input.Route)
		assert.NotNil(t, input.Metadata)
	})

	t.Run("CreateOutflowInput builder", func(t *testing.T) {
		input := models.NewCreateOutflowInput("USD", "100", &models.SourceInput{
			From: []models.FromToInput{{Account: "source", Amount: models.AmountInput{Asset: "USD", Value: "100"}}},
		}).
			WithDescription("Withdrawal").
			WithCode("WTH-001").
			WithMetadata(map[string]any{"destination": "external"}).
			WithChartOfAccountsGroupName("liabilities").
			WithRoute("withdrawal")

		assert.Equal(t, "Withdrawal", input.Description)
		assert.Equal(t, "WTH-001", input.Code)
		assert.Equal(t, "liabilities", input.ChartOfAccountsGroupName)
		assert.Equal(t, "withdrawal", input.Route)
		assert.NotNil(t, input.Metadata)
	})

	t.Run("CreateAnnotationInput builder", func(t *testing.T) {
		input := models.NewCreateAnnotationInput("Note").
			WithCode("NOTE-001").
			WithMetadata(map[string]any{"type": "comment"}).
			WithChartOfAccountsGroupName("annotations")

		assert.Equal(t, "Note", input.Description)
		assert.Equal(t, "NOTE-001", input.Code)
		assert.Equal(t, "annotations", input.ChartOfAccountsGroupName)
		assert.NotNil(t, input.Metadata)
	})
}
