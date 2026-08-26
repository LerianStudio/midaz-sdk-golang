# Midaz-sdk-golang Changelog

## [5.0.0](https://github.com/LerianStudio/midaz-sdk-golang/releases/tag/v5.0.0)

Features:
- Promote the `develop` branch to `main`. (@fredcamaral)
- Serve both `midaz` `v1` and `v2` surfaces. (@fredcamaral)
- Exercise the `v2` surface in the mass demo generator. (@fredcamaral)
- Serve the complete `v2` ledger surface. (@fredcamaral)
- Add the `v2` transaction contract, separate from `v1`. (@fredcamaral)
- Expose the three generated `v1` endpoints that had no facade. (@fredcamaral)
- Route balances and operations over the generated client. (@fredcamaral)
- Group the ledger accessors by server version. (@fredcamaral)
- Remove the alias service, dead at every server version. (@fredcamaral)
- Add retry `RoundTripper` on plane path (money-path, no-double-charge). (@fredcamaral)
- Thread effective retry policy into plane clients (fix ordering hazard). (@fredcamaral)
- Add tracer audit-events facade (list + get + verify). (@fredcamaral)
- Add tracer reservations facade (reserve + lifecycle). (@fredcamaral)
- Add tracer validations facade + shared context models. (@fredcamaral)
- Add tracer limits facade + models (money-path + immutable-field probe). (@fredcamaral)
- Add tracer rules facade + models (flat-envelope pattern). (@fredcamaral)
- Add protection audit facade + models (cursor list). (@fredcamaral)
- Add encryption facade (provision + status). (@fredcamaral)
- Add billing-calculate models + genledger-backed facade. (@fredcamaral)
- Add billing-packages models + genledger-backed facade. (@fredcamaral)
- Add fee-estimate models + genledger-backed facade. (@fredcamaral)
- Add fee-packages models + genledger-backed facade. (@fredcamaral)
- Add genledger-backed composition facade. (@fredcamaral)
- Add genledger-backed instruments facade. (@fredcamaral)
- Add genledger-backed holders facade. (@fredcamaral)
- Add instrument and holder-account composition models. (@fredcamaral)
- Add readCount helper, harden transactions Count error path, add onboarding counts. (@fredcamaral)
- Add asset-rates facade over generated ledger client. (@fredcamaral)
- Add transaction-routes facade over generated ledger client. (@fredcamaral)
- Add operation-routes facade over generated ledger client. (@fredcamaral)
- Add UpdateTransaction and UpdateOperation to transactions facade. (@fredcamaral)
- Add read path (get/list-cursor/count) to facade. (@fredcamaral)
- Add commit/cancel/revert lifecycle to facade. (@fredcamaral)
- Facade for the four create paths (money-write). (@fredcamaral)
- Idempotency resolver for generated write path. (@fredcamaral)
- Expose tri-block ledger settings on facade. (@fredcamaral)
- Add MetadataIndexes facade (global, non-paginated). (@fredcamaral)
- Add Accounts facade (CRUD, alias, cursor sub-lists). (@fredcamaral)
- Add AccountTypes facade (full CRUD). (@fredcamaral)
- Add Ledgers/Assets/Portfolios/Segments facades (full CRUD). (@fredcamaral)
- Extend Organizations facade to full CRUD (write-exemplar). (@fredcamaral)
- Add Organizations List facade over generated ledger client. (@fredcamaral)
- Build two-plane clients over generated packages with auth `RoundTripper`. (@fredcamaral)
- Generate committed ledger + tracer OpenAPI clients. (@fredcamaral)
- Add Go-native OAS `3.1` to `3.0.3` spec downgrade tool. (@fredcamaral)

Fixes:
- Upgrade codegen chain, `lib-observability` `v3`, workflows. (@fredcamaral)
- Credit builder delegation only within the same operation. (@fredcamaral)
- Close the last silent-narrowing path in the builder-method scan. (@fredcamaral)
- Make the count window span exactly `DEMO_COUNT_DAYS` dates. (@fredcamaral)
- Refuse an unrunnable `v2` phase and unpin the count window. (@fredcamaral)
- Make every input boundary check the value it sends. (@fredcamaral)
- Align write payloads and account listing with server. (@fredcamaral)
- Refuse a nil response in the count decode too. (@fredcamaral)
- Refuse a nil response instead of dereferencing it. (@fredcamaral)
- Refuse a whitespace-only transaction list filter. (@fredcamaral)
- Refuse the six dead transaction list filters on `v1` too. (@fredcamaral)
- Refuse an empty `2xx` list body on the `v1` surface too. (@fredcamaral)
- Refuse a percent sign in a path id. (@fredcamaral)
- Refuse a `2xx` list body that carries no page. (@fredcamaral)
- Refuse `v2` transaction list filters the ledger cannot express. (@fredcamaral)
- Refuse a `2xx` that carries no resource. (@fredcamaral)
- Report a bodiless `204` delete as the success it is. (@fredcamaral)
- Reject dot-segment and separator path ids before the request leaves the SDK. (@fredcamaral)
- Reject empty path ids before the request leaves the SDK. (@fredcamaral)
- Reconcile the path and body ledger on package creates. (@fredcamaral)
- Surface the upstream provider error from problem bodies. (@fredcamaral)
- Adopt the server spec and send the live asset wire field. (@fredcamaral)
- Reconcile the path and body ledger on billing and fee estimates. (@fredcamaral)
- Reject a version suffix on the ledger base URL. (@fredcamaral)
- Version the hand-rolled balance, operation and alias paths. (@fredcamaral)
- Stop a decode failure answering to "internal error". (@fredcamaral)
- Tell the truth about a response we could not decode. (@fredcamaral)
- Address transaction legs by account alias, not account ID. (@fredcamaral)
- Reject metadata that departs from canonical serialization. (@fredcamaral)
- Send transaction legs with a trimmed account alias. (@fredcamaral)
- Validate inner fees on create and align Fee json tags with server. (@fredcamaral)
- Validations date filter must be `RFC3339` (matches tracer server). (@fredcamaral)
- Reject unsupported date filter on rules/limits cursor list. (@fredcamaral)
- Stop cursor sub-lists on empty `next_cursor`. (@fredcamaral)
- Thread `X-Request-ID` and propagate `IncludeDeleted`. (@fredcamaral)
- Classify prefixed idempotency code `0084`. (@fredcamaral)
- Bridge OAS `3.1` `contentEncoding:base64` to `format:byte`. (@fredcamaral)
- Fail fast in drift check instead of reporting false "no drift". (@bedatty)

