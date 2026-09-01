# SDK v6 — midaz v4 stable cut — Mini Plan

> **For implementers:** one-phase plan. Epics are parallel streams: dispatch every
> epic whose dependencies are met, at the same time, one agent per epic, same
> branch. All work lands in a single PR.
>
> NOTA (wave 2, updated after Epic 1.1 landed): the tree is GREEN after 1.1 —
> the fee/billing facades build bodies from `models.*` types, so removing the
> generated fields broke no compile. The break is a SILENT wire incompatibility:
> the SDK still marshals `ledgerId` into fee/billing bodies and every existing
> test passes, because tests assert against SDK-owned mocks. The wire-shape
> assertions in Tasks 1.2.1/1.2.2/1.2.3 are therefore load-bearing — they are
> the only detection for the bug this plan fixes. A wave-2 agent whose
> verification fails ONLY inside sibling-owned files re-runs after the wave
> completes; the orchestrator confirms all wave-2 verifications before Epic 1.5.
> Wave-2 agents do NOT run `git commit` — one shared worktree, concurrent
> commits race; the orchestrator commits per epic after the wave.

**Goal:** Align the SDK with the midaz v4.0.0-rc.2 contract and release it as v6.0.0; v5.x stays as the line paired with the pre-RC2 contract.
**Scope:** `api/`, `internal/genledger/`, `models/`, `entities/`, `examples/mass-demo-generator/`, `contract/`, `go.mod`, `README.md`, `docs/mapping/`.

