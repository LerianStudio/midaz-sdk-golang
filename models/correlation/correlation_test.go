package correlation

import (
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
	for _, rail := range []Rail{RailTED, RailPix} {
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
			name:      "unknown rail names the value",
			mutate:    func(c *Correlation) { c.Rail = "TEF" },
			wantParts: []string{"rail", `"TEF"`},
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

func TestContractVersionIsOne(t *testing.T) {
	assert.Equal(t, "1", ContractVersion)
}
