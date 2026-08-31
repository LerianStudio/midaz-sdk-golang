// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import "github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"

// Midaz serves TWO ledger surfaces at once and the SDK mirrors that split
// instead of hiding it.
//
// The server carries the version inside every operation path ("/v1/organizations",
// "/v2/organizations"), deprecated all of /v1 while keeping it alive, and did NOT
// mirror every resource across the two. The asymmetry is real and permanent for as
// long as /v1 lives:
//
//   - asset rates exist ONLY on /v1 — /v2 dropped them;
//   - the transaction creation styles (json, inflow, outflow, annotation) exist
//     ONLY on /v1 — /v2 replaced them with top-level direct/hold;
//   - holders, instruments, encryption, composition, protection audit and the
//     whole billing family exist ONLY on /v2 — /v1 had them removed;
//   - the billing family also MOVED scope between versions, from organization to
//     ledger.
//
// Grouping the accessors by version turns that asymmetry into a compile error
// instead of a 404. A caller reaching for [V1Services.AssetRates] or
// [V2Services.Holders] is told at build time which surface serves what, and there
// is no accessor whose behavior silently depends on a base-URL version segment
// (the Ledger base URL is bare — see [normalizeServiceURL]).
//
// The Tracer plane is NOT grouped: it versions itself in its base URL, serves one
// surface, and its accessors stay flat on [Entity] (Rules, Limits, Validations,
// Reservations, AuditEvents).

// V1Services is the Midaz /v1 ledger surface: deprecated server-side but alive,
// and still the only version that serves asset rates and the legacy transaction
// creation styles.
//
// Reached as client.V1.<Service> (promoted through the embedded *Entity). Held
// by VALUE on Entity — see the V1 field there for why.
type V1Services struct {
	// Organizations is the tenant root. Also exposes Count.
	Organizations *organizationsFacade

	// Ledgers is the ledger surface. Also exposes GetSettings / UpdateSettings
	// and Count.
	Ledgers *ledgersFacade

	// Accounts is the account surface. Also exposes Count.
	Accounts *accountsFacade

	// AccountTypes is the account-type surface.
	AccountTypes *accountTypesFacade

	// Assets is the asset surface. Also exposes Count.
	Assets *assetsFacade

	// AssetRates is the asset-rate surface. V1 ONLY — /v2 dropped it, so there is
	// deliberately no V2 twin.
	AssetRates *assetRatesFacade

	// Balances is the balance surface, including the point-in-time history reads
	// and the alias / external-code account lookups.
	Balances *balancesFacade

	// Operations is the account-scoped operation surface plus the
	// transaction-scoped operation update.
	Operations *operationsFacade

	// Portfolios is the portfolio surface. Also exposes Count.
	Portfolios *portfoliosFacade

	// Segments is the segment surface. Also exposes Count.
	Segments *segmentsFacade

	// OperationRoutes is the operation-route surface.
	OperationRoutes *operationRoutesFacade

	// TransactionRoutes is the transaction-route surface.
	TransactionRoutes *transactionRoutesFacade

	// Transactions is the transaction surface: the four V1 creation styles
	// (json, inflow, outflow, annotation), the commit / revert / cancel
	// transitions, the reads, the patch, and Count.
	Transactions *transactionsFacade

	// MetadataIndexes is the global metadata-index surface (/v1/settings/...,
	// not organization-scoped).
	MetadataIndexes *metadataIndexesFacade
}

