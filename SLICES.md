# Midaz Go SDK Review Slices

Generated from autonomous multi-phase codebase exploration.

## Slices

1. Public SDK Construction, Config, Auth
2. Shared Transport Runtime Safety
3. Shared Models, Pagination, Validation
4. Organization + Ledger Topology
5. Account Domain + Balances
6. Asset + Asset Rates
7. Portfolio + Segment
8. Transaction Core
9. Transaction Helpers + DSL
10. Routes + Metadata Indexes
11. CRM Identity
12. Generator, Demo Data, Integrity, Stats
13. Concurrency + Performance Utilities
14. Examples + Documentation Public DX
15. CI, Release, Tooling

## Slice 1: Public SDK Construction, Config, Auth

Review goal: public constructor, option ordering, auth posture, config/env behavior, root API aliases.

Backend comparison anchor: mostly SDK-internal. Later compare service URLs against `../midaz/components/ledger` and `../midaz/components/crm`.

```text
midaz.go
midaz_options.go
types.go
client_test.go
client_coverage_test.go
midaz_surface_regression_test.go
types_contract_test.go
validation_contract_test.go
slice2_regression_test.go
slice8_regression_test.go
example_test.go

pkg/config/config.go
pkg/config/config_test.go
pkg/config/defensive_test.go
pkg/config/slice2_regression_test.go
pkg/config/slice8_regression_test.go

pkg/auth/access_manager.go
pkg/auth/access_manager_test.go
pkg/auth/doc.go
pkg/auth/slice2_regression_test.go

pkg/version/version.go
pkg/version/version_test.go

internal/reflectutil/reflectutil.go
internal/reflectutil/reflectutil_test.go

docs/auth.md
docs/configuration.md
docs/environment.md
docs/migration-v2-to-v3.md
docs/multi-tenancy.md

examples/01-hello-world/main.go
examples/02-auth/main.go
examples/configuration/main.go
examples/internal/quickstart/quickstart.go
```

## Slice 2: Shared Transport Runtime Safety

Review goal: HTTP execution, auth header propagation, token refresh, retry, idempotency, error mapping, security validation, observability, logging, request/URL helpers.

Backend comparison anchor: all SDK API calls pass through this slice before reaching `../midaz`.

```text
entities/entity.go
entities/entity_test.go
entities/service.go
entities/shared_http_client_test.go
entities/constants.go
entities/constants_test.go
entities/request.go
entities/url.go
entities/internal_context.go
entities/observability.go
entities/business_observability_test.go
entities/http.go
entities/http_test.go
entities/http_diagnostics_test.go
entities/http_idempotency_test.go
entities/http_idempotency_precedence_test.go
entities/http_tracing_test.go
entities/transport_classification_test.go
entities/contract_http_test.go
entities/access_manager_test.go
entities/slice2_regression_test.go
entities/slice3_regression_test.go
entities/slice4_regression_test.go
entities/slice5_regression_test.go
entities/slice7_regression_test.go

pkg/retry/doc.go
pkg/retry/retry.go
pkg/retry/retry_test.go
pkg/retry/http.go
pkg/retry/http_test.go
pkg/retry/example_test.go
pkg/retry/slice8_regression_test.go

pkg/errors/doc.go
pkg/errors/errors.go
pkg/errors/errors_test.go
pkg/errors/details.go
pkg/errors/transport.go
pkg/errors/transport_test.go
pkg/errors/isbootstrap_test.go
pkg/errors/security_test.go
pkg/errors/example_test.go
pkg/errors/fuzz_test.go
pkg/errors/slice3_regression_test.go

pkg/sdkctx/sdkctx.go
pkg/sdkctx/sdkctx_test.go
pkg/sdkctx/example_test.go

pkg/security/doc.go
pkg/security/http_request.go
pkg/security/http_request_test.go
pkg/security/slice8_regression_test.go

pkg/observability/doc.go
pkg/observability/observability.go
pkg/observability/observability_test.go
pkg/observability/context.go
pkg/observability/http.go
pkg/observability/logging.go
pkg/observability/logging_redaction_test.go
pkg/observability/metrics.go
pkg/observability/middleware_test.go
pkg/observability/sanitize.go
pkg/observability/sanitize_test.go
pkg/observability/simple_test.go
pkg/observability/span.go
pkg/observability/tracing_test.go
pkg/observability/comprehensive_test.go
pkg/observability/slice1_regression_test.go

pkg/performance/json.go
pkg/performance/json_test.go

docs/errors.md
docs/logging.md
docs/tracing.md

examples/06-idempotency/main.go
examples/07-retries/main.go
examples/08-logging-slog/main.go
examples/tracing/main.go
examples/tracing-server/main.go
```

