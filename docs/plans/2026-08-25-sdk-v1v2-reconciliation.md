# SDK v1+v2 endpoint reconciliation with Midaz develop

**Date:** 2026-08-25
**Branch:** `agent/sdk-v1v2-reconciliation` (worktree `/srv/worktrees/sdk-v1v2-reconciliation`, based on develop @ 4143472; rename to `feat/...` before PR)
**Decision (Fred, 2026-08-25):** the SDK serves BOTH Midaz surfaces — v1 (deprecated but alive) and v2 (the real one) — with explicit version mechanics mirroring the server. No single-version pin.

## Diagnosis (verified against .references/midaz @ 07cebd20e, develop)

- Midaz built the full /v2 surface Jun–Aug 2026, deprecated ALL of /v1 (`db56dde04`), and **removed from /v1**: CRM (`cede08b1a`), fees (`109e9c1de`), composition (`804825a93`).
- v2 dropped legacy transaction creates (`7150848bd`) — creation is `POST /v2/transactions/direct|hold`, plus `block|unblock`, all top-level (not org/ledger-scoped in the URL).
- v2 dropped asset-rates (`c781f6a97`) — asset-rates is v1-only.
- Billing family moved org-scope → **ledger-scope** in v2 (`/v2/organizations/{org}/ledgers/{ledger}/billing-packages|packages|estimates|billing/calculate`).
- SDK today pins `/v1` on every base URL (`entities/entity.go:normalizeServiceURL`, `config.DefaultLedgerAPIVersionPath`). Consequence: holders, instruments, encryption, billing-packages, packages, estimates, composition, protection/audit **404 against Midaz develop**.
- SDK bugs found on the way: `entities/operations.go` PATCHes `.../accounts/{id}/operations/{opId}` — that path exists on NO server version (server: `.../transactions/{txId}/operations/{opId}`). `entities/aliases.go` calls `/aliases`, `/holders/{id}/aliases` — routes that no longer exist anywhere (renamed to instruments, v2-only).
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

- **Tracer spec HAS drifted** — the diagnosis line "SDK spec == server spec, path-for-path" holds for PATHS only. Schemas diverge: the server renamed `currency` → `asset` in three limit/usage schemas, added an RFC 9457 `upstream` error extension member (+ `Upstream` schema), and returns **201 Created** where the SDK spec still says 200 on three operations. Left untouched in Epic 1 (semantic, not cosmetic, and outside a ledger-scoped epic). Needs its own commit: adopt the server tracer spec, regen, fix the `Currency` field rename (3 refs in `internal/gentracer`) and the 200/201 gates.
- `WithBaseURL` and the environment defaults fan one base URL out to both planes; each fan-out now has to re-shape the tracer copy (`tracerURLFromSharedOrigin`). Any new fan-out path must do the same or tracer calls land one segment short.
- `CreateOrganization` lost its OAS `Authorization` param (now a security scheme). It was always passed empty — the auth round tripper sets the header — so this is a wire no-op.
- Only `CreateOrganization` drifted among the v1 operations; every other v1 signature is byte-identical, so v1 wire behavior is preserved.
- The `ledgerId` query filter is gone from `PackagesFilters` / `BillingPackagesFilters` (the ledger is a path segment on v2). Public model change already landed.
- `entities/aliases.go`, `balances.go`, `operations.go` are untouched and still hand-rolled; they read the `crm`/`transaction` base-URL keys, which are now bare, so they no longer double-version. Their migration/deletion stays Epic 2.

## Risks
- v2 request/response schema divergences beyond paths (typed fields) surface during Epic 3 — the generated types are ground truth, not the v1 siblings (see plane-clients memory).
- `/v2/transactions/direct|hold` carry org/ledger in the body — contract to be read from the generated types, not assumed.
- Release tooling: v5 module path already handled (#239); no module bump here.

## Status
- [x] Diagnosis + decision
- [x] Epic 1 — **landed** (`ad3e193`). Server spec adopted; the generated client now carries both surfaces (v1 ids bare, v2 ids `*V2`). Ledger base URL is bare, tracer base keeps `/v1` (verified against the server's Fiber `/v1` group mount). The families Midaz removed from /v1 — holders, instruments, encryption, composition, protection/audit — are rewired onto their V2 operations; the billing family (billing-packages, packages, estimates, billing/calculate) is rescoped to ledger and its facades take a `ledgerID`. Gates green: `go build`, `check-codegen-drift`, `make test`, `make lint` (0 issues).
- [ ] Epic 2
- [ ] Epic 3
- [ ] Epic 4
