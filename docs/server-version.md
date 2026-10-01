# Server version and fee mode

Midaz v4 can apply fees inside the ledger; Midaz v3 cannot. A service that
posts transactions and charges fees must pick exactly one fee owner per flow,
so it needs to know which ledger it is talking to. The SDK answers that in two
calls:

```go
v, err := c.ServerVersion(ctx)   // GET <LedgerURL>/version
mode := midaz.ResolveFeeMode(v)  // midaz.FeeModeNative or midaz.FeeModeLegacy
```

| Mode | Where new transactions go | Who charges the fee |
|---|---|---|
| `FeeModeNative` | `/v2` (`c.V2.Transactions.CreateDirect` / `CreateHold`) | the ledger; the service never calls an external fee engine |
| `FeeModeLegacy` | `/v1` | the service, through its external fee engine (e.g. `plugin-fees`); the ledger applies no fee on `/v1` |

## The three `/version` shapes

`/version` is public and unversioned (never `/v1/version`) on every Midaz line,
mounted before auth. Its body changed twice:

| Midaz | Served by | Body | `Source` |
|---|---|---|---|
| v3.8.x | lib-commons/v5 `commons/net/http.Version` | `{"version", "requestDate"}`; `version` comes from the `VERSION` env, default `"0.0.0"` | `legacy` |
| v4.0.x | `pkg/buildinfo.VersionHandler` | `{"version", "requestDate", "commit", "buildTime", "dirty"}`; `version` still comes from the `VERSION` env | `legacy` |
| v4.1+ | lib-commons/v7 `commons/buildinfo.Handler` | `{"schemaVersion": "v1", "service", "version", "revision", "buildTime", "modified", "goVersion"}`; `version` is compiled into the binary | `buildinfo-v1` |

Parsing rules:

- A leading `v` is dropped: `"v3.8.0"` is 3.8.0.
- `schemaVersion: "v1"` gives `buildinfo-v1`; `version` without `schemaVersion`
  gives `legacy`. Unknown fields are ignored.
- Any other `schemaVersion` (a future `v2`) gives `unavailable`, so the SDK
  stays on legacy until it learns the new shape.
- `"0.0.0"`, `"dev"`, an empty string or an invalid SemVer give `Known=false`.
- A prerelease keeps its major: `4.2.0-beta.5` is `Major=4`.
- A 404, a non-2xx status, a timeout, a cancelled context, a refused
  connection, a non-JSON body (a proxy HTML page) or a body over 64 KiB give
  `Source=unavailable` plus a non-nil error to log. An unknown version is not
  an error.

## The decision rule

`ResolveFeeMode` returns native if and only if `Known && Major >= 4`.

| What `/version` says | Mode |
|---|---|
| v3.x | legacy |
| v4.0.x, v4.1+, any v4 prerelease | native |
| `0.0.0`, `dev`, invalid, unknown `schemaVersion` | legacy |
| no answer (any failure above) | legacy |

## Why every doubt falls back to legacy

Legacy is correct on both lines; native is correct only on v4.

- On v3, `/v1` is the only path and the ledger has no fee engine, so the
  service's external fee engine is the one owner.
- On v4, the `/v1` path never applies fees, and the external fee engine keeps
  charging exactly as it did on v3. Still one owner.
- Guessing native on a v3 ledger posts to a `/v2` that does not exist, or, if
  the service stopped calling its fee engine, skips the fee.

So erring to legacy never charges twice and never skips the fee. A transient
`/version` failure on a v4 ledger only moves new operations to legacy until
the next refresh succeeds.

## Using it in a service

1. Resolve once at boot, before the first operation.
2. Refresh on a ticker (minutes, not per request) and log each change with the
   mode, the raw version and the source.
3. Read the cached mode when an operation is created, and store it with the
   operation. Its retries, commit, cancel, revert and reconciliation use the
   mode and the surface (`/v1` or `/v2`) it was created with; a refresh only
   changes operations created after it.
4. In a multi-tenant service, keep one cached mode per Midaz client (per tenant).

[`examples/11-server-version`](../examples/11-server-version/) shows steps 1
and 2 with a lock-free cached mode.