## Slice 3: Shared Models, Pagination, Validation

Review goal: public envelope types, page/cursor list contracts, validation primitives, field errors, formatting/conversion utilities.

Backend comparison anchor: validate query parameter names and list shapes against `../midaz/components/ledger` and CRM routes.

```text
entities/iter.go
entities/iter_test.go
entities/iter_behavior_test.go
entities/list_opts_validation_test.go

models/model.go
models/model_test.go
models/common.go
models/constants.go
models/constants_test.go
models/metrics.go
models/list_opts.go
models/cursor_list_opts.go
models/typed_list_opts_test.go
models/coverage_contract_test.go
models/slice7_regression_test.go

pkg/validation/core/core.go
pkg/validation/core/core_test.go
pkg/validation/core/allowlists.go

pkg/validation/validation.go
pkg/validation/validation_test.go
pkg/validation/helpers.go
pkg/validation/helpers_test.go
pkg/validation/enhanced.go
pkg/validation/enhanced_test.go
pkg/validation/field_error.go
pkg/validation/field_error_test.go
pkg/validation/suggestion.go
pkg/validation/suggestion_test.go
pkg/validation/security_test.go
pkg/validation/example_test.go
pkg/validation/fuzz_test.go
pkg/validation/slice8_regression_test.go

pkg/format/format.go
pkg/format/format_test.go
pkg/format/date.go
pkg/format/date_test.go

pkg/conversion/convert.go
pkg/conversion/convert_test.go
pkg/conversion/conversion_test.go
pkg/conversion/date.go
pkg/conversion/metadata.go
pkg/conversion/model.go
pkg/conversion/model_test.go

pkg/utils/utils.go
pkg/utils/utils_test.go

docs/pagination.md

examples/04-listing-cursor/main.go
examples/05-listing-pages/main.go
examples/pkg-validation-demo/main.go
```

## Slice 4: Organization + Ledger Topology

Review goal: top-level Midaz hierarchy, org/ledger-scoped URL construction, list/count/update/delete behavior.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/organizations.go
entities/organizations_test.go
entities/ledgers.go
entities/ledgers_test.go
entities/mocks/mock_organizations.go
entities/mocks/mock_ledgers.go

models/organization.go
models/organizations_list_opts.go
models/ledger.go
models/ledger_test.go
models/ledgers_list_opts.go
models/slice4_regression_test.go

examples/01-hello-world/main.go
examples/05-listing-pages/main.go
examples/workflow-with-entities/pkg/workflows/organization.go
examples/workflow-with-entities/pkg/workflows/ledger.go
examples/workflow-with-entities/pkg/workflows/workflow.go
examples/workflow-with-entities/pkg/entities/organizations.go
examples/workflow-with-entities/pkg/entities/ledgers.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 5: Account Domain + Balances

Review goal: account types, accounts, account aliases, account filters, balance reads, balance pagination, account utility package.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/account_types.go
entities/account_types_test.go
entities/accounts.go
entities/accounts_test.go
entities/balances.go
entities/balances_test.go
entities/mocks/mock_account_types.go
entities/mocks/mock_accounts.go
entities/mocks/mock_balances.go

models/account_type.go
models/account_types_list_opts.go
models/account.go
models/account_test.go
models/accounts_list_opts.go
models/balance.go
models/balances_list_opts.go
models/slice4_regression_test.go

pkg/accounts/accounts.go
pkg/accounts/accounts_test.go

examples/workflow-with-entities/pkg/workflows/account_type.go
examples/workflow-with-entities/pkg/workflows/account.go
examples/workflow-with-entities/pkg/workflows/account_list.go
examples/workflow-with-entities/pkg/workflows/list_methods.go
examples/workflow-with-entities/pkg/workflows/delete_methods.go
examples/workflow-with-entities/pkg/entities/accounts.go
examples/concurrency/balance-fetch/main.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 6: Asset + Asset Rates

