# 11-server-version

Decide who owns fees against the connected Midaz ledger: resolve the fee
mode once at boot, refresh it on a ticker, and fall back to legacy whenever
`GET /version` cannot prove the ledger applies fees.

## What this demonstrates

- `c.ServerVersion(ctx)` reading the ledger's public `/version` route
- `midaz.ResolveFeeMode(v)` turning it into `FeeModeNative` (post on `/v2`,
  the ledger applies fees) or `FeeModeLegacy` (post on `/v1`, the service
  charges fees itself)
- A cached, lock-free mode: the request path reads it and never calls `/version`
- A refresh loop that logs only when the mode changes
- Any failure (404, 5xx, timeout, proxy page) resolving to legacy

## When to use this pattern

Any service that posts transactions and charges fees, and may run against
Midaz v3 or v4. Keep one cached mode per Midaz client (per tenant in a
multi-tenant service). Store the mode with each operation: its retries,
commit, cancel and revert keep the mode it was created with, even after a
refresh changes it.

## How to run

```bash
go run ./examples/11-server-version
```

Requires a local Midaz stack. Stop it with Ctrl-C. The demo refreshes every
10 seconds; a service refreshes every few minutes.

## Expected output

Against Midaz v4.1+:

```
INFO midaz fee mode feeMode=native serverVersion=4.1.3 source=buildinfo-v1
```

Against Midaz v3.8:

```
INFO midaz fee mode feeMode=legacy serverVersion=v3.8.4 source=legacy
```

With no ledger running:

```
WARN midaz server version unavailable, using legacy fees error="network error during midaz.Client.ServerVersion: ..."
INFO midaz fee mode feeMode=legacy serverVersion="" source=unavailable
```

## Related

- [`docs/server-version.md`](../../docs/server-version.md) — the `/version`
  shapes, the decision rule, and why the fallback is legacy
- [`03-end-to-end/`](../03-end-to-end/) — posting on `/v2`
