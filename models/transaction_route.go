package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation/core"
	"github.com/google/uuid"
)

// TransactionRoute is the SDK-native transaction route response (Track 7E — audit 7.1).
type TransactionRoute struct {
	ID              uuid.UUID        `json:"id,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	OrganizationID  uuid.UUID        `json:"organizationId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	LedgerID        uuid.UUID        `json:"ledgerId,omitempty" example:"01965ed9-7fa4-75b2-8872-fc9e8509ab0a"`
	Title           string           `json:"title,omitempty" example:"Charge Settlement"`
	Description     string           `json:"description,omitempty" example:"Settlement route for service charges"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	OperationRoutes []OperationRoute `json:"operationRoutes,omitempty"`
	CreatedAt       time.Time        `json:"createdAt" example:"2025-01-01T00:00:00Z"`
	UpdatedAt       time.Time        `json:"updatedAt" example:"2025-01-01T00:00:00Z"`
	DeletedAt       *time.Time       `json:"deletedAt" example:"2025-01-01T00:00:00Z"`
}

// CreateTransactionRouteInput is the SDK-native transaction-route creation payload.
type CreateTransactionRouteInput struct {
	Title           string         `json:"title,omitempty" example:"Charge Settlement"`
	Description     string         `json:"description,omitempty" example:"Settlement route for service charges"`
	Metadata        map[string]any `json:"metadata"`
	OperationRoutes []uuid.UUID    `json:"operationRoutes,omitempty" format:"uuid"`

	// parseErr stashes any UUID-parse failure from NewCreateTransactionRouteInput
	// so it can surface from Validate(). Unexported, deliberately tag-free.
	parseErr error
}

// Validate validates the CreateTransactionRouteInput fields.
func (input *CreateTransactionRouteInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	// parseErr from constructor takes precedence — caller passed
	// malformed UUIDs and the rest of validation is moot.
	if input.parseErr != nil {
		return input.parseErr
	}

	var errs validation.FieldErrors

	if input.Title == "" {
		errs.Append("title", "is required")
	} else {
		appendRouteTextLength(&errs, input.Title, maxRouteTitleLength, "title")
	}

	appendRouteTextLength(&errs, input.Description, maxRouteDescriptionLength, "description")

	if len(input.OperationRoutes) == 0 {
		errs.Append("operationRoutes", "is required")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// UpdateTransactionRouteInput is the SDK-native transaction-route patch payload.
//
// All fields use omitempty so a zero-valued PATCH never sends accidental
// nullify directives. RFC 7396 merge-patch treats explicit null as "remove",
// so emitting metadata: null when the caller never set it would silently
// wipe server-side metadata.
type UpdateTransactionRouteInput struct {
	Title           string         `json:"title,omitempty" example:"Charge Settlement"`
	Description     string         `json:"description,omitempty" example:"Settlement route for service charges"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	OperationRoutes *[]uuid.UUID   `json:"operationRoutes,omitempty" format:"uuid"`
}

// Validate validates the UpdateTransactionRouteInput fields.
func (input *UpdateTransactionRouteInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	var errs validation.FieldErrors

	appendRouteTextLength(&errs, input.Title, maxRouteTitleLength, "title")
	appendRouteTextLength(&errs, input.Description, maxRouteDescriptionLength, "description")

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

func (input *UpdateTransactionRouteInput) hasChanges() bool {
	if input == nil {
		return false
	}

	return input.Title != "" || input.Description != "" || input.Metadata != nil || input.OperationRoutes != nil
}

// NewCreateTransactionRouteInput creates a new CreateTransactionRouteInput with required fields.
//
// Parameters:
//   - title: Short text summarizing the purpose of the transaction
//   - description: A description for the Transaction Route
//
// Returns:
//   - A pointer to the newly created CreateTransactionRouteInput
func NewCreateTransactionRouteInput(title, description string, operationRoutes []string) *CreateTransactionRouteInput {
	// Convert string UUIDs to uuid.UUID type
	var parseErr error

	uuidRoutes := make([]uuid.UUID, 0, len(operationRoutes))

	for i, routeStr := range operationRoutes {
		routeUUID, err := uuid.Parse(routeStr)
		if err != nil {
			parseErr = fmt.Errorf("operationRoutes[%d] must be a valid UUID: %w", i, err)
			uuidRoutes = nil

			break
		}

		uuidRoutes = append(uuidRoutes, routeUUID)
	}

	return &CreateTransactionRouteInput{
		Title:           title,
		Description:     description,
		OperationRoutes: uuidRoutes,
		parseErr:        parseErr,
	}
}

// WithMetadata sets the metadata for CreateTransactionRouteInput.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateTransactionRouteInput for method chaining
func (input *CreateTransactionRouteInput) WithMetadata(metadata map[string]any) *CreateTransactionRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}

// NewUpdateTransactionRouteInput creates a new UpdateTransactionRouteInput.
//
// Returns:
//   - A pointer to the newly created UpdateTransactionRouteInput
func NewUpdateTransactionRouteInput() *UpdateTransactionRouteInput {
	return &UpdateTransactionRouteInput{}
}

// WithTitle sets the title for UpdateTransactionRouteInput.
//
// Parameters:
//   - title: The new title for the transaction route
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func (input *UpdateTransactionRouteInput) WithTitle(title string) *UpdateTransactionRouteInput {
	if input == nil {
		return nil
	}

	input.Title = title

	return input
}

// WithDescription sets the description for UpdateTransactionRouteInput.
//
// Parameters:
//   - description: The new description for the transaction route
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func (input *UpdateTransactionRouteInput) WithDescription(description string) *UpdateTransactionRouteInput {
	if input == nil {
		return nil
	}

	input.Description = description

	return input
}

// WithMetadata sets the metadata for UpdateTransactionRouteInput.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func (input *UpdateTransactionRouteInput) WithMetadata(metadata map[string]any) *UpdateTransactionRouteInput {
	if input == nil {
		return nil
	}

	input.Metadata = cloneAnyMap(metadata)

	return input
}
