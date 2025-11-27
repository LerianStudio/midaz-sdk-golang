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

type mockTransactionRoutesService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error)
}

func (m *mockTransactionRoutesService) CreateTransactionRoute(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}
	return &models.TransactionRoute{Title: input.Title}, nil
}

func (m *mockTransactionRoutesService) GetTransactionRoute(ctx context.Context, orgID, ledgerID, id string) (*models.TransactionRoute, error) {
	return nil, nil
}

func (m *mockTransactionRoutesService) ListTransactionRoutes(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.TransactionRoute], error) {
	return nil, nil
}

func (m *mockTransactionRoutesService) UpdateTransactionRoute(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error) {
	return nil, nil
}

func (m *mockTransactionRoutesService) DeleteTransactionRoute(ctx context.Context, orgID, ledgerID, id string) error {
	return nil
}

func TestNewTransactionRouteGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewTransactionRouteGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewTransactionRouteGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestTransactionRouteGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewTransactionRouteGenerator(nil, nil)

	input := models.NewCreateTransactionRouteInput(
		"Test Route",
		"Test description",
		[]string{"op-route-1", "op-route-2"},
	)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionRouteGenerator_Generate_NilTransactionRoutesService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewTransactionRouteGenerator(e, nil)

	input := models.NewCreateTransactionRouteInput(
		"Test Route",
		"Test description",
		[]string{"op-route-1", "op-route-2"},
	)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestTransactionRouteGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			return &models.TransactionRoute{
				Title: input.Title,
			}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	input := models.NewCreateTransactionRouteInput(
		"Payment Flow",
		"Customer pays merchant",
		[]string{"op-route-1", "op-route-2"},
	).WithMetadata(map[string]any{"pattern": "payment"})

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	require.NoError(t, err)
	assert.Equal(t, "Payment Flow", result.Title)
}

func TestTransactionRouteGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			return nil, errors.New("transaction route creation failed")
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	input := models.NewCreateTransactionRouteInput(
		"Test Route",
		"Test description",
		[]string{"op-route-1"},
	)

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "transaction route creation failed")
}

func TestTransactionRouteGenerator_GenerateDefaults_EmptyOpRoutes(t *testing.T) {
	mockSvc := &mockTransactionRoutesService{}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", []*models.OperationRoute{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestTransactionRouteGenerator_GenerateDefaults_WithValidOpRoutes(t *testing.T) {
	var createdRoutes []string
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			createdRoutes = append(createdRoutes, input.Title)
			return &models.TransactionRoute{
				Title: input.Title,
			}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Source: Customer (CHECKING)"},
		{ID: uuid.New(), Title: "Source: Merchant (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Merchant (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Platform Fee (alias)"},
		{ID: uuid.New(), Title: "Destination: Settlement Pool (alias)"},
		{ID: uuid.New(), Title: "Destination: Customer (CHECKING)"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Contains(t, createdRoutes, "Payment Flow")
	assert.Contains(t, createdRoutes, "Refund Flow")
	assert.Contains(t, createdRoutes, "Transfer Flow")
}

func TestTransactionRouteGenerator_GenerateDefaults_PaymentFlowOnly(t *testing.T) {
	var createdRoutes []string
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			createdRoutes = append(createdRoutes, input.Title)
			return &models.TransactionRoute{Title: input.Title}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Source: Customer (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Merchant (CHECKING)"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, createdRoutes, "Payment Flow")
}

func TestTransactionRouteGenerator_GenerateDefaults_Error(t *testing.T) {
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			return nil, errors.New("defaults creation failed")
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Source: Customer (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Merchant (CHECKING)"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "defaults creation failed")
}

func TestTransactionRouteGenerator_Generate_VerifyIDs(t *testing.T) {
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID
			return &models.TransactionRoute{}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	input := models.NewCreateTransactionRouteInput(
		"Test Route",
		"Test description",
		[]string{"op-route-1"},
	)

	_, err := gen.Generate(context.Background(), "test-org", "test-ledger", input)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
}

func TestTransactionRouteGenerator_GenerateDefaults_MissingRoutes(t *testing.T) {
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			return &models.TransactionRoute{Title: input.Title}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Unknown Route"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestTransactionRouteGenerator_GenerateDefaults_RefundFlow(t *testing.T) {
	var createdRoutes []string
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			createdRoutes = append(createdRoutes, input.Title)
			return &models.TransactionRoute{Title: input.Title}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Source: Merchant (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Customer (CHECKING)"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, createdRoutes, "Refund Flow")
}

func TestTransactionRouteGenerator_GenerateDefaults_TransferFlow(t *testing.T) {
	var createdRoutes []string
	mockSvc := &mockTransactionRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
			createdRoutes = append(createdRoutes, input.Title)
			return &models.TransactionRoute{Title: input.Title}, nil
		},
	}

	e := &entities.Entity{
		TransactionRoutes: mockSvc,
	}

	gen := NewTransactionRouteGenerator(e, nil)

	opRoutes := []*models.OperationRoute{
		{ID: uuid.New(), Title: "Source: Customer (CHECKING)"},
		{ID: uuid.New(), Title: "Destination: Customer (CHECKING)"},
	}

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123", opRoutes)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, createdRoutes, "Transfer Flow")
}
