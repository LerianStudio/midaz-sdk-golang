// Package integrity provides post-flight integrity checks for Midaz
// resources. After a transaction lands, you can use this package to
// verify that the persisted state matches the expected end state —
// account balances, transaction status, operation counts.
//
// The checks are eventually-consistent-aware: each check accepts a
// retry-with-backoff policy so a temporarily-stale read replica
// doesn't produce a false negative. The default policy retries every
// 200ms for up to 5 seconds.
//
// # Public surface
//
//   - [Checker] — entry point. Wraps a [github.com/LerianStudio/midaz-sdk-golang/v3.Client]
//     plus optional retry/timeout knobs.
//   - Per-check methods: [Checker.CheckAccountBalance],
//     [Checker.CheckTransactionStatus], etc.
//   - Result types carry expected vs actual values for actionable
//     debug output.
//
// # When to use
//
//   - Test suites that assert end-state correctness after
//     transaction creation (post-conditions in the BDD sense).
//   - Operational tooling that audits a sample of resources for
//     drift after a migration or schema change.
//
// # When NOT to use
//
//   - Hot-path application code: integrity checks are read-amplifying
//     and synchronous. Use observability signals instead.
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability] —
//     structured logging + tracing for production
//   - examples/mass-demo-generator — uses this package post-generation
package integrity