**Contract drift being fixed (assessed 2026-09-01, `.references/midaz` @ v4.0.0-rc.2):**
- Server removed `ledgerId` from fee/billing request bodies (create package, create billing package, estimate, calculate). A body still sending it is rejected with 400 (unknown field, `additionalProperties: false`). The SDK always sends it → the whole fee/billing write surface is dead against midaz v4 stable.
- `?ledgerId=` refused on the two package listings (error 0235). Not publicly exposed by the SDK; the generated param slot disappears on regen.
- `listAccountsByHolderV2` now officially declares `ledger_id` as an optional query param (midaz#2405 delivered) — the SDK's hand-injected workaround retires.
- Additive: v2 account responses carry `holderId`/`holderCheckSkipped` (schema `AccountV2`); v2 transaction legs accept optional `description` (max 256). Tracer spec unchanged.

## Streams

| Epic | Delivers | Depends on | Files |
|------|----------|------------|-------|
| 1.1 | RC2 spec regenerated + module path `/v6` repo-wide, build green | none | `api/ledger.openapi.yaml`, `internal/genledger/ledger.gen.go`, `go.mod`, `contract/go.mod`, `contract/drift_test.go`, every `.go` import block, `README.md`, `docs/**` (path refs only) |
| 1.2 | fee/billing writes stop sending `ledgerId`; inputs, constructors, facades, tests updated | 1.1 | `models/fee_package.go`, `models/fee_estimate.go`, `models/billing_package.go`, `models/billing_calculate.go`, their `*_test.go`, `entities/fee_packages_facade.go`, `entities/fee_estimate_facade.go`, `entities/billing_packages_facade.go`, `entities/billing_calculate_facade.go`, their `*_test.go` |
| 1.3 | holder-accounts listing uses the native generated `ledger_id` param, hand-injection deleted | 1.1 | `entities/instruments_facade.go`, `entities/instruments_facade_test.go` |
| 1.4 | v2 responses expose holder fields; v2 legs accept `description` | 1.1 | `models/account.go`, `models/transaction_v2.go`, `models/transaction_v2_test.go`, `models/transaction_v2_validation_test.go`, `v2_surface_wire_test.go`, `v2_transaction_create_wire_test.go` |
| 1.5 | dead helper deleted, examples/docs swept, `make ci` green, repo-wide absence checks | 1.2, 1.3, 1.4 | `entities/organizations_facade.go`, `examples/mass-demo-generator/v2_phases.go`, `docs/mapping/external_apis.md`, `README.md` (fee/billing snippets) |

## Contracts

Frozen before dispatch. An agent MUST NOT change these; if one does not fit, stop and report.

**Module path (every import, both go.mod files):**
```
github.com/LerianStudio/midaz-sdk-golang/v6
```

**Input structs — `LedgerID` field REMOVED from exactly these four (update inputs and response models keep theirs):**
- `models.CreatePackageInput` (models/fee_package.go)
- `models.FeeEstimateInput` (models/fee_estimate.go)
- `models.CreateBillingPackageInput` (models/billing_package.go)
- `models.BillingCalculateInput` (models/billing_calculate.go)

**Constructor signatures after the change (ledgerID parameter dropped, order otherwise preserved):**
```go
func NewCreatePackageInput(feeGroupLabel, minAmount, maxAmount string, fees map[string]Fee) *CreatePackageInput
func NewFeeEstimateInput(packageID string, send *SendInput) *FeeEstimateInput
func NewBillingCalculateInput(period string) *BillingCalculateInput
func NewCreateVolumeBillingPackageInput(label, assetCode, debitAlias, creditAlias string) *CreateBillingPackageInput
func NewCreateMaintenanceBillingPackageInput(label, assetCode, feeAmount, maintenanceCreditAccount string) *CreateBillingPackageInput
```

**Facade signatures unchanged.** `ledgerID` stays a required parameter on every fee/billing facade method (it scopes the URL path) and on `Instruments.ListAccountsByHolder*` (org-wide listing deliberately not exposed — YAGNI until someone asks).

**Shared helper:** `reconcileBodyLedgerID` (entities/organizations_facade.go) — epics 1.2 and 1.3 only REMOVE their call sites. Epic 1.5 deletes the function. Nobody else touches entities/organizations_facade.go.

**Additive fields:**
```go
// models.Account — decode-only; populated by /v2 endpoints, absent on /v1 responses.
HolderID           *string `json:"holderId,omitempty"`
HolderCheckSkipped bool    `json:"holderCheckSkipped,omitempty"`

// models.TransactionV2Leg — optional, server max length 256.
Description string `json:"description,omitempty"`
```

---

### Epic 1.1: Foundation — RC2 spec regen + v6 module path

**Goal:** The repo builds on module path `/v6` with clients generated from the RC2 ledger spec.
**Scope:** Spec copy, codegen, mechanical import sweep.
**Dependencies:** none
**Done when:** `go build ./...` and `scripts/check-codegen-drift.sh` pass; no `midaz-sdk-golang/v5` string remains outside `.references/` and CHANGELOG history.
**Status:** Done

#### Task 1.1.1: Regenerate the ledger client from the RC2 spec
- [x] Done

**Context:** SDK specs live in `api/*.openapi.yaml`; `.gen.go` is committed and drift-checked. Tracer spec is byte-identical to RC2 — do not touch it.

**Implementation vision:** Copy `.references/midaz/components/ledger/api/openapi.huma.yaml` over `api/ledger.openapi.yaml`. Run `scripts/generate-clients.sh`. Expect in the regenerated `internal/genledger/ledger.gen.go`: `AccountV2` schema, `LedgerId` param on `ListAccountsByHolderV2Params`, `ledgerId` gone from `FeeCreatePackageInput`/`FeeEstimate`/`FeeBillingCalculateRequest` and from `GetAllBillingPackagesParams`/fee-packages list params, create-billing-package body typed as `FeeCreateBillingPackageInput`. The repo will NOT compile after this task alone (facades reference removed generated fields) — that is expected; Task 1.1.2 verification is the compile gate for what THIS epic owns, and wave-2 epics fix the facades.

**Files:**
- Modify: `api/ledger.openapi.yaml`, `internal/genledger/ledger.gen.go`

**Verification:** `scripts/check-codegen-drift.sh` passes (spec ↔ gen in sync). `grep -c ledgerId internal/genledger/ledger.gen.go` drops versus HEAD.

**Done when:** Drift check green and the expected generated symbols listed above exist/are absent.

#### Task 1.1.2: Bump module path to /v6 repo-wide
- [x] Done

**Context:** Release tooling tags majors WITHOUT bumping go.mod (recorded v4 gap) — the path bump must be in source. `contract/` is a nested module with its own go.mod, a require on the SDK, and a `replace => ../`.

**Implementation vision:** `go.mod` module line → `/v6`. Sed every `.go`, `README.md`, and `docs/**` occurrence of `midaz-sdk-golang/v5` → `/v6` (exclude `.references/` and `CHANGELOG.md`). In `contract/go.mod`: module path suffix, require line → `github.com/LerianStudio/midaz-sdk-golang/v6 v6.0.0` (replace directive keeps it resolving locally); `go mod tidy` inside `contract/`. Fix compile errors ONLY where caused by the path sweep — leave facade breakage from Task 1.1.1 for wave 2; if `go build ./...` still fails on fee/billing facades referencing removed generated fields, that is the expected wave-2 surface: verify instead that failures are confined to those files.

**Files:**
- Modify: `go.mod`, `contract/go.mod`, `contract/drift_test.go`, every `.go` file importing the module, `README.md`, `docs/**`

**Verification:** `rg 'midaz-sdk-golang/v5' --glob '!.references/**' --glob '!CHANGELOG.md'` returns nothing. `go vet ./models/...` compiles (models does not depend on genledger fee params).

**Done when:** Zero `/v5` references outside exclusions; remaining build failures (if any) are only the fee/billing facade files owned by Epic 1.2.

---

### Epic 1.2: Fees + billing — remove ledgerId from request bodies

**Goal:** Every fee/billing write serializes a body the RC2 server accepts; the path is the sole ledger authority.
**Scope:** Four input models, four facades, their tests.
**Dependencies:** 1.1
**Done when:** `go test ./models/ ./entities/ -run 'Fee|Billing'` passes; no fee/billing request-body wire assertion contains `ledgerId`.
**Status:** Done

#### Task 1.2.1: Strip LedgerID from the fee input models
- [x] Done

**Context:** `CreatePackageInput` and `FeeEstimateInput` carry `LedgerID string json:"ledgerId"` (required in Validate, set by constructors). RC2 rejects the field with 400.

**Implementation vision:** Remove the field, its Validate clause, and the constructor parameter per the frozen signatures in `## Contracts`. Update doc comments that name the server DTO behavior (path is sole authority now). Update the model tests: drop required-ledgerId assertions, add a wire-shape assertion that the marshaled create/estimate JSON contains no `ledgerId` key.

**Files:**
- Modify: `models/fee_package.go`, `models/fee_estimate.go`
- Test: `models/fee_package_test.go`, `models/fee_estimate_test.go`

**Verification:** `go test ./models/ -run 'FeePackage|FeeEstimate'`

**Done when:** Tests pass and marshaling either input produces JSON without `ledgerId`.

#### Task 1.2.2: Strip LedgerID from the billing input models
- [x] Done

**Context:** Same change as 1.2.1 for `CreateBillingPackageInput` (two constructors) and `BillingCalculateInput` (the SDK auto-filled the body field from the path — that behavior dies with the field).

**Implementation vision:** Mirror 1.2.1. `NewBillingCalculateInput(period)` per contract. Wire-shape assertions: no `ledgerId` key in create/calculate bodies.

**Files:**
- Modify: `models/billing_package.go`, `models/billing_calculate.go`
- Test: `models/billing_package_test.go`, `models/billing_calculate_test.go`

**Verification:** `go test ./models/ -run 'BillingPackage|BillingCalculate'`

**Done when:** Tests pass and marshaled bodies carry no `ledgerId`.

#### Task 1.2.3: Drop body-ledger reconciliation from the four facades
- [x] Done

**Context:** `fee_packages`, `fee_estimate`, `billing_packages`, `billing_calculate` facades each clone the input and call `reconcileBodyLedgerID(operation, ledgerID, &reconciled.LedgerID)`. The field no longer exists. The create-billing-package request body type in genledger changed to the new input schema — adjust any typed reference. ATENÇÃO: do NOT delete `reconcileBodyLedgerID` itself (entities/organizations_facade.go belongs to Epic 1.5).

**Implementation vision:** Remove the reconcile call sites and the now-pointless input cloning where reconciliation was its only purpose. Keep `requirePathIDs` guards — path still needs orgID+ledgerID. Update facade doc comments (drop the "body ledgerId inherits the path" contract text). Update facade tests: remove mismatch-rejection cases (server retired code 0234), keep path-scoping and wire tests, assert request bodies carry no `ledgerId`. If list-opts mapping references a removed generated `LedgerId` param slot, delete the mapping.

**Files:**
- Modify: `entities/fee_packages_facade.go`, `entities/fee_estimate_facade.go`, `entities/billing_packages_facade.go`, `entities/billing_calculate_facade.go`
- Test: `entities/fee_packages_facade_test.go`, `entities/fee_estimate_facade_test.go`, `entities/billing_packages_facade_test.go`, `entities/billing_calculate_facade_test.go`

**Verification:** `go test ./entities/ -run 'Fee|Billing'`

**Done when:** Tests pass; `rg 'reconcileBodyLedgerID' entities/ --glob '!organizations_facade.go'` returns nothing.

---

### Epic 1.3: Holder accounts — native ledger_id param

**Goal:** The holder-accounts listing fills the now-official `ledger_id` query param through the generated slot instead of hand injection.
**Scope:** One facade and its test.
**Dependencies:** 1.1
**Done when:** `go test ./entities/ -run Instruments` passes with zero request-editor injection of `ledger_id`.
**Status:** Done

#### Task 1.3.1: Replace setQueryParam injection with the generated param slot
- [x] Done

**Context:** `listAccountsCursor` injects `ledger_id` via `setQueryParam` because the old spec did not declare the param (documented in the in-file comment as temporary: "when the contract catches up this moves into listAccountsByHolderParams and the editor goes away"). Post-regen, `ListAccountsByHolderV2Params` has the slot. Public signatures stay unchanged; `ledgerID` remains required (see `## Contracts`).

**Implementation vision:** Fill the new params field in `listAccountsByHolderParams`, delete the `ledger_id` editor and its contract-drift comment block. The cursor editor stays only if the generated params still lack a cursor slot — check the regenerated params first and prefer a native slot there too. Update tests asserting the query string.

**Files:**
- Modify: `entities/instruments_facade.go`
- Test: `entities/instruments_facade_test.go`

**Verification:** `go test ./entities/ -run Instruments`

**Done when:** Requests carry `ledger_id` via params; `rg 'setQueryParam\("ledger_id"' entities/` returns nothing.

---

### Epic 1.4: v4 additive surface — holder fields + leg description

**Goal:** SDK users read `holderId`/`holderCheckSkipped` from v2 account responses and can set a per-leg `description` on v2 transactions.
**Scope:** Two models, their tests, two root wire tests.
**Dependencies:** 1.1
**Done when:** `go test ./models/ -run 'Account|TransactionV2'` and `go test . -run 'V2'` pass.
**Status:** Done — `go test ./models/ -run 'Account|TransactionV2'` and `go test . -run 'V2Surface|V2Transaction'` both green in the worktree (re-run after Epic 1.2 landed; during the wave the `models` test build was red on sibling files only). `go build ./...` still fails ONLY in `examples/mass-demo-generator/v2_phases.go` on the four changed fee/billing constructor signatures — that is Task 1.5.2's file, untouched here.

#### Task 1.4.1: Expose holder fields on models.Account
- [x] Done

**Context:** RC2 splits the response schema: `/v1` withholds holder keys, `/v2` answers with `AccountV2` carrying them. `models.Account` is shared by both facades and today has no holder fields.

**Implementation vision:** Add the two fields per `## Contracts` with a doc comment stating: populated only by `/v2` endpoints; always empty on `/v1` responses by server contract. Decode-only — create/update inputs gain nothing. Extend the v2 wire test to assert both fields decode from an `AccountV2`-shaped payload and stay zero on a v1-shaped payload.

**Files:**
- Modify: `models/account.go`, `v2_surface_wire_test.go`

**Verification:** `go test ./models/ -run Account && go test . -run V2Surface`

**Done when:** Both wire assertions pass.

#### Task 1.4.2: Add optional Description to TransactionV2Leg
- [x] Done

**Context:** RC2's `V2LegInput` gained optional `description` (maxLength 256). SDK leg type: `models.TransactionV2Leg` (models/transaction_v2.go:254).

**Implementation vision:** Add the field per `## Contracts`. If `TransactionV2Leg` has a Validate path, enforce the 256 bound there with a field error, matching the file's existing validation idiom; otherwise leave enforcement to the server. Extend the create wire test: a leg with description serializes it, a leg without omits the key.

**Files:**
- Modify: `models/transaction_v2.go`, `v2_transaction_create_wire_test.go`
- Test: `models/transaction_v2_test.go`, `models/transaction_v2_validation_test.go`

**Verification:** `go test ./models/ -run TransactionV2 && go test . -run V2Transaction`

**Done when:** Wire tests prove presence/omission of the key.

---

### Epic 1.5: Integration — dead code, examples, docs, full CI

**Goal:** One coherent v6 tree: no orphaned helper, examples compile against the new signatures, docs match, `make ci` green.
**Scope:** Repo-wide verification plus the files below.
**Dependencies:** 1.2, 1.3, 1.4
**Done when:** `make ci` passes; repo-wide absence checks below hold.
**Status:** Done — `make ci` green end to end (lint 0 issues on a cleared cache, gosec clean, coverage 81.6% over the 80.0% threshold, codegen drift clean, all examples build, contract module tests pass). All three repo-wide assertions hold.

#### Task 1.5.1: Delete reconcileBodyLedgerID and its tests
- [x] Done

**Context:** After 1.2 the helper in `entities/organizations_facade.go` has zero callers. Its behavior tests (mismatch → error, empty → inherit) live in the facade test files 1.2 already rewrote; any remaining direct test of the helper dies with it.

**Implementation vision:** Delete the function and any test exercising it directly. Compiler + `rg reconcileBodyLedgerID` (zero hits repo-wide) are the arbiter.

**Files:**
- Modify: `entities/organizations_facade.go`

**Verification:** `go build ./... && rg -c 'reconcileBodyLedgerID' || true` → no matches.

**Done when:** Zero references repo-wide, build green.

#### Task 1.5.2: Sweep examples and docs to the new signatures
- [x] Done

**Context:** `examples/mass-demo-generator/v2_phases.go` builds fee/billing inputs through the constructors whose signatures changed (frozen in `## Contracts`). `docs/mapping/external_apis.md` and README fee/billing snippets document the old inputs.

**Implementation vision:** Update constructor calls in the example (drop the ledgerID argument). Update `docs/mapping/external_apis.md` entries for the four inputs and the holder-accounts param note; add the two additive fields. Refresh README fee/billing snippets if they show `LedgerID`. Do not run `make demo-data` (needs a live server); compile gate suffices.

**Files:**
- Modify: `examples/mass-demo-generator/v2_phases.go`, `docs/mapping/external_apis.md`, `README.md`

**Verification:** `go build ./examples/... && rg 'LedgerID' examples/ | rg -i 'fee|billing|estimate|calculate'` → no matches.

**Done when:** Examples compile; no fee/billing LedgerID usage remains in examples or docs.

#### Task 1.5.3: Full pipeline + repo-wide contract assertions
- [x] Done

**Context:** Absence claims belong here (siblings done writing). CI note: golangci warm cache can hide findings — clear `~/.cache/golangci-lint` before linting if results look suspiciously clean.

**Implementation vision:** Run `make ci`. Then the repo-wide checks: (a) `rg '"ledgerId"' models/` hits only response/decode models (`FeePackage`, `BillingPackage`, and any other server-owned response struct), never a create/estimate/calculate input; (b) `rg 'midaz-sdk-golang/v5' --glob '!.references/**' --glob '!CHANGELOG.md' --glob '!PLAN.md' --glob '!docs/plans/**'` → nothing (PLAN.md records the v5 path as a historical fact; this plan file quotes the check itself); (c) `contract/` module tests pass (`cd contract && go test ./...`). Fix what fails, rerun.

**Files:**
- — (verification only)

**Verification:** `make ci` green; the three assertions above hold.

**Done when:** All green in final state, not predicted.
