# SDK v1+v2 endpoint reconciliation with Midaz develop

**Date:** 2026-08-25
**Branch:** `agent/sdk-v1v2-reconciliation` (worktree `/srv/worktrees/sdk-v1v2-reconciliation`, based on develop @ 4143472; rename to `feat/...` before PR)
**Decision (Fred, 2026-08-25):** the SDK serves BOTH Midaz surfaces — v1 (deprecated but alive) and v2 (the real one) — with explicit version mechanics mirroring the server. No single-version pin.

## Diagnosis (verified against .references/midaz @ d816e289b, develop)

- Midaz built the full /v2 surface Jun–Aug 2026, deprecated ALL of /v1 (`db56dde04`), and **removed from /v1**: CRM (`cede08b1a`), fees (`109e9c1de`), composition (`804825a93`).
- v2 dropped legacy transaction creates (`7150848bd`) — creation is `POST /v2/transactions/direct|hold`, plus `block|unblock`, all top-level (not org/ledger-scoped in the URL).
- v2 dropped asset-rates (`c781f6a97`) — asset-rates is v1-only.
- Billing family moved org-scope → **ledger-scope** in v2 (`/v2/organizations/{org}/ledgers/{ledger}/billing-packages|packages|estimates|billing/calculate`).
- SDK today pins `/v1` on every base URL (`entities/entity.go:normalizeServiceURL`, `config.DefaultLedgerAPIVersionPath`). Consequence: holders, instruments, encryption, billing-packages, packages, estimates, composition, protection/audit **404 against Midaz develop**.
- SDK bugs found on the way: `entities/aliases.go` calls `/aliases`, `/holders/{id}/aliases` — routes that no longer exist anywhere (renamed to instruments, v2-only). **Correction (2026-08-25, review):** the earlier claim that `entities/operations.go` PATCHes a non-existent account-scoped path was WRONG on both halves. The update already targets the transactions-scoped path the server serves (`.../transactions/{txId}/operations/{opId}`), and the account-scoped operation READ (`.../accounts/{id}/operations[/{opId}]`) does exist server-side, deprecated. Epic 2 must migrate that file onto the generated client, NOT "fix" a path that is already correct.
- Tracer plane: SDK spec == server spec, path-for-path. **Zero tracer work.**
- Old branch `sync/monorepo-20251219`: evaluated, discarded (Dec 2025, 442 commits behind, content superseded by v4 remodel + PR #236).

## Mechanics that make this cheap

- Server OAS (`components/ledger/api/openapi.huma.yaml`): `servers: [{url: "/"}]`, version prefix **in the paths**, operationId seam `listOrganizations` (v1) / `listOrganizationsV2` (v2). 197 ops: 87 v1, 110 v2.
- SDK codegen: `api/ledger.openapi.yaml` → `scripts/generate-clients.sh` (specdowngrade 3.1→3.0.3 + oapi-codegen) → `internal/genledger`, with drift gate (`make check-codegen-drift`). Adopting the server spec + regen produces both versioned client surfaces automatically.

## Target public surface (code decision)

`client.Entity.V1.*` and `client.Entity.V2.*` service groups, each exposing only what its server version serves. The asymmetry (asset-rates v1-only; holders/billing/direct-hold v2-only) becomes compile-time visible. Base URLs lose the version pin (server base is `/`). Tracer accessors unchanged. Breaking for v5-beta users — accepted, v5 is beta.

## Epics

### Epic 1 — Spec adoption + regen + pin removal (foundation)
1. Copy `.references/midaz/components/ledger/api/openapi.huma.yaml` → `api/ledger.openapi.yaml`. Verify tracer spec byte-diff (expected no-op).
2. `make generate`.
3. Remove the `/v1` pin in the same stroke (spec paths now carry /v1; keeping the pin would produce `/v1/v1/...`): `normalizeServiceURL` keeps host+optional path, no version append; drop `DefaultLedgerAPIVersionPath`; update `pkg/config` + the three `.env*.example` files + `check-config-parity`. NOTE: the tracer spec has unversioned paths and the tracer serves under /v1 — the tracer base URL must keep its /v1 (pin stays tracer-side or moves into tracer URL default); verify against server routing before choosing.
4. Fix compile fallout in facades by renaming to the new v1 operationIds — wire behavior on v1 preserved.
5. Gate: `go build ./...`, `make test`, `make check-codegen-drift` green.

### Epic 2 — Version groups + legacy repair
1. Introduce `Entity.V1` / `Entity.V2` groups; move existing facades under `V1` (paths now carry /v1 from the spec).
3. Delete `entities/aliases.go` (dead server-side). Migrate `entities/balances.go` + `entities/operations.go` onto the generated client (kills the `transaction`/`crm` base-URL keys and fixes the operations PATCH path to transactions-scoped).
4. Gate: existing v1-family tests green under the new accessors.

### Epic 3 — V2 surface
1. V2 facades for the dual families (organizations, ledgers, accounts, account-types, assets, balances, operations, portfolios, segments, routes, settings, metadata-indexes, counts).
2. Move the v2-only families to their real home: holders, instruments, encryption, protection/audit, composition under `V2`; billing-packages/packages/estimates/billing-calculate **rescoped to ledger**.
3. Transactions V2: reads/patch/cancel/commit/revert under ledger scope + top-level `direct`/`hold`/`block`/`unblock`; wire to the canonical contract work from PR #236; model divergences (v2 dropped deprecated response fields) mapped in `models/`.
4. Missing v1 endpoints worth exposing while here: `transactions/block|unblock` (v1), external-code account/balance lookups (already generated, no facade).
5. Gate: unit tests per facade; `make ci`.

### Epic 4 — Consumers + verification
1. Examples (`examples/workflow-with-entities`, `examples/mass-demo-generator`) onto the new accessors; `make demo-data` run.
2. Docs: `README.md`, `docs/README.md`, `docs/mapping/{external,internal}_apis.md`.
3. Full `make ci` + `make verify-sdk`; PR (base develop, `feat!:` — breaking).

## Findings from Epic 1 (input for Epics 2-4)

- **Tracer spec HAS drifted** — the diagnosis line "SDK spec == server spec, path-for-path" holds for PATHS only. Schemas diverge: the server renamed `currency` → `asset` in three limit/usage schemas, added an RFC 9457 `upstream` error extension member (+ `Upstream` schema), and returns **201 Created** where the SDK spec still says 200 on three operations. Left untouched in Epic 1 (semantic, not cosmetic, and outside a ledger-scoped epic). Needs its own commit: adopt the server tracer spec, regen, fix the `Currency` field rename (3 refs in `internal/gentracer`) and the 200/201 gates. **Closed in Epic 2 — and it was WORSE than "drift":** the rename is not confined to three response schemas. The Tracer's REQUEST bodies are opaque `format: binary` in its OAS (a Huma artifact), so the SDK's hand-written input models were the only place the request field name lived, and they still said `currency`. Server-side `asset` is `validate:"required"` on all three write paths (`internal/adapters/http/in/limit_validation.go:94`, `pkg/model/validation.go:78`, and `ReserveRequest`, which embeds `ValidationRequest`), and the tracer server has **zero** `json:"currency"` tags left in non-test Go code. So every limit create, transaction validation and reservation the SDK sent was rejected on the wire, and every limit / validation read decoded an empty asset. Six public model fields renamed (`Limit`, `CreateLimitInput`, `ValidateTransactionInput`, `ReserveInput`, `TransactionValidation`, `ValidationSummary`).
- `WithBaseURL` and the environment defaults fan one base URL out to both planes; each fan-out now has to re-shape the tracer copy (`tracerURLFromSharedOrigin`). Any new fan-out path must do the same or tracer calls land one segment short.
- `CreateOrganization` lost its OAS `Authorization` param (now a security scheme). It was always passed empty — the auth round tripper sets the header — so this is a wire no-op.
- Among the v1 operations only `CreateOrganization` drifted in its Go SIGNATURE, but the error CONTRACT changed across all 85 v1 `Parse*Resp` functions: the regen made the server's real v1 error shape explicit, swapping `Error` for `LegacyError` and `application/problem+json` for `application/json`. Request/response wire behavior on the success path is preserved; the error path was NOT — the SDK error decoder only read RFC 9457 members, so every v1 error decoded to "API error with status code N" with field detail dropped. Fixed on the decode side (FINDING 6, `fix(errors): decode v1 legacy error bodies`): the decoder now reads `message`/`fields`/`entityType` when the RFC members are absent.
- The `ledgerId` query filter is gone from `PackagesFilters` / `BillingPackagesFilters` (the ledger is a path segment on v2). Public model change already landed.
- **Correction (2026-08-25, review):** the original line here — "they read the `crm`/`transaction` base-URL keys, which are now bare, so they no longer double-version" — recorded a CRITICAL regression as a non-event. Bare bases did not make those three services correct; it made them UNDER-version. Building paths by concatenation off a bare base emitted `/organizations/...` where the server routes only `/v1/organizations/...`, i.e. a guaranteed 404 across 24 methods (18 balances, 5 operations, aliases). Fixed by stamping `/v1` in the hand-built paths (`fix(entities): version the hand-rolled balance, operation and alias paths`) — a stopgap that retires when Epic 2 migrates balances and operations onto the generated client. **Aliases is NOT fixed by that stamp and cannot be:** the server has zero alias routes at any version, so the stamp only swapped one 404 for another. `entities/aliases.go` stays dead until Epic 2 deletes it. The old tests could not catch it: they injected a base URL that already carried `/v1`, so they asserted a shape the shipped client never produces. Epic 2 must keep the pipeline-level path test (client built from `WithBaseURL` against a live server) when it removes the stopgap.

## Findings from Epic 2 (input for Epics 3-4)

Everything below was verified against `.references/midaz` @ `d816e289b` or against a live `httptest.Server` through the public client.

**Corrections to earlier claims:**

- **The three CRM validators had NO consumer.** The Epic-2 brief asserted `validateCRMOrganizationID` / `validateCRMUUIDParam` / `validateOptionalCRMUUIDParam` were used by the holders/instruments/composition facades. They were not — each had exactly one caller, `entities/aliases.go`. Those facades route over the generated client and validate through it. Deleting the alias service left the whole of `entities/crm_shared.go` dead; it is gone. (`models.validateCRMNullFields` is a different symbol and does survive, via `models/instrument.go` and `models/holder.go`.)
- **The plan's own Epic 2 item 3 was already corrected once and the correction held**: the operations PATCH was always transactions-scoped, and the account-scoped operation READ does exist server-side as deprecated. Neither was "fixed"; both were migrated as-is.
- **The `transaction` base-URL key was a phantom, not just redundant.** After the migration nothing read it to build a request, yet `pkg/config.Validate` and `midaz.setupEntity` both FAILED without it — a mandatory config key with no effect. Removed end to end, along with `pkg/config.ServiceTransaction`. `onboarding` is now the sole Ledger key and `tracer` the sole Tracer key. No `.env` change was needed: `MIDAZ_TRANSACTION_URL` and `MIDAZ_CRM_URL` never existed as env vars, only as internal labels.

**Money-path bugs found while migrating (all pre-existing, all now fixed and guarded):**

- **The balances list is cursor-paginated and the legacy iterator was an unbounded loop.** `components/ledger/internal/services/query/get_all_balances.go:32` calls `filter.ToCursorPagination()`, which (`pkg/net/http/httputils.go:533`) carries only Limit/Cursor/SortOrder/StartDate/EndDate — `page` is dropped on the floor. The old `ListBalancesPages` gated on `Pagination.HasMore()` (true whenever `NextCursor != ""`) and then did `current.Page++`, a parameter the server ignores, so against a real multi-page ledger `ListBalancesAll` re-requested page 1 forever, yielding the same balances indefinitely. `models.BalancesListOpts` therefore had to change shape (`PageListOpts` → `CursorListOpts`); the method signatures are unchanged. Guarded by `TestListBalancesAll_AdvancesByCursor`.
- **`models.BalancesFilters` (AccountID / AssetCode / Status) was never honoured** — no wire slot in the generated params, no server-side reader. Deleted rather than left silently dropping the caller's filter.
- **`models.OperationsFilters` exposed the wrong three fields.** The server (`get_all_operations_account.go:32-37`) filters on `OperationType`, `Direction`, `RouteID`, `RouteCode`; the SDK exposed `Status` and `AssetCode`, which the server parses and then ignores, and exposed none of the honoured ones. `OperationsListOpts`/`OperationsFilters` are now aliases of the `AccountOperations*` pair: one server endpoint, one type, no drift possible.
- **The alias and external-code balance lookups take no query parameters and are not paginated** (`.../http/in/balance.go:197-239` returns a fixed `Pagination{Limit: 10}`; both generated ops have no params struct). Their `*Pages`/`*All` iterators were fiction. Non-zero opts are now rejected up front.
- **The point-in-time date validator was too NARROW, not too loose.** It demanded RFC3339 with an explicit offset and rejected `"2026-01-02 03:04:05"` — the exact format the OAS documents for the `date` query. The server accepts RFC3339, `2006-01-02T15:04:05` and `2006-01-02 15:04:05`, and separately requires a time component. The facade now mirrors that set exactly.
- **The RFC 9457 `upstream` member was being discarded by the SDK error decoder.** lib-commons scrubs detail, `errors[]` and code at status >= 500 and lifts `upstream` through that scrub as the single exception (`commons/net/http/problem/install.go:81`), so on a scrubbed 5xx it is the only actionable diagnostic there is. It now lands in `Details["upstream"]`.

**Deliberate decisions (Fred/orchestrator, 2026-08-25):**

- **`WithLedgerURL` does not derive the Tracer base; only `WithBaseURL` fans one origin out to both planes.** Kept as-is: a per-plane option addresses one plane, and `WithBaseURL` is the single fan-out. Recorded because it is an asymmetry a reader will trip over next to the Epic-1 fan-out finding.
- **`onboarding` stays the Ledger plane's key name for now**, but it is a rename candidate (`onboarding` → `ledger`): it is the last place the pre-two-plane service naming survives, and it now names the whole Ledger plane rather than one service. Flagged for the review round, not done here.
- **`entities/observability.go` stays** (415 lines, zero callers, one file-level `//nolint:unused`). It plus the three `t.Skip`ped tests in `business_observability_test.go` ARE the specification that the pre-existing deferred Task 5.2.6 re-homes (`docs/plans/2026-06-30-sdk-v4-remodel.md:621`). `entities/operations.go` was the last business-event emitter, so emission is now zero-coverage across all accessors — which was already the sanctioned state for the other 15.

**Latent, cross-cutting, NOT introduced by this epic — wants one decision across all 15 facades:**

- A single-object response whose `id` is not a UUID fails inside the generated `Parse*Resp` and surfaces as `errors.NewInternalError`, i.e. an SDK internal fault rather than a decode error. Harmless against Midaz today (ids are UUIDs) but a server-side id-format change would degrade into the wrong error class.
- **Every generated DELETE parser unmarshals the body as an error whenever `Content-Type` contains `json`.** A 204 with `Content-Type: application/json` and an empty body therefore returns `unexpected end of JSON input` — a SUCCESSFUL delete reported as a failure. Midaz is safe today because its delete handlers return `struct{}` and Huma emits a bodiless 204 with no content type, but any gateway or proxy that adds one breaks every delete in the SDK.

## Risks
- v2 request/response schema divergences beyond paths (typed fields) surface during Epic 3 — the generated types are ground truth, not the v1 siblings (see plane-clients memory).
- `/v2/transactions/direct|hold` carry org/ledger in the body — contract to be read from the generated types, not assumed.
- Release tooling: v5 module path already handled (#239); no module bump here.

## Status
- [x] Diagnosis + decision
- [x] Epic 1 — **landed** (`ad3e193`). Server spec adopted; the generated client now carries both surfaces (v1 ids bare, v2 ids `*V2`). Ledger base URL is bare, tracer base keeps `/v1` (verified against the server's Fiber `/v1` group mount). The families Midaz removed from /v1 — holders, instruments, encryption, composition, protection/audit — are rewired onto their V2 operations; the billing family (billing-packages, packages, estimates, billing/calculate) is rescoped to ledger and its facades take a `ledgerID`. Gates green: `go build`, `check-codegen-drift`, `make test`, `make lint` (0 issues).
- [x] Epic 1 fix round (2026-08-25, post-review). Seven findings closed, one commit each:
  1. `fix(entities)` — the three hand-rolled services emitted unversioned paths off the now-bare base (guaranteed 404 on 24 methods). `/v1` stamped in; pipeline-level path tests added; the version-blind test injections replaced. This repaired balances (18 methods) and operations (5); aliases stayed broken either way — the server serves no alias route at any version, so the stamp only changed which 404 it earns. Epic 2 deletes that file.
  2. `fix(entities)` — a `/v1` or `/v2` suffix on the Ledger base is now rejected at construction (it silently double-versioned; old `.env` files still carry it). Tracer stamping untouched.
  3. `docs(examples)` — the workflow example no longer instructs the rejected `MIDAZ_LEDGER_URL=.../v1`.
  4. `docs` — base-URL statements corrected in `docs/environment.md`, `docs/comprehensive-architecture.md`, `docs/mapping/internal_apis.md`, `README.md`. Surrounding prose stays Epic 4.
  5. `fix(entities)` — billing calculation and fee estimation reconcile the ledger in the path with the `ledgerId` in the body (fill when empty, reject when contradicting). Money-adjacent.
  6. `fix(errors)` — v1 legacy error bodies (`message`/`fields`/`entityType`) now decode instead of degrading to "API error with status code N".
  7. `refactor(entities)` — the unreachable per-plane base-URL fallbacks at entity construction removed.
- [x] Epic 1 fix round 2 (2026-08-25, final). Four findings closed, one commit each:
  1. `fix(errors)` — a construction failure rendered as "failed to initialize entity API" and nothing else; the actionable cause (which base URL, which variable to edit) only survived behind `errors.Unwrap`. The cause text now travels in `Error()`; the wrapped cause is unchanged.
  2. `fix(entities)` — billing-package and fee-package creation now reconcile the path ledger with the body `ledgerId`, closing the same money-adjacent gap fixed on calculation/estimation in round 1.
  3. `docs` — the `make demo-data` guidance and the CRM-plane statements in `docs/comprehensive-architecture.md` corrected (they still instructed the rejected `/v1` base and a `MIDAZ_CRM_URL` that no longer exists). `docs/mapping/*` still carries `WithCRMURL`/`MIDAZ_CRM_URL` — Epic 4 owns those files.
  4. `docs(plans)` — this record: the round-1 stamp did not fix aliases.
- [x] Epic 2 — **landed** (7 commits, `e355c0a`..`731e9c1`, not pushed). Gates green: `go build ./...`, `make test`, `make check-codegen-drift`, `make lint` (cold cache, 0 issues), `scripts/check-config-parity.sh`.
  1. `fix(tracer)` + `fix(errors)` — server tracer spec adopted; the retired `currency` wire field replaced by `asset` on three write paths and three read paths; the `upstream` provider error now decodes. See the corrected Epic-1 finding above.
  2. `docs(entities)` ×2 — the fused `writeJSON`/`reconcileBodyLedgerID` doc comments restored, and the three tracer create-status comments corrected (the generated parsers now fill `JSON201`; the facades' any-2xx gate is unchanged, so the accepted status set did not move).
  3. `feat(entities)!` — **alias service deleted.** The server serves no alias route at any version; the resource was renamed to instruments and is v2-only, already covered. Took the `crm` base-URL key, `crmHeaders` and `entities/crm_shared.go` with it. `models/alias.go` became `models/crm_shared_types.go` — the shared CRM types (`BankingDetails`, `RegulatoryFields`, `RelatedParty`) survive because instruments and composition use them.
  4. `feat(entities)!` — **balances (18 methods) and operations (5) migrated onto the generated client**, net −4507 lines. Wire paths preserved exactly; the root-level pipeline test (`legacy_v1_paths_test.go` → `balance_operation_wire_test.go`, still built from `WithBaseURL` against a live server, never an injected base-URL map) went from 6 rows asserting path to 13 rows asserting method + path + query, plus three behavioural guards. Killed the hand-rolled HTTP, `serviceEntity`, `legacyV1BaseURL` and `Entity.baseURLs`.
  5. `refactor(config)!` — the phantom `transaction` service URL removed (see findings).
  6. `feat(entities)!` — **version groups.** `client.V1.<Service>` (14 members) and `client.V2.<Service>` (9 members) for the Ledger plane; the flat ledger accessors are gone. Tracer accessors stay flat. The groups are struct VALUES, not pointers, so a zero-value `Entity` stays guardable — with pointer groups the idiomatic `e != nil && e.V1.Accounts != nil` check panics one level below what the caller can see. `entities/version_groups_test.go` pins both that property and each resource's group membership.
- [ ] Epic 3 — **inputs from Epic 2:**
  - Every accessor is now a concrete facade pointer; no interface indirection remains anywhere, so adding a V2 twin is a pure addition to `V2Services` + a `newV2Services` line.
  - `V1.Balances` and `V1.Operations` are v1-only today, but the generated client ALREADY carries the complete v2 twin for all 13 of their operations (`getAllBalancesV2`, `getAllOperationsByAccountV2`, `updateOperationV2`, …) with no facade. Same for the other dual families.
  - `V1.Accounts.ListBalances` / `ListOperations` / `BalancesAtTimestamp` call the **same three endpoints** as `V1.Balances.ListAccountBalances` / `V1.Operations.ListOperations` / `V1.Balances.GetAccountBalancesHistory`. Both spellings work and both are tested. Whether both survive is a product call, not a code one.
  - `pkg/integrity` now reaches `V1.Balances` through a consumer-side one-method `balancesLister` interface, mirroring its existing `accountsGetter`. That is the pattern any downstream consumer needs now that the facades are unexported concrete types and the generated mocks are gone.
  - The generated params are ground truth per operation — three of the five model-shape corrections above came from reading them instead of a sibling facade's opts.
- [ ] Epic 4 — **doc debt inventory (beyond the prose already scoped):** `docs/mapping/external_apis.md` (lines ~236, 279, 532, 534) and `docs/mapping/internal_apis.md` (~20, 95, 96) still document `BalancesService` / `OperationsService` / `AliasesService`, `BalancesFilters` and `AliasesListOpts`; `docs/examples.md:112` too; `docs/godoc/**` is generated and stale. `README.md` was updated in Epic 2 (accessor tree, quickstart, and the mocks claim — `entities/mocks/` no longer exists) because it carried runnable code that no longer compiled; the surrounding prose in `docs/` is still Epic 4's.
