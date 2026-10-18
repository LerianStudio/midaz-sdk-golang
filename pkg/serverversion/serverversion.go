// Package serverversion parses a Midaz /version body and decides who applies fees and which CRM API to read.
// An uncertain version resolves to FeeModeLegacy: /v1 never applies fees on v3 or v4, so legacy never
// charges twice nor skips the fee, provided the caller's legacy mode has its own fee engine configured.
package serverversion

import (
	"encoding/json"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// Source names which /version body shape produced the value.
type Source string

// The /version body shapes Parse recognises.
const (
	SourceBuildInfoV1 Source = "buildinfo-v1" // v4.1+: schemaVersion "v1"
	SourceLegacy      Source = "legacy"       // v3.x and v4.0.x: {"version", "requestDate", ...}
	SourceUnavailable Source = "unavailable"  // transport error, non-2xx, non-JSON, no version field
)

// ServerVersion is what the connected Midaz ledger says it is.
type ServerVersion struct {
	Raw        string // "version" exactly as served ("" when unavailable)
	Major      int
	Minor      int
	Patch      int
	Prerelease string // "beta.5" for "4.2.0-beta.5"; "" otherwise
	Known      bool   // false for unavailable, unparsable, "0.0.0", "dev"
	Source     Source
}

// Parse decodes a /version response body. It never returns an error for an
// unknown or placeholder version: it returns Known=false instead.
func Parse(body []byte) ServerVersion {
	var doc struct {
		SchemaVersion *string `json:"schemaVersion"`
		Version       *string `json:"version"`
	}

	if err := json.Unmarshal(body, &doc); err != nil || doc.Version == nil {
		return ServerVersion{Source: SourceUnavailable}
	}

	source := SourceLegacy

	if doc.SchemaVersion != nil {
		if *doc.SchemaVersion != "v1" {
			return ServerVersion{Source: SourceUnavailable}
		}

		source = SourceBuildInfoV1
	}

	v, ok := parseSemVer(*doc.Version)
	if !ok || v.Major == 0 && v.Minor == 0 && v.Patch == 0 {
		return ServerVersion{Raw: *doc.Version, Source: source}
	}

	v.Raw, v.Known, v.Source = *doc.Version, true, source

	return v
}

// parseSemVer accepts [v]MAJOR.MINOR.PATCH[-prerelease][+build] with
// non-negative decimal components and non-empty suffixes; build metadata is
// discarded.
func parseSemVer(s string) (ServerVersion, bool) {
	s, build, hasBuild := strings.Cut(strings.TrimPrefix(strings.TrimSpace(s), "v"), "+")

	core, prerelease, hasPrerelease := strings.Cut(s, "-")
	if hasBuild && build == "" || hasPrerelease && prerelease == "" {
		return ServerVersion{}, false
	}

	parts := strings.Split(core, ".")

	var nums [3]int

	if len(parts) != len(nums) {
		return ServerVersion{}, false
	}

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return ServerVersion{}, false
		}

		nums[i] = n
	}

	return ServerVersion{Major: nums[0], Minor: nums[1], Patch: nums[2], Prerelease: prerelease}, true
}

// FeeMode is who owns fee application against the connected ledger.
type FeeMode string

// The two fee modes ResolveFeeMode can return.
const (
	FeeModeLegacy FeeMode = "legacy" // post on /v1; the server never applies fees there
	FeeModeNative FeeMode = "native" // post on /v2; the ledger applies fees
)

// ResolveFeeMode is the single decision rule: native iff Known && version >= 4.1.0,
// the first Midaz that marks its fee legs (operation metadata feeLeg) on /v2.
func ResolveFeeMode(v ServerVersion) FeeMode {
	if v.Known && (v.Major > 4 || v.Major == 4 && v.Minor >= 1) {
		return FeeModeNative
	}

	return FeeModeLegacy
}

// CRMAPI is which CRM API answers at a CRM URL.
type CRMAPI string

// The two CRM APIs ResolveCRMAPI can return.
const (
	CRMAPIV1 CRMAPI = "v1" // standalone CRM 3.x: /v1/aliases and /v1/holders, organization in X-Organization-Id
	CRMAPIV2 CRMAPI = "v2" // CRM embedded in the Midaz ledger 4.x: /v2/organizations/{org}/instruments and holders
)

// ResolveCRMAPI decides the API from the /version served at the CRM URL: v2 iff Known && major >= 4.
// No standalone CRM ships major 4 (it ended at 3.8.x), so a major-4 answer is a ledger; a placeholder
// ("0.0.0", a CRM 3.x with VERSION unset) is a CRM 3.x.
func ResolveCRMAPI(v ServerVersion) CRMAPI {
	if v.Known && v.Major >= 4 {
		return CRMAPIV2
	}

	return CRMAPIV1
}

// TracerRulesOnlyMinVersion is the first Tracer tag that serves POST /v1/validations/rules-only.
const TracerRulesOnlyMinVersion = "4.3.0-beta.78"

// SupportsRulesOnly reports whether the Tracer behind v serves the rules-only route: Known and
// v >= TracerRulesOnlyMinVersion by SemVer precedence, pre-releases included, build metadata ignored,
// except 4.3.0-rc.1 through rc.5, which sort above it but were cut before the route existed.
func SupportsRulesOnly(v ServerVersion) bool {
	sv := "v" + strings.TrimPrefix(strings.TrimSpace(v.Raw), "v")
	preRouteRC := semver.Compare(sv, "v4.3.0-rc.1") >= 0 && semver.Compare(sv, "v4.3.0-rc.6") < 0

	return v.Known && semver.Compare(sv, "v"+TracerRulesOnlyMinVersion) >= 0 && !preRouteRC
}