Improvements:
- Bump `lib-observability` to `v3.2.0`. (@fredcamaral)
- Bump checkout, setup-go and import-gpg to `v7` and pin midaz-drift by SHA. (@fredcamaral)
- Bump the shared workflows to `v1.58.0`. (@fredcamaral)
- Adopt `lib-observability` `v3` and refresh the direct dependency set. (@fredcamaral)
- Upgrade the openapi codegen chain and drop its CVE suppressions. (@fredcamaral)
- Align architecture, pagination and example docs with the shipped surface. (@fredcamaral)
- Record PR `#241` CI rounds and CodeRabbit triage. (@fredcamaral)
- Record phase C — generator as live proof, instruments defects fixed. (@fredcamaral)
- Record the live integration run against midaz develop. (@fredcamaral)
- Close epic 4 after the orchestrator final review. (@fredcamaral)
- Record the epic 4 phase B fix round 2. (@fredcamaral)
- Record the epic 4 phase B coverage fix round. (@fredcamaral)
- Record epic 4 phase B, and the coverage gate it did not cause. (@fredcamaral)
- Tell the `v5` story in the guides, and retire a superseded draft plan. (@fredcamaral)
- Reconcile the API maps with the dual-version surface. (@fredcamaral)
- Move the examples onto the `v2` surface and fix two runtime defects. (@fredcamaral)
- Close epic 4 phase A after the orchestrator final review. (@fredcamaral)
- Record epic 4 phase A fix round 8. (@fredcamaral)
- Record epic 4 phase A fix round 7. (@fredcamaral)
- Record the third shadowing family, the parenthesised callee, and repoint three deleted cross-references. (@fredcamaral

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v4.1.0...v5.0.0)

---

## [4.1.0](https://github.com/LerianStudio/midaz-sdk-golang/releases/tag/v4.1.0)

- **Features:**
  - Added functionality to track midaz contract drift via a pinned baseline and nightly check.

- **Fixes:**
  - Exposed `AccountingEntries.overdraft` and `Balance.direction` in models.
  - Hardened midaz-drift workflow following CodeRabbit review.

Contributors: @fredcamaral,

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v4.0.1...v4.1.0)

---

## [Unreleased]

### ⚠️ Breaking Changes (v4 remodel)

The v4 remodel repoints the ledger accessors onto concrete facades over the generated plane clients and removes the legacy DSL surface.

**Removed (breaking):**
- The 13 legacy ledger `XService` interfaces and their generated mocks.
- `models.TransactionDSLInput` and its DSL machinery (`FromTransactionMap`, and the DSL create paths `CreateTransactionWithDSL` / `CreateTransactionWithDSLFile`).
- Five exported DSL validators from `pkg/validation`: `ValidateTransactionDSL`, `EnhancedValidateTransactionDSL`, `TransactionDSLValidator`, the `AccountReference` interface, and `EnhancedValidateAccountReference`.
- The legacy transaction-create surface on `models.CreateTransactionInput`: the `ExternalID`, `Amount`, `AssetCode`, and `Operations` fields, plus the `WithExternalID` and `WithOperations` builders. None of them reached the ledger: `ExternalID` was `json:"-"` and gave no deduplication, while `Amount` / `AssetCode` / `Operations` only fed an internal adapter that no-opped whenever `Send` was set — so a caller filling them believed in a contract the SDK dropped. Migration: describe the money movement in `Send` only (`Send.Asset` replaces `AssetCode`, `Send.Value` replaces `Amount`, `Send.Source.From` / `Send.Distribute.To` replace `Operations`); use `IdempotencyKey` or `sdkctx.WithIdempotencyKey` for duplicate protection, and `Metadata` for durable correlation. A create with a nil `Send` now fails validation with `send is required; legacy Operations input was removed in v4.2`. The same field is gone from the `pkg/transaction` helper options (`TransferOptions`, `DepositOptions`, `WithdrawalOptions`, `MultiTransferOptions`), which had no other place to send it.
- `models.CreateTransactionInput.Template`. It carried `json:"template,omitempty"` but `ToLibTransaction` never emitted it, so every value set on it was silently discarded — the same defect as the four fields above. Nothing in the SDK read it. Migration: none; the ledger create endpoints have no template input.
- `models.CreateOperationInput` and its surface (`NewCreateOperationInput`, `WithAccountAlias`, `WithRoute`, `Validate`), plus the `midaz.CreateOperationInput` re-export. The type only ever fed `CreateTransactionInput.Operations`, removed above; the ledger has no operation-create endpoint, so nothing could send one. Migration: describe every leg in `Send.Source.From` / `Send.Distribute.To` with `models.FromToInput`.

