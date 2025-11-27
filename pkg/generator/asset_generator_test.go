package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAssetsService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error)
}

func (m *mockAssetsService) CreateAsset(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}
	return &models.Asset{ID: "asset-123", Name: input.Name, Code: input.Code}, nil
}

func (m *mockAssetsService) GetAsset(ctx context.Context, orgID, ledgerID, id string) (*models.Asset, error) {
	return nil, nil
}

func (m *mockAssetsService) ListAssets(ctx context.Context, orgID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Asset], error) {
	return nil, nil
}

func (m *mockAssetsService) UpdateAsset(ctx context.Context, orgID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error) {
	return nil, nil
}

func (m *mockAssetsService) DeleteAsset(ctx context.Context, orgID, ledgerID, id string) error {
	return nil
}

func (m *mockAssetsService) GetAssetsMetricsCount(ctx context.Context, orgID, ledgerID string) (*models.MetricsCount, error) {
	return nil, nil
}

func TestNewAssetGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewAssetGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewAssetGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestAssetGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewAssetGenerator(nil, nil)
	template := data.AssetTemplate{
		Name: "US Dollar",
		Code: "USD",
		Type: "currency",
	}

	ctx := WithOrgID(context.Background(), "org-123")
	_, err := gen.Generate(ctx, "ledger-123", template)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAssetGenerator_Generate_NilAssetsService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name: "US Dollar",
		Code: "USD",
		Type: "currency",
	}

	ctx := WithOrgID(context.Background(), "org-123")
	_, err := gen.Generate(ctx, "ledger-123", template)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestAssetGenerator_Generate_MissingOrgID(t *testing.T) {
	mockSvc := &mockAssetsService{}
	e := &entities.Entity{
		Assets: mockSvc,
	}
	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name: "US Dollar",
		Code: "USD",
		Type: "currency",
	}

	_, err := gen.Generate(context.Background(), "ledger-123", template)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "organization id missing")
}

func TestAssetGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockAssetsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
			return &models.Asset{
				ID:   "asset-success",
				Name: input.Name,
				Code: input.Code,
			}, nil
		},
	}

	e := &entities.Entity{
		Assets: mockSvc,
	}

	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name:  "US Dollar",
		Code:  "USD",
		Type:  "currency",
		Scale: 2,
		Metadata: map[string]any{
			"symbol": "$",
		},
	}

	ctx := WithOrgID(context.Background(), "org-123")
	result, err := gen.Generate(ctx, "ledger-123", template)
	require.NoError(t, err)
	assert.Equal(t, "asset-success", result.ID)
	assert.Equal(t, "US Dollar", result.Name)
	assert.Equal(t, "USD", result.Code)
}

func TestAssetGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockAssetsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
			return nil, errors.New("asset creation failed")
		},
	}

	e := &entities.Entity{
		Assets: mockSvc,
	}

	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name: "US Dollar",
		Code: "USD",
		Type: "currency",
	}

	ctx := WithOrgID(context.Background(), "org-123")
	result, err := gen.Generate(ctx, "ledger-123", template)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "asset creation failed")
}

