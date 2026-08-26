package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
)

// PortfolioGenerator creates portfolios within a ledger.
type PortfolioGenerator interface {
	Generate(ctx context.Context, organizationID, ledgerID, name, entityID string, metadata map[string]any) (*models.Portfolio, error)
}

// portfoliosAPI is the narrow slice of the portfolios facade this generator needs.
type portfoliosAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreatePortfolioInput) (*models.Portfolio, error)
}

type portfolioGenerator struct {
	portfolios portfoliosAPI
	obs        observability.Provider
}

// NewPortfolioGenerator creates a new portfolio generator.
func NewPortfolioGenerator(e *entities.Entity, obs observability.Provider) PortfolioGenerator {
	g := &portfolioGenerator{obs: obs}
	if e != nil && e.V1.Portfolios != nil {
		g.portfolios = e.V1.Portfolios
	}

	return g
}

// Generate creates a single portfolio with the specified parameters.
func (g *portfolioGenerator) Generate(ctx context.Context, organizationID, ledgerID, name, entityID string, metadata map[string]any) (*models.Portfolio, error) {
	ctx = normalizeContext(ctx)

	if g.portfolios == nil {
		return nil, errors.New("entity portfolios service not initialized")
	}

	input := models.NewCreatePortfolioInput(entityID, name).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(metadata)

	var out *models.Portfolio

	err := observability.WithSpan(ctx, g.obs, "GeneratePortfolio", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				p, err := g.portfolios.Create(ctx, organizationID, ledgerID, input)
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

	if out == nil {
		return nil, errNilGenerated("portfolio")
	}

	return out, nil
}
