package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetRateStruct(t *testing.T) {
	scale := 4.0
	source := "Central Bank"
	now := time.Now()

	rate := AssetRate{
		ID:             "rate-123",
		OrganizationID: "org-456",
		LedgerID:       "ledger-789",
		ExternalID:     "ext-001",
		From:           "USD",
		To:             "BRL",
		Rate:           5.25,
		Scale:          &scale,
		Source:         &source,
		TTL:            3600,
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata: map[string]any{
			"provider": "forex-api",
		},
	}

	assert.Equal(t, "rate-123", rate.ID)
	assert.Equal(t, "org-456", rate.OrganizationID)
	assert.Equal(t, "ledger-789", rate.LedgerID)
	assert.Equal(t, "ext-001", rate.ExternalID)
	assert.Equal(t, "USD", rate.From)
	assert.Equal(t, "BRL", rate.To)
	assert.InDelta(t, 5.25, rate.Rate, 0.001)
	assert.NotNil(t, rate.Scale)
	assert.InDelta(t, 4.0, *rate.Scale, 0.001)
	assert.NotNil(t, rate.Source)
	assert.Equal(t, "Central Bank", *rate.Source)
	assert.Equal(t, 3600, rate.TTL)
	assert.Equal(t, now, rate.CreatedAt)
	assert.Equal(t, now, rate.UpdatedAt)
	assert.Equal(t, "forex-api", rate.Metadata["provider"])
}

func TestAssetRateStructWithNilOptionalFields(t *testing.T) {
	rate := AssetRate{
		ID:   "rate-123",
		From: "USD",
		To:   "EUR",
		Rate: 0.92,
	}

	assert.Equal(t, "rate-123", rate.ID)
	assert.Equal(t, "USD", rate.From)
	assert.Equal(t, "EUR", rate.To)
	assert.InDelta(t, 0.92, rate.Rate, 0.001)
	assert.Nil(t, rate.Scale)
	assert.Nil(t, rate.Source)
	assert.Nil(t, rate.Metadata)
}

func TestNewCreateAssetRateInput(t *testing.T) {
	tests := []struct {
		name     string
		from     string
		to       string
		rate     int
		wantFrom string
		wantTo   string
		wantRate int
	}{
		{
			name:     "USD to BRL conversion",
			from:     "USD",
			to:       "BRL",
			rate:     525,
			wantFrom: "USD",
			wantTo:   "BRL",
			wantRate: 525,
		},
		{
			name:     "EUR to USD conversion",
			from:     "EUR",
			to:       "USD",
			rate:     108,
			wantFrom: "EUR",
			wantTo:   "USD",
			wantRate: 108,
		},
		{
			name:     "same currency rate",
			from:     "USD",
			to:       "USD",
			rate:     100,
			wantFrom: "USD",
			wantTo:   "USD",
			wantRate: 100,
		},
		{
			name:     "crypto currency conversion",
			from:     "BTC",
			to:       "USD",
			rate:     4350000,
			wantFrom: "BTC",
			wantTo:   "USD",
			wantRate: 4350000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput(tt.from, tt.to, tt.rate)

			assert.NotNil(t, input)
			assert.Equal(t, tt.wantFrom, input.From)
			assert.Equal(t, tt.wantTo, input.To)
			assert.Equal(t, tt.wantRate, input.Rate)
			assert.Equal(t, 0, input.Scale)
			assert.Nil(t, input.Source)
			assert.Nil(t, input.TTL)
			assert.Nil(t, input.ExternalID)
			assert.Nil(t, input.Metadata)
		})
	}
}

func TestCreateAssetRateInputWithScale(t *testing.T) {
	tests := []struct {
		name      string
		scale     int
		wantScale int
	}{
		{
			name:      "scale of 2",
			scale:     2,
			wantScale: 2,
		},
		{
			name:      "scale of 4",
			scale:     4,
			wantScale: 4,
		},
		{
			name:      "scale of 0",
			scale:     0,
			wantScale: 0,
		},
		{
			name:      "high precision scale",
			scale:     8,
			wantScale: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput("USD", "BRL", 525).WithScale(tt.scale)

			assert.Equal(t, tt.wantScale, input.Scale)
		})
	}
}

func TestCreateAssetRateInputWithSource(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantSource string
	}{
		{
			name:       "central bank source",
			source:     "Central Bank",
			wantSource: "Central Bank",
		},
		{
			name:       "forex api source",
			source:     "Forex API",
			wantSource: "Forex API",
		},
		{
			name:       "manual source",
			source:     "Manual Entry",
			wantSource: "Manual Entry",
		},
		{
			name:       "empty source",
			source:     "",
			wantSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput("USD", "BRL", 525).WithSource(tt.source)

			assert.NotNil(t, input.Source)
			assert.Equal(t, tt.wantSource, *input.Source)
		})
	}
}

func TestCreateAssetRateInputWithTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     int
		wantTTL int
	}{
		{
			name:    "1 hour TTL",
			ttl:     3600,
			wantTTL: 3600,
		},
		{
			name:    "24 hour TTL",
			ttl:     86400,
			wantTTL: 86400,
		},
		{
			name:    "1 minute TTL",
			ttl:     60,
			wantTTL: 60,
		},
		{
			name:    "zero TTL",
			ttl:     0,
			wantTTL: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput("USD", "BRL", 525).WithTTL(tt.ttl)

			assert.NotNil(t, input.TTL)
			assert.Equal(t, tt.wantTTL, *input.TTL)
		})
	}
}

