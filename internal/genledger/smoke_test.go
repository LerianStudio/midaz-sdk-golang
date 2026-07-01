package genledger

import "testing"

// TestGeneratedSurface is a compile-time smoke check that the generated ledger
// client and a representative operation type exist and are referenceable. If
// codegen regresses (wrong package, dropped operation, unresolved collision),
// this file fails to build.
func TestGeneratedSurface(t *testing.T) {
	// Client constructor exists and returns the generated *Client.
	var _ func(string, ...ClientOption) (*Client, error) = NewClient

	// The Organizations list operation surfaces its params type...
	var _ ListOrganizationsParams
	// ...and its response wrapper carries the collision-avoiding Resp suffix.
	var _ *ListOrganizationsResp

	// The schema that collided with a client wrapper survived as its own type,
	// distinct from the renamed wrapper.
	var _ ProvisionEncryptionResponse // schema
	var _ ProvisionEncryptionResp     // renamed client wrapper
}
