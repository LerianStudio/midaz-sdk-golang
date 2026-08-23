package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
)

// transactionRoutesAPI is the narrow slice of the transaction-routes facade this generator needs.
type transactionRoutesAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error)
}

type transactionRouteGenerator struct {
	transactionRoutes transactionRoutesAPI
	obs               observability.Provider
}

// NewTransactionRouteGenerator creates a new generator for transaction routes.
func NewTransactionRouteGenerator(e *entities.Entity, obs observability.Provider) TransactionRouteGenerator {
	g := &transactionRouteGenerator{obs: obs}
	if e != nil && e.TransactionRoutes != nil {
		g.transactionRoutes = e.TransactionRoutes
	}

	return g
}

// Generate creates a single transaction route from the provided input.
func (g *transactionRouteGenerator) Generate(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	ctx = normalizeContext(ctx)

	if g.transactionRoutes == nil {
		return nil, errors.New("entity transaction routes service not initialized")
	}

	if input == nil {
		return nil, errors.New("transaction route input is required")
	}

	validationErr := input.Validate()
	if validationErr != nil {
		return nil, validationErr
	}

	var out *models.TransactionRoute

	err := observability.WithSpan(ctx, g.obs, "GenerateTransactionRoute", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				tr, err := g.transactionRoutes.Create(ctx, organizationID, ledgerID, input)
				if err != nil {
					return err
				}

				out = tr

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, errNilGenerated("transaction route")
	}

	return out, nil
}

// GenerateDefaults creates default transaction routes for common flows.
// Requires the operation routes (by ID) already created via OperationRouteGenerator.
//
//nolint:cyclop,revive // The explicit route assembly keeps demo route names and dependencies visible.
func (g *transactionRouteGenerator) GenerateDefaults(ctx context.Context, organizationID, ledgerID string, opRoutes []*models.OperationRoute) ([]*models.TransactionRoute, error) {
	// Map titles for convenience
	byTitle := map[string]string{}

	for _, or := range opRoutes {
		if or == nil {
			return nil, errors.New("operation route list contains nil route")
		}

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

		tr, err := g.Generate(ctx, organizationID, ledgerID, input)
		if err != nil {
			return nil, err
		}

		routes = append(routes, tr)
	}

	fundingOps := []string{}
	if sourceExternalID, ok := byTitle["Source: External (any)"]; ok {
		fundingOps = append(fundingOps, sourceExternalID)
	}

	if destCustomerID, ok := byTitle["Destination: Customer (CHECKING)"]; ok {
		fundingOps = append(fundingOps, destCustomerID)
	}

	if len(fundingOps) >= 2 {
		input := models.NewCreateTransactionRouteInput("External Funding Flow", "External source funds customer account", fundingOps).
			WithMetadata(map[string]any{"pattern": "external_funding"})

		tr, err := g.Generate(ctx, organizationID, ledgerID, input)
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

		tr, err := g.Generate(ctx, organizationID, ledgerID, input)
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

		tr, err := g.Generate(ctx, organizationID, ledgerID, input)
		if err != nil {
			return nil, err
		}

		routes = append(routes, tr)
	}

	return routes, nil
}
