package errors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// codeError is an interface for errors that have a Code method
type codeError interface {
	Code() string
}

// errorCodeError is an interface for errors that have an ErrorCode method
type errorCodeError interface {
	ErrorCode() string
}

// statusCodeError is an interface for errors that have a StatusCode method
type statusCodeError interface {
	StatusCode() int
}

// httpStatusCodeError is an interface for errors that have an HTTPStatusCode method
type httpStatusCodeError interface {
	HTTPStatusCode() int
}

// ErrorDetails contains detailed information about an error
type ErrorDetails struct {
	// Message is the human-readable error message
	Message string

	// Code is the error code, if available
	Code string

	// APICode is the raw API error code, if available.
	APICode string

	// Title is the raw API error title, if available.
	Title string

	// EntityType is the raw API entity type, if available.
	EntityType string

	// Fields is the raw API field list, if available.
	Fields []string

	// Details contains raw API error details, if available.
	Details map[string]any

	// RequestID is the API request ID, if available.
	RequestID string

	// HTTPStatus is the HTTP status code, if available
	HTTPStatus int

	// OriginalError is the original error that occurred
	OriginalError error
}

// GetErrorDetails extracts detailed information from an error
func GetErrorDetails(err error) ErrorDetails {
	if isNilError(err) {
		return ErrorDetails{}
	}

	details := ErrorDetails{
		Message:       safeErrorString(err),
		OriginalError: err,
	}
	details.Code = extractErrorCode(err)
	details.HTTPStatus = extractErrorHTTPStatus(err)
	populateStructuredErrorDetails(err, &details)

	// If no status code was found, try to determine it from the error type
	if details.HTTPStatus == 0 {
		details.HTTPStatus = determineHTTPStatusFromError(err)
	}

	return details
}

func populateStructuredErrorDetails(err error, details *ErrorDetails) {
	var sdkErr *Error
	if !errors.As(err, &sdkErr) || sdkErr == nil || details == nil {
		return
	}

	details.APICode = sdkErr.APICode
	details.Title = sdkErr.Title
	details.EntityType = sdkErr.EntityType

	if sdkErr.Fields != nil {
		details.Fields = append([]string(nil), sdkErr.Fields...)
	}

	if sdkErr.Details != nil {
		clonedDetails := make(map[string]any, len(sdkErr.Details))
		for key, value := range sdkErr.Details {
			clonedDetails[key] = value
		}

		details.Details = clonedDetails
	}

	details.RequestID = sdkErr.RequestID
}

func extractErrorCode(err error) string {
	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return string(sdkErr.Code)
	}

	var midazErr *MidazError
	if errors.As(err, &midazErr) && midazErr != nil {
		return string(midazErr.Code)
	}

	// Try to extract error code using errors.As
	var (
		ce  codeError
		ece errorCodeError
	)

	if errors.As(err, &ce) {
		return ce.Code()
	}

	if errors.As(err, &ece) {
		return ece.ErrorCode()
	}

	return ""
}

func extractErrorHTTPStatus(err error) int {
	var sdkErr *Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sdkErr.StatusCode
	}

	// Try to extract HTTP status code using errors.As
	var (
		sce  statusCodeError
		hsce httpStatusCodeError
	)

	if errors.As(err, &sce) {
		return sce.StatusCode()
	}

	if errors.As(err, &hsce) {
		return hsce.HTTPStatusCode()
	}

	return 0
}

// determineHTTPStatusFromError tries to determine an appropriate HTTP status code
// based on the error type and message
func determineHTTPStatusFromError(err error) int {
	errString := strings.ToLower(safeErrorString(err))

	if strings.Contains(errString, "not found") {
		return http.StatusNotFound
	}

	if strings.Contains(errString, "permission") || strings.Contains(errString, "unauthorized") {
		return http.StatusUnauthorized
	}

	if strings.Contains(errString, "forbidden") {
		return http.StatusForbidden
	}

	if strings.Contains(errString, "invalid") || strings.Contains(errString, "bad request") {
		return http.StatusBadRequest
	}

	if strings.Contains(errString, "conflict") || strings.Contains(errString, "already exists") {
		return http.StatusConflict
	}

	if strings.Contains(errString, "timeout") || strings.Contains(errString, "deadline exceeded") {
		return http.StatusGatewayTimeout
	}

	if strings.Contains(errString, "rate limit") || strings.Contains(errString, "too many requests") {
		return http.StatusTooManyRequests
	}

	// Default to internal server error
	return http.StatusInternalServerError
}

// GetErrorStatusCode returns the HTTP status code for an error
func GetErrorStatusCode(err error) int {
	return GetErrorDetails(err).HTTPStatus
}

// FormatErrorDetails formats an error for display to the user
func FormatErrorDetails(err error) string {
	if isNilError(err) {
		return ""
	}

	details := GetErrorDetails(err)

	if details.Code != "" {
		return fmt.Sprintf("[%s] %s", details.Code, details.Message)
	}

	return details.Message
}

// FormatOperationError formats an error specific to transaction operations
func FormatOperationError(err error, operation string) string {
	if isNilError(err) {
		return ""
	}

	details := GetErrorDetails(err)

	if details.Code != "" {
		return fmt.Sprintf("%s failed: [%s] %s", operation, details.Code, details.Message)
	}

	return fmt.Sprintf("%s failed: %s", operation, details.Message)
}
