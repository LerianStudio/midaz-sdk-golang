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

type mockPortfoliosService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error)
}

func (m *mockPortfoliosService) CreatePortfolio(ctx context.Context, orgID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}

	return &models.Portfolio{ID: "port-123", Name: input.Name}, nil
}

func (*mockPortfoliosService) GetPortfolio(_ context.Context, _, _, _ string) (*models.Portfolio, error) {
	return nil, errors.New("mock: GetPortfolio not implemented")
}

func (*mockPortfoliosService) ListPortfolios(_ context.Context, _, _ string, _ *models.ListOptions) (*models.ListResponse[models.Portfolio], error) {
	return nil, errors.New("mock: ListPortfolios not implemented")
}

func (*mockPortfoliosService) UpdatePortfolio(_ context.Context, _, _, _ string, _ *models.UpdatePortfolioInput) (*models.Portfolio, error) {
	return nil, errors.New("mock: UpdatePortfolio not implemented")
}

func (*mockPortfoliosService) DeletePortfolio(_ context.Context, _, _, _ string) error {
	return nil
}

func (*mockPortfoliosService) GetPortfoliosMetricsCount(_ context.Context, _, _ string) (*models.MetricsCount, error) {
	return nil, errors.New("mock: GetPortfoliosMetricsCount not implemented")
}

func TestNewPortfolioGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewPortfolioGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewPortfolioGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestPortfolioGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewPortfolioGenerator(nil, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Portfolio", "entity-456", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPortfolioGenerator_Generate_NilPortfoliosService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewPortfolioGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Portfolio", "entity-456", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestPortfolioGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
			return &models.Portfolio{
				ID:       "port-success",
				Name:     input.Name,
				EntityID: input.EntityID,
			}, nil
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)
	metadata := map[string]any{
		"strategy":   "growth",
		"risk_level": "moderate",
	}

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Growth Portfolio", "entity-789", metadata)
	require.NoError(t, err)
	assert.Equal(t, "port-success", result.ID)
	assert.Equal(t, "Growth Portfolio", result.Name)
	assert.Equal(t, "entity-789", result.EntityID)
}

func TestPortfolioGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, _, _ string, _ *models.CreatePortfolioInput) (*models.Portfolio, error) {
			return nil, errors.New("portfolio creation failed")
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Portfolio", "entity-456", nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "portfolio creation failed")
}

func TestPortfolioGenerator_Generate_NilMetadata(t *testing.T) {
	var capturedInput *models.CreatePortfolioInput

	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
			capturedInput = input

			return &models.Portfolio{ID: "port-123"}, nil
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Portfolio", "entity-456", nil)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.Equal(t, "Test Portfolio", capturedInput.Name)
	assert.Equal(t, "entity-456", capturedInput.EntityID)
}

func TestPortfolioGenerator_Generate_VerifyIDs(t *testing.T) {
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, orgID, ledgerID string, _ *models.CreatePortfolioInput) (*models.Portfolio, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID

			return &models.Portfolio{ID: "port-123"}, nil
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "test-org", "test-ledger", "Test", "entity-123", nil)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
}

func TestPortfolioGenerator_Generate_WithMetadata(t *testing.T) {
	var capturedInput *models.CreatePortfolioInput

	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
			capturedInput = input

			return &models.Portfolio{ID: "port-123"}, nil
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)
	metadata := map[string]any{
		"category": "investment",
		"manager":  "AI Bot",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Investment Portfolio", "entity-456", metadata)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.NotNil(t, capturedInput.Metadata)
}

func TestPortfolioGenerator_Generate_VerifyEntityID(t *testing.T) {
	var capturedInput *models.CreatePortfolioInput

	mockSvc := &mockPortfoliosService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreatePortfolioInput) (*models.Portfolio, error) {
			capturedInput = input

			return &models.Portfolio{ID: "port-123"}, nil
		},
	}

	e := &entities.Entity{
		Portfolios: mockSvc,
	}

	gen := NewPortfolioGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Portfolio", "custom-entity-id", nil)
	require.NoError(t, err)
	assert.Equal(t, "custom-entity-id", capturedInput.EntityID)
}
