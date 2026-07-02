package entities

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
)

// retryRoundTripper is the OUTERMOST transport layer on the two generated plane
// clients (Ledger + Tracer). The composed chain is:
//
//	http.Client.Transport = retryRoundTripper → authRefreshRoundTripper → base
//
// It re-runs a request through the shared pkg/retry engine on retryable HTTP
// statuses (and transport errors the engine classifies as retryable), while
// upholding the money-path no-double-charge invariant:
//
//   - Unsafe methods (POST/PUT/PATCH/DELETE) retry ONLY when an X-Idempotency
//     key is present, mirroring the legacy *HTTPClient gate
//     (entities/http.go executeRequestWithRetry). A keyless write is tried
//     exactly once — a retried keyless write could be a second balance
//     mutation.
//   - Every attempt carries the IDENTICAL request headers (idempotency key
//     included) and an identical, freshly-rewound body, so a retry is a
//     byte-for-byte replay of the same operation.
//   - The inner authRefreshRoundTripper owns 401 refresh-and-replay; 401 is
//     deliberately NOT in the retry RT's retryable set.
type retryRoundTripper struct {
	base http.RoundTripper
	opts retry.Options
	// customPolicy is additive: it can request a retry on any status,
	// including statuses outside opts.RetryableHTTPCodes. Nil is safe.
	customPolicy func(*http.Response, error) bool
}

// newRetryRoundTripper wraps base with the retry layer. base is the inner auth
// round tripper (or, in tests, the raw transport); nil falls back to
// http.DefaultTransport.
func newRetryRoundTripper(base http.RoundTripper, opts retry.Options, customPolicy func(*http.Response, error) bool) *retryRoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return &retryRoundTripper{base: base, opts: opts, customPolicy: customPolicy}
}

// RoundTrip executes req through the retry engine. On a retryable outcome it
// replays the request; on success or a non-retryable outcome it returns the
// response as-is; on exhaustion of a retryable status it returns the final
// (buffered, still-decodable) response with a nil error so the facade parses
// the persistent problem-JSON exactly as it would with no retry.
func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	effective := rt.resolveOptions(req)

	// lastResp holds the most recent *decodable* response. On a retryable
	// status it is the buffered copy (body preserved); on success or a
	// non-retryable status it is the untouched response. It stays nil for a
	// transport failure, so the exhaustion branch can tell "persistent 5xx"
	// (return the response) from "network down" (return the error).
	var lastResp *http.Response

	err := retry.DoWithContext(retry.WithOptionsContext(ctx, &effective), func() error {
		//nolint:bodyclose // the kept response is either returned to the caller (who closes it), superseded by the next attempt, or already drained+rebuilt by bufferRetryableResponse.
		resp, attemptErr := rt.attempt(ctx, req, effective.RetryableHTTPCodes)
		lastResp = resp

		return attemptErr
	})
	if err != nil {
		// A retryable status that exhausted the budget: return the buffered
		// final response so the caller decodes it. Only a true transport
		// failure (no response kept) surfaces as a bare error.
		if lastResp != nil {
			return lastResp, nil
		}

		return nil, err
	}

	return lastResp, nil
}

// resolveOptions produces the per-request effective retry policy: it composes
// the internal "custom policy said retry" predicate with any caller-supplied
// predicate, and applies the no-double-charge gate that forces a single
// attempt for suppressed contexts, keyless unsafe methods, and unrewindable
// bodies.
func (rt *retryRoundTripper) resolveOptions(req *http.Request) retry.Options {
	// Shallow copy: only MaxRetries and ErrorPredicate are mutated here; the
	// RetryableErrors/RetryableHTTPCodes slices are read-only and shared.
	effective := rt.opts

	// Compose with the internal predicate exactly as the legacy path does
	// (entities/http.go executeRequestWithRetry). This is what makes a
	// retryableCustomPolicyError retryable regardless of its HTTP status.
	userPredicate := effective.ErrorPredicate
	effective.ErrorPredicate = func(err error) bool {
		if isInternalRetryableError(err) {
			return true
		}

		return userPredicate != nil && userPredicate(err)
	}

	hasIdempotencyKey := strings.TrimSpace(req.Header.Get(idempotencyHeader)) != ""
	unrewindableBody := req.Body != nil && req.GetBody == nil

	if httpRetriesSuppressed(req.Context()) || (isUnsafeMethod(req.Method) && !hasIdempotencyKey) || unrewindableBody {
		effective.MaxRetries = 0
	}

	return effective
}

