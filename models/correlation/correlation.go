// Package correlation is the single source of truth for the correlation
// metadata that transactional plugins attach to Midaz ledger transactions.
//
// The ledger metadata is a closed, versioned whitelist: only the fields of
// Correlation ever reach the ledger, and every emitted payload carries
// ContractVersion. Arbitrary client metadata is never forwarded — a plugin that
// needs an extra field is asking for a contract change (a new version of this
// package), not for a free-form map. That closure is what keeps PII out of the
// ledger by construction instead of by inspection.
//
// Typical use in a plugin producer:
//
//	c := correlation.Correlation{
//	    Plugin:      "br-bank-transfer",
//	    Rail:        correlation.RailTED,
//	    Flow:        correlation.FlowCashOut,
//	    AggregateID: transfer.ID,
//	    Direction:   correlation.DirectionOut,
//	}
//	if err := c.Validate(); err != nil {
//	    return err
//	}
//	input.Metadata = c.ToMetadata()
package correlation

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ContractVersion is the version of the ledger metadata contract emitted by
// ToMetadata. It is bumped only when the whitelist itself changes.
const ContractVersion = "1"

// Rail is the payment rail that produced the transaction.
type Rail string

const (
	// RailTED is the Brazilian wire transfer rail (SPB/STR).
	RailTED Rail = "TED"

	// RailPix is the Brazilian instant payment rail (SPI).
	RailPix Rail = "PIX"
)

// Flow is the business flow of the transaction. It is a separate dimension from
// Direction: a refund of an outbound TED is Flow=REFUND with Direction=IN.
type Flow string

const (
	// FlowCashOut is money leaving a client account towards a third party.
	FlowCashOut Flow = "CASH_OUT"

	// FlowCashIn is money arriving at a client account from a third party.
	FlowCashIn Flow = "CASH_IN"

	// FlowP2P is a transfer between two accounts of the same institution.
	FlowP2P Flow = "P2P"

	// FlowRefund is a return of a previously settled transaction. It requires
	// OriginalAggregateID.
	FlowRefund Flow = "REFUND"

	// FlowMED is a Pix special return mechanism (Mecanismo Especial de
	// Devolução) claim settlement.
	FlowMED Flow = "MED"

	// FlowAutomaticDebit is a scheduled/authorized recurring debit.
	FlowAutomaticDebit Flow = "AUTOMATIC_DEBIT"
)

// Direction is the money direction relative to the client account.
type Direction string

const (
	// DirectionIn is money entering the client account.
	DirectionIn Direction = "IN"

	// DirectionOut is money leaving the client account.
	DirectionOut Direction = "OUT"
)

var (
	rails      = []Rail{RailTED, RailPix}
	flows      = []Flow{FlowCashOut, FlowCashIn, FlowP2P, FlowRefund, FlowMED, FlowAutomaticDebit}
	directions = []Direction{DirectionIn, DirectionOut}
)

// Correlation is the canonical correlation contract between a transactional
// plugin and the Midaz ledger. It carries only identifiers and classification —
// never amounts, names, documents, or any other client data.
type Correlation struct {
	// Plugin is the producing plugin's identity, e.g. "br-bank-transfer" or
	// "br-pix-direct-jd". Required.
	Plugin string

	// Rail is the payment rail. Required.
	Rail Rail

	// Flow is the business flow. Required.
	Flow Flow

	// AggregateID is the id of the plugin-side record this transaction settles.
	// Required — it is the join key from the ledger back to the plugin.
	AggregateID string

	// EndToEndID is the rail-assigned end-to-end identifier, when the rail
	// assigns one. Optional.
	EndToEndID string

	// ProviderMessageID is the provider/partner message identifier. Optional.
	ProviderMessageID string

	// ProviderMessageCode is the provider/partner message type code. Optional.
	ProviderMessageCode string

	// OriginalAggregateID is the plugin-side id of the transaction being
	// returned. Required when Flow is FlowRefund, optional otherwise.
	OriginalAggregateID string

	// Direction is the money direction relative to the client account.
	// Optional.
	Direction Direction
}

