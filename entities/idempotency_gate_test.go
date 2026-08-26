// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import "testing"

// idemGateConfig is a Config that exposes the optional GetEnableIdempotency,
// built on the shared typedNilConfig stub.
type idemGateConfig struct {
	typedNilConfig
	enabled bool
}

func (c *idemGateConfig) GetEnableIdempotency() bool { return c.enabled }

// TestConfigEnableIdempotency proves 5.2.3: the gate defaults to true when a
// Config does not expose GetEnableIdempotency (parity with the legacy default),
// and reflects the value when it does.
func TestConfigEnableIdempotency(t *testing.T) {
	if !configEnableIdempotency(&typedNilConfig{}) {
		t.Fatal("want true when GetEnableIdempotency is absent (legacy default)")
	}

	if configEnableIdempotency(&idemGateConfig{enabled: false}) {
		t.Fatal("want false when config reports false")
	}

	if !configEnableIdempotency(&idemGateConfig{enabled: true}) {
		t.Fatal("want true when config reports true")
	}
}

// TestFacadeIdempotencyGateThreadsAndIsolates proves 5.2.3: the gate bool
// threads into each of the 7 wired write-facade constructors and is stored
// per-instance (isolation — not a global), so two clients with different
// WithIdempotency settings do not interfere.
func TestFacadeIdempotencyGateThreadsAndIsolates(t *testing.T) {
	on := newRulesFacade(nil, true)
	off := newRulesFacade(nil, false)

	if !on.enableIdempotency || off.enableIdempotency {
		t.Fatalf("rules gate not isolated per-instance: on=%v off=%v", on.enableIdempotency, off.enableIdempotency)
	}

	if !newLimitsFacade(nil, true).enableIdempotency {
		t.Fatal("limits gate not threaded")
	}

	if !newEncryptionFacade(nil, true).enableIdempotency {
		t.Fatal("encryption gate not threaded")
	}

	if !newInstrumentsFacade(nil, true).enableIdempotency {
		t.Fatal("instruments gate not threaded")
	}

	if !newCompositionFacade(nil, true).enableIdempotency {
		t.Fatal("composition gate not threaded")
	}

	if !newFeePackagesFacade(nil, true).enableIdempotency {
		t.Fatal("fee packages gate not threaded")
	}

	if !newBillingPackagesFacade(nil, true).enableIdempotency {
		t.Fatal("billing packages gate not threaded")
	}
}
