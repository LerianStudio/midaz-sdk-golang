package entities

import (
	"net/http"
)

// This file contains compatibility functions for backward compatibility
// with existing code that uses the old service constructors without observability.

// GetWrappedHTTPClient gets an HTTP client with nil observability.
// This is for backward compatibility with existing code.
func GetWrappedHTTPClient(client *http.Client, _ string) *http.Client {
	// Return the original client since we can't wrap it at this level
	return client
}
