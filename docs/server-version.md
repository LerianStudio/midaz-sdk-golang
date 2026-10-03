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

`ServerVersion` returns a non-nil error if and only if `Source` is
`unavailable`:

- a transport failure (timeout, cancelled context, refused connection) or a
  non-2xx status, such as a 404;
- a 2xx body that is not a `/version` response: non-JSON (a proxy HTML page),
  cut at 64 KiB, `{}`, no `version` field, or an unknown `schemaVersion`.

A recognised shape with a placeholder or unparsable version (`0.0.0`, `dev`,
`4.1.x`) keeps `Known=false` and its real `Source`, with a nil error.

## The decision rule

`ResolveFeeMode` returns native if and only if `Known` and the version is 4.1.0 or later: v4.1.0 is the first Midaz that marks its fee legs (operation metadata `feeLeg`), which a caller needs to read the fee the ledger charged.

| What `/version` says | Mode |
|---|---|
| v3.x, v4.0.x | legacy |
| v4.1+, any v4.1+ prerelease, v5+ | native |
| `0.0.0`, `dev`, invalid, unknown `schemaVersion` | legacy |
| no answer (any failure above) | legacy |

## Why an unknown version falls back to legacy

Native is correct only from v4.1: on a v3 ledger it posts to a `/v2` that does not
exist, and v4.0.x charges fees without marking the fee legs. Legacy posts on `/v1`, which never applies fees on either line. So when
`/version` serves a version the SDK cannot use (`0.0.0`, `dev`, invalid), or
cannot be read at boot, the mode is legacy.

Legacy charges the fee only through the service's own fee path (an external
fee engine such as `plugin-fees`, switched on). With that path in place,
legacy charges exactly once on v3 and on v4. Without it, legacy never charges
twice but charges nothing, where native would have charged through the
ledger's fee packages.

That is why a failed refresh keeps the last resolved mode instead of falling
back: a v4 ledger that is up answers `/version`, so a real downgrade shows on
the next successful read, and a ledger that is down fails posting in either
mode. Only a service that has never read `/version` starts on legacy.

## Using it in a service

1. Resolve once at boot, before the first operation.
2. Refresh on a ticker (minutes, not per request) and log each change with the
   mode, the raw version and the source. A refresh that returns an error keeps
   the last mode.
3. Read the cached mode when an operation is created, and store it with the
   operation. Its retries, commit, cancel, revert and reconciliation use the
   mode and the surface (`/v1` or `/v2`) it was created with; a refresh only
   changes operations created after it.
4. In a multi-tenant service, keep one cached mode per Midaz client (per tenant).

[`examples/11-server-version`](../examples/11-server-version/) shows steps 1
and 2 with a lock-free cached mode.