// attempt runs one wire attempt. It returns:
//
//   - (nil, transportErr)                on a transport failure — no response
//     to keep; the engine classifies the error.
//   - (buffered, retrySignal)            on a retryable status or custom-policy
//     retry — the body is preserved so an exhausted retry still yields a
//     decodable response.
//   - (resp, nil)                        on success or a non-retryable status —
//     the untouched response, whose body the caller closes.
func (rt *retryRoundTripper) attempt(ctx context.Context, req *http.Request, codes []int) (*http.Response, error) {
	attemptReq := req.Clone(ctx)

	// Rewind the body for THIS attempt. Each attempt gets a fresh reader from
	// GetBody so the outer retry never exhausts the payload; the inner auth RT
	// may additionally rewind once for its 401 replay.
	if req.GetBody != nil {
		body, bodyErr := req.GetBody()
		if bodyErr != nil {
			// Non-retryable: a replay we cannot rebuild must not be retried.
			return nil, retry.AsNonRetryable(fmt.Errorf("retry: rewind request body: %w", bodyErr))
		}

		attemptReq.Body = body
	}

	resp, rtErr := rt.base.RoundTrip(attemptReq)
	if rtErr != nil {
		// Transport failure. Hand the raw error to the engine, which classifies
		// it against RetryableErrors / typed Retryable(); we do not invent new
		// retryable transport errors here.
		return nil, rtErr
	}

	// Custom policy first: additive, consulted with a nil error because the RT
	// has not parsed the body into an SDK error yet.
	if rt.customPolicy != nil && rt.customPolicy(resp, nil) {
		return bufferRetryableResponse(resp), retryableCustomPolicyError{err: retryableHTTPError{statusCode: resp.StatusCode}}
	}

	// Status-based retry via the configured RetryableHTTPCodes.
	if statusRetryable(resp.StatusCode, codes) {
		return bufferRetryableResponse(resp), retryableHTTPError{statusCode: resp.StatusCode}
	}

	// Success or non-retryable failure: hand the untouched response back; the
	// caller (generated client) closes its body.
	return resp, nil
}

// bufferRetryableResponse drains resp.Body into memory (returning the
// underlying connection to the pool) and rebuilds resp.Body from the buffered
// bytes. This reconciles two needs on a retryable response: the connection must
// be reusable for the next attempt, AND — if retries exhaust — the final
// response handed back must still carry a decodable body. Bounded at
// maxHTTPResponseBodyBytes so a hostile server cannot pin arbitrary memory
// across retries.
func bufferRetryableResponse(resp *http.Response) *http.Response {
	if resp == nil {
		return nil
	}

	if resp.Body == nil {
		return resp
	}

	// A read error here is non-actionable: we are about to either discard this
	// response (on retry) or hand the buffered bytes to a decoder (on
	// exhaustion), where a short/corrupt body surfaces as a decode error. This
	// mirrors the best-effort drain in drainAndCloseResponseBody.
	//nolint:errcheck // best-effort drain; a short read surfaces downstream as a decode error.
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBodyBytes))
	_ = resp.Body.Close()

	resp.Body = io.NopCloser(bytes.NewReader(buf))
	resp.ContentLength = int64(len(buf))

	return resp
}

// statusRetryable reports whether status is in the configured retryable set.
func statusRetryable(status int, codes []int) bool {
	for _, code := range codes {
		if status == code {
			return true
		}
	}

	return false
}
