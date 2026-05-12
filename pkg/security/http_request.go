package security

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

const (
	maxIPv4Octet = 255
	maxRedirects = 10
)

// ValidateOutboundRequest validates the minimal security requirements for outbound HTTP requests.
// It ensures requests are absolute and use an allowed HTTP scheme.
func ValidateOutboundRequest(req *http.Request) error {
	return validateOutboundRequest(req, false)
}

func validateOutboundRequest(req *http.Request, allowInsecureHTTP bool) error {
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

	if scheme == "http" && !allowInsecureHTTP && !isLocalhost(req.URL.Hostname()) {
		return fmt.Errorf("insecure HTTP is only allowed for localhost targets: %s", req.URL.Host)
	}

	return nil
}

// ValidateRedirect validates an SDK-owned HTTP redirect target and rejects
// cross-origin redirects when the previous request carried sensitive headers.
func ValidateRedirect(req *http.Request, via []*http.Request) error {
	return ValidateRedirectWithInsecureHTTP(req, via, false)
}

// ValidateRedirectWithInsecureHTTP validates a redirect target while optionally
// allowing plain HTTP for non-local targets. This is only for explicitly
// trusted in-cluster Access Manager flows.
func ValidateRedirectWithInsecureHTTP(req *http.Request, via []*http.Request, allowInsecureHTTP bool) error {
	if err := validateOutboundRequest(req, allowInsecureHTTP); err != nil {
		return err
	}

	if len(via) >= maxRedirects {
		return errors.New("stopped after 10 redirects")
	}

	if len(via) == 0 {
		return nil
	}

	previous := via[len(via)-1]
	if isSensitiveCrossOriginRedirect(previous, req) {
		return errors.New("refusing authenticated redirect to a different origin")
	}

	return nil
}

// EnsureRedirectPolicy returns a shallow client copy that enforces the SDK
// redirect policy before any caller-provided redirect policy.
func EnsureRedirectPolicy(client *http.Client) *http.Client {
	if client == nil {
		return client
	}

	clientCopy := *client
	callerRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := ValidateRedirect(req, via); err != nil {
			return err
		}
		if callerRedirect != nil {
			return callerRedirect(req, via)
		}

		return nil
	}

	return &clientCopy
}

func isSensitiveCrossOriginRedirect(previous, next *http.Request) bool {
	if previous == nil || next == nil || sameOrigin(previous.URL, next.URL) {
		return false
	}

	return hasSensitiveRedirectHeaders(previous) || requestMayReplaySensitiveBody(previous)
}

func requestMayReplaySensitiveBody(req *http.Request) bool {
	if req == nil {
		return false
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method != "" && method != http.MethodGet && method != http.MethodHead {
		return true
	}

	return req.Body != nil || req.GetBody != nil
}

func hasSensitiveRedirectHeaders(req *http.Request) bool {
	if req == nil {
		return false
	}

	for key := range req.Header {
		if isSensitiveHeaderName(key) {
			return true
		}
	}

	return false
}

func sameOrigin(previous, next *url.URL) bool {
	if previous == nil || next == nil {
		return false
	}

	return strings.EqualFold(previous.Scheme, next.Scheme) && strings.EqualFold(previous.Host, next.Host)
}

func isSensitiveHeaderName(name string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(name)))
	if normalized == "" {
		return false
	}

	for _, marker := range []string{"authorization", "cookie", "token", "secret", "password", "apikey", "idempotency", "tenant", "organization"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	return false
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
		if n > maxIPv4Octet {
			return false
		}
	}

	return true
}
