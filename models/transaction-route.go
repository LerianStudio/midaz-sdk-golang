package models

import (
	"fmt"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
	"github.com/google/uuid"
)

// TransactionRoute is an alias for mmodel.TransactionRoute to maintain compatibility while using midaz entities.
type TransactionRoute = mmodel.TransactionRoute

// CreateTransactionRouteInput wraps mmodel.CreateTransactionRouteInput to maintain compatibility while using midaz entities.
type CreateTransactionRouteInput struct {
	mmodel.CreateTransactionRouteInput
}

// Validate validates the CreateTransactionRouteInput fields.
func (input *CreateTransactionRouteInput) Validate() error {
	if input.Title == "" {
		return fmt.Errorf("title is required")
	}
	if input.Description == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// UpdateTransactionRouteInput wraps mmodel.UpdateTransactionRouteInput to maintain compatibility while using midaz entities.
type UpdateTransactionRouteInput struct {
	mmodel.UpdateTransactionRouteInput
}

// Validate validates the UpdateTransactionRouteInput fields.
func (input *UpdateTransactionRouteInput) Validate() error {
	// For updates, fields are optional so validation is minimal
	return nil
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
	uuidRoutes := make([]uuid.UUID, len(operationRoutes))
	for i, routeStr := range operationRoutes {
		if routeUUID, err := uuid.Parse(routeStr); err == nil {
			uuidRoutes[i] = routeUUID
		}
		// If parsing fails, we'll use a zero UUID
	}

	return &CreateTransactionRouteInput{
		CreateTransactionRouteInput: mmodel.CreateTransactionRouteInput{
			Title:           title,
			Description:     description,
			OperationRoutes: uuidRoutes,
		},
	}
}

// WithTransactionRouteMetadata sets the metadata for CreateTransactionRouteInput.
//
// Parameters:
//   - input: The CreateTransactionRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified CreateTransactionRouteInput for method chaining
func WithTransactionRouteMetadata(input *CreateTransactionRouteInput, metadata map[string]any) *CreateTransactionRouteInput {
	input.Metadata = metadata
	return input
}

// WithMetadata sets the metadata for CreateTransactionRouteInput (method on struct).
func (input *CreateTransactionRouteInput) WithMetadata(metadata map[string]any) *CreateTransactionRouteInput {
	input.Metadata = metadata
	return input
}

// NewUpdateTransactionRouteInput creates a new UpdateTransactionRouteInput.
//
// Returns:
//   - A pointer to the newly created UpdateTransactionRouteInput
func NewUpdateTransactionRouteInput() *UpdateTransactionRouteInput {
	return &UpdateTransactionRouteInput{}
}

// WithUpdateTransactionRouteTitle sets the title for UpdateTransactionRouteInput.
//
// Parameters:
//   - input: The UpdateTransactionRouteInput to modify
//   - title: The new title for the transaction route
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func WithUpdateTransactionRouteTitle(input *UpdateTransactionRouteInput, title string) *UpdateTransactionRouteInput {
	input.Title = title
	return input
}

// WithUpdateTransactionRouteDescription sets the description for UpdateTransactionRouteInput.
//
// Parameters:
//   - input: The UpdateTransactionRouteInput to modify
//   - description: The new description for the transaction route
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func WithUpdateTransactionRouteDescription(input *UpdateTransactionRouteInput, description string) *UpdateTransactionRouteInput {
	input.Description = description
	return input
}

// WithUpdateTransactionRouteMetadata sets the metadata for UpdateTransactionRouteInput.
//
// Parameters:
//   - input: The UpdateTransactionRouteInput to modify
//   - metadata: A map of key-value pairs to store as metadata
//
// Returns:
//   - A pointer to the modified UpdateTransactionRouteInput for method chaining
func WithUpdateTransactionRouteMetadata(input *UpdateTransactionRouteInput, metadata map[string]any) *UpdateTransactionRouteInput {
	input.Metadata = metadata
	return input
}