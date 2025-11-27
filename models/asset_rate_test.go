package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, 5.25, rate.Rate)
	assert.NotNil(t, rate.Scale)
	assert.Equal(t, 4.0, *rate.Scale)
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
			name:           "custom external id",
			externalID:     "rate-usd-brl-2024",
			wantExternalID: "rate-usd-brl-2024",
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
		WithExternalID("ext-rate-001").
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
	assert.Equal(t, "ext-rate-001", *input.ExternalID)
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
			input:   NewCreateAssetRateInput("USD", "BRL", 525).WithScale(2).WithSource("API").WithTTL(3600).WithExternalID("ext-1"),
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
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAssetRatesResponse(t *testing.T) {
	nextCursor := "next-abc123"
	prevCursor := "prev-xyz789"

	response := AssetRatesResponse{
		Items: []AssetRate{
			{
				ID:   "rate-1",
				From: "USD",
				To:   "BRL",
				Rate: 5.25,
			},
			{
				ID:   "rate-2",
				From: "EUR",
				To:   "BRL",
				Rate: 5.75,
			},
		},
		Limit:      10,
		NextCursor: &nextCursor,
		PrevCursor: &prevCursor,
	}

	assert.Len(t, response.Items, 2)
	assert.Equal(t, 10, response.Limit)
	assert.NotNil(t, response.NextCursor)
	assert.Equal(t, "next-abc123", *response.NextCursor)
	assert.NotNil(t, response.PrevCursor)
	assert.Equal(t, "prev-xyz789", *response.PrevCursor)
}

func TestAssetRatesResponseEmptyItems(t *testing.T) {
	response := AssetRatesResponse{
		Items: []AssetRate{},
		Limit: 10,
	}

	assert.Empty(t, response.Items)
	assert.Equal(t, 10, response.Limit)
	assert.Nil(t, response.NextCursor)
	assert.Nil(t, response.PrevCursor)
}

func TestAssetRatesResponseNilCursors(t *testing.T) {
	response := AssetRatesResponse{
		Items: []AssetRate{
			{ID: "rate-1", From: "USD", To: "BRL", Rate: 5.25},
		},
		Limit: 10,
	}

	assert.Nil(t, response.NextCursor)
	assert.Nil(t, response.PrevCursor)
}

func TestNewAssetRateListOptions(t *testing.T) {
	options := NewAssetRateListOptions()

	assert.NotNil(t, options)
	assert.Equal(t, DefaultLimit, options.Limit)
	assert.Equal(t, DefaultSortDirection, options.SortOrder)
	assert.Empty(t, options.To)
	assert.Empty(t, options.StartDate)
	assert.Empty(t, options.EndDate)
	assert.Empty(t, options.Cursor)
}

func TestAssetRateListOptionsWithTo(t *testing.T) {
	tests := []struct {
		name   string
		to     []string
		wantTo []string
	}{
		{
			name:   "single target asset",
			to:     []string{"BRL"},
			wantTo: []string{"BRL"},
		},
		{
			name:   "multiple target assets",
			to:     []string{"BRL", "EUR", "GBP"},
			wantTo: []string{"BRL", "EUR", "GBP"},
		},
		{
			name:   "empty target assets",
			to:     []string{},
			wantTo: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewAssetRateListOptions().WithTo(tt.to...)

			assert.Equal(t, tt.wantTo, options.To)
		})
	}
}

func TestAssetRateListOptionsWithLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{
			name:      "valid limit",
			limit:     25,
			wantLimit: 25,
		},
		{
			name:      "zero limit defaults to default",
			limit:     0,
			wantLimit: DefaultLimit,
		},
		{
			name:      "negative limit defaults to default",
			limit:     -5,
			wantLimit: DefaultLimit,
		},
		{
			name:      "limit exceeding max gets capped",
			limit:     150,
			wantLimit: MaxLimit,
		},
		{
			name:      "limit at max",
			limit:     MaxLimit,
			wantLimit: MaxLimit,
		},
		{
			name:      "limit at 1",
			limit:     1,
			wantLimit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewAssetRateListOptions().WithLimit(tt.limit)

			assert.Equal(t, tt.wantLimit, options.Limit)
		})
	}
}