// V2Services is the current Midaz /v2 ledger surface, and the one to build
// against: /v1 is deprecated server-side.
//
// It is WIDER than [V1Services], not narrower. Every dual-served family has its
// twin here, the families Midaz removed from /v1 exist only here, and the only
// things /v1 still serves alone are asset rates and the four legacy transaction
// creation styles that /v2 replaced with direct/hold.
//
// Two members differ in SHAPE from their V1 namesake rather than just in path,
// and a caller moving across will meet both:
//
//   - Transactions creates at the TOP level (/v2/transactions/direct) with the
//     organization and ledger travelling in the body, and answers with
//     [models.TransactionV2] — four /v1 fields fewer, two more.
//   - Accounts does NOT re-expose the account-scoped balance and operation
//     reads. /v1 spells those on two facades each; /v2 spells them once, on
//     Balances and Operations.
//
// Reached as client.V2.<Service> (promoted through the embedded *Entity). Held
// by VALUE on Entity — see the V1 field there for why.
type V2Services struct {
	// Organizations is the tenant root. Also exposes Count.
	Organizations *organizationsV2Facade

	// Ledgers is the ledger surface. Also exposes GetSettings / UpdateSettings
	// and Count.
	Ledgers *ledgersV2Facade

	// Accounts is the account surface, including the alias and external-code
	// lookups. The account-scoped balance and operation reads live on Balances
	// and Operations — see the type doc.
	Accounts *accountsV2Facade

	// AccountTypes is the account-type surface. No count endpoint exists on
	// either version.
	AccountTypes *accountTypesV2Facade

	// Assets is the asset surface. Also exposes Count.
	Assets *assetsV2Facade

	// Balances is the balance surface: the ledger-wide and account-scoped lists,
	// the point-in-time reads, the alias and external-code lookups, and the
	// balance writes.
	Balances *balancesV2Facade

	// Operations is the account-scoped operation surface plus the
	// transaction-scoped operation update.
	Operations *operationsV2Facade

	// Portfolios is the portfolio surface. Also exposes Count.
	Portfolios *portfoliosV2Facade

	// Segments is the segment surface. Also exposes Count.
	Segments *segmentsV2Facade

	// OperationRoutes is the operation-route surface.
	OperationRoutes *operationRoutesV2Facade

	// TransactionRoutes is the transaction-route surface.
	TransactionRoutes *transactionRoutesV2Facade

	// Transactions is the transaction surface: the four top-level creates
	// (direct, hold, block, unblock), the commit / revert / cancel transitions,
	// the reads, the patch and Count. Its creates and its responses both differ
	// in shape from V1 — see the type doc.
	Transactions *transactionsV2Facade

	// MetadataIndexes is the global metadata-index surface
	// (/v2/settings/metadata-indexes, not organization-scoped).
	MetadataIndexes *metadataIndexesV2Facade

	// Holders is the holder surface (organization-scoped). V2 ONLY.
	Holders *holdersFacade

	// Instruments is the instrument surface — the resource /v1 served as
	// "aliases" before Midaz renamed it. V2 ONLY.
	Instruments *instrumentsFacade

	// Encryption is the field-encryption provisioning and status surface.
	// V2 ONLY.
	Encryption *encryptionFacade

	// Composition creates a holder and its account atomically, bridging the
	// holder and ledger surfaces. V2 ONLY.
	Composition *compositionFacade

	// ProtectionAudit reads the protection audit trail. V2 ONLY.
	ProtectionAudit *auditFacade

	// BillingPackages is the billing-package surface. V2 ONLY, and
	// LEDGER-scoped: the billing family moved from organization scope to ledger
	// scope in V2, so every method here takes a ledger ID.
	BillingPackages *billingPackagesFacade

	// FeePackages is the fee-package surface (/v2/.../packages). V2 ONLY and
	// ledger-scoped, same move as BillingPackages.
	FeePackages *feePackagesFacade

	// FeeEstimates estimates fees (/v2/.../estimates). V2 ONLY and
	// ledger-scoped.
	FeeEstimates *feeEstimateFacade

	// BillingCalculations calculates billing (/v2/.../billing/calculate).
	// V2 ONLY and ledger-scoped. Money-adjacent: the ledger in the path and the
	// ledgerId in the body are reconciled before the request leaves the SDK.
	BillingCalculations *billingCalculateFacade
}

// newV1Services wires every V1 ledger accessor over one plane client.
func newV1Services(ledger *genledger.ClientWithResponses, enableIdempotency bool) V1Services {
	return V1Services{
		Organizations:     newOrganizationsFacade(ledger, enableIdempotency),
		Ledgers:           newLedgersFacade(ledger, enableIdempotency),
		Accounts:          newAccountsFacade(ledger, enableIdempotency),
		AccountTypes:      newAccountTypesFacade(ledger, enableIdempotency),
		Assets:            newAssetsFacade(ledger, enableIdempotency),
		AssetRates:        newAssetRatesFacade(ledger, enableIdempotency),
		Balances:          newBalancesFacade(ledger, enableIdempotency),
		Operations:        newOperationsFacade(ledger, enableIdempotency),
		Portfolios:        newPortfoliosFacade(ledger, enableIdempotency),
		Segments:          newSegmentsFacade(ledger, enableIdempotency),
		OperationRoutes:   newOperationRoutesFacade(ledger, enableIdempotency),
		TransactionRoutes: newTransactionRoutesFacade(ledger, enableIdempotency),
		Transactions:      newTransactionsFacade(ledger, enableIdempotency),
		MetadataIndexes:   newMetadataIndexesFacade(ledger, enableIdempotency),
	}
}

// newV2Services wires every V2 ledger accessor over one plane client.
func newV2Services(ledger *genledger.ClientWithResponses, enableIdempotency bool) V2Services {
	return V2Services{
		Organizations:       newOrganizationsV2Facade(ledger, enableIdempotency),
		Ledgers:             newLedgersV2Facade(ledger, enableIdempotency),
		Accounts:            newAccountsV2Facade(ledger, enableIdempotency),
		AccountTypes:        newAccountTypesV2Facade(ledger, enableIdempotency),
		Assets:              newAssetsV2Facade(ledger, enableIdempotency),
		Balances:            newBalancesV2Facade(ledger, enableIdempotency),
		Operations:          newOperationsV2Facade(ledger, enableIdempotency),
		Portfolios:          newPortfoliosV2Facade(ledger, enableIdempotency),
		Segments:            newSegmentsV2Facade(ledger, enableIdempotency),
		OperationRoutes:     newOperationRoutesV2Facade(ledger, enableIdempotency),
		TransactionRoutes:   newTransactionRoutesV2Facade(ledger, enableIdempotency),
		Transactions:        newTransactionsV2Facade(ledger, enableIdempotency),
		MetadataIndexes:     newMetadataIndexesV2Facade(ledger, enableIdempotency),
		Holders:             newHoldersFacade(ledger, enableIdempotency),
		Instruments:         newInstrumentsFacade(ledger, enableIdempotency),
		Encryption:          newEncryptionFacade(ledger, enableIdempotency),
		Composition:         newCompositionFacade(ledger, enableIdempotency),
		ProtectionAudit:     newAuditFacade(ledger),
		BillingPackages:     newBillingPackagesFacade(ledger, enableIdempotency),
		FeePackages:         newFeePackagesFacade(ledger, enableIdempotency),
		FeeEstimates:        newFeeEstimateFacade(ledger),
		BillingCalculations: newBillingCalculateFacade(ledger),
	}
}
