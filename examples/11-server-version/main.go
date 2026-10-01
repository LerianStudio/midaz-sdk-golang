// Package main shows how a service decides who owns fees against the Midaz
// ledger it talks to: resolve the fee mode once at boot, refresh it on a
// ticker, and fall back to legacy whenever GET /version cannot prove the
// ledger applies fees.
//
// Usage:
//
//	go run ./examples/11-server-version
//
// Stop it with Ctrl-C. See docs/server-version.md for the /version shapes and
// the decision rule.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
)

// refreshEvery is short so the demo shows refreshes; a service uses minutes.
const refreshEvery = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		log.Fatalf("midaz.New: %v", err)
	}
	defer func() {
		if err := c.Shutdown(context.Background()); err != nil {
			log.Printf("client shutdown: %v", err)
		}
	}()

	// Boot: resolve before the first operation. A new operation reads
	// fees.Mode() once and keeps that mode for its retries, commit, cancel
	// and revert, even after a refresh changes it.
	fees := newFeeMode(ctx, c)
	fees.refresh(ctx, refreshEvery)
}

// feeMode caches the fee mode of one Midaz client. Mode is a lock-free read,
// so the request path never calls GET /version.
type feeMode struct {
	client *midaz.Client
	mode   atomic.Pointer[midaz.FeeMode]
}

func newFeeMode(ctx context.Context, c *midaz.Client) *feeMode {
	f := &feeMode{client: c}
	f.resolve(ctx)

	return f
}

// Mode is the fee mode a new operation must use.
func (f *feeMode) Mode() midaz.FeeMode {
	return *f.mode.Load()
}

// resolve reads GET /version once. A failure yields an unavailable version,
// which ResolveFeeMode turns into legacy: never two fee owners, never none.
func (f *feeMode) resolve(ctx context.Context) {
	v, err := f.client.ServerVersion(ctx)
	if err != nil {
		slog.Warn("midaz server version unavailable, using legacy fees", "error", err)
	}

	mode := midaz.ResolveFeeMode(v)
	if old := f.mode.Swap(&mode); old == nil || *old != mode {
		slog.Info("midaz fee mode", "feeMode", mode, "serverVersion", v.Raw, "source", v.Source)
	}
}

// refresh re-resolves every interval until ctx ends.
func (f *feeMode) refresh(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.resolve(ctx)
		}
	}
}
