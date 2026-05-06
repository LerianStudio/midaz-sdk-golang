# configuration

Reference example for **client and config setup patterns** — the
many ways to construct a `*midaz.Client` for different deployment
shapes.

## What this demonstrates

- `config.NewConfig(config.FromEnvironment())` for env-driven setup
- `config.NewConfig(config.WithEnvironment(...), ...)` for explicit setup
- Mixing environment defaults with explicit overrides
- The two-layer Option contract: every `midaz.With*` option has a
  matching `config.With*` builder for advanced use cases
- The mutual-exclusion between `WithAccessManager` and `WithAnonymous`

## When to use this pattern

Any time a single `midaz.New(...)` call isn't expressive enough — for
example, when you need to validate config before constructing the
client, share config across multiple clients, or load config from a
custom source (CLI flags, secrets manager, etc.).

## How to run

```bash
go run ./examples/configuration
```

## Related

- [`docs/configuration.md`](../../docs/configuration.md) — every
  available option, both layers
- [`pkg/config`](../../pkg/config/) — the underlying config builder
