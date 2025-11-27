package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type PortfolioGenerator interface {
	Generate(ctx context.Context, orgID, ledgerID, name, entityID string, metadata map[string]any) (*models.Portfolio, error)
}

type portfolioGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

func NewPortfolioGenerator(e *entities.Entity, obs observability.Provider) PortfolioGenerator {
	return &portfolioGenerator{e: e, obs: obs}
}

func (g *portfolioGenerator) Generate(ctx context.Context, orgID, ledgerID, name, entityID string, metadata map[string]any) (*models.Portfolio, error) {
	if g.e == nil || g.e.Portfolios == nil {
		return nil, errors.New("entity portfolios service not initialized")
	}

	input := models.NewCreatePortfolioInput(entityID, name).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(metadata)

	var out *models.Portfolio

	err := observability.WithSpan(ctx, g.obs, "GeneratePortfolio", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				p, err := g.e.Portfolios.CreatePortfolio(ctx, orgID, ledgerID, input)
				if err != nil {
					return err
				}

				out = p

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}
