package entities

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
)

func validateUpdatePayload(operation string, input any, typedName string) error {
	if isNilAny(input) {
		return sdkerrors.NewMissingParameterError(operation, "input")
	}

	if validator, ok := input.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return sdkerrors.NewValidationError(operation, "update validation failed", err)
		}

		return nil
	}

	switch typed := input.(type) {
	case models.UpdateOperationInput:
		if err := (&typed).Validate(); err != nil {
			return sdkerrors.NewValidationError(operation, "update validation failed", err)
		}

		return nil
	case models.UpdateTransactionInput:
		if err := (&typed).Validate(); err != nil {
			return sdkerrors.NewValidationError(operation, "update validation failed", err)
		}

		return nil
	}

	if payload, ok := input.(map[string]any); ok {
		return validateUpdateMapPayload(operation, payload, typedName)
	}

	return sdkerrors.NewValidationError(operation, "unsupported update input", fmt.Errorf("use %s", typedName))
}

func validateUpdateMapPayload(operation string, payload map[string]any, typedName string) error {
	metadata, ok := payload["metadata"]
	if !ok || metadata == nil {
		return nil
	}

	metadataMap, ok := metadata.(map[string]any)
	if !ok {
		return sdkerrors.NewValidationError(operation, "invalid metadata", fmt.Errorf("metadata must be an object; use %s", typedName))
	}

	if err := core.ValidateMetadata(metadataMap); err != nil {
		return sdkerrors.NewValidationError(operation, "invalid metadata", err)
	}

	return nil
}
