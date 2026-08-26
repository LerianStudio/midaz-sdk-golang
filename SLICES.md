# Midaz Go SDK Review Slices

Generated from autonomous multi-phase codebase exploration.

Regenerated after the v4 remodel cutover. The SDK now targets a two-plane
backend: a **Ledger plane** (generated client `internal/genledger`) and a
**Tracer plane** (generated client `internal/gentracer`), both wired through
`entities/plane_clients.go`. Nearly every resource is now exposed through a
hand-written facade (`entities/<domain>_facade.go`) that adapts the SDK-native
`models.*` types onto the oapi-codegen-generated `ClientWithResponses`. Every
resource now routes that way: the last legacy direct implementations (balances,
operations) moved onto the generated client and the alias resource was removed
with the server surface it served.

## Slices

1. Public SDK Construction, Config, Auth
2. Plane Clients + Codegen Internals
3. Shared Transport Runtime Safety
4. Shared Models, Pagination, Validation
5. Organization + Ledger Topology
6. Account Domain, Balances + Instruments
7. Asset + Asset Rates
8. Portfolio + Segment
9. Transaction Core
10. Transaction Helpers
11. Routes + Metadata Indexes
12. CRM Identity + Composition
13. Tracer Plane: Rules, Limits, Validations, Reservations, Audit Events
14. Ledger Protection: Audit + Encryption
15. Fees + Billing
16. Generator, Demo Data, Integrity, Stats
17. Concurrency + Performance Utilities
18. Examples + Documentation Public DX
19. CI, Release, Tooling

## Slice 1: Public SDK Construction, Config, Auth

Review goal: public constructor, option ordering, two-plane bootstrap, auth posture, config/env behavior, root API aliases.

Backend comparison anchor: mostly SDK-internal. Compare service URLs against `../midaz` ledger and tracer components. Plane construction detail lives in Slice 2.

```text
midaz.go
midaz_options.go
types.go
example_test.go
midaz_test.go
midaz_options_test.go
midaz_plane_retry_test.go
types_test.go

pkg/config/config.go
pkg/config/defaults.go
pkg/config/config_test.go
pkg/config/defensive_test.go
pkg/config/example_test.go
pkg/config/config_validation_http_test.go
pkg/config/config_option_validation_regression_test.go
pkg/config/config_readiness_regression_test.go
pkg/config/config_strictness_copying_regression_test.go

pkg/auth/access_manager.go
pkg/auth/access_manager_test.go
pkg/auth/doc.go
pkg/auth/example_test.go
pkg/auth/access_manager_regression_test.go

pkg/version/version.go
pkg/version/version_test.go

internal/reflectutil/reflectutil.go
internal/reflectutil/reflectutil_test.go

entities/access_manager_test.go

docs/auth.md
docs/configuration.md
docs/environment.md
docs/multi-tenancy.md

examples/01-hello-world/main.go
examples/02-auth/main.go
examples/configuration/main.go
examples/internal/quickstart/quickstart.go
```

## Slice 2: Plane Clients + Codegen Internals

Review goal: the two-plane transport wiring (Ledger + Tracer generated clients), oapi-codegen output, spec downgrade tooling, contract drift checks, and backend-parity mapping. This slice is the seam between the SDK and the generated OpenAPI surface.

Backend comparison anchor: `../midaz` — the generated clients are produced from the backend's OpenAPI specs. `scripts/check-midaz-drift.sh` and `.github/workflows/midaz-drift.yml` guard drift; `contract/drift_test.go` (its own module) asserts codegen-vs-model parity.

```text
entities/plane_clients.go
entities/plane_clients_test.go

internal/genledger/ledger.gen.go
internal/genledger/oapi-codegen.yaml
internal/genledger/smoke_test.go
internal/gentracer/tracer.gen.go
internal/gentracer/oapi-codegen.yaml
internal/gentracer/smoke_test.go

internal/cmd/specdowngrade/main.go
internal/cmd/specdowngrade/downgrade_test.go

contract/drift_test.go
contract/go.mod
contract/go.sum

scripts/generate-clients.sh
scripts/check-codegen-drift.sh
scripts/check-midaz-drift.sh
.github/workflows/midaz-drift.yml

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
docs/architecture.md
docs/comprehensive-architecture.md
```

