# Repository guidelines

## Project structure and module organization

- `midaz.go` - Root SDK entry point (`github.com/LerianStudio/midaz-sdk-golang/v3`, package name `midaz`). Defines `Client`, `New`, and the user-facing `With*` options.
- `types.go` - Re-exports of the most commonly used `models.*` types (Account, Ledger, Transaction, etc.) and the `Environment` constants under the `midaz` package.
- `entities/` - Entity service interfaces, HTTP transport, request helpers, and service accessors.
- `models/` - Public API types, request builders, response wrappers, pagination options, and Midaz model aliases.
- `pkg/` - SDK utilities including access manager, concurrency, config, errors, formatting, observability, pagination, performance, retry, security, transaction helpers, validation, and versioning.
- `examples/` - Runnable examples. Start with `examples/mass-demo-generator` for end-to-end demo data.
- `docs/` - Generated Go docs and hand-written guides; `docs/mapping/` contains public and internal API maps.
- `scripts/` - Automation helpers for docs and repository maintenance.
- Root files: `Makefile`, `go.mod`, `.env.example`, `CONTRIBUTING.md`, `README.md`.

## Build, test, and development commands

- `make set-env` - Create `.env` from `.env.example`.
- `make tidy` / `make fmt` / `make lint` - Tidy dependencies, format code, and run `golangci-lint`.
- `make gosec` - Run security scanning.
- `make test` / `make test-fast` - Run all tests or short tests.
- `make coverage` - Produce an HTML coverage report in `artifacts/`.
- `make verify-sdk` - Run quick API build and compatibility checks.
- `make ci` - Preferred full local pipeline: tidy, fmt, lint, gosec, test, coverage, and SDK verification.
- `go build ./...` - Build all packages.
- `go test -v ./path/to/package -run TestName` - Run a targeted test.
- `make docs` - Generate static documentation.
- `make godoc` - Start an interactive docs server at http://localhost:6060.
- `make demo-data` - Run the mass demo generator in non-interactive mode.
- `cd examples/mass-demo-generator && DEMO_NON_INTERACTIVE=1 go run . --org-locale=br` - Run the generator directly.

## Coding style and naming conventions

- Go 1.26.x. Run `make fmt` before committing.
- Follow standard Go style and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Package names: lowercase, single-word when possible.
- Exported names: CamelCase with first letter capitalized.
- Unexported names: camelCase with first letter lowercase.
- Import order: standard library, external packages, internal packages.
- Keep functions small, context-aware (`context.Context` first parameter), and return rich errors.
- Use functional options for configuration and fluent builders for model input helpers.
- Use interfaces for external dependencies and service boundaries.
- Document all exported functions, types, and variables.
- No panics in library code. Return errors instead.

## Testing guidelines

- Use Go's `testing` package with `testify` for assertions and `gomock` where mocking helps.
- Name test files `*_test.go` and test functions `TestXxx`.
- Prefer table-driven tests for validation, mapping, transport, and retry behavior.
- Write unit tests for all new code; target at least 80% coverage for new critical logic.
- Run `make ci` before opening a PR, or run targeted checks plus `make test` when iterating quickly.

## Commit and pull request guidelines

- Use Conventional Commits: `<type>(<scope>): <description>`.
- Supported types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`.
- Examples: `feat(accounts): add balance caching`, `fix(retry): handle timeout errors`.
- PRs must include purpose, scope, key changes, how to test, and linked issues.
- Prefer `make ci` before opening a PR. Include example commands for demo or generator changes.

## Security and configuration tips

- Never commit secrets. Configure local runs via `.env` copied from `.env.example`.
- Service URL variables: `MIDAZ_BASE_URL`, `MIDAZ_ONBOARDING_URL`, `MIDAZ_TRANSACTION_URL`, `MIDAZ_CRM_URL`.
- Access Manager variables: `PLUGIN_AUTH_ENABLED`, `PLUGIN_AUTH_ADDRESS`, `MIDAZ_CLIENT_ID`, `MIDAZ_CLIENT_SECRET`.
- Other common variables: `MIDAZ_ENVIRONMENT`, `MIDAZ_USER_AGENT`, `MIDAZ_TIMEOUT`, `MIDAZ_DEBUG`, `MIDAZ_MAX_RETRIES`, `MIDAZ_IDEMPOTENCY`.
- Environment loading is explicit: use `config.NewConfig(config.FromEnvironment())`.
- Ensure idempotent unsafe requests by setting `X-Idempotency` via `sdkctx.WithIdempotencyKey(ctx, key)` when a stable caller-supplied key is required. The SDK auto-generates one for unsafe methods by default.

## Agent-specific instructions

- Keep changes minimal and scoped.
- Touch only relevant packages and docs; follow existing folder conventions.
- After generator or example changes, verify with `make demo-data` or a targeted `go run` command and update `docs/examples.md`.
- After public API changes, update `README.md`, `docs/README.md`, and `docs/mapping/`.
