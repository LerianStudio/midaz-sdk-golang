#!/usr/bin/env bash
# Regenerate the per-plane OpenAPI clients from the native OAS 3.1 specs.
#
# Pipeline per plane:
#   1. specdowngrade api/<plane>.openapi.yaml -> ephemeral 3.0.3 intermediate
#   2. oapi-codegen (pinned via go.mod tool directive) -> committed .gen.go
#
# The 3.0.3 intermediate is a build artifact and is NOT committed; only the
# source specs (api/*.openapi.yaml) and the generated .gen.go are tracked.
# Consumers must not run codegen at build time.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

for plane in ledger tracer; do
  echo "==> generating $plane client"
  go run ./internal/cmd/specdowngrade \
    "api/${plane}.openapi.yaml" "$TMP/${plane}.303.yaml"
  # Run oapi-codegen from the plane's package dir so its config's relative
  # output path lands beside the config.
  ( cd "internal/gen${plane}" && \
    go tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
      -config oapi-codegen.yaml "$TMP/${plane}.303.yaml" )
done

echo "==> done"