## Slice 3: Shared Transport Runtime Safety

Review goal: HTTP execution, auth header propagation via round-tripper, token refresh, retry round-tripper, idempotency stamping/gating/TTL, error mapping, security validation, observability, logging, request/URL helpers, Entity facade wiring.

Backend comparison anchor: all SDK API calls pass through this slice before reaching the generated plane clients (Slice 2).

```text
entities/entity.go
entities/entity_test.go
entities/entity_facade_wiring_test.go
entities/entity_http_client_safety_regression_test.go
entities/shared_http_client_test.go
entities/constants.go
entities/constants_test.go
entities/internal_context.go
entities/observability.go
entities/business_observability_test.go
entities/example_test.go
entities/http.go
entities/http_test.go
entities/http_diagnostics_test.go
entities/http_tracing_test.go
entities/http_retry_response.go
entities/http_error_fields_regression_test.go
entities/http_error_response_regression_test.go
entities/http_readiness_regression_test.go
entities/transport_classification_test.go
entities/auth_roundtripper.go
entities/auth_roundtripper_test.go
entities/retry_roundtripper.go
entities/retry_roundtripper_test.go
entities/retry_composition_test.go
entities/idempotency.go
entities/idempotency_test.go
entities/idempotency_gate_test.go
entities/idempotency_sibling_test.go
entities/idempotency_ttl_test.go
entities/idempotency_stamp_ledger_test.go
entities/idempotency_stamp_tracer_test.go
entities/idempotency_stamp_transactions_test.go
entities/idempotency_stamp_resources_test.go
entities/http_idempotency_test.go
entities/http_idempotency_precedence_test.go

pkg/retry/doc.go
pkg/retry/retry.go
pkg/retry/retry_test.go
pkg/retry/http.go
pkg/retry/http_test.go
pkg/retry/http_internal_test.go
pkg/retry/example_test.go
pkg/retry/retry_options_copying_regression_test.go

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
pkg/errors/catalog.go
pkg/errors/catalog_test.go
pkg/errors/lifecycle_codes.go
pkg/errors/lifecycle_codes_test.go
pkg/errors/nilcheck.go
pkg/errors/problem_decoder.go
pkg/errors/problem_decoder_test.go
pkg/errors/error_redaction_envelope_regression_test.go

pkg/sdkctx/sdkctx.go
pkg/sdkctx/sdkctx_test.go
pkg/sdkctx/example_test.go

pkg/security/doc.go
pkg/security/http_request.go
pkg/security/http_request_test.go
pkg/security/outbound_request_validation_regression_test.go

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
pkg/observability/observability_middleware_regression_test.go
pkg/observability/observability_readiness_regression_test.go

docs/errors.md
docs/logging.md

examples/06-idempotency/main.go
examples/07-retries/main.go
examples/08-logging-slog/main.go
examples/tracing/main.go
examples/tracing-server/main.go
```

## Slice 4: Shared Models, Pagination, Validation

Review goal: public envelope types, page/cursor list contracts, list-option primitives, validation primitives, field errors, formatting/conversion utilities.

Backend comparison anchor: validate query parameter names and list shapes against `../midaz` ledger and tracer routes.

```text
entities/iter.go
entities/iter_test.go

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
models/example_test.go

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
pkg/validation/validation_contract_regression_test.go

pkg/format/format.go
pkg/format/format_test.go
pkg/format/date.go
pkg/format/date_test.go
pkg/format/example_test.go

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

## Slice 5: Organization + Ledger Topology

Review goal: top-level Midaz hierarchy, org/ledger-scoped facade behavior over the Ledger plane, list/count/update/delete semantics, onboarding model shapes.

Backend comparison anchor: `../midaz` ledger component (onboarding/ledger routes).

```text
entities/organizations_facade.go
entities/organizations_facade_test.go
entities/ledgers_facade.go
entities/ledgers_facade_test.go

