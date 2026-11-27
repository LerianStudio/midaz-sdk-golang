#!/usr/bin/env bash
# Regenerate the per-plane OpenAPI clients from the native OAS 3.1 specs.
#
# Pipeline per plane:
#   1. specoverlay api/<plane>.openapi.yaml + the plane's local fragment
#      (only planes listed in fragment_for) -> ephemeral merged spec
#   2. specdowngrade <spec> -> ephemeral 3.0.3 intermediate
#   3. oapi-codegen (pinned via go.mod tool directive) -> committed .gen.go
#
# Steps 1 and 2 run for every plane before the first oapi-codegen, so a failed
# merge (a fragment key the server spec already declares) or downgrade stops
# the script with no generated file rewritten.
#
# The merged spec and the 3.0.3 intermediate are build artifacts and are NOT
# committed; only the source specs and fragments (api/*.openapi.yaml) and the
# generated .gen.go are tracked. Consumers must not run codegen at build time.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

PLANES=(ledger tracer)

# fragment_for prints the local fragment merged into a plane's server spec, or
# nothing when the plane is generated from its server spec alone.
fragment_for() {
  case "$1" in
    tracer) echo "api/tracer.legacy-reservations.openapi.yaml" ;;
  esac
}

for plane in "${PLANES[@]}"; do
  echo "==> preparing $plane spec"
  spec="api/${plane}.openapi.yaml"
  fragment="$(fragment_for "$plane")"
  if [[ -n "$fragment" ]]; then
    go run ./internal/cmd/specoverlay \
      "$spec" "$fragment" "$TMP/${plane}.merged.yaml"
    spec="$TMP/${plane}.merged.yaml"
  fi
  go run ./internal/cmd/specdowngrade \
    "$spec" "$TMP/${plane}.303.yaml"
done

for plane in "${PLANES[@]}"; do
  echo "==> generating $plane client"
  # Run oapi-codegen from the plane's package dir so its config's relative
  # output path lands beside the config.
  ( cd "internal/gen${plane}" && \
    go tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
      -config oapi-codegen.yaml "$TMP/${plane}.303.yaml" )
done

echo "==> done"