Review goal: asset CRUD, asset-rate transaction-service behavior, cursor list semantics, asset model validation.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/assets.go
entities/assets_test.go
entities/asset_rates.go
entities/asset_rates_test.go
entities/mocks/mock_assets.go
entities/mocks/mock_asset_rates.go

models/asset.go
models/asset_test.go
models/assets_list_opts.go
models/asset_rate.go
models/asset_rate_test.go
models/asset_rates_list_opts.go

examples/workflow-with-entities/pkg/workflows/asset.go
examples/workflow-with-entities/pkg/entities/assets.go
examples/tracing/main.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 7: Portfolio + Segment

Review goal: account grouping/categorization domains and their ledger/account integration.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/portfolios.go
entities/portfolios_test.go
entities/segments.go
entities/segments_test.go
entities/mocks/mock_portfolios.go
entities/mocks/mock_segments.go

models/portfolio.go
models/portfolio_test.go
models/portfolios_list_opts.go
models/segment.go
models/segments_list_opts.go
models/slice4_regression_test.go

examples/workflow-with-entities/pkg/workflows/portfolio.go
examples/workflow-with-entities/pkg/workflows/segment.go
examples/workflow-with-entities/pkg/workflows/list_methods.go
examples/workflow-with-entities/pkg/entities/portfolios.go
examples/workflow-with-entities/pkg/entities/segments.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 8: Transaction Core

Review goal: canonical transaction creation, operation read/update, transaction response parsing, lifecycle endpoints, idempotency-sensitive mutation behavior.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/transactions.go
entities/transactions_test.go
entities/transactions_http_test.go
entities/operations.go
entities/operations_test.go
entities/mocks/mock_transactions.go
entities/mocks/mock_operations.go
entities/contract_http_test.go
entities/slice5_regression_test.go
entities/slice7_regression_test.go

models/transaction.go
models/transaction_test.go
models/transactions_list_opts.go
models/operation.go
models/operations_list_opts.go
models/slice7_regression_test.go

examples/03-end-to-end/main.go
examples/workflow-with-entities/pkg/workflows/transaction.go
examples/workflow-with-entities/pkg/workflows/transaction_concurrent.go
examples/workflow-with-entities/pkg/workflows/transaction_insufficient_funds.go
examples/workflow-with-entities/pkg/entities/transactions.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 9: Transaction Helpers + DSL

Review goal: convenience transaction APIs, DSL rendering, batch helper retries, reports, helper idempotency behavior.

Backend comparison anchor: `../midaz/components/ledger`, with Slice 2 as context for retry/idempotency.

```text
models/transaction_dsl.go

pkg/transaction/helpers.go
pkg/transaction/helpers_test.go
pkg/transaction/helpers_format_amount_test.go
pkg/transaction/helper_contract_test.go
pkg/transaction/batch.go
pkg/transaction/batch_test.go
pkg/transaction/report.go
pkg/transaction/report_test.go

examples/workflow-with-entities/pkg/workflows/transaction_helpers.go
examples/workflow-with-entities/pkg/workflows/transaction_concurrent.go
examples/workflow-with-entities/pkg/workflows/transaction_insufficient_funds.go
examples/03-end-to-end/main.go

docs/examples.md
docs/migration-v2-to-v3.md
```

Context-only for this slice:

```text
entities/transactions.go
models/transaction.go
entities/http.go
pkg/sdkctx/sdkctx.go
pkg/retry/retry.go
```

## Slice 10: Routes + Metadata Indexes

Review goal: operation routes, transaction routes, metadata index settings, route payload/query shape.

Backend comparison anchor: `../midaz/components/ledger`.

