# Midaz Go SDK v3 — DX Plan

> **Status:** IN PROGRESS — Phase A active (Track 1 at 5 of 7 batches done)
> **Owner:** Fred (Lerian)
> **Last updated:** 2026-05-05 (post-session 1)
> **Scope:** Greenfield v3 with breaking-change budget. Deprecated shims acceptable for 1–2 minor versions to ease migration.

---

## ⚡ Resume State (read this first if picking up the work)

**Current branch:** `v3` (created from `develop`, **NOT yet pushed to origin**)

**Last commit on v3:** `588997a feat(v3): re-export 56 hot model types at the midaz package level`

**Total v3 commits so far:** 12 (3 docs + 8 feat + 1 chore)

```
588997a feat(v3): re-export 56 hot model types at midaz package      ← Batch 1G ✅ — Track 1 CLOSED
ab98a64 chore(v3): sweep 11 pre-existing lint issues                 ← Lint sweep ✅
36f292f docs(v3): record Batch 1E completion in plan
ab69e05 feat(v3)!: collapse entities/ constructors to single entry   ← Batch 1E ✅
507a64e docs(v3): update plan with session 1 progress
582dd99 feat(v3): eager validation contract at midaz.New()           ← Batch 1F ✅
eb16c8e docs(v3): update status tracker after batches 1A-1D
af8bbea feat(v3)!: introduce pkg/sdkctx for per-request helpers      ← Batch 1D ✅
d5e410a feat(v3)!: hoist services to top-level Client                ← Batch 1C ✅
f2a0fee feat(v3)!: always initialize Entity surface; delete Use*     ← Batch 1B ✅
f8e2109 feat(v3)!: rename module to /v3 and root package to midaz    ← Batch 1A ✅
424aae0 docs(v3): add comprehensive v3 DX plan                       ← This doc
```

**Verification status (last run end of Batch 1G):**
- `go build ./...` → clean
- `go test ./...` → 27/27 packages green, zero failures (59 new tests added in Batch 1G)
- `make verify-sdk` → ✅ clean
- `make lint` → ✅ 0 issues (lint sweep brought codebase to fully clean state)

**Track 1 — STATUS: COMPLETE.** All 7 batches (1A-1G) shipped. Acceptance criteria status:
- ✅ Module path → `/v3`
- ✅ Root package → `midaz`
- ✅ Use* trio deleted
- ✅ Services hoisted via embedded `*entities.Entity` (`c.Accounts.X` works; back-compat `c.Entity.Accounts.X` works)
- ✅ `pkg/sdkctx/` package with 5 helpers
- ✅ Eager validation at `midaz.New()` (`errors.IsConfigurationError(err)` works)
- ✅ 16 service constructors unexported; 3 redundant entity constructors deleted; `WithContext` no-op deleted
- ✅ 56 type aliases on root midaz package
- ⏸️ `WithAuthToken` → deferred to **Track 2**
- ⏸️ `Logger()` always non-nil → deferred to **Track 4**
- ⏸️ Examples migration → deferred to **Track 9**

**Next action when resuming:**
1. **Track 2** (Auth & Tenant Chaos) — adds `midaz.WithAuthToken(token)` option, re-exports `AccessManager`, renames `pkg/access-manager` → `pkg/auth`, makes auth source required at construction (`midaz.New()` with no auth source returns typed error `"no auth source configured; use WithAuthToken, WithAccessManager, or WithAnonymous"`).
2. **Track 3** (Implicit env-var reads) — independent of Track 2; can run parallel. Eliminates the 14× `os.Getenv("MIDAZ_DEBUG")` reads in `entities/*.go` plus `MIDAZ_USER_AGENT`, `MIDAZ_IDEMPOTENCY`, `MIDAZ_MAX_RETRIES`, `MIDAZ_ENABLE_RETRIES`, `MIDAZ_SKIP_AUTH_CHECK` (all reads should go through `*config.Config`).
3. **Track 4** (Logging gap) — depends on Track 3. Introduces `*slog.Logger` as canonical logging contract; deletes `MIDAZ_DEBUG` bypass; adds retry-attempt logging.

**Important context preserved for the next session:**

- The migration uses **anonymous embedding** of `*entities.Entity` in `Client`, which gives both `c.Accounts.X` (new v3 idiom) and `c.Entity.Accounts.X` (back-compat) for free. This is intentional and works because Go's embedded-field name is the type name without the pointer.
- All implicit env-var reads in `entities/*.go` (the 14 `MIDAZ_DEBUG` reads + `MIDAZ_USER_AGENT` + `MIDAZ_IDEMPOTENCY` + `MIDAZ_MAX_RETRIES` + `MIDAZ_ENABLE_RETRIES`) are still present. **Track 3 will eliminate them.** Don't worry about them in Track 1 batches.
- The 32 example files still use `client "github.com/.../v3"` import alias and `client.X` references. They work via Go's alias mechanism even though the package is now `midaz`. Track 9 (Phase C) rewrites examples to use the canonical `midaz.X` idiom.
- `pkg/transaction/batch.go` had a quirk: `goimports` (or similar tool) auto-inserted `github.com/moby/moby/client` when I dropped the `client` alias. Watch for this when modifying files that previously imported the root v3 package as `client`.
- An auto-tool also stripped `sdkerrors` import from `midaz.go` once after I added it. Re-add it explicitly if needed.
- The plan's original Batch 1D ambition was to fully delete `entities/` package. We deferred this — `entities/` still exists as a thin layer because services live there. Full deletion happens in Phase B (Track 7) when services move to per-service packages.
- **Resume-state correction from Batch 1E**: the original plan claimed 6 'factory traps' on Client (`NewAccount/NewLedger/...`) at `midaz.go:733-761`. They don't exist (verified via grep). Either deleted in 1A-1D, or never existed in v2. Scope item considered satisfied.
- **Batch 1E test infrastructure note**: `entities/entity_test.go` now hosts `newTestEntity(t, ...)` — a private test helper that mirrors the deleted `entities.NewEntity` contract. Tests inside `entities/*_test.go` cannot route through `midaz.New()` because of the import cycle direction (entities is a leaf), so this helper is the in-package replacement. External tests (`pkg/transaction/helper_contract_test.go`) DO use `midaz.New()` + `c.SetAuthToken(...)`.
- **Process improvement (Batch 1E discovery)**: `make lint` was apparently NOT run during Batches 1A-1F. 11 issues had accumulated. **Going forward, `make lint` is part of the verification flow alongside `go build`, `go test`, and `make verify-sdk`** for every batch.
- **Generic type aliases work in Go 1.26.x** (Batch 1G uses one: `ListResponse[T any] = models.ListResponse[T]`). Verified via `TestGenericListResponseAlias`. Useful pattern for future track work that needs to alias generic types.
- **Lint accommodation pattern (Batch 1G)**: when a file has many type aliases that are self-documenting (`X = models.X`), use `//revive:disable:exported` at file scope with a written rationale, instead of adding 50+ near-identical `// X is an alias for models.X` comments. Rationale: godoc follows the alias and surfaces the source doc directly, which is the canonical view users get.

**Customer-visible v3 changes shipped on the v3 branch so far:**

