package models

import (
	"fmt"
	"time"
)

// TransactionRoute represents a transaction route entity
type TransactionRoute struct {
	ID              string           `json:"id"`
	OrganizationID  string           `json:"organizationId"`
	LedgerID        string           `json:"ledgerId"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	OperationRoutes []OperationRoute `json:"operationRoutes"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
	DeletedAt       *time.Time       `json:"deletedAt,omitempty"`
}

// CreateTransactionRouteInput represents the input for creating a transaction route
type CreateTransactionRouteInput struct {
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	OperationRoutes []string       `json:"operationRoutes"` // Array of operation route IDs
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// NewCreateTransactionRouteInput creates a new CreateTransactionRouteInput with required fields
func NewCreateTransactionRouteInput(title, description string, operationRoutes []string) *CreateTransactionRouteInput {
	return &CreateTransactionRouteInput{
		Title:           title,
		Description:     description,
		OperationRoutes: operationRoutes,
	}
}

// WithMetadata sets the metadata for the transaction route
func (input *CreateTransactionRouteInput) WithMetadata(metadata map[string]any) *CreateTransactionRouteInput {
	input.Metadata = metadata
	return input
}

// Validate validates the CreateTransactionRouteInput
func (input *CreateTransactionRouteInput) Validate() error {
	if input.Title == "" {
		return fmt.Errorf("title is required")
	}

	if input.Description == "" {
		return fmt.Errorf("description is required")
	}

	if len(input.OperationRoutes) == 0 {
		return fmt.Errorf("operationRoutes must contain at least one operation route ID")
	}

	// Validate each operation route ID is not empty
	for i, routeID := range input.OperationRoutes {
		if routeID == "" {
			return fmt.Errorf("operationRoutes[%d] cannot be empty", i)
		}
	}

	return nil
}

// UpdateTransactionRouteInput represents the input for updating a transaction route
type UpdateTransactionRouteInput struct {
	Title           *string        `json:"title,omitempty"`
	Description     *string        `json:"description,omitempty"`
	OperationRoutes []string       `json:"operationRoutes,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// NewUpdateTransactionRouteInput creates a new UpdateTransactionRouteInput
func NewUpdateTransactionRouteInput() *UpdateTransactionRouteInput {
	return &UpdateTransactionRouteInput{}
}

// WithTitle sets the title for the update
func (input *UpdateTransactionRouteInput) WithTitle(title string) *UpdateTransactionRouteInput {
	input.Title = &title
	return input
}

// WithDescription sets the description for the update
func (input *UpdateTransactionRouteInput) WithDescription(description string) *UpdateTransactionRouteInput {
	input.Description = &description
	return input
}

// WithOperationRoutes sets the operation routes for the update
func (input *UpdateTransactionRouteInput) WithOperationRoutes(operationRoutes []string) *UpdateTransactionRouteInput {
	input.OperationRoutes = operationRoutes
	return input
}

// WithMetadata sets the metadata for the update
func (input *UpdateTransactionRouteInput) WithMetadata(metadata map[string]any) *UpdateTransactionRouteInput {
	input.Metadata = metadata
	return input
}

// Validate validates the UpdateTransactionRouteInput
func (input *UpdateTransactionRouteInput) Validate() error {
	// If operation routes are provided, validate them
	if len(input.OperationRoutes) > 0 {
		// Validate each operation route ID is not empty
		for i, routeID := range input.OperationRoutes {
			if routeID == "" {
				return fmt.Errorf("validation failed: operationRoutes[%d] cannot be empty", i)
			}
		}
	}

	return nil
}
