# Plan: expose transaction metadata filtering on `ListTransactions`

**Status:** Draft for review
**Repo:** `github.com/LerianStudio/midaz-sdk-golang/v5` (branch develop)
**Scope:** `models/transactions_list_opts.go`, `entities/transactions.go` (wiring only), validation, tests, docs. **No new SDK method; no server change.**

---

## 1. Problem

A client cannot look up or list ledger transactions by a business correlation id (e.g. a caller's `transferId`). This blocks the lost-response recovery path: when a `CreateTransaction` HTTP response is lost (timeout / 5xx), the client has no Midaz `transaction.ID` and today has **no way to ask the ledger "did this credit land?"** by anything it actually knows.

Concretely, the SDK's transaction query surface:
- `GetTransaction` is by Midaz `transaction.ID` only (`entities/transactions.go:670-705`) — useless when the id was never received.
- `ListTransactions` filters expose only `AssetCode/Status/Reference/DestinationAccount/SourceAccount/Route` + cursor/date (`models/transactions_list_opts.go:31-50`, `ToQueryParams:61-89`). **No metadata filter.**
- The only correlation the client can stamp on create that survives to the ledger is `Metadata["transferId"]` (metadata IS serialized on create; `models/transaction.go:905-907`).

So the client can WRITE the business id (into metadata) but cannot QUERY by it.

## 2. Root cause — and the dead ends to avoid

The SDK does not expose the server's metadata query parameter. Three tempting "fixes" are dead ends, verified against the server source (`../midaz`, `components/ledger`, develop):

- **`ExternalID` is inert.** `CreateTransactionInput.ExternalID` is `json:"-"` (`models/transaction.go:300`); the wire-body builder `ToLibTransaction` (`models/transaction.go:857-910`) has never emitted it. It was severed SDK-side in `8fc6e8a` (a broad model sweep), not in reaction to a server change. The server has **no `external_id` column on transactions** (operation/FromTo-level only) and cannot filter by it. Do **not** resurrect ExternalID as the lever.
- **`reference` / `code` don't pair.** The list filter `reference` and the create field `code` are different server concepts; the server transaction table has neither a `reference` nor a queryable `code` column — `code` survives only inside the `body` JSON blob, which the list endpoint cannot filter. Do **not** route correlation through them.
- **Idempotency key is not a read handle.** `X-Idempotency` is Redis-backed with a default 5-min TTL; a duplicate create within the window returns the original (replay), but there is **no get-by-idempotency-key endpoint**. Useful as a *secondary* recovery inside the TTL, never a durable lookup.

## 3. Verified server capability (the real lever)

`GET .../transactions?metadata.transferId=<uuid>` is a **working server capability** today (`../midaz/components/ledger`):
- Query parse: `pkg/net/http/httputils.go:153-155` captures any `metadata.*` key into a Mongo filter.
- Dispatch: `transaction_query_handlers.go:75-93` routes to `GetAllMetadataTransactions` when a metadata filter is present.
- Execution: `get_all_metadata_transactions.go:37-62` queries the Mongo metadata collection (`metadata.mongodb.go:332-336`, exact-match on the dotted key), collects `entity_id`s, then joins Postgres `WHERE id = ANY(uuids)`. Cursor-paginated.
- Metadata is also returned on list responses (`get_all_transactions.go:100-102`), enabling client-side confirmation.

**Two constraints the server imposes — must shape the SDK API:**
1. **Single metadata predicate.** The parser assigns a single `bson.M{key: value}`; multiple `metadata.*` params do not AND together (last-wins). The SDK must expose a **single key/value pair**, not a multi-key map that would imply unsupported AND semantics. (Confirm exact multi-key behavior during impl; default to single-pair.)
2. **Unindexed by default.** Only `entity_id` is auto-indexed on the transaction metadata collection (`config.mongo.transaction.go:128-144`); arbitrary metadata keys are collection scans until an operator creates the index via the admin `CreateIndex` API. This is a performance note for the SDK docs + a cross-repo ops follow-up, not an SDK blocker.

## 4. The fix

Expose a single metadata key/value filter on the transactions list, emitted as `metadata.<key>=<value>`.

### 4.1 `models/transactions_list_opts.go`
Add to `TransactionsFilters`:
```go
// MetadataKey / MetadataValue filter transactions by a single metadata field,
// rendered as the query param `metadata.<MetadataKey>=<MetadataValue>`.
// The server honors ONE metadata predicate per request (not AND-combinable),
// so this is a single pair by design. Both must be set together.
MetadataKey   string
MetadataValue string
```
In `ToQueryParams` (after the existing filters):
```go
if o.Filters.MetadataKey != "" && o.Filters.MetadataValue != "" {
    params["metadata."+o.Filters.MetadataKey] = o.Filters.MetadataValue
}
```
**API-shape decision:** explicit `MetadataKey`/`MetadataValue` pair over a `map[string]string`, because the server is single-predicate last-wins — a map would silently mislead callers into expecting multi-key AND. (Alternative considered and rejected: `map[string]string` capped at one entry with validation — more surface, same capability, worse ergonomics.)

### 4.2 `Validate()` (`transactions_list_opts.go`)
Reject the half-set case (one of key/value empty) and validate the key against the server's metadata key rules (reuse `pkg/validation/core` metadata-key validation if present). Keep `ValidateCursorListOpts` as-is.

### 4.3 Wiring
No signature change: `ListTransactions` / `ListTransactionsAll` / `ListTransactionsPages` already take `TransactionsListOpts` and call `ToQueryParams` (`entities/transactions.go:64-84`). The new param flows through automatically.

### 4.4 (Optional) convenience helper
Consider a `WithMetadataFilter(key, value string)` fluent option for parity with the existing `With*` builders. Not required — the struct fields are enough. Decide during review.

## 5. Tests
- **Unit** (`models/transactions_list_opts_test.go`): `ToQueryParams` emits `metadata.transferId=<uuid>` when the pair is set; emits nothing when either side is empty; `Validate` rejects half-set pairs.
- **Contract regression** (`entities/transaction_contract_regression_test.go` style): assert the rendered request query string contains `metadata.transferId=` for a representative filter — locks the wire shape against future drift.
- **Iterator coverage:** a `ListTransactionsAll` test confirming the metadata param is carried across cursor pages.

## 6. Docs
- Update `docs/mapping/external_apis.md` / `internal_apis.md` to document the metadata list filter.
- Godoc on the new fields: the single-predicate constraint AND the **unindexed-by-default performance caveat** — recommend operators create the `metadata.<key>` index via the Midaz admin `CreateIndex` API for hot correlation keys (e.g. `metadata.transferId`) to avoid Mongo collection scans at scale.
- CHANGELOG entry.

## 7. Non-goals
- Do NOT resurrect `ExternalID` (inert; no server support for transactions).
- Do NOT add `reference`/`code` correlation (no queryable server column).
- Do NOT add a get/list-by-idempotency-key (no server endpoint).
- No multi-key metadata AND (server doesn't support it).

## 8. Cross-repo follow-ups (out of SDK scope, flag to owners)
- **Server (`midaz`):** consider auto-indexing common transaction metadata keys, or document the `CreateIndex` requirement for correlation keys, so `metadata.transferId` lookups don't scan at scale.
- **Consumer (`plugin-br-bank-transfer`):** the §8 reconciliation resolver in `docs/plans/ted-in-intent-first-persistence.md` consumes this filter — once shipped, the resolver lists `metadata.transferId=<id>` (scoped by date window) instead of relying on idempotency replay. Bump the SDK dep there.

## 9. Sequencing
1. Add fields + `ToQueryParams` emission + `Validate` (4.1–4.2) with unit tests (RED→GREEN).
2. Contract-regression + iterator tests (5).
3. Docs + CHANGELOG (6).
4. Tag/release; notify the plugin to bump and wire the §8 resolver.
