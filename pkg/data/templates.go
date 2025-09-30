package data

import (
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// OrgTemplate models an organization blueprint with metadata constraints in mind.
type OrgTemplate struct {
	LegalName string
	TradeName string
	TaxID     string // CNPJ/EIN format validation placeholder
	Address   models.Address
	Status    models.Status  // ACTIVE, INACTIVE, PENDING, etc.
	Metadata  map[string]any // max 100 char keys, 2000 values (validated later)
	Industry  string         // for metadata categorization
	Size      string         // small, medium, large, enterprise
}

// AssetTemplate models an asset blueprint with scale/precision configured.
type AssetTemplate struct {
	Name     string
	Type     string // currency, security, commodity
	Code     string // ISO codes for currencies
	Scale    int    // Decimal places (2 for USD, 8 for BTC)
	Metadata map[string]any
}

// AccountTemplate models account creation with type and relationships.
type AccountTemplate struct {
	Name            string
	Type            string // REQUIRED: checking, savings, creditCard, expense
	Status          models.Status
	Alias           *string // Must not start with @external/
	ParentAccountID *string // For hierarchy
	PortfolioID     *string
	SegmentID       *string
	EntityID        *string // External system reference
	AccountTypeKey  *string // Optional: explicit link to an account type key
	Metadata        map[string]any
}

// LedgerTemplate models basic ledger creation options.
type LedgerTemplate struct {
	Name     string
	Status   models.Status
	Metadata map[string]any
	// Additional fields for currency scope, region, etc., can be appended as needed
}

// TransactionPattern captures DSL-based generation parameters.
type TransactionPattern struct {
	ChartOfAccountsGroupName string
	Description              string
	DSLTemplate              string // DSL script template
	RequiresCommit           bool   // For pending transactions
	IdempotencyKey           string // UUID for dedup
	ExternalID               string // External reference
	Metadata                 map[string]any
}

// StrPtr is a small helper to create a *string from a string literal when building templates.
func StrPtr(s string) *string { return &s }
