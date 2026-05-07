// Package data provides reference catalogs and template data used by
// the demo generator (examples/mass-demo-generator) and by integration
// tests that need realistic-looking inputs without hitting external
// data sources.
//
// Catalogs in this package are deliberately small (a few dozen entries
// each) and locale-aware where it matters — for example, the asset
// catalog holds ISO 4217 currency codes plus illustrative
// commodity/index instruments; the accounts catalog provides typical
// chart-of-accounts names; the organizations catalog provides
// realistic legal-name + legal-document pairs for both en-US and pt-BR
// locales.
//
// # When to use this package
//
//   - Authoring a demo / smoke-test program that needs deterministic
//     but plausible Midaz data without authoring it by hand.
//   - Validating against representative inputs in unit tests
//     (Validate methods on amounts, asset codes, etc.).
//
// # When NOT to use this package
//
// Production code paths. The catalogs are illustrative — they are not
// guaranteed to be exhaustive, internationalized, or kept current with
// regulatory updates. For production data, source your own catalogs.
//
// # See also
//
//   - examples/mass-demo-generator — primary consumer
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/generator] —
//     higher-level generator that composes catalogs from this package
package data