func TestAssetGenerator_GenerateWithRates_NotImplemented(t *testing.T) {
	gen := NewAssetGenerator(nil, nil)

	err := gen.GenerateWithRates(context.Background(), "ledger-123", "USD")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestAssetGenerator_UpdateRates_NotImplemented(t *testing.T) {
	gen := NewAssetGenerator(nil, nil)

	rates := map[string]float64{
		"EUR": 0.85,
		"GBP": 0.73,
	}

	err := gen.UpdateRates(context.Background(), "ledger-123", rates)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestMergeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]any
		b        map[string]any
		expected map[string]any
	}{
		{
			name:     "Both nil",
			a:        nil,
			b:        nil,
			expected: nil,
		},
		{
			name: "First nil",
			a:    nil,
			b:    map[string]any{"key": "value"},
			expected: map[string]any{
				"key": "value",
			},
		},
		{
			name: "Second nil",
			a:    map[string]any{"key": "value"},
			b:    nil,
			expected: map[string]any{
				"key": "value",
			},
		},
		{
			name: "Both have values",
			a:    map[string]any{"key1": "value1"},
			b:    map[string]any{"key2": "value2"},
			expected: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "Overlapping keys - b wins",
			a:    map[string]any{"key": "value_a"},
			b:    map[string]any{"key": "value_b"},
			expected: map[string]any{
				"key": "value_b",
			},
		},
		{
			name:     "Empty maps",
			a:        map[string]any{},
			b:        map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "Complex values",
			a:    map[string]any{"nested": map[string]any{"inner": "value"}},
			b:    map[string]any{"number": 42},
			expected: map[string]any{
				"nested": map[string]any{"inner": "value"},
				"number": 42,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMetadata(tt.a, tt.b)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAssetTemplate_Fields(t *testing.T) {
	t.Run("Currency asset", func(t *testing.T) {
		template := data.AssetTemplate{
			Name:  "US Dollar",
			Type:  "currency",
			Code:  "USD",
			Scale: 2,
			Metadata: map[string]any{
				"symbol":   "$",
				"iso_code": "840",
			},
		}

		assert.Equal(t, "US Dollar", template.Name)
		assert.Equal(t, "currency", template.Type)
		assert.Equal(t, "USD", template.Code)
		assert.Equal(t, 2, template.Scale)
		assert.NotNil(t, template.Metadata)
	})

	t.Run("Crypto asset", func(t *testing.T) {
		template := data.AssetTemplate{
			Name:  "Bitcoin",
			Type:  "crypto",
			Code:  "BTC",
			Scale: 8,
		}

		assert.Equal(t, "Bitcoin", template.Name)
		assert.Equal(t, "crypto", template.Type)
		assert.Equal(t, "BTC", template.Code)
		assert.Equal(t, 8, template.Scale)
	})

	t.Run("Points asset", func(t *testing.T) {
		template := data.AssetTemplate{
			Name:  "Loyalty Points",
			Type:  "points",
			Code:  "POINTS",
			Scale: 0,
		}

		assert.Equal(t, "Loyalty Points", template.Name)
		assert.Equal(t, "points", template.Type)
		assert.Equal(t, "POINTS", template.Code)
		assert.Equal(t, 0, template.Scale)
	})
}

func TestAssetGenerator_Generate_VerifyInput(t *testing.T) {
	var receivedInput *models.CreateAssetInput
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockAssetsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
			receivedInput = input
			receivedOrgID = orgID
			receivedLedgerID = ledgerID
			return &models.Asset{ID: "asset-123"}, nil
		},
	}

	e := &entities.Entity{
		Assets: mockSvc,
	}

	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name:  "Euro",
		Code:  "EUR",
		Type:  "currency",
		Scale: 2,
		Metadata: map[string]any{
			"region": "EU",
		},
	}

	ctx := WithOrgID(context.Background(), "test-org")
	_, err := gen.Generate(ctx, "test-ledger", template)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
	assert.Equal(t, "Euro", receivedInput.Name)
	assert.Equal(t, "EUR", receivedInput.Code)
	assert.Equal(t, "currency", receivedInput.Type)
}

func TestWithOrgID_InContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithOrgID(ctx, "org-456")

	val := ctx.Value(contextKeyOrgID{})
	assert.Equal(t, "org-456", val)
}

func TestAssetGenerator_Generate_WithCircuitBreaker(t *testing.T) {
	mockSvc := &mockAssetsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
			return &models.Asset{ID: "asset-cb"}, nil
		},
	}

	e := &entities.Entity{
		Assets: mockSvc,
	}

	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name: "Test Asset",
		Code: "TEST",
		Type: "currency",
	}

	ctx := WithOrgID(context.Background(), "org-123")

	result, err := gen.Generate(ctx, "ledger-123", template)
	require.NoError(t, err)
	assert.Equal(t, "asset-cb", result.ID)
}

func TestAssetGenerator_Generate_MetadataWithScale(t *testing.T) {
	var capturedMetadata map[string]any

	mockSvc := &mockAssetsService{
		createFunc: func(ctx context.Context, orgID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
			capturedMetadata = input.Metadata
			return &models.Asset{ID: "asset-123"}, nil
		},
	}

	e := &entities.Entity{
		Assets: mockSvc,
	}

	gen := NewAssetGenerator(e, nil)
	template := data.AssetTemplate{
		Name:  "Japanese Yen",
		Code:  "JPY",
		Type:  "currency",
		Scale: 0,
		Metadata: map[string]any{
			"country": "Japan",
		},
	}

	ctx := WithOrgID(context.Background(), "org-123")
	_, err := gen.Generate(ctx, "ledger-123", template)
	require.NoError(t, err)

	assert.NotNil(t, capturedMetadata)
	assert.Equal(t, "Japan", capturedMetadata["country"])
	assert.Equal(t, 0, capturedMetadata["scale"])
}
