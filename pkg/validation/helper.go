package validation

// isValidMetadataValueType checks if a value is of a type supported in metadata
// This function is needed by enhanced.go
func isValidMetadataValueType(value any) bool {
	switch value.(type) {
	case string, bool, int, float64, nil:
		return true
	default:
		return false
	}
}

// validateMetadataSize validates the total size of metadata
// This function is needed by enhanced.go
func validateMetadataSize(metadata map[string]any) error {
	totalSize := 0
	for key, value := range metadata {
		totalSize += len(key)
		switch v := value.(type) {
		case string:
			totalSize += len(v)
		case bool, int, float64:
			totalSize += 8 // Approximate size for these types
		}
	}

	if totalSize > 4096 {
		return ErrMetadataSizeExceeded
	}

	return nil
}

// ErrMetadataSizeExceeded is returned when metadata exceeds the maximum allowed size
var ErrMetadataSizeExceeded = &ValidationError{Message: "total metadata size exceeds maximum allowed size of 4KB"}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Message
}
