package models

import (
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
)

// TransactionRoute is an alias for mmodel.TransactionRoute to maintain compatibility while using midaz entities.
type TransactionRoute = mmodel.TransactionRoute

// CreateTransactionRouteInput wraps mmodel.CreateTransactionRouteInput to maintain compatibility while using midaz entities.
type CreateTransactionRouteInput struct {
	mmodel.CreateTransactionRouteInput
	parseErr error
}

// Validate validates the CreateTransactionRouteInput fields.
func (input *CreateTransactionRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if input.Title == "" {
		return errors.New("title is required")
	}

	if err := validateRouteText(input.Title, maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if input.parseErr != nil {
		return input.parseErr
	}

	if len(input.OperationRoutes) == 0 {
		return errors.New("operationRoutes is required")
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// UpdateTransactionRouteInput wraps mmodel.UpdateTransactionRouteInput to maintain compatibility while using midaz entities.
type UpdateTransactionRouteInput struct {
	mmodel.UpdateTransactionRouteInput
}

// Validate validates the UpdateTransactionRouteInput fields.
func (input *UpdateTransactionRouteInput) Validate() error {
	if input == nil {
		return errors.New("input is required")
	}

	if !input.hasChanges() {
		return errors.New("empty update payload not allowed")
	}

	if err := validateRouteText(input.Title, maxRouteTitleLength, "title"); err != nil {
		return err
	}

	if err := validateRouteText(input.Description, maxRouteDescriptionLength, "description"); err != nil {
		return err
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
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
		CreateTransactionRouteInput: mmodel.CreateTransactionRouteInput{
			Title:           title,
			Description:     description,
			OperationRoutes: uuidRoutes,
		},
		parseErr: parseErr,
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
