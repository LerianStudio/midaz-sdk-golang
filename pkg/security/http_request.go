// Package security provides security-focused validation helpers used by the SDK.
package security

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ValidateOutboundRequest validates the minimal security requirements for outbound HTTP requests.
// It ensures requests are absolute and use an allowed HTTP scheme.
func ValidateOutboundRequest(req *http.Request) error {
	if req == nil {
		return errors.New("http request cannot be nil")
	}

	if req.URL == nil {
		return errors.New("http request URL cannot be nil")
	}

	if req.URL.Hostname() == "" {
		return errors.New("http request URL must include host")
	}

	scheme := strings.ToLower(req.URL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", req.URL.Scheme)
	}

	return nil
}
