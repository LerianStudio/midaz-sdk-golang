// Package security provides security-focused validation helpers used by the SDK.
package security

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
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

	if scheme == "http" && !isLocalhost(req.URL.Hostname()) {
		return fmt.Errorf("insecure HTTP is only allowed for localhost targets: %s", req.URL.Host)
	}

	return nil
}

func isLocalhost(hostname string) bool {
	hostname = strings.TrimSuffix(strings.TrimSpace(strings.ToLower(hostname)), ".")
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}

	if addr, err := netip.ParseAddr(hostname); err == nil {
		return addr.IsLoopback()
	}

	if isIPv4LoopbackAlias(hostname) {
		return true
	}

	// RFC 6761 §6.3: ".localhost" is a reserved special-use TLD that resolvers
	// must treat as loopback. Used by Docker Compose aliases and dev tooling.
	return strings.HasSuffix(hostname, ".localhost")
}

func isIPv4LoopbackAlias(hostname string) bool {
	if hostname == "127" {
		return true
	}

	if !strings.HasPrefix(hostname, "127.") {
		return false
	}

	for _, part := range strings.Split(hostname, ".") {
		if part == "" {
			return false
		}

		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}

	return true
}
