# 06-idempotency

Demonstrates the v3 idempotency surface — three modes for the
`X-Idempotency` request header on unsafe (POST/PUT/PATCH/DELETE)
requests.

## What this demonstrates

1. **Auto-generated key (default)** — every unsafe request gets a UUID
   in `X-Idempotency`. Free retry safety with no caller code.

2. **Caller-supplied key** — `sdkctx.WithIdempotencyKey(ctx, "stable-key")`
   makes the SDK emit the caller's key. The recommended pattern for
   at-least-once producers (saga steps, outbox rows, UI submissions).

3. **Per-call opt-out** — `sdkctx.WithoutAutoIdempotency(ctx)` suppresses
   the auto-generated key. For the rare endpoint where idempotency is
   genuinely undesired. Unsafe retries are also disabled unless the caller
   supplies `X-Idempotency` explicitly.

4. **Precedence rule** — an explicit key set via `WithIdempotencyKey`
   ALWAYS wins over `WithoutAutoIdempotency`. Useful when middleware
   blanket-applies suppression but a specific call-site wants
   idempotency back on.

Global default: `midaz.WithIdempotency(false)` disables auto-idempotency
for the entire client lifetime.

## When to use this pattern

Always. Auto-idempotency is the default for a reason. The only question
is whether the producer carries a stable key (Mode 2 — recommended for
financial movements) or relies on the SDK's per-request UUID (Mode 1 —
default).

## How to run

```bash
go run ./examples/06-idempotency
```

Requires a local Midaz stack with auth disabled. The example creates
4 organizations (one per mode demonstration). Re-running with the same
caller-supplied key (Mode 2) will not produce a duplicate.

## Expected output

```
--- Auto-generated idempotency key (default) ---
created with SDK-generated X-Idempotency header
--- Caller-supplied idempotency key ---
created with X-Idempotency=demo-stable-key-001 (re-runs are safe)
--- Auto-idempotency suppressed for one call ---
created with NO X-Idempotency header on the wire
--- Explicit key wins over suppression ---
created with X-Idempotency=demo-overrides-suppression-002
```

## Related

- `pkg/sdkctx` godoc — full reference for `WithIdempotencyKey` /
  `WithoutAutoIdempotency` and the related observability helpers.
- `midaz.WithIdempotency` — global on/off switch.
