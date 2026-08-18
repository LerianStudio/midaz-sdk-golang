package correlation

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonical is a minimally valid correlation: every required field set, no
// optional field set.
func canonical() Correlation {
	return Correlation{
		Plugin:      "br-bank-transfer",
		Rail:        RailTED,
		Flow:        FlowCashOut,
		AggregateID: "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	}
}

func TestValidateAcceptsEveryRail(t *testing.T) {
	for _, rail := range []Rail{RailTED, RailPix, RailInternal} {
		t.Run(string(rail), func(t *testing.T) {
			c := canonical()
			c.Rail = rail

			require.NoError(t, c.Validate())
		})
	}
}

func TestValidateAcceptsEveryFlow(t *testing.T) {
	for _, flow := range []Flow{
		FlowCashOut,
		FlowCashIn,
		FlowP2P,
		FlowRefund,
		FlowMED,
		FlowAutomaticDebit,
	} {
		t.Run(string(flow), func(t *testing.T) {
			c := canonical()
			c.Flow = flow

			if flow == FlowRefund {
				c.OriginalAggregateID = "0c2d4f6a-8b1e-4c3d-9a5f-1e7b3d9c2f40"
			}

			require.NoError(t, c.Validate())
		})
	}
}

func TestValidateAcceptsEveryDirectionAndAbsence(t *testing.T) {
	for _, direction := range []Direction{"", DirectionIn, DirectionOut} {
		t.Run("direction="+string(direction), func(t *testing.T) {
			c := canonical()
			c.Direction = direction

			require.NoError(t, c.Validate())
		})
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(c *Correlation)
		wantParts []string
	}{
		{
			name:      "missing plugin",
			mutate:    func(c *Correlation) { c.Plugin = "" },
			wantParts: []string{"plugin", "required"},
		},
		{
			name:      "blank plugin",
			mutate:    func(c *Correlation) { c.Plugin = "   " },
			wantParts: []string{"plugin", "required"},
		},
		{
			name:      "missing rail",
			mutate:    func(c *Correlation) { c.Rail = "" },
			wantParts: []string{"rail", "required"},
		},
		{
			name:      "missing flow",
			mutate:    func(c *Correlation) { c.Flow = "" },
			wantParts: []string{"flow", "required"},
		},
		{
			name:      "missing aggregate id",
			mutate:    func(c *Correlation) { c.AggregateID = "" },
			wantParts: []string{"aggregateId", "required"},
		},
		{
			name:      "unknown rail names the value and the accepted set",
			mutate:    func(c *Correlation) { c.Rail = "TEF" },
			wantParts: []string{"rail", `"TEF"`, "want TED, PIX or INTERNAL"},
		},
		{
			name:      "unknown flow names the value",
			mutate:    func(c *Correlation) { c.Flow = "CASHOUT" },
			wantParts: []string{"flow", `"CASHOUT"`},
		},
		{
			name:      "unknown direction names the value",
			mutate:    func(c *Correlation) { c.Direction = "BOTH" },
			wantParts: []string{"direction", `"BOTH"`},
		},
		{
			name:      "refund without original aggregate id",
			mutate:    func(c *Correlation) { c.Flow = FlowRefund },
			wantParts: []string{"originalAggregateId", string(FlowRefund)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := canonical()
			tt.mutate(&c)

			err := c.Validate()
			require.Error(t, err)

			for _, part := range tt.wantParts {
				assert.Contains(t, err.Error(), part)
			}
		})
	}
}

func TestToMetadataEmitsEveryWhitelistedKey(t *testing.T) {
	c := Correlation{
		Plugin:              "br-pix-direct-jd",
		Rail:                RailPix,
		Flow:                FlowRefund,
		AggregateID:         "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
		EndToEndID:          "E1234567820260817120000abcdef123",
		ProviderMessageID:   "msg-9911",
		ProviderMessageCode: "PACS008",
		OriginalAggregateID: "0c2d4f6a-8b1e-4c3d-9a5f-1e7b3d9c2f40",
		Direction:           DirectionIn,
	}
	require.NoError(t, c.Validate())

	assert.Equal(t, map[string]any{
		"contractVersion":     ContractVersion,
		"plugin":              "br-pix-direct-jd",
		"rail":                "PIX",
		"flow":                "REFUND",
		"aggregateId":         "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
		"endToEndId":          "E1234567820260817120000abcdef123",
		"providerMessageId":   "msg-9911",
		"providerMessageCode": "PACS008",
		"originalAggregateId": "0c2d4f6a-8b1e-4c3d-9a5f-1e7b3d9c2f40",
		"direction":           "IN",
	}, c.ToMetadata())
}

// A book transfer between two accounts of the same institution touches no
// external rail, and Rail is required — RailInternal is the value that lets such
// a transfer emit a conformant correlation instead of borrowing TED or PIX.
func TestToMetadataEmitsInternalRailForBookTransfer(t *testing.T) {
	c := Correlation{
		Plugin:      "br-bank-transfer",
		Rail:        RailInternal,
		Flow:        FlowP2P,
		AggregateID: "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	}
	require.NoError(t, c.Validate())

	assert.Equal(t, map[string]any{
		"contractVersion": ContractVersion,
		"plugin":          "br-bank-transfer",
		"rail":            "INTERNAL",
		"flow":            "P2P",
		"aggregateId":     "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	}, c.ToMetadata())
}

func TestToMetadataOmitsEmptyAndBlankOptionalKeys(t *testing.T) {
	c := canonical()
	c.EndToEndID = "   "
	c.ProviderMessageCode = ""

	assert.Equal(t, map[string]any{
		"contractVersion": ContractVersion,
		"plugin":          "br-bank-transfer",
		"rail":            "TED",
		"flow":            "CASH_OUT",
		"aggregateId":     "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	}, c.ToMetadata())
}

// A padded identifier passes Validate (presence is decided after trimming), so
// the emitter has to trim too: aggregateId is the exact-match join key from the
// ledger back to the plugin, and " agg-1 " matches nothing.
func TestToMetadataTrimsEmittedValues(t *testing.T) {
	c := canonical()
	c.Plugin = "  br-bank-transfer  "
	c.AggregateID = " 7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10\t"
	c.EndToEndID = " E1234567820260817120000abcdef123 "
	require.NoError(t, c.Validate())

	assert.Equal(t, map[string]any{
		"contractVersion": ContractVersion,
		"plugin":          "br-bank-transfer",
		"rail":            "TED",
		"flow":            "CASH_OUT",
		"aggregateId":     "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
		"endToEndId":      "E1234567820260817120000abcdef123",
	}, c.ToMetadata())
}

// Keys is the whitelist every conformance checker consumes, and it is derived
// from allFieldsSet through ToMetadata — so allFieldsSet must populate every
// field of Correlation, or a real contract key would silently drop out of the
// whitelist and become "unknown" to the checkers.
func TestKeysCoverEveryContractField(t *testing.T) {
	value := reflect.ValueOf(allFieldsSet)
	for i := range value.NumField() {
		assert.NotEmpty(t, value.Field(i).String(),
			"allFieldsSet leaves %s empty, so Keys() would omit its metadata key",
			value.Type().Field(i).Name)
	}

	assert.Equal(t, []string{
		"aggregateId",
		"contractVersion",
		"direction",
		"endToEndId",
		"flow",
		"originalAggregateId",
		"plugin",
		"providerMessageCode",
		"providerMessageId",
		"rail",
	}, Keys())
}

// The emitter and the reader are two halves of one contract: anything ToMetadata
// writes, FromMetadata must read back, so a checker rebuilding a Correlation
// from ledger metadata sees what the producer put there.
func TestFromMetadataRoundTripsEveryField(t *testing.T) {
	require.NoError(t, allFieldsSet.Validate())

	assert.Equal(t, allFieldsSet, FromMetadata(allFieldsSet.ToMetadata()))
}

// A metadata map that is missing keys, or carries non-string values under them,
// rebuilds into an invalid Correlation instead of a plausible-looking one.
func TestFromMetadataTreatsMissingAndNonStringKeysAsEmpty(t *testing.T) {
	rebuilt := FromMetadata(map[string]any{
		"plugin":      "br-bank-transfer",
		"rail":        42,
		"flow":        nil,
		"aggregateId": "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	})

	assert.Equal(t, Correlation{
		Plugin:      "br-bank-transfer",
		AggregateID: "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
	}, rebuilt)
	require.Error(t, rebuilt.Validate())
}

func TestContractVersionIsOne(t *testing.T) {
	assert.Equal(t, "1", ContractVersion)
}