**Changed (breaking):**
- `json.Marshal` of a transaction create input now returns the endpoint's request body, not the struct-tag shape. `CreateTransactionInput`, `CreateInflowInput`, `CreateOutflowInput`, `CreateAnnotationInput`, `SendInput`, `SourceInput`, `DistributeInput`, `FromToInput` and `AmountInput` implement `MarshalJSON` delegating to their mapper (`ToLibTransaction` / `ToMap`), which is what the facade puts on the wire. Observable differences for anyone who marshals an input for a log, an audit row, an outbox record or a payload fingerprint: money values render as decimal strings (an `any` holding `100` becomes `"100"`, not `100`), a leg carrying `share` / `remaining` / `rate` omits `amount` entirely (`AmountInput{}` marshals to `null`), a nil `Source` / `Distribute` is omitted instead of emitted as `null`, and fields the mapper never emitted stop appearing. A stored document or hash taken from a marshaled input therefore changes with this version. There is no matching `UnmarshalJSON`: unmarshaling still reads struct tags, so a marshal→unmarshal round trip of a create input is now lossy — marshal for transport and inspection, not as a persistence format for the input itself. `entities/transactions_facade_wire_test.go` pins each endpoint's body against a hand-written golden.
- `client.X` accessors now return concrete `*xFacade` structs exposing generic CRUD method names (`List` / `Get` / `Create` / `Update` / `Delete` / `All` / `Pages` / `Count`) instead of the old verbose `XService` methods — e.g. `ListAccounts` → `List`, `CreateTransaction` → `CreateJSON`.
- `route` and `routeId` are now mutually exclusive on every transaction create. Both fields serialize whenever set, so a payload carrying the pair left the ledger to pick which routing decision applied. Validation now rejects it at the transaction level (`transaction-level route and routeId are mutually exclusive; keep routeId`, enforced for `CreateJSON`, inflow, outflow, and annotation) and on each leg (`leg accountAlias=<alias>: route and routeId are mutually exclusive; keep routeId`). Migration: send `RouteID` (UUID) and drop `Route`; `Route` remains supported alone for server-side alias compatibility.
- `models.FromToInput` now carries a single account identity: `AccountAlias`. The `Account` field is removed. On the transaction creates it never reached the wire — the leg mapper copied it into `accountAlias` whenever `AccountAlias` was empty, so a caller setting `Account` to an account ID had it silently reinterpreted as an alias. Migration: rename `Account:` to `AccountAlias:` in every source and destination leg; a leg with an empty `AccountAlias` now fails validation with `accountAlias is required`.
- `models.FeeAdjustedFromTo.Account` (`json:"account"`) is now `AccountAlias` (`json:"accountAlias"`). The fee engine projects the same leg DTO on the way out as on the way in (`pkg/mtransaction.FromTo`, which has only `accountAlias`), so the old field never decoded and every fee-adjusted leg read back with an empty account identity. Migration: read `AccountAlias`.
- The fee-estimate request body (`POST /organizations/{id}/estimates`, `FeeEstimates.EstimateFee`) changed shape, because `models.FeeEstimateInput` reuses the transaction leg types: legs now ship `accountAlias` instead of `account`, `send.value` and every `amount.value` ship as decimal strings instead of raw JSON numbers, an absent `source`/`distribute` is omitted instead of serialized as `null`, and a share/remaining/rate leg no longer ships an empty `amount` object. This aligns the request with the DTO the fee engine unmarshals (`components/ledger/pkg/feeshared/model.FeeEstimate` embeds `pkg/mtransaction.Transaction`, whose `FromTo` exposes only `accountAlias`), so an estimate that previously reached the engine with no account identity on any leg now resolves the accounts it quotes. `entities/fee_estimate_facade_test.go` pins the full body.

### ✨ Added (v4 remodel)
- `models/correlation` — the versioned, closed contract for the correlation metadata a transactional plugin attaches to a ledger transaction. `correlation.Correlation` carries only identifiers and classification (plugin, rail, flow, aggregate id, end-to-end id, provider message id/code, original aggregate id, direction), `Validate` enforces the enums and the rule that a refund names the aggregate it returns, and `ToMetadata` emits the whitelist under camelCase keys with `contractVersion` (currently `"1"`). The rails are `RailTED`, `RailPix` and `RailInternal` — the last one for a book transfer between accounts of the same institution, which settles on no external rail and would otherwise have to borrow `TED` or `PIX` to satisfy the required `Rail` field. `FromMetadata` reads a payload back, and `Keys` exposes the whitelist, so no consumer needs a second copy of the key set. Arbitrary client metadata is never forwarded: extending the whitelist is a versioned change to this package, which is what keeps counterparty PII out of the ledger by construction.
- `models/correlation/correlationtest` — `AssertCanonical(tb, input)`, the shared conformance gate every transactional plugin runs over the create inputs it produces: the metadata must declare the current contract version and rebuild into a valid `Correlation`, no metadata key on the transaction or on any leg may fall outside `correlation.Keys`, and no level of the payload may set both `route` and `routeId`.
- `errors.NewResponseDecodeError` / `errors.IsResponseDecodeError` / `errors.CodeResponseDecode` — the shape returned when the SDK received a response it could not decode. Previously every facade decode failure came back as `NewInternalError`, which stamps `HTTPRequestSent=false`, `HTTPResponseReceived=false` and a synthetic HTTP 500: on a create, that told the caller the request never left the SDK when in fact the ledger had answered, and the transaction may already exist. The new error carries the truth (request sent, response received, no upstream status code to classify by) so a caller on the money path can recognise "outcome unknown, never replay" with `IsResponseDecodeError` instead of sniffing `encoding/json` error types across a module boundary. Applies to every write and single-object read that goes through the shared facade decode path.
- 13 plane-native accessors: Tracer plane (`Rules`, `Limits`, `Validations`, `Reservations`, `AuditEvents`) and ledger-plane extensions (`ProtectionAudit`, `Encryption`, `Instruments`, `Composition`, `FeePackages`, `FeeEstimates`, `BillingPackages`, `BillingCalculations`).
- `transaction.WaitForSettlement` — polls an account's balances until a caller-supplied predicate matches (an accepted transaction returns HTTP 201, which is not the same as settled).
- Model builders: `models.NewFeeEstimateInput`, `models.NewBillingCalculateInput`, and `models.NewCreateHolderAccountInput` (each with `With*` optionals).
- Typed error predicates: `IsSkipNotPermitted`, `IsHolderRequired`, `IsHolderNotFound`, `IsFeeError`, and `IsFeatureNotAvailable`.

