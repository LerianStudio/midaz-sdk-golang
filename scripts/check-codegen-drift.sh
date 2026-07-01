#!/usr/bin/env bash
# Assert the committed OpenAPI clients match a fresh `make generate`.
#
# "Codegen drift" = the tracked generated output (internal/gen*/*.gen.go) no
# longer reproduces from the source specs (api/*.openapi.yaml) via the pinned
# toolchain. This is the analogue of the docs-pipeline drift gate: it protects
# the committed generated code from silent divergence.
#
#   exit 0 -> no drift (regeneration is a byte-for-byte no-op)
#   exit 1 -> drift detected (diff printed)
#   exit 2 -> refuses to run: generated paths already dirty before regen
#
# Scoped to the codegen-owned paths only. `make generate` also runs `go mod
# tidy`, which can touch go.mod/go.sum from unrelated dependency state; that
# churn is NOT codegen drift, so this gate must not diff the whole tree.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Paths that `make generate` owns and this gate is allowed to judge.
PATHS=(internal/genledger/ledger.gen.go internal/gentracer/tracer.gen.go)

# A pre-existing dirty generated tree would make any post-regen diff ambiguous
# (was it the regen, or uncommitted edits?). Refuse rather than report a
# misleading verdict — same fail-fast stance as check-midaz-drift.sh.
if ! git diff --quiet -- "${PATHS[@]}"; then
  echo "error: generated files are already modified before regeneration:" >&2
  git diff --stat -- "${PATHS[@]}" >&2
  echo "commit or discard those changes, then re-run the drift check." >&2
  exit 2
fi

echo "codegen drift check: regenerating clients..."
./scripts/generate-clients.sh >/dev/null

if git diff --quiet -- "${PATHS[@]}"; then
  echo "✅ no codegen drift: committed clients reproduce from the specs"
  exit 0
fi

echo "⚠️  CODEGEN DRIFT detected: committed clients do not match the specs." >&2
git diff --stat -- "${PATHS[@]}" >&2
echo >&2
echo "Run 'make generate' and commit the result." >&2
git --no-pager diff -- "${PATHS[@]}" >&2
exit 1
