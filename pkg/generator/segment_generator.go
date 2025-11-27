package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type SegmentGenerator interface {
	Generate(ctx context.Context, orgID, ledgerID, name string, metadata map[string]any) (*models.Segment, error)
}

type segmentGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

func NewSegmentGenerator(e *entities.Entity, obs observability.Provider) SegmentGenerator {
	return &segmentGenerator{e: e, obs: obs}
}

func (g *segmentGenerator) Generate(ctx context.Context, orgID, ledgerID, name string, metadata map[string]any) (*models.Segment, error) {
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
				s, err := g.e.Segments.CreateSegment(ctx, orgID, ledgerID, input)
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

	return out, nil
}