```text
entities/operation_routes.go
entities/transaction_routes.go
entities/metadata_indexes.go
entities/metadata_indexes_test.go
entities/mockgen_smoke_test.go
entities/mocks/mock_operation_routes.go
entities/mocks/mock_transaction_routes.go
entities/mocks/mock_metadata_indexes.go

models/operation_route.go
models/operation_routes_list_opts.go
models/transaction_route.go
models/transaction_routes_list_opts.go
models/metadata_index.go
models/slice5_regression_test.go

examples/workflow-with-entities/pkg/workflows/operation_route.go
examples/workflow-with-entities/pkg/workflows/transaction_route.go
examples/workflow-with-entities/pkg/entities/operation_routes.go
examples/workflow-with-entities/pkg/entities/transaction_routes.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 11: CRM Identity

Review goal: holders, aliases, CRM base URL, `X-Organization-Id`, holder-scoped alias paths, CRM-to-ledger account link semantics.

Backend comparison anchor: `../midaz/components/crm`.

```text
entities/crm_shared.go
entities/holders.go
entities/holders_test.go
entities/aliases.go
entities/aliases_test.go
entities/slice6_regression_test.go
entities/mocks/mock_holders.go
entities/mocks/mock_aliases.go

models/holder.go
models/holders_list_opts.go
models/alias.go
models/aliases_list_opts.go
models/crm_test.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

Context-only for CRM integration:

```text
midaz_options.go
pkg/config/config.go
entities/entity.go
entities/accounts.go
models/accounts_list_opts.go
examples/workflow-with-entities/pkg/entities/accounts.go
examples/workflow-with-entities/pkg/workflows/operation_route.go
```

## Slice 12: Generator, Demo Data, Integrity, Stats

Review goal: demo-data orchestration, remote write generators, catalogs/templates, post-flight checks, TPS counters, mass demo behavior.

Backend comparison anchor: both `../midaz/components/ledger` and generated demo workflows.

```text
pkg/generator/account_generator.go
pkg/generator/account_generator_test.go
pkg/generator/account_hierarchy.go
pkg/generator/account_hierarchy_test.go
pkg/generator/account_type_generator.go
pkg/generator/account_type_generator_test.go
pkg/generator/asset_generator.go
pkg/generator/asset_generator_test.go
pkg/generator/circuit.go
pkg/generator/circuit_test.go
pkg/generator/config.go
pkg/generator/config_test.go
pkg/generator/constants.go
pkg/generator/doc.go
pkg/generator/errors.go
pkg/generator/errors_test.go
pkg/generator/interfaces.go
pkg/generator/ledger_generator.go
pkg/generator/ledger_generator_test.go
pkg/generator/operation_routes_generator.go
pkg/generator/operation_routes_generator_test.go
pkg/generator/options.go
pkg/generator/options_test.go
pkg/generator/org_generator.go
pkg/generator/org_generator_test.go
pkg/generator/portfolio_generator.go
pkg/generator/portfolio_generator_test.go
pkg/generator/segment_generator.go
pkg/generator/segment_generator_test.go
pkg/generator/slice9_regression_test.go
pkg/generator/transaction_generator.go
pkg/generator/transaction_generator_test.go
pkg/generator/transaction_lifecycle.go
pkg/generator/transaction_routes_generator.go
pkg/generator/transaction_routes_generator_test.go

pkg/data/accounts.go
pkg/data/accounts_test.go
pkg/data/amounts.go
pkg/data/amounts_test.go
pkg/data/assets.go
pkg/data/assets_test.go
pkg/data/doc.go
pkg/data/organizations.go
pkg/data/organizations_test.go
pkg/data/templates.go
pkg/data/templates_test.go
pkg/data/transactions.go
pkg/data/transactions_test.go
pkg/data/validate.go
pkg/data/validate_test.go

pkg/integrity/checker.go
pkg/integrity/checker_test.go
pkg/integrity/doc.go

pkg/stats/doc.go
pkg/stats/stats.go
pkg/stats/stats_test.go

examples/mass-demo-generator/main.go
examples/mass-demo-generator/demo_helpers.go
examples/mass-demo-generator/demo_helpers_test.go
examples/mass-demo-generator/README.md
examples/mass-demo-generator/default.yaml

docs/examples.md
docs/comprehensive-architecture.md
docs/mapping/external_apis.md
```

Excluded from this slice:

```text
examples/mass-demo-generator/.env
examples/mass-demo-generator/mass-demo-generator
```

## Slice 13: Concurrency + Performance Utilities

Review goal: worker pools, batch execution, circuit breaker, HTTP batch behavior, JSON/client performance helpers, race risk.

