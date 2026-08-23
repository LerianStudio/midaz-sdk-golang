// Package midaz re-exports the most commonly used types from the models package
// so that user code can stay on a single import path:
//
//	import "github.com/LerianStudio/midaz-sdk-golang/v5"
//
// Without the aliases below, every typical user file would also need:
//
//	import "github.com/LerianStudio/midaz-sdk-golang/v5/models"
//
// All aliases use Go's `type X = Y` form, which preserves type identity. That
// means `midaz.Account` and `models.Account` are interchangeable — the same
// underlying type with two names. Power users who need the full type catalog
// (auxiliary builders, deprecated types, internal request shapes) can still
// import the models package directly.
//
// Aliases are grouped into thematic blocks. Adding a new type here is a
// deliberate API surface decision: prefer pulling in only the types that
// appear in everyday code paths (resources, top-level inputs, transaction
// sub-DTOs, and pagination).

package midaz

import (
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

// The aliases below are intentionally undocumented per-line: each alias just
// re-exports the type of the same name from the models package, and the
// package-level commentary above already documents the contract. Adding 56
// near-identical "// X is an alias for models.X" comments would be noise
// without information. Lookup is "// X is exported from models" — Go's
// godoc tool follows the alias and surfaces the source type's doc directly,
// which is the canonical doc users should read.
//
//revive:disable:exported

// -----------------------------------------------------------------------------
// Resource entities (read shapes returned by the API).
// -----------------------------------------------------------------------------

type (
	Account          = models.Account
	AccountType      = models.AccountType
	Alias            = models.Alias
	Asset            = models.Asset
	AssetRate        = models.AssetRate
	Balance          = models.Balance
	Holder           = models.Holder
	Ledger           = models.Ledger
	MetadataIndex    = models.MetadataIndex
	Operation        = models.Operation
	OperationRoute   = models.OperationRoute
	Organization     = models.Organization
	Portfolio        = models.Portfolio
	Segment          = models.Segment
	Transaction      = models.Transaction
	TransactionRoute = models.TransactionRoute
)

// -----------------------------------------------------------------------------
// Create inputs (write requests).
// -----------------------------------------------------------------------------

type (
	CreateAccountInput          = models.CreateAccountInput
	CreateAccountTypeInput      = models.CreateAccountTypeInput
	CreateAliasInput            = models.CreateAliasInput
	CreateAssetInput            = models.CreateAssetInput
	CreateAssetRateInput        = models.CreateAssetRateInput
	CreateBalanceInput          = models.CreateBalanceInput
	CreateHolderInput           = models.CreateHolderInput
	CreateLedgerInput           = models.CreateLedgerInput
	CreateMetadataIndexInput    = models.CreateMetadataIndexInput
	CreateOperationRouteInput   = models.CreateOperationRouteInput
	CreateOrganizationInput     = models.CreateOrganizationInput
	CreatePortfolioInput        = models.CreatePortfolioInput
	CreateSegmentInput          = models.CreateSegmentInput
	CreateTransactionInput      = models.CreateTransactionInput
	CreateTransactionRouteInput = models.CreateTransactionRouteInput
)

// -----------------------------------------------------------------------------
// Update inputs (patch requests).
// -----------------------------------------------------------------------------

type (
	UpdateAccountInput          = models.UpdateAccountInput
	UpdateAccountTypeInput      = models.UpdateAccountTypeInput
	UpdateAliasInput            = models.UpdateAliasInput
	UpdateAssetInput            = models.UpdateAssetInput
	UpdateBalanceInput          = models.UpdateBalanceInput
	UpdateHolderInput           = models.UpdateHolderInput
	UpdateLedgerInput           = models.UpdateLedgerInput
	UpdateOperationInput        = models.UpdateOperationInput
	UpdateOperationRouteInput   = models.UpdateOperationRouteInput
	UpdateOrganizationInput     = models.UpdateOrganizationInput
	UpdatePortfolioInput        = models.UpdatePortfolioInput
	UpdateSegmentInput          = models.UpdateSegmentInput
	UpdateTransactionInput      = models.UpdateTransactionInput
	UpdateTransactionRouteInput = models.UpdateTransactionRouteInput
)

// -----------------------------------------------------------------------------
// Transaction sub-DTOs (compose CreateTransactionInput and UpdateTransactionInput).
// -----------------------------------------------------------------------------

type (
	AmountInput     = models.AmountInput
	DistributeInput = models.DistributeInput
	FromToInput     = models.FromToInput
	SendInput       = models.SendInput
	SourceInput     = models.SourceInput
)

// -----------------------------------------------------------------------------
// Pagination and list response (used by every Service.List* method).
// -----------------------------------------------------------------------------

type (
	Pagination          = models.Pagination
	ListResponse[T any] = models.ListResponse[T]
)

// -----------------------------------------------------------------------------
// Common cross-cutting types.
// -----------------------------------------------------------------------------

type (
	Status  = models.Status
	Address = models.Address
)

// -----------------------------------------------------------------------------
// Auth — re-exported from pkg/auth so a static-credential setup needs only
// the midaz package import:
//
//	c, err := midaz.New(midaz.WithAccessManager(midaz.AccessManager{
//	    Address:      "https://auth.midaz.io",
//	    ClientID:     "abc",
//	    ClientSecret: "xyz",
//	}))
//
// The Enabled field on AccessManager is auto-populated by midaz.WithAccessManager
// — callers should not set it themselves.
// -----------------------------------------------------------------------------

type (
	AccessManager = auth.AccessManager
)

// -----------------------------------------------------------------------------
// Environment — re-exported from pkg/config so client construction needs only
// the midaz package import:
//
//	c, err := midaz.New(
//	    midaz.WithEnvironment(midaz.EnvironmentProduction),
//	    midaz.WithAccessManager(midaz.AccessManager{ ... }),
//	)
//
// The three values cover every Midaz deployment shape:
//   - [EnvironmentLocal] — developer machine running the open-source
//     stack via docker-compose. No auth by default.
//   - [EnvironmentDevelopment] — shared development / staging cluster.
//   - [EnvironmentProduction] — production traffic. Requires Access
//     Manager authentication; anonymous clients return a typed
//     configuration error at construction time.
//
// See also [WithEnvironment], [WithAccessManager], docs/auth.md.
// -----------------------------------------------------------------------------

type (
	Environment = config.Environment
)

const (
	EnvironmentLocal       = config.EnvironmentLocal
	EnvironmentDevelopment = config.EnvironmentDevelopment
	EnvironmentProduction  = config.EnvironmentProduction
)
