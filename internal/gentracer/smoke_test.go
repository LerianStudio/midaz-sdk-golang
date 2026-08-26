package gentracer

import "testing"

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
