# 01-hello-world

The smallest possible Midaz SDK demo. Build a client, list organizations,
print them. ~17 body lines (40 with imports + doc comment).

## What this demonstrates

- `midaz.New` with the two minimum-viable options: `WithEnvironment` + `WithAnonymous`
- One API call: `c.Organizations.ListOrganizations(ctx, opts)`
- Reading the typed page response

## When to use this pattern

Never directly. This is here to prove the SDK works in 17 lines of code.
For real usage:

- Add authentication: see [`02-auth/`](../02-auth/) (Access Manager)
- Add retries / observability / logging: see [`07-retries/`](../07-retries/),
  [`08-logging-slog/`](../08-logging-slog/), [`10-observability-otel/`](../10-observability-otel/)
- Use `examples/internal/quickstart` to skip the boilerplate

## How to run

```bash
go run ./examples/01-hello-world
```

Requires a local Midaz stack with auth disabled. Start it with the
recipe in your local Midaz repo's `make up` or equivalent.

## Expected output

```
- Acme Corp (org_01HX...)
- Initech (org_01HZ...)
```

If the local stack is empty, output is empty (no error). The example
limits to 5 results — adjust `Limit` to see more.

## Related

- [`05-listing-pages/`](../05-listing-pages/) — full pagination story
- [`docs/auth.md`](../../docs/auth.md) — picking an auth source for non-local stacks
- [`docs/configuration.md`](../../docs/configuration.md) — every available SDK option
