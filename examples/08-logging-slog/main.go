// Example: configuring the Midaz SDK with a *slog.Logger.
//
// Demonstrates the v3 logging contract:
//   - midaz.WithLogger plugs in any *slog.Logger
//   - Retry attempts emit structured DEBUG/WARN log lines
//   - midaz.WithSlowCallThreshold emits a WARN on slow calls
//   - The SDK is silent unless WithLogger is configured
//
// Run with:
//
//	go run ./examples/08-logging-slog
//
// Set MIDAZ_DEMO_VERBOSE=1 to see DEBUG output as well as INFO.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("MIDAZ_DEMO_VERBOSE") == "1" {
		level = slog.LevelDebug
	}

	// Build a JSON slog logger writing to stdout.
	// In production prefer NewJSONHandler over NewTextHandler so log
	// shippers (Loki, ELK, Datadog) can parse fields directly.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	// midaz.New with WithLogger and a 1-second slow-call threshold.
	// The SDK will emit:
	//   - DEBUG lines for retry attempts (cause, delay_ms, attempt)
	//   - WARN lines for the final retry before exhaustion
	//   - WARN lines for any successful call slower than 1s
	c, err := midaz.New(
		midaz.WithLogger(logger),
		// WithAnonymous opts out of auth so the example runs without
		// credentials against a local stack. v3 requires exactly one auth
		// source at construction; the first API call will still fail with
		// 401 if the server enforces auth.
		midaz.WithAnonymous(),
		midaz.WithSlowCallThreshold(1*time.Second),
		midaz.WithEnvironment("local"),
	)
	if err != nil {
		logger.Error("midaz.New failed", slog.Any("err", err))
		os.Exit(1)
	}

	defer func() {
		if err := c.Shutdown(context.Background()); err != nil {
			logger.Error("shutdown failed", slog.Any("err", err))
		}
	}()

	// Verify that c.Logger() returns the same logger we configured.
	c.Logger().Info("midaz client constructed",
		slog.String("sdk.name", "midaz-go-sdk"),
		slog.String("environment", "local"),
	)

	// Drive an arbitrary call so the retry/slow-call paths exercise
	// the logger. The call will fail (no auth), but the SDK still
	// emits the same structured lines as a real workload.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = c.V2.Organizations.List(ctx, models.OrganizationsListOpts{})
	if err != nil {
		// Application-side error logging — same logger, same structure.
		c.Logger().Warn("ListOrganizations failed",
			slog.Any("err", err),
		)
	}

	// Tip: to redirect any *slog.Logger to a specific destination
	// (file, byte buffer, syslog), wrap io.Writer construction around
	// the handler. The retry/slow-call path is unaware of the sink.
}
