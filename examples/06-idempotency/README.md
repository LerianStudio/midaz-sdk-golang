# 06-idempotency

Demonstrates the idempotency surface — the modes for the
`X-Idempotency` request header on unsafe (POST/PUT/PATCH/DELETE)
requests.

## What this demonstrates

1. **No key (default)** — the SDK sends no random `X-Idempotency` (the demo
   creates an organization, which carries no key). A keyless `/v1` transaction
   create sends a key derived from the request (SHA-256 of method, path and
   body), so the same request sent again returns the first outcome, while
   block and unblock with one body never share the ledger's `/v1` slot. Two
   intentional transactions with identical requests inside the ledger's
   idempotency window are answered with the first one: pass a distinct key
   (mode 2) for each. Opt in to a random UUID per create, update and delete
   with `midaz.WithIdempotency(true)` or `MIDAZ_IDEMPOTENCY=true`.

2. **Caller-supplied key** — `sdkctx.WithIdempotencyKey(ctx, "stable-key")`
   makes the SDK emit the caller's key. The recommended pattern for
   at-least-once producers (saga steps, outbox rows, UI submissions).

3. **Precedence rule** — an explicit key set via `WithIdempotencyKey`
   ALWAYS wins over `WithoutAutoIdempotency` (which only stops the random key
   of a client opted in with `midaz.WithIdempotency(true)`, never the `/v1`
   derived key). Useful when middleware
   blanket-applies suppression but a specific call-site wants
   idempotency back on.

Global default: auto-idempotency is off; `midaz.WithIdempotency(true)` turns
on a random key per create, update and delete request for the client lifetime.

## When to use this pattern

Always carry a distinct stable key (Mode 2) for financial movements that
are intentionally identical to an earlier one; otherwise the ledger's body
dedupe (Mode 1 — default) answers them with the first outcome.

## How to run

```bash
go run ./examples/06-idempotency
```

Requires a local Midaz stack with auth disabled. The example creates
3 organizations (one per demo). Re-running with the same
caller-supplied key (Mode 2) will not produce a duplicate.

## Expected output

```
--- No idempotency key (default) ---
created with NO X-Idempotency header on the wire
--- Caller-supplied idempotency key ---
created with X-Idempotency=demo-stable-key-001 (re-runs are safe)
--- Explicit key wins over suppression ---
created with X-Idempotency=demo-overrides-suppression-002
```

## Related

- `pkg/sdkctx` godoc — full reference for `WithIdempotencyKey` /
  `WithoutAutoIdempotency` and the related observability helpers.
- `midaz.WithIdempotency` — global opt-in switch.