### ✨ Added
- **`WithAllowInsecureHTTP` config option**: opt-in that permits plain `http://` Ledger and CRM service URLs for non-loopback hosts, both at config build time (`parseURL` / `WithLedgerURL` / `WithCRMURL` / `WithBaseURL`) and at every outbound request (`security.ValidateOutboundRequestWithInsecureHTTP`). DEFAULT IS FALSE — strict behavior is preserved for every existing caller. Intended for Kubernetes cluster-internal services reached over the cluster mesh (e.g. `http://midaz-ledger.midaz-mt.svc.cluster.local:3000`) and dev/test deployments behind a controlled network boundary. Independent from `WithAllowInsecureAccessManagerHTTP` (auth plane). Equivalent env var: `MIDAZ_ALLOW_INSECURE_HTTP=true` (loaded before URL env vars by `FromEnvironment` so ordering is automatic). Production environment rejects the flag at `Validate()` time. Public companion helpers: `pkg/config.WithAllowInsecureHTTP`, `pkg/config.Config.AllowInsecureHTTP`, `pkg/config.Config.GetAllowInsecureHTTP`, `pkg/security.ValidateOutboundRequestWithInsecureHTTP`, `entities.HTTPClient.SetAllowInsecureHTTP` / `AllowInsecureHTTP`. The redirect policy installed by the data-plane HTTPClient is automatically swapped to the permissive `ValidateRedirectWithInsecureHTTP` variant when the flag is on.

### 🐛 Bug Fixes
- **Config build fails for cluster-internal `http://*.svc.cluster.local` Ledger/CRM URLs**: downstream plugins running in Kubernetes multi-tenant staging (e.g. `plugin-br-bank-transfer`) configured `MIDAZ_LEDGER_URL=http://midaz-ledger.midaz-mt.svc.cluster.local:3000` and hit `build midaz sdk config: invalid transaction URL: insecure HTTP is only allowed for localhost targets`. The new `WithAllowInsecureHTTP` opt-in (and matching `MIDAZ_ALLOW_INSECURE_HTTP` env var) unblocks this canonical in-cluster pattern without weakening the SDK default.

### ⚠️ Known Limitations
- `pkg/retry.DoHTTPRequest` and `pkg/performance.BatchProcessor` retain strict outbound-URL validation regardless of `WithAllowInsecureHTTP`. Direct users of those public packages who reach an in-cluster `http://` target should call `security.ValidateOutboundRequestWithInsecureHTTP` from their own pre-flight or keep using HTTPS / loopback. A follow-up will thread the flag through these public retry / batch surfaces.

import "github.com/LerianStudio/midaz-sdk-golang/v2"

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0-beta.5...v2.1.0-beta.6)
Contributors: Guilherme Moreira Rodrigues

### 🐛 Bug Fixes
- **Configuration Update**: Resolved a misconfiguration in the `github-actions-gptchangelog` action. This fix enhances the stability and reliability of our continuous integration and deployment workflows, ensuring they run smoothly without the need for manual intervention. Users will experience more consistent and dependable automated processes as a result.

### 🔧 Maintenance
- **Environment Configuration**: Improved the setup of automated workflows, contributing to the overall robustness and efficiency of our development and deployment pipeline.


## [v2.3.0] - 2026-03-10

### ✨ Features
- add outbound URL validation package

### 🐛 Bug Fixes
- sanitize integrity checker log messages
- address remaining log injection findings
- sanitize logs for CodeQL alerts
- address CodeRabbit and CodeQL findings
- fix context cancellation leaks in batch processors
- validate outbound HTTP requests to prevent SSRF
- replace lib-commons v1 with midaz/v3/pkg/utils and lib-commons/v4

### 🔄 Changes
- inline context timeout logic
- simplify context cancellation result handler
- extract CLI flag parsing to dedicated cliFlags struct
- pre-allocate slices and use fmt.Fprintf to reduce allocations

### 🏗️ Build
- align Go toolchain to 1.26.0
- upgrade Go version to 1.26.1 and CI tooling
- pin gosec version and use 'go run' for reproducible checks

### 👷 CI/CD
- update versions of github actions in workflows

### 🔧 Maintenance
- Update CHANGELOG
- Update CHANGELOG
- Update CHANGELOG
- Update CHANGELOG
- Update CHANGELOG
- Update CHANGELOG
- update go modules to get latest features and fixes
- remove obsolete PLAN.md file
- Update CHANGELOG
- bump midaz, otel, gofakeit, and transitive dependencies


## [v2.3.0-beta.4] - 2026-03-10

### 🐛 Bug Fixes
- address CodeRabbit and CodeQL findings

### 🔧 Maintenance
- Update CHANGELOG


## [v2.3.0-beta.3] - 2026-03-10

### 🔄 Changes
- simplify context cancellation result handler

### 🔧 Maintenance
- Update CHANGELOG


## [v2.3.0-beta.2] - 2026-03-10

### 🏗️ Build
- pin gosec version and use 'go run' for reproducible checks

### 🔧 Maintenance
- Update CHANGELOG


## [v2.3.0-beta.1] - 2026-03-10

### 👷 CI/CD
- update versions of GitHub Actions in workflows

### 🔧 Maintenance
- Update CHANGELOG


## [v2.3.0-beta.5] - 2026-03-10

### ✨ Features
- add outbound URL validation package

### 🐛 Bug Fixes
- fix context cancellation leaks in batch processors
- validate outbound HTTP requests to prevent SSRF
- replace lib-commons v1 with midaz/v3/pkg/utils and lib-commons/v4

