package config_test

import (
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

// ExampleNewConfig shows the typical anonymous local-development setup. The
// SDK refuses to construct a client with no auth source unless WithAnonymous
// is explicit — this closes the v2 silent-localhost footgun where a missing
// AccessManager would only surface as 401s on the first real request.
func ExampleNewConfig() {
	cfg, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentLocal),
		config.WithTimeout(30*time.Second),
		config.WithAnonymous(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("env:", cfg.Environment)
	fmt.Println("timeout:", cfg.Timeout)
	fmt.Println("anonymous:", cfg.Anonymous)
	// Output:
	// env: local
	// timeout: 30s
	// anonymous: true
}

// ExampleNewConfig_withAccessManager wires the Lerian Access Manager
// credential broker. WithAccessManager auto-flips Enabled, so callers only
// supply Address/ClientID/ClientSecret. Real code should pull the secret from
// the environment or a secret manager, never from a literal.
func ExampleNewConfig_withAccessManager() {
	cfg, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentProduction),
		config.WithAccessManager(auth.AccessManager{
			Address:      "https://auth.midaz.io",
			ClientID:     "client-id-from-secret-store",
			ClientSecret: "client-secret-from-secret-store",
		}),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("env:", cfg.Environment)
	fmt.Println("auth enabled:", cfg.AccessManager.Enabled)
	fmt.Println("auth address:", cfg.AccessManager.Address)
	// Output:
	// env: production
	// auth enabled: true
	// auth address: https://auth.midaz.io
}

// ExampleNewConfig_customEndpoint targets a self-hosted Midaz stack. WithBaseURL
// derives both the Ledger and Tracer plane URLs from a single base, which is
// the right knob for in-cluster deployments behind a single ingress.
func ExampleNewConfig_customEndpoint() {
	cfg, err := config.NewConfig(
		config.WithBaseURL("https://midaz.internal.example.com"),
		config.WithIdempotency(true),
		config.WithAnonymous(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("idempotency:", cfg.EnableIdempotency)
	fmt.Println("ledger:", cfg.ServiceURLs[config.ServiceOnboarding])
	// Output:
	// idempotency: true
	// ledger: https://midaz.internal.example.com
}
