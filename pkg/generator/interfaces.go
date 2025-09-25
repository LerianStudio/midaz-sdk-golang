package generator

import (
	"context"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
)

// OrganizationGenerator creates organizations from templates or batches.
type OrganizationGenerator interface {
	Generate(ctx context.Context, template data.OrgTemplate) (*models.Organization, error)
	GenerateBatch(ctx context.Context, count int) ([]*models.Organization, error)
}

// LedgerGenerator creates ledgers and supports pagination listing.
type LedgerGenerator interface {
	Generate(ctx context.Context, orgID string, template data.LedgerTemplate) (*models.Ledger, error)
	GenerateForOrg(ctx context.Context, orgID string, count int) ([]*models.Ledger, error)
	ListWithPagination(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error)
}

// AssetGenerator creates assets and manages their rates.
type AssetGenerator interface {
	Generate(ctx context.Context, ledgerID string, template data.AssetTemplate) (*models.Asset, error)
	GenerateWithRates(ctx context.Context, ledgerID string, baseAsset string) error
	UpdateRates(ctx context.Context, ledgerID string, rates map[string]float64) error
}

// TransactionGenerator creates transactions based on DSL patterns.
type TransactionGenerator interface {
	GenerateWithDSL(ctx context.Context, orgID, ledgerID string, pattern data.TransactionPattern) (*models.Transaction, error)
	GenerateBatch(ctx context.Context, orgID, ledgerID string, patterns []data.TransactionPattern, tps float64) ([]*models.Transaction, error)
}

// TransactionLifecycle manages transaction states (pending/commit/revert).
type TransactionLifecycle interface {
	CreatePending(ctx context.Context, input *models.CreateTransactionInput) (*models.Transaction, error)
	Commit(ctx context.Context, txID string) error
	Revert(ctx context.Context, txID string) error
	HandleInsufficientFunds(ctx context.Context, err error) error
}

// Generator orchestrates high-level generation flows using the configured
// sub-generators and the provided GeneratorConfig.
type Generator interface {
	Run(ctx context.Context, cfg GeneratorConfig) error
}

// AccountTypeGenerator creates a set of account types for a ledger.
type AccountTypeGenerator interface {
	Generate(ctx context.Context, orgID, ledgerID string, name, key string, metadata map[string]any) (*models.AccountType, error)
	GenerateDefaults(ctx context.Context, orgID, ledgerID string) ([]*models.AccountType, error)
}

// AccountGenerator creates accounts with hierarchy and metadata applied.
type AccountGenerator interface {
	Generate(ctx context.Context, orgID, ledgerID, assetCode string, template data.AccountTemplate) (*models.Account, error)
	GenerateBatch(ctx context.Context, orgID, ledgerID, assetCode string, templates []data.AccountTemplate) ([]*models.Account, error)
}
