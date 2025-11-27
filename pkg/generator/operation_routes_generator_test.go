package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOperationRoutesService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error)
}

func (m *mockOperationRoutesService) CreateOperationRoute(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}
	return &models.OperationRoute{Title: input.Title}, nil
}

func (m *mockOperationRoutesService) GetOperationRoute(ctx context.Context, orgID, ledgerID, id string) (*models.OperationRoute, error) {
	return nil, nil
}

func (m *mockOperationRoutesService) ListOperationRoutes(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.OperationRoute], error) {
	return nil, nil
}

func (m *mockOperationRoutesService) UpdateOperationRoute(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error) {
	return nil, nil
}

func (m *mockOperationRoutesService) DeleteOperationRoute(ctx context.Context, orgID, ledgerID, id string) error {
	return nil
}

func TestNewOperationRouteGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewOperationRouteGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewOperationRouteGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestOperationRouteGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewOperationRouteGenerator(nil, nil)

	input := models.NewCreateOperationRouteInput(
		"Test Route",
		"Test description",
		string(models.OperationRouteInputTypeSource),
	)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOperationRouteGenerator_Generate_NilOperationRoutesService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewOperationRouteGenerator(e, nil)

	input := models.NewCreateOperationRouteInput(
		"Test Route",
		"Test description",
		string(models.OperationRouteInputTypeSource),
	)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestOperationRouteGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			return &models.OperationRoute{
				Title: input.Title,
			}, nil
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	input := models.NewCreateOperationRouteInput(
		"Source Route",
		"Source route description",
		string(models.OperationRouteInputTypeSource),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "customer"})

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	require.NoError(t, err)
	assert.Equal(t, "Source Route", result.Title)
}

func TestOperationRouteGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			return nil, errors.New("operation route creation failed")
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	input := models.NewCreateOperationRouteInput(
		"Test Route",
		"Test description",
		string(models.OperationRouteInputTypeSource),
	)

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", input)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "operation route creation failed")
}

func TestOperationRouteGenerator_GenerateDefaults_NilEntity(t *testing.T) {
	gen := NewOperationRouteGenerator(nil, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	assert.Error(t, err)
	assert.Nil(t, results)
}

func TestOperationRouteGenerator_GenerateDefaults_Success(t *testing.T) {
	var createdRoutes []string
	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			createdRoutes = append(createdRoutes, input.Title)
			return &models.OperationRoute{
				Title: input.Title,
			}, nil
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	require.NoError(t, err)
	assert.Len(t, results, 6)
	assert.Equal(t, 6, len(createdRoutes))
}

func TestOperationRouteGenerator_GenerateDefaults_Error(t *testing.T) {
	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			return nil, errors.New("defaults creation failed")
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	results, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "defaults creation failed")
}

func TestOperationRouteGenerator_Generate_VerifyIDs(t *testing.T) {
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID
			return &models.OperationRoute{}, nil
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	input := models.NewCreateOperationRouteInput(
		"Test Route",
		"Test description",
		string(models.OperationRouteInputTypeSource),
	)

	_, err := gen.Generate(context.Background(), "test-org", "test-ledger", input)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
}

func TestOperationRouteGenerator_GenerateDefaults_VerifyTemplates(t *testing.T) {
	var createdInputs []*models.CreateOperationRouteInput
	mockSvc := &mockOperationRoutesService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
			createdInputs = append(createdInputs, input)
			return &models.OperationRoute{
				Title: input.Title,
			}, nil
		},
	}

	e := &entities.Entity{
		OperationRoutes: mockSvc,
	}

	gen := NewOperationRouteGenerator(e, nil)

	_, err := gen.GenerateDefaults(context.Background(), "org-123", "ledger-123")
	require.NoError(t, err)

	sourceCount := 0
	destCount := 0

	for _, input := range createdInputs {
		if input.OperationType == string(models.OperationRouteInputTypeSource) {
			sourceCount++
		} else if input.OperationType == string(models.OperationRouteInputTypeDestination) {
			destCount++
		}
	}

	assert.Equal(t, 2, sourceCount)
	assert.Equal(t, 4, destCount)
}
