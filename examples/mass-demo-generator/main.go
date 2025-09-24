package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    client "github.com/LerianStudio/midaz-sdk-golang/v2"
    data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
    gen "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/generator"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
    "github.com/joho/godotenv"
    "github.com/google/uuid"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
    conc "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
)

func main() {
    // Load .env if present (non-fatal)
    _ = godotenv.Load()

    // Basic flags to tweak Phase 1 config at runtime
    var (
        timeoutSec        = flag.Int("timeout", 120, "overall generation timeout in seconds")
        orgs              = flag.Int("orgs", 2, "number of organizations to create")
        ledgersPerOrg     = flag.Int("ledgers", 2, "number of ledgers per organization")
        accountsPerLedger = flag.Int("accounts", 50, "number of accounts per ledger")
        txPerAccount      = flag.Int("tx", 20, "number of transactions per account")
        concurrency       = flag.Int("concurrency", 0, "worker pool size (0 = auto)")
        batchSize         = flag.Int("batch", 50, "batch size for parallel ops")
        doDemo             = flag.Bool("demo", false, "create a minimal demo org/ledger/assets (requires running Midaz server)")
    )
    flag.Parse()

    // Observability provider setup
    obsProvider, _ := observability.New(context.Background(),
        observability.WithServiceName("mass-demo-generator"),
        observability.WithServiceVersion("0.1.0"),
        observability.WithEnvironment("local"),
        observability.WithComponentEnabled(true, true, true),
    )
    defer func() {
        _ = obsProvider.Shutdown(context.Background())
    }()

    // Root context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
    defer cancel()

    // Configure SDK from environment with safe defaults for local dev
    cfg, err := config.NewConfig(
        config.FromEnvironment(),
        config.WithEnvironment(config.EnvironmentLocal),
        config.WithIdempotency(true),
    )
    if err != nil {
        log.Fatalf("failed to create SDK config: %v", err)
    }

    // Retry options (exponential backoff + jitter), propagate via context
    r := retry.DefaultOptions()
    _ = retry.WithMaxRetries(3)(r)
    _ = retry.WithInitialDelay(100 * time.Millisecond)(r)
    _ = retry.WithMaxDelay(2 * time.Second)(r)
    _ = retry.WithBackoffFactor(2.0)(r)
    _ = retry.WithJitterFactor(0.25)(r)
    ctx = retry.WithOptionsContext(ctx, r)

    // Reflect retry options into SDK config as well
    cfg.MaxRetries = r.MaxRetries
    cfg.RetryWaitMin = r.InitialDelay
    cfg.RetryWaitMax = r.MaxDelay

    // Initialize SDK client with observability provider
    c, err := client.New(
        client.WithConfig(cfg),
        client.WithObservabilityProvider(obsProvider),
        client.UseAllAPIs(),
    )
    if err != nil {
        log.Fatalf("failed to create SDK client: %v", err)
    }
    defer func() {
        _ = c.Shutdown(context.Background())
    }()

    // Build generator configuration (Phase 1)
    gcfg := gen.DefaultConfig()
    gcfg.Organizations = *orgs
    gcfg.LedgersPerOrg = *ledgersPerOrg
    gcfg.AccountsPerLedger = *accountsPerLedger
    gcfg.TransactionsPerAccount = *txPerAccount
    if *concurrency > 0 {
        gcfg.ConcurrencyLevel = *concurrency
    }
    if *batchSize > 0 {
        gcfg.BatchSize = *batchSize
    }

    // Optional circuit breaker
    if gcfg.EnableCircuitBreaker {
        cb := conc.NewCircuitBreakerNamed("entity-api",
            gcfg.CircuitBreakerFailureThreshold,
            gcfg.CircuitBreakerSuccessThreshold,
            gcfg.CircuitBreakerOpenTimeout,
        )
        ctx = gen.WithCircuitBreaker(ctx, cb)
    }

    // Summary
    fmt.Println("Mass Demo Generator - Phase 1 bootstrap")
    fmt.Println("Environment:", cfg.Environment)
    fmt.Println("Onboarding API:", cfg.ServiceURLs[config.ServiceOnboarding])
    fmt.Println("Transaction API:", cfg.ServiceURLs[config.ServiceTransaction])
    fmt.Printf("Config: orgs=%d ledgers/org=%d accounts/ledger=%d tx/account=%d concurrency=%d batch=%d\n",
        gcfg.Organizations,
        gcfg.LedgersPerOrg,
        gcfg.AccountsPerLedger,
        gcfg.TransactionsPerAccount,
        gcfg.ConcurrencyLevel,
        gcfg.BatchSize,
    )

    // Phase 1 ends with a validated setup. Later phases will use gcfg and c.Entity
    // to materialize data across organizations, ledgers, accounts, and transactions.
    if os.Getenv("MIDAZ_AUTH_TOKEN") == "" {
        fmt.Println("Warning: MIDAZ_AUTH_TOKEN is not set. Local dev server allows any token.")
    }

    // Phase 2: Load and validate templates (no API calls yet)
    orgTemplates := data.DefaultOrganizations()
    assetTemplates := data.AllAssetTemplates()
    accountTemplates := data.AllAccountTemplates()

    // Minimal validation to ensure templates meet constraints
    for _, ot := range orgTemplates {
        if err := data.ValidateOrgTemplate(ot); err != nil {
            log.Fatalf("org template invalid: %v", err)
        }
    }
    for _, at := range assetTemplates {
        if err := data.ValidateAssetTemplate(at); err != nil {
            log.Fatalf("asset template invalid (%s): %v", at.Code, err)
        }
    }
    for _, acct := range accountTemplates {
        if err := data.ValidateAccountTemplate(acct); err != nil {
            log.Fatalf("account template invalid (%s): %v", acct.Name, err)
        }
    }

    // Create a few DSL patterns (payment, refund) as blueprints and validate
    pid := uuid.New().String()
    payment := data.PaymentPattern("USD", 100, pid, "ext-pay-001")
    if err := data.ValidateTransactionPattern(payment); err != nil {
        log.Fatalf("payment pattern invalid: %v", err)
    }

    rid := uuid.New().String()
    refund := data.RefundPattern("USD", 100, rid, "ext-ref-001")
    if err := data.ValidateTransactionPattern(refund); err != nil {
        log.Fatalf("refund pattern invalid: %v", err)
    }

    fmt.Printf("Templates loaded: orgs=%d assets=%d accounts=%d txn_patterns=%d\n", len(orgTemplates), len(assetTemplates), len(accountTemplates), 2)
    fmt.Println("Phase 2 complete: data templates and constraints prepared.")

    // Optional Phase 3 smoke run (minimal): create one org, one ledger, and a few assets
    if *doDemo {
        fmt.Println("\n🚀 Running Phase 3 minimal generation (org+ledger+assets)...")

        orgGen := gen.NewOrganizationGenerator(c.Entity, obsProvider)
        ledGen := gen.NewLedgerGenerator(c.Entity, obsProvider, "")
        assetGen := gen.NewAssetGenerator(c.Entity, obsProvider)

        // Use first org template
        org, err := orgGen.Generate(ctx, orgTemplates[0])
        if err != nil {
            log.Fatalf("organization generation failed: %v", err)
        }
        fmt.Println("Created org:", org.ID, org.LegalName)

        // Create one ledger
        ledgerTemplate := data.LedgerTemplate{
            Name:     "Demo Ledger",
            Status:   models.NewStatus(models.StatusActive),
            Metadata: map[string]any{"purpose": "operational"},
        }
        ledger, err := ledGen.Generate(ctx, org.ID, ledgerTemplate)
        if err != nil {
            log.Fatalf("ledger generation failed: %v", err)
        }
        fmt.Println("Created ledger:", ledger.ID, ledger.Name)

        // Add a few assets to the ledger
        // Asset API requires orgID and ledgerID; pass orgID via context helper
        actx := gen.WithOrgID(ctx, org.ID)
        for i, at := range assetTemplates {
            if i >= 3 { // create a few
                break
            }
            a, err := assetGen.Generate(actx, ledger.ID, at)
            if err != nil {
                log.Fatalf("asset generation failed for %s: %v", at.Code, err)
            }
            fmt.Println("Created asset:", a.ID, a.Code)
        }

        // Create default account types and a few demo accounts using USD
        atGen := gen.NewAccountTypeGenerator(c.Entity, obsProvider)
        if _, err := atGen.GenerateDefaults(ctx, org.ID, ledger.ID); err != nil {
            log.Fatalf("account type generation failed: %v", err)
        }
        fmt.Println("Created default account types")

        accGen := gen.NewAccountGenerator(c.Entity, obsProvider)
        // Select a few account templates
        var batch []data.AccountTemplate
        for i, t := range accountTemplates {
            if i >= 5 {
                break
            }
            batch = append(batch, t)
        }
        created, err := accGen.GenerateBatch(ctx, org.ID, ledger.ID, "USD", batch)
        if err != nil {
            log.Fatalf("account generation failed: %v", err)
        }
        fmt.Println("Created accounts:", len(created))

        // Create a Portfolio and two Segments, then generate a DSL transaction
        pGen := gen.NewPortfolioGenerator(c.Entity, obsProvider)
        portfolio, err := pGen.Generate(ctx, org.ID, ledger.ID, "Customer Portfolio", "demo-entity-1", map[string]any{"category": "customer"})
        if err != nil {
            log.Fatalf("portfolio generation failed: %v", err)
        }
        fmt.Println("Created portfolio:", portfolio.ID)

        sGen := gen.NewSegmentGenerator(c.Entity, obsProvider)
        segNA, err := sGen.Generate(ctx, org.ID, ledger.ID, "NA", map[string]any{"region": "north_america"})
        if err != nil {
            log.Fatalf("segment generation failed: %v", err)
        }
        segEU, err := sGen.Generate(ctx, org.ID, ledger.ID, "EU", map[string]any{"region": "europe"})
        if err != nil {
            log.Fatalf("segment generation failed: %v", err)
        }
        fmt.Println("Created segments:", segNA.ID, segEU.ID)

        // Build a small account hierarchy: Customers Root -> Customer A/B
        hGen := gen.NewAccountHierarchyGenerator(accGen)
        customersRootAlias := "customers_root"
        customerAAlias := "customer_a"
        customerBAlias := "customer_b"
        nodes := []gen.AccountNode{
            {
                Template: data.AccountTemplate{
                    Name:   "Customers Root",
                    Type:   "deposit",
                    Status: models.NewStatus(models.StatusActive),
                    Alias:  &customersRootAlias,
                    PortfolioID: &portfolio.ID,
                    SegmentID:   &segNA.ID,
                    Metadata: map[string]any{"role": "internal", "group": "customers"},
                },
                Children: []gen.AccountNode{
                    {Template: data.AccountTemplate{Name: "Customer A", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerAAlias, PortfolioID: &portfolio.ID, SegmentID: &segNA.ID, Metadata: map[string]any{"role": "customer"}}},
                    {Template: data.AccountTemplate{Name: "Customer B", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerBAlias, PortfolioID: &portfolio.ID, SegmentID: &segEU.ID, Metadata: map[string]any{"role": "customer"}}},
                },
            },
        }
        createdTree, err := hGen.GenerateTree(ctx, org.ID, ledger.ID, "USD", nodes)
        if err != nil {
            log.Fatalf("account hierarchy generation failed: %v", err)
        }
        fmt.Println("Created account hierarchy nodes:", len(createdTree))

        // Generate a sample payment via DSL
        tGen := gen.NewTransactionGenerator(c.Entity, obsProvider)
        payPattern := data.PaymentPattern("USD", 100, uuid.New().String(), "ext-demo-001")
        tx, err := tGen.GenerateWithDSL(ctx, org.ID, ledger.ID, payPattern)
        if err != nil {
            log.Fatalf("dsl transaction failed: %v", err)
        }
        fmt.Println("Created transaction:", tx.ID)

        fmt.Println("✅ Phase 3 minimal generation complete.")
    }
}

func strPtr(s string) *string { return &s }
