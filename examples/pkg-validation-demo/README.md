# pkg-validation-demo

Reference example for the **`pkg/validation` utility package**. Does
NOT use the SDK client surface — it exercises the standalone validators
that ship with the SDK for asset-code, metadata-size, currency-code,
and account-alias rules.

## What this demonstrates

- `pkg/validation` validators for individual fields:
  - Asset code format (3-4 uppercase letters)
  - Metadata size limits and value-type rules
  - Currency code format (ISO 4217)
  - Account alias format
- `validation.FieldErrors` accumulator (Track 8B):
  - `Append` / `AppendWith`
  - `OrNil` for the Go nil-interface trap
  - `Errs()` for programmatic walking

## When to use this pattern

When your service runs SDK-aligned validation locally before persisting
or forwarding data. The same validators the SDK uses inside `Validate()`
methods are also available standalone for upstream UI / form / handler code.

## How to run

```bash
go run ./examples/pkg-validation-demo
```

No SDK client. No network. No dependencies beyond `pkg/validation`.

## Related

- [`pkg/validation`](../../pkg/validation/) — the validation package
- Track 8C — model `Validate()` methods that consume these validators
