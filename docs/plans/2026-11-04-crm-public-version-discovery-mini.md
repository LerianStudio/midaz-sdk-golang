# CRM public version discovery mini plan

> One implementation writer in this worktree. Independent verification follows; no live services or containers.

**Goal:** Resolve a public CRM API before requiring credentials whose selection depends on that API, while preserving authenticated gateways.
**Scope:** CRM version discovery, existing callbacks/cache, local regression tests and its documentation.
**Baseline:** develop 554749966353b7ccbc7c41bba40721c485ae31dc, including stable SDK 6.5.3.

## Streams

| Epic | Delivers | Depends on | Files |
|------|----------|------------|-------|
| 1.1 | Compatible public discovery and authenticated challenge | none | crm_reader.go, crm_reader_test.go, crm_reader_public_probe_test.go, docs/server-version.md |
| 1.2 | Independent SDK and consumer compatibility verification | 1.1 | Verification only |

### Epic 1.1: Resolve metadata without an unused credential dependency

**Goal:** Public metadata works when the unused CRM credential fails or stalls; a protected gateway still receives authentication.
**Scope:** Existing HTTP version helper, singleflight partitioning, resolution commit and data authorization.
**Dependencies:** none
**Done when:** Focused regressions fail before the patch and pass afterward without weakening data authorization or callbacks.
**Status:** Done

#### Task 1.1.1: Preserve public and protected discovery

- [x] Done

**Context:** Published consumer v3.5.0 with SDK 6.3.0 discovers a public ledger before selecting its valid ledger provider. SDK 6.5.3 instead calls the unused CRM provider first. External tests using real constructors reproduced failure on CRM token 401/503 while the ledger provider remained valid. A provider timeout has the same dependency problem and needs a bounded regression.

**Implementation vision:** Try public version metadata first. Only an actual HTTP 401 or 403 refusal requires the configured Authorization callback and a credential-isolated authenticated probe. Obtain credentials with the original caller context; send only their header on the detached bounded metadata request. Do not report an intermediate challenge as a final resolution failure. Reuse readServerVersion, singleflight and the existing ordered resolution machinery. Keep cached reads/background refresh, final typed 401, final 403 classification, masked discovery 404 and callback retry/convergence. A completed credential-refused refresh must not defer another credential's refresh; a cached-error callback reading the API must not restart its active refresh. Retain backoff for service and transport failures. Continue evaluating configured data authorization after resolution; an authorization failure never sends a data request. No consumer credential fallback/tracker, anonymous data, external gateway change, new option/interface/dependency or version change.

**Files:**
- Modify: crm_reader.go, crm_reader_test.go, docs/server-version.md
- Create/Test: crm_reader_public_probe_test.go

**Verification:** Focused local HTTP race tests cover public v2 with unavailable/slow unused CRM credentials, public v1 with required CRM failure, authenticated gateway challenge, no configured bearer, final refusal, cancellation and existing concurrent credential/callback cases. A missing tenant may reach public metadata as in 6.3.0, but never data or Secrets Manager. Keep test barriers deterministic and bound waits; no scheduler sleeps or generated load. Adapt the old zero-probe assertion only to state the corrected metadata-versus-data contract.

**Done when:** Public discovery no longer waits for an unused provider; protected discovery and data still enforce their existing credentials, errors and isolation.

### Epic 1.2: Verify the full integration

**Goal:** Prove the final SDK tree and real consumer baseline behavior without publishing or changing the candidate consumer.
**Scope:** SDK-wide local gates and the existing external consumer reproduction.
**Dependencies:** 1.1
**Done when:** All local gates pass and the previously working consumer configuration works with the corrected SDK.
**Status:** Done

#### Task 1.2.1: Run independent gates and the constructor-based consumer proof

- [x] Done

**Context:** Consumer /srv/worktrees/bump-midaz-sdk remains uncommitted at published SDK 6.5.3. The immutable original reproduction is in the session scratch crm-contract-repro directory; its final authoritative results preserve the old/new failure evidence.

**Implementation vision:** An independent verifier checks the actual diff, runs full SDK race/vet/build and repository pinned lint, then copies the reproduction to a separate scratch directory and points only that temporary module at this SDK tree. Preserve the original reproduction and consumer files. Require real consumer reads to pass with public v2 plus failing or slow unused CRM providers, while ledger data refusal, missing-tenant isolation and protected version 401 semantics remain intact. Run heavy work only in owned verified 64 GiB scopes with shared Go caches. No production APIs, secrets, login, Docker, installs, synthetic CPU load, commit, push or release.

**Files:**
- Test: existing SDK suites and a scratch copy of crm-contract-repro/contract_test.go
- Modify: none

**Verification:** Record exact commands, raw exits, source delta and caller-context/credential assertions. Any unresolved regression blocks the ready PR. Later independent review, stable publication and the consumer pin remain owned by the fleet plan.

**Done when:** Focused and full SDK gates and the compatible consumer proof pass without source workarounds or suppressed failures.