models/organization.go
models/organizations_list_opts.go
models/ledger.go
models/ledger_test.go
models/ledgers_list_opts.go
models/onboarding_models_regression_test.go

examples/01-hello-world/main.go
examples/05-listing-pages/main.go
examples/workflow-with-entities/pkg/workflows/organization.go
examples/workflow-with-entities/pkg/workflows/ledger.go
examples/workflow-with-entities/pkg/workflows/workflow.go
examples/workflow-with-entities/pkg/workflows/workflow_test.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 6: Account Domain, Balances + Instruments

Review goal: account types, accounts, account/balance reads and pagination, the account-utility package, and payment instruments (account-scoped). Instruments are a new Ledger-plane domain and reference `models.Account`.

Backend comparison anchor: `../midaz` ledger component. Balances route the Ledger plane via `entities/balances_facade.go`.

```text
entities/account_types_facade.go
entities/account_types_facade_test.go
entities/accounts_facade.go
entities/accounts_facade_test.go
entities/instruments_facade.go
entities/instruments_facade_test.go

models/account_type.go
models/account_types_list_opts.go
models/account.go
models/account_test.go
models/accounts_list_opts.go
models/account_operations_list_opts.go
models/balance.go
models/balances_list_opts.go
models/instrument.go
models/instrument_test.go

pkg/accounts/accounts.go
pkg/accounts/accounts_test.go

examples/workflow-with-entities/pkg/workflows/account_type.go
examples/workflow-with-entities/pkg/workflows/account.go
examples/workflow-with-entities/pkg/workflows/account_list.go
examples/workflow-with-entities/pkg/workflows/list_methods.go
examples/workflow-with-entities/pkg/workflows/get_methods.go
examples/workflow-with-entities/pkg/workflows/delete_methods.go
examples/concurrency/balance-fetch/main.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 7: Asset + Asset Rates

Review goal: asset CRUD, asset-rate facade behavior, cursor list semantics, asset model validation.

Backend comparison anchor: `../midaz` ledger component.

```text
entities/assets_facade.go
entities/assets_facade_test.go
entities/asset_rates_facade.go
entities/asset_rates_facade_test.go

models/asset.go
models/asset_test.go
models/assets_list_opts.go
models/asset_rate.go
models/asset_rate_test.go
models/asset_rates_list_opts.go

examples/workflow-with-entities/pkg/workflows/asset.go
examples/tracing/main.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 8: Portfolio + Segment

Review goal: account grouping/categorization domains and their ledger/account integration.

Backend comparison anchor: `../midaz` ledger component.

```text
entities/portfolios_facade.go
entities/portfolios_facade_test.go
entities/segments_facade.go
entities/segments_facade_test.go

models/portfolio.go
models/portfolio_test.go
models/portfolios_list_opts.go
models/segment.go
models/segments_list_opts.go

examples/workflow-with-entities/pkg/workflows/portfolio.go
examples/workflow-with-entities/pkg/workflows/segment.go
examples/workflow-with-entities/pkg/workflows/list_methods.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 9: Transaction Core

Review goal: canonical transaction creation over the Ledger plane, operation read/update, transaction response parsing, lifecycle endpoints, idempotency-sensitive mutation behavior.

Backend comparison anchor: `../midaz` ledger component. Operations route the Ledger plane via `entities/operations_facade.go`. Idempotency stamping specifics live in Slice 3.

```text
entities/transactions_facade.go
entities/transactions_facade_test.go
entities/transaction_contract_regression_test.go

models/transaction.go
models/transaction_test.go
models/transactions_list_opts.go
models/operation.go
models/operations_list_opts.go

examples/03-end-to-end/main.go
examples/workflow-with-entities/pkg/workflows/transaction.go
examples/workflow-with-entities/pkg/workflows/transaction_concurrent.go
examples/workflow-with-entities/pkg/workflows/transaction_insufficient_funds.go
examples/workflow-with-entities/pkg/entities/transactions.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 10: Transaction Helpers

Review goal: convenience transaction APIs, batch helper retries, settlement, reports, helper idempotency behavior, and the SDK-native transaction-convenience model.

