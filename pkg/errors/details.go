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

	// HTTPStatus is the HTTP status code, if available
	HTTPStatus int

	// OriginalError is the original error that occurred
	OriginalError error
}

// GetErrorDetails extracts detailed information from an error
func GetErrorDetails(err error) ErrorDetails {
	if err == nil {
		return ErrorDetails{}
	}

	details := ErrorDetails{
		Message:       err.Error(),
		OriginalError: err,
	}

	// Try to extract error code using errors.As
	var (
		ce  codeError
		ece errorCodeError
	)

	if errors.As(err, &ce) {
		details.Code = ce.Code()
	} else if errors.As(err, &ece) {
		details.Code = ece.ErrorCode()
	}

	// Try to extract HTTP status code using errors.As
	var (
		sce  statusCodeError
		hsce httpStatusCodeError
	)

	if errors.As(err, &sce) {
		details.HTTPStatus = sce.StatusCode()
	} else if errors.As(err, &hsce) {
		details.HTTPStatus = hsce.HTTPStatusCode()
	}

	// If no status code was found, try to determine it from the error type
	if details.HTTPStatus == 0 {
		details.HTTPStatus = determineHTTPStatusFromError(err)
	}

	return details
}

// determineHTTPStatusFromError tries to determine an appropriate HTTP status code
// based on the error type and message
func determineHTTPStatusFromError(err error) int {
	errString := strings.ToLower(err.Error())

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
	if err == nil {
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
	if err == nil {
		return ""
	}

	details := GetErrorDetails(err)

	if details.Code != "" {
		return fmt.Sprintf("%s failed: [%s] %s", operation, details.Code, details.Message)
	}

	return fmt.Sprintf("%s failed: %s", operation, details.Message)
}
