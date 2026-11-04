# CRM credential selection mini plan

> **For implementers:** one writing lane in this worktree. Independent review follows implementation. All changes land in one SDK PR.

**Goal:** Preserve authenticated version probing while choosing the data credential after CRM API discovery, as supported before v6.5.1.
**Scope:** `crm_reader.go`, `crm_reader_test.go`; no consumer workaround or licence change.
**Base:** `develop`, verified at `b526c0f37d0d3b1541a787a86234cc86047ed23f`.
**Branch:** `fix/crm-authorization-after-resolution`.

## Streams

| Epic | Delivers | Depends on | Files |
|------|----------|------------|-------|
| 1.1 | Compatible credential selection with authenticated discovery | none | `crm_reader.go`, `crm_reader_test.go` |

### Epic 1.1: Restore credential selection after discovery

**Goal:** A caller can select ledger credentials in OnResolve and use them on its first v2 data request.
**Scope:** SDK CRM reader request/probe sequencing and boundary tests.
**Dependencies:** none
**Done when:** The regression fails first, then passes; existing credential isolation, cancellation, refresh and routing tests remain green.
**Status:** Done

#### Task 1.1.1: Reproduce the supported dynamic credential callback

- [x] Done

**Context:** `crm_reader.go:174-185` captures Authorization before API discovery. Bank-transfer's existing `activeProvider` selects ledger only after OnResolve reports v2. Its current first-read credential test passes on SDK v6.3.0 but is not proof for v6.5.1.

**Implementation vision:** Extend the existing SDK HTTP-boundary tests. Begin with unresolved API and a CRM credential. OnResolve switches the callback to ledger for v2. Require CRM on the first version probe and ledger on the first v2 data read; v1 keeps CRM. Also cover failure obtaining the post-resolution credential: return the wrapped cause and send no data request. Use no prewarming, real credentials, production endpoints or sleeps that presume concurrency.

**Files:**
- Modify: `crm_reader_test.go`
- Test: `crm_reader_test.go`

**Verification:** Run the new named test with `go test -race -count=1 -run '<new exact test name>' .`; observe an actual wrong credential/failure, not a compile error or zero tests.

**Done when:** Capture RED output against unchanged production code, with the initial authenticated probe observed.

**Evidence:** The new v2 case received the CRM credential instead of ledger; the post-resolution callback error case unexpectedly succeeded. Both failed before production changed, then passed. Bank-transfer's existing first-v2-read assertion independently failed against published v6.5.1 without repository dependency changes.

#### Task 1.1.2: Select data authorization after resolving the API

- [x] Done

**Context:** `get` is the sole caller of private `api`. The SDK's cached API fast path is already present; initial probes use credential-specific singleflight, and refresh failures keep the known API.

**Implementation vision:** Resolve the API before choosing the data Authorization value. Move the authorization lookup needed for probing into the unresolved/expired path of `api`; do not add a second lookup to every cached read. Preserve the authenticated probe, credential-specific initial singleflight key, detached bounded probe, caller cancellation and cached refresh semantics. Preserve OnResolve/publication ordering rather than expanding this fix into a resolver redesign. Keep the public callback and method signatures unchanged. Clarify the callback comment because probe and post-resolution read can require different credentials. Nil callbacks remain anonymous; callback errors abort that operation without fallback.

**Files:**
- Modify: `crm_reader.go`, `crm_reader_test.go`
- Test: `crm_reader_test.go`

**Verification:** New regression GREEN; `go test -race -count=1 -run TestCRMReader .`, then `go test -race -count=1 ./...`, `go vet ./...`, and the installed `golangci-lint run`. SDK tests are untagged; confirm nonzero test execution. Run no Docker or global installs.

**Done when:** The first v2 data query uses ledger, v1 stays on CRM, callback failure sends no data request, and existing probe/isolation tests pass. Independent code, logic, security, pentest and performance reviews plus a contrarian pass precede commit/push/PR.

**Evidence:** Final full SDK race suite, vet and lint passed. Five reviews and the contrarian found no Critical/High; test indirection and a duplicate v1 entry were removed. The existing consumer regression failed against v6.5.1 and passed with this local source. One wider consumer run reported HTTP goleak; isolated old-release, v6.5.1 and local-source rechecks passed, with cause still unproven. No real service or credential was used.
