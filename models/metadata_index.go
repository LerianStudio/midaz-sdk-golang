package models

import (
	"errors"
	"regexp"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
)

// MetadataIndex represents a custom metadata index.
type MetadataIndex struct {
	IndexName   string      `json:"indexName"`
	EntityName  string      `json:"entityName"`
	MetadataKey string      `json:"metadataKey"`
	Unique      bool        `json:"unique"`
	Sparse      bool        `json:"sparse"`
	Stats       *IndexStats `json:"stats,omitempty"`
}

// IndexStats represents usage statistics for a metadata index.
type IndexStats struct {
	Accesses   int64      `json:"accesses"`
	StatsSince *time.Time `json:"statsSince,omitempty"`
}

// CreateMetadataIndexInput is the payload for creating a metadata index.
type CreateMetadataIndexInput struct {
	MetadataKey string `json:"metadataKey"`
	Unique      bool   `json:"unique"`
	Sparse      *bool  `json:"sparse,omitempty"`
}

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// Validate validates the CreateMetadataIndexInput fields.
func (input *CreateMetadataIndexInput) Validate() error {
	if input == nil {
		return errors.New("input cannot be nil")
	}

	var errs validation.FieldErrors

	switch {
	case input.MetadataKey == "":
		errs.Append("metadataKey", "is required")
	case len(input.MetadataKey) > 100:
		errs.Append("metadataKey", "must be at most 100 characters")
	case !metadataKeyPattern.MatchString(input.MetadataKey):
		errs.Append("metadataKey", "must start with a letter and contain only letters, numbers, or underscores")
	}

	return errs.OrNil()
}

// NewCreateMetadataIndexInput creates a new CreateMetadataIndexInput.
func NewCreateMetadataIndexInput(metadataKey string) *CreateMetadataIndexInput {
	return &CreateMetadataIndexInput{
		MetadataKey: metadataKey,
	}
}

// WithUnique sets whether the metadata index should be unique.
func (input *CreateMetadataIndexInput) WithUnique(unique bool) *CreateMetadataIndexInput {
	if input == nil {
		return nil
	}

	input.Unique = unique

	return input
}

// WithSparse sets whether the metadata index should be sparse.
func (input *CreateMetadataIndexInput) WithSparse(sparse bool) *CreateMetadataIndexInput {
	if input == nil {
		return nil
	}

	input.Sparse = &sparse

	return input
}

// IsValidMetadataIndexEntity reports whether entityName supports metadata indexes.
func IsValidMetadataIndexEntity(entityName string) bool {
	switch entityName {
	case "organization", "ledger", "segment", "account", "portfolio", "asset", "account_type",
		"transaction", "operation", "operation_route", "transaction_route":
		return true
	default:
		return false
	}
}
