// Package correlationtest provides the shared conformance check that every
// transactional plugin runs against the transaction inputs it produces, so the
// canonical correlation contract is verified in one place instead of being
// re-implemented — and re-diverged — per plugin.
//
// The check keeps no copy of the contract. It rebuilds a correlation.Correlation
// from the transaction metadata and runs the emitter's own Validate, and it takes
// the whitelist from correlation.Keys, so a contract change lands in one package
// and the checker follows automatically instead of quietly disagreeing with the
// producer.
//
// Usage in a plugin producer test:
//
//	input := adapter.BuildTransaction(transfer)
//	correlationtest.AssertCanonical(t, input)
package correlationtest

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models/correlation"
)

// contractKeys is the closed whitelist, taken from the emitter. There is no
// namespace escape hatch on purpose: an extra key (a fee key, a client's own
// field) is admitted by extending the contract in models/correlation and bumping
// its version, never by a prefix match here. A "fee" prefix would also admit
// feePayerDocument and feedbackFromCustomer — precisely the counterparty PII the
// closed whitelist exists to keep out of the ledger.
var contractKeys = correlation.Keys()

// AssertCanonical fails tb unless input conforms to the canonical correlation
// contract: the transaction metadata declares contract version
// correlation.ContractVersion and rebuilds into a valid correlation.Correlation
// (required fields present, rail/flow/direction inside their enums, a refund
// naming the aggregate it returns), every contract key holds exactly what
// correlation.ToMetadata would emit (trimmed values, blank fields omitted), no
// metadata key on the transaction or on any leg falls outside correlation.Keys,
// and no level of the payload sets both route and routeId. Every whitelist and
// route violation is reported, so a single run names all of them.
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

	found := contractViolations(input.Metadata)
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

// contractViolations reports metadata that does not carry a valid correlation of
// this contract version. The correlation rules are not re-implemented here: the
// payload is rebuilt into a Correlation and validated by the package that emits
// it, so this check can never accept metadata the producer's own Validate would
// have refused — a refund with no original aggregate, or rail "TEF", fails both.
//
// Validation alone is not enough: Validate accepts a padded aggregateId, but the
// create request serializes the raw map, and the ledger lookup is an exact
// match. So each contract key must also hold exactly what ToMetadata would emit
// (trimmed values, blank fields omitted entirely).
func contractViolations(metadata map[string]any) []string {
	var found []string

	version, _ := metadata["contractVersion"].(string) //nolint:errcheck // an absent or non-string version is reported below

	switch {
	case version == "":
		found = append(found, `transaction metadata is missing required key "contractVersion"`)
	case version != correlation.ContractVersion:
		found = append(found, fmt.Sprintf(
			"transaction metadata declares contractVersion %q, want %q", version, correlation.ContractVersion))
	}

	if err := correlation.FromMetadata(metadata).Validate(); err != nil {
		found = append(found, "transaction metadata is not a valid correlation: "+err.Error())
	}

	canonical := correlation.FromMetadata(metadata).ToMetadata()

	for _, key := range contractKeys {
		if key == "contractVersion" {
			// Fully checked above, presence and exact value.
			continue
		}

		raw, rawPresent := metadata[key]
		want, wantPresent := canonical[key]

		if rawPresent && (!wantPresent || !reflect.DeepEqual(raw, want)) {
			found = append(found, fmt.Sprintf(
				"transaction metadata key %q holds %#v, not its canonical ToMetadata serialization", key, raw))
		}
	}

	return found
}

// whitelistViolations reports metadata keys outside the correlation contract.
func whitelistViolations(label string, metadata map[string]any) []string {
	var unknown []string

	for key := range metadata {
		if slices.Contains(contractKeys, key) {
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
