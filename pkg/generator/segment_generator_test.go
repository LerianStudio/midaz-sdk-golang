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

type mockSegmentsService struct {
	createFunc func(ctx context.Context, orgID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error)
}

func (m *mockSegmentsService) CreateSegment(ctx context.Context, orgID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, orgID, ledgerID, input)
	}

	return &models.Segment{ID: "seg-123", Name: input.Name}, nil
}

func (*mockSegmentsService) GetSegment(_ context.Context, _, _, _ string) (*models.Segment, error) {
	return nil, errors.New("mock: GetSegment not implemented")
}

func (*mockSegmentsService) ListSegments(_ context.Context, _, _ string, _ *models.ListOptions) (*models.ListResponse[models.Segment], error) {
	return nil, errors.New("mock: ListSegments not implemented")
}

func (*mockSegmentsService) UpdateSegment(_ context.Context, _, _, _ string, _ *models.UpdateSegmentInput) (*models.Segment, error) {
	return nil, errors.New("mock: UpdateSegment not implemented")
}

func (*mockSegmentsService) DeleteSegment(_ context.Context, _, _, _ string) error {
	return nil
}

func (*mockSegmentsService) GetSegmentsMetricsCount(_ context.Context, _, _ string) (*models.MetricsCount, error) {
	return nil, errors.New("mock: GetSegmentsMetricsCount not implemented")
}

func TestNewSegmentGenerator(t *testing.T) {
	t.Run("Create with nil entity", func(t *testing.T) {
		gen := NewSegmentGenerator(nil, nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with entity", func(t *testing.T) {
		e := &entities.Entity{}
		gen := NewSegmentGenerator(e, nil)
		assert.NotNil(t, gen)
	})
}

func TestSegmentGenerator_Generate_NilEntity(t *testing.T) {
	gen := NewSegmentGenerator(nil, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Segment", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSegmentGenerator_Generate_NilSegmentsService(t *testing.T) {
	e := &entities.Entity{}
	gen := NewSegmentGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Segment", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestSegmentGenerator_Generate_Success(t *testing.T) {
	mockSvc := &mockSegmentsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateSegmentInput) (*models.Segment, error) {
			return &models.Segment{
				ID:   "seg-success",
				Name: input.Name,
			}, nil
		},
	}

	e := &entities.Entity{
		Segments: mockSvc,
	}

	gen := NewSegmentGenerator(e, nil)
	metadata := map[string]any{
		"region":   "us-west",
		"category": "retail",
	}

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Retail Segment", metadata)
	require.NoError(t, err)
	assert.Equal(t, "seg-success", result.ID)
	assert.Equal(t, "Retail Segment", result.Name)
}

func TestSegmentGenerator_Generate_Error(t *testing.T) {
	mockSvc := &mockSegmentsService{
		createFunc: func(_ context.Context, _, _ string, _ *models.CreateSegmentInput) (*models.Segment, error) {
			return nil, errors.New("segment creation failed")
		},
	}

	e := &entities.Entity{
		Segments: mockSvc,
	}

	gen := NewSegmentGenerator(e, nil)

	result, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Segment", nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "segment creation failed")
}

func TestSegmentGenerator_Generate_NilMetadata(t *testing.T) {
	var capturedInput *models.CreateSegmentInput

	mockSvc := &mockSegmentsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateSegmentInput) (*models.Segment, error) {
			capturedInput = input

			return &models.Segment{ID: "seg-123"}, nil
		},
	}

	e := &entities.Entity{
		Segments: mockSvc,
	}

	gen := NewSegmentGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Test Segment", nil)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.Equal(t, "Test Segment", capturedInput.Name)
}

func TestSegmentGenerator_Generate_VerifyIDs(t *testing.T) {
	var receivedOrgID, receivedLedgerID string

	mockSvc := &mockSegmentsService{
		createFunc: func(_ context.Context, orgID, ledgerID string, _ *models.CreateSegmentInput) (*models.Segment, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID

			return &models.Segment{ID: "seg-123"}, nil
		},
	}

	e := &entities.Entity{
		Segments: mockSvc,
	}

	gen := NewSegmentGenerator(e, nil)

	_, err := gen.Generate(context.Background(), "test-org", "test-ledger", "Test", nil)
	require.NoError(t, err)

	assert.Equal(t, "test-org", receivedOrgID)
	assert.Equal(t, "test-ledger", receivedLedgerID)
}

func TestSegmentGenerator_Generate_WithMetadata(t *testing.T) {
	var capturedInput *models.CreateSegmentInput

	mockSvc := &mockSegmentsService{
		createFunc: func(_ context.Context, _, _ string, input *models.CreateSegmentInput) (*models.Segment, error) {
			capturedInput = input

			return &models.Segment{ID: "seg-123"}, nil
		},
	}

	e := &entities.Entity{
		Segments: mockSvc,
	}

	gen := NewSegmentGenerator(e, nil)
	metadata := map[string]any{
		"priority": "high",
		"type":     "premium",
	}

	_, err := gen.Generate(context.Background(), "org-123", "ledger-123", "Premium Segment", metadata)
	require.NoError(t, err)
	assert.NotNil(t, capturedInput)
	assert.NotNil(t, capturedInput.Metadata)
}