Backend comparison anchor: `../midaz` ledger component, with Slice 3 as context for retry/idempotency.

```text
models/transaction_convenience.go

pkg/transaction/helpers.go
pkg/transaction/helpers_test.go
pkg/transaction/helpers_format_amount_test.go
pkg/transaction/helper_contract_test.go
pkg/transaction/batch.go
pkg/transaction/batch_test.go
pkg/transaction/report.go
pkg/transaction/report_test.go
pkg/transaction/settlement.go
pkg/transaction/settlement_test.go

examples/workflow-with-entities/pkg/workflows/transaction_helpers.go
examples/workflow-with-entities/pkg/workflows/transaction_concurrent.go
examples/workflow-with-entities/pkg/workflows/transaction_insufficient_funds.go
examples/03-end-to-end/main.go

docs/examples.md
```

Context-only for this slice:

```text
entities/transactions_facade.go
models/transaction.go
entities/http.go
pkg/sdkctx/sdkctx.go
pkg/retry/retry.go
```

## Slice 11: Routes + Metadata Indexes

Review goal: operation routes, transaction routes, metadata index settings, route payload/query shape.

Backend comparison anchor: `../midaz` ledger component.

```text
entities/operation_routes_facade.go
entities/operation_routes_facade_test.go
entities/transaction_routes_facade.go
entities/transaction_routes_facade_test.go
entities/metadata_indexes_facade.go
entities/metadata_indexes_facade_test.go

models/operation_route.go
models/operation_routes_list_opts.go
models/transaction_route.go
models/transaction_routes_list_opts.go
models/transaction_route_models_regression_test.go
models/metadata_index.go

examples/workflow-with-entities/pkg/workflows/operation_route.go
examples/workflow-with-entities/pkg/workflows/transaction_route.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 12: CRM Identity + Composition

Review goal: holders, holder-account composition (atomic holder+account creation bridging CRM and Ledger). Holders route the Ledger plane via facade. The alias resource is gone: Midaz renamed it to instruments on /v2 and removed it from /v1, so there was no server surface left for it.

Backend comparison anchor: `../midaz` CRM component. `composition_facade.go` bridges CRM holders and Ledger accounts.

```text
entities/holders_facade.go
entities/holders_facade_test.go
entities/composition_facade.go
entities/composition_facade_test.go

models/holder.go
models/holders_list_opts.go
models/composition.go
models/composition_test.go
models/crm_and_response_models_regression_test.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 13: Tracer Plane: Rules, Limits, Validations, Reservations, Audit Events

Review goal: the Tracer-plane risk/spending-control domain. All facades in this slice route through `gentracer` (verified via `gentracer.` usage). Covers rule scoping, spending limits, transaction validation verdicts (ALLOW/DENY/REVIEW), reservations, and the tracer audit-event stream with hash-chain verification.

Backend comparison anchor: `../midaz` tracer component. `models.Scope` is shared by rules and limits and mirrors `gentracer.Scope` one-for-one.

```text
entities/rules_facade.go
entities/rules_facade_test.go
entities/limits_facade.go
entities/limits_facade_test.go
entities/validations_facade.go
entities/validations_facade_test.go
entities/reservations_facade.go
entities/reservations_facade_test.go
entities/audit_events_facade.go
entities/audit_events_facade_test.go

models/rule.go
models/rule_test.go
models/scope.go
models/scope_test.go
models/limit.go
models/limit_test.go
models/validation.go
models/validation_context.go
models/validation_test.go
models/reservation.go
models/reservation_test.go
models/audit_event.go
models/audit_event_test.go
models/audit_event_record.go
models/audit_event_record_test.go

docs/mapping/external_apis.md
docs/mapping/internal_apis.md
```

## Slice 14: Ledger Protection: Audit + Encryption

Review goal: Ledger-plane data-protection facades — the protection audit reader and encryption provisioning/status. Both route through `genledger`.

