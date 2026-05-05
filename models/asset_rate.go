// Package models defines the data models used by the Midaz SDK.
package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/google/uuid"
)

const (
	maxAssetRateCodeLength   = 10
	maxAssetRateSourceLength = 200
	maxAssetRateScale        = 18
)

// AssetRate represents a conversion rate between two assets in the Midaz system.
// It contains information about the source and target assets, the conversion rate,
// and metadata about when the rate was created and updated.
type AssetRate struct {
	// ID is the unique identifier for the asset rate
	ID string `json:"id"`

	// OrganizationID is the ID of the organization that owns this asset rate
	OrganizationID string `json:"organizationId"`

	// LedgerID is the ID of the ledger containing this asset rate
	LedgerID string `json:"ledgerId"`

	// ExternalID is an external identifier for integration with third-party systems
	ExternalID string `json:"externalId"`

	// From is the source asset code (e.g., "USD")
	From string `json:"from"`

	// To is the target asset code (e.g., "BRL")
	To string `json:"to"`

	// Rate is the conversion rate value
	Rate float64 `json:"rate"`

	// Scale is the decimal places for the rate
	Scale *float64 `json:"scale"`

	// Source is the source of rate information (e.g., "Central Bank")
	Source *string `json:"source"`

	// TTL is the time-to-live in seconds
	TTL int `json:"ttl"`

	// CreatedAt is the timestamp when the asset rate was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the asset rate was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// Metadata contains additional custom attributes
	Metadata map[string]any `json:"metadata"`
}

// CreateAssetRateInput is the input payload to create or update an asset rate.
// It contains the required and optional fields for setting up asset conversion rates.
type CreateAssetRateInput struct {
	// From is the source asset code (required)
	From string `json:"from"`

	// To is the target asset code (required)
	To string `json:"to"`

	// Rate is the conversion rate value (required)
	Rate int `json:"rate"`

	// Scale is the decimal places for the rate (optional)
	Scale int `json:"scale,omitempty"`

	// Source is the source of rate information (optional)
	Source *string `json:"source,omitempty"`

	// TTL is the time-to-live in seconds (optional)
	TTL *int `json:"ttl,omitempty"`

	// ExternalID is an external identifier for integration (optional)
	ExternalID *string `json:"externalId,omitempty"`

	// Metadata contains additional custom attributes (optional)
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewCreateAssetRateInput creates a new CreateAssetRateInput with required fields.
//
// Parameters:
//   - from: The source asset code (e.g., "USD")
//   - to: The target asset code (e.g., "BRL")
//   - rate: The conversion rate value
//
// Returns:
//   - A new CreateAssetRateInput instance with the specified fields
func NewCreateAssetRateInput(from, to string, rate int) *CreateAssetRateInput {
	return &CreateAssetRateInput{
		From: from,
		To:   to,
		Rate: rate,
	}
}

// WithScale sets the decimal places for the rate.
func (input *CreateAssetRateInput) WithScale(scale int) *CreateAssetRateInput {
	if input == nil {
		return nil
	}

	input.Scale = scale

	return input
}

// WithSource sets the source of rate information.
func (input *CreateAssetRateInput) WithSource(source string) *CreateAssetRateInput {
	if input == nil {
		return nil
	}

	input.Source = &source

	return input
}

// WithTTL sets the time-to-live in seconds.
func (input *CreateAssetRateInput) WithTTL(ttl int) *CreateAssetRateInput {
	if input == nil {
		return nil
	}

	input.TTL = &ttl

	return input
}

// WithExternalID sets the external identifier for integration.
func (input *CreateAssetRateInput) WithExternalID(externalID string) *CreateAssetRateInput {
	if input == nil {
		return nil
	}

	input.ExternalID = &externalID

	return input
}

// WithMetadata sets the metadata for the asset rate.
func (input *CreateAssetRateInput) WithMetadata(metadata map[string]any) *CreateAssetRateInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// Validate validates the CreateAssetRateInput fields.
func (input *CreateAssetRateInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if err := input.validateAssetCodes(); err != nil {
		return err
	}

	if err := input.validateRateFields(); err != nil {
		return err
	}

	return input.validateOptionalFields()
}

func (input *CreateAssetRateInput) validateAssetCodes() error {
	if input.From == "" {
		return errors.New("from asset code is required")
	}

	if len(input.From) > maxAssetRateCodeLength {
		return fmt.Errorf("from asset code must be at most %d characters", maxAssetRateCodeLength)
	}

	if input.To == "" {
		return errors.New("to asset code is required")
	}

	if len(input.To) > maxAssetRateCodeLength {
		return fmt.Errorf("to asset code must be at most %d characters", maxAssetRateCodeLength)
	}

	return nil
}

func (input *CreateAssetRateInput) validateRateFields() error {
	if input.Rate <= 0 {
		return errors.New("rate must be greater than zero")
	}

	if input.Scale < 0 {
		return errors.New("scale must be non-negative")
	}

	if input.Scale > maxAssetRateScale {
		return fmt.Errorf("scale must be at most %d", maxAssetRateScale)
	}

	return nil
}

func (input *CreateAssetRateInput) validateOptionalFields() error {
	if input.Source != nil && len(*input.Source) > maxAssetRateSourceLength {
		return fmt.Errorf("source must be at most %d characters", maxAssetRateSourceLength)
	}

	if input.TTL != nil && *input.TTL < 0 {
		return errors.New("ttl must be non-negative")
	}

	if input.ExternalID != nil && strings.TrimSpace(*input.ExternalID) != "" {
		if _, err := uuid.Parse(*input.ExternalID); err != nil {
			return fmt.Errorf("externalID must be a valid UUID: %w", err)
		}
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}
