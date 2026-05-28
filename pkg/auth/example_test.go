package auth_test

import (
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
)

// ExampleAccessManager shows the canonical credential bag for the Lerian
// Access Manager. Most callers do not construct AccessManager directly;
// they pass it to midaz.WithAccessManager (or config.WithAccessManager),
// which auto-flips Enabled. The struct is shown here so callers can see
// the field shape — in real code, ClientID/ClientSecret come from a
// secret store, never from a literal.
func ExampleAccessManager() {
	am := auth.AccessManager{
		Address:      "https://auth.midaz.io",
		ClientID:     "client-id-from-secret-store",
		ClientSecret: "client-secret-from-secret-store",
	}

	fmt.Println("address:", am.Address)
	fmt.Println("client id:", am.ClientID)
	fmt.Println("has secret:", am.ClientSecret != "")
	fmt.Println("enabled (before WithAccessManager):", am.Enabled)
	// Output:
	// address: https://auth.midaz.io
	// client id: client-id-from-secret-store
	// has secret: true
	// enabled (before WithAccessManager): false
}

// ExampleValidateAccessManagerAddress demonstrates the local-only validator
// the SDK uses before issuing any token request. Plain http:// is accepted
// for loopback (127.0.0.1, ::1, localhost) but rejected for any other host
// — the credentials posted to the token endpoint are effectively long-lived
// passwords and must not cross a plaintext link. Use
// [auth.ValidateAccessManagerAddressWithInsecure] only for the in-cluster
// Kubernetes Service DNS pattern where transport security is provided by
// the service mesh.
func ExampleValidateAccessManagerAddress() {
	// Loopback HTTP is fine.
	if err := auth.ValidateAccessManagerAddress("http://localhost:4000"); err != nil {
		fmt.Println("localhost:", err)
	} else {
		fmt.Println("localhost: ok")
	}

	// Public host over HTTPS is fine.
	if err := auth.ValidateAccessManagerAddress("https://auth.midaz.io"); err != nil {
		fmt.Println("https public:", err)
	} else {
		fmt.Println("https public: ok")
	}

	// Public host over plain HTTP is rejected by default.
	if err := auth.ValidateAccessManagerAddress("http://auth.example.com"); err != nil {
		fmt.Println("http public: rejected")
	} else {
		fmt.Println("http public: ok")
	}
	// Output:
	// localhost: ok
	// https public: ok
	// http public: rejected
}
