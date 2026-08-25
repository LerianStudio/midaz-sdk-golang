// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"net/http"
	"slices"
	"sort"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestNoRawPairHelperPanicsOnANilResponse is the CLASS behind one fix.
//
// # The defect
//
// Every generated client method returns the raw pair (*http.Response, error),
// and a helper that receives it may be handed (nil, nil): the natural spelling
// cannot produce that, because Go makes a caller name both results, but a
// deliberate `resp, _ :=` discards a transport failure and hands the pair on
// with a nil response and a nil error. A helper that guards only `err != nil`
// then dereferences nil — and this SDK bans panics in library code, on a surface
// where every single-object read and every write funnels through these helpers.
//
// readRawResponse was fixed for exactly this. readCount had the identical hole,
// unguarded, for the whole of that round: it does not route through
// readRawResponse (a HEAD count is headers-only, so there is no body to drain)
// and carried its own `defer resp.Body.Close()` behind an `err != nil` check.
// readCount(nil, nil) panicked, on the shared decode for the transactions count
// and the six onboarding counts — the money path.
//
// # Why a table over the class, and not a test for readCount
//
// The site the review named is never the class. This test enumerates the class
// MECHANICALLY, from the source, and probes every member with (nil, nil), so the
// next helper written with this shape is covered when it lands rather than when
// someone remembers it. rawPairHelperDeclarations is the enumeration; the table
// below is the behaviour, and the two are asserted equal — a helper added
// without a row fails here, and a row naming a helper that no longer exists
// fails too.
//
// # What counts as a member, precisely
//
// A function that RECEIVES the raw pair: an *http.Response followed by an error
// in its own parameter list, or a function-typed parameter whose RESULTS carry
// the same two — writeJSON's send and the three transition helpers' call, which
// reach the same nil through one more hop.
//
// "Followed by", not "immediately followed by", and the difference is not
// theoretical: writeJSON's send returns (*http.Response, []byte, error), and the
// enumeration found HTTPClient.recordRequestFailure — (..., resp, elapsed, err)
// — which the review that prompted this table did not name. One member out of
// sixteen was invisible to the list and visible to the parser, which is the
// whole argument for deriving the class instead of writing it down.
//
// Deliberately NOT members: functions that take a response WITHOUT the error
// beside it (statusOf, requestIDOf, decodeOne, guardListBody, closeResponseBody,
// safeHTTPLogAttrs, ...). Those are handed a response their caller already holds,
// downstream of the guard, so they cannot be the entry point a nil pair arrives
// through. Also not members: functions that RETURN a response (RoundTrip,
// bufferRetryableResponse) — they produce the pair rather than receive it — and
// a func-typed parameter with the pair in its PARAMETERS rather than its results
// (SetCustomRetryPolicy's retry predicate), which describes a callback the SDK
// invokes, not a value it is handed.
func TestNoRawPairHelperPanicsOnANilResponse(t *testing.T) {
	probes := nilPairProbes(t)

	names := make([]string, 0, len(probes))

	for _, probe := range probes {
		names = append(names, probe.name)

		t.Run(probe.name, func(t *testing.T) {
			require.NotPanics(t, func() { probe.check(t) },
				"%s receives the raw (*http.Response, error) pair, so it must refuse a nil response "+
					"instead of dereferencing it", probe.name)
		})
	}

	sort.Strings(names)

	require.Equal(t, rawPairHelperDeclarations(t), names,
		"the probe table must cover every helper that receives the raw (*http.Response, error) pair; "+
			"a helper added without a row is one nil dereference away from a panic on the money path")
}

// nilPairProbe is one member of the class: the name the enumeration reports for
// it, and the call that hands it a nil response with no error.
//
// check takes require.TestingT rather than *testing.T because it is an assertion
// closure, not a subtest body — the distinction the thelper linter draws, and
// the shape that has tripped the COLD linter in this package once per fix round.
type nilPairProbe struct {
	name  string
	check func(require.TestingT)
}

// nilTracerCall stands in for a generated tracer method that returned nothing at
// all — the shape the three transition helpers receive their pair through.
//
//nolint:nilnil // returning the nil pair is the entire point of this stub.
func nilTracerCall(context.Context, string, ...gentracer.RequestEditorFn) (*http.Response, error) {
	return nil, nil
}

// nilPairProbes is the behavioural half. Each entry hands its helper (nil, nil)
// and asserts the refusal, so a member that starts dereferencing again fails
// with an error rather than with a panic somewhere downstream.
func nilPairProbes(t *testing.T) []nilPairProbe {
	t.Helper()

	return slices.Concat(
		responseHelperProbes(recordingSpanContext(t)),
		httpClientLogProbes(NewHTTPClient(&http.Client{}, "", nil)),
	)
}

