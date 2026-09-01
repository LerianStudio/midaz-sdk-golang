package correlationtest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models/correlation"
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

// recordingTB is a testing.TB that records Errorf calls instead of failing, so
// the exported gate can be tested for FAILING. Only Helper and Errorf are ever
// called by AssertCanonical; the embedded nil interface would panic on anything
// else, which is the point — it keeps this stub honest about what it stands in
// for.
type recordingTB struct {
	testing.TB

	reported []string
}

func (*recordingTB) Helper() {}

func (tb *recordingTB) Errorf(format string, args ...any) {
	tb.reported = append(tb.reported, fmt.Sprintf(format, args...))
}

func TestAssertCanonicalAcceptsConformantInput(t *testing.T) {
	AssertCanonical(t, canonicalInput())
}

// The gate has to actually fail. Every rule below is asserted through the
// EXPORTED AssertCanonical against a recording TB, so replacing its body with a
// no-op (or swapping Errorf for Logf) breaks this test — the 14 Phase 2/3
// producers depend on that wiring, not on the unexported helper.
func TestAssertCanonicalReportsEveryFailureMode(t *testing.T) {
	route := "cash-out"

	tests := []struct {
		name      string
		mutate    func(input *models.CreateTransactionInput)
		wantParts []string
	}{
		{
			name:      "conformant input reports nothing",
			mutate:    func(_ *models.CreateTransactionInput) {},
			wantParts: nil,
		},
		{
			name: "missing required key",
			mutate: func(input *models.CreateTransactionInput) {
				delete(input.Metadata, "plugin")
			},
			wantParts: []string{"plugin", "required"},
		},
		{
			name: "metadata key outside the whitelist",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["payerDocument"] = "12345678900"
			},
			wantParts: []string{`"payerDocument"`, "whitelist"},
		},
		{
			name: "route and routeId together on the transaction",
			mutate: func(input *models.CreateTransactionInput) {
				input.Route = route
			},
			wantParts: []string{"transaction", `"route"`, `"routeId"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := canonicalInput()
			tt.mutate(input)

			recorder := &recordingTB{}
			AssertCanonical(recorder, input)

			if len(tt.wantParts) == 0 {
				assert.Empty(t, recorder.reported)

				return
			}

			require.NotEmpty(t, recorder.reported, "AssertCanonical reported no failure")

			joined := strings.Join(recorder.reported, "\n")
			assert.Contains(t, joined, "correlation contract v"+correlation.ContractVersion)

			for _, part := range tt.wantParts {
				assert.Contains(t, joined, part)
			}
		})
	}
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
			// The whitelist is closed: "fee"-prefixed keys are not a reserved
			// namespace, because a prefix admits feePayerDocument and
			// feedbackFromCustomer along with any real fee key.
			name: "fee-prefixed keys are outside the whitelist",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["feeRuleId"] = "5b8d1e3f-7a2c-4d6e-9f1b-2c4a6e8d0f30"
				input.Metadata["feePayerDocument"] = "12345678900"
				input.Metadata["feedbackFromCustomer"] = "Joao da Silva"
				input.Metadata["fee"] = "1.00"
			},
			wantParts: []string{
				`"fee"`,
				`"feeRuleId"`,
				`"feePayerDocument"`,
				`"feedbackFromCustomer"`,
				"whitelist",
			},
		},
		{
			name: "missing required key",
			mutate: func(input *models.CreateTransactionInput) {
				delete(input.Metadata, "plugin")
			},
			wantParts: []string{"plugin", "required"},
		},
		{
			name: "blank required key",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["aggregateId"] = "   "
			},
			wantParts: []string{"aggregateId", "required"},
		},
		{
			name: "non-string required key",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["aggregateId"] = 42
			},
			wantParts: []string{"aggregateId", "required"},
		},
		{
			// Validate trims before checking presence, so a padded id is a
			// "valid" correlation. The raw map is what the create request
			// serializes, and the ledger lookup is exact-match, so padding must
			// be rejected as non-canonical.
			name: "padded required value is not canonically serialized",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["aggregateId"] = " 7f1c9e2a-0b45-4a1e-9f3d-2c8b5d6e7a10 "
			},
			wantParts: []string{"aggregateId", "canonical"},
		},
		{
			name: "present-but-blank optional value is not canonically serialized",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["endToEndId"] = ""
			},
			wantParts: []string{"endToEndId", "canonical"},
		},
		{
			name: "padded optional value is not canonically serialized",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["endToEndId"] = " E2E123 "
			},
			wantParts: []string{"endToEndId", "canonical"},
		},
		{
			name: "nil metadata reports the contract version and the first missing field",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata = nil
			},
			wantParts: []string{`"contractVersion"`, "plugin is required"},
		},
		{
			name: "foreign contract version",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["contractVersion"] = "7"
			},
			wantParts: []string{"contractVersion", `"7"`, `"1"`},
		},
		{
			// Value checks, not just presence: these four passed the old
			// presence-only helper while the producer's own Validate rejected
			// them.
			name: "rail outside the enum",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["rail"] = "TEF"
			},
			wantParts: []string{"rail", `"TEF"`},
		},
		{
			name: "flow outside the enum",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["flow"] = "CASHOUT"
			},
			wantParts: []string{"flow", `"CASHOUT"`},
		},
		{
			name: "direction outside the enum",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["direction"] = "BOTH"
			},
			wantParts: []string{"direction", `"BOTH"`},
		},
		{
			name: "refund without the original aggregate id",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["flow"] = string(correlation.FlowRefund)
			},
			wantParts: []string{"originalAggregateId", "required"},
		},
		{
			name: "refund naming its original aggregate is conformant",
			mutate: func(input *models.CreateTransactionInput) {
				input.Metadata["flow"] = string(correlation.FlowRefund)
				input.Metadata["originalAggregateId"] = "0c2d4f6a-8b1e-4c3d-9a5f-1e7b3d9c2f40"
			},
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

			joined := strings.Join(got, "\n")
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
