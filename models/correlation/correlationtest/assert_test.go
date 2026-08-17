package correlationtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models/correlation"
)

func canonicalMetadata() map[string]any {
	return correlation.Correlation{
		Plugin:      "br-bank-transfer",
		Rail:        correlation.RailTED,
		Flow:        correlation.FlowCashOut,
		AggregateID: "7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10",
		Direction:   correlation.DirectionOut,
	}.ToMetadata()
}

// canonicalInput is a create input a conformant plugin producer would build:
// correlation metadata on the transaction, exactly one route representation.
func canonicalInput() *models.CreateTransactionInput {
	routeID := "1f3c5a7e-9b2d-4e6f-8a1c-3d5b7f9e1a20"

	return &models.CreateTransactionInput{
		RouteID:  routeID,
		Metadata: canonicalMetadata(),
		Send: &models.SendInput{
			Asset: "BRL",
			Value: "100.00",
			Source: &models.SourceInput{From: []models.FromToInput{{
				AccountAlias: "@external/BRL",
				Amount:       models.AmountInput{Asset: "BRL", Value: "100.00"},
				RouteID:      &routeID,
			}}},
			Distribute: &models.DistributeInput{To: []models.FromToInput{{
				AccountAlias: "customer_john_doe",
				Amount:       models.AmountInput{Asset: "BRL", Value: "100.00"},
				RouteID:      &routeID,
			}}},
		},
	}
}

func TestAssertCanonicalAcceptsConformantInput(t *testing.T) {
	AssertCanonical(t, canonicalInput())
}

func TestViolations(t *testing.T) {
	route := "cash-out"

	tests := []struct {
		name      string
		mutate    func(input *models.CreateTransactionInput)
		wantParts []string
	}{
		{
			name:   "canonical input has no violations",
			mutate: func(_ *models.CreateTransactionInput) {},
		},
		{
			name: "metadata key outside the whitelist",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["payerDocument"] = "12345678900"
			},
			wantParts: []string{`"payerDocument"`, "whitelist"},
		},
		{
			name: "fee namespace keys are reserved and allowed",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["feeRuleId"] = "5b8d1e3f-7a2c-4d6e-9f1b-2c4a6e8d0f30"
			},
		},
		{
			name: "missing required key",
			mutate: func(input *models.CreateTransactionInput) {
				delete(input.Metadata, "plugin")
			},
			wantParts: []string{`"plugin"`, "required"},
		},
		{
			name: "blank required key",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["aggregateId"] = "   "
			},
			wantParts: []string{`"aggregateId"`, "empty"},
		},
		{
			name: "nil metadata reports every required key",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata = nil
			},
			wantParts: []string{
				`"contractVersion"`,
				`"plugin"`,
				`"rail"`,
				`"flow"`,
				`"aggregateId"`,
			},
		},
		{
			name: "foreign contract version",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["contractVersion"] = "7"
			},
			wantParts: []string{"contractVersion", `"7"`, `"1"`},
		},
		{
			name: "route and routeId together on the transaction",
			mutate: func(input *models.CreateTransactionInput) {
				input.Route = route
			},
			wantParts: []string{"transaction", `"route"`, `"routeId"`},
		},
		{
			name: "route and routeId together on a source leg",
			mutate: func(input *models.CreateTransactionInput) {
				input.Send.Source.From[0].Route = route
			},
			wantParts: []string{"send.source.from[0]", `"route"`, `"routeId"`},
		},
		{
			name: "route and routeId together on a distribute leg",
			mutate: func(input *models.CreateTransactionInput) {
				input.Send.Distribute.To[0].Route = route
			},
			wantParts: []string{"send.distribute.to[0]", `"route"`, `"routeId"`},
		},
		{
			name: "leg metadata key outside the whitelist",
			mutate: func(input *models.CreateTransactionInput) {
				input.Send.Distribute.To[0].Metadata = map[string]any{"payerName": "John Doe"}
			},
			wantParts: []string{"send.distribute.to[0]", `"payerName"`, "whitelist"},
		},
		{
			name: "only one route representation per leg is fine",
			mutate: func(input *models.CreateTransactionInput) {
				input.Send.Source.From[0].RouteID = nil
				input.Send.Source.From[0].Route = route
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := canonicalInput()
			tt.mutate(input)

			got := violations(input)

			if len(tt.wantParts) == 0 {
				assert.Empty(t, got)

				return
			}

			require.NotEmpty(t, got)

			joined := ""
			for _, violation := range got {
				joined += violation + "\n"
			}

			for _, part := range tt.wantParts {
				assert.Contains(t, joined, part)
			}
		})
	}
}

func TestViolationsRejectsNilInput(t *testing.T) {
	got := violations(nil)

	require.Len(t, got, 1)
	assert.Contains(t, got[0], "nil")
}
