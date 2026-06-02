package midaz_test

import (
	"fmt"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v4"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
)

// ExampleNew demonstrates the simplest valid v4 client construction:
// pinned environment + anonymous auth. Suitable for local Midaz stacks
// where authentication is disabled.
func ExampleNew() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(c != nil)
	// Output: true
}

// ExampleNew_missingAuth demonstrates the v4 'must have exactly one
// auth source' invariant. Calling New without WithAccessManager AND
// without WithAnonymous returns a typed configuration error rather than
// silently constructing a misconfigured client.
func ExampleNew_missingAuth() {
	_, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
	)
	fmt.Println(err != nil)
	// Output: true
}

// ExampleWithIdempotency shows the global on/off switch for
// auto-idempotency. Setting false disables the SDK's auto-generated
// X-Idempotency header for the entire client lifetime — only requests
// with an explicit caller-supplied key (via sdkctx.WithIdempotencyKey)
// will emit the header.
func ExampleWithIdempotency() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
		midaz.WithIdempotency(false),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(c.GetConfig().EnableIdempotency)
	// Output: false
}

// ExampleWithoutRetries shows the per-client retry kill-switch. Use
// when retries are someone else's job (workflow engine, saga
// coordinator) and the SDK should fail fast.
func ExampleWithoutRetries() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
		midaz.WithoutRetries(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(c.GetConfig().MaxRetries)
	// Output: 0
}