Backend comparison anchor: mostly SDK-internal, with remote HTTP batch behavior checked against Midaz request safety assumptions.

```text
pkg/concurrent/benchmark_test.go
pkg/concurrent/circuit_breaker.go
pkg/concurrent/circuit_breaker_test.go
pkg/concurrent/concurrent.go
pkg/concurrent/concurrent_test.go
pkg/concurrent/helpers.go
pkg/concurrent/helpers_test.go
pkg/concurrent/http_batch.go
pkg/concurrent/http_batch_adapter.go
pkg/concurrent/http_batch_redaction_test.go
pkg/concurrent/http_batch_test.go

pkg/performance/batch.go
pkg/performance/batch_adapter.go
pkg/performance/batch_test.go
pkg/performance/client.go
pkg/performance/client_test.go
pkg/performance/http.go
pkg/performance/http_test.go
pkg/performance/json.go
pkg/performance/json_test.go
pkg/performance/performance.go
pkg/performance/performance_test.go

examples/concurrency/main.go
examples/concurrency/README.md
examples/concurrency/balance-fetch/main.go

docs/examples.md
docs/comprehensive-architecture.md
```

## Slice 14: Examples + Documentation Public DX

Review goal: user-facing learning path, docs correctness, examples compile/use canonical public API, docs-to-code drift.

Backend comparison anchor: docs/mapping should align with SDK and `../midaz`.

```text
README.md
docs/README.md
docs/architecture.md
docs/auth.md
docs/comprehensive-architecture.md
docs/configuration.md
docs/environment.md
docs/errors.md
docs/examples.md
docs/logging.md
docs/migration-v2-to-v3.md
docs/multi-tenancy.md
docs/pagination.md
docs/tracing-implementation.md
docs/tracing.md
docs/v3-dx-plan.md
docs/mapping/external_apis.md
docs/mapping/internal_apis.md

examples/README.md
examples/01-hello-world/README.md
examples/01-hello-world/main.go
examples/02-auth/README.md
examples/02-auth/main.go
examples/03-end-to-end/README.md
examples/03-end-to-end/main.go
examples/04-listing-cursor/README.md
examples/04-listing-cursor/main.go
examples/05-listing-pages/README.md
examples/05-listing-pages/main.go
examples/06-idempotency/README.md
examples/06-idempotency/main.go
examples/07-retries/README.md
examples/07-retries/main.go
examples/08-logging-slog/README.md
examples/08-logging-slog/main.go
examples/09-testing-with-mocks/README.md
examples/09-testing-with-mocks/reporter.go
examples/09-testing-with-mocks/reporter_test.go
examples/10-observability-otel/README.md
examples/10-observability-otel/observability_demo.go
examples/concurrency/README.md
examples/concurrency/main.go
examples/concurrency/balance-fetch/main.go
examples/configuration/README.md
examples/configuration/main.go
examples/context/README.md
examples/context/main.go
examples/internal/quickstart/quickstart.go
examples/mass-demo-generator/README.md
examples/mass-demo-generator/demo_helpers.go
examples/mass-demo-generator/demo_helpers_test.go
examples/mass-demo-generator/main.go
examples/pkg-validation-demo/README.md
examples/pkg-validation-demo/main.go
examples/tracing/README.md
examples/tracing/main.go
examples/tracing-server/README.md
examples/tracing-server/main.go
examples/workflow-with-entities/README.md
examples/workflow-with-entities/main.go
examples/workflow-with-entities/pkg/entities/accounts.go
examples/workflow-with-entities/pkg/entities/assets.go
examples/workflow-with-entities/pkg/entities/ledgers.go
examples/workflow-with-entities/pkg/entities/operation_routes.go
examples/workflow-with-entities/pkg/entities/organizations.go
examples/workflow-with-entities/pkg/entities/portfolios.go
examples/workflow-with-entities/pkg/entities/segments.go
examples/workflow-with-entities/pkg/entities/transaction_routes.go
examples/workflow-with-entities/pkg/entities/transactions.go
examples/workflow-with-entities/pkg/workflows/account.go
examples/workflow-with-entities/pkg/workflows/account_list.go
examples/workflow-with-entities/pkg/workflows/account_type.go
examples/workflow-with-entities/pkg/workflows/asset.go
examples/workflow-with-entities/pkg/workflows/common.go
examples/workflow-with-entities/pkg/workflows/delete_methods.go
examples/workflow-with-entities/pkg/workflows/get_methods.go
examples/workflow-with-entities/pkg/workflows/ledger.go
examples/workflow-with-entities/pkg/workflows/list_methods.go
examples/workflow-with-entities/pkg/workflows/operation_route.go
examples/workflow-with-entities/pkg/workflows/organization.go
examples/workflow-with-entities/pkg/workflows/portfolio.go
examples/workflow-with-entities/pkg/workflows/segment.go
examples/workflow-with-entities/pkg/workflows/transaction.go
examples/workflow-with-entities/pkg/workflows/transaction_concurrent.go
examples/workflow-with-entities/pkg/workflows/transaction_helpers.go
examples/workflow-with-entities/pkg/workflows/transaction_insufficient_funds.go
examples/workflow-with-entities/pkg/workflows/transaction_route.go
examples/workflow-with-entities/pkg/workflows/workflow.go
```