### 🔄 Changes
- extract CLI flag parsing to dedicated cliFlags struct
- pre-allocate slices and use fmt.Fprintf to reduce allocations

### 🔧 Maintenance
- Update CHANGELOG


## [v2.2.2-beta.1] - 2026-03-10

### 🔧 Maintenance
- update go modules to get latest features and fixes
- remove obsolete PLAN.md file
- Update CHANGELOG


## [v2.2.2-beta.1] - 2026-03-02

### 🔧 Maintenance
- bump midaz, otel, gofakeit, and transitive dependencies


## [v2.2.1-beta.1] - 2025-12-01

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0...v2.2.1-beta.1)
Contributors: Jefferson Rodrigues, lerian-studio

### 🐛 Bug Fixes
- **Improved Workflow Compatibility**: We've addressed an issue with example configurations and frontend components to ensure seamless operation across different deployment stages. This fix enhances the reliability and consistency of your testing and development processes, reducing potential disruptions when moving between environments.

### 📚 Documentation
- **Updated Changelog**: The CHANGELOG has been refreshed to accurately reflect recent changes and improvements, keeping you informed of the project's development progress and ensuring transparency.


## [v2.2.2-beta.2] - 2026-03-10

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0...v2.2.0)
Contributors: Arnaldo Pereira, Fred Amaral, Jefferson Rodrigues, lerian-studio

### ⚠️ Breaking Changes
- **Configuration Update**: The `DefaultRetryWaitMin` constant has been removed. Users relying on this setting must update their configurations to ensure compatibility with this version. Please refer to the updated configuration guide for migration steps.

### ✨ Features
- **Enhanced Observability**: OpenTelemetry context injection is now available for outgoing HTTP requests, allowing for improved tracing and monitoring across distributed systems. This feature enhances the ability to diagnose and resolve issues quickly.
- **Real-Time Asset Rates**: A new asset rates service has been introduced, providing real-time access to asset rate information. This is particularly beneficial for financial applications requiring up-to-date market data.
- **SDK Robustness**: The SDK now includes comprehensive tracing capabilities, significantly improving monitoring and debugging processes for developers.

### 🐛 Bug Fixes
- **API Consistency**: Fixed an issue to ensure consistent API versioning across services, reducing integration complexities and ensuring uniform behavior (#114).
- **Security Improvements**: Addressed potential security vulnerabilities by refining log sanitization and suppressing false positives in security analysis tools, enhancing overall code security.
- **Metrics Retrieval**: Corrected the HTTP method for metrics requests from HEAD to GET, aligning with standard practices and ensuring accurate data retrieval.

### ⚡ Performance
- **Log Sanitization**: Improved security and performance by using `strconv.Quote` for log sanitization, preventing log injection vulnerabilities and ensuring efficient logging.

### 🔄 Changes
- **Code Quality**: Refined `.golangci.yml` and updated `golangci-lint` rules to enforce stricter code quality, resulting in more maintainable and error-free code.

### 📚 Documentation
- **Enhanced Documentation**: Updated and clarified documentation, including improved godoc generation and the addition of missing release notes, ensuring users have access to accurate and comprehensive information.

### 🔧 Maintenance
- **Dependency Updates**: Updated Go module dependencies to the latest versions, ensuring compatibility and benefiting from performance improvements and security patches.
- **Build System Optimization**: Streamlined the build configuration by removing obsolete binaries and updating the `.gitignore` file to exclude generated outputs, enhancing the development workflow.


## [v2.2.0-beta.13] - 2025-11-29

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.12...v2.2.0-beta.13)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Improved Development Workflow**: Resolved a linting configuration issue by excluding the `utils` package from the variable naming rule. This fix reduces unnecessary linting errors, allowing developers to concentrate on more critical issues and maintain code quality more efficiently.

### 📚 Documentation
- **Updated Changelog**: The CHANGELOG has been updated to accurately reflect recent changes and improvements, providing users and developers with a clear history of modifications for better project transparency and tracking.

### 🔧 Maintenance
- **Release Management**: Ensured that the project documentation is current, indirectly benefiting users by supporting a more stable and well-documented software product.


## [v2.2.0-beta.12] - 2025-11-29

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.11...v2.2.0-beta.12)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Utilities (deps, test)**: Corrected the placement of the `nolint` directive in the utils package. This fix enhances code reliability by ensuring that linting processes ignore intended sections, helping developers maintain code quality and adhere to standards without unnecessary interruptions.

### 📚 Documentation
- **Changelog Updates**: The changelog has been updated to accurately reflect recent changes and improvements. This ensures that users and developers are well-informed about modifications, promoting transparency and better understanding of the project's evolution.

### 🔧 Maintenance
- **General Maintenance**: Regular updates and maintenance tasks have been performed to keep the project in optimal condition, supporting ongoing development and stability.


## [v2.2.0-beta.11] - 2025-11-29

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.10...v2.2.0-beta.11)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Build/Deps**: Removed an unused `nolint` directive from `utils.go`. This improvement enhances code quality by ensuring adherence to linting standards, which helps in reducing potential technical debt and contributes to a more stable and reliable software environment.

### 📚 Documentation
- **Changelog Update**: The changelog has been updated to reflect the latest changes and improvements. This ensures that users and developers have access to the most current information about the project's progress, facilitating better understanding and tracking of the software's evolution.

### 🔧 Maintenance
- **Release Management**: The maintenance of the codebase and documentation has been prioritized to ensure high standards of quality and clarity, contributing to a robust development environment.


## [v2.2.0-beta.10] - 2025-11-29

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.9...v2.2.0-beta.10)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Improved Code Quality**: Corrected the placement of the `nolint` directive to align with the package declaration. This ensures linting tools interpret the directive correctly, reducing false-positive linting errors and improving the overall reliability of the codebase. Users will experience fewer interruptions and more accurate linting results, particularly in the `deps` and `test` components.

