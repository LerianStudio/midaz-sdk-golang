package entities

import (
	"strings"

	"github.com/google/uuid"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

func crmHeaders(organizationID string) map[string]string {
	return map[string]string{"X-Organization-Id": strings.TrimSpace(organizationID)}
}

func validateCRMOrganizationID(operation, organizationID string) (string, error) {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return "", errors.NewMissingParameterError(operation, "organizationID")
	}

	return organizationID, nil
}

func validateCRMUUIDParam(operation, name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.NewMissingParameterError(operation, name)
	}

	if _, err := uuid.Parse(value); err != nil {
		return "", errors.NewValidationError(operation, name+" must be a valid UUID", err)
	}

	return value, nil
}

func validateOptionalCRMUUIDParam(operation, name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if _, err := uuid.Parse(value); err != nil {
		return "", errors.NewValidationError(operation, name+" must be a valid UUID", err)
	}

	return value, nil
}
