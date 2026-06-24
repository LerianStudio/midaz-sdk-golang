#!/usr/bin/env bash
# Detect contract drift between the pinned midaz commit and the current midaz HEAD.
# "Drift" = changes in the tracked OpenAPI specs only, not raw commit count.
#
#   exit 0  -> no contract drift (specs unchanged since the pin)
#   exit 1  -> drift detected (specs changed; diff printed)
#   exit 2  -> usage / environment error
#
# Usage:
#   scripts/check-midaz-drift.sh [branch]      # branch defaults to the SDK's current branch
#   MIDAZ_REPO=/path/to/midaz scripts/check-midaz-drift.sh develop
set -euo pipefail

sdk_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
baseline="$sdk_root/midaz-baseline.json"
midaz_repo="${MIDAZ_REPO:-$sdk_root/../midaz}"
branch="${1:-$(git -C "$sdk_root" rev-parse --abbrev-ref HEAD)}"

command -v jq >/dev/null || { echo "error: jq is required" >&2; exit 2; }
[[ -f "$baseline" ]] || { echo "error: baseline not found: $baseline" >&2; exit 2; }
[[ -d "$midaz_repo/.git" ]] || { echo "error: midaz repo not found at '$midaz_repo' (set MIDAZ_REPO)" >&2; exit 2; }

pinned="$(jq -r --arg b "$branch" '.branches[$b].commit // empty' "$baseline")"
[[ -n "$pinned" ]] || { echo "error: no pin for branch '$branch' in $baseline" >&2; exit 2; }
mapfile -t specs < <(jq -r '.tracked_paths[]' "$baseline")

echo "midaz drift check: branch=$branch pinned=${pinned:0:9}"
git -C "$midaz_repo" fetch --quiet origin "$branch" || { echo "error: git fetch origin $branch failed" >&2; exit 2; }
git -C "$midaz_repo" cat-file -e "${pinned}^{commit}" 2>/dev/null \
  || { echo "error: pinned commit $pinned not in local midaz (shallow clone?). Run: git -C '$midaz_repo' fetch --unshallow" >&2; exit 2; }

head="$(git -C "$midaz_repo" rev-parse "origin/$branch")"
ahead="$(git -C "$midaz_repo" rev-list --count "${pinned}..${head}")"
echo "midaz origin/$branch is at ${head:0:9} (${ahead} commit(s) ahead of pin)"

# git diff exits non-zero under set -e when there ARE changes; capture without tripping it.
stat="$(git -C "$midaz_repo" diff --stat "${pinned}..${head}" -- "${specs[@]}" || true)"
if [[ -z "$stat" ]]; then
  echo "✅ no contract drift: tracked specs unchanged since pin"
  exit 0
fi

echo "⚠️  CONTRACT DRIFT detected in tracked specs:"
echo "$stat"
echo
echo "Full diff:"
echo "  git -C '$midaz_repo' diff ${pinned:0:9}..origin/$branch -- ${specs[*]}"
exit 1
