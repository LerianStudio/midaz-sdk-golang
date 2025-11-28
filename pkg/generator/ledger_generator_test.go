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

type mockLedgersService struct {
	createFunc func(ctx context.Context, orgID string, input *models.CreateLedgerInput) (*models.Ledger, error)
	listFunc   func(ctx context.Context, orgID string, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error)
}

func (m *mockLedgersService) CreateLedger(ctx context.Context, orgID string, input *models.CreateLedgerInput) (*models.Ledger, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, input)
	}

	return &models.Ledger{ID: "ledger-123", Name: input.Name}, nil
}

func (*mockLedgersService) GetLedger(_ context.Context, _, _ string) (*models.Ledger, error) {
	return nil, errors.New("mock: GetLedger not implemented")
}

func (m *mockLedgersService) ListLedgers(ctx context.Context, orgID string, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, orgID, opts)
	}

	return &models.ListResponse[models.Ledger]{Items: []models.Ledger{}}, nil
}

func (*mockLedgersService) UpdateLedger(_ context.Context, _, _ string, _ *models.UpdateLedgerInput) (*models.Ledger, error) {
	return nil, errors.New("mock: UpdateLedger not implemented")
}

func (*mockLedgersService) DeleteLedger(_ context.Context, _, _ string) error {
	return nil
}

func (*mockLedgersService) GetLedgersMetricsCount(_ context.Context, _ string) (*models.MetricsCount, error) {
	return nil, errors.New("mock: GetLedgersMetricsCount not implemented")
}

func TestNewLedgerGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewLedgerGenerator(nil, nil, "")
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity and default org", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewLedgerGenerator(e, nil, "org-123")
		assert.NotNil(t, gen)
	})

	t.Run("Create with empty default org", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewLedgerGenerator(e, nil, "")
		assert.NotNil(t, gen)
	})
}

func TestLedgerGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewLedgerGenerator(nil, nil, "")
	template := data.LedgerTemplate{
		Name: "Test Ledger",
	}

	_, err := gen.Generate(context.Background(), "org-123", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestLedgerGenerator_Generate_NilLedgersService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewLedgerGenerator(e, nil, "")
	template := data.LedgerTemplate{
		Name: "Test Ledger",
	}

	_, err := gen.Generate(context.Background(), "org-123", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestLedgerGenerator_Generate_EmptyOrgID(t *testing.T) {
	mockSvc := &mockLedgersService{}
	e := &entities.Entity{
		Ledgers: mockSvc,
	}
	gen := NewLedgerGenerator(e, nil, "")
	template := data.LedgerTemplate{
		Name: "Test Ledger",
	}

	_, err := gen.Generate(context.Background(), "", template)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization id is required")
}

func TestLedgerGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockLedgersService{
		createFunc: func(_ context.Context, _ string, input *models.CreateLedgerInput) (*models.Ledger, error) {
			return &models.Ledger{
				ID:   "ledger-success",
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "")
	template := data.LedgerTemplate{
		Name:   "Test Ledger",
		Status: models.NewStatus(models.StatusActive),
		Metadata: map[string]any{
			"purpose": "operational",
		},
	}

	result, err := gen.Generate(context.Background(), "org-123", template)
	require.NoError(t, err)
	assert.Equal(t, "ledger-success", result.ID)
	assert.Equal(t, "Test Ledger", result.Name)
}

func TestLedgerGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockLedgersService{
		createFunc: func(_ context.Context, _ string, _ *models.CreateLedgerInput) (*models.Ledger, error) {
			return nil, errors.New("ledger creation failed")
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "")
	template := data.LedgerTemplate{
		Name: "Test Ledger",
	}

	result, err := gen.Generate(context.Background(), "org-123", template)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "ledger creation failed")
}

func TestLedgerGenerator_GenerateForOrg_ZeroCount(t *testing.T) {
	gen := NewLedgerGenerator(nil, nil, "")

	results, err := gen.GenerateForOrg(context.Background(), "org-123", 0)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestLedgerGenerator_GenerateForOrg_NegativeCount(t *testing.T) {
	gen := NewLedgerGenerator(nil, nil, "")

	results, err := gen.GenerateForOrg(context.Background(), "org-123", -5)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestLedgerGenerator_GenerateForOrg_NilEntity(t *testing.T) {
	gen := NewLedgerGenerator(nil, nil, "")

	results, err := gen.GenerateForOrg(context.Background(), "org-123", 3)
	require.Error(t, err)
	assert.Empty(t, results)
}

func TestLedgerGenerator_GenerateForOrg_Success(t *testing.T) {
	var callCount atomic.Int32
	mockSvc := &mockLedgersService{
		createFunc: func(_ context.Context, _ string, input *models.CreateLedgerInput) (*models.Ledger, error) {
			count := callCount.Add(1)

			return &models.Ledger{
				ID:   "ledger-" + string(rune('0'+count)),
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "")
	ctx := WithWorkers(context.Background(), 2)

	results, err := gen.GenerateForOrg(ctx, "org-123", 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestLedgerGenerator_GenerateForOrg_PartialError(t *testing.T) {
	var callCount atomic.Int32
	mockSvc := &mockLedgersService{
		createFunc: func(_ context.Context, _ string, input *models.CreateLedgerInput) (*models.Ledger, error) {
			count := callCount.Add(1)
			if count == 2 {
				return nil, errors.New("partial failure")
			}

			return &models.Ledger{
				ID:   "ledger-ok",
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "")
	ctx := WithWorkers(context.Background(), 1)

	results, err := gen.GenerateForOrg(ctx, "org-123", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partial failure")
	assert.Len(t, results, 2)
}

func TestLedgerGenerator_ListWithPagination_NoDefaultOrg(t *testing.T) {
	gen := NewLedgerGenerator(nil, nil, "")

	_, err := gen.ListWithPagination(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default organization id not configured")
}

func TestLedgerGenerator_ListWithPagination_Success(t *testing.T) {
	mockSvc := &mockLedgersService{
		listFunc: func(_ context.Context, _ string, _ *models.ListOptions) (*models.ListResponse[models.Ledger], error) {
			return &models.ListResponse[models.Ledger]{
				Items: []models.Ledger{
					{ID: "ledger-1", Name: "Ledger 1"},
					{ID: "ledger-2", Name: "Ledger 2"},
				},
			}, nil
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "default-org")

	result, err := gen.ListWithPagination(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
}

func TestLedgerGenerator_ListWithPagination_Error(t *testing.T) {
	mockSvc := &mockLedgersService{
		listFunc: func(_ context.Context, _ string, _ *models.ListOptions) (*models.ListResponse[models.Ledger], error) {
			return nil, errors.New("list failed")
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "default-org")

	result, err := gen.ListWithPagination(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list failed")
}

func TestLedgerGenerator_ListWithPagination_WithOptions(t *testing.T) {
	var receivedOpts *models.ListOptions

	mockSvc := &mockLedgersService{
		listFunc: func(_ context.Context, _ string, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error) {
			receivedOpts = opts

			return &models.ListResponse[models.Ledger]{
				Items: []models.Ledger{},
			}, nil
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "default-org")

	opts := &models.ListOptions{
		Limit: 10,
	}

	_, err := gen.ListWithPagination(context.Background(), opts)
	require.NoError(t, err)
	assert.NotNil(t, receivedOpts)
	assert.Equal(t, 10, receivedOpts.Limit)
}

func TestLedgerTemplate_Fields(t *testing.T) {
	t.Run("Complete template", func(t *testing.T) {
		template := data.LedgerTemplate{
			Name:   "Test Ledger",
			Status: models.NewStatus(models.StatusActive),
			Metadata: map[string]any{
				"purpose":        "operational",
				"currency_scope": "multi",
				"region":         "us",
			},
		}

		assert.Equal(t, "Test Ledger", template.Name)
		assert.NotNil(t, template.Status)
		assert.NotNil(t, template.Metadata)
		assert.Equal(t, "operational", template.Metadata["purpose"])
	})

	t.Run("Minimal template", func(t *testing.T) {
		template := data.LedgerTemplate{
			Name: "Minimal Ledger",
		}

		assert.Equal(t, "Minimal Ledger", template.Name)
		assert.Nil(t, template.Metadata)
	})
}

func TestLedgerGenerator_GenerateForOrg_WithWorkers(t *testing.T) {
	tests := []struct {
		name    string
		workers int
	}{
		{"Default workers", 0},
		{"Small worker pool", 2},
		{"Large worker pool", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &mockLedgersService{
				createFunc: func(_ context.Context, _ string, input *models.CreateLedgerInput) (*models.Ledger, error) {
					return &models.Ledger{
						ID:   "ledger-test",
						Name: input.Name,
					}, nil
				},
			}

			e := &entities.Entity{
				Ledgers: mockSvc,
			}

			gen := NewLedgerGenerator(e, nil, "")
			ctx := context.Background()

			if tt.workers > 0 {
				ctx = WithWorkers(ctx, tt.workers)
			}

			results, err := gen.GenerateForOrg(ctx, "org-123", 5)
			require.NoError(t, err)
			assert.Len(t, results, 5)
		})
	}
}

func TestLedgerGenerator_GenerateForOrg_AllErrors(t *testing.T) {
	mockSvc := &mockLedgersService{
		createFunc: func(_ context.Context, _ string, _ *models.CreateLedgerInput) (*models.Ledger, error) {
			return nil, errors.New("all failed")
		},
	}

	e := &entities.Entity{
		Ledgers: mockSvc,
	}

	gen := NewLedgerGenerator(e, nil, "")
	ctx := WithWorkers(context.Background(), 1)

	results, err := gen.GenerateForOrg(ctx, "org-123", 3)
	require.Error(t, err)
	assert.Empty(t, results)
}