// Validate reports whether the correlation can be emitted to the ledger: every
// required field is present, and Rail, Flow, and Direction hold known values.
// Errors name the offending field and value.
func (c Correlation) Validate() error {
	for _, required := range []struct {
		key   string
		value string
	}{
		{"plugin", c.Plugin},
		{"rail", string(c.Rail)},
		{"flow", string(c.Flow)},
		{"aggregateId", c.AggregateID},
	} {
		if isBlank(required.value) {
			return fmt.Errorf("%s is required", required.key)
		}
	}

	if !slices.Contains(rails, c.Rail) {
		return fmt.Errorf("rail %q is not a known rail: want TED or PIX", c.Rail)
	}

	if !slices.Contains(flows, c.Flow) {
		return fmt.Errorf(
			"flow %q is not a known flow: want CASH_OUT, CASH_IN, P2P, REFUND, MED or AUTOMATIC_DEBIT",
			c.Flow,
		)
	}

	if c.Direction != "" && !slices.Contains(directions, c.Direction) {
		return fmt.Errorf("direction %q is not a known direction: want IN or OUT", c.Direction)
	}

	if c.Flow == FlowRefund && isBlank(c.OriginalAggregateID) {
		return fmt.Errorf("originalAggregateId is required when flow is %s", FlowRefund)
	}

	return nil
}

// ToMetadata renders the correlation as the ledger metadata payload: the
// contract version plus every non-blank field, under the canonical camelCase
// keys. Blank fields are omitted entirely — the ledger never receives an empty
// value. Callers should Validate first; ToMetadata renders whatever it holds.
//
// Values are emitted trimmed, matching the presence rule Validate applies: a
// padded identifier passes validation, so emitting it verbatim would ship
// " agg-1 " as the aggregate id and break the exact-match lookup that is the
// key's whole purpose.
func (c Correlation) ToMetadata() map[string]any {
	metadata := map[string]any{"contractVersion": ContractVersion}

	for key, value := range map[string]string{
		"plugin":              c.Plugin,
		"rail":                string(c.Rail),
		"flow":                string(c.Flow),
		"aggregateId":         c.AggregateID,
		"endToEndId":          c.EndToEndID,
		"providerMessageId":   c.ProviderMessageID,
		"providerMessageCode": c.ProviderMessageCode,
		"originalAggregateId": c.OriginalAggregateID,
		"direction":           string(c.Direction),
	} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			metadata[key] = trimmed
		}
	}

	return metadata
}

// FromMetadata rebuilds a Correlation from a metadata payload ToMetadata
// produced. A key that is absent, or present with a non-string value, becomes an
// empty field — so the rebuilt correlation is exactly as valid as the payload,
// and FromMetadata(m).Validate() is the canonical way to check that a metadata
// map carries a conformant correlation (that is what
// correlationtest.AssertCanonical does, instead of re-implementing the rules).
//
// contractVersion is not a Correlation field: compare it to ContractVersion
// separately.
func FromMetadata(metadata map[string]any) Correlation {
	return Correlation{
		Plugin:              metadataString(metadata, "plugin"),
		Rail:                Rail(metadataString(metadata, "rail")),
		Flow:                Flow(metadataString(metadata, "flow")),
		AggregateID:         metadataString(metadata, "aggregateId"),
		EndToEndID:          metadataString(metadata, "endToEndId"),
		ProviderMessageID:   metadataString(metadata, "providerMessageId"),
		ProviderMessageCode: metadataString(metadata, "providerMessageCode"),
		OriginalAggregateID: metadataString(metadata, "originalAggregateId"),
		Direction:           Direction(metadataString(metadata, "direction")),
	}
}

// allFieldsSet is a Correlation with every field populated. Keys derives the
// contract's key set from it through ToMetadata, so the whitelist has one
// definition — the emitter — and no checker can hold a hand-copied second copy
// that drifts when the contract grows. TestKeysCoverEveryContractField pins the
// population.
var allFieldsSet = Correlation{
	Plugin:              "plugin",
	Rail:                RailTED,
	Flow:                FlowRefund,
	AggregateID:         "aggregateId",
	EndToEndID:          "endToEndId",
	ProviderMessageID:   "providerMessageId",
	ProviderMessageCode: "providerMessageCode",
	OriginalAggregateID: "originalAggregateId",
	Direction:           DirectionIn,
}

// Keys returns every metadata key contract version ContractVersion emits,
// sorted. It is the closed whitelist: a metadata key outside it is not part of
// this contract, and admitting one is a versioned contract change, never a
// per-plugin decision.
func Keys() []string {
	return slices.Sorted(maps.Keys(allFieldsSet.ToMetadata()))
}

func metadataString(metadata map[string]any, key string) string {
	text, _ := metadata[key].(string)

	return text
}

func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}
