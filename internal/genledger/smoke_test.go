package genledger

import "testing"

// TestGeneratedSurface is a compile-time smoke check that the generated ledger
// client and a representative operation type exist and are referenceable. If
// codegen regresses (wrong package, dropped operation, unresolved collision),
// this file fails to build.
func TestGeneratedSurface(_ *testing.T) {
	// Client constructor exists and returns the generated *Client. The explicit
	// type pins the constructor signature: codegen drift (a dropped ...ClientOption
	// variadic or a changed return) would fail this build. staticcheck's QF1011
	// ("type is inferable") is generically right but blind to the deliberate pin.
	//nolint:staticcheck // QF1011: explicit type is an intentional signature-contract assertion
	var _ func(string, ...ClientOption) (*Client, error) = NewClient

	// The Organizations list operation surfaces its params type...
	var _ ListOrganizationsParams
	// ...and its response wrapper carries the collision-avoiding Resp suffix.
	var _ *ListOrganizationsResp

	// The spec serves BOTH server versions off one unversioned base: v1 operation
	// ids are bare, v2 ids carry the V2 suffix. Pin one of each so a regen that
	// collapsed the two surfaces into one would fail this build.
	var _ *ListOrganizationsV2Resp

	// The schema that collided with a client wrapper survived as its own type,
	// distinct from the renamed wrapper.
	var _ ProvisionEncryptionResponse // schema
	var _ ProvisionEncryptionV2Resp   // renamed client wrapper
}
