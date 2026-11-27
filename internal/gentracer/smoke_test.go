package gentracer

import (
	"context"
	"io"
	"net/http"
	"testing"
)

// TestGeneratedSurface is a compile-time smoke check that the generated tracer
// client and a representative operation type exist and are referenceable. If
// codegen regresses (wrong package, dropped operation, unresolved collision),
// this file fails to build. Mirrors internal/genledger/smoke_test.go.
func TestGeneratedSurface(_ *testing.T) {
	// Client constructor exists and returns the generated *Client. The explicit
	// type pins the constructor signature: codegen drift (a dropped ...ClientOption
	// variadic or a changed return) would fail this build. staticcheck's QF1011
	// ("type is inferable") is generically right but blind to the deliberate pin.
	//nolint:staticcheck // QF1011: explicit type is an intentional signature-contract assertion
	var _ func(string, ...ClientOption) (*Client, error) = NewClient

	// A representative audit-events list operation surfaces its params type...
	var _ ListAuditEventsParams
	// ...and its response wrapper carries the collision-avoiding Resp suffix.
	var _ *ListAuditEventsResp

	// The schema that collided with the client wrapper survived as its own type,
	// distinct from the renamed wrapper.
	var _ ListAuditEventsResponse // schema
}

// TestLegacyReservationSurface pins at compile time the reservation operations
// and response types the server spec no longer declares. They come from the
// local fragment api/tracer.legacy-reservations.openapi.yaml, which
// scripts/generate-clients.sh merges into the Tracer spec before generation;
// if the fragment stops being merged, this file fails to build naming the
// missing symbols. entities/reservations_facade.go calls these methods.
func TestLegacyReservationSurface(_ *testing.T) {
	// The explicit types pin each method signature, as NewClient's does above.
	//nolint:staticcheck // QF1011: explicit type is an intentional signature-contract assertion
	var (
		_ func(*Client, context.Context, string, io.Reader, ...RequestEditorFn) (*http.Response, error) = (*Client).CreateReservationWithBody
		_ func(*Client, context.Context, string, ...RequestEditorFn) (*http.Response, error)            = (*Client).ConfirmReservation
		_ func(*Client, context.Context, string, ...RequestEditorFn) (*http.Response, error)            = (*Client).ReleaseReservation
		_ func(*Client, context.Context, string, ...RequestEditorFn) (*http.Response, error)            = (*Client).ConfirmReservationByTransaction
		_ func(*Client, context.Context, string, ...RequestEditorFn) (*http.Response, error)            = (*Client).ReleaseReservationByTransaction
	)

	var _ ReserveResponse
	var _ ReservationActionResponse
	var _ TransactionActionResponse
}
