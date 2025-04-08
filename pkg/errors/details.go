package errors

import (
	"fmt"
	"net/http"
	"strings"
)

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

	// Try to extract error code and HTTP status
	switch e := err.(type) {
	case interface{ Code() string }:
		details.Code = e.Code()
	case interface{ ErrorCode() string }:
		details.Code = e.ErrorCode()
	}

	// Try to extract HTTP status code
	switch e := err.(type) {
	case interface{ StatusCode() int }:
		details.HTTPStatus = e.StatusCode()
	case interface{ HTTPStatusCode() int }:
		details.HTTPStatus = e.HTTPStatusCode()
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