// responseHelperProbes covers the facade layer's response helpers. spanCtx
// carries a RECORDING span, which only the enrichHTTPSpan row needs and which
// has to be built where a *testing.T is in scope.
func responseHelperProbes(spanCtx context.Context) []nilPairProbe {
	ctx := context.Background()

	return []nilPairProbe{
		{"deleteResource", func(t require.TestingT) {
			require.ErrorIs(t, deleteResource("op", nil, nil), errNoResponse)
		}},
		{"readOne", func(t require.TestingT) {
			_, err := readOne[models.Account]("op", nil, nil)
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"readList", func(t require.TestingT) {
			_, err := readList[models.Account]("op", nil, nil)
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"readSlice", func(t require.TestingT) {
			_, err := readSlice[models.Balance]("op", nil, nil)
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"readRawResponse", func(t require.TestingT) {
			//nolint:bodyclose // the probe hands it no response at all; there is nothing to close.
			_, _, err := readRawResponse(nil, nil)
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"readCount", func(t require.TestingT) {
			count, err := readCount(nil, nil)
			require.ErrorIs(t, err, errNoResponse)
			require.Zero(t, count, "a refused count must not read as zero results")
		}},
		{"writeJSON", func(t require.TestingT) {
			_, err := writeJSON[models.Account](ctx, "op", map[string]string{}, nilSend)
			require.Error(t, err)
		}},
		{"ruleTransition", func(t require.TestingT) {
			_, err := ruleTransition(ctx, "op", nilTracerCall, "id")
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"limitTransition", func(t require.TestingT) {
			_, err := limitTransition(ctx, "op", nilTracerCall, "id")
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"reservationTransition", func(t require.TestingT) {
			_, err := reservationTransition[models.ReservationActionResponse](ctx, "op", "id", nilTracerCall, "id")
			require.ErrorIs(t, err, errNoResponse)
		}},
		{"enrichHTTPSpan", func(require.TestingT) {
			enrichHTTPSpan(spanCtx, http.MethodGet, "https://example.test/v1/x", nil, nil)
		}},
	}
}

// nilSend is writeJSON's send parameter, returning the pair with nothing in it.
func nilSend(io.Reader) (*http.Response, []byte, error) {
	return nil, nil, nil
}

// httpClientLogProbes covers the legacy client's terminal log lines, which take
// the same pair. They are in the class by the same rule as the response helpers
// and are probed the same way — a nil response is their ORDINARY input (a
// transport failure produces exactly that), which is why they already survive it
// and why a future edit must keep doing so.
func httpClientLogProbes(client *HTTPClient) []nilPairProbe {
	const (
		method = http.MethodGet
		url    = "https://example.test/v1/x"
	)

	ctx := context.Background()

	return []nilPairProbe{
		{"HTTPClient.logHTTPPhaseFailure", func(require.TestingT) {
			client.logHTTPPhaseFailure(ctx, method, url, nil, nil)
		}},
		{"HTTPClient.logHTTPTerminalFailure", func(require.TestingT) {
			client.logHTTPTerminalFailure(ctx, method, url, nil, nil, 0)
		}},
		{"HTTPClient.logAuthRefresh", func(require.TestingT) {
			client.logAuthRefresh(ctx, "failed", method, url, nil, nil)
		}},
		{"HTTPClient.logRetryExhausted", func(require.TestingT) {
			client.logRetryExhausted(ctx, method, url, nil, nil, 0)
		}},
		{"HTTPClient.recordRequestFailure", func(require.TestingT) {
			client.recordRequestFailure(ctx, method, url, nil, 0, nil)
		}},
	}
}

// recordingSpanContext returns a context carrying a RECORDING span, so the
// enrichHTTPSpan probe exercises the attribute-building body rather than
// returning at its first line — which is what a non-recording span would make it
// do, leaving the probe vacuous.
func recordingSpanContext(t *testing.T) context.Context {
	t.Helper()

	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(tracetest.NewSpanRecorder()))
	ctx, span := provider.Tracer("nil-pair-probe").Start(context.Background(), "probe")

	t.Cleanup(func() { span.End() })

	return ctx
}

// rawPairHelperDeclarations reads this package and returns, sorted, the name of
// every function that receives the raw (*http.Response, error) pair.
//
// Derived from the source rather than listed, for the reason four enumerations
// in this epic were wrong by one: counting is not enumerating.
func rawPairHelperDeclarations(t *testing.T) []string {
	t.Helper()

	names := map[string]bool{}

	for _, file := range parseGoFiles(t, token.NewFileSet(), ".") {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Params == nil || !receivesRawPair(fn.Type.Params) {
				continue
			}

			names[funcLabel(fn)] = true
		}
	}

	require.NotEmpty(t, names, "found no raw-pair helpers; the enumeration is broken, not the code")

	return sortedKeys(names)
}

// receivesRawPair reports whether a signature is handed the raw pair, directly
// or through a function-typed parameter that returns it.
func receivesRawPair(params *ast.FieldList) bool {
	if carriesResponseThenError(flattenTypes(params)) {
		return true
	}

	for _, field := range params.List {
		fn, ok := field.Type.(*ast.FuncType)
		if ok && fn.Results != nil && carriesResponseThenError(flattenTypes(fn.Results)) {
			return true
		}
	}

	return false
}

// carriesResponseThenError reports whether a flattened type list holds an
// *http.Response followed, at any later position, by an error.
//
// "Later", not "adjacent": writeJSON's send returns (*http.Response, []byte,
// error), and the byte slice between them changes nothing about the nil the
// helper can be handed.
func carriesResponseThenError(list []string) bool {
	for i, typ := range list {
		if typ != "*http.Response" {
			continue
		}

		for _, later := range list[i+1:] {
			if later == "error" {
				return true
			}
		}
	}

	return false
}

// flattenTypes renders a field list as one type string per position, so a
// grouped "a, b string" yields two entries and an unnamed result yields one.
func flattenTypes(fields *ast.FieldList) []string {
	var out []string

	for _, field := range fields.List {
		rendered := types.ExprString(field.Type)

		for i := 0; i < max(len(field.Names), 1); i++ {
			out = append(out, rendered)
		}
	}

	return out
}
