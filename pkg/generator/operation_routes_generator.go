package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
)

// operationRoutesAPI is the narrow slice of the operation-routes facade this generator needs.
type operationRoutesAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error)
}

type operationRouteGenerator struct {
	operationRoutes operationRoutesAPI
	obs             observability.Provider
}

// NewOperationRouteGenerator creates a new generator for operation routes.
func NewOperationRouteGenerator(e *entities.Entity, obs observability.Provider) OperationRouteGenerator {
	g := &operationRouteGenerator{obs: obs}
	if e != nil && e.V1.OperationRoutes != nil {
		g.operationRoutes = e.V1.OperationRoutes
	}

	return g
}

// Generate creates a single operation route from the provided input.
func (g *operationRouteGenerator) Generate(ctx context.Context, organizationID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	ctx = normalizeContext(ctx)

	if g.operationRoutes == nil {
		return nil, errors.New("entity operation routes service not initialized")
	}

	if input == nil {
		return nil, errors.New("operation route input is required")
	}

	validationErr := input.Validate()
	if validationErr != nil {
		return nil, validationErr
	}

	var out *models.OperationRoute

	err := observability.WithSpan(ctx, g.obs, "GenerateOperationRoute", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				or, err := g.operationRoutes.Create(ctx, organizationID, ledgerID, input)
				if err != nil {
					return err
				}

				out = or

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, errNilGenerated("operation route")
	}

	return out, nil
}

// GenerateDefaults creates a minimal set of operation routes for common flows.
func (g *operationRouteGenerator) GenerateDefaults(ctx context.Context, organizationID, ledgerID string) ([]*models.OperationRoute, error) {
	out := make([]*models.OperationRoute, 0, 5)

	// Source: Customer (CHECKING)
	srcCustomer := models.NewCreateOperationRouteInput(
		"Source: Customer (CHECKING)",
		"Allows checking-type customer accounts as source",
		string(models.OperationRouteInputTypeSource),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "customer", "route": "source_checking"})
	srcCustomer.AccountingEntries = sourceDirectAccountingEntries("customer")

	// Source: External (any account) for demo funding flows.
	srcExternal := models.NewCreateOperationRouteInput(
		"Source: External (any)",
		"Allows external source funding entries",
		string(models.OperationRouteInputTypeSource),
	).WithMetadata(map[string]any{"role": "external", "route": "source_external"})
	srcExternal.AccountingEntries = sourceDirectAccountingEntries("external")

	// Source: Merchant (CHECKING)
	srcMerchant := models.NewCreateOperationRouteInput(
		"Source: Merchant (CHECKING)",
		"Allows checking-type merchant accounts as source (refund)",
		string(models.OperationRouteInputTypeSource),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "merchant", "route": "source_checking_merchant"})
	srcMerchant.AccountingEntries = sourceDirectAccountingEntries("merchant")

	// Destination: Merchant (CHECKING)
	dstMerchant := models.NewCreateOperationRouteInput(
		"Destination: Merchant (CHECKING)",
		"Allows checking-type merchant accounts as destination",
		string(models.OperationRouteInputTypeDestination),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "merchant", "route": "dest_checking"})
	dstMerchant.AccountingEntries = destinationDirectAccountingEntries("merchant")

	// Destination: Platform Fee (alias)
	dstPlatformFee := models.NewCreateOperationRouteInput(
		"Destination: Platform Fee (alias)",
		"Routes to platform fee account by alias",
		string(models.OperationRouteInputTypeDestination),
	).WithMetadata(map[string]any{"role": "internal", "route": "dest_platform_fee"})
	dstPlatformFee = dstPlatformFee.WithAccountAlias("platform_fee")
	dstPlatformFee.AccountingEntries = destinationDirectAccountingEntries("platform_fee")

	// Destination: Settlement Pool (alias)
	dstSettlement := models.NewCreateOperationRouteInput(
		"Destination: Settlement Pool (alias)",
		"Routes to settlement pool account by alias",
		string(models.OperationRouteInputTypeDestination),
	).WithMetadata(map[string]any{"role": "internal", "route": "dest_settlement"})
	dstSettlement = dstSettlement.WithAccountAlias("settlement_pool")
	dstSettlement.AccountingEntries = destinationDirectAccountingEntries("settlement")

	// Destination: Customer (CHECKING) for refunds
	dstCustomer := models.NewCreateOperationRouteInput(
		"Destination: Customer (CHECKING)",
		"Allows checking-type customer accounts as destination (refund)",
		string(models.OperationRouteInputTypeDestination),
	).WithAccountTypes([]string{"CHECKING"}).WithMetadata(map[string]any{"role": "customer", "route": "dest_checking_customer"})
	dstCustomer.AccountingEntries = destinationDirectAccountingEntries("customer")

	templates := []*models.CreateOperationRouteInput{srcCustomer, srcExternal, srcMerchant, dstMerchant, dstPlatformFee, dstSettlement, dstCustomer}
	for _, tpl := range templates {
		or, err := g.Generate(ctx, organizationID, ledgerID, tpl)
		if err != nil {
			return nil, err
		}

		out = append(out, or)
	}

	return out, nil
}

func sourceDirectAccountingEntries(label string) *models.AccountingEntries {
	return &models.AccountingEntries{Direct: &models.AccountingEntry{Debit: accountingRubric("1000", label+" debit")}}
}

func destinationDirectAccountingEntries(label string) *models.AccountingEntries {
	return &models.AccountingEntries{Direct: &models.AccountingEntry{Credit: accountingRubric("2000", label+" credit")}}
}

func accountingRubric(code, description string) *models.AccountingRubric {
	return &models.AccountingRubric{Code: code, Description: description}
}