```go
// v2 (current main/develop) — 3 imports, 4 ways to construct, panic risk
import client "github.com/LerianStudio/midaz-sdk-golang/v2"
import "github.com/LerianStudio/midaz-sdk-golang/v2/entities"
import "github.com/LerianStudio/midaz-sdk-golang/v2/models"

c, _ := client.New(client.WithConfig(cfg), client.UseAllAPIs())  // panic if you forget UseAllAPIs
ctx = entities.WithIdempotencyKey(ctx, key)
ctx = entities.WithTenantID(ctx, tenant)  // confused with client.WithTenantID
e, _ := entities.NewEntity(httpClient, token, urls, nil)  // 4 redundant entity constructors
acc, _ := c.Entity.Accounts.GetAccount(ctx, ...)
input := models.CreateAccountInput{...}  // every input/output type qualified

// v3 (current v3 branch state) — 2 imports, 1 way to construct, typed errors
import "github.com/LerianStudio/midaz-sdk-golang/v3"
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"

c, err := midaz.New(midaz.WithConfig(cfg))      // Entity always init'd; eager validation
if errors.IsConfigurationError(err) {            // typed setup errors
    log.Fatalf("midaz misconfigured: %v", err)
}
ctx = sdkctx.WithIdempotencyKey(ctx, key)
ctx = sdkctx.WithRequestTenantID(ctx, tenant)    // unambiguous; renamed from WithTenantID
ctx = sdkctx.WithIncludeDeleted(ctx, true)       // NEW (Track 7 dependency)
ctx = sdkctx.WithHardDelete(ctx, true)           // NEW (Track 7 dependency)
acc, _ := c.Accounts.GetAccount(ctx, ...)        // hoisted via embedded Entity
input := midaz.CreateAccountInput{...}           // 56 hot types re-exported on midaz.*

// What's GONE in v3:
//   * entities.NewEntity / .New / .NewWithServiceURLs / .WithContext
//   * 16 NewXxxEntity service constructors (unexported)
//   * UseAllAPIs / UseEntityAPI / UseEntity trio (Entity always init'd)
//   * Models import for everyday work (56 type aliases on midaz package)
// midaz.New() is the canonical entry point. One way to do it.
```

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [The Convergence Insight](#the-convergence-insight)
3. [v3 Design Principles](#v3-design-principles)
4. [Tracks](#tracks)
   - [Track 1 — Naked SDK & Entry Points](#track-1--naked-sdk--entry-points)
   - [Track 2 — Auth & Tenant Chaos](#track-2--auth--tenant-chaos)
   - [Track 3 — Implicit Env-Var Reads](#track-3--implicit-env-var-reads)
   - [Track 4 — Logging Gap](#track-4--logging-gap)
   - [Track 5 — Pagination Footguns](#track-5--pagination-footguns)
   - [Track 6 — Functional Options Sprawl](#track-6--functional-options-sprawl)
   - [Track 7 — Builder/Model API Drift](#track-7--buildermodel-api-drift)
   - [Track 8 — Error System Actionability](#track-8--error-system-actionability)
   - [Track 9 — Examples, Godoc, Mocks](#track-9--examples-godoc-mocks)
5. [Sequencing & Dependencies](#sequencing--dependencies)
6. [Migration Story](#migration-story)
7. [Acceptance Criteria Summary](#acceptance-criteria-summary)
8. [Decision Log](#decision-log)
9. [Status Tracker](#status-tracker)

---

## Executive Summary

The Midaz Go SDK v2 has the right components but the wrong shape. After a 6-dimensional DX audit covering ~683 exported symbols, ~70 distinct findings emerged, clustering into **9 thematic tracks**. The root cause is **organic accretion across 3 design eras** without a convergence pass — same concept named three different ways, three different mechanisms to set tenant ID, four ways to enable observability, five pagination shapes, and 21 implicit env-var reads scattered through code that bypass the documented configuration system.

**v3 is a coherence release**, not a feature release. The SDK gets re-shaped around six design principles (below). Public-API breaking changes are accepted; deprecated shims preserve a 1–2 minor-version migration window for downstream consumers.

**Estimated total effort:** 12–15 weeks, executed in three phases:

| Phase | Tracks | Duration | Outcome |
|-------|--------|----------|---------|
| A — Foundation | 1, 2, 3, 4 | 4–5 weeks | Client surface shape; auth; env hygiene; slog logging |
| B — Models & data flow | 5, 6, 7, 8 | 4–6 weeks | Pagination iter.Seq2; option consolidation; builder unification; error contracts |
| C — Polish | 9 | 2–3 weeks | Examples, godoc, mocks migration |

**Quick wins from the audit are folded into v3** (per Fred's decision): no v2.x patch ship; the 10 surgical fixes (env-debug cleanup, error-context plumbing, network-error typing, broken doc fix, mocks migration, hello-world example, etc.) become part of the v3 commit history.

---

## The Convergence Insight

A first-time Go dev who lands on `pkg.go.dev/github.com/LerianStudio/midaz-sdk-golang/v2` and follows the README will:

1. Write `client.New()` with no options, get a `*Client` whose `c.Entity` is `nil`, and panic on the first call. (Naked SDK: dead.)
2. Discover they need `client.UseAllAPIs()`, then write code that points at `localhost:3002` because that's the default. (Now the SDK silently dials localhost in prod.)
3. Realize they have a `MIDAZ_AUTH_TOKEN` and look for `client.WithAuthToken` — **there is none.** They must build a `config.Config`, then a `*config.Config`, then pass it via `client.WithConfig(cfg)`. (Auth: 3-package import dance.)
4. Forget to set `MIDAZ_DEBUG=false` from CI shell, and now their production binary leaks request bodies to stderr because 14 entity constructors silently re-read the env var.
5. Try to integrate the SDK with their `slog` logger and find no `WithLogger` option exists. (Logging: not a feature.)
6. Hit a 404 on `GetAccount(orgID, ledgerID, "abc-123")` and see `not_found error for Account: Account not found` — **the SDK knows the resource ID but throws it away.**
7. Paginate transactions with `WithPage(2)` and silently get page 1 (transactions are cursor-only; the `Page` field is dropped to stderr).

Each papercut is fixable. **Fixing them together** produces a v3 that feels like a 2026-era SDK — not one that grew organically across three eras.

The convergence problem is structural. Solving it requires committing to **one canonical path per concept** and being ruthless about deleting the alternates.

---

## v3 Design Principles

These six principles are the litmus test for every v3 change. If a proposed fix violates one of them, it gets challenged before merging.

### 1. Convergence over compatibility

Same concept = same name = same shape, everywhere. We accept breaking changes to enforce this. Where backward compat matters, we ship deprecated shims with `// Deprecated:` markers and migration breadcrumbs.

### 2. Explicit over implicit

The SDK reads zero configuration from the environment unless the client explicitly opts in via `config.FromEnvironment()`. No more sneaky `os.Getenv` calls in entity constructors. No more "this defaults to true if env unset" logic. What you pass is what you get; what's in env stays in env until you ask.

### 3. Compile-time safety wherever Go allows

If we can prevent misuse at compile time (via per-service typed `ListOpts`, typed update inputs, generic responses), we do. We reduce the runtime "this option is silently ignored on this endpoint" surface area.

### 4. `*slog.Logger` is the logging contract

No bespoke Logger interface. No parallel `MIDAZ_DEBUG` stderr path. Users plug in `*slog.Logger` (stdlib or any compatible third-party). The SDK is silent by default (discard handler) and emits structured logs with a documented field schema when a logger is configured.

### 5. `iter.Seq2` is the iteration contract

Every list endpoint exposes `ListAll(ctx, ...) iter.Seq2[T, error]`. Users write `for x, err := range ...`. The SDK handles cursor preservation, filter copying, and termination internally. Single-page access remains available via `List(...)` for callers that need page metadata.

### 6. One canonical path per concept

- One way to set auth (with one variant for OAuth)
- One way to set tenant ID (one default + one per-request override)
- One way to enable observability
- One way to configure retries
- One way to construct an entity input
- One error type with documented field semantics
- One pagination shape per endpoint kind (page-based or cursor-based, never both visible to the caller)

Every alternate path either becomes a deprecated shim or gets deleted.

---

## Tracks

Each track below contains:
- **Severity** — CRITICAL / HIGH / MEDIUM
- **Effort** — S (days) / M (1–2 weeks) / L (2–4 weeks)
- **Phase** — A (Foundation) / B (Models & data flow) / C (Polish)
- **Findings table** — the audit-discovered issues with `file:line` references
- **Proposed v3 shape** — concrete API design with code samples
- **Acceptance criteria** — measurable bar for "this track is done"
- **Dependencies** — which tracks must land first

---

### Track 1 — Naked SDK & Entry Points

**Severity:** CRITICAL · **Effort:** M · **Phase:** A · **Dependencies:** none (foundation)

This is the highest-leverage track. The single largest barrier to onboarding is what happens when a user types `client.New(...)`.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 1.1 | CRITICAL | `client.New()` returns client with `c.Entity == nil` → panics on first call | `client.go:38, :88, :472-489` |
| 1.2 | CRITICAL | No public `WithAuthToken` exists; static-token users have no documented path | `client.go:176-606` (no auth option), `entities/entity.go:139` (legacy hidden) |
| 1.3 | HIGH | `DefaultConfig()` hardcodes `EnvironmentLocal` → silent localhost in prod | `pkg/config/config.go:1077-1101`, `client.go:74` |
| 1.4 | HIGH | No fail-fast validation at `client.New()` — errors deferred to first request | `client.go:56-95`, `pkg/config/config.go:856-876` (validateConfig bypassed) |
| 1.5 | HIGH | 4+ paths to set auth, no canonical doc, undocumented precedence | `client.go` (none), `pkg/config/config.go:535`, `entities/options.go:144`, `entities/entity.go:501`, env vars |
| 1.6 | HIGH | Tenant-ID has 4 setters, subtle precedence rules | `client.go:599`, `pkg/config/config.go:515`, `entities/options.go:127`, `entities/context.go:47` |
| 1.7 | MEDIUM | `client.WithAccessManager`/`WithIdempotency`/`WithMaxRetries` not exposed at client level | `client.go:176-615` (missing); requires `WithConfig(cfg)` indirection |
| 1.8 | MEDIUM | Top-level package exports nothing useful — common types require `models/` import | `client.go:1-3`, `client.go:733-761` (empty-struct factories) |
| 1.9 | MEDIUM | 16 redundant public `entities.NewXxxEntity()` constructors compete with `client.New()` | `entities/{accounts,account_types,...}.go` × 16 |
| 1.10 | LOW | `Option` nil-error doesn't say which index was nil | `client.go:78-80` |
| 1.11 | LOW | `entities.WithContext` is a documented no-op | `entities/options.go:73-89` |
| 1.12 | LOW | `WithObservabilityOptions`/`WithObservability`/`WithObservabilityProvider`/`WithCollectorEndpoint` are 4 overlapping ways to do the same thing | `client.go:280-429` |
| 1.13 | LOW | Vestigial `Use*` trio (`UseAllAPIs`, `UseEntityAPI`, `UseEntity`) | `client.go:472-489, :613-615` |

#### Proposed v3 shape

**Constructor:**

```go
// client/client.go
package client

// New constructs a fully-initialized Midaz SDK client.
//
// Quickstart:
//
//   c, err := client.New(
//       client.WithEnvironment(client.EnvProduction),
//       client.WithAuthToken("midaz_pat_..."),
//       client.WithTenantID("acme-prod"),
//   )
//   if err != nil { return err }
//   defer c.Shutdown(ctx)
//
//   org, err := c.Organizations.Get(ctx, "org-id")
//
// New always validates configuration eagerly. It returns an actionable error if
// auth, URLs, or required fields are missing. There is no "naked" client.
func New(opts ...Option) (*Client, error) { /* ... */ }
```

**Mandatory options** (validated at `New()`):

- An auth source: one of `WithAuthToken(string)`, `WithAccessManager(AccessManager)`, or `WithAnonymous()` (test-only escape hatch)
- An environment or URL: one of `WithEnvironment(Environment)`, `WithBaseURL(string)`, or `FromEnvironment()`

If neither is provided, `New()` returns `&errors.Error{Category: ConfigurationError, Operation: "client.New", Message: "no auth source configured; use WithAuthToken, WithAccessManager, or WithAnonymous"}`.

**Tier 1 client options (everything a typical user needs):**

```go
// Authentication
WithAuthToken(token string) Option
WithAccessManager(am AccessManager) Option
WithAnonymous() Option // test-only; explicit opt-in to no-auth

// Environment & connectivity
WithEnvironment(env Environment) Option
WithBaseURL(url string) Option
WithOnboardingURL(url string) Option  // advanced
WithTransactionURL(url string) Option // advanced
WithCRMURL(url string) Option         // advanced
WithHTTPClient(client *http.Client) Option
WithTimeout(d time.Duration) Option
WithUserAgent(ua string) Option

// Tenant
WithTenantID(id string) Option

// Behavior
WithIdempotency(enabled bool) Option
WithDebug(enabled bool) Option // installs default debug-level slog handler if no logger configured
WithContext(ctx context.Context) Option

// Env loader (still opt-in)
FromEnvironment() Option

// Eager health check (off by default)
WithEagerCheck(enabled bool) Option
```

**Tier 2 advanced options (variadic delegation to subsystem packages):**

```go
WithLogger(logger *slog.Logger) Option
WithSlowCallThreshold(d time.Duration) Option
WithRetryOptions(opts ...retry.Option) Option
WithObservability(opts ...observability.Option) Option
WithObservabilityProvider(p observability.Provider) Option // BYO provider
WithRequestHook(fn RequestHook) Option
WithResponseHook(fn ResponseHook) Option
WithRedactedHeaders(headers ...string) Option
WithRedactedQueryParams(keys ...string) Option

// "Off" toggles (clear, explicit)
WithoutRetries() Option
WithoutTenantID() Option
WithoutObservability() Option
```

**Client struct shape:**

```go
type Client struct {
    Organizations      OrganizationsService
    Ledgers            LedgersService
    Accounts           AccountsService
    AccountTypes       AccountTypesService
    Assets             AssetsService
    AssetRates         AssetRatesService
    Balances           BalancesService
    Holders            HoldersService
    Aliases            AliasesService
    Portfolios         PortfoliosService
    Segments           SegmentsService
    Operations         OperationsService
    OperationRoutes    OperationRoutesService
    Transactions       TransactionsService
    TransactionRoutes  TransactionRoutesService
    MetadataIndexes    MetadataIndexesService

    // Observability surface (always non-nil; discard logger if not configured)
    Logger() *slog.Logger
    Tracer() trace.Tracer
    Meter()  metric.Meter

    // Lifecycle
    Shutdown(ctx context.Context) error
}
```

Notes:
- Services live **directly on `Client`**, not behind a nested `c.Entity.Accounts.X` struct. The `Entity` indirection is removed.
- `Client.Logger()` always returns a non-nil `*slog.Logger`. Callers can never deref `nil`.
- Top-level package re-exports common types (`Account`, `Transaction`, `ListOpts`, etc.) via aliases so users don't need to import `models/` separately for everyday work.

**Deletions:**

- `client.UseAllAPIs`, `client.UseEntityAPI`, `client.UseEntity` (Use* trio)
- `client.NewAccount/NewLedger/NewOrganization/NewTransaction/NewOperation/NewAsset` (empty-struct factory traps at `client.go:733-761`)
- `entities.NewEntity`, `entities.New`, `entities.NewWithServiceURLs`, `entities.NewEntityWithConfig` (move to internal)
- All 16 public `entities.NewXxxEntity` constructors (make unexported)
- `entities.WithContext` (documented no-op)
- `client.WithCollectorEndpoint` (subsumed by `WithObservability(observability.WithCollectorEndpoint(...))`)
- `client.WithObservability(bool, bool, bool)` 3-arg form (subsumed by variadic)
- `entities.SetAuthToken` post-hoc setter

#### Acceptance criteria

- [ ] **(Pending Track 2)** `midaz.New()` with zero options returns a typed error: `"no auth source configured; use WithAuthToken, WithAccessManager, or WithAnonymous"` — does not panic, does not silently produce a localhost client. *Currently New() with zero options succeeds because no auth check exists yet; Track 2 adds the WithAuthToken option and the auth-required check.*
- [x] **(Batch 1B)** `midaz.New()` with valid options returns a client where every service field is non-nil and immediately usable. ✅ Embedded `*entities.Entity` always initialized.
- [ ] **(Pending Track 2)** `midaz.WithAuthToken("...")` works as a single-line auth setup with no `pkg/config` imports.
- [ ] **(Pending Track 4)** `Client.Logger()` returns a non-nil `*slog.Logger` always (default = discard handler). *Currently Logger() can return nil; Track 4 fixes.*
- [x] **(Batch 1E)** All 16 `entities.NewXxxEntity` are unexported (`newAccountsEntity`, `newTransactionsEntity`, etc.); 3 redundant entity-level constructors deleted (`entities.NewEntity`, `entities.New`, `entities.NewWithServiceURLs`); `entities.WithContext` no-op deleted. Godoc on `entities` shows only the canonical entry path (`NewEntityWithConfig` for embedders + `NewHTTPClient` for access-manager + `InitServices` for plugin auth duck-typing).
- [x] **(Batch 1G)** Top-level `midaz` package re-exports **56 hot model types** via type aliases: 16 resource entities (Account, Transaction, Ledger, ...), 16 Create inputs, 14 Update inputs, 5 transaction sub-DTOs (AmountInput, FromToInput, SendInput, SourceInput, DistributeInput), 3 pagination/list types (including the generic `ListResponse[T any]`), 2 common types (Status, Address). Verified via 59 contract tests (typed-identity check + cross-package mutual assignability).
- [x] **(Batch 1F)** `midaz.New()` runs `c.config.Validate()` after applying options; validation errors are typed `*errors.Error{Category: CategoryConfiguration}` with operation context. ✅ `errors.IsConfigurationError(err)` works.
- [ ] **(Pending Phase C / Track 9)** Compile-time: no test in `examples/` uses `c.Entity.X.Y` form anymore (all migrate to `c.X.Y`). *Currently mass-replaced internally; examples still use `client.UseAllAPIs()`-era patterns. Track 9 polishes.*

**Additional Track 1 acceptance criteria delivered (not in original plan):**

- [x] **(Batch 1A)** Module path migrated to `github.com/LerianStudio/midaz-sdk-golang/v3`
- [x] **(Batch 1A)** Root package renamed `client` → `midaz`
- [x] **(Batch 1B)** `UseAllAPIs`, `UseEntityAPI`, `UseEntity` trio deleted; `useEntity` flag removed from `Client` struct
- [x] **(Batch 1C)** Services hoisted via embedded `*entities.Entity`: `c.Accounts`, `c.Transactions`, etc. work directly. Back-compat `c.Entity.Accounts` still works.
- [x] **(Batch 1D)** `pkg/sdkctx/` package created with 5 helpers: `WithIdempotencyKey`, `WithoutAutoIdempotency`, `WithRequestTenantID` (renamed from `WithTenantID`), `WithIncludeDeleted` (NEW), `WithHardDelete` (NEW)
- [x] **(Batch 1D)** `entities/context.go` reduced to deprecated shims that delegate to `sdkctx`
- [x] **(Batch 1F)** `errors.NewConfigurationError`, `errors.IsConfigurationError`, `errors.ErrConfiguration`, `errors.CategoryConfiguration`, `errors.CodeConfiguration` all added
- [x] **(Batch 1F)** Nil-option errors include the option's array index for debuggability
- [x] **(Batch 1F)** `pkg/config.Config.Validate()` is now a public method (was private `validateConfig`)
- [x] **(Batch 1F)** 11 new validation contract tests added in `validation_contract_test.go`
- [x] **(Batch 1D)** 8 new sdkctx tests added in `pkg/sdkctx/sdkctx_test.go`

---

### Track 2 — Auth & Tenant Chaos

**Severity:** CRITICAL · **Effort:** M · **Phase:** A · **Dependencies:** Track 1 (uses the new client surface)

Auth and tenant ID setup are the two single-most-asked questions on any SDK. Today, each has 4+ documented or undocumented mechanisms with subtle precedence.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 2.1 | CRITICAL | No `client.WithAuthToken` exists; static-token users have no documented path | `client.go:176-606` (no auth option) |
| 2.2 | CRITICAL | Auth requires three-package import dance (`pkg/access-manager` + `pkg/config` + `client`) | `pkg/access-manager/access-manager.go:232`, `pkg/config/config.go:535`, `client.go:499` |
| 2.3 | HIGH | 4+ paths to set auth (config option, env vars, entity option, post-hoc setter) | `pkg/config/config.go:535`, env (`PLUGIN_AUTH_*`), `entities/options.go:144`, `entities/entity.go:501` |
| 2.4 | HIGH | Tenant-ID has 4 setters; precedence subtle and undocumented | `client.go:599`, `pkg/config/config.go:515`, `entities/options.go:127`, `entities/context.go:47` |
| 2.5 | HIGH | `WithTenantID` × 3 across packages with two semantics (option vs context) | Same as 2.4 |
| 2.6 | MEDIUM | `pkg/access-manager` directory contains `package auth` — IDE auto-import mismatch | `pkg/access-manager/access-manager.go:1` |
| 2.7 | MEDIUM | No `docs/auth.md` — auth is the least-documented surface | `docs/` (absent) |

#### Proposed v3 shape

**Two canonical mechanisms:**

```go
// Static token (most common case)
c, _ := client.New(
    client.WithAuthToken("midaz_pat_xyz"),
    client.WithEnvironment(client.EnvProduction),
)

// OAuth via Access Manager (plugin auth)
c, _ := client.New(
    client.WithAccessManager(client.AccessManager{
        Address:      "https://auth.midaz.io",
        ClientID:     "abc",
        ClientSecret: "xyz",
    }),
    client.WithEnvironment(client.EnvProduction),
)

// Anonymous (test-only, MUST be explicit)
c, _ := client.New(
    client.WithAnonymous(),
    client.WithBaseURL("http://localhost:3000"),
)
```

**Type aliases at the client package level:**

```go
// client/auth.go
type AccessManager = auth.AccessManager // re-exported from pkg/auth
```

**Tenant ID — exactly two setters:**

```go
// Default tenant for all requests on this client
c, _ := client.New(client.WithTenantID("acme-prod"))

// Per-request override (uses context)
ctx := entities.WithRequestTenantID(ctx, "acme-staging")
acc, _ := c.Accounts.Get(ctx, "org", "ledger", "acc-id")
```

**Precedence (documented in code AND `docs/auth.md`):**

```
Per-request context > client option > FromEnvironment > error
```

**Renamings:**

- `pkg/access-manager` → `pkg/auth` (directory matches package name)
- `entities.WithTenantID(ctx, id)` → `entities.WithRequestTenantID(ctx, id)` (disambiguates from `client.WithTenantID(string) Option`)
- `entities.WithDefaultTenantID(id)` → deleted (redundant with `client.WithTenantID`)

**Deletions:**

- `pkg/config.WithTenantID` (replaced by client-level)
- `entities.WithDefaultTenantID`
- `(*HTTPClient).SetTenantID` post-hoc setter
- `entities.WithPluginAuth` (replaced by `client.WithAccessManager`)
- `(*Entity).SetAuthToken` post-hoc setter

**New documentation:**

- `docs/auth.md` — comprehensive auth setup guide with code samples for static token, OAuth via AccessManager, env-based config, and per-request tenant override
- `docs/multi-tenancy.md` — tenant ID precedence + per-request override patterns
- Cross-references in godoc: `client.WithAuthToken` doc references `client.WithAccessManager` and links to `docs/auth.md`

#### Acceptance criteria

- [ ] `client.WithAuthToken("token")` configures auth in a single line — no `pkg/config` import required
- [ ] `client.WithAccessManager(am)` works without touching `pkg/config` or `pkg/auth`
- [ ] `client.AccessManager` is the public type (re-exported from `pkg/auth`)
- [ ] Without an auth source AND without explicit `WithAnonymous()`, `client.New()` returns a typed error
- [ ] Tenant ID precedence implemented + documented + tested (per-request > client > env)
- [ ] `docs/auth.md` and `docs/multi-tenancy.md` exist with runnable examples
- [ ] `pkg/access-manager` directory renamed to `pkg/auth`; package name matches directory; godoc surfaces auth as a discoverable subpackage
- [ ] `entities.WithRequestTenantID` is the only context helper for tenant ID; `entities.WithTenantID` (the old context one) is gone
- [ ] `MIDAZ_TENANT_ID` env var added to `FromEnvironment()` (currently absent)

---

### Track 3 — Implicit Env-Var Reads

**Severity:** CRITICAL · **Effort:** S · **Phase:** A · **Dependencies:** none (independent cleanup)

The audit found **21 implicit env-var reads** across 6 distinct variables, scattered through entity constructors and one validator. Every implicit read violates Design Principle #2 ("explicit over implicit"). The fix is mechanical but high-impact.

#### Findings — implicit reads in entity layer

| File:Line | Env Var | What it does | Severity |
|-----------|---------|--------------|----------|
| `entities/http.go:52` | `MIDAZ_USER_AGENT` | Overrides `WithUserAgent` silently | HIGH |
| `entities/http.go:114` | `MIDAZ_DEBUG` | Toggles debug logging | HIGH |
| `entities/http.go:132` | `MIDAZ_IDEMPOTENCY` | Toggles auto-idempotency (semantics: `!= "false"` defaults ON) | HIGH |
| `entities/http.go:166` | `MIDAZ_MAX_RETRIES` | Sets retry count on entity HTTP client | HIGH |
| `entities/http.go:175` | `MIDAZ_ENABLE_RETRIES` | **Hidden killswitch** — not documented, not in `FromEnvironment` | HIGH |
| 14 × `entities/{accounts,...}.go` | `MIDAZ_DEBUG` | Redundant copy-paste reads in every NewXxxEntity | MEDIUM (redundant) |

#### Findings — implicit read in config layer

| File:Line | Env Var | What it does | Severity |
|-----------|---------|--------------|----------|
| `pkg/config/config.go:870` | `MIDAZ_SKIP_AUTH_CHECK` | Bypasses validation on every `NewConfig()` regardless of `FromEnvironment()` opt-in | MEDIUM |

#### Findings — stdlib proxy reads (KEEP, document)

| File:Line | What it does |
|-----------|--------------|
| `pkg/config/config.go:980` | `http.ProxyFromEnvironment` (transport) |
| `entities/http.go:151` | `http.ProxyFromEnvironment` (transport) |
| `pkg/performance/http.go:327` | `http.ProxyFromEnvironment` (transport) |

These are stdlib Go conventions (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`). Keep them; document in `docs/environment.md`.

#### Spookiness map (current state — kill all of this)

| Env Var | Implicit effect |
|---------|----------------|
| `MIDAZ_DEBUG=true` | Every entity flips debug mode regardless of `WithDebug(false)` in code |
| `MIDAZ_USER_AGENT=foo/1.0` | UA header overridden even when `WithUserAgent(...)` was set in code |
| `MIDAZ_IDEMPOTENCY=false` | Auto-idempotency disabled silently |
| `MIDAZ_MAX_RETRIES=N` | Retry count overridden |
| `MIDAZ_ENABLE_RETRIES=false` | Retries disabled silently — hidden killswitch |
| `MIDAZ_SKIP_AUTH_CHECK=true` | Auth validation bypassed silently |

#### Proposed v3 shape

**Single source of truth: `config.FromEnvironment()`.** Every `os.Getenv` call moves into that function or is deleted. Entity-layer code accepts only explicit configuration via constructor args or option-applied state.

**Refactor `entities.NewHTTPClient` to take an explicit struct:**

```go
// entities/http.go
type HTTPClientConfig struct {
    HTTPClient        *http.Client
    AuthToken         string
    Provider          observability.Provider
    UserAgent         string
    Debug             bool
    EnableIdempotency bool
    Retries           retry.HTTPOptions
    Logger            *slog.Logger
}

func NewHTTPClient(cfg HTTPClientConfig) *HTTPClient {
    // No env reads. Pure value-in / value-out.
}
```

**Delete the 14 redundant entity-constructor env reads.** They predate the centralized propagation in `Entity.initServices`. Replace each `if debugEnv := os.Getenv(EnvMidazDebug); debugEnv == BoolTrue { ... }` block with nothing — the parent `Entity` already passes debug through.

**Replace `MIDAZ_SKIP_AUTH_CHECK` with internal `Config.skipAuthCheck` field:**

```go
// pkg/config/config.go (test-only)
func WithSkipAuthCheck() Option { /* sets unexported skipAuthCheck=true */ }
```

The env var becomes test plumbing only, set inside `FromEnvironment()` if it's set in the env. `validateConfig` reads the field, never the env directly.

**`MIDAZ_ENABLE_RETRIES` decision:** delete entirely. It's redundant with `MIDAZ_MAX_RETRIES=0`. Update `docs/environment.md` to document `MIDAZ_MAX_RETRIES=0` as the disable mechanism.

**Consolidate plugin-auth env reads.** Today there are TWO copies of `PLUGIN_AUTH_*` reads (in `FromEnvironment` and in `NewLocalConfig`). Refactor `NewLocalConfig` to call `FromEnvironment` internally so plugin auth has exactly one env-loading code path.

#### Acceptance criteria

- [ ] `rg "os.Getenv" /Users/fredamaral/repos/lerianstudio/midaz-sdk-golang --type go | grep -v _test.go | grep -v examples/ | grep -v ProxyFromEnvironment` returns ≤ 17 matches (the 17 explicit reads in `pkg/config/config.go` inside `FromEnvironment` and `NewLocalConfig`)
- [ ] Setting `MIDAZ_DEBUG=true` in shell has zero effect on `client.New(client.WithDebug(false))` behavior (validated by integration test)
- [ ] `MIDAZ_ENABLE_RETRIES` is removed from the codebase entirely; `docs/environment.md` documents `MIDAZ_MAX_RETRIES=0` as the disable path
- [ ] `MIDAZ_SKIP_AUTH_CHECK` env var still works for tests but only via `FromEnvironment()`-driven config; programmatic `NewConfig` ignores it unless explicitly opted in
- [ ] `docs/environment.md` lists every env var the SDK reads, organized by `FromEnvironment()`-loaded vs stdlib proxy
- [ ] No entity constructor calls `os.Getenv` (verified via grep + test)
- [ ] `entities.NewHTTPClient` accepts an explicit `HTTPClientConfig` struct; all knobs are caller-supplied

---

### Track 4 — Logging Gap

**Severity:** CRITICAL · **Effort:** M · **Phase:** A · **Dependencies:** Tracks 1, 3 (uses new client surface; assumes env reads cleaned up)

The most confused subsystem. Two parallel logging systems coexist, neither is `slog`-based, no logger injection exists, and retry logging — the most-needed feature — is literally a `// TODO` in production code.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 4.1 | CRITICAL | No `WithLogger(*slog.Logger)` option; users can't integrate slog/zap/zerolog | `client.go` (absent) |
| 4.2 | HIGH | `MIDAZ_DEBUG` bypasses the Logger interface entirely (writes raw text to stderr) | `entities/http.go:1739-1772` |
| 4.3 | HIGH | 3 production sites write to `os.Stderr` unconditionally — users cannot redirect/silence | `entities/query.go:27`, `models/common.go:929`, `pkg/performance/client.go:67` |
| 4.4 | HIGH | Zero retry-attempt logging — code admits it: `// Parameter reserved for future retry attempt logging` | `pkg/retry/http.go:435` |
| 4.5 | MEDIUM | `MetricsCollector.RecordRetry` is dead code outside tests | `pkg/observability/metrics.go:170-171` |
| 4.6 | MEDIUM | No slow-call threshold warning; `elapsed` captured but never compared | `entities/http.go:589, 655, 755` |
| 4.7 | MEDIUM | `Fatal/Fatalf` in library logger interface — invites host-process termination | `pkg/observability/logging.go:79-82, 273-280` |
| 4.8 | MEDIUM | `Client.Logger()` returns `nil` when observability disabled — panic-bait | `client.go:670-676` |
| 4.9 | MEDIUM | Documented usage example is broken — `fmt.Sprint`s field map into message | `docs/tracing.md:191-195` |
| 4.10 | LOW | 14 redundant per-entity `MIDAZ_DEBUG` reads (covered in Track 3) | (Track 3) |
| 4.11 | LOW | `sanitizeSensitiveString` only redacts string field values, not deeply-nested struct fields | `pkg/observability/logging.go:181-191` |

#### Proposed v3 shape

**Design philosophy:**
1. `*slog.Logger` is the contract. No bespoke Logger interface.
2. Silent by default (discard handler). Users opt in to volume.
3. One trigger, one path. No `MIDAZ_DEBUG` bypass.
4. Structured everywhere. Required field schema documented.
5. Per-call ctx override for noisy operations.
6. Observability stays separate (OTel) but composable (logger receives `trace_id`/`span_id`).

**Public API:**

```go
// Top-level options
client.New(
    client.WithLogger(slog.New(slog.NewJSONHandler(os.Stderr, nil))),
    client.WithLogLevel(slog.LevelInfo),                     // overrides handler level
    client.WithSlowCallThreshold(2 * time.Second),
    client.WithRedactedHeaders("X-My-Custom-Auth"),          // additive to defaults
    client.WithRedactedQueryParams("custom_token"),          // additive to defaults
    client.WithRequestHook(func(ctx context.Context, req *http.Request) {
        // user-controlled; no built-in body printing
    }),
    client.WithResponseHook(func(ctx context.Context, req *http.Request, resp *http.Response, elapsed time.Duration, err error) {
        // user-controlled
    }),
)

// Per-call overrides (operate on context)
ctx = midaz.WithLogger(ctx, customLogger)         // override for this call only
ctx = midaz.WithLoggerSilenced(ctx)               // hard mute (e.g., polling loops)
ctx = midaz.WithLoggerLevel(ctx, slog.LevelDebug) // bump verbosity for one call

// Defaults:
//   No WithLogger → slog.New(slog.NewTextHandler(io.Discard, nil))
//   MIDAZ_DEBUG=true (via FromEnvironment) AND no WithLogger →
//     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
```

**Required structured fields per log line:**

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `sdk.name` | string | constant `"midaz-go-sdk"` | identifies emitter |
| `sdk.version` | string | `client.Version` | matches `pkg/version` |
| `sdk.component` | string | e.g. `"http"`, `"retry"`, `"validation"` | which subsystem emitted |
| `operation` | string | e.g. `"transactions.Create"` | already tracked in observability spans |
| `http.method` | string | when applicable | |
| `url.path` | string | redacted via `normalizeTelemetryURL` (no IDs) | |
| `http.status_code` | int | when applicable | |
| `duration_ms` | int64 | when applicable | |
| `request_id` | string | from `X-Request-ID` response header | already extracted at `entities/http.go:1373` |
| `trace_id` / `span_id` | string | from `ctx` if traced | replaces today's `WithSpan`/`WithContext` proprietary helpers |
| `tenant_id` | string | redacted unless `WithLogTenantID(true)` opt-in | |
| `error.type` / `error.message` | string | for failures | already redacted via `RedactSensitiveString` |

**Retry log shape (per attempt):**

```json
{
  "time": "2026-05-05T14:22:11Z",
  "level": "DEBUG",
  "msg": "retrying request",
  "sdk.name": "midaz-go-sdk",
  "sdk.version": "3.0.0",
  "sdk.component": "retry",
  "operation": "transactions.Create",
  "http.method": "POST",
  "url.path": "/v1/organizations/:id/ledgers/:id/transactions",
  "attempt": 2,
  "max_attempts": 3,
  "delay_ms": 400,
  "cause": "503 service unavailable",
  "http.status_code": 503,
  "request_id": "req_abc123",
  "trace_id": "0123…",
  "span_id": "abcd…"
}
```

Final attempt before exhaustion logs at `WARN`. Successful retry logs once at `INFO` with `total_attempts`.

**Redaction policy (defaults):**

Always redacted, no opt-out:
- HTTP headers: `Authorization`, `Cookie`, `Set-Cookie`, `Proxy-Authorization`
- Query params matching `*token*`, `*secret*`, `*password*`, `*api[_-]?key*`, `external_id`, `banking_details_*`, `*document*`, `*metadata*`
- String values matching `bearer\s+[\w.\-]+` or `(token|secret|password|api[_-]?key|auth[_-]?token)\s*[=:]\s*\S+`

Default-redacted, opt-out via `WithLogTenantID(true)`:
- `X-Tenant-ID`, `X-Idempotency`, idempotency keys

Bodies: always shown as `[REDACTED len=N]`. Override only via explicit `WithUnsafeBodyLogging()` (off by default; doc warning that this leaks PII).

**User integration examples (added to `docs/logging.md`):**

```go
// stdlib slog with JSON
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
c, _ := client.New(client.WithLogger(logger), client.WithAuthToken("..."))

// charmbracelet/log (already implements slog.Handler)
import "github.com/charmbracelet/log"
clog := log.NewWithOptions(os.Stderr, log.Options{ReportCaller: true, Level: log.DebugLevel})
c, _ := client.New(client.WithLogger(slog.New(clog)), client.WithAuthToken("..."))

// zap via slog adapter (Go 1.22+)
import "go.uber.org/zap/exp/zapslog"
zl, _ := zap.NewProduction()
c, _ := client.New(
    client.WithLogger(slog.New(zapslog.NewHandler(zl.Core(), nil))),
    client.WithAuthToken("..."),
)

// Per-call silence for noisy poll loops
quietCtx := midaz.WithLoggerSilenced(ctx)
for {
    _, _ = c.Balances.Get(quietCtx, orgID, ledgerID, balID)
    time.Sleep(time.Second)
}
```

**Observability + logging composition:**

When both `WithLogger` and `WithObservability` are configured:
- Logger receives auto-injected `trace_id`/`span_id` from active spans
- Observability provider exposes `Logger()` which returns the same `*slog.Logger`
- No double-emission, no separate logger-of-its-own inside the provider

**Deletions:**

- Old `pkg/observability.Logger` interface (replaced by `*slog.Logger`)
- `Fatal/Fatalf` from any logger surface
- `MIDAZ_DEBUG` bypass path in `entities/http.go:1739-1772` (delete `debugLog`; replace with logger calls)
- 3 unconditional stderr writes (`entities/query.go:27`, `models/common.go:929`, `pkg/performance/client.go:67`) — route through logger at Warn

**New code:**

- `client/logger.go` — `WithLogger`, `WithLogLevel`, `WithSlowCallThreshold`, `WithRedactedHeaders`, `WithRedactedQueryParams`, `WithRequestHook`, `WithResponseHook`, `WithoutLogger`, `WithLogTenantID`, `WithUnsafeBodyLogging`, ctx helpers
- `internal/logging/redact.go` — redaction policy, configurable
- `internal/logging/fields.go` — required-field shape and helpers (`fieldsFromContext`, `httpRequestFields`, `retryFields`)
- Wire retry-attempt logging into `pkg/retry/http.go:435` AND call `MetricsCollector.RecordRetry`

#### Acceptance criteria

- [ ] `client.WithLogger(*slog.Logger)` works with stdlib slog out of the box
- [ ] Default `Client.Logger()` returns a non-nil discard logger when nothing is configured
- [ ] `MIDAZ_DEBUG=true` (via `FromEnvironment()`) installs a debug-level stderr slog handler when no logger configured
- [ ] Setting `MIDAZ_DEBUG=true` while `WithLogger(jsonLogger)` is configured does NOT bypass the json logger
- [ ] Every retry attempt emits a structured log line with the documented field schema
- [ ] `MetricsCollector.RecordRetry` is called from production code (not just tests)
- [ ] `WithSlowCallThreshold(2*time.Second)` emits a Warn log when a call exceeds 2s
- [ ] Three previous unconditional stderr writes now route through the logger (or are silenced if no logger)
- [ ] `Fatal/Fatalf` is removed from the public Logger surface
- [ ] `docs/tracing.md:191-195` example is fixed (uses correct slog idiom)
- [ ] `docs/logging.md` exists with integration examples for stdlib slog, charmbracelet, zap, and zerolog
- [ ] `examples/08-logging-slog/main.go` exists and runs

---

### Track 5 — Pagination Footguns

**Severity:** CRITICAL · **Effort:** L · **Phase:** B · **Dependencies:** Tracks 1, 6 (uses new client; coordinates with options consolidation)

The most user-facing track. Five distinct pagination shapes, silent stderr warnings, mutable shared options, dead `pkg/pagination` package, and zero `iter.Seq2` helpers despite Go 1.26 making them idiomatic.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 5.1 | CRITICAL | Five distinct pagination shapes coexist | (see breakdown below) |
| 5.2 | CRITICAL | `pkg/pagination` is 1488 lines of dead code unused by any entity | `pkg/pagination/paginator.go`, `adapters.go` |
| 5.3 | CRITICAL | No `iter.Seq2` iterator helpers (Go 1.26 idiom missing) | All List methods |
| 5.4 | HIGH | Silent default limit (10), silent max-cap (100) — `Limit: 5000` → 100 with no error | `models/constants.go:97`, `models/common.go:495, 909-911` |
| 5.5 | HIGH | `Page/Offset` silently dropped to stderr on cursor endpoints | `entities/query.go:26-30` |
| 5.6 | HIGH | `*ListOptions` mutable; sharing across goroutines causes data races | `models/common.go:601-611`, workaround at `examples/workflow-with-entities/pkg/workflows/account_list.go:211` |
| 5.7 | HIGH | `MaxLimit=100` enforced silently (no error returned) | `models/common.go:909-911` |
| 5.8 | MEDIUM | `Pagination.TotalPages()` returns 1 when total unknown — encourages buggy "Page 3 of 1" UIs | `models/common.go:378-394` |
| 5.9 | MEDIUM | `ListOptions.Validate()` exists but no entity calls it | `models/common.go:880-895` |
| 5.10 | MEDIUM | `AssetRatesResponse` uses bespoke `*string` cursors instead of unified `Pagination` | `models/asset_rate.go:243-255` |
| 5.11 | LOW | `ListMetadataIndexes` returns slice with no pagination metadata | `entities/metadata_indexes.go:46` |
| 5.12 | LOW | Per-endpoint filter explosion on `ListOptions` (20+ `WithX` methods only valid for 1-2 endpoints) | `models/common.go:686-773` |

**The five pagination shapes:**

1. Page-based via `*ListOptions` → `*ListResponse[T]` (12 methods)
2. Cursor-only via `*ListOptions` → `*ListResponse[T]` with silent stderr warning on `Page` misuse (5 methods)
3. `*AssetRateListOptions` → `*AssetRatesResponse` with `*string` cursors (1 method)
4. Plain slice return, no pagination (`ListMetadataIndexes`)
5. Standalone `pkg/pagination.Paginator[T]` (zero entity wiring)

#### Proposed v3 shape

**Per-service typed `ListOpts`:**

Each service exposes its own typed options struct with **only the fields valid for that endpoint kind**. This prevents `WithPage` from compiling on a cursor-only endpoint.

```go
// services/accounts/options.go (page-based endpoint)
package accounts

type ListOpts struct {
    Limit          int                  // 0 = server default; > MaxLimit returns error
    Page           int                  // 1-indexed; 0 → start at page 1
    SortDirection  SortDirection
    Filters        AccountFilters       // typed, per-service
}

type AccountFilters struct {
    AssetCode     string
    Type          string
    HolderID      string
    PortfolioID   string
    SegmentID     string
    Status        string
    IncludeDeleted bool
    CreatedAfter  time.Time
    CreatedBefore time.Time
    UpdatedAfter  time.Time
    UpdatedBefore time.Time
}

func (o ListOpts) Validate() error { /* limit cap, page bounds, etc. */ }
```

```go
// services/transactions/options.go (cursor-only endpoint)
package transactions

type ListOpts struct {
    Limit          int             // 0 = server default; > MaxLimit returns error
    Cursor         string          // server-issued cursor; empty for first page
    SortDirection  SortDirection
    Filters        TransactionFilters
}

// Note: NO Page or Offset fields. Compile-time prevention of footgun 5.5.
```

**Service interface:**

```go
type AccountsService interface {
    // List returns one page. Returns ErrInvalidOptions if opts is malformed.
    List(ctx context.Context, orgID, ledgerID string, opts AccountsListOpts) (*ListResponse[Account], error)

    // ListAll yields every account, handling pagination automatically.
    // Stops on first error (caller decides whether to continue).
    //
    //   for account, err := range svc.ListAll(ctx, orgID, ledgerID, opts) {
    //       if err != nil { return err }
    //       process(account)
    //   }
    ListAll(ctx context.Context, orgID, ledgerID string, opts AccountsListOpts) iter.Seq2[Account, error]

    // ListPages is for callers needing page-level metadata (progress UIs, etc.).
    ListPages(ctx context.Context, orgID, ledgerID string, opts AccountsListOpts) iter.Seq2[*ListResponse[Account], error]
}
```

**Unified `ListResponse[T]`:**

```go
type ListResponse[T any] struct {
    Items      []T
    Pagination Pagination
}

type Pagination struct {
    Limit       int
    Total       int     // 0 if unknown — use TotalKnown() to check
    NextCursor  string  // empty if no more pages
    PrevCursor  string  // empty if no previous page
    Page        int     // 0 if cursor-based
    HasMore     bool
}

func (p Pagination) TotalKnown() bool { return p.Total > 0 }
func (p Pagination) HasNext() bool    { return p.HasMore }
```

`AssetRatesResponse` is deleted; asset-rates returns `*ListResponse[AssetRate]` like everyone else.

**Convenience helpers:**

```go
// Collect first N items into a slice (cap-aware; errors short-circuit)
func Collect[T any](seq iter.Seq2[T, error], maxItems int) ([]T, error) { /* ... */ }

// Drain all items into a slice (callers should know the total is bounded)
func CollectAll[T any](seq iter.Seq2[T, error]) ([]T, error) { /* ... */ }
```

**MaxLimit enforcement:**

```go
const MaxLimit = 100

func (o ListOpts) Validate() error {
    if o.Limit > MaxLimit {
        return errors.NewValidationError("ListOpts.Validate", "limit too large",
            fmt.Errorf("limit %d exceeds max %d", o.Limit, MaxLimit))
    }
    return nil
}
```

Entity methods invoke `opts.Validate()` automatically. No more silent capping.

**`pkg/pagination` decision: DELETE.** It's 1488 lines of unused generic abstraction. The new typed-per-service approach replaces it. If users need a generic paginator, they can build one on top of `iter.Seq2` in user-space (it's now stdlib idiom).

**Deletions:**

- `pkg/pagination` package entirely (1488 lines)
- `models.AssetRateListOptions` (folded into `services/assetrates.ListOpts`)
- `models.AssetRatesResponse` (replaced by `ListResponse[AssetRate]`)
- `models.ListOptions` mega-struct (replaced by per-service types)
- `models.Accounts`, `models.ListAccountInput`, `models.ListAccountResponse`, `models.AccountFilter` (dead types from earlier API)
- `Pagination.TotalPages()` (replaced by `TotalKnown()` + caller arithmetic)
- All 20+ `ListOptions.WithBankingDetailsIBAN`, `.WithRelatedPartyRole` etc. (folded into per-service typed Filters)

**Cursor-pagination example added at `examples/04-listing-cursor/main.go`:**

```go
// Demonstrates iterating cursor-paginated transactions
opts := transactions.ListOpts{
    Limit:         50,
    Filters: transactions.Filters{
        DateAfter:  time.Now().AddDate(0, -1, 0),
        DateBefore: time.Now(),
    },
}

count := 0
for tx, err := range c.Transactions.ListAll(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("transactions list failed at item %d: %w", count, err)
    }
    fmt.Printf("[%d] %s %s\n", count, tx.ID, tx.Status)
    count++
    if count >= 1000 { break } // user controls termination
}
```

#### Acceptance criteria

- [ ] Every list method exposes `List(ctx, ..., opts) (*ListResponse[T], error)`, `ListAll(ctx, ..., opts) iter.Seq2[T, error]`, `ListPages(ctx, ..., opts) iter.Seq2[*ListResponse[T], error]`
- [ ] Each service has its own typed `ListOpts` with only valid fields for that endpoint kind (compile-time prevention of `WithPage` on cursor endpoints)
- [ ] `MaxLimit` is enforced via `opts.Validate()` returning a typed error — no silent capping
- [ ] `pkg/pagination` package is deleted
- [ ] `models.AssetRateListOptions` and `models.AssetRatesResponse` deleted; asset rates use `ListResponse[AssetRate]`
- [ ] No silent stderr writes anywhere in the listing path; misuse is a typed error
- [ ] `*ListOpts` is a value type (not a pointer); shared safely across goroutines without `Clone()`
- [ ] `Pagination.TotalKnown()` exists; `TotalPages()` is removed
- [ ] `ListMetadataIndexes` returns `*ListResponse[MetadataIndex]` (consistent with other list methods)
- [ ] `examples/04-listing-cursor/main.go` exists and demonstrates `iter.Seq2` iteration over transactions
- [ ] Godoc Example function for `c.Accounts.ListAll` and `c.Transactions.ListAll`

---

### Track 6 — Functional Options Sprawl

**Severity:** HIGH · **Effort:** M · **Phase:** B · **Dependencies:** Tracks 1, 2 (uses new client surface; auth options consolidated)

29 distinct findings; the recurring patterns are: same name across packages with different meanings, no-op zombies polluting autocomplete, multiple "off" toggles, hidden features behind deep package paths, and order-dependent options without documentation.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 6.1 | CRITICAL | `WithRetries` has two contradictory signatures: `(int, dur, dur)` vs `(bool)` | `client.go:219` vs `pkg/config/config.go:430` |
| 6.2 | CRITICAL | `WithCollectorEndpoint` silently destroys prior observability config | `client.go:395-428` |
| 6.3 | HIGH | `WithEnvironment` × 3 with three meanings | `client.go:439`, `pkg/config/config.go:156`, `pkg/observability/observability.go:188` |
| 6.4 | HIGH | Three names for "turn retries off": `DisableRetries()`, `WithRetries(false)`, `WithNoRetry()`, `WithHTTPNoRetry()` | `client.go:266`, `pkg/config/config.go:430`, `pkg/retry/retry.go:353`, `pkg/retry/http.go:298` |
| 6.5 | HIGH | 4 overlapping observability options at client level — order-sensitive, undocumented | `client.go:280, 318, 357, 395` |
| 6.6 | HIGH | `WithTenantID` × 3 (covered in Track 2) | (Track 2) |
| 6.7 | HIGH | `WithObservability` × 2 with incompatible signatures | `client.go:318` vs `entities/options.go:43` |
| 6.8 | HIGH | Hidden env-var coupling (covered in Track 3) | (Track 3) |
| 6.9 | HIGH | Order-dependent URL configuration is undocumented | `pkg/config/config.go:296-308, 163-167, 842-854` |
| 6.10 | HIGH | UA env var silently overrides explicit `WithUserAgent` | `entities/http.go:49-57` |
| 6.11 | HIGH | Free-function model "options" duplicate fluent builder methods | `models/operation-route.go:158, 305+`, `models/account-type.go:193+, 204+`, `models/transaction-route.go:140-213` |
| 6.12 | MEDIUM | `WithObservabilityProvider(nil)` silently no-ops; typed nil errors | `client.go:359-365`, `pkg/config/config.go:475-477` |
| 6.13 | MEDIUM | `WithUserAgent` validation inconsistent across layers | `pkg/config/config.go:378`, `client.go:204`, `entities/options.go:30` |
| 6.14 | MEDIUM | `WithRetries(int, dur, dur)` non-atomic; partial mutation on validation failure | `client.go:219-236` |
| 6.15 | MEDIUM | `entities.WithContext` is a no-op (covered in Track 1) | (Track 1) |
| 6.16 | MEDIUM | `WithRetry*` family naming inconsistency (`WithRetries` × 2, `WithRetryConfig`, `WithMaxRetries`, `WithRetryWaitMin/Max`, `WithRetryableErrors`, `WithRetryCount`, `WithBatchRetryCount`) | Multiple |
| 6.17 | MEDIUM | `WithMaxIdleConnsPerHost` collision inside `pkg/performance` | `pkg/performance/performance.go:110`, `pkg/performance/http.go:116` |
| 6.18 | MEDIUM | Duplicate `WithBatchTimeout` / `WithMaxBatchSize` across `pkg/performance` and `pkg/concurrent` | `pkg/performance/batch.go:84,97`, `pkg/concurrent/http_batch.go:106,125` |
| 6.19 | MEDIUM | `WithCustomRetryPolicy` accepts `nil` silently | `client.go:246-258` |
| 6.20 | MEDIUM | `WithObservabilityOptions` constructs fresh provider but doesn't honor `WithEnvironment` | `client.go:280-306` |
| 6.21 | MEDIUM | `WithHTTPClient` × 4 with subtly different semantics | `client.go:531`, `pkg/config/config.go:322`, `entities/options.go:93`, `pkg/performance/batch.go:219` |
| 6.22 | MEDIUM | Tier-2 retry policy buried (`pkg/retry.WithJitterFactor`, `WithErrorPredicate`, `WithHighReliability` not surfacable from client) | `pkg/retry/retry.go:166`; `client.go:219` only exposes 3-arg form |
| 6.23 | MEDIUM | `pkg/observability/http.go` middleware options (Ignore/Mask/Hide) completely undiscoverable from client | `pkg/observability/http.go:75-168` |
| 6.24 | MEDIUM | `*tenantIDSet` flags leak across precedence layers without documentation | `client.go:47, 122-130`, `pkg/config/config.go:135` |
| 6.25 | LOW | `WithEnable*` / `Enable*` / `*Enabled` parameter naming drift | Multiple `WithObservability(t,m,l)`, `WithComponentEnabled`, `WithIdempotency(enable)`, `WithDebug(enable)`, etc. |
| 6.26 | LOW | `WithJSONIterator` is a documented no-op | `pkg/performance/performance.go:127` |
| 6.27 | LOW | `WithWaitGroup` is a documented no-op (parameter is `_`) | `pkg/concurrent/concurrent.go:669` |
| 6.28 | LOW | Pagination overlapping default-limit options across two struct types | `pkg/pagination/paginator.go:288`, `pkg/pagination/adapters.go:32` |

#### Proposed v3 shape

**Tier 1 — Top-level on `client.New(...)` (basic, every user needs them):**

```go
// Authentication (Track 2)
client.WithAuthToken(string)
client.WithAccessManager(AccessManager)
client.WithAnonymous()

// Environment & connectivity
client.WithEnvironment(Environment)
client.WithBaseURL(string)
client.WithOnboardingURL(string)
client.WithTransactionURL(string)
client.WithCRMURL(string)
client.WithHTTPClient(*http.Client)
client.WithTimeout(time.Duration)
client.WithUserAgent(string)

// Tenant
client.WithTenantID(string)

// Behavior
client.WithIdempotency(bool)
client.WithDebug(bool)
client.WithContext(context.Context)

// Env loader (still opt-in)
client.FromEnvironment()
```

**Tier 2 — Advanced (variadic delegation; importable subsystem packages):**

```go
client.WithLogger(*slog.Logger)                          // Track 4
client.WithLogLevel(slog.Level)                          // Track 4
client.WithSlowCallThreshold(time.Duration)              // Track 4
client.WithRedactedHeaders(...string)                    // Track 4
client.WithRequestHook(RequestHook)                      // Track 4
client.WithResponseHook(ResponseHook)                    // Track 4

client.WithRetryOptions(...retry.Option)                 // delegates to pkg/retry
client.WithObservability(...observability.Option)        // delegates to pkg/observability
client.WithObservabilityProvider(observability.Provider) // BYO escape hatch
client.WithObservabilityHTTPOptions(...observability.HTTPOption) // promote pkg/observability/http.go

// "Off" toggles (clear, single name per concept)
client.WithoutRetries()
client.WithoutTenantID()
client.WithoutObservability()
client.WithoutLogger()
```

**Convention enforcement:**

- All boolean params named `enabled` (never `enable`/`enableX`)
- All `On/Off` use either `WithX(bool)` OR a dedicated `WithoutX()` — never both naming styles for the same concept
- All options return `Option` (never bare functions); all return `error` from validation if any
- No nil silently accepted — typed-nil and untyped-nil both return error

**Naming canonicalization:**

| Concept | v2 today | v3 |
|---------|----------|-----|
| Disable retries | `DisableRetries()`, `WithRetries(false)`, `WithNoRetry()`, `WithHTTPNoRetry()` | `client.WithoutRetries()` |
| Configure retries | `client.WithRetries(int, dur, dur)`, `config.WithRetryConfig`, `config.WithMaxRetries` + `config.WithRetryWaitMin/Max` | `client.WithRetryOptions(...retry.Option)` |
| Disable obs | `WithObservability(false, false, false)` | `client.WithoutObservability()` |
| Configure obs | `WithObservabilityOptions`, `WithObservability(t,m,l)`, `WithCollectorEndpoint`, `WithObservabilityProvider` | `client.WithObservability(...observability.Option)` + `client.WithObservabilityProvider(p)` (BYO only) |
| Tenant ID default | `client.WithTenantID`, `config.WithTenantID`, `entities.WithDefaultTenantID` | `client.WithTenantID(string)` |
| Tenant ID per-request | `entities.WithTenantID(ctx, id)` | `entities.WithRequestTenantID(ctx, id)` |
| HTTP client | `client.WithHTTPClient`, `config.WithHTTPClient`, `entities.WithHTTPClient`, `pkg/performance.WithHTTPClient` | `client.WithHTTPClient(*http.Client)` only at client level |

**Promote (currently hidden):**

- `pkg/observability/http.go` Ignore/Mask/Hide options → `client.WithObservabilityHTTPOptions(...)`
- `pkg/retry.WithJitterFactor`, `WithErrorPredicate`, `WithHighReliability` → reachable via `client.WithRetryOptions(retry.WithHighReliability(), ...)` with documented examples
- `pkg/observability.WithDevelopmentDefaults`/`WithProductionDefaults` → re-export as `client.WithProductionObservability()` / `client.WithDevelopmentObservability()` for one-line setups
- `pkg/performance.WithHighThroughput` / `WithLowLatency` → expose as `client.WithHighThroughput()` / `client.WithLowLatency()` presets
- `entities.WithIdempotencyKey(ctx, key)` and `entities.WithoutAutoIdempotency(ctx)` → document prominently in idempotency guide

**Deletions:**

- `client.DisableRetries`, `client.WithRetries(int, dur, dur)`, `client.WithRetries(bool)` (config), `pkg/retry.WithNoRetry`, `pkg/retry.WithHTTPNoRetry`
- `client.WithObservability(bool, bool, bool)`, `client.WithCollectorEndpoint`
- `client.UseAllAPIs`, `client.UseEntityAPI`, `client.UseEntity` (Track 1)
- `entities.WithContext`, `entities.WithObservability`, `entities.WithPluginAuth` (renamed/moved per Track 2)
- `config.WithRetries(bool)`, `config.WithRetryConfig`, `config.WithTenantID`
- `pkg/performance.WithJSONIterator`, `pkg/concurrent.WithWaitGroup` (no-ops)
- `pkg/performance.WithMaxIdleConnsPerHost` (Option variant; keep TransportOption variant)
- All free-function model "options" in `models/account-type.go:193-340`, `models/operation-route.go:305-470`, `models/transaction-route.go:140-213` (replaced by methods)
- `pkg/performance/batch.go` and `pkg/concurrent/http_batch.go` overlap → consolidate into `pkg/batch`

#### Acceptance criteria

- [ ] `rg "^func With\w+|^func Without\w+|^func Disable\w+|^func Enable\w+" --type go` returns ≤ 50% of today's count (current ~120 → target ≤ 60)
- [ ] No two packages export options with the same name and different meanings (lint check)
- [ ] All `With*` options return `Option` and `error` (lint check)
- [ ] No `// Deprecated:` markers on the canonical option surface (deprecated shims live in a separate file `client/deprecated.go`)
- [ ] All `Without*` "off" toggles exist for: retries, observability, tenant ID, logger, idempotency
- [ ] `client.WithRetryOptions(...retry.Option)` accepts the full `pkg/retry.Option` surface
- [ ] `client.WithObservability(...observability.Option)` accepts the full `pkg/observability.Option` surface
- [ ] Documented precedence rules in `docs/configuration.md`: option order, env vs option, per-request vs default
- [ ] Boolean params named `enabled` consistently; no `enable`/`enableX` left in the public surface

---

### Track 7 — Builder/Model API Drift

**Severity:** HIGH · **Effort:** L · **Phase:** B · **Dependencies:** Track 6 (uses canonical option surface)

The Account / Ledger / Asset / Portfolio / Segment / AccountType / OperationRoute / TransactionRoute / Holder / Alias / AssetRate / Balance / Operation / MetadataIndex / Transaction families do not share a shape. Three different `CreateInput` patterns, two builder calling conventions, inconsistent required-fields, type-unsafe Updates, validate inconsistently invoked.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 7.1 | HIGH | Three different shapes for `CreateXInput` | (see below) |
| 7.2 | HIGH | `New*Input` constructors take inconsistent required-field sets | All entities |
| 7.3 | MEDIUM | Two builder calling conventions (methods + package-level functions) for AccountType, OperationRoute, TransactionRoute | `models/account-type.go:193-340`, `models/operation-route.go:305-470`, `models/transaction-route.go:140-213` |
| 7.4 | HIGH | Pagination split across 5 dialects (covered in Track 5) | (Track 5) |
| 7.5 | HIGH | `UpdateTransaction` and `UpdateTransactionOperation` accept `input any` — no compile-time guarantee | `entities/transactions.go:66`, `entities/operations.go:170, 175` |
| 7.6 | MEDIUM | Update method validation is inconsistent — sometimes called, sometimes not | `entities/portfolios.go:292`, `entities/segments.go:291`, vs `entities/ledgers.go:387-420` (LedgerSettings — no Validate) |
| 7.7 | LOW | `NewSegmentsEntity` does not honor `MIDAZ_DEBUG` (covered in Track 3) | `entities/segments.go:135-140` |
| 7.8 | LOW | 16 entity constructors duplicate identical body | All `entities/*.go:NewXxxEntity` |
| 7.9 | MEDIUM | `ListMetadataIndexes` parameter shape breaks cross-entity convention | `entities/metadata_indexes.go:15` |
| 7.10 | MEDIUM | Soft-delete / hard-delete / include-deleted handled three different ways | `entities/holders.go:20, 24`, `entities/aliases.go:20, 24`, vs `models/common.go:666` |
| 7.11 | LOW | `Status` is set in some `Create*Input` constructors and not others | `models/account.go:213` (sets) vs all others |
| 7.12 | LOW | Holder's `Type` field is `*string` yet ctor takes `string` | `models/holder.go:27, 56` |
| 7.13 | MEDIUM | Operation lacks typed Create/Update builder pair like every other entity | `models/operation.go:199, 398` |
| 7.14 | LOW | `UpdateOrganizationInput` exposes redundant aliased setters | `models/organization.go:282-310` |
| 7.15 | COSMETIC | Filename hyphens vs underscores | `models/account-type.go`, `operation-route.go`, `transaction-route.go` |
| 7.16 | MEDIUM | Transaction "Convenience" Create methods obscure the canonical Create path | `entities/transactions.go:32, 38, 44, 93, 99, 105` |
| 7.17 | LOW | AssetRate has no UpdateInput, just an "upsert" Create | `entities/asset_rates.go:43` |
| 7.18 | LOW | `Accounts`, `ListAccountInput`, `ListAccountResponse`, `AccountFilter` are dead/parallel types | `models/account.go:663, 686, 692, 733` |
| 7.19 | LOW | Empty-update-payload error split — sometimes from Validate, sometimes from MarshalJSON | `models/holder.go:307`, `models/alias.go:230` vs `models/account.go:426` |
| 7.20 | LOW | Get/Update parameter-order drift; method-name collisions | `entities/accounts.go:132, 144, 149`, `entities/balances.go:199`, `entities/operations.go:162, 345` |
| 7.21 | LOW | Dead/stub method `GetAccountTypesMetricsCount` shipped as deprecated-on-arrival | `entities/account_types.go:102, 365` |

**The three CreateInput shapes:**

1. **SDK-native struct, hand-written** (Account, AssetRate, Balance, Holder, Alias, MetadataIndex)
2. **`mmodel`-embedding wrapper** (Organization, Ledger, Asset, Portfolio, Segment, AccountType, OperationRoute, TransactionRoute) — leaks internal `mmodel` type into public API
3. **Bespoke parallel structs** (Transaction has 4: `CreateTransactionInput`, `CreateInflowInput`, `CreateOutflowInput`, `CreateAnnotationInput`)

#### Proposed v3 shape

**Account is the gold standard.** Converge every other entity to this pattern:

```go
// services/accounts/types.go
package accounts

// Account is the SDK-native type. ToWire() produces the wire form for transport.
type Account struct {
    ID         string
    Name       string
    AssetCode  string
    Type       string
    Status     Status
    Metadata   map[string]any
    // ... etc
}

// CreateInput is the first-class input type. Fields are discoverable from this file alone.
// No embedded mmodel types.
type CreateInput struct {
    // Required (must match what Validate() considers required)
    Name      string
    AssetCode string
    Type      string

    // Optional, fluent-set
    Status   Status
    Metadata map[string]any
    Alias    *string
    // ... etc
}

// NewCreateInput takes EXACTLY the required fields. Everything else via With*.
func NewCreateInput(name, assetCode, accountType string) *CreateInput {
    return &CreateInput{Name: name, AssetCode: assetCode, Type: accountType}
}

func (i *CreateInput) WithStatus(s Status) *CreateInput          { i.Status = s; return i }
func (i *CreateInput) WithMetadata(m map[string]any) *CreateInput { i.Metadata = m; return i }
func (i *CreateInput) WithAlias(a string) *CreateInput            { i.Alias = &a; return i }

func (i *CreateInput) Validate() error {
    var errs validation.FieldErrors
    if i.Name == "" { errs.Append("name", "is required") }
    if i.AssetCode == "" { errs.Append("assetCode", "is required") }
    if i.Type == "" { errs.Append("type", "is required") }
    if len(i.Metadata) > MaxMetadataSize { errs.Append("metadata", "exceeds max size") }
    return errs.OrNil()
}

// MarshalJSON omits unset optional fields so server defaults apply.
func (i *CreateInput) MarshalJSON() ([]byte, error) { /* selective emission */ }

// Symmetric UpdateInput
type UpdateInput struct { /* all fields optional */ }
func NewUpdateInput() *UpdateInput { return &UpdateInput{} }
func (i *UpdateInput) WithName(name string) *UpdateInput { i.Name = &name; return i }
func (i *UpdateInput) Validate() error {
    if !i.hasChanges() { return errors.NewValidationError("UpdateInput.Validate", "empty update payload not allowed", nil) }
    return nil
}
func (i *UpdateInput) MarshalJSON() ([]byte, error) { /* emit only set fields */ }
```

**Service interface template:**

```go
type Service interface {
    List(ctx context.Context, parents ..., opts ListOpts) (*ListResponse[Account], error)
    ListAll(ctx context.Context, parents ..., opts ListOpts) iter.Seq2[Account, error]
    Get(ctx context.Context, parents ..., id string) (*Account, error)
    Create(ctx context.Context, parents ..., input *CreateInput) (*Account, error)
    Update(ctx context.Context, parents ..., id string, input *UpdateInput) (*Account, error)
    Delete(ctx context.Context, parents ..., id string) error
}
```

Where `parents...` is always `(orgID)`, `(orgID, ledgerID)`, etc., **always in hierarchy order, always before the resource id**.

**Entity implementation template:**

```go
func (s *accountsService) Create(ctx context.Context, orgID, ledgerID string, input *CreateInput) (*Account, error) {
    const operation = "accounts.Create"
    if orgID == "" { return nil, errors.NewMissingParameterError(operation, "organizationID") }
    if ledgerID == "" { return nil, errors.NewMissingParameterError(operation, "ledgerID") }
    if input == nil { return nil, errors.NewMissingParameterError(operation, "input") }
    if err := input.Validate(); err != nil {
        return nil, errors.NewValidationError(operation, "account validation failed", err)
    }
    return s.transport.Post(ctx, transport.Request{
        Operation: operation,
        Resource:  "account",
        Path:      fmt.Sprintf("/v1/organizations/%s/ledgers/%s/accounts", orgID, ledgerID),
        Body:      input,
    }, &Account{})
}
```

**Mandatory cross-entity guarantees (lint targets for v3):**

1. Every `CreateInput` and `UpdateInput` has `Validate() error`
2. Every entity Create/Update method calls `input.Validate()` after the nil check, before serialization
3. Every `UpdateInput.Validate()` returns `"empty update payload not allowed"` when no field is set; never from `MarshalJSON`
4. Every list method returns `*ListResponse[T]` (no `[]T`, no bespoke response types)
5. Every list method accepts the per-service typed `ListOpts` value type
6. Every Update method takes `input *<Service>.UpdateInput` (never `any`)
7. No `Status:` defaults baked into `New*Input` constructors. Server controls defaults.
8. Entity hierarchy parameters in order before resource ID. No exceptions.
9. Soft-delete / hard-delete / include-deleted goes through `ctx` helpers, not function-signature booleans.

**Concrete v3 changes:**

- DELETE: `models.AssetRateListOptions`, `models.AssetRatesResponse`, `models.Accounts`, `models.ListAccountInput`, `models.ListAccountResponse`, `models.AccountFilter`, `NewCreateAssetInput` (deprecated), `GetAccountTypesMetricsCount`, all `WithCreate<Entity>*`/`WithUpdate<Entity>*` package-level funcs, `transactionID ...string` variadic on `GetOperation`, Organization's `*Update`-suffix builder aliases, `input any` UpdateTransaction signatures
- ADD: `NewCreateOperationInput`/`NewUpdateOperationInput`/`With*` for Operation, `Validate()` on `UpdateLedgerSettingsInput`, soft-delete context helpers (`entities.WithIncludeDeleted`, `entities.WithHardDelete`)
- CHANGE: `MetadataIndexesService.ListMetadataIndexes` to take `*ListOpts` and return `*ListResponse[MetadataIndex]`
- CONVERGE: All wrapper-type Create/Update inputs (Organization/Ledger/Asset/Portfolio/Segment/AccountType/OperationRoute/TransactionRoute) become hand-written SDK structs with `ToWire()` adapter
- RENAME: `models/account-type.go` → `models/account_type.go`; same for route files; same for any other hyphenated filenames
- CONSOLIDATE: 16 entity constructors share a single `newServiceEntity[T]` helper

**Transaction Create methods decision:**

Six current variants: `CreateTransaction`, `CreateTransactionWithDSL`, `CreateTransactionWithDSLFile`, `CreateInflowTransaction`, `CreateOutflowTransaction`, `CreateAnnotationTransaction`.

v3: Keep all 6 as separate methods (they hit different endpoints). Document explicitly at the interface top:

```go
// TransactionsService creates and queries transactions.
//
// Five distinct creation paths exist, each backed by a different API endpoint:
//
//   - Create         — standard JSON transaction (most common)
//   - CreateFromDSL  — DSL-defined transaction
//   - CreateInflow   — money flowing into the ledger
//   - CreateOutflow  — money flowing out of the ledger
//   - CreateAnnotation — non-financial annotation
//
// Use Create unless you have a specific reason to use one of the others.
type TransactionsService interface { /* ... */ }
```

#### Acceptance criteria

- [ ] All entity Create/Update inputs follow the SDK-native struct + fluent builder pattern (no `mmodel.X` embedding visible in public types)
- [ ] Every `CreateInput` and `UpdateInput` has `Validate() error`
- [ ] Every entity Create/Update method calls `input.Validate()` automatically
- [ ] `LedgerSettings` has `Validate()` (currently missing)
- [ ] Operation has full `NewCreateInput`/`NewUpdateInput`/`With*` builder pair
- [ ] No `input any` parameters in any entity Update method (compile-time check)
- [ ] All `Update*Input.MarshalJSON` is single-source (no duplicate empty-payload checks)
- [ ] `pkg/auth` (renamed from `pkg/access-manager`) and all hyphenated files renamed to underscore
- [ ] 16 entity constructors collapse to a single `newServiceEntity[T]` helper
- [ ] Lint rule (in CI) enforces: every `*.Service` interface has `List`, `ListAll`, `Get`, `Create`, `Update`, `Delete` (where applicable)
- [ ] `ListMetadataIndexes` takes `*ListOpts` and returns `*ListResponse[MetadataIndex]`
- [ ] All deletes accept context helpers `entities.WithHardDelete(ctx, true)`, `entities.WithIncludeDeleted(ctx, true)` instead of function-signature booleans
- [ ] All soft-delete / include-deleted Get methods consume the same context helper
- [ ] Dead types removed: `models.Accounts`, `models.ListAccountInput`, `models.ListAccountResponse`, `models.AccountFilter`, `GetAccountTypesMetricsCount`

---

### Track 8 — Error System Actionability

**Severity:** HIGH · **Effort:** M · **Phase:** B · **Dependencies:** Tracks 1, 4 (uses new client; logger integration for error correlation)

The taxonomy is good; the production of well-formed errors is the gap. Network failures bypass the typed system entirely. HTTP errors drop SDK call-site context. Internal retry strings leak. Validation collapses multi-field problems. Two parallel naming schemes for the same predicates. Legacy `MidazError` still walks the codebase.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 8.1 | CRITICAL | Network failures bypass typed error system entirely; `IsNetworkError(err)` returns false on real network errors | `entities/http.go:1289-1304`, `pkg/retry/retry.go:500, 531, 537`, `pkg/access-manager/access-manager.go:259, 396, 422, 430` |
| 8.2 | HIGH | HTTP-derived errors drop `Operation` and `ResourceID` context | `pkg/errors/errors.go:1353-1380`, `entities/http.go:1830-1840` |
| 8.3 | HIGH | Internal retry-wrapper string `"custom retryable: "` leaks to users | `entities/http.go:1349-1351`, `pkg/retry/retry.go:537` |
| 8.4 | HIGH | Model `Validate()` returns first error only — multi-field problems collapse | All `models/*.go` Validate methods |
| 8.5 | MEDIUM | Two parallel naming conventions: `Check*Error` and `Is*Error` are identical | `pkg/errors/errors.go:813-1128` |
| 8.6 | MEDIUM | Legacy `MidazError` type still walks the codebase | `pkg/errors/errors.go:714-801`, `pkg/errors/details.go:117-119`, `entities/observability.go:308` |
| 8.7 | MEDIUM | `models.ErrorResponse` and inline parser struct have diverged | `models/common.go:1146-1167` vs `entities/http.go:1782-1791` |
| 8.8 | MEDIUM | HTTP-derived errors don't wrap an underlying cause (`errors.Unwrap` returns nil) | `pkg/errors/errors.go:1361-1380` |
| 8.9 | MEDIUM | `fmt.Errorf("...: %w", err)` rethrows in retry/access-manager — typed predicates can't classify | Multiple in `pkg/retry/`, `pkg/access-manager/`, `entities/http.go` |
| 8.10 | LOW | `redactSensitive` runs on every `Error()` call unconditionally | `pkg/errors/errors.go:172-198, 308-320` |
| 8.11 | LOW | `defer resp.Body.Close()` ignores close error in access-manager | `pkg/access-manager/access-manager.go:398` |
| 8.12 | LOW | `validation.Error` exists in parallel | `pkg/validation/helpers.go:118-126` |
| 8.13 | LOW | `FieldError.Error()` includes literal `'%s'` quoting; `safeValue()` truncates at 128 chars | `pkg/validation/field_error.go:33-89` |
| 8.14 | LOW | Public README has no error system narrative; no `pkg/errors/doc.go` | `pkg/errors/` (no doc.go) |

#### Proposed v3 shape

**Single canonical error type:**

```go
// pkg/errors/errors.go
package errors

type Error struct {
    // Classification — required for every error
    Category Category
    Code     Code

    // Human-readable
    Message string

    // Call-site context — required for every SDK-originated error
    Operation  string  // e.g., "accounts.Create"
    Resource   string  // e.g., "account"
    ResourceID string  // populated when call site has it (Get/Update/Delete)

    // Validation-specific (nil unless Category == Validation)
    Fields []FieldError

    // Server context (nil unless server-originated)
    Server *ServerContext

    // Causal chain
    Err error
}

type ServerContext struct {
    StatusCode int
    RequestID  string
    APICode    string
    Title      string
    EntityType string
    Details    map[string]any
}

type FieldError struct {
    Path        string  // dot-notation: "metadata.user.email"
    Value       any     // redacted-on-render
    Message     string
    Constraint  string  // "required" | "min" | "format" | etc.
    Suggestions []string
}

func (e *Error) Retryable() bool         { /* derived from Category + StatusCode */ }
func (e *Error) Error() string           { /* deterministic, lazily redacted */ }
func (e *Error) Unwrap() error           { return e.Err }
func (e *Error) Is(target error) bool    { /* matches by category+code */ }
```

**Categories (collapsed taxonomy):**

```go
type Category string
const (
    CategoryValidation     Category = "validation"
    CategoryNotFound       Category = "not_found"
    CategoryConflict       Category = "conflict"
    CategoryAuth           Category = "auth"          // collapses authn + authz
    CategoryRateLimit      Category = "rate_limit"
    CategoryTimeout        Category = "timeout"
    CategoryCancelled      Category = "cancelled"
    CategoryNetwork        Category = "network"
    CategoryUnprocessable  Category = "unprocessable" // domain rule violation
    CategoryInternal       Category = "internal"
    CategoryConfiguration  Category = "configuration" // SDK setup error
)
```

**Required fields by error source:**

| Source | Operation | Resource | ResourceID | Fields | Server | Err |
|--------|-----------|----------|------------|--------|--------|-----|
| Client validation (entity) | required | required | when known | required for multi-field | — | wraps inner |
| Client validation (model.Validate) | required | required | — | required (collect all) | — | wraps inner |
| HTTP 4xx response | required | required | required when call site has it | optional (server fields) | required | wraps a leaf |
| HTTP 5xx response | required | optional | — | — | required | wraps a leaf |
| Network failure | required | — | — | — | — | wraps net error |
| Retry exhaustion | required | inherits last attempt | inherits | — | inherits | wraps last |
| Auth refresh failure | required | — | — | — | optional (on 401 from auth service) | wraps inner |

**`errors.Is/As` contract (documented in `pkg/errors/doc.go`):**

```go
// 1. Match by category (broad)
errors.Is(err, errors.ErrNotFound) // matches any not_found, regardless of Code

// 2. Match by code (specific)
errors.Is(err, errors.ErrInsufficientBalance)

// 3. Extract typed error for full context
var sdkErr *errors.Error
if errors.As(err, &sdkErr) {
    log.Printf("op=%s status=%d req=%s api=%s",
        sdkErr.Operation,
        sdkErr.Server.StatusCode,
        sdkErr.Server.RequestID,
        sdkErr.Server.APICode)
}

// 4. Walk validation field errors
if errors.As(err, &sdkErr) && sdkErr.Category == errors.CategoryValidation {
    for _, fe := range sdkErr.Fields {
        log.Printf("field=%s message=%s", fe.Path, fe.Message)
    }
}
```

**Standard wrapping pattern (for SDK contributors, documented in `CONTRIBUTING.md`):**

```go
// At every public service method:
const operation = "accounts.Create"

// 1. Required-param check
if input == nil {
    return nil, errors.MissingParameter(operation, "input")
}

// 2. Model validation — collect ALL field errors, not just first
if err := input.Validate(); err != nil {
    return nil, errors.Validation(operation, "account", "", err) // input.Validate returns FieldErrors
}

// 3. Transport call. Operation + resource + resourceID flow via request context.
acc, err := s.transport.Post(ctx, transport.Request{
    Operation:  operation,
    Resource:   "account",
    ResourceID: "", // create has no ID yet
    Path:       /* ... */,
    Body:       input,
}, &Account{})
if err != nil {
    return nil, err // err is already a fully-populated *errors.Error
}

return &acc, nil
```

**Network error classification:**

```go
// internal/transport/errors.go
func ClassifyTransportError(operation string, err error) error {
    switch {
    case errors.Is(err, context.Canceled):
        return errors.NewCancellation(operation, err)
    case errors.Is(err, context.DeadlineExceeded):
        return errors.NewTimeout(operation, "request deadline exceeded", err)
    case isDNSError(err) || isConnRefused(err) || isTLSError(err):
        return errors.NewNetwork(operation, err)
    default:
        return errors.NewInternal(operation, err)
    }
}
```

**Validation accumulator (replaces first-error-wins):**

```go
// pkg/validation/field_errors.go
type FieldErrors struct { errs []FieldError }

func (e *FieldErrors) Append(field, message string)                 { /* ... */ }
func (e *FieldErrors) AppendWith(field string, opts ...FieldOption) { /* ... */ }
func (e *FieldErrors) OrNil() error                                  { /* nil if empty */ }
func (e *FieldErrors) Error() string                                 { /* multi-line render */ }
func (e *FieldErrors) Is(target error) bool                          { /* matches CategoryValidation */ }
```

Then every model `Validate()` becomes:

```go
func (i *CreateInput) Validate() error {
    var errs validation.FieldErrors
    if i.Name == "" { errs.Append("name", "is required") }
    if i.AssetCode == "" { errs.Append("assetCode", "is required") }
    if i.Type == "" { errs.Append("type", "is required") }
    if !validateAssetCodeFormat(i.AssetCode) {
        errs.AppendWith("assetCode", validation.Constraint("format"),
            validation.Suggest("Use a 3-4 letter uppercase code like USD, EUR, BTC"))
    }
    return errs.OrNil()
}
```

**Deletions:**

- `MidazError`, `NewMidazError`, `ValueOfOriginalType`, `legacyMidazTarget`, all `MidazError` test machinery
- All `Check*Error` aliases (keep `Is*Error`)
- `IsAlreadyExistsError` (alias of `IsConflictError`)
- `IsPermissionError` (alias of `IsAuthorizationError`) — collapse `CategoryAuth`
- `validation.Error` (`pkg/validation/helpers.go:119`)
- `FormatTransactionError` (alias of `FormatUnifiedTransactionError`)
- The `"custom retryable: "` prefix on `retryableCustomPolicyError.Error()`
- `models.ErrorResponse` (drop the public type; keep only the internal parser struct)

**New surface:**

- `errors.MissingParameter(op, paramName) *Error`
- `errors.Validation(op, resource, resourceID string, fields error) *Error`
- `errors.Network(op string, cause error) *Error` — and **actually called** from transport
- `errors.Configuration(op, message string, cause error) *Error` — for SDK setup errors
- `pkg/errors/doc.go` — package-level doc with taxonomy table
- `transport.Request{Operation, Resource, ResourceID, ...}` struct — populates the typed error from call site

#### Acceptance criteria

- [ ] `IsNetworkError(err)` returns `true` for real DNS / conn-refused / TLS failures (validated by integration tests against `localhost:0`)
- [ ] Every HTTP-derived `*Error` has non-empty `Operation` and `Resource`; `ResourceID` populated whenever the call site has it
- [ ] No user-facing error string contains `"custom retryable:"` prefix
- [ ] `models.<Entity>.Validate()` returns `*FieldErrors` accumulating all field problems (not just first)
- [ ] `errors.Is(err, errors.ErrInsufficientBalance)` works from retry-exhausted, network-wrapped, and direct error chains
- [ ] All `Check*Error` predicates removed; only `Is*Error` remain
- [ ] `MidazError` legacy type removed entirely
- [ ] `pkg/errors/doc.go` exists with the canonical taxonomy table and `errors.Is/As` patterns
- [ ] `Error.Retryable() bool` is the single source of truth for retry policies
- [ ] Integration test asserts: `client.New(client.WithBaseURL("http://nope.invalid:9999"), client.WithAuthToken("..."))` returns a `*Error{Category: CategoryNetwork}` on first call
- [ ] No `fmt.Errorf("...: %w", err)` in retry / access-manager that doesn't ultimately become a typed `*Error`

---

### Track 9 — Examples, Godoc, Mocks

**Severity:** MEDIUM · **Effort:** M · **Phase:** C · **Dependencies:** Phases A and B stable (examples reflect final API)

The polish phase. Discoverability lives or dies in pkg.go.dev — a coherent v3 API that nobody can find is wasted work. Today: zero `// See also` cross-references, only 7 runnable godoc Examples, no per-example READMEs, deprecated mock library, missing scenarios.

#### Findings

| # | Severity | Issue | Location |
|---|----------|-------|----------|
| 9.1 | HIGH | No README per example; no `examples/README.md` index | `examples/` |
| 9.2 | HIGH | No "hello world" minimal example (smallest is 98 lines) | `examples/` |
| 9.3 | HIGH | Zero examples for cursor-paginated endpoints | `examples/` |
| 9.4 | HIGH | `entities/mocks/` invisible — no example, uses deprecated `golang/mock` | `entities/mocks/`, `go.mod` |
| 9.5 | MEDIUM | Only 7 runnable godoc Examples in entire SDK; none on `client.New`, `c.X.*`, `models.*` | All packages |
| 9.6 | MEDIUM | 5 packages missing package-level doc comment | `pkg/access-manager`, `pkg/data`, `pkg/integrity`, `pkg/stats`, partial others |
| 9.7 | MEDIUM | `pkg/access-manager` directory contains `package auth` (covered in Track 2) | (Track 2) |
| 9.8 | MEDIUM | Zero `// See also` cross-references anywhere in codebase | All packages |
| 9.9 | MEDIUM | `client.go` package comment is 2 lines; no how-to-start, no example | `client.go:1-3` |
| 9.10 | MEDIUM | No `slog` integration example | `examples/` |
| 9.11 | MEDIUM | No idempotency-only example | `examples/` |
| 9.12 | MEDIUM | `examples/observability-demo/` is 6 months stale | `examples/observability-demo/` |
| 9.13 | LOW | `validation-example` doesn't actually use the SDK | `examples/validation-example/` |
| 9.14 | LOW | Stale committed binary path | `examples/mass-demo-generator/mass-demo-generator` |
| 9.15 | LOW | Setup boilerplate duplicated across 12 examples | `examples/*/main.go` |
| 9.16 | LOW | `pkg/pagination` package doc doesn't warn it's not wired (resolves with Track 5 deletion) | (Track 5) |

#### Proposed v3 shape

**`examples/README.md` index:**

A single landing page that maps user intent → example:

```
# Midaz Go SDK Examples

## Start here
- [01-hello-world/](01-hello-world/) — minimal init + 1 API call (≤30 lines)
- [02-auth/](02-auth/) — static token + Access Manager + tenant ID

## Common workflows
- [03-end-to-end/](03-end-to-end/) — org → ledger → account → transaction
- [04-listing-cursor/](04-listing-cursor/) — paginate transactions with iter.Seq2
- [05-listing-pages/](05-listing-pages/) — paginate accounts with page metadata

## Behavior & resilience
- [06-idempotency/](06-idempotency/) — auto, manual, and per-call opt-out
- [07-retries/](07-retries/) — default, custom policy, disabled
- [08-logging-slog/](08-logging-slog/) — slog integration

## Testing & observability
- [09-testing-with-mocks/](09-testing-with-mocks/) — go.uber.org/mock for unit tests
- [10-observability-otel/](10-observability-otel/) — OTel tracing + metrics + logs

## Reference / advanced
- [mass-demo-generator/](mass-demo-generator/) — production-like data generator
- [workflow-with-entities/](workflow-with-entities/) — full-stack workflow reference
```

**Per-example README template:**

```markdown
# 04-listing-cursor

## What this demonstrates
Iterating cursor-paginated transactions with `iter.Seq2`, including:
- Filter setup
- Error handling per item
- Early termination

## When to use this pattern
- Listing transactions, operations, operation routes, transaction routes
- Any endpoint where the SDK signals cursor-only via the typed `ListOpts`

## How to run
```bash
export MIDAZ_AUTH_TOKEN=midaz_pat_...
go run ./examples/04-listing-cursor
```

## Expected output
```
[0] tx_abc123 SETTLED
[1] tx_def456 PENDING
...
processed 50 transactions
```
```

**Hello-world example (`examples/01-hello-world/main.go`):**

```go
// Package main is the simplest possible Midaz SDK demo.
//
// Usage:
//   export MIDAZ_AUTH_TOKEN=midaz_pat_...
//   go run ./examples/01-hello-world
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/LerianStudio/midaz-sdk-golang/v3"
)

func main() {
    c, err := client.New(
        client.WithEnvironment(client.EnvProduction),
        client.WithAuthToken(os.Getenv("MIDAZ_AUTH_TOKEN")),
    )
    if err != nil {
        log.Fatalf("client init failed: %v", err)
    }
    defer c.Shutdown(context.Background())

    ctx := context.Background()
    orgs, err := c.Organizations.List(ctx, client.OrgListOpts{Limit: 5})
    if err != nil {
        log.Fatalf("list orgs failed: %v", err)
    }
    for _, org := range orgs.Items {
        fmt.Printf("- %s (%s)\n", org.LegalName, org.ID)
    }
}
```

**Runnable godoc Examples — minimum bar:**

Every public constructor and every `List*` / `Create*` / `Update*` / `Delete*` method gets a `func Example*` that compiles and runs against a mock/fake. Target list:

- `client.New` (with WithAuthToken + WithEnvironment)
- `client.New` (with WithAccessManager)
- `client.New` (with FromEnvironment)
- `c.Organizations.Create`, `Get`, `Update`, `Delete`, `List`, `ListAll`
- `c.Ledgers.*` (same set)
- `c.Accounts.*` (same set + `GetAccountBalance`)
- `c.Transactions.Create`, `CreateInflow`, `CreateOutflow`, `CreateAnnotation`, `Get`, `List`, `ListAll`
- `c.Balances.Get`, `List`, `ListByAccountAlias`
- `entities.WithIdempotencyKey(ctx, key)`
- `entities.WithoutAutoIdempotency(ctx)`
- `entities.WithRequestTenantID(ctx, id)`
- `entities.WithIncludeDeleted(ctx, true)`
- `errors.Is(err, errors.ErrNotFound)` pattern
- `errors.As(err, &sdkErr)` pattern with field walk

**Cross-references via `// See also:`:**

Every doc comment for a related concept gets explicit links:

```go
// CreateInput is the input for creating an Account.
//
// Use NewCreateInput to construct a properly-initialized instance.
//
// See also:
//   - models.UpdateInput for partial updates
//   - models.ListOpts for listing accounts
//   - entities.WithIdempotencyKey for idempotent creation
type CreateInput struct { /* ... */ }
```

```go
// WithIdempotencyKey attaches a user-supplied idempotency key to the request context.
//
// See also:
//   - WithoutAutoIdempotency for opting out of auto-generated keys
//   - client.WithIdempotency to disable auto-idempotency globally
func WithIdempotencyKey(ctx context.Context, key string) context.Context { /* ... */ }
```

**Mocks migration:**

```go
// entities/mocks/mock_accounts.go (regenerated)
//go:generate mockgen -source=../accounts.go -destination=mock_accounts.go -package=mocks
```

Migrate from deprecated `github.com/golang/mock` to `go.uber.org/mock` (already in `go.mod` indirect). Single PR; mostly mechanical.

**Testing example (`examples/09-testing-with-mocks/main_test.go`):**

```go
package main

import (
    "context"
    "testing"

    "github.com/LerianStudio/midaz-sdk-golang/v3/services/accounts"
    "github.com/LerianStudio/midaz-sdk-golang/v3/services/accounts/mocks"
    "go.uber.org/mock/gomock"
)

func TestAccountCreation(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockSvc := mocks.NewMockService(ctrl)

    expected := &accounts.Account{ID: "acc-123", Name: "Test"}
    mockSvc.EXPECT().
        Create(gomock.Any(), "org-1", "ledger-1", gomock.Any()).
        Return(expected, nil)

    // ... use mockSvc as accounts.Service in your handler under test
}
```

**Setup boilerplate consolidation:**

Add `examples/internal/quickstart/quickstart.go`:

```go
// Package quickstart provides shared example setup helpers.
package quickstart

func MustClient() *client.Client {
    c, err := client.New(
        client.FromEnvironment(),
        client.WithDebug(os.Getenv("DEMO_DEBUG") == "true"),
    )
    if err != nil {
        log.Fatalf("client init failed: %v", err)
    }
    return c
}
```

Each example imports it: `c := quickstart.MustClient()`.

**Package doc cleanup:**

Add or expand package comments for:
- `pkg/auth` (renamed from `pkg/access-manager`)
- `pkg/data`
- `pkg/integrity`
- `pkg/stats`
- `pkg/generator`
- `pkg/validation`

Each gets a multi-line comment with: what it does, when to use it, link to relevant docs.

**`client/doc.go`:**

```go
// Package client is the entry point for the Midaz Go SDK.
//
// # Quickstart
//
//   c, err := client.New(
//       client.WithEnvironment(client.EnvProduction),
//       client.WithAuthToken("midaz_pat_..."),
//   )
//   if err != nil { return err }
//   defer c.Shutdown(ctx)
//
//   org, err := c.Organizations.Get(ctx, "org-id")
//
// # Authentication
//
// See [docs/auth.md] for a full walkthrough of authentication options:
// static API tokens, OAuth via Access Manager, env-driven configuration.
//
// # Multi-tenancy
//
// Set a default tenant via WithTenantID, override per-request via
// entities.WithRequestTenantID(ctx, id). See [docs/multi-tenancy.md].
//
// # Logging & observability
//
// Inject a *slog.Logger via WithLogger. See [docs/logging.md] and
// [docs/tracing.md].
//
// # Pagination
//
// Every List* method returns one page. ListAll returns iter.Seq2[T, error]
// for full-collection iteration. See [docs/pagination.md].
//
// # Errors
//
// Every error is *errors.Error with structured Category, Code, Operation,
// and Resource fields. Use errors.Is / errors.As. See [docs/errors.md].
package client
```

#### Acceptance criteria

- [ ] `examples/README.md` exists and indexes every example
- [ ] Every `examples/*/` has a `README.md` following the template
- [ ] `examples/01-hello-world/main.go` exists, ≤ 30 lines, runs against local Midaz with `MIDAZ_AUTH_TOKEN`
- [ ] At least 30 runnable `func Example*` godoc functions across the SDK (currently 7)
- [ ] Every package has a package-level doc comment with a "See also" / "Examples" / "Quickstart" section
- [ ] `client/doc.go` exists with the full quickstart + topic links
- [ ] `entities/mocks/` regenerated using `go.uber.org/mock`; deprecated `github.com/golang/mock` removed from `go.mod`
- [ ] `examples/09-testing-with-mocks/main_test.go` exists and documents mock-based testing
- [ ] `examples/observability-demo/` updated to compile and run against v3 API
- [ ] `examples/validation-example/` either renamed (`examples/pkg-validation-demo/`) or merged into the validation package's tests
- [ ] `examples/internal/quickstart/` package extracts shared setup boilerplate
- [ ] All public types/funcs cross-reference related concepts via `// See also:` comments
- [ ] `Makefile` target `make examples-test` builds and lints every example
- [ ] No "stale" examples (last meaningful commit > 90 days old without verification PR)

---

## Sequencing & Dependencies

The 9 tracks have hard dependencies. Order matters.

### Phase A — Foundation (4–5 weeks)

The v3 surface shape. Nothing in Phase B works without these.

```
Week 1-2: Track 1 (Naked SDK & Entry Points) + Track 3 (Implicit env reads, parallel)
Week 3:   Track 2 (Auth & Tenant) — depends on Track 1
Week 4-5: Track 4 (Logging) — depends on Tracks 1, 3
```

**Phase A exit criteria:**
- [ ] `client.New(client.WithAuthToken("..."), client.WithEnvironment(client.EnvProduction))` produces a fully-validated client
- [ ] All implicit env-var reads removed; `FromEnvironment()` is the only env loader
- [ ] `client.WithLogger(*slog.Logger)` is the canonical logging surface
- [ ] No `MIDAZ_DEBUG` bypass path; debug routes through the logger
- [ ] All deprecated v2 surface marked with `// Deprecated:` with migration breadcrumbs

### Phase B — Models & Data Flow (4–6 weeks)

Where most of the surface area churn happens. Tracks 5, 6, 7, 8 can partially overlap once Track 6 (options) lands.

```
Week 1-2: Track 6 (Functional Options Sprawl) — must land first
Week 3-4: Track 7 (Builder/Model Drift) — depends on Track 6
Week 4-6: Track 5 (Pagination) + Track 8 (Errors), parallel — depend on Tracks 1, 6
```

**Phase B exit criteria:**
- [ ] `client.With*` exposes ≤ 60 options total (current ~120)
- [ ] Every entity follows the canonical Account-shaped builder pattern
- [ ] Every list endpoint exposes `List`, `ListAll`, `ListPages` returning unified types
- [ ] Network failures are typed `*Error{Category: CategoryNetwork}`
- [ ] HTTP errors carry `Operation` + `Resource` + `ResourceID` end-to-end
- [ ] `pkg/pagination` deleted

### Phase C — Polish (2–3 weeks)

```
Week 1: examples/, godoc Examples, READMEs
Week 2: package docs, cross-references, mocks migration
Week 3: docs/ updates, migration guide, release prep
```

**Phase C exit criteria:**
- [ ] At least 30 runnable godoc Examples
- [ ] Every example has a README following the template
- [ ] Every package has a real package-level doc comment
- [ ] Mocks migrated to `go.uber.org/mock`; deprecated `github.com/golang/mock` removed
- [ ] `docs/migration-v2-to-v3.md` exists with worked examples for every breaking change

### Total estimated timeline

**Realistic:** 12–15 weeks of focused engineering, including testing, docs, and review checkpoints.

**Aggressive:** 9 weeks if Phases A and B overlap heavily (Track 6 design starts in parallel with Phase A while Track 6 implementation waits for Phase A to land).

**Conservative:** 18 weeks if multiple iterations are needed on the v3 API shape based on internal review feedback.

---

## Migration Story

The v3 release ships with deprecated shims for one minor version (v2.99.x) so downstream Lerian customers have a graceful migration path. Two release vehicles:

### v2.99.0 (transition release)

Adds v3-shaped APIs **alongside** v2 APIs as opt-in. Marks v2 surface as `// Deprecated:` with replacement breadcrumbs.

```go
// v2.99.0 example: both styles work
c, _ := client.New(
    client.WithAuthToken("..."),    // NEW v3-style
    client.WithEnvironment(client.EnvProduction),
)

// also works with deprecation warning:
c, _ := client.New(
    client.WithConfig(cfg),         // DEPRECATED — see WithAuthToken/WithAccessManager
    client.UseAllAPIs(),            // DEPRECATED — Entity is always initialized in v3
)
```

CI lints customer code for deprecated symbols and emits warnings.

### v3.0.0 (cleanup release)

Removes all deprecated v2 surface. Adopts the `/v3` module path:

```go
import client "github.com/LerianStudio/midaz-sdk-golang/v3"
```

`go.mod` major-version bump to v3. Customers update their import path; deprecated APIs are no longer available.

### `docs/migration-v2-to-v3.md`

A worked-examples guide. For every breaking change, side-by-side v2 / v3 code:

```markdown
## Auth setup

### Before (v2)
```go
am := auth.AccessManager{...}
cfg, _ := config.NewConfig(config.WithAccessManager(am))
c, _ := client.New(client.WithConfig(cfg), client.UseAllAPIs())
```

### After (v3)
```go
c, _ := client.New(client.WithAccessManager(client.AccessManager{...}))
```
```

(Repeat for every track's breaking changes.)

### Backward-compat fallbacks (v2.99 only)

Some changes are sufficiently invasive that the v2.99 shim is non-trivial. Notably:

- **`*ListOptions` → per-service typed `ListOpts`** — v2.99 keeps `*ListOptions` as a deprecated alias that converts to/from the typed struct internally
- **`c.Entity.Accounts.X` → `c.Accounts.X`** — v2.99 keeps `c.Entity` as a deprecated struct that delegates to the new top-level fields
- **Two `Validate()` semantics (first-error vs accumulated)** — v2.99 keeps both signatures; deprecated path returns first error, new path returns `*FieldErrors`

These are documented as v2.99 → v3 breaking changes in the migration guide so customers know what to expect.

### Customer notification timeline

- **T-0 (v2.99.0 release):** Announce v3 design; migration guide live; v2 marked deprecated; customers can begin migrating
- **T+30 days (v2.99.1+):** Bug-fix patches only on v2.99; new features land only on v3 main
- **T+60 days (v3.0.0 release):** Hard cutoff; v2 surface fully removed
- **T+180 days (v3.1.0+):** v2.99 maintenance ends; security fixes only

---

## Acceptance Criteria Summary

A v3 candidate release passes ALL of these:

### Track 1 — Naked SDK
- [ ] `client.New()` (zero opts) returns typed configuration error
- [ ] All services on `*Client` always non-nil after `New()` succeeds
- [ ] No public `entities.NewXxxEntity` constructors
- [ ] No `Use*` options
- [ ] Top-level package re-exports common types

### Track 2 — Auth & Tenant
- [ ] `client.WithAuthToken("token")` is a single-line setup
- [ ] `client.WithAccessManager(am)` works without `pkg/config` import
- [ ] `pkg/auth` directory matches package name
- [ ] `docs/auth.md` and `docs/multi-tenancy.md` exist

### Track 3 — Implicit env reads
- [ ] `os.Getenv` calls outside `pkg/config/FromEnvironment` and stdlib proxy: 0
- [ ] `MIDAZ_DEBUG=true` does NOT override `WithDebug(false)` (test asserted)
- [ ] `MIDAZ_ENABLE_RETRIES` removed from codebase

### Track 4 — Logging
- [ ] `client.WithLogger(*slog.Logger)` works with stdlib slog
- [ ] `Client.Logger()` returns non-nil always
- [ ] Retry attempts emit structured logs with documented field schema
- [ ] No unconditional stderr writes anywhere
- [ ] `docs/logging.md` with integration examples for slog/charm/zap/zerolog

### Track 5 — Pagination
- [ ] Every list method exposes `List`, `ListAll`, `ListPages`
- [ ] Per-service typed `ListOpts` (compile-time prevention of misuse)
- [ ] `pkg/pagination` deleted
- [ ] `MaxLimit` enforced via typed error

### Track 6 — Options
- [ ] ≤ 60 `With*` options (down from ~120)
- [ ] No name collisions across packages
- [ ] All `With*` options return `Option, error`
- [ ] Single canonical "off" toggle per concept

### Track 7 — Builder/Model
- [ ] All entities follow Account-shaped pattern (no `mmodel` embedding in public types)
- [ ] Every Update method takes typed pointer (no `input any`)
- [ ] Every input has `Validate()` returning `*FieldErrors`
- [ ] Hyphenated filenames renamed to underscores

### Track 8 — Errors
- [ ] `IsNetworkError(err)` works for real network failures
- [ ] HTTP errors carry `Operation` + `ResourceID`
- [ ] `MidazError` legacy type removed
- [ ] `pkg/errors/doc.go` exists

### Track 9 — Examples & docs
- [ ] `examples/README.md` indexes every example
- [ ] Per-example READMEs follow template
- [ ] `examples/01-hello-world/` ≤ 30 lines
- [ ] ≥ 30 runnable godoc Examples
- [ ] Mocks migrated to `go.uber.org/mock`

### Cross-cutting
- [ ] `make ci` passes clean (lint, vet, test, gosec)
- [ ] No `// nolint` annotations on critical paths without ADR justification
- [ ] Coverage ≥ 80% on changed code
- [ ] `examples/` builds clean
- [ ] No `panic`, `log.Fatal`, `os.Exit` in library code

---

## Decision Log

A running record of design decisions with rationale. Append-only; never edit history.

### 2026-05-05 — Greenfield v3 with deprecated shims for 1–2 minor versions
**Decision:** v3 is a major-version bump. Breaking changes are accepted. v2.99.x ships as a transition release with deprecated shims for ~30 days.
**Rationale:** Convergence problem only goes away if we converge fully. Half-measures (additive only) leave the user with two surfaces forever.
**Tradeoffs:** Migration cost on Lerian customers. Mitigated by: clear migration guide, automated lint warnings, 30-day transition window.
**Decided by:** Fred

### 2026-05-05 — Roll quick wins into v3 (no v2.x patch ship)
**Decision:** The 10 surgical "non-breaking quick wins" identified during the audit (env-debug cleanup, error-context plumbing, network-error typing, broken doc fix, mocks migration, hello-world example) ship as part of v3, not as a separate v2.x patch.
**Rationale:** Single coherent release moment. Customers feel "the SDK got way better in v3" rather than "small papercuts fixed across two minor versions."
**Tradeoffs:** Customers wait 12+ weeks for any improvement. Acceptable because v2 is functional today.
**Decided by:** Fred

### 2026-05-05 — Single living document at `docs/v3-dx-plan.md`
**Decision:** No GitHub issues per track. No separate audit report files. One markdown doc that evolves through the project.
**Rationale:** Lower coordination overhead; the doc is the source of truth and gets updated as decisions ship. GitHub issues would duplicate state and rot.
**Tradeoffs:** Less external visibility (Lerian engineers must know to read this doc). Mitigated by: linking from CONTRIBUTING.md and the v3 release notes.
**Decided by:** Fred

### 2026-05-05 — All 9 tracks land in v3
**Decision:** Maximum scope. ~12–15 weeks of focused engineering.
**Rationale:** Convergence is binary. Shipping 5 of 9 tracks leaves the SDK partially converged, which is worse than today (because users have to learn TWO conventions instead of one).
**Tradeoffs:** Long timeline; risk of context loss across the project. Mitigated by: this living doc + status tracker below.
**Decided by:** Fred

### 2026-05-05 — `*slog.Logger` is the canonical logger
**Decision:** Drop the bespoke `observability.Logger` interface; replace with `*slog.Logger`.
**Rationale:** Every modern Go SDK in 2026 uses slog. Adapters exist in user-space for zap/zerolog/logrus. Custom interface is convergence debt.
**Tradeoffs:** Customers using the v2 `observability.Logger` interface need to migrate. Mitigated by: documented adapter shim in v2.99 + migration guide.

### 2026-05-05 — `iter.Seq2` is the iteration contract
**Decision:** All `ListAll` methods return `iter.Seq2[T, error]`.
**Rationale:** Go 1.26 idiom; compiler-enforced; composes with `slices.Collect`, `maps.Values`, etc. Project go.mod already requires Go 1.26.
**Tradeoffs:** Customers on Go < 1.23 can't use `range` over `iter.Seq2`. Mitigated by: project requires Go 1.26 already (`go.mod` line 3); no compat issue.

### 2026-05-05 — Per-service typed `ListOpts`, not unified mega-struct
**Decision:** Each service exposes its own `ListOpts` value type with only fields valid for that endpoint kind.
**Rationale:** Compile-time prevention of `WithPage` on cursor-only endpoints. Eliminates the silent stderr-warning footgun (Track 5 finding 5.5).
**Tradeoffs:** More types to learn (16 ListOpts vs 1). Mitigated by: each is small (3-5 fields + typed Filters); follows the same shape; godoc auto-discovers them.

### 2026-05-05 — Delete `pkg/pagination` entirely
**Decision:** The `pkg/pagination` package (1488 LOC) is dead code from a user perspective. Delete in v3 rather than wire it.
**Rationale:** It was an abstraction without a consumer. The new typed-per-service approach replaces it. If users need a generic paginator, they can build one on `iter.Seq2` in user-space — `slices.Collect` + a thin loop is sufficient.
**Tradeoffs:** Customers (if any) who imported `pkg/pagination` directly will need to migrate. Mitigated by: low likelihood (zero entity wiring means it was never the natural path); migration guide will cover.

### 2026-05-05 — `client.Entity` indirection removed; services live directly on `*Client`
**Decision:** `c.Entity.Accounts.Get(...)` → `c.Accounts.Get(...)`.
**Rationale:** The Entity layer was a transitional refactor that never delivered the value it promised. It just adds typing for users.
**Tradeoffs:** Significant migration ceremony (every customer call site changes). Mitigated by: deprecated `c.Entity` field in v2.99 that delegates to top-level services.

### 2026-05-05 — `MIDAZ_ENABLE_RETRIES` removed; `MIDAZ_MAX_RETRIES=0` is the disable mechanism
**Decision:** Hidden killswitch deleted. Documented mechanism is `MIDAZ_MAX_RETRIES=0`.
**Rationale:** `MIDAZ_ENABLE_RETRIES` was undocumented, lived only in `entities/http.go:175`, and duplicated functionality already covered by `MIDAZ_MAX_RETRIES`.
**Tradeoffs:** Anyone (internal or external) using `MIDAZ_ENABLE_RETRIES=false` in their deployment configs needs to migrate to `MIDAZ_MAX_RETRIES=0`. Mitigated by: this var was undocumented, so customer impact should be zero.

### 2026-05-05 — Batch 1A: module path `/v2` → `/v3`, package `client` → `midaz` (commit f8e2109)
**Decision:** Mass rename via sed: 201 Go files updated to `/v3` import path; root package renamed; `client.go` → `midaz.go`. Aliases (`client "..."`) in 32 importers were initially left in place because Go doesn't require alias to match package name; the alias still resolves to the renamed `midaz` package.
**Implementation surprise:** When dropping the `client` alias in `pkg/transaction/batch.go`, `goimports` (or some auto-tool in the dev environment) silently inserted `github.com/moby/moby/client` to satisfy the orphan `client.X` references. Workaround: explicitly add the v3 import without an alias and verify with `go build` immediately after.
**Tradeoffs:** Examples still use `client "..."` aliases as a back-compat. Track 9 (Phase C) rewrites them to use the canonical `midaz.X` idiom from day 1.

### 2026-05-05 — Batch 1B: always-on Entity surface (commit f2a0fee)
**Decision:** Removed `Client.useEntity` flag; deleted `UseAllAPIs`, `UseEntityAPI`, `UseEntity` trio; `setupEntity()` is called unconditionally in `New()`.
**Rationale:** The opt-in flag was a footgun: customers forgetting `UseAllAPIs()` got `c.Entity == nil` and panicked on first call. v3 makes it impossible.
**Net change:** -56 lines (pure deletion). 14 files touched (5 source + tests + 12 example files where `Use*` calls were stripped).

### 2026-05-05 — Batch 1C: services hoisted via anonymous embed (commit d5e410a)
**Decision:** `Client` embeds `*entities.Entity` anonymously. Promoted fields (`c.Accounts`, `c.Transactions`, ...) become the v3 idiom; `c.Entity.Accounts` remains accessible because the embedded field name is `Entity` (matches the type name without `*`).
**Rationale:** Embedding gives both new idiom AND back-compat for free. Considered Option B (explicit per-service fields, no `Entity` field at all) but rejected — embedding is one line of struct change with promotion as a bonus, and the back-compat eases v2.99 transition.
**Implementation note:** Mass `sed -E` replaced `.Entity.<Service>` → `.<Service>` across 30 non-entity files in one pass. Macro pattern enumerates all 16 service names; verified no collision risk because no `<Service>Service` field exists on the Entity struct (only types).

### 2026-05-05 — Batch 1D: `pkg/sdkctx/` introduced; entities helpers deprecated (commit af8bbea)
**Decision:** Created new `pkg/sdkctx/` package with 5 helpers. Renamed `entities.WithTenantID` → `sdkctx.WithRequestTenantID` to disambiguate from the client-level `midaz.WithTenantID` option. Added two new helpers (`WithIncludeDeleted`, `WithHardDelete`) as Track 7 dependencies.
**Implementation note:** Originally planned to fully delete `entities/` package in this batch. Deferred — `entities/` still hosts the 16 service interfaces and HTTP client, which is too much to move in one batch. Full deletion happens in Phase B (Track 7) when services move to per-service packages. For now, `entities/context.go` is reduced to deprecated shims that delegate to sdkctx.
**Tests:** 8 new tests in `pkg/sdkctx/sdkctx_test.go` covering all helpers, edge cases (nil ctx, whitespace trim, empty key no-op), precedence rules (explicit key beats suppression), and helper independence (5-channel non-interference).

### 2026-05-05 — Batch 1F: eager validation contract (commit 582dd99)
**Decision:** `midaz.New()` runs `c.config.Validate()` after applying options. Failures become typed `*errors.Error{Category: CategoryConfiguration}`. Underlying option-apply errors are wrapped via `%w` and reachable via `errors.Unwrap`.
**Public API additions:** `errors.CategoryConfiguration`, `errors.CodeConfiguration`, `errors.ErrConfiguration` sentinel, `errors.NewConfigurationError(operation, message, err) *Error` constructor, `errors.IsConfigurationError(err) bool` predicate, `pkg/config.Config.Validate()` (was private `validateConfig`).
**Migration cost for tests:** 3 existing tests asserted on raw error strings (`"option cannot be nil"`, `"config cannot be nil"`). Updated to use `errors.IsConfigurationError(err)` + `errors.Unwrap(err)` for the original message check.
**Implementation note:** When I added `sdkerrors "github.com/.../v3/pkg/errors"` import to midaz.go, an auto-tool stripped it once. Re-added explicitly. Watch for this pattern in dev env.

### 2026-05-06 — Batch 1E: collapse `entities/` constructors to single canonical entry (commit ab69e05)
**Decision:** Made `entities.NewEntityWithConfig` (called only by `midaz.New()`) the single supported construction path. Deleted 3 redundant entity-level constructors (`NewEntity`, `New`, `NewWithServiceURLs`) and the no-op `WithContext` option. Unexported all 16 service constructors (`NewAccountsEntity` → `newAccountsEntity` etc.). Stripped tutorial-style godoc bloat from 7 service constructors that demonstrated now-impossible external usage.
**Net delta:** 39 files, **-490 lines** (749 deletions, 259 insertions). Largest cleanup batch in Track 1 by far.
**Resume-state correction:** The previous resume note claimed 6 factory traps existed on `Client` (`NewAccount/NewLedger/...` at `midaz.go:733-761`). They don't (verified via grep + reading). Either deleted in 1A-1D or never existed in v2. Scope item considered satisfied by absence; no code change needed.
**Test infrastructure decision:** Tests inside `entities/*_test.go` cannot use `midaz.New()` because of the import cycle direction (entities is a leaf; root depends on it). Three options considered: (a) directly construct `&Entity{...}` per call site — 8 lines × ~10 sites = 80 lines of boilerplate; (b) use `NewEntityWithConfig` with a fake Config interface impl — verbose for tests; (c) add a private `newTestEntity(t, ...)` helper that mirrors the deleted `NewEntity` contract and lives in `entities/entity_test.go`. Chose (c). 11 test sites migrated cleanly. The helper depends on the same internal primitives (`normalizeBaseURLs`, `NewHTTPClient`, `initServices`) so it stays in lock-step with production behavior.
**Test migrations completed:**
  - `pkg/transaction/helper_contract_test.go`: now routes through `midaz.New()` + `c.SetAuthToken("token")`. This is the *correct* v3 path for any external test, and the migration verified end-to-end public construction works.
  - `entities/entity_test.go`: refactored `TestNewWithServiceURLs_DefaultsMissingCRMURLToOnboarding` to test `normalizeBaseURLs` directly (semantically equivalent; tests the actual primitive doing the work).
  - `entities/business_observability_test.go` (3 sites), `entities/http_tenant_test.go` (4 sites), `entities/slice2_regression_test.go` (4 sites): all migrated to `newTestEntity`.
  - `TestEntityConstructors_WithNilOption_ReturnError` and `TestSlice2NewWithServiceURLs_DefaultsMissingCRMURLToOnboarding` deleted as duplicates (coverage now lives in `validation_contract_test.go` and `entity_test.go` respectively).
**Implementation surprise — sed corrupted test names:** The mass `s/NewXxxEntity/newXxxEntity/g` rename also caught test function names: `func TestNewAccountsEntity` → `func TestnewAccountsEntity`, which `go vet` rejects with "first letter after 'Test' must not be lowercase". Renamed all 8 affected to `Test_newXxxEntity` form (Go's test framework allows `Test_xxx` for unexported subjects). Lesson: when bulk-renaming a function, anchor your sed with `func Newname(` or `Newname\(` to avoid catching test names.
**Lint regression:** Batch 1E added exactly **1 net lint issue** (`c.Entity.SetAuthToken("token")` should be `c.SetAuthToken("token")` via promoted method) — fixed before commit. Discovery: `make lint` pre-1E shows **11 pre-existing issues** that were never caught during 1A-1F. Apparently the lint step was not part of the verification flow. **Action item logged**: dedicated lint-cleanup commit before Batch 1G or Track 1 close. Categorized:
  - 8 staticcheck `QF1008 drop .Entity from selector` in `client_test.go`, `midaz.go`, `slice2_regression_test.go` — trivial.
  - 3 testifylint `error-is-as` / `error-nil` in `validation_contract_test.go` (Batch 1F) — refactor to `assert.ErrorIs(t, err, target)` etc.
  - 5 staticcheck `SA1012 do not pass nil context` in `pkg/sdkctx/sdkctx_test.go` (Batch 1D) — these are *intentional* nil-safety tests. Fix is `//nolint:staticcheck // intentional nil-context for nil-safety verification` annotations.

### 2026-05-06 — Lint sweep: 11 pre-existing issues cleaned (commit ab98a64)
**Decision:** Brought the v3 branch to a fully lint-clean state by addressing all 11 pre-existing issues that had accumulated during Batches 1A-1F. `make lint` was apparently not part of the verification flow until Batch 1E surfaced the gap. Going forward, `make lint` joins `go build`, `go test`, and `make verify-sdk` as required verification before every batch commit.
**Categories fixed:**
  - 3 staticcheck QF1008 `drop .Entity from selector` in `client_test.go:183`, `midaz.go:302`, `slice2_regression_test.go:43,55`. The .Entity qualifier was redundant once Client embeds *entities.Entity (Batch 1C); the linter was right that the v3 'flat surface' contract obscured itself when test code still wrote the long form.
  - 3 testifylint in `validation_contract_test.go` (Batch 1F): `assert.True(t, stderrors.Is(err, target))` → `require.ErrorIs(t, err, target)`; `require.True(t, stderrors.As(err, &sdkErr))` → `require.ErrorAs(t, err, &sdkErr)`; `require.NotNil(t, inner)` → `require.Error(t, inner)` (last one because `Unwrap` returns an `error` value; the canonical 'wrap-reachable' assertion is `require.Error`).
  - 5 staticcheck SA1012 `do not pass nil context` in `pkg/sdkctx/sdkctx_test.go` (Batch 1D): annotated each with `//nolint:staticcheck // intentional nil-context for nil-safety verification`. These tests deliberately exercise nil-ctx behavior on the helpers (which promote nil to context.Background and return zero-values from extractors). The nil pass IS the test; rewriting to context.TODO() would defeat the test.
**Net:** 5 files, +12/-7. Zero behavior change. Lint baseline 11 → 0.

### 2026-05-06 — Batch 1G: 56 type aliases at midaz package level (commit 588997a)
**Decision:** Re-exported 56 high-value types from `models/` via Go's `type X = Y` form (true type identity, not distinct types). Single source of truth: types live in `models/`; the aliases in `types.go` are pure naming convenience that lets user code stay on a single import path. Curated from a 106-type universe in `models/` — anything sitting in 95% of normal SDK usage is included; deprecated types and internal request shapes stay in `models/` only.
**Aliases by category:**
  - 16 resource entities (Account, AccountType, Alias, Asset, AssetRate, Balance, Holder, Ledger, MetadataIndex, Operation, OperationRoute, Organization, Portfolio, Segment, Transaction, TransactionRoute)
  - 16 Create inputs (CreateAccountInput, …, CreateTransactionRouteInput)
  - 14 Update inputs (UpdateAccountInput, …, UpdateTransactionRouteInput) — note 14 not 16 because UpdateMetadataIndexInput and UpdateAssetRateInput don't exist as standalone types in v2 schema
  - 5 transaction sub-DTOs (AmountInput, DistributeInput, FromToInput, SendInput, SourceInput)
  - 3 pagination & list types (ListOptions, ListResponse[T], Pagination)
  - 2 common (Status, Address)
**Generic alias technical note:** `type ListResponse[T any] = models.ListResponse[T]` is a Go 1.24+ feature (parameterized type aliases). Repo runs Go 1.26.0+ per `go.mod`, so this is safe. Verified via `TestGenericListResponseAlias`.
**Lint accommodation:** revive flagged each alias as 'exported needs doc comment' (56 issues). Adding 56 near-identical `// X is an alias for models.X` comments would be noise without info — godoc follows the alias and surfaces the source type's doc directly, which is the canonical user view. Suppressed via `//revive:disable:exported` at file scope with a written rationale comment. The package-level commentary documents the contract.
**Tests added (59 cases in `types_contract_test.go`):**
  - `TestTypeAliasesAreIdentical`: 56 sub-tests using `reflect.TypeOf` comparison — the canonical proof of type identity. If any future commit silently downgrades a `type X = Y` alias to a distinct type `type X Y`, this test fails.
  - `TestGenericListResponseAlias`: separate case for the generic alias (because reflect.TypeOf needs a concrete instantiation).
  - `TestAliasesUsableInUserFlow`: 2 directional checks proving values flow without conversions or boxing — the practical user-flow proof.
**Track 1 closes here.** All Batch 1A-1G acceptance criteria within Track 1's scope are satisfied. Three deferred items (`WithAuthToken`, `Logger()` non-nil, examples migration) live in Tracks 2/4/9 respectively per the original sequencing plan.

### 2026-05-05 — Anonymous embedding decision deferred Track 7's mmodel concern
**Observation:** The current `Entity.Accounts` field is typed `entities.AccountsService` (an interface). When we eventually delete `entities/` in Phase B, the embed becomes `*services.Hub` or similar. The migration path: change one line in Client struct, regenerate `c.X` references. Easy. Documented here so future-me doesn't trip over it.

---

## Status Tracker

Live as of: 2026-05-05 (post-session 1). Update with every commit batch.

### Phase A — Foundation

| Track | Status | Started | Completed | Branch / Commits | Notes |
|-------|--------|---------|-----------|------------------|-------|
| 1 — Naked SDK & Entry Points | 🟢 **COMPLETE** (7/7 + lint sweep) | 2026-05-05 | 2026-05-06 | `v3` branch, commits f8e2109..588997a | All batches shipped. Acceptance criteria for `WithAuthToken` (Track 2), `Logger()` non-nil (Track 4), and example migration (Track 9) intentionally deferred. |
| 2 — Auth & Tenant Chaos | 🔵 Not started | — | — | — | Depends on Track 1 completion. Adds `WithAuthToken`, re-exports `AccessManager`, renames `pkg/access-manager` → `pkg/auth`. |
| 3 — Implicit env reads | 🔵 Not started | — | — | — | Independent; can start any time. **High-impact mechanical cleanup**: 14 redundant `os.Getenv("MIDAZ_DEBUG")` reads + 5 hidden env vars in `entities/http.go`. |
| 4 — Logging gap | 🔵 Not started | — | — | — | Depends on Tracks 1, 3. **Fred-flagged critical**. Introduces `*slog.Logger` as canonical contract; deletes `MIDAZ_DEBUG` bypass; adds retry-attempt logging. |

### Phase B — Models & Data Flow

| Track | Status | Started | Completed | Branch / Commits | Notes |
|-------|--------|---------|-----------|------------------|-------|
| 5 — Pagination footguns | 🔵 Not started | — | — | — | Depends on Tracks 1, 6. Per-service typed `ListOpts`, `iter.Seq2` iterators, deletes dead `pkg/pagination`. |
| 6 — Functional options sprawl | 🔵 Not started | — | — | — | Must land before 5, 7. Consolidates ~120 options to ≤60. |
| 7 — Builder/Model API drift | 🔵 Not started | — | — | — | Depends on Track 6. Converges every entity on Account-shaped pattern. |
| 8 — Error system actionability | 🔵 Not started | — | — | — | Depends on Tracks 1, 4. Network-error typing, `Operation`/`ResourceID` plumbing. |

### Phase C — Polish

| Track | Status | Started | Completed | Branch / Commits | Notes |
|-------|--------|---------|-----------|------------------|-------|
| 9 — Examples, godoc, mocks | 🔵 Not started | — | — | — | Depends on Phase A + B stable. Per-example READMEs, hello-world, slog example, mocks → `go.uber.org/mock`. |

### Track 1 batch-level progress

| Batch | Scope | Status | Commit | Verification |
|-------|-------|--------|--------|--------------|
| 1A | go.mod /v3, package midaz, ~80 file imports | ✅ Done | `f8e2109` | 202 files, 458/458 sym swap, all green |
| 1B | Always-on Entity, delete Use* trio | ✅ Done | `f2a0fee` | 14 files, -56 net lines, all green |
| 1C | Service hoisting via embedded `*entities.Entity` | ✅ Done | `d5e410a` | 31 files, +8 net, all green |
| 1D | Introduce `pkg/sdkctx`; deprecated shims in entities | ✅ Done | `af8bbea` | 9 files, +324 net, +8 sdkctx tests |
| 1E | Constructor cleanup (16 `entities.NewXxxEntity` unexport, 3 redundant entity constructors deleted, `entities.WithContext` no-op deleted) | ✅ Done | `ab69e05` | 39 files, **-490 net** (749 deletions, 259 insertions), test infrastructure helper added |
| 1F | Eager validation contract at `midaz.New()` | ✅ Done | `582dd99` | 7 files, +271 net, +11 validation tests |
| 1G | Top-level type re-exports (56 type aliases on `midaz.*`) | ✅ Done | `588997a` | 2 files (+279 lines), 59 contract tests added (typed-identity verification + cross-package assignability) |
| 1-Lint | Sweep 11 pre-existing lint issues (3 staticcheck QF1008, 3 testifylint, 5 SA1012 nolint) | ✅ Done | `ab98a64` | 5 files, +12/-7, takes lint baseline from 11 to 0 |

---

## Open Questions

A list of decisions deferred to later phases or pending Fred input. Move to Decision Log once resolved.

### Q1: Module path strategy for v3 — ✅ DECIDED 2026-05-05
**Decision:** `github.com/LerianStudio/midaz-sdk-golang/v3` — clean Go semantic import versioning. Customers update their import path; deprecated APIs are no longer available in v3.
**Decided by:** Fred

### Q2: `c.Entity` deprecation shim duration
**Question:** Should v2.99 keep `c.Entity` as a deprecated field that delegates to top-level services, or is that too much shim code?
**Options:**
- A: Keep `c.Entity` for the full v2.99 lifetime (~30 days)
- B: Remove `c.Entity` immediately in v2.99; force migration before v3.0
**Recommendation:** A. Reduces customer migration friction.
**Status:** Pending Fred decision.

### Q3: AccessManager re-export type strategy
**Question:** `client.AccessManager` should be a type alias (`type AccessManager = auth.AccessManager`) or a wrapping struct?
**Options:**
- A: Type alias — zero-cost, identical to underlying type
- B: Wrapping struct — allows future evolution without breaking clients of `client.AccessManager`
**Recommendation:** A. Aliases are sufficient; we can break v3→v4 if needed.
**Status:** Pending Fred decision.

### Q4: Eager health check default
**Question:** Should `client.New()` perform an eager `GET /health` check by default, or stay lazy?
**Options:**
- A: Lazy by default; opt-in via `client.WithEagerCheck(true)`. Lower init time; failures surface on first call.
- B: Eager by default; opt-out via `client.WithoutEagerCheck()`. Slower init but fail-fast.
**Recommendation:** A. Eager checks add network calls during init that surprise users.
**Status:** Pending Fred decision.

### Q5: How to handle the `transactions.CreateInflow/Outflow/Annotation` family
**Question:** Six Create methods on `TransactionsService` (Create, CreateFromDSL, CreateFromDSLFile, CreateInflow, CreateOutflow, CreateAnnotation). Keep all six?
**Options:**
- A: Keep all six; document the decision tree at the interface top
- B: Collapse into one `Create(ctx, ..., input TransactionInput)` where `TransactionInput` is an interface implemented by 5 concrete types
- C: Three methods: `Create`, `CreateFromDSL`, `CreateSpecial(ctx, ..., kind, input)`
**Recommendation:** A. Each hits a different endpoint; explicit names map to explicit endpoints.
**Status:** Pending Fred decision.

### Q6: Should `client.WithDebug(true)` install a default logger or only affect log level?
**Question:** If user calls `WithDebug(true)` but never `WithLogger(...)`, do we install a default debug-level stderr slog handler, or leave the discard handler and emit nothing?
**Options:**
- A: Install default debug handler — convenient; matches today's "MIDAZ_DEBUG=true → see output" behavior
- B: Leave discard handler — strict; user must explicitly opt in via `WithLogger`
**Recommendation:** A. Maintains today's user expectation; explicit opt-out via `WithoutLogger()` if user really wants silence.
**Status:** Pending Fred decision.

### Q7: Naming of `entities` package in v3 — ✅ DECIDED 2026-05-05
**Decision:** Move all context helpers (`WithIdempotencyKey`, `WithRequestTenantID`, `WithoutAutoIdempotency`, new `WithIncludeDeleted`/`WithHardDelete`) to a new `pkg/sdkctx/` package. **Delete `entities/` entirely.** Service interfaces live directly on `*midaz.Client`. The "entity" abstraction was transitional; v3 retires it.
**Customer-facing migration:**
```go
// v2 (deprecated)
ctx = entities.WithIdempotencyKey(ctx, "key-123")
ctx = entities.WithTenantID(ctx, "acme")

// v3
import "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"

ctx = sdkctx.WithIdempotencyKey(ctx, "key-123")
ctx = sdkctx.WithRequestTenantID(ctx, "acme")
```
**Decided by:** Fred

### Q8: Top-level package import name — ✅ DECIDED 2026-05-05
**Decision:** Rename root package from `client` to `midaz`. Customers write `c := midaz.New(...)` instead of `client.New(...)`. Reads naturally; matches the product name; idiomatic for a domain SDK (cf. `aws.NewSession`, `stripe.NewClient`).
**Customer-facing migration:**
```go
// v2 (deprecated)
import client "github.com/LerianStudio/midaz-sdk-golang/v2"
c, _ := client.New(client.WithConfig(cfg), client.UseAllAPIs())

// v3
import "github.com/LerianStudio/midaz-sdk-golang/v3"
c, _ := midaz.New(midaz.WithAuthToken("..."), midaz.WithEnvironment(midaz.EnvProduction))
```
**Decided by:** Fred

---

## Appendix: Quick Wins Folded Into v3

Per Fred's decision to roll quick wins into v3 (no v2.x patch ship), these surgical fixes become part of the v3 commit history. Listed here for traceability.

| # | Fix | Track | Effort |
|---|-----|-------|--------|
| QW1 | Remove 14 redundant `os.Getenv("MIDAZ_DEBUG")` blocks from entity constructors | 3 | 30 min |
| QW2 | Strip `"custom retryable: "` prefix from `retryableCustomPolicyError.Error()` | 8 | 5 min |
| QW3 | Wrap network errors in `NewNetworkError` at `entities/http.go:1289` | 8 | 30 min |
| QW4 | Plumb `Operation` into `parseErrorResponse` | 8 | 1 hour |
| QW5 | Plumb `ResourceID` from `Get*`/`Update*`/`Delete*` call sites | 8 | 2 hours |
| QW6 | Add `pkg/errors/doc.go` with taxonomy | 8 | 30 min |
| QW7 | Add `examples/README.md` index + per-example READMEs | 9 | 4 hours |
| QW8 | Add `examples/01-hello-world/` | 9 | 1 hour |
| QW9 | Fix broken `docs/tracing.md:191-195` example | 4 | 5 min |
| QW10 | Migrate `entities/mocks/` to `go.uber.org/mock` | 9 | 2 hours |

---

*End of v3 DX Plan. This document is live and updates with every batch of merged work. Open questions block on Fred's input. Status tracker reflects ground truth.*

