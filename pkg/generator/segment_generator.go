package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
)

// SegmentGenerator creates segments within a ledger.
type SegmentGenerator interface {
	Generate(ctx context.Context, organizationID, ledgerID, name string, metadata map[string]any) (*models.Segment, error)
}

type segmentGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

// NewSegmentGenerator creates a new segment generator.
func NewSegmentGenerator(e *entities.Entity, obs observability.Provider) SegmentGenerator {
	return &segmentGenerator{e: e, obs: obs}
}

// Generate creates a single segment with the specified parameters.
func (g *segmentGenerator) Generate(ctx context.Context, organizationID, ledgerID, name string, metadata map[string]any) (*models.Segment, error) {
	ctx = normalizeContext(ctx)

	if g.e == nil || g.e.Segments == nil {
		return nil, errors.New("entity segments service not initialized")
	}

	input := models.NewCreateSegmentInput(name).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(metadata)

	var out *models.Segment

	err := observability.WithSpan(ctx, g.obs, "GenerateSegment", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				s, err := g.e.Segments.CreateSegment(ctx, organizationID, ledgerID, input)
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
