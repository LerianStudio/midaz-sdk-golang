package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5"
	conc "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	gen "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/generator"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"gopkg.in/yaml.v3"
)

// demoConfig holds all configuration values for the demo
type demoConfig struct {
	timeoutSecVal        int
	orgsVal              int
	ledgersPerOrgVal     int
	accountsPerLedgerVal int
	txPerAccountVal      int
	concurrencyVal       int
	batchSizeVal         int
	doDemoVal            bool
	assetsCountVal       int
	createHierarchyVal   bool
	runBatchVal          bool
	assetCodeVal         string
	chartGroupVal        string
	orgLocaleVal         string
}

type demoFileDefaults struct {
	Timeout           *int    `yaml:"timeout"`
	Orgs              *int    `yaml:"orgs"`
	LedgersPerOrg     *int    `yaml:"ledgers_per_org"`
	AccountsPerLedger *int    `yaml:"accounts_per_ledger"`
	TxPerAccount      *int    `yaml:"tx_per_account"`
	Concurrency       *int    `yaml:"concurrency"`
	BatchSize         *int    `yaml:"batch_size"`
	Assets            *int    `yaml:"assets"`
	CreateHierarchy   *bool   `yaml:"create_hierarchy"`
	RunBatch          *bool   `yaml:"run_batch"`
	AssetCode         *string `yaml:"asset_code"`
	ChartGroup        *string `yaml:"chart_group"`
	Locale            *string `yaml:"locale"`
	RunFlow           *bool   `yaml:"run_flow"`
}

type demoDefaultsWrapper struct {
	MassDemo *demoFileDefaults `yaml:"mass_demo"`
}

const demoDefaultsPath = "default.yaml"

const (
	maxDemoTimeoutSeconds = 24 * 60 * 60
	maxDemoOrgs           = 100
	maxDemoLedgersPerOrg  = 50
	maxDemoAccounts       = 10_000
	maxDemoTransactions   = 10_000
	maxDemoAssets         = 100
	maxDemoBatchSize      = 10_000
	maxDemoConcurrency    = 100
)

var (
	demoDefaultsOnce   sync.Once
	cachedDemoDefaults demoFileDefaults
)

// gatherUserConfiguration collects configuration from user input or environment
func gatherUserConfiguration(timeoutSec, orgs, ledgersPerOrg, accountsPerLedger, txPerAccount, concurrency, batchSize *int, orgLocaleFlag *string, explicit map[string]bool) demoConfig {
	fmt.Println("\n=== Mass Demo Generator — Booting ===")

	defaults := defaultDemoConfig(timeoutSec, orgs, ledgersPerOrg, accountsPerLedger, txPerAccount, concurrency, batchSize, resolveLocale(*orgLocaleFlag), explicit)

	if os.Getenv("DEMO_NON_INTERACTIVE") == "1" {
		fmt.Println("Running in non-interactive mode (DEMO_NON_INTERACTIVE=1)")
		return defaults
	}

	cfg := runInteractiveConfiguration(defaults)

	if *orgLocaleFlag != "" {
		cfg.orgLocaleVal = strings.ToLower(*orgLocaleFlag)
	}

	return cfg
}

func getDemoFileDefaults() demoFileDefaults {
	demoDefaultsOnce.Do(func() {
		cachedDemoDefaults = loadDemoFileDefaults(demoDefaultsPath)
	})

	return cachedDemoDefaults
}

func loadDemoFileDefaults(path string) demoFileDefaults {
	for _, candidate := range demoDefaultsCandidates(path) {
		if defaults, ok := tryLoadDemoFileDefaults(candidate); ok {
			return defaults
		}
	}

	return demoFileDefaults{}
}

func demoDefaultsCandidates(path string) []string {
	if path == demoDefaultsPath {
		return []string{demoDefaultsPath, filepath.Join("examples", "mass-demo-generator", demoDefaultsPath)}
	}

	return []string{path}
}

func tryLoadDemoFileDefaults(path string) (demoFileDefaults, bool) {
	// Validate and sanitize the file path to prevent directory traversal attacks
	cleanPath := filepath.Clean(path)

	// Get the absolute path of the provided path
	providedAbsPath, err := filepath.Abs(cleanPath)
	if err != nil {
		log.Printf("warning: could not resolve provided config file path: %v", err)
		return demoFileDefaults{}, false
	}

	// Check if file exists and is a regular file (not a directory or special file)
	fileInfo, err := os.Stat(providedAbsPath)
	if err != nil {
		return demoFileDefaults{}, false
	}

	if !fileInfo.Mode().IsRegular() {
		log.Printf("warning: configuration path is not a regular file: %s", providedAbsPath)
		return demoFileDefaults{}, false
	}

	// #nosec G304 - Configuration file path is validated (exists, is regular file) before reading.
	// This is a CLI tool, not a web service, so the path comes from the operator, not untrusted input.
	data, err := os.ReadFile(providedAbsPath)
	if err != nil {
		return demoFileDefaults{}, false
	}

	var wrapper demoDefaultsWrapper
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		log.Printf("warning: failed to parse %s: %v", providedAbsPath, err)
		return demoFileDefaults{}, false
	}

	if wrapper.MassDemo == nil {
		log.Printf("warning: %s does not contain a mass_demo defaults block", providedAbsPath)
		return demoFileDefaults{}, false
	}

	return *wrapper.MassDemo, true
}

