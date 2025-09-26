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
    if err := input.Validate(); err != nil {
        return nil, err
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
    if id, ok := byTitle["Source: Customer (CHECKING)"]; ok {
        payOps = append(payOps, id)
    }
    if id, ok := byTitle["Destination: Merchant (CHECKING)"]; ok {
        payOps = append(payOps, id)
    }
    if id, ok := byTitle["Destination: Platform Fee (alias)"]; ok {
        payOps = append(payOps, id)
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
    if id, ok := byTitle["Destination: Merchant (CHECKING)"]; ok {
        // Use merchant as source by referencing same operation route in a transaction route context
        refundOps = append(refundOps, id)
    }
    if id, ok := byTitle["Destination: Customer (CHECKING)"]; ok {
        refundOps = append(refundOps, id)
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
    if id, ok := byTitle["Source: Customer (CHECKING)"]; ok {
        transferOps = append(transferOps, id)
    }
    if id, ok := byTitle["Destination: Customer (CHECKING)"]; ok {
        transferOps = append(transferOps, id)
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
