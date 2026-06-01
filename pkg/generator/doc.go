// Package generator builds realistic Midaz resource graphs (organizations,
// ledgers, assets, accounts, balances, portfolios, segments, routes,
// transactions) for demos, integration tests, and load testing.
//
// The package is the engine behind examples/mass-demo-generator. It
// composes data templates from
// [github.com/LerianStudio/midaz-sdk-golang/v4/pkg/data] with concrete
// SDK calls, applies bounded concurrency, and emits structured progress
// reports.
//
// # Public surface
//
//   - Per-resource generators: [OrganizationGenerator], [LedgerGenerator],
//     [AssetGenerator], [AccountGenerator], [PortfolioGenerator],
//     [SegmentGenerator], [TransactionGenerator], etc. Each takes a
//     parent resource ID and produces N child resources.
//   - [GeneratorConfig] — top-level knobs: counts per resource, locale,
//     retry policy, observability provider.
//   - Context helpers — [WithCircuitBreaker], [WithLedgerID], [WithOrgID],
//     [WithOrgLocale], [WithWorkers] inject shared state across generator
//     calls (circuit-breaker primitive, default IDs, locale, worker pool
//     size).
//
// # Quickstart
//
//	// e is an *entities.Entity, obs is an observability.Provider —
//	// typically obtained from a configured midaz.Client.
//	g := generator.NewOrganizationGenerator(e, obs)
//	orgs, err := g.GenerateMany(ctx, 100)
//
// # When to use
//
//   - Seeding a fresh Midaz install with believable organizations,
//     accounts, and transaction history for QA / demos.
//   - Generating load-test scenarios with realistic transaction shapes.
//
// # When NOT to use
//
// Production code. Generators are NOT idempotent across runs — calling
// GenerateMany twice produces two distinct resource sets.
//
// # See also
//
//   - examples/mass-demo-generator — runnable end-to-end consumer
//   - [github.com/LerianStudio/midaz-sdk-golang/v4/pkg/data] — catalogs
//   - docs/examples.md — example and generator usage guidance
package generator
