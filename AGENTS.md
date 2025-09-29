# Repository Guidelines

## Project Structure & Module Organization

- `entities/` — HTTP clients and service accessors.
- `models/` — Public API types (request/response payloads).
- `pkg/` — SDK utilities (config, validation, observability, retry, performance, generator, integrity).
- `examples/` — Runnable examples. Start with `examples/mass-demo-generator`.
- `docs/` — Generated docs and guides; `scripts/` — automation helpers.
- Root files: `Makefile`, `go.mod`, `.env.example`, `PLAN.md`.

## Build, Test, and Development Commands

- `make set-env` — Create `.env` from `.env.example`.
- `make test` / `make test-fast` — Run all/short tests.
- `make coverage` — Produce HTML coverage report in `artifacts/`.
- `make lint` / `make fmt` / `make tidy` — Lint, format, and tidy deps.
- `make verify-sdk` — Quick API build/compat checks.
- `go build ./...` — Build all packages.
- `go test -v ./path/to/file_test.go` — Test single file.
- `go test -v ./path/to/package -run TestName` — Run specific test.
- `make docs` — Generate static documentation.
- Example (interactive off):
  - `make demo-data`
  - or `cd examples/mass-demo-generator && DEMO_NON_INTERACTIVE=1 go run main.go --org-locale=br --patterns=false`
- Docs server: `make godoc` (http://localhost:6060).

## Coding Style & Naming Conventions

- Go 1.24.x. Run `make fmt` before committing.
- Follow standard Go style and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Package names: lowercase, single-word.
- Exported names: CamelCase with first letter capitalized.
- Unexported names: camelCase with first letter lowercase.
- Import order: standard library, external packages, internal packages.
- Keep functions small, context-aware (`context.Context` first param), and return rich errors.
- Use functional options pattern for configuration.
- Use interfaces for external dependencies (especially lib-commons).
- Document all exported functions, types, and variables.
- Lint with `golangci-lint` (`make lint`). No panics in library code.

## Testing Guidelines

- Use Go's `testing` with `testify` for assertions and `gomock` where mocking helps.
- Name tests `*_test.go`; functions `TestXxx` and table-driven where appropriate.
- Write unit tests for all new code (minimum 80% coverage).
- Run `make test` locally; target >80% coverage for new critical logic; generate report with `make coverage`.

## Commit & Pull Request Guidelines

- Conventional Commits: `<type>(<scope>): <description>`
  - Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`
  - Examples: `feat(accounts): add balance caching`, `fix(retry): handle timeout errors`
- PRs must include: purpose, scope, key changes, how-to-test, and linked issues.
- Run `make fmt lint test verify-sdk` before opening a PR. Include example commands for demos when applicable.

## Security & Configuration Tips

- Never commit secrets. Configure via `.env` (copy from `.env.example`).
- Typical vars: auth token and service URLs for onboarding/transaction APIs.
- Prefer idempotent requests (client sets `X-Idempotency` from context).

## Agent-Specific Instructions

- Keep changes minimal and scoped; update `PLAN.md` when you finish milestones or add flags/patterns.
- Touch only relevant packages; follow existing folder conventions.
- After generator or example changes, verify with `make demo-data` and document new flags in `docs/`.