Backend comparison anchor: `../midaz` ledger protection routes. Note: `entities/audit_facade.go` (Ledger plane, protection audit) and `entities/audit_events_facade.go` (Tracer plane, Slice 13) both consume `models.AuditEvent` but hit different planes — see Preliminary Review Flags.

```text
entities/audit_facade.go
entities/audit_facade_test.go
entities/encryption_facade.go
entities/encryption_facade_test.go

models/encryption.go
models/encryption_test.go
```

## Slice 15: Fees + Billing

Review goal: fee packages, fee estimation, billing packages, and billing calculation. All four facades route through `genledger` (Ledger plane). Shared builder-validation tests live in `models/builders_5_5_1_test.go`.

Backend comparison anchor: `../midaz` ledger fees/billing routes.

```text
entities/fee_packages_facade.go
entities/fee_packages_facade_test.go
entities/fee_estimate_facade.go
entities/fee_estimate_facade_test.go
entities/billing_packages_facade.go
entities/billing_packages_facade_test.go
entities/billing_calculate_facade.go
entities/billing_calculate_facade_test.go

models/fee_package.go
models/fee_package_test.go
models/fee_packages_list_opts.go
models/fee_estimate.go
models/fee_estimate_test.go
models/billing_package.go
models/billing_package_test.go
models/billing_packages_list_opts.go
models/billing_calculate.go
models/billing_calculate_test.go
models/builders_5_5_1_test.go
```

## Slice 16: Generator, Demo Data, Integrity, Stats

Review goal: demo-data orchestration, remote write generators, DSL conversion, catalogs/templates, post-flight integrity checks, TPS counters, mass demo behavior.

Backend comparison anchor: both `../midaz` ledger routes and generated demo workflows.

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
pkg/generator/dsl_convert.go
pkg/generator/dsl_convert_test.go
pkg/generator/errors.go
pkg/generator/errors_test.go
pkg/generator/generator_contract_regression_test.go
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

## Slice 17: Concurrency + Performance Utilities

Review goal: worker pools, batch execution, circuit breaker, HTTP batch behavior, JSON/client performance helpers, race risk.

Backend comparison anchor: mostly SDK-internal, with remote HTTP batch behavior checked against Midaz request-safety assumptions.

```text
pkg/concurrent/benchmark_test.go
pkg/concurrent/circuit_breaker.go
pkg/concurrent/circuit_breaker_test.go
pkg/concurrent/concurrent.go
pkg/concurrent/concurrent_test.go
pkg/concurrent/example_test.go
pkg/concurrent/helpers.go
pkg/concurrent/helpers_test.go
pkg/concurrent/http_batch.go
pkg/concurrent/http_batch_adapter.go
pkg/concurrent/http_batch_cross_chunk_test.go
pkg/concurrent/http_batch_internal_test.go
pkg/concurrent/http_batch_mutex_regression_test.go
pkg/concurrent/http_batch_redaction_test.go
pkg/concurrent/http_batch_test.go
pkg/concurrent/worker_panic_test.go

pkg/performance/batch.go
pkg/performance/batch_adapter.go
pkg/performance/batch_test.go
pkg/performance/client.go
pkg/performance/client_test.go
pkg/performance/example_test.go
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

## Slice 18: Examples + Documentation Public DX

Review goal: user-facing learning path, docs correctness, examples compile/use canonical public API, docs-to-code drift.

Backend comparison anchor: `docs/mapping/` should align with the SDK surface and `../midaz`.

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
docs/multi-tenancy.md
docs/pagination.md
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
examples/workflow-with-entities/pkg/workflows/workflow_test.go
```

Conditional docs asset:

```text
image/midaz-banner.png
```

## Slice 19: CI, Release, Tooling

Review goal: mechanical repo behavior, build/test/lint/security, release/archive config, env parity, dependency policy, git hooks.

Backend comparison anchor: none, except generated SDK docs/examples must not drift from backend-compatible API surface. Codegen/drift tooling lives in Slice 2.