### 📚 Documentation
- **Updated Changelog**: The CHANGELOG has been refreshed to accurately reflect recent changes and improvements. This update provides all stakeholders with the latest information on the project's development, ensuring transparency and ease of access to critical updates.

### 🔧 Maintenance
- **Documentation Maintenance**: Regular updates to documentation ensure that users have access to clear, up-to-date information, enhancing the usability and understanding of the SDK's features and improvements.


## [v2.2.0-beta.9] - 2025-11-29

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.8...v2.2.0-beta.9)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Utilities**: Corrected the placement of the `nolint` directive in the utilities package. This fix ensures that linting tools correctly ignore intended sections of the code, reducing false positives and enhancing code quality and maintainability. This improvement affects the `deps` and `test` components, ensuring automated checks run smoothly.

### 📚 Documentation
- **Changelog Update**: The CHANGELOG has been updated to reflect recent changes and improvements. This update helps users and developers stay informed about the latest modifications and enhancements in the project, ensuring transparency and ease of tracking project evolution.


## [v2.2.0-beta.8] - 2025-11-28

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.7...v2.2.0-beta.8)
Contributors: Fred Amaral

### 🐛 Bug Fixes
- **Improved Security Scans**: Suppressed false positive alerts from CodeQL in the authentication module, enhancing the reliability of security scans. This ensures that developers can concentrate on real security issues, improving the overall security posture of the application.

### 🔧 Maintenance
- **Code Quality Improvements**: Updated code annotations and comments across build, backend, and documentation components to suppress false positives in CodeQL analysis. This maintenance task ensures a cleaner codebase and more accurate feedback from automated code review tools, allowing developers to focus on meaningful improvements.


## [v2.2.0-beta.6] - 2025-11-28

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.5...v2.2.0-beta.6)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Enhanced Log Security:** Improved log sanitization across various components (Auth, Backend, Build, Docs) by using `strconv.Quote`. This update prevents log injection vulnerabilities by properly escaping potentially harmful characters, safeguarding sensitive information, and enhancing overall system security.

### 🔧 Maintenance
- **Changelog Update:** The CHANGELOG has been updated to reflect the latest changes and improvements, ensuring users have access to the most current information about the software's evolution and can easily track updates and fixes.


## [v2.2.0-beta.5] - 2025-11-28

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.4...v2.2.0-beta.5)
Contributors: Fred Amaral, lerian-studio

### 🐛 Bug Fixes
- **Security Enhancement**: Resolved log injection vulnerabilities across key components, including authentication, backend, and build processes. This fix prevents malicious log entries, safeguarding data integrity and system stability.

### 📚 Documentation
- **Changelog Update**: The CHANGELOG has been updated to accurately reflect recent changes and improvements. This ensures users have the latest information for effective version tracking and understanding of updates.

### 🔧 Maintenance
- **Release Process Improvement**: Updated the CHANGELOG as part of the release process, ensuring all changes are well-documented and communicated. This enhances transparency and supports a well-organized development lifecycle.


## [v2.2.0-beta.4] - 2025-11-28

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.3...v2.2.0-beta.4)
Contributors: Fred Amaral

### 🗑️ Removed
- **Obsolete Tracing Example Binaries**: We have removed outdated tracing example binaries from the backend, build, docs, and frontend components. This cleanup reduces clutter and potential confusion, ensuring a more streamlined and efficient development experience. New contributors will benefit from a clearer codebase, free from misleading examples.

### 🔧 Maintenance
- **Codebase Cleanup**: By eliminating unnecessary files, we improve the overall maintainability of the project. This change helps keep the codebase organized and easier to navigate, which is especially beneficial for new developers joining the project.


## [v2.2.0-beta.3] - 2025-11-28

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.2...v2.2.0-beta.3)
Contributors: Fred Amaral

### ✨ Features
- **Asset Rates Service**: New `AssetRatesService` for managing currency conversion rates with full CRUD operations, pagination, and validation. Essential for multi-currency ledger systems requiring standardized exchange rate management.
- **Transaction Type Helpers**: Added `CreateInflowTransaction`, `CreateOutflowTransaction`, and `CreateAnnotationTransaction` methods for common ledgering patterns (deposits, withdrawals, metadata-only entries).
- **OpenTelemetry Context Propagation**: HTTP client now automatically injects OpenTelemetry tracing context into outgoing requests, enabling distributed tracing across service boundaries.

### 🐛 Bug Fixes
- **API Versioning**: Resolved inconsistencies in API versioning across services for more reliable integrations.
- **Metrics Endpoint**: Changed from HEAD to GET method for metrics requests, fixing compatibility issues.
- **URL Path Escaping**: Special characters in URL path parameters are now properly escaped, preventing routing errors.
- **Nil Pointer Panics**: Fixed potential nil pointer dereferences in example code and concurrent data race conditions.
- **CI Pipeline**: Updated golangci-lint to v2.6.2 and aligned Go version in coverage commands.

### ⚡ Performance
- **Test Suite Expansion**: Added comprehensive test coverage (25,000+ lines) across all packages including entities, models, pkg utilities, and examples.

### 🔧 Maintenance
- **Stricter Linting**: Overhauled `.golangci.yml` configuration with significantly tighter rules for improved code quality.
- **Code Refactoring**: Extracted constants, improved error handling patterns, and simplified return statements across entities and pkg modules.
- **Documentation**: Updated godoc documentation and improved package-level documentation throughout the codebase.
- **Naming Consistency**: Renamed `DefaultRetryWaitMin` constant and `url` variables to `endpoint` for semantic clarity.


## [v2.2.0-beta.2] - 2025-10-08

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.2.0-beta.1...v2.2.0-beta.2)
Contributors: Arnaldo Pereira, lerian-studio

### 🐛 Bug Fixes
- **Account Balance Retrieval**: Fixed an issue in the `accounts.GetBalance()` method where incorrect API endpoint paths were used. This ensures accurate and reliable balance queries, enhancing user trust and system dependability.