func coalesceIntPtr(ptr *int, fallback int) int {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func coalesceBoolPtr(ptr *bool, fallback bool) bool {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func coalesceStringPtr(ptr *string, fallback string) string {
	if ptr != nil {
		return *ptr
	}
	return fallback
}

func envInt(name string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func envString(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func defaultDemoConfig(timeoutSec, orgs, ledgersPerOrg, accountsPerLedger, txPerAccount, concurrency, batchSize *int, locale string, explicit map[string]bool) demoConfig {
	fileDefaults := getDemoFileDefaults()

	cfg := demoConfig{
		timeoutSecVal:        envIntUnlessFlag("DEMO_TIMEOUT", *timeoutSec, "timeout", explicit),
		orgsVal:              envIntUnlessFlag("DEMO_ORGS", *orgs, "orgs", explicit),
		ledgersPerOrgVal:     envIntUnlessFlag("DEMO_LEDGERS_PER_ORG", *ledgersPerOrg, "ledgers", explicit),
		accountsPerLedgerVal: envIntUnlessFlag("DEMO_ACCOUNTS_PER_LEDGER", *accountsPerLedger, "accounts", explicit),
		txPerAccountVal:      envIntUnlessFlag("DEMO_TX_PER_ACCOUNT", *txPerAccount, "tx", explicit),
		concurrencyVal:       envIntUnlessFlag("DEMO_CONCURRENCY", *concurrency, "concurrency", explicit),
		batchSizeVal:         envIntUnlessFlag("DEMO_BATCH_SIZE", *batchSize, "batch", explicit),
		doDemoVal:            envBool("DEMO_RUN_FLOW", coalesceBoolPtr(fileDefaults.RunFlow, true)),
		assetsCountVal:       envInt("DEMO_ASSETS", coalesceIntPtr(fileDefaults.Assets, 3)),
		createHierarchyVal:   envBool("DEMO_CREATE_HIERARCHY", coalesceBoolPtr(fileDefaults.CreateHierarchy, true)),
		runBatchVal:          envBool("DEMO_RUN_BATCH", coalesceBoolPtr(fileDefaults.RunBatch, true)),
		assetCodeVal:         envString("DEMO_ASSET_CODE", coalesceStringPtr(fileDefaults.AssetCode, "USD")),
		chartGroupVal:        envString("DEMO_CHART_GROUP", coalesceStringPtr(fileDefaults.ChartGroup, "")),
		orgLocaleVal:         locale,
	}

	localeFallback := coalesceStringPtr(fileDefaults.Locale, cfg.orgLocaleVal)
	if explicit["org-locale"] {
		cfg.orgLocaleVal = strings.ToLower(locale)
	} else {
		cfg.orgLocaleVal = strings.ToLower(envString("DEMO_LOCALE", localeFallback))
	}

	return cfg
}

func validateDemoConfig(cfg demoConfig) error {
	checks := []struct {
		name  string
		value int
		min   int
		max   int
	}{
		{name: "timeout", value: cfg.timeoutSecVal, min: 1, max: maxDemoTimeoutSeconds},
		{name: "orgs", value: cfg.orgsVal, min: 0, max: maxDemoOrgs},
		{name: "ledgers", value: cfg.ledgersPerOrgVal, min: 0, max: maxDemoLedgersPerOrg},
		{name: "accounts", value: cfg.accountsPerLedgerVal, min: 0, max: maxDemoAccounts},
		{name: "tx", value: cfg.txPerAccountVal, min: 0, max: maxDemoTransactions},
		{name: "assets", value: cfg.assetsCountVal, min: 0, max: maxDemoAssets},
		{name: "batch", value: cfg.batchSizeVal, min: 1, max: maxDemoBatchSize},
		{name: "concurrency", value: cfg.concurrencyVal, min: 0, max: maxDemoConcurrency},
	}

	for _, check := range checks {
		if check.value < check.min || check.value > check.max {
			return fmt.Errorf("%s must be between %d and %d", check.name, check.min, check.max)
		}
	}

	switch cfg.orgLocaleVal {
	case "us", "br":
		return nil
	default:
		return errors.New("org locale must be us or br")
	}
}

func envIntUnlessFlag(name string, fallback int, flagName string, explicit map[string]bool) int {
	if explicit[flagName] {
		return fallback
	}

	return envInt(name, fallback)
}

func resolveLocale(flagValue string) string {
	if flagValue == "" {
		return "us"
	}

	return strings.ToLower(flagValue)
}

func runInteractiveConfiguration(defaults demoConfig) demoConfig {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== Mass Demo Generator — Interactive Setup ===")
	printDefaultsSummary(defaults)

	if askBool(reader, "Use defaults above?", true) {
		return defaults
	}

	cfg := defaults

	cfg.timeoutSecVal = askInt(reader, "Overall timeout (seconds)", cfg.timeoutSecVal)
	cfg.orgsVal = askInt(reader, "Organizations to create", cfg.orgsVal)
	cfg.ledgersPerOrgVal = askInt(reader, "Ledgers per organization", cfg.ledgersPerOrgVal)
	cfg.accountsPerLedgerVal = askInt(reader, "Accounts per ledger", cfg.accountsPerLedgerVal)
	cfg.txPerAccountVal = askInt(reader, "Transactions per account (demo batch)", cfg.txPerAccountVal)
	cfg.concurrencyVal = askInt(reader, "Worker pool size (0 = default)", cfg.concurrencyVal)
	cfg.batchSizeVal = askInt(reader, "Batch size for parallel ops", cfg.batchSizeVal)
	cfg.doDemoVal = askBool(reader, "Run demo (org+ledger+assets+accounts)?", cfg.doDemoVal)

	if cfg.doDemoVal {
		cfg.assetsCountVal = askInt(reader, "How many assets to create (demo)", cfg.assetsCountVal)
		cfg.createHierarchyVal = askBool(reader, "Create account hierarchy with Customer A/B?", cfg.createHierarchyVal)
		cfg.runBatchVal = askBool(reader, "Run Send-based transfer batch demo?", cfg.runBatchVal)

		if cfg.runBatchVal {
			cfg.assetCodeVal = askString(reader, "Asset code", cfg.assetCodeVal)
			cfg.chartGroupVal = askString(reader, "Chart of accounts group (leave blank for server default)", cfg.chartGroupVal)
		}
	} else {
		cfg.runBatchVal = false
	}

	cfg.orgLocaleVal = strings.ToLower(askString(reader, "Organization locale (us|br)", cfg.orgLocaleVal))

	cfg.chartGroupVal = strings.TrimSpace(cfg.chartGroupVal)

	return cfg
}

func printDefaultsSummary(cfg demoConfig) {
	fmt.Println("--- Default configuration ---")
	fmt.Printf("Timeout: %d seconds\n", cfg.timeoutSecVal)
	fmt.Printf("Organizations: %d\n", cfg.orgsVal)
	fmt.Printf("Ledgers per org: %d\n", cfg.ledgersPerOrgVal)
	fmt.Printf("Accounts per ledger: %d\n", cfg.accountsPerLedgerVal)
	fmt.Printf("Transactions per account: %d\n", cfg.txPerAccountVal)
	fmt.Printf("Worker concurrency: %d\n", cfg.concurrencyVal)
	fmt.Printf("Batch size: %d\n", cfg.batchSizeVal)
	fmt.Printf("Locale: %s\n", strings.ToUpper(cfg.orgLocaleVal))
	fmt.Printf("Run demo flow: %t\n", cfg.doDemoVal)
	if cfg.doDemoVal {
		fmt.Printf("  Assets to create: %d\n", cfg.assetsCountVal)
		fmt.Printf("  Create hierarchy: %t\n", cfg.createHierarchyVal)
		fmt.Printf("  Run batch demo: %t\n", cfg.runBatchVal)
		if cfg.runBatchVal {
			fmt.Printf("    Asset code: %s\n", cfg.assetCodeVal)
			fmt.Printf("    Chart group: %s\n", cfg.chartGroupVal)
		}
	}
	fmt.Println("------------------------------")
}

// setupSDKAndContext configures the SDK client and context
func setupSDKAndContext(userConfig demoConfig, obsProvider observability.Provider) (context.Context, *midaz.Client, gen.GeneratorConfig, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(userConfig.timeoutSecVal)*time.Second)
	ctx = gen.WithOrgLocale(ctx, strings.ToLower(userConfig.orgLocaleVal))

	cfg, err := createSDKConfig()
	if err != nil {
		cancel()
		return nil, nil, gen.GeneratorConfig{}, nil, err
	}
	r := configureRetryOptions()
	ctx = retry.WithOptionsContext(ctx, r)
	applyRetryToConfig(cfg, r)

	c, err := createSDKClient(cfg, obsProvider)
	if err != nil {
		cancel()
		return nil, nil, gen.GeneratorConfig{}, nil, err
	}
	gcfg := buildGeneratorConfig(userConfig)
	ctx = applyCircuitBreaker(ctx, gcfg)

	printBootstrapInfo(cfg, gcfg)

	return ctx, c, gcfg, func() {
		cancel()
		shutdownClient(c)
	}, nil
}

func createSDKConfig() (*config.Config, error) {
	opts := []config.Option{
		config.FromEnvironment(),
		config.WithEnvironment(config.EnvironmentLocal),
		config.WithIdempotency(true),
	}

	// The SDK requires exactly one auth source. When PLUGIN_AUTH_ENABLED is unset or
	// false, require an explicit demo-only anonymous mode instead of silently
	// removing auth from a production-shaped run.
	if os.Getenv("PLUGIN_AUTH_ENABLED") != "true" {
		if os.Getenv("DEMO_AUTH_MODE") != "anonymous-local" {
			return nil, errors.New("auth is disabled; set DEMO_AUTH_MODE=anonymous-local only for an unsecured local Midaz stack, or configure Access Manager with PLUGIN_AUTH_ENABLED=true")
		}

		opts = append(opts, config.WithAnonymous())
	}

	cfg, err := config.NewConfig(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK config: %w", err)
	}

	return cfg, nil
}

func configureRetryOptions() *retry.Options {
	r := retry.DefaultOptions()

	if err := retry.WithMaxRetries(3)(r); err != nil {
		log.Printf("warning: failed to set max retries: %v", err)
	}

	if err := retry.WithInitialDelay(100 * time.Millisecond)(r); err != nil {
		log.Printf("warning: failed to set initial delay: %v", err)
	}

	if err := retry.WithMaxDelay(2 * time.Second)(r); err != nil {
		log.Printf("warning: failed to set max delay: %v", err)
	}

	if err := retry.WithBackoffFactor(2.0)(r); err != nil {
		log.Printf("warning: failed to set backoff factor: %v", err)
	}

	if err := retry.WithJitterFactor(0.25)(r); err != nil {
		log.Printf("warning: failed to set jitter factor: %v", err)
	}

	return r
}

func applyRetryToConfig(cfg *config.Config, r *retry.Options) {
	cfg.MaxRetries = r.MaxRetries
	cfg.RetryWaitMin = r.InitialDelay
	cfg.RetryWaitMax = r.MaxDelay
}

func createSDKClient(cfg *config.Config, obsProvider observability.Provider) (*midaz.Client, error) {
	c, err := midaz.New(
		midaz.WithConfig(cfg),
		midaz.WithObservabilityProvider(obsProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SDK client: %w", err)
	}

	return c, nil
}

func buildGeneratorConfig(userConfig demoConfig) gen.GeneratorConfig {
	gcfg := gen.DefaultConfig()
	gcfg.Organizations = userConfig.orgsVal
	gcfg.LedgersPerOrg = userConfig.ledgersPerOrgVal
	gcfg.AccountsPerLedger = userConfig.accountsPerLedgerVal
	gcfg.TransactionsPerAccount = userConfig.txPerAccountVal

	if userConfig.concurrencyVal > 0 {
		gcfg.ConcurrencyLevel = userConfig.concurrencyVal
	}

	if userConfig.batchSizeVal > 0 {
		gcfg.BatchSize = userConfig.batchSizeVal
	}

	return gcfg
}

func applyCircuitBreaker(ctx context.Context, gcfg gen.GeneratorConfig) context.Context {
	if gcfg.EnableCircuitBreaker {
		cb := conc.NewCircuitBreakerNamed("entity-api",
			gcfg.CircuitBreakerFailureThreshold,
			gcfg.CircuitBreakerSuccessThreshold,
			gcfg.CircuitBreakerOpenTimeout,
		)
		ctx = gen.WithCircuitBreaker(ctx, cb)
	}

	return ctx
}

func printBootstrapInfo(cfg *config.Config, gcfg gen.GeneratorConfig) {
	fmt.Println("Mass Demo Generator - Bootstrap")
	fmt.Println("Environment:", cfg.Environment)
	fmt.Println("Onboarding API:", cfg.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Config: orgs=%d ledgers/org=%d accounts/ledger=%d tx/account=%d concurrency=%d batch=%d\n",
		gcfg.Organizations,
		gcfg.LedgersPerOrg,
		gcfg.AccountsPerLedger,
		gcfg.TransactionsPerAccount,
		gcfg.ConcurrencyLevel,
		gcfg.BatchSize,
	)
}

func shutdownClient(c *midaz.Client) {
	sdCtx, sdCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sdCancel()

	if err := c.Shutdown(sdCtx); err != nil {
		log.Printf("client shutdown: %v", err)
	}
}
