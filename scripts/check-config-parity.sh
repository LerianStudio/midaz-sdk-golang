#!/usr/bin/env bash
# check-config-parity.sh
#
# Track 6 (Functional Options Sprawl) lint rule.
#
# Enforces the v3 two-layer Option contract: every exported With*
# function declared in pkg/config must have a matching exported With*
# function declared in midaz.go (the root client package). The exception
# list below covers Options that legitimately exist only at the Config
# layer because the midaz layer exposes them through a different
# canonical surface (e.g. retry knobs are exposed at midaz layer via
# WithRetryOptions(retry.Option...) instead of three positional knobs).
#
# Why this rule exists:
#   - In v2, pkg/config had ~16 With* options and midaz had ~12.
#     The mismatched surface meant users discovered Config-only knobs
#     by accident (autocomplete on cfg, then manual WithConfig dance).
#   - In v3 Track 6 we formalized the two-layer contract: midaz.With*
#     is the user-facing entry point; pkg/config.With* is the internal
#     /test layer that operates on Config directly.
#   - Without a guard rail, drift will sneak back in: someone adds
#     pkg/config.WithFoo without realizing they need midaz.WithFoo too.
#     This script fails the build the moment that happens.
#
# Output: prints offending function names and exits 1 on violation;
# silent and exits 0 on success.

set -euo pipefail

# Allow-list: pkg/config Options that are intentionally Config-only.
# Each entry must be paired with a justification comment. New entries
# require Track 6 owner approval (see docs/v3-dx-plan.md Decision Log).
ALLOW_LIST=(
    # Retry knobs: the canonical midaz-layer path is WithRetryOptions(
    # retry.Option...) which composes uniformly with every
    # retry.Option (BackoffFactor, JitterFactor, RetryableErrors,
    # RetryableHTTPCodes, presets). Adding three positional knobs
    # at the midaz layer would duplicate the surface. See Track 6
    # Batch 6C kickoff decision.
    "WithMaxRetries"
    "WithRetryWaitMin"
    "WithRetryWaitMax"
)

is_allowed() {
    local name="$1"
    for allowed in "${ALLOW_LIST[@]}"; do
        if [[ "$name" == "$allowed" ]]; then
            return 0
        fi
    done
    return 1
}

# Extract function names from `func With<Name>(...)` declarations.
# Match only top-level declarations (no leading whitespace).
extract_with_names() {
    local file="$1"
    grep -E '^func With[A-Z][A-Za-z0-9]*\(' "$file" \
        | sed -E 's/^func (With[A-Za-z0-9]+)\(.*/\1/' \
        | sort -u
}

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIDAZ_FILE="$REPO_ROOT/midaz.go"
CONFIG_FILE="$REPO_ROOT/pkg/config/config.go"

if [[ ! -f "$MIDAZ_FILE" ]]; then
    echo "❌ check-config-parity: cannot find $MIDAZ_FILE" >&2
    exit 2
fi
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "❌ check-config-parity: cannot find $CONFIG_FILE" >&2
    exit 2
fi

MIDAZ_OPTS="$(extract_with_names "$MIDAZ_FILE")"
CONFIG_OPTS="$(extract_with_names "$CONFIG_FILE")"

VIOLATIONS=()

while IFS= read -r config_opt; do
    [[ -z "$config_opt" ]] && continue
    if is_allowed "$config_opt"; then
        continue
    fi
    if ! grep -qx "$config_opt" <<< "$MIDAZ_OPTS"; then
        VIOLATIONS+=("$config_opt")
    fi
done <<< "$CONFIG_OPTS"

if [[ ${#VIOLATIONS[@]} -eq 0 ]]; then
    echo "✅ Config / midaz two-layer parity verified ($(echo "$CONFIG_OPTS" | grep -c .) config Options checked)"
    exit 0
fi

echo "❌ check-config-parity: pkg/config Options without a midaz counterpart:"
for v in "${VIOLATIONS[@]}"; do
    echo "    - pkg/config.$v"
done
echo ""
echo "Each pkg/config.With* Option must have a matching midaz.With* wrapper."
echo "Either (a) add midaz.$v as a delegating wrapper, OR (b) add the function"
echo "name to ALLOW_LIST in scripts/check-config-parity.sh with a justification."
echo ""
echo "See Track 6 (docs/v3-dx-plan.md) for the two-layer surface contract."
exit 1