### 📚 Documentation
- **Changelog Update**: The changelog has been updated to include recent changes and improvements, ensuring users have access to the most current information about software updates and fixes.

### 🔧 Maintenance
- **Documentation Maintenance**: Regular updates to documentation help maintain clarity and accuracy, supporting users in understanding and utilizing the software effectively.


## [v2.2.0-beta.1] - 2025-10-06

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.1-beta.1...v2.2.0-beta.1)
Contributors: Arnaldo Pereira, lerian-studio

### ✨ Features
- **Comprehensive Tracing**: Gain in-depth insights into API call flows and performance metrics with our new tracing feature. This enhancement allows for precise monitoring and easier debugging, helping you track requests throughout the system and optimize performance efficiently.

### ⚡ Performance
- **Integrated Tracing in Build**: Tracing is now seamlessly integrated into the build process, ensuring consistent monitoring across all environments without additional setup. This integration enhances system observability and reduces the time needed for configuration.

### 📚 Documentation
- **Tracing Guides**: We have updated our documentation to include detailed guides on utilizing the new tracing features. Access step-by-step instructions to maximize system observability and performance analysis.

### 🔧 Maintenance
- **Expanded Test Coverage**: Our test suite now includes comprehensive tests for the new tracing functionalities, ensuring reliability and stability across the API.
- **Changelog Update**: The CHANGELOG has been revised to reflect the latest updates and improvements, keeping all stakeholders informed about the system's enhancements.


## [v2.1.1-beta.1] - 2025-10-03

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0...v2.1.1-beta.1)
Contributors: Arnaldo Pereira