func TestCreateAssetRateInputWithExternalID(t *testing.T) {
	tests := []struct {
		name           string
		externalID     string
		wantExternalID string
	}{
		{
			name:           "uuid external id",
			externalID:     "550e8400-e29b-41d4-a716-446655440000",
			wantExternalID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:           "another uuid external id",
			externalID:     "550e8400-e29b-41d4-a716-446655440001",
			wantExternalID: "550e8400-e29b-41d4-a716-446655440001",
		},
		{
			name:           "empty external id",
			externalID:     "",
			wantExternalID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput("USD", "BRL", 525).WithExternalID(tt.externalID)

			assert.NotNil(t, input.ExternalID)
			assert.Equal(t, tt.wantExternalID, *input.ExternalID)
		})
	}
}

func TestCreateAssetRateInputWithMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadata     map[string]any
		wantMetadata map[string]any
	}{
		{
			name: "single key metadata",
			metadata: map[string]any{
				"provider": "forex-api",
			},
			wantMetadata: map[string]any{
				"provider": "forex-api",
			},
		},
		{
			name: "multiple keys metadata",
			metadata: map[string]any{
				"provider":   "central-bank",
				"region":     "latam",
				"confidence": 0.99,
			},
			wantMetadata: map[string]any{
				"provider":   "central-bank",
				"region":     "latam",
				"confidence": 0.99,
			},
		},
		{
			name:         "empty metadata",
			metadata:     map[string]any{},
			wantMetadata: map[string]any{},
		},
		{
			name:         "nil metadata",
			metadata:     nil,
			wantMetadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetRateInput("USD", "BRL", 525).WithMetadata(tt.metadata)

			assert.Equal(t, tt.wantMetadata, input.Metadata)
		})
	}
}

func TestCreateAssetRateInputBuilderChaining(t *testing.T) {
	metadata := map[string]any{
		"provider": "forex-api",
		"region":   "global",
	}

	input := NewCreateAssetRateInput("USD", "BRL", 52500).
		WithScale(4).
		WithSource("Central Bank").
		WithTTL(3600).
		WithExternalID("550e8400-e29b-41d4-a716-446655440002").
		WithMetadata(metadata)

	assert.Equal(t, "USD", input.From)
	assert.Equal(t, "BRL", input.To)
	assert.Equal(t, 52500, input.Rate)
	assert.Equal(t, 4, input.Scale)
	assert.NotNil(t, input.Source)
	assert.Equal(t, "Central Bank", *input.Source)
	assert.NotNil(t, input.TTL)
	assert.Equal(t, 3600, *input.TTL)
	assert.NotNil(t, input.ExternalID)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440002", *input.ExternalID)
	assert.Equal(t, metadata, input.Metadata)
}

func TestCreateAssetRateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateAssetRateInput
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid input with all required fields",
			input:   NewCreateAssetRateInput("USD", "BRL", 525),
			wantErr: false,
		},
		{
			name:    "valid input with scale",
			input:   NewCreateAssetRateInput("USD", "BRL", 52500).WithScale(4),
			wantErr: false,
		},
		{
			name:    "valid input with all optional fields",
			input:   NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2).WithSource("API").WithTTL(3600).WithExternalID("550e8400-e29b-41d4-a716-446655440003"),
			wantErr: false,
		},
		{
			name: "empty from asset code",
			input: &CreateAssetRateInput{
				From: "",
				To:   "BRL",
				Rate: 525,
			},
			wantErr: true,
			errMsg:  "from asset code is required",
		},
		{
			name: "empty to asset code",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "",
				Rate: 525,
			},
			wantErr: true,
			errMsg:  "to asset code is required",
		},
		{
			name: "zero rate",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "BRL",
				Rate: 0,
			},
			wantErr: true,
			errMsg:  "rate must be greater than zero",
		},
		{
			name: "negative rate",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "BRL",
				Rate: -100,
			},
			wantErr: true,
			errMsg:  "rate must be greater than zero",
		},
		{
			name: "negative scale",
			input: &CreateAssetRateInput{
				From:  "USD",
				To:    "BRL",
				Rate:  525,
				Scale: -1,
			},
			wantErr: true,
			errMsg:  "scale must be non-negative",
		},
		{
			name: "both from and to empty",
			input: &CreateAssetRateInput{
				From: "",
				To:   "",
				Rate: 525,
			},
			wantErr: true,
			errMsg:  "from asset code is required",
		},
		{
			name: "whitespace only from",
			input: &CreateAssetRateInput{
				From: "   ",
				To:   "BRL",
				Rate: 525,
			},
			wantErr: false,
		},
		{
			name: "valid rate of 1",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "USD",
				Rate: 1,
			},
			wantErr: false,
		},
		{
			name: "large rate value",
			input: &CreateAssetRateInput{
				From: "BTC",
				To:   "USD",
				Rate: 4350000000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantErr {
				require.Error(t, err)
				// 8C: Validate accumulates field errors. Substring
				// match instead of full equality so multi-field
				// cases ("both from and to empty") that surface
				// MORE diagnostics still pass the contract.
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
