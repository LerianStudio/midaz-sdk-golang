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

	if req.URL.User != nil {
		return errors.New("URL must not include user information")
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

// isIPv4LoopbackAlias returns true for any string that the OS resolver would
// canonically parse as an IPv4 address inside the 127.0.0.0/8 loopback
// block. The accepted shapes are:
//
//   - 4-octet form: 127.A.B.C, with each part 0-255 (no leading zeros).
//   - 2-octet short form: 127.N (treated as 127.0.0.N by traditional
//     inet_aton), with N 0-(2^24-1) — accepted when the trailing token is
//     numeric.
//
// The previous implementation accepted "127.999" and "127.0.0.256" because
// it only checked that every dot-separated part was a string of digits.
// We now bound each part to 0-255 (or, for the 2-octet form, the trailing
// integer to 24 bits) and reject leading-zero parts so we don't get
// confused by an octal-looking input.
func isIPv4LoopbackAlias(hostname string) bool {
	parts := strings.Split(hostname, ".")
	switch len(parts) {
	case 4:
		return isStrict127DotOctet(parts)
	case 2:
		return isShort127Form(parts)
	default:
		return false
	}
}

func isStrict127DotOctet(parts []string) bool {
	for i, part := range parts {
		if !isCanonicalOctet(part) {
			return false
		}

		if i == 0 && part != "127" {
			return false
		}
	}

	return true
}

func isShort127Form(parts []string) bool {
	if parts[0] != "127" {
		return false
	}

	tail := parts[1]
	if tail == "" || (len(tail) > 1 && tail[0] == '0') {
		return false
	}

	var n int

	for _, r := range tail {
		if r < '0' || r > '9' {
			return false
		}

		n = n*10 + int(r-'0')
		if n >= 1<<24 {
			return false
		}
	}

	return true
}

// isCanonicalOctet reports whether part is a decimal IPv4 octet in canonical
// form (0-255, no leading zeros except for "0" itself).
func isCanonicalOctet(part string) bool {
	if part == "" {
		return false
	}

	if len(part) > 1 && part[0] == '0' {
		return false
	}

	var n int

	for _, r := range part {
		if r < '0' || r > '9' {
			return false
		}

		n = n*10 + int(r-'0')
		if n > 255 {
			return false
		}
	}

	return true
}
