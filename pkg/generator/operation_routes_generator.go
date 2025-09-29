package generator

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type operationRouteGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

// NewOperationRouteGenerator creates a new generator for operation routes.
func NewOperationRouteGenerator(e *entities.Entity, obs observability.Provider) OperationRouteGenerator {
	return &operationRouteGenerator{e: e, obs: obs}
}

func (g *operationRouteGenerator) Generate(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	if g.e == nil || g.e.OperationRoutes == nil {
		return nil, fmt.Errorf("entity operation routes service not initialized")
	}

	validationErr := input.Validate()
	if validationErr != nil {
		return nil, validationErr
	}

	var out *models.OperationRoute

	err := observability.WithSpan(ctx, g.obs, "GenerateOperationRoute", func(ctx context.Context) error {
		return retry.DoWithContext(ctx, func() error {
			or, err := g.e.OperationRoutes.CreateOperationRoute(ctx, orgID, ledgerID, input)
			if err != nil {
				return err
			}

			out = or

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GenerateDefaults creates a minimal set of operation routes for common flows.
func (g *operationRouteGenerator) GenerateDefaults(ctx context.Context, orgID, ledgerID string) ([]*models.OperationRoute, error) {
	out := make([]*models.OperationRoute, 0, 5)

	// Source: Customer (CHECKING)
	srcCustomer := models.NewCreateOperationRouteInput(
		"Source: Customer (CHECKING)",
		"Allows checking-type customer accounts as source",
		string(models.OperationRouteInputTypeSource),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "customer", "route": "source_checking"})

	// Source: Merchant (CHECKING)
	srcMerchant := models.NewCreateOperationRouteInput(
		"Source: Merchant (CHECKING)",
		"Allows checking-type merchant accounts as source (refund)",
		string(models.OperationRouteInputTypeSource),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "merchant", "route": "source_checking_merchant"})

	// Destination: Merchant (CHECKING)
	dstMerchant := models.NewCreateOperationRouteInput(
		"Destination: Merchant (CHECKING)",
		"Allows checking-type merchant accounts as destination",
		string(models.OperationRouteInputTypeDestination),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "merchant", "route": "dest_checking"})

	// Destination: Platform Fee (alias)
	dstPlatformFee := models.NewCreateOperationRouteInput(
		"Destination: Platform Fee (alias)",
		"Routes to platform fee account by alias",
		string(models.OperationRouteInputTypeDestination),
	).WithMetadata(map[string]any{"role": "internal", "route": "dest_platform_fee"})
	dstPlatformFee = models.WithCreateOperationRouteAccountAlias(dstPlatformFee, "platform_fee")

	// Destination: Settlement Pool (alias)
	dstSettlement := models.NewCreateOperationRouteInput(
		"Destination: Settlement Pool (alias)",
		"Routes to settlement pool account by alias",
		string(models.OperationRouteInputTypeDestination),
	).WithMetadata(map[string]any{"role": "internal", "route": "dest_settlement"})
	dstSettlement = models.WithCreateOperationRouteAccountAlias(dstSettlement, "settlement_pool")

	// Destination: Customer (CHECKING) for refunds
	dstCustomer := models.NewCreateOperationRouteInput(
		"Destination: Customer (CHECKING)",
		"Allows checking-type customer accounts as destination (refund)",
		string(models.OperationRouteInputTypeDestination),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "customer", "route": "dest_checking_customer"})

	templates := []*models.CreateOperationRouteInput{srcCustomer, srcMerchant, dstMerchant, dstPlatformFee, dstSettlement, dstCustomer}
	for _, tpl := range templates {
		or, err := g.Generate(ctx, orgID, ledgerID, tpl)
		if err != nil {
			return nil, err
		}

		out = append(out, or)
	}

	return out, nil
}
