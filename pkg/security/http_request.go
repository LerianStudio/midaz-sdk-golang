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

// ErrAuthenticatedRedirect is the sentinel returned by [ValidateRedirect]
// (and the wrapper in [EnsureRedirectPolicy]) when a cross-origin
// redirect is refused because the previous request carried
// credential-bearing headers or a replayable body. Callers that need to
// distinguish this rejection from a generic redirect failure should
// match via [errors.Is] rather than scraping the rendered string.
var ErrAuthenticatedRedirect = errors.New("refusing authenticated redirect to a different origin")

// ValidateOutboundRequest validates the minimal security requirements for outbound HTTP requests.
// It ensures requests are absolute and use an allowed HTTP scheme.
func ValidateOutboundRequest(req *http.Request) error {
	return validateOutboundRequest(req, false)
}

// ValidateOutboundRequestWithInsecureHTTP validates the minimal security
// requirements for outbound HTTP requests while optionally permitting
// plain http:// for non-loopback hosts. The scheme allowlist
// (http/https), userinfo rejection, missing-host rejection, and
// nil-request guard are always enforced; allowInsecureHTTP gates only
// the "http:// only for localhost" rule.
//
// SECURITY: this lifts a deliberate transport-security gate. Callers
// must reach the target over an inherently secure channel that http://
// cannot represent — typically a Kubernetes cluster-internal service
// reached via the service mesh, or a dev/test deployment behind a
// controlled network boundary. Never enable for traffic crossing the
// public internet.
//
// Wired into the SDK by the Config-layer [pkg/config.WithAllowInsecureHTTP]
// (data plane) and [pkg/config.WithAllowInsecureAccessManagerHTTP] (auth
// plane); callers building their own HTTP path on top of pkg/security can
// use this directly.
func ValidateOutboundRequestWithInsecureHTTP(req *http.Request, allowInsecureHTTP bool) error {
	return validateOutboundRequest(req, allowInsecureHTTP)
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
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}

	if len(via) == 0 {
		return nil
	}

	previous := via[len(via)-1]
	if isSensitiveCrossOriginRedirect(previous, req) {
		return ErrAuthenticatedRedirect
	}

	return nil
}

// EnsureRedirectPolicy returns a SHALLOW client copy whose CheckRedirect
// enforces the SDK redirect policy before delegating to any
// caller-supplied policy.
//
// Shallow-copy semantics — important:
//
//   - The returned *http.Client is a NEW value: mutating its CheckRedirect
//     does not affect the input client and vice versa.
//
//   - The underlying [http.Client.Transport], [http.Client.Jar], and
//     [http.Client.Timeout] are SHARED via field assignment. Changes to
//     the input client's Transport (e.g. setting TLSClientConfig) AFTER
//     EnsureRedirectPolicy returns will silently flow through to every
//     request made via the returned wrapper. Callers should finalize
//     their HTTP-client configuration BEFORE passing it to
//     WithHTTPClient (which calls EnsureRedirectPolicy internally).
//
//   - Concurrency: the shared Transport is safe for concurrent use per
//     the net/http contract. The CheckRedirect closure captures the
//     caller's original CheckRedirect by value at wrap time; later
//     reassignments to the input client's CheckRedirect are NOT
//     observed by the wrapper.
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

// requestMayReplaySensitiveBody reports whether following a redirect on
// this request could leak the previous request's body to the new
// destination.
//
// Any non-safe HTTP method (anything other than GET / HEAD) is flagged
// REGARDLESS of whether the request body is empty. POST without a body
// is still a state-mutating call: the server treats the redirect target
// as the new mutation site, which is exactly the credential-replay
// shape we want the cross-origin guard to refuse. Fail-closed here
// avoids a subtle gap where "POST /logout" with an empty body would
// otherwise be allowed to replay to a foreign origin.
//
// For safe methods we additionally check req.Body / req.GetBody so a
// GET with a buffered body (rare but valid) is also caught — net/http
// replays such bodies on follow-up requests.
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

// sameOrigin compares two URLs using strict equality on scheme + host.
//
// "Strict" is intentional and fail-closed. We do NOT normalize either side
// before comparing because:
//
//   - Trailing-dot hostnames ("example.com.") are valid FQDN forms but DNS
//     resolvers may treat them as a distinct identity from the bare form.
//     Treating them as cross-origin is the safe baseline — a redirect that
//     introduces or removes the trailing dot is suspicious in the
//     credential-replay threat model this guard exists to defeat.
//
//   - Explicit-default-port forms ("example.com:443" for HTTPS, ":80" for
//     HTTP) also compare unequal to the bare host. Same reasoning: a
//     redirect that flips the port representation is a signal worth
//     refusing, even if the resolved network endpoint is identical.
//
// Callers that legitimately need to canonicalize hosts before the SDK runs
// this guard should do so in their HTTP transport, not here.
func sameOrigin(previous, next *url.URL) bool {
	if previous == nil || next == nil {
		return false
	}

	return strings.EqualFold(previous.Scheme, next.Scheme) && strings.EqualFold(previous.Host, next.Host)
}

// headerNameNormalizer strips the separator characters used in HTTP header
// name conventions ("X-Api-Key", "x_api_key", "x.api.key") so the
// case-insensitive substring scan in [isSensitiveHeaderName] does not
// have to enumerate every separator variant. Hoisted to package scope so
// the Replacer (and its internal trie) is built once at init time — the
// previous per-call construction showed up under pprof on header-redact
// hot paths.
var headerNameNormalizer = strings.NewReplacer("-", "", "_", "", ".", "")

// sensitiveHeaderMarkers is the list of normalized header-name fragments
// whose presence flags a header as carrying credential, session-bearing,
// or tenant-context material. The list is intentionally tight: Midaz
// organization IDs are resource identifiers, not tenant identifiers, so
// organization headers are not included here.
//
// Markers must survive the [headerNameNormalizer] transform (no '-',
// '_', or '.'). When adding new markers, write them in the
// already-normalized form.
var sensitiveHeaderMarkers = []string{
	"authorization",
	"cookie",
	"token",
	"secret",
	"password",
	"apikey",
	"idempotency",
	"tenant",
	// D2: defense-in-depth on credential/PII headers commonly emitted by
	// API gateways and identity-provider integrations.
	"signature",
	"session",
	"accountnumber",
	"customeremail",
	"pii",
	"wwwauthenticate",
	"proxyauthenticate",
}

func isSensitiveHeaderName(name string) bool {
	normalized := headerNameNormalizer.Replace(strings.ToLower(strings.TrimSpace(name)))
	if normalized == "" {
		return false
	}

	for _, marker := range sensitiveHeaderMarkers {
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
