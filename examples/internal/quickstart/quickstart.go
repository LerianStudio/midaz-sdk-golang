// Package quickstart provides shared bootstrap helpers used by the
// runnable examples under examples/. It exists for one reason: to keep
// each example focused on the ONE concept it teaches, instead of
// re-implementing client construction and env-var plumbing 12 times.
//
// The package is intentionally NOT a public SDK helper. It lives under
// examples/internal/ so Go's import-path rules forbid downstream code
// from depending on it. If you find yourself reaching for these
// helpers in real code, copy the relevant lines into your own bootstrap
// — that's the explicit signal that the SDK contract you actually
// depend on is the few midaz.With* options below, not this convenience
// wrapper.
//
// # Environment variables
//
// All helpers read the standard SDK env vars via [config.FromEnvironment]:
//
//	MIDAZ_ENVIRONMENT     local | development | production (default: local)
//	MIDAZ_BASE_URL        override service URLs at one place
//	PLUGIN_AUTH_ENABLED   true → Access Manager required
//	PLUGIN_AUTH_ADDRESS   Access Manager service URL
//	MIDAZ_CLIENT_ID       OAuth client ID (Access Manager)
//	MIDAZ_CLIENT_SECRET   OAuth client secret (Access Manager)
//	MIDAZ_DEBUG           true → install a stderr slog handler at debug level
//
// See docs/auth.md for the full auth-source matrix and docs/configuration.md
// for every environment variable the SDK recognizes.
package quickstart

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v4"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
)

// Client builds a Midaz SDK client from environment variables. It is
// intended for example/demo programs that want a one-line bootstrap.
// Production code should call midaz.New directly with explicit options
// — see docs/configuration.md.
//
// Client honors PLUGIN_AUTH_ENABLED — if set, the SDK expects the full
// Access Manager triple (PLUGIN_AUTH_ADDRESS, MIDAZ_CLIENT_ID,
// MIDAZ_CLIENT_SECRET). If unset or false, an anonymous client is
// constructed (suitable for a local Midaz stack with auth disabled).
//
// MIDAZ_DEBUG=true installs a stderr slog handler at debug level.
func Client() (*midaz.Client, error) {
	cfg, err := config.NewConfig(config.FromEnvironment())
	if err != nil {
		return nil, fmt.Errorf("config.NewConfig: %w", err)
	}

	opts := []midaz.Option{
		midaz.WithConfig(cfg),
	}

	// Local stacks may not have Access Manager configured at all. When
	// PLUGIN_AUTH_ENABLED is missing or false, FromEnvironment does not
	// install an auth source, so we explicitly opt out via WithAnonymous
	// to satisfy the v3 'must have exactly one auth source' invariant.
	if !accessManagerEnabled() {
		opts = append(opts, midaz.WithAnonymous())
	}

	if os.Getenv("MIDAZ_DEBUG") == "true" {
		opts = append(opts, midaz.WithLogger(debugLogger()))
	}

	c, err := midaz.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("midaz.New: %w", err)
	}

	return c, nil
}

// LocalClient builds a client pinned to EnvironmentLocal with anonymous
// auth. It is the simplest possible bootstrap and is appropriate when
// the example is meant to demonstrate behavior against a developer's
// local Midaz stack regardless of any MIDAZ_* env vars that may be set.
func LocalClient() (*midaz.Client, error) {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		return nil, fmt.Errorf("midaz.New: %w", err)
	}

	return c, nil
}

// Shutdown gracefully shuts down the client and logs any error. Intended
// for use as `defer quickstart.Shutdown(ctx, c)` at the top of main().
func Shutdown(ctx context.Context, c *midaz.Client) {
	if err := c.Shutdown(ctx); err != nil {
		log.Printf("client shutdown: %v", err)
	}
}

func accessManagerEnabled() bool {
	return os.Getenv("PLUGIN_AUTH_ENABLED") == "true"
}

func debugLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}
