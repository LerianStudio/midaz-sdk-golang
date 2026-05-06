# 09-testing-with-mocks

Demonstrates **unit-testing your code against the Midaz SDK** using
`go.uber.org/mock` and the SDK's pre-generated mock implementations
under `entities/mocks/`.

## What this demonstrates

- Depending on the SDK service **interfaces** (e.g.,
  `entities.AccountsService`) rather than the concrete `*midaz.Client`
- Wiring `c.Accounts` in production and `mocks.NewMockAccountsService`
  in tests
- Building synthetic `iter.Seq2[T, error]` streams in mock returns —
  the iterator-based pagination shape requires this when mocking
  ListAccountsAll / ListAccountsPages
- Asserting both happy-path and error-path behavior with proper
  `errors.Is` wrap-checking

## When to use this pattern

Always, in unit tests of code that calls the SDK. The reasons:

1. **Speed.** No network. No Midaz container. Tests run in milliseconds.
2. **Determinism.** No flaky API timeouts, no shared-data drift.
3. **Coverage.** You can mock specific error responses (5xx, validation,
   network failure) that are otherwise hard to reproduce.
4. **Decoupling.** Your code depends on a stable SDK interface, not on
   a running backend. Refactoring backend behavior doesn't break your
   tests.

For end-to-end coverage, run integration tests separately against a
real local stack.

## How it's wired

Mock generation is automatic via `//go:generate` directives on each
SDK service source file:

```go
// entities/accounts.go
package entities

//go:generate mockgen -source=accounts.go -destination=mocks/mock_accounts.go -package=mocks AccountsService
```

To regenerate mocks after the SDK interfaces change:

```bash
go generate ./entities/...
```

You don't need to do this — the SDK ships pre-generated mocks for
every service.

## Files

- `reporter.go` — `AccountReporter` type that depends on
  `entities.AccountsService`. The unit under test.
- `reporter_test.go` — 4 tests covering happy paths, stream errors,
  not-found errors, and wrap propagation.

## How to run

```bash
go test ./examples/09-testing-with-mocks/...
```

## Expected output

```
=== RUN   TestCountAccounts_Success
--- PASS: TestCountAccounts_Success (0.00s)
=== RUN   TestCountAccounts_StreamError
--- PASS: TestCountAccounts_StreamError (0.00s)
=== RUN   TestFindByAlias_Success
--- PASS: TestFindByAlias_Success (0.00s)
=== RUN   TestFindByAlias_NotFound
--- PASS: TestFindByAlias_NotFound (0.00s)
PASS
```

## Related

- [`go.uber.org/mock`](https://github.com/uber-go/mock) — the mock
  framework. Note: this is the active fork. The deprecated
  `github.com/golang/mock` was removed from the SDK in v3.
- [`entities/mocks/`](../../entities/mocks/) — pre-generated mocks
- [Track 8 / Error system](../../docs/v3-dx-plan.md#track-8--error-system-actionability) —
  context for the `errors.Is` wrap pattern