### 🐛 Bug Fixes
- **Consistent API Versioning**: Resolved discrepancies in API versioning across services, ensuring more reliable and predictable interactions between components. Users will experience smoother service integration and fewer unexpected behaviors (#114).

### 🔧 Maintenance
- **Code Cleanup and Refactoring**: Improved code readability and maintainability by reducing the codebase by 27 lines. This behind-the-scenes enhancement supports better system management and future development, although it doesn't directly affect user-facing features.


## [v2.1.0-beta.4] - 2025-09-30

[Compare changes](https://github.com/LerianStudio/midaz-sdk-golang/compare/v2.1.0-beta.3...v2.1.0-beta.4)
Contributors: Guilherme Moreira Rodrigues

### 🐛 Bug Fixes
- **Improved GitHub Actions Compatibility**: Updated the configuration to use the 'with' keyword for input parameters instead of 'env'. This change prevents potential execution errors, ensuring that the CI/CD pipeline runs smoothly and reliably, which is crucial for maintaining consistent deployment processes.

### 🔧 Maintenance
- **Configuration Update**: This behind-the-scenes improvement aligns our setup with the latest GitHub Actions best practices, reducing the risk of future compatibility issues and enhancing the overall stability of our development workflow.


## [v2.3.0-beta.1] - 2026-03-10

This major release of the midaz-sdk-golang introduces significant enhancements to deployment processes and system architecture, alongside improvements in documentation and code quality.

### ⚠️ Breaking Changes
- **Backend/Config**: Models have transitioned to utilize Midaz entities, requiring updates to backend service integrations. This change enhances consistency and future-proofs the architecture. Users should review and adjust their model interfaces accordingly. [Migration Guide](#)

### ✨ Features  
- **Config**: A new release flow now supports Hotfixes (HF) and Breaking Changes (BC), offering more flexible and controlled deployment options for smoother updates and rollbacks.

### 🐛 Bug Fixes
- **Frontend**: Resolved various linting issues and improved variable naming conventions, enhancing code clarity and reducing potential errors.

### 📚 Documentation
- **Docs**: Expanded to include new accounting features and removed outdated scale fields, providing clearer guidance and reducing confusion for users.

### 🔧 Maintenance
- **Build/Deps**: Updated dependencies, including `github.com/LerianStudio/lib-commons` from 1.8.0 to 1.12.1, addressing security vulnerabilities and ensuring compatibility with the latest features.
- **Build/Docs/Frontend/Test**: Comprehensive cleanup of golangci-lint violations, improving code quality and maintainability across multiple components.

This release focuses on enhancing user experience through improved deployment processes, clearer documentation, and robust code quality standards.

This changelog is structured to provide users with a clear understanding of the changes, focusing on the impact and benefits of the new version. It includes essential details about breaking changes, new features, bug fixes, documentation updates, and maintenance improvements, all presented in a user-friendly format.

## [v2.0.0-beta.1] - 2025-08-04

This release introduces significant enhancements to the midaz-sdk-golang, including a major transition to Midaz entity models, improved code quality, and updated documentation. These changes aim to improve data consistency, maintainability, and user experience.

### ⚠️ Breaking Changes
- **Backend**: Transition to Midaz entities for all models. This change enhances data consistency and aligns with Midaz standards. **Action Required**: Update your integrations and data handling processes to accommodate these new entities. [Migration Guide](#)

### ✨ Features  
- **Backend**: Introduced Midaz entity models, offering a standardized and robust data structure that supports future scalability and integration with other Midaz services. This update is crucial for maintaining compatibility with future SDK updates.

### 🐛 Bug Fixes
- **Test**: Adjusted routing methods and removed obsolete scale fields, improving test accuracy and reliability, ensuring smoother testing processes.

### ⚡ Performance
- **Frontend**: Refactored code to replace 'interface{}' with 'any', improving code readability and maintainability, which enhances developer experience and aligns with modern Go practices.

### 🔄 Changes
- **Build/Test**: Cleaned up golangci-lint violations across multiple components, resulting in improved code quality and reduced technical debt.

### 📚 Documentation
- **Docs**: Updated documentation to include new accounting features and removed outdated scale fields, ensuring users have access to the latest feature information and guidelines.

### 🔧 Maintenance
- **Dependencies**: Upgraded dependency versions to ensure compatibility with the latest security patches and performance improvements.
- **Code Quality**: Various linting improvements, including variable renaming and code standardization, enhancing overall codebase maintainability.

This changelog provides a clear and concise overview of the changes in version 2.0.0, focusing on user impact and necessary actions. It highlights the benefits of new features, improvements, and maintenance updates, ensuring users understand the importance and implications of this release.

## [v1.4.0-beta.1] - 2025-07-31

This release introduces a streamlined configuration process for faster updates and enhances system performance through key dependency updates.

### ✨ Features
- **New Release Flow for Configuration**: We've implemented a new release flow to support Hot Fix (HF) and Bug Correction (BC) processes. This enhancement ensures quicker deployment of critical updates, improving system stability and performance for all users.

### ⚡ Performance
- **Dependency Update**: Upgraded the `github.com/LerianStudio/lib-commons` library from version 1.8.0 to 1.12.1. This update brings performance improvements and bug fixes that enhance the efficiency and reliability of components like authentication and build processes.

### 🔧 Maintenance
- **Build System Robustness**: The dependency update not only improves performance but also ensures compatibility with the latest security patches, maintaining the robustness and security of our build system.

By focusing on these enhancements and maintenance updates, users can expect a more streamlined and efficient experience, with improved system reliability and performance.

## [v2.3.0-beta.2] - 2026-03-10

### ✨ Features
- Improve release flow by fixing the goreleaser file, enhancing the overall release process.

### 🔧 Maintenance
- Bump `go.opentelemetry.io/otel` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/metric` from version 1.35.0 to 1.36.0.
- Bump `go.opentelemetry.io/otel/trace` from version 1.35.0 to 1.36.0.

## [v1.3.0-beta.2] - 2025-05-27

### 🔧 Maintenance
- Bump `go.opentelemetry.io/otel/trace` from version 1.35.0 to 1.36.0 to ensure compatibility with the latest features and improvements (#38).
- Update CHANGELOG to reflect recent changes and maintain accurate project documentation.

## [v1.3.0-beta.1] - 2025-05-05

### ✨ Features
- Update `goreleaser` configuration to improve release flow, enhancing the efficiency and reliability of the release process.

### 📚 Documentation
- Update CHANGELOG with recent changes to ensure it reflects the latest updates and improvements.

## [v2.3.0-beta.3] - 2026-03-10

### 🔧 Maintenance
- Rename `pluginAuth` to `AccessManager` and update related documentation for clarity and consistency.

### 📚 Documentation
- Update CHANGELOG to reflect recent changes and improvements in the project.

## [v2.3.0-beta.4] - 2026-03-10

### ✨ Features
- Rebuild release steps using custom modules to streamline the deployment process.
- Add gosec security checks to Makefile to enhance code security.

### 🐛 Bug Fixes
- Correct goreleaser step in the release process to ensure successful builds.

### 🔄 Changes
- Rename `pluginAuth` to `pluginAuth` and update related documentation for clarity.
- Adjust logging in `observability-demo.go` to prevent unused variable warnings.

### 🔧 Maintenance
- Configure checkout tags in CI workflow to improve version control accuracy.
- Set CodeQL analysis on default execution and add CodeQL analysis step to workflow for enhanced code quality checks.
- Configure additional workflow steps to optimize CI/CD processes.
- Remove unused `debugLog` function from `client.go` and replace unused client parameter with underscore in `main.go` for cleaner code.

### 📚 Documentation
- Update documentation to reflect changes in `pluginAuth`.


ianStudio/midaz-sdk-golang/compare/v1.0.7...v1.1.0-beta.1) (2025-04-09)

### Features

* **docs:** improve documentation on auxiliary packages ([9cd23e8](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/9cd23e8251bbcf9080d4f6bd73d8b6b79d7f665f))

## [1.0.7](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.6...v1.0.7) (2025-04-08)

### Bug Fixes

* **readme:** alignment ([bb62be1](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/bb62be17112245645e80747f7f24761af40ce62f))
* **readme:** alignment ([a4ce92c](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/a4ce92cca5efbf322e0f14d3fc03b49deb1a71b0))

## [1.0.6](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.5...v1.0.6) (2025-04-08)

### Bug Fixes

* **readme:** minor ([590a02e](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/590a02e9b584380949420501a6b2446ac7688cb5))

## [1.0.5](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.4...v1.0.5) (2025-04-08)

### Bug Fixes

* **readme:** banner image ([c362c6c](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/c362c6c32f1a929641025854066fa943fbd92c6b))

## [1.0.4](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.3...v1.0.4) (2025-04-08)

### Bug Fixes

* **readme:** fixing readme banner ([3a6d42a](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/3a6d42ab3aa86eda9f47a64863e7d9763610ca51))

## [1.0.3](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.2...v1.0.3) (2025-04-08)

## [1.0.2](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.1...v1.0.2) (2025-04-08)

### Bug Fixes

* **tests:** time tests to comply with pipeline machine time ([1912dd0](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/1912dd0b994bdb7d06e2522bf1451b1014865c05))
* **tests:** time tests to comply with pipeline machine time ([bb7806f](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/bb7806ff4e381c3c82bdaec47b60f19d50445cf7))

## [1.0.1](https://github.com/LerianStudio/midaz-sdk-golang/v2/compare/v1.0.0...v1.0.1) (2025-04-08)

### Bug Fixes

* **pipeline:** artifacts version ([6bb53f2](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/6bb53f2891d45ea6dc15b8a4f79c9fdbe97807e5))

## 1.0.0 (2025-04-08)

### Features

* **sdk:** init repo ([709cb58](https://github.com/LerianStudio/midaz-sdk-golang/v2/commit/709cb5813927c4c505cd7d3da45cbf370cc67273))

