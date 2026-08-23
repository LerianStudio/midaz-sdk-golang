package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
)

// SegmentGenerator creates segments within a ledger.
type SegmentGenerator interface {
	Generate(ctx context.Context, organizationID, ledgerID, name string, metadata map[string]any) (*models.Segment, error)
}

// segmentsAPI is the narrow slice of the segments facade this generator needs.
type segmentsAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreateSegmentInput) (*models.Segment, error)
}

type segmentGenerator struct {
	segments segmentsAPI
	obs      observability.Provider
}

// NewSegmentGenerator creates a new segment generator.
func NewSegmentGenerator(e *entities.Entity, obs observability.Provider) SegmentGenerator {
	g := &segmentGenerator{obs: obs}
	if e != nil && e.Segments != nil {
		g.segments = e.Segments
	}

	return g
}

// Generate creates a single segment with the specified parameters.
func (g *segmentGenerator) Generate(ctx context.Context, organizationID, ledgerID, name string, metadata map[string]any) (*models.Segment, error) {
	ctx = normalizeContext(ctx)

	if g.segments == nil {
		return nil, errors.New("entity segments service not initialized")
	}

	input := models.NewCreateSegmentInput(name).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(metadata)

	var out *models.Segment

	err := observability.WithSpan(ctx, g.obs, "GenerateSegment", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				s, err := g.segments.Create(ctx, organizationID, ledgerID, input)
				if err != nil {
					return err
				}

				out = s

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, errNilGenerated("segment")
	}

	return out, nil
}
