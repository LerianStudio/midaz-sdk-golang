# 09-testing-with-mocks

Demonstrates **unit-testing your code against the Midaz SDK** using a
consumer-defined narrow interface and a small hand-written mock — the
idiomatic pattern ("accept interfaces, return structs").

## What this demonstrates

- Declaring a NARROW interface on the consumer side (`accountSource`, with only
  the `All` and `GetByAlias` methods this code actually calls) instead of
  importing a broad SDK interface
- `c.V2.Accounts` (the concrete facade) satisfies that interface structurally in
  production; a tiny local mock satisfies it in tests — no generated mocks, no
  SDK test dependency
- Building synthetic `iter.Seq2[T, error]` streams in mock returns — the
  iterator-based pagination shape requires this when mocking `All`
- Asserting both happy-path and error-path behavior with proper `errors.Is`
  wrap-checking

## When to use this pattern

Always, in unit tests of code that calls the SDK. The reasons:

1. **Speed.** No network. No Midaz container. Tests run in milliseconds.
2. **Determinism.** No flaky API timeouts, no shared-data drift.
3. **Coverage.** You can return specific error responses that are otherwise
   hard to reproduce.
4. **Decoupling.** Your code depends on the narrow interface you own, not on a
   running backend or a broad SDK type.

For end-to-end coverage, run integration tests separately against a real local
stack.

## How it's wired

Declare the narrow interface next to the code that needs it, and pass whatever
satisfies it:

```go
type accountSource interface {
    All(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error]
    GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}
```

In production you pass `c.V2.Accounts`; in tests you pass a hand-written mock with
func fields (see `reporter_test.go`'s `mockAccountSource`). No code-generation
step is involved.

## Files

- `reporter.go` — `AccountReporter`, which depends on the consumer-defined
  `accountSource` interface. The unit under test.
- `reporter_test.go` — 4 tests covering happy paths, stream errors, not-found
  errors, and wrap propagation, using a local `mockAccountSource`.

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

- ["Accept interfaces, return structs"](https://go.dev/wiki/CodeReviewComments#interfaces) —
  the Go idiom this example follows
- [`errors.Is`](https://pkg.go.dev/errors#Is) — the error-wrap pattern the tests assert
