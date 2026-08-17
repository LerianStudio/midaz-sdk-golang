// Package correlationtest provides the shared conformance check that every
// transactional plugin runs against the transaction inputs it produces, so the
// canonical correlation contract is verified in one place instead of being
// re-implemented — and re-diverged — per plugin.
//
// Usage in a plugin producer test:
//
//	input := adapter.BuildTransaction(transfer)
//	correlationtest.AssertCanonical(t, input)
package correlationtest

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models/correlation"
)

// requiredKeys are the metadata keys every conformant transaction carries.
var requiredKeys = []string{
	"contractVersion",
	"plugin",
	"rail",
	"flow",
	"aggregateId",
}

// optionalKeys are the remaining keys of the correlation contract: allowed when
// present, never required.
var optionalKeys = []string{
	"endToEndId",
	"providerMessageId",
	"providerMessageCode",
	"originalAggregateId",
	"direction",
}

// feePrefix is the metadata namespace reserved for the embedded-fees contract.
// Keys under it are tolerated but are not part of the correlation whitelist.
const feePrefix = "fee"

// AssertCanonical fails tb unless input conforms to the canonical correlation
// contract: the transaction metadata carries every required key of contract
// version ContractVersion, no metadata key on the transaction or on any leg
// falls outside the contract whitelist (the fee namespace aside), and no level
// of the payload sets both route and routeId. Every violation is reported, so a
// single run names all of them.
func AssertCanonical(tb testing.TB, input *models.CreateTransactionInput) {
	tb.Helper()

	for _, violation := range violations(input) {
		tb.Errorf("correlation contract v%s: %s", correlation.ContractVersion, violation)
	}
}

// violations returns every way input departs from the canonical contract, in a
// stable order.
func violations(input *models.CreateTransactionInput) []string {
	if input == nil {
		return []string{"transaction input is nil"}
	}

	found := requiredViolations(input.Metadata)
	found = append(found, whitelistViolations("transaction", input.Metadata)...)

	if input.Route != "" && input.RouteID != "" {
		found = append(found, `transaction sets both "route" and "routeId": they are mutually exclusive`)
	}

	for _, entry := range legs(input) {
		found = append(found, whitelistViolations(entry.label, entry.metadata)...)

		if entry.route != "" && entry.routeID != "" {
			found = append(found, fmt.Sprintf(
				`%s sets both "route" and "routeId": they are mutually exclusive`, entry.label))
		}
	}

	return found
}

// requiredViolations reports required keys that are absent, blank, or not
// strings, plus a contractVersion that names another version of the contract.
func requiredViolations(metadata map[string]any) []string {
	var found []string

	for _, key := range requiredKeys {
		value, present := metadata[key]
		if !present {
			found = append(found, fmt.Sprintf("transaction metadata is missing required key %q", key))

			continue
		}

		text, isText := value.(string)
		if !isText || strings.TrimSpace(text) == "" {
			found = append(found, fmt.Sprintf("transaction metadata key %q is empty", key))
		}
	}

	if version, isText := metadata["contractVersion"].(string); isText && version != correlation.ContractVersion {
		found = append(found, fmt.Sprintf(
			"transaction metadata declares contractVersion %q, want %q", version, correlation.ContractVersion))
	}

	return found
}

// whitelistViolations reports metadata keys outside the correlation contract.
func whitelistViolations(label string, metadata map[string]any) []string {
	var unknown []string

	for key := range metadata {
		if slices.Contains(requiredKeys, key) || slices.Contains(optionalKeys, key) ||
			strings.HasPrefix(key, feePrefix) {
			continue
		}

		unknown = append(unknown, key)
	}

	slices.Sort(unknown)

	found := make([]string, 0, len(unknown))
	for _, key := range unknown {
		found = append(found, fmt.Sprintf(
			"%s metadata key %q is outside the contract whitelist", label, key))
	}

	return found
}

// leg is one source or destination entry of the payload, flattened for
// per-level checks.
type leg struct {
	label    string
	route    string
	routeID  string
	metadata map[string]any
}

func legs(input *models.CreateTransactionInput) []leg {
	if input.Send == nil {
		return nil
	}

	var flattened []leg

	if input.Send.Source != nil {
		flattened = append(flattened, flatten("send.source.from", input.Send.Source.From)...)
	}

	if input.Send.Distribute != nil {
		flattened = append(flattened, flatten("send.distribute.to", input.Send.Distribute.To)...)
	}

	return flattened
}

func flatten(prefix string, entries []models.FromToInput) []leg {
	flattened := make([]leg, 0, len(entries))

	for index, entry := range entries {
		routeID := ""
		if entry.RouteID != nil {
			routeID = *entry.RouteID
		}

		flattened = append(flattened, leg{
			label:    fmt.Sprintf("%s[%d]", prefix, index),
			route:    entry.Route,
			routeID:  routeID,
			metadata: entry.Metadata,
		})
	}

	return flattened
}