Conditional docs asset:

```text
image/midaz-banner.png
```

## Slice 15: CI, Release, Tooling

Review goal: mechanical repo behavior, build/test/lint/security, release/archive config, env parity, dependency policy.

Backend comparison anchor: none, except generated SDK docs/examples must not drift from backend-compatible API surface.

```text
Makefile
go.mod
go.sum
.env.example
.gitignore
.golangci.yml
.goreleaser.yml
.releaserc.yml
.releaserc.hotfix.yml
AGENTS.md
CLAUDE.md
CONTRIBUTING.md

.github/dependabot.yml
.github/workflows/check-branch.yml
.github/workflows/go-combined-analysis.yml
.github/workflows/release.yml

scripts/check-config-parity.sh
scripts/install-hooks.sh
scripts/run_tests.sh
scripts/utils/ascii.sh
scripts/utils/colors.sh
```

## Generated Or Local Artifacts To Exclude

These should not be fed to review agents unless the task is specifically about generated docs/artifacts.

```text
docs/godoc/**
docs/codereview/**
docs/plans/**
artifacts/**
.ruff_cache/**
examples/mass-demo-generator/.env
examples/workflow-with-entities/.env
examples/mass-demo-generator/mass-demo-generator
03-end-to-end
06-idempotency
09-testing-with-mocks
```

## Exploration Summary

Architecture: SDK facade over domain service interfaces with one shared transport adapter. Evidence: `midaz.Client` embeds `*entities.Entity` at `midaz.go:100-126`; `Entity` exposes service fields at `entities/entity.go:37-65`; services are initialized at `entities/entity.go:172-210`.

Organization: hybrid. Top level is layered (`midaz`, `entities`, `models`, `pkg`, `examples`, `docs`), but the actual reviewable implementation should be sliced by resource/domain because the risk lives in `entities/<domain>.go` plus matching `models/<domain>.go` plus list opts/tests.

Transport hotspot: `entities/http.go` is the blast-radius center. It owns request construction, auth, retry, idempotency, observability, response decoding, and error conversion. Key anchors: `entities/http.go:530`, `entities/http.go:598`, `entities/http.go:711`, `entities/http.go:2016`.

Backend parity anchor: `../midaz` exists and has `components/ledger` and `components/crm`. For contract review, slices 4 through 10 should be reviewed against `../midaz/components/ledger`; slice 11 against `../midaz/components/crm`.

## Preliminary Review Flags

These came up during exploration and are worth seeding into later review prompts:

```text
pkg/config/config.go:1166-1173 may drop Access Manager AllowInsecureHTTP when building plugin auth config for entity bootstrap.
.goreleaser.yml references client.go, but the root SDK entry file is midaz.go.
CONTRIBUTING.md appears stale on Go version/path compared with go.mod and CI.
docs/mapping/external_apis.md has likely CRM includeDeleted/hardDelete drift versus sdkctx-based implementation.
docs/mapping/external_apis.md has likely AssetRate list option naming drift.
pkg/transaction helper defaults may reuse idempotency keys if option structs are reused.
pkg/concurrent/http_batch.go has mutable defaultHeaders behavior worth checking for races.
```
