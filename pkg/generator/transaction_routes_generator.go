package generator

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type transactionRouteGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

// NewTransactionRouteGenerator creates a new generator for transaction routes.
func NewTransactionRouteGenerator(e *entities.Entity, obs observability.Provider) TransactionRouteGenerator {
	return &transactionRouteGenerator{e: e, obs: obs}
}

func (g *transactionRouteGenerator) Generate(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	if g.e == nil || g.e.TransactionRoutes == nil {
		return nil, fmt.Errorf("entity transaction routes service not initialized")
	}

	validationErr := input.Validate()
	if validationErr != nil {
		return nil, validationErr
	}

	var out *models.TransactionRoute

	err := observability.WithSpan(ctx, g.obs, "GenerateTransactionRoute", func(ctx context.Context) error {
		return retry.DoWithContext(ctx, func() error {
			tr, err := g.e.TransactionRoutes.CreateTransactionRoute(ctx, orgID, ledgerID, input)
			if err != nil {
				return err
			}

			out = tr

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GenerateDefaults creates default transaction routes for common flows.
// Requires the operation routes (by ID) already created via OperationRouteGenerator.
func (g *transactionRouteGenerator) GenerateDefaults(ctx context.Context, orgID, ledgerID string, opRoutes []*models.OperationRoute) ([]*models.TransactionRoute, error) {
	// Map titles for convenience
	byTitle := map[string]string{}
	for _, or := range opRoutes {
		byTitle[or.Title] = or.ID.String()
	}

	routes := make([]*models.TransactionRoute, 0, 3)

	// Payment: Customer Source (CHECKING) -> Merchant Dest (CHECKING) + Platform Fee Dest
	payOps := []string{}
	srcCustomerID, srcOk := byTitle["Source: Customer (CHECKING)"]

	if srcOk {
		payOps = append(payOps, srcCustomerID)
	}

	dstMerchantID, dstMerchantOk := byTitle["Destination: Merchant (CHECKING)"]
	if dstMerchantOk {
		payOps = append(payOps, dstMerchantID)
	}

	dstPlatformID, dstPlatformOk := byTitle["Destination: Platform Fee (alias)"]
	if dstPlatformOk {
		payOps = append(payOps, dstPlatformID)
	}

	if len(payOps) >= 2 { // at least source + one dest
		input := models.NewCreateTransactionRouteInput("Payment Flow", "Customer pays merchant with platform fee", payOps).
			WithMetadata(map[string]any{"pattern": "payment"})

		tr, err := g.Generate(ctx, orgID, ledgerID, input)
		if err != nil {
			return nil, err
		}

		routes = append(routes, tr)
	}

	// Refund: Merchant Source (CHECKING) -> Customer Dest (CHECKING)
	refundOps := []string{}
	refundMerchantID, refundMerchantOk := byTitle["Source: Merchant (CHECKING)"]

	if refundMerchantOk {
		refundOps = append(refundOps, refundMerchantID)
	}

	refundCustomerID, refundCustomerOk := byTitle["Destination: Customer (CHECKING)"]
	if refundCustomerOk {
		refundOps = append(refundOps, refundCustomerID)
	}

	if len(refundOps) >= 2 {
		input := models.NewCreateTransactionRouteInput("Refund Flow", "Merchant refunds customer", refundOps).
			WithMetadata(map[string]any{"pattern": "refund"})

		tr, err := g.Generate(ctx, orgID, ledgerID, input)
		if err != nil {
			return nil, err
		}

		routes = append(routes, tr)
	}

	// Transfer: Checking -> Checking (generic)
	transferOps := []string{}
	transferSrcID, transferSrcOk := byTitle["Source: Customer (CHECKING)"]

	if transferSrcOk {
		transferOps = append(transferOps, transferSrcID)
	}

	transferDstID, transferDstOk := byTitle["Destination: Customer (CHECKING)"]
	if transferDstOk {
		transferOps = append(transferOps, transferDstID)
	}

	if len(transferOps) >= 2 {
		input := models.NewCreateTransactionRouteInput("Transfer Flow", "Internal transfer between checking accounts", transferOps).
			WithMetadata(map[string]any{"pattern": "transfer"})

		tr, err := g.Generate(ctx, orgID, ledgerID, input)
		if err != nil {
			return nil, err
		}

		routes = append(routes, tr)
	}

	return routes, nil
}
