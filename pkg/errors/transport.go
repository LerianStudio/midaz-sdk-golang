// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
)

// ClassifyTransportError converts a transport-layer error from the Go
// standard library into a typed *Error with the right [Category] and
// [Code] so callers see uniform shape regardless of failure origin.
//
// Audit 8.1 (CRITICAL): before this helper landed, network failures
// (DNS, connection refused, TLS handshake) bypassed the typed system
// entirely. IsNetworkError(err) returned false on real network errors
// because the predicate matched *Error{Category: CategoryNetwork} and
// the transport layer was returning bare *net.OpError.
//
// Classification rules, evaluated in order:
//
//  1. context.Canceled         → CategoryCancellation, CodeCancellation
//  2. context.DeadlineExceeded → CategoryTimeout, CodeTimeout
//  3. *net.OpError with timeout flag → CategoryTimeout
//  4. *net.OpError              → CategoryNetwork (DNS, conn-refused, broken pipe)
//  5. *net.DNSError             → CategoryNetwork
//  6. tls.RecordHeaderError /
//     tls.CertificateVerificationError → CategoryNetwork
//  7. syscall.ECONNREFUSED /
//     syscall.ECONNRESET / EPIPE → CategoryNetwork
//  8. anything else             → CategoryInternal
//
// The wrapped err is preserved as Error.Err so callers retain the full
// causal chain via errors.Unwrap.
//
// operation is required and populates Error.Operation. Pass the SDK
// call site (e.g. "accounts.Create"). The transport layer at the
// boundary of entities/http.go is the canonical caller.
//
// Returns nil if err is nil. Returns err unchanged if it's already an
// *Error (idempotent — safe to apply at every boundary).
//
// Audit M14: errors that wrap a stdlib *url.Error have their URL
// userinfo (user[:password]) stripped before classification so a
// transport-layer log line cannot leak credentials baked into the
// request URL.
func ClassifyTransportError(operation string, err error) error {
	if err == nil {
		return nil
	}

	// Idempotent: if the error was already classified upstream, don't
	// re-wrap.
	var alreadyTyped *Error
	if errors.As(err, &alreadyTyped) && alreadyTyped != nil {
		return err
	}

	err = stripURLUserinfo(err)

	switch {
	case errors.Is(err, context.Canceled):
		return NewCancellationError(operation, err)

	case errors.Is(err, context.DeadlineExceeded):
		return NewTimeoutError(operation, "request deadline exceeded", err)

	case isTimeoutError(err):
		return NewTimeoutError(operation, "request timed out", err)

	case isNetworkLevelError(err):
		return NewNetworkError(operation, err)

	default:
		return withSyntheticStatus(NewInternalError(operation, err), ErrorSourceTransport, true)
	}
}

// stripURLUserinfo replaces the URL inside a wrapped *url.Error with
// a userinfo-free copy. The stdlib helpfully masks the password to
// "xxxxx" but leaves the username intact — both halves are sensitive
// for our purposes, so we drop the whole userinfo segment.
//
// Returns the original error unchanged when there's no *url.Error in
// the chain (most cases).
func stripURLUserinfo(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) || urlErr == nil {
		return err
	}

	cleaned := redactURLUserinfo(urlErr.URL)
	if cleaned == urlErr.URL {
		return err
	}

	urlErr.URL = cleaned

	return err
}

// isTimeoutError reports whether err carries a timeout signal — either
// a [net.Error] whose Timeout() returns true, or a wrapped one.
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}

// isNetworkLevelError reports whether err originated at the transport
// layer — DNS lookup failure, connection refused, broken pipe, TLS
// handshake failure. These are all retryable, distinct from generic
// internal errors, and warrant CategoryNetwork classification.
func isNetworkLevelError(err error) bool {
	// DNS resolution failures.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Generic net.OpError covers conn-refused, host-unreachable, and
	// the broad "I tried to do a network thing and the OS said no" set.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// TLS-layer failures: cert verification, record header malformation.
	var tlsRecordErr tls.RecordHeaderError
	if errors.As(err, &tlsRecordErr) {
		return true
	}

	var tlsCertErr *tls.CertificateVerificationError
	if errors.As(err, &tlsCertErr) {
		return true
	}

	// Syscall-level: EPIPE, ECONNREFUSED, ECONNRESET. These reach us
	// through net.OpError in most cases, but standalone errno values
	// can leak out of custom transports. The errors.Is walk also
	// catches them when wrapped in non-net.OpError shells.
	for _, errno := range networkSyscalls {
		if errors.Is(err, errno) {
			return true
		}
	}

	// Final substring fallback for esoteric transport implementations
	// that synthesize errors without using the stdlib types. Bounded
	// to a tight allowlist; the typed checks above are the primary path.
	msg := err.Error()
	for _, fragment := range networkErrorFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
}

var networkSyscalls = []error{
	syscall.ECONNREFUSED,
	syscall.ECONNRESET,
	syscall.EPIPE,
	syscall.EHOSTUNREACH,
	syscall.ENETUNREACH,
	syscall.ENETDOWN,
}

// networkErrorFragments is the substring-fallback allowlist for
// transports that synthesize string-only errors. Order doesn't matter;
// any match means CategoryNetwork.
var networkErrorFragments = []string{
	"no such host",
	"connection refused",
	"connection reset",
	"network is unreachable",
	"host is down",
	"broken pipe",
	"i/o timeout",
}
