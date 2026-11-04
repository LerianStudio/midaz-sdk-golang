# CRM probe state mini plan

> **For implementers:** one-phase plan, one writing lane and one PR. Follow the existing public SDK contracts. The fleet plan owns the later release and consumer bump.

**Goal:** keep CRM credentials consistent after concurrent discovery and let the consumer invalidate a bearer refused during initial discovery.
**Scope:** `crm_reader.go`, focused regression tests and `docs/server-version.md`. No consumer, gateway, licence, public signature, configuration or release-policy changes.
**Baseline:** `ea670ff598146d948e66e994c2a1fb7ab8fa3585`; relevant source and go.mod are identical to published v6.5.2.

## Streams

| Epic | Delivers | Depends on | Files |
|------|----------|------------|-------|
| 1.1 | Ordered discovery notifications and recoverable initial 401 | none | `crm_reader.go`, `crm_reader_probe_test.go`, `docs/server-version.md` |

### Epic 1.1: Preserve authenticated discovery state

**Goal:** a late failed discovery cannot erase a completed v2 credential selection; an initial 401 retains its real status.
**Scope:** CRMReader probe completion and local HTTP regressions.
**Dependencies:** none
**Done when:** both regressions fail before their fixes and pass afterward; existing contracts and SDK gates pass.
**Status:** Done

#### Task 1.1.1: Serialize probe result publication and notification

- [x] Done

**Evidence:** `go test -race -count=1 -timeout=30s -run '^TestCRMReaderProbe_NotificationsRemainOrdered$' .` failed before the fix with `successful notification overtook failed callback`, then passed. Focused CRM tests also cover cached-read reentrancy and credential-isolated probes.

**Context:** `CRMReader.probe` takes a resolution snapshot after HTTP, publishes a result and invokes OnResolve without ordering concurrent credential-isolated probes. A locally reproduced late empty callback overwrites a completed v2 selection in the consumer, while the SDK retains v2 paths.

**Implementation vision:** serialize only the post-HTTP section that reads the last resolution, commits it and invokes its callback. Preserve credential-isolated first probes and shared refresh. Never hold the new lock during HTTP or authorization. Preserve cache-before-callback publication and the lock-free cache fast path, including a callback that re-enters a cached read. Do not drop failed-discovery callbacks, change callback signatures or add a consumer workaround. The older cache-before-callback observation window is outside this fix.

**Files:**
- Modify: `crm_reader.go`
- Create: `crm_reader_probe_test.go`
- Test: `crm_reader_probe_test.go`, existing `crm_reader_test.go`

**Verification:** run the focused new ordering regression with `go test -race -count=1 -timeout=30s -run '^TestCRMReaderProbe' .` before and after the fix. Use channels and bounded waits, not scheduler guesses, synthetic CPU load or an unbounded test. Then exercise the existing credential-isolation and authorization tests.

**Done when:** a failed notification cannot overlap a later successful result commit and leave the external API selection empty; the next v2 read uses the ledger credential. Existing callback and singleflight behavior remains supported.

#### Task 1.1.2: Preserve an initial discovery authentication refusal

- [x] Done

**Evidence:** `go test -race -count=1 -timeout=30s -run '^TestCRMReaderProbe_InitialUnauthorizedPreservesStatus$' .` failed before the fix because actual status was absent, then `go test -race -count=1 -timeout=30s -run '^TestCRMReader' .` passed, including discovery 404 and holder 404.

**Context:** `CRMReader.probe` deliberately masks initial discovery failures to avoid interpreting a /version 404 as account absence. The newly authenticated probe also masks 401. A local test with the consumer's real token provider reproduced one OAuth exchange and repeated rejected probes without invalidation.

**Implementation vision:** preserve the original typed error only for an initial actual HTTP 401, before generic network-error wrapping. Retain retryable handling for other initial discovery failures and the existing last-API behavior on failed refresh. Keep discovery 404 distinct from holder/account 404. No consumer changes, extra retry, token cache, fallback or generalized error mapping.

**Files:**
- Modify: `crm_reader.go`, `crm_reader_probe_test.go`, `docs/server-version.md`
- Test: `crm_reader_probe_test.go`, existing discovery/not-found tests

**Verification:** add a minimal desired-behavior regression; prove RED then GREEN with `-race -count=1 -timeout=30s`. Verify ActualHTTPStatus reports 401, authentication classification survives, discovery 404 is not not-found and actual holder 404 remains not-found. Reuse existing cases instead of copying an entire diagnostic harness.

**Done when:** a caller can identify and invalidate the refused initial bearer without misclassifying discovery failure as missing client data.

#### Task 1.1.3: Verify the SDK and consumer boundary

- [x] Done

**Review:** Five independent reviews and the final contrarian passed with no Critical, High or Medium findings. The reentrancy-test Low was fixed using one deadline for both reads; focused CRM race tests and diff-check passed afterward under an owned scope with `memory.max=68719476736`. The contrarian's documentation Low was fixed by documenting the initial-401 exception and credential-isolated probing in `docs/server-version.md`.

**Evidence:** SDK `go test -race -count=1 -timeout=180s ./...`, `go vet ./...`, `go build ./...`, and `make lint` passed; pinned golangci-lint 2.14.0 used an isolated temporary lint cache. Consumer credential routing, invalidation and re-exchange tests passed with `go test -mod=mod -modfile=<temporary modfile> -tags=unit -race -count=1 -timeout=120s -run 'TestCRMAdapter_(CredentialFollowsTheResolvedAPI|V2UnauthorizedInvalidatesTheLedgerCredential)|TestLookupSenderAccount_CRMUnauthorizedReexchangesOnTheNextCall' ./internal/bank_transfer/adapters/crm`; the temporary modfile replaced the SDK with this worktree. Heavy gates ran under an owned `systemd-run --user --scope` with `memory.max=68719476736` confirmed inside. Initial scope launch aborted before gates because systemd expanded shell variables; rerun with `--expand-environment=no` passed. No consumer files were edited.

**Context:** the consumer worktree `/srv/worktrees/bump-midaz-sdk` changes only its module pin and checksums. It must remain untouched by this lane. The published v6.5.2 diagnostic proved both failures with local HTTP only.

**Implementation vision:** run SDK full gates, then verify the existing consumer credential tests using a temporary modfile/replace or overlay outside both repositories. Do not modify consumer manifests or tests, use real services, read secrets, invoke Docker/Testcontainers, install tools or generate load. Bound every long process; use the shared Go cache and an owned memory-limited scope for heavy work. Add no suppressions. Update this plan with actual evidence and report production/test/docs line deltas; do not commit or push.

**Files:**
- Modify: `docs/plans/2026-11-04-crm-probe-state-mini.md` for evidence only
- Test: SDK root/package suites; consumer `internal/bank_transfer/adapters/crm`

**Verification:** `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, repository lint with isolated temporary lint cache and no installation, `git diff --check`; consumer existing credential-routing/invalidation tests with `-tags=unit -race -count=1`. If the SDK suite uses untagged tests, retain its established commands. Record unavailable tools or failed gates rather than claiming success.

**Done when:** narrow regressions and wider gates pass; only the planned production, test and documentation files changed. A different agent reviews the final diff before integration. Stable publication and the later consumer bump remain separate fleet-plan work.