func TestAssetRateListOptionsWithDateRange(t *testing.T) {
	tests := []struct {
		name          string
		startDate     string
		endDate       string
		wantStartDate string
		wantEndDate   string
	}{
		{
			name:          "valid date range",
			startDate:     "2024-01-01",
			endDate:       "2024-12-31",
			wantStartDate: "2024-01-01",
			wantEndDate:   "2024-12-31",
		},
		{
			name:          "same start and end date",
			startDate:     "2024-06-15",
			endDate:       "2024-06-15",
			wantStartDate: "2024-06-15",
			wantEndDate:   "2024-06-15",
		},
		{
			name:          "only start date",
			startDate:     "2024-01-01",
			endDate:       "",
			wantStartDate: "2024-01-01",
			wantEndDate:   "",
		},
		{
			name:          "only end date",
			startDate:     "",
			endDate:       "2024-12-31",
			wantStartDate: "",
			wantEndDate:   "2024-12-31",
		},
		{
			name:          "empty dates",
			startDate:     "",
			endDate:       "",
			wantStartDate: "",
			wantEndDate:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewAssetRateListOptions().WithDateRange(tt.startDate, tt.endDate)

			assert.Equal(t, tt.wantStartDate, options.StartDate)
			assert.Equal(t, tt.wantEndDate, options.EndDate)
		})
	}
}

func TestAssetRateListOptionsWithSortOrder(t *testing.T) {
	tests := []struct {
		name          string
		sortOrder     string
		wantSortOrder string
	}{
		{
			name:          "ascending order",
			sortOrder:     "asc",
			wantSortOrder: "asc",
		},
		{
			name:          "descending order",
			sortOrder:     "desc",
			wantSortOrder: "desc",
		},
		{
			name:          "empty order",
			sortOrder:     "",
			wantSortOrder: "",
		},
		{
			name:          "custom order value",
			sortOrder:     "custom",
			wantSortOrder: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewAssetRateListOptions().WithSortOrder(tt.sortOrder)

			assert.Equal(t, tt.wantSortOrder, options.SortOrder)
		})
	}
}

func TestAssetRateListOptionsWithCursor(t *testing.T) {
	tests := []struct {
		name       string
		cursor     string
		wantCursor string
	}{
		{
			name:       "valid cursor",
			cursor:     "abc123xyz",
			wantCursor: "abc123xyz",
		},
		{
			name:       "empty cursor",
			cursor:     "",
			wantCursor: "",
		},
		{
			name:       "long cursor",
			cursor:     "eyJsYXN0X2lkIjoiMTIzNDU2Nzg5MCIsImxhc3RfdmFsdWUiOiIyMDI0LTAxLTE1VDEwOjMwOjAwWiJ9",
			wantCursor: "eyJsYXN0X2lkIjoiMTIzNDU2Nzg5MCIsImxhc3RfdmFsdWUiOiIyMDI0LTAxLTE1VDEwOjMwOjAwWiJ9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := NewAssetRateListOptions().WithCursor(tt.cursor)

			assert.Equal(t, tt.wantCursor, options.Cursor)
		})
	}
}

func TestAssetRateListOptionsBuilderChaining(t *testing.T) {
	options := NewAssetRateListOptions().
		WithTo("BRL", "EUR", "GBP").
		WithLimit(50).
		WithDateRange("2024-01-01", "2024-12-31").
		WithSortOrder("asc").
		WithCursor("cursor123")

	assert.Equal(t, []string{"BRL", "EUR", "GBP"}, options.To)
	assert.Equal(t, 50, options.Limit)
	assert.Equal(t, "2024-01-01", options.StartDate)
	assert.Equal(t, "2024-12-31", options.EndDate)
	assert.Equal(t, "asc", options.SortOrder)
	assert.Equal(t, "cursor123", options.Cursor)
}