```text
Makefile
go.mod
go.sum
AGENTS.md
CLAUDE.md
CONTRIBUTING.md
.env.example
.env.local.example
.env.production.example
.gitignore
.golangci.yml
.goreleaser.yml
.releaserc.yml
.releaserc.hotfix.yml

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

Note: `CLAUDE.md` is a symlink to `AGENTS.md`; review the target, not the link.

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
internal/obslogbridge/**
```

Note on generated code: `internal/genledger/ledger.gen.go` (~20k lines) and
`internal/gentracer/tracer.gen.go` (~5k lines) are oapi-codegen output. Review
them for *wiring correctness* (Slice 2), not line-by-line style — they are
regenerated by `scripts/generate-clients.sh`.

## Exploration Summary

Architecture: SDK facade over a two-plane generated backend client. Evidence:
`midaz.Client` embeds `*entities.Entity` at `midaz.go:128` (struct declared at
`midaz.go:115`); the plane clients are constructed before Entity in
`setupEntity` at `midaz.go:419`. `entities.PlaneClients` at
`entities/plane_clients.go:22` holds `Ledger *genledger.ClientWithResponses`
and `Tracer *gentracer.ClientWithResponses`. `entities.Entity` is declared at
`entities/entity.go:144`; the `Config` interface it consumes is at
`entities/entity.go:35`; construction runs through
`NewEntityWithConfigContext` at `entities/entity.go:229`, and services/facades
are wired in `initServices` at `entities/entity.go:325`.

Facade pattern: 26 resources are exposed as `entities/<domain>_facade.go`,
each adapting SDK-native `models.*` types onto the generated
`ClientWithResponses`. Plane routing is unambiguous from `genledger.` /
`gentracer.` usage: 21 facades hit the Ledger plane; 5 hit the Tracer plane
(rules, limits, validations, reservations, audit_events — Slice 13). No
direct-implementation resources remain: balances and operations moved onto the
generated client, the alias resource was removed, and `entities/mocks/` went
with them.

Transport hotspot: `entities/http.go` (1242 lines) remains the blast-radius
center for the low-level HTTP client, but the v4 remodel split cross-cutting
concerns into round-trippers: `entities/auth_roundtripper.go`,
`entities/retry_roundtripper.go`, and idempotency into `entities/idempotency.go`.
Key `http.go` anchors: `NewHTTPClient` at `entities/http.go:165`, `doRequest`
at `entities/http.go:597`, `doRawRequest` at `entities/http.go:665`.

Backend parity anchor: `../midaz` is the source of the OpenAPI specs that
generate `internal/genledger` and `internal/gentracer`. Drift is guarded by
`scripts/check-midaz-drift.sh` + `.github/workflows/midaz-drift.yml` and by
`contract/drift_test.go` (a separate Go module under `contract/`). For contract
review, Ledger-plane slices (5–12, 14, 15) compare against the ledger/CRM
components; Tracer-plane Slice 13 compares against the tracer component.

## Preliminary Review Flags

Observed during this regeneration; seed into later review prompts. (The prior
flag set was verified resolved/stale by an earlier review and is not carried
forward.)

```text
internal/obslogbridge/ is an empty package directory (no .go files) — dead scaffolding. Wire it or delete it; it currently contributes nothing and confuses the module layout.

Two facades named "audit" split across planes: entities/audit_facade.go (Ledger plane, protection audit) and entities/audit_events_facade.go (Tracer plane) both consume models.AuditEvent. Confirm the shared model is intentional and that the two planes' AuditEvent shapes have not silently diverged from a single source.

Legacy un-migrated resources: none remain. Balances and operations route the generated Ledger client like every other resource, the alias resource was removed with its server surface, and the generated mocks in entities/mocks/ went with them.

contract/ ships its own go.mod/go.sum (separate module). contract/drift_test.go will NOT run under the root module's `go test ./...` — verify CI invokes it explicitly (e.g. via scripts/check-codegen-drift.sh) or codegen drift goes unchecked.

models/builders_5_5_1_test.go embeds a release number (5_5_1) in the filename. Version-stamped test files are a naming anti-pattern — they couple a test to a release and mislead future readers. Consider renaming to a domain-descriptive name (it covers fee-estimate, billing-calculate, and holder-account builders).
```