func TestAssetRateListOptionsToQueryParams(t *testing.T) {
	tests := []struct {
		name       string
		options    *AssetRateListOptions
		wantParams map[string]string
	}{
		{
			name:       "default options",
			options:    NewAssetRateListOptions(),
			wantParams: map[string]string{"limit": "10", "sort_order": "desc"},
		},
		{
			name: "single to asset",
			options: &AssetRateListOptions{
				To:        []string{"BRL"},
				Limit:     10,
				SortOrder: "desc",
			},
			wantParams: map[string]string{"to": "BRL", "limit": "10", "sort_order": "desc"},
		},
		{
			name: "multiple to assets",
			options: &AssetRateListOptions{
				To:        []string{"BRL", "EUR", "GBP"},
				Limit:     10,
				SortOrder: "desc",
			},
			wantParams: map[string]string{"to": "BRL,EUR,GBP", "limit": "10", "sort_order": "desc"},
		},
		{
			name: "all options set",
			options: &AssetRateListOptions{
				To:        []string{"BRL", "EUR"},
				Limit:     25,
				StartDate: "2024-01-01",
				EndDate:   "2024-12-31",
				SortOrder: "asc",
				Cursor:    "cursor123",
			},
			wantParams: map[string]string{
				"to":         "BRL,EUR",
				"limit":      "25",
				"start_date": "2024-01-01",
				"end_date":   "2024-12-31",
				"sort_order": "asc",
				"cursor":     "cursor123",
			},
		},
		{
			name: "with date range only",
			options: &AssetRateListOptions{
				Limit:     10,
				StartDate: "2024-06-01",
				EndDate:   "2024-06-30",
			},
			wantParams: map[string]string{"limit": "10", "start_date": "2024-06-01", "end_date": "2024-06-30"},
		},
		{
			name: "with cursor only",
			options: &AssetRateListOptions{
				Limit:  10,
				Cursor: "xyz789",
			},
			wantParams: map[string]string{"limit": "10", "cursor": "xyz789"},
		},
		{
			name: "zero limit not included",
			options: &AssetRateListOptions{
				Limit: 0,
			},
			wantParams: map[string]string{},
		},
		{
			name: "empty to array not included",
			options: &AssetRateListOptions{
				To:    []string{},
				Limit: 10,
			},
			wantParams: map[string]string{"limit": "10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := tt.options.ToQueryParams()

			assert.Equal(t, len(tt.wantParams), len(params))
			for key, expectedValue := range tt.wantParams {
				assert.Equal(t, expectedValue, params[key], "mismatch for key %s", key)
			}
		})
	}
}

func TestAssetRateListOptionsToQueryParamsEmptyOptions(t *testing.T) {
	options := &AssetRateListOptions{}
	params := options.ToQueryParams()

	assert.Empty(t, params)
}

func TestCreateAssetRateInputValidateEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateAssetRateInput
		wantErr bool
	}{
		{
			name: "rate at boundary (1)",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "BRL",
				Rate: 1,
			},
			wantErr: false,
		},
		{
			name: "scale at zero",
			input: &CreateAssetRateInput{
				From:  "USD",
				To:    "BRL",
				Rate:  100,
				Scale: 0,
			},
			wantErr: false,
		},
		{
			name: "very large scale",
			input: &CreateAssetRateInput{
				From:  "USD",
				To:    "BRL",
				Rate:  100,
				Scale: 18,
			},
			wantErr: false,
		},
		{
			name: "three character asset codes",
			input: &CreateAssetRateInput{
				From: "USD",
				To:   "EUR",
				Rate: 92,
			},
			wantErr: false,
		},
		{
			name: "lowercase asset codes",
			input: &CreateAssetRateInput{
				From: "usd",
				To:   "eur",
				Rate: 92,
			},
			wantErr: false,
		},
		{
			name: "mixed case asset codes",
			input: &CreateAssetRateInput{
				From: "Usd",
				To:   "EuR",
				Rate: 92,
			},
			wantErr: false,
		},
		{
			name: "numeric asset codes",
			input: &CreateAssetRateInput{
				From: "840",
				To:   "978",
				Rate: 92,
			},
			wantErr: false,
		},
		{
			name: "special characters in asset code",
			input: &CreateAssetRateInput{
				From: "USD-TEST",
				To:   "BRL_TEST",
				Rate: 100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAssetRateListOptionsToQueryParamsWithBuilder(t *testing.T) {
	options := NewAssetRateListOptions().
		WithTo("BRL", "EUR").
		WithLimit(50).
		WithDateRange("2024-01-01", "2024-06-30").
		WithSortOrder("asc").
		WithCursor("page2")

	params := options.ToQueryParams()

	assert.Equal(t, "BRL,EUR", params["to"])
	assert.Equal(t, "50", params["limit"])
	assert.Equal(t, "2024-01-01", params["start_date"])
	assert.Equal(t, "2024-06-30", params["end_date"])
	assert.Equal(t, "asc", params["sort_order"])
	assert.Equal(t, "page2", params["cursor"])
}

func TestCreateAssetRateInputImmutability(t *testing.T) {
	input := NewCreateAssetRateInput("USD", "BRL", 525)

	input.WithScale(2)

	assert.Equal(t, 2, input.Scale)

	input.WithScale(4)

	assert.Equal(t, 4, input.Scale)
}

func TestAssetRateListOptionsReplacement(t *testing.T) {
	options := NewAssetRateListOptions().
		WithTo("BRL").
		WithTo("EUR", "GBP")

	assert.Equal(t, []string{"EUR", "GBP"}, options.To)
	assert.NotContains(t, options.To, "BRL")
}
