package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	conc "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	gen "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/generator"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	txpkg "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/transaction"
	"github.com/joho/godotenv"
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
		txPerAccount      = flag.Int("tx", 20, "number of transactions per account (demo batch)")
		concurrency       = flag.Int("concurrency", 0, "worker pool size (0 = auto)")
		batchSize         = flag.Int("batch", 50, "batch size for parallel ops")
		// deprecated: demo flag replaced by interactive toggle
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
		// Ensure observability shutdown never blocks program exit
		sdCtx, sdCancel := context.WithTimeout(context.Background(), 1*time.Second)
		_ = obsProvider.Shutdown(sdCtx)
		sdCancel()
	}()

	// Interactive toggles (stdio) or non-interactive via env DEMO_NON_INTERACTIVE=1
	fmt.Println("\n=== Mass Demo Generator — Booting ===")
	var (
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
		fundAmountVal        string
		amountVal            string
		assetCodeVal         string
		chartGroupVal        string
	)

	if os.Getenv("DEMO_NON_INTERACTIVE") == "1" {
		// Fast path defaults for CI/non-interactive runs
		timeoutSecVal = *timeoutSec
		orgsVal = *orgs
		ledgersPerOrgVal = *ledgersPerOrg
		accountsPerLedgerVal = *accountsPerLedger
		txPerAccountVal = *txPerAccount
		concurrencyVal = *concurrency
		batchSizeVal = *batchSize
		doDemoVal = true
		assetsCountVal = 3
		createHierarchyVal = true
		runBatchVal = true
		fundAmountVal = "100.00"
		amountVal = "1.00"
		assetCodeVal = "USD"
		chartGroupVal = "" // use server default chart group
		fmt.Println("Running in non-interactive mode (DEMO_NON_INTERACTIVE=1)")
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("\n=== Mass Demo Generator — Interactive Setup ===")

		timeoutSecVal = askInt(reader, "Overall timeout (seconds)", *timeoutSec)
		orgsVal = askInt(reader, "Organizations to create", *orgs)
		ledgersPerOrgVal = askInt(reader, "Ledgers per organization", *ledgersPerOrg)
		accountsPerLedgerVal = askInt(reader, "Accounts per ledger", *accountsPerLedger)
		txPerAccountVal = askInt(reader, "Transactions per account (demo batch)", *txPerAccount)
		concurrencyVal = askInt(reader, "Worker pool size (0 = default)", *concurrency)
		batchSizeVal = askInt(reader, "Batch size for parallel ops", *batchSize)
		doDemoVal = askBool(reader, "Run demo (org+ledger+assets+accounts)? [Y/n]", true)
		assetsCountVal = 3
		if doDemoVal {
			assetsCountVal = askInt(reader, "How many assets to create (demo)", 3)
		}
		createHierarchyVal = true
		if doDemoVal {
			createHierarchyVal = askBool(reader, "Create account hierarchy with Customer A/B? [Y/n]", true)
		}
		runBatchVal = false
		if doDemoVal {
			runBatchVal = askBool(reader, "Run Send-based transfer batch demo? [Y/n]", true)
		}
		amountVal = "1.00"
		assetCodeVal = "USD"
		chartGroupVal = "transfer-transactions"
		if runBatchVal {
			// Ask for funding amount (to avoid insufficient funds) and transfer amount
			fundAmountVal = askString(reader, "Funding amount", "99999999")
			amountVal = askString(reader, "Transfer amount", "1.00")
			assetCodeVal = askString(reader, "Asset code", "USD")
			chartGroupVal = askString(reader, "Chart of accounts group (leave blank for server default)", "")
		}
	}

	// Root context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecVal)*time.Second)
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
	gcfg.Organizations = orgsVal
	gcfg.LedgersPerOrg = ledgersPerOrgVal
	gcfg.AccountsPerLedger = accountsPerLedgerVal
	gcfg.TransactionsPerAccount = txPerAccountVal
	if concurrencyVal > 0 {
		gcfg.ConcurrencyLevel = concurrencyVal
	}
	if batchSizeVal > 0 {
		gcfg.BatchSize = batchSizeVal
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

	// Summarize loaded templates
	fmt.Printf("Templates loaded: orgs=%d assets=%d accounts=%d\n", len(orgTemplates), len(assetTemplates), len(accountTemplates))
	fmt.Println("Phase 2 complete: data templates and constraints prepared.")

	// Optional Phase 3 smoke run (minimal): create one org, one ledger, and a few assets
	if doDemoVal {
		fmt.Println("\n🚀 Running Phase 3 minimal generation (org+ledger+assets)...")

		orgGen := gen.NewOrganizationGenerator(c.Entity, obsProvider)
		ledGen := gen.NewLedgerGenerator(c.Entity, obsProvider, "")
		assetGen := gen.NewAssetGenerator(c.Entity, obsProvider)

		// Phase 9 reporting helpers
		phaseTimings := map[string]string{}
		apiCalls := 0
		reportEntities := txpkg.ReportEntities{Counts: txpkg.ReportEntityCounts{}}
		t0 := time.Now()

		// Use first org template
		org, err := orgGen.Generate(ctx, orgTemplates[0])
		if err != nil {
			log.Fatalf("organization generation failed: %v", err)
		}
		apiCalls++
		reportEntities.Counts.Organizations++
		reportEntities.IDs.OrganizationIDs = append(reportEntities.IDs.OrganizationIDs, org.ID)
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
		apiCalls++
		reportEntities.Counts.Ledgers++
		reportEntities.IDs.LedgerIDs = append(reportEntities.IDs.LedgerIDs, ledger.ID)
		fmt.Println("Created ledger:", ledger.ID, ledger.Name)

		// Add a few assets to the ledger
		// Asset API requires orgID and ledgerID; pass orgID via context helper
		actx := gen.WithOrgID(ctx, org.ID)
		for i, at := range assetTemplates {
			if i >= assetsCountVal { // create a few
				break
			}
			a, err := assetGen.Generate(actx, ledger.ID, at)
			if err != nil {
				log.Fatalf("asset generation failed for %s: %v", at.Code, err)
			}
			apiCalls++
			reportEntities.Counts.Assets++
			reportEntities.IDs.AssetIDs = append(reportEntities.IDs.AssetIDs, a.ID)
			fmt.Println("Created asset:", a.ID, a.Code)
		}

		// Record timing for org/ledger/assets
		phaseTimings["org_ledger_assets"] = time.Since(t0).String()

		// Create default account types and a few demo accounts using USD
		atGen := gen.NewAccountTypeGenerator(c.Entity, obsProvider)
		if _, err := atGen.GenerateDefaults(ctx, org.ID, ledger.ID); err != nil {
			log.Fatalf("account type generation failed: %v", err)
		}
		apiCalls++
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
		tAcc := time.Now()
		created, err := accGen.GenerateBatch(ctx, org.ID, ledger.ID, "USD", batch)
		if err != nil {
			log.Fatalf("account generation failed: %v", err)
		}
		apiCalls += len(created)
		reportEntities.Counts.Accounts += len(created)
		for _, a := range created {
			reportEntities.IDs.AccountIDs = append(reportEntities.IDs.AccountIDs, a.ID)
		}
		phaseTimings["accounts_creation"] = time.Since(tAcc).String()
		fmt.Println("Created accounts:", len(created))

		// Create a Portfolio and two Segments, then generate a DSL transaction
		pGen := gen.NewPortfolioGenerator(c.Entity, obsProvider)
		tPS := time.Now()
		portfolio, err := pGen.Generate(ctx, org.ID, ledger.ID, "Customer Portfolio", "demo-entity-1", map[string]any{"category": "customer"})
		if err != nil {
			log.Fatalf("portfolio generation failed: %v", err)
		}
		apiCalls++
		reportEntities.Counts.Portfolios++
		reportEntities.IDs.PortfolioIDs = append(reportEntities.IDs.PortfolioIDs, portfolio.ID)
		fmt.Println("Created portfolio:", portfolio.ID)

		sGen := gen.NewSegmentGenerator(c.Entity, obsProvider)
		segNA, err := sGen.Generate(ctx, org.ID, ledger.ID, "NA", map[string]any{"region": "north_america"})
		if err != nil {
			log.Fatalf("segment generation failed: %v", err)
		}
		apiCalls++
		segEU, err := sGen.Generate(ctx, org.ID, ledger.ID, "EU", map[string]any{"region": "europe"})
		if err != nil {
			log.Fatalf("segment generation failed: %v", err)
		}
		apiCalls++
		reportEntities.Counts.Segments += 2
		reportEntities.IDs.SegmentIDs = append(reportEntities.IDs.SegmentIDs, segNA.ID, segEU.ID)
		phaseTimings["portfolio_segments"] = time.Since(tPS).String()
		fmt.Println("Created segments:", segNA.ID, segEU.ID)

		// Choose two accounts for demo batch
		var accA, accB *models.Account
		if createHierarchyVal {
			// Build a small account hierarchy: Customers Root -> Customer A/B
			hGen := gen.NewAccountHierarchyGenerator(accGen)
			customersRootAlias := "customers_root"
			customerAAlias := "customer_a"
			customerBAlias := "customer_b"
			nodes := []gen.AccountNode{
				{
					Template: data.AccountTemplate{
						Name:        "Customers Root",
						Type:        "deposit",
						Status:      models.NewStatus(models.StatusActive),
						Alias:       &customersRootAlias,
						PortfolioID: &portfolio.ID,
						SegmentID:   &segNA.ID,
						Metadata:    map[string]any{"role": "internal", "group": "customers"},
					},
					Children: []gen.AccountNode{
						{Template: data.AccountTemplate{Name: "Customer A", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerAAlias, PortfolioID: &portfolio.ID, SegmentID: &segNA.ID, Metadata: map[string]any{"role": "customer"}}},
						{Template: data.AccountTemplate{Name: "Customer B", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerBAlias, PortfolioID: &portfolio.ID, SegmentID: &segEU.ID, Metadata: map[string]any{"role": "customer"}}},
					},
				},
			}
			tHier := time.Now()
			createdTree, err := hGen.GenerateTree(ctx, org.ID, ledger.ID, assetCodeVal, nodes)
			if err != nil {
				log.Fatalf("account hierarchy generation failed: %v", err)
			}
			fmt.Println("Created account hierarchy nodes:", len(createdTree))

			apiCalls += len(createdTree)
			reportEntities.Counts.Accounts += len(createdTree)
			for _, a := range createdTree {
				reportEntities.IDs.AccountIDs = append(reportEntities.IDs.AccountIDs, a.ID)
			}
			phaseTimings["hierarchy_creation"] = time.Since(tHier).String()

			for _, a := range createdTree {
				if a.Alias != nil && *a.Alias == "customer_a" {
					accA = a
				}
				if a.Alias != nil && *a.Alias == "customer_b" {
					accB = a
				}
			}
			if accA == nil || accB == nil {
				log.Fatalf("failed to locate demo child accounts by alias")
			}
		} else {
			if len(created) < 2 {
				log.Fatalf("not enough accounts to run demo batch")
			}
			accA, accB = created[0], created[1]
		}

		if runBatchVal {
			// Ensure Customer A has funds: deposit from @external/<asset>
			aliasA := models.GetAccountAlias(*accA)
			extAlias := fmt.Sprintf("@external/%s", assetCodeVal)
			fundTx := &models.CreateTransactionInput{
				Description:              "Funding Customer A",
				Amount:                   fundAmountVal,
				AssetCode:                assetCodeVal,
				ChartOfAccountsGroupName: chartGroupVal, // allow server default when blank
				Send: &models.SendInput{
					Asset: assetCodeVal,
					Value: fundAmountVal,
					Source: &models.SourceInput{From: []models.FromToInput{{
						Account: extAlias,
						Amount:  models.AmountInput{Asset: assetCodeVal, Value: fundAmountVal},
					}}},
					Distribute: &models.DistributeInput{To: []models.FromToInput{{
						Account: aliasA,
						Amount:  models.AmountInput{Asset: assetCodeVal, Value: fundAmountVal},
					}}},
				},
			}
			// Preview funding payload
			if data, err := json.MarshalIndent(fundTx.ToLibTransaction(), "", "  "); err == nil {
				fmt.Println("Preview funding transaction payload:")
				fmt.Println(string(data))
			}
			// Execute funding transaction
			fundStart := time.Now()
			ftx, err := c.Entity.Transactions.CreateTransaction(ctx, org.ID, ledger.ID, fundTx)
			apiCalls++
			if err != nil {
				log.Printf("funding transaction failed: %v (will proceed to batch anyway)", err)
			} else if ftx != nil {
				reportEntities.Counts.Transactions++
				reportEntities.IDs.TransactionIDs = append(reportEntities.IDs.TransactionIDs, ftx.ID)
			}
			phaseTimings["funding"] = time.Since(fundStart).String()

			inputs := make([]*models.CreateTransactionInput, 0, txPerAccountVal)
			// Resolve aliases (API expects accountAlias)
			aliasA = models.GetAccountAlias(*accA)
			aliasB := models.GetAccountAlias(*accB)
			if aliasA == "" || aliasB == "" {
				log.Fatalf("missing account alias for demo accounts (A:%q B:%q)", aliasA, aliasB)
			}

			for i := 0; i < txPerAccountVal; i++ {
				desc := fmt.Sprintf("Demo transfer #%d", i+1)
				inputs = append(inputs, &models.CreateTransactionInput{
					Description:              desc,
					Amount:                   amountVal,
					AssetCode:                assetCodeVal,
					ChartOfAccountsGroupName: chartGroupVal,
					Send: &models.SendInput{
						Asset: assetCodeVal,
						Value: amountVal,
						Source: &models.SourceInput{From: []models.FromToInput{{
							Account: aliasA,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: amountVal},
						}}},
						Distribute: &models.DistributeInput{To: []models.FromToInput{{
							Account: aliasB,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: amountVal},
						}}},
					},
				})
			}

			// Show a preview of the first payload to help diagnose 422s
			if len(inputs) > 0 {
				m := inputs[0].ToLibTransaction()
				if data, err := json.MarshalIndent(m, "", "  "); err == nil {
					fmt.Println("Preview first transaction payload:")
					fmt.Println(string(data))
				}
			}

			// Use batch processor with multi-bar progress and JSON report
			options := txpkg.DefaultBatchOptions()
			if gcfg.ConcurrencyLevel > 0 {
				options.Concurrency = gcfg.ConcurrencyLevel
			} else {
				options.Concurrency = 8
			}
			mpc := txpkg.NewMultiProgressController(len(inputs), "overall")
			options.OnProgress = mpc.OnProgressCallback()

			bctx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			tBatch := time.Now()
			results, err := txpkg.BatchTransactions(bctx, c, org.ID, ledger.ID, inputs, options)
			mpc.Wait()
			if err != nil {
				log.Printf("batch encountered errors: %v", err)
			}
			phaseTimings["batch"] = time.Since(tBatch).String()
			apiCalls += len(results)
			// Print a few sample errors for troubleshooting
			sample := 0
			for _, r := range results {
				if r.Error != nil {
					sample++
					log.Printf("sample error [%d]: %v", sample, r.Error)
					if sample >= 5 {
						break
					}
				}
			}
			// Build extended Phase 9 report
			summary := txpkg.GetBatchSummary(results)
			reportEntities.Counts.Transactions += summary.SuccessCount
			dataSummary := &txpkg.ReportDataSummary{
				TransactionVolumeByAccount: map[string]int{},
				AccountDistributionByType:  map[string]int{},
				AssetUsage:                 map[string]int{},
				BalanceSummaries:           map[string]map[string]any{},
			}
			// Transaction volume per alias (source+dest)
			dataSummary.TransactionVolumeByAccount[aliasA] += txPerAccountVal
			dataSummary.TransactionVolumeByAccount[aliasB] += txPerAccountVal
			// Account distribution by type
			for _, a := range created {
				dataSummary.AccountDistributionByType[strings.ToUpper(a.Type)]++
			}
			// Asset usage
			dataSummary.AssetUsage[assetCodeVal] = txPerAccountVal + 1 // include funding
			// Balance summaries for A and B
			if accA != nil {
				if bal, err := c.Entity.Accounts.GetBalance(ctx, org.ID, ledger.ID, accA.ID); err == nil && bal != nil {
					apiCalls++
                    dataSummary.BalanceSummaries[aliasA] = map[string]any{
                        "asset":     bal.AssetCode,
                        "available": bal.Available,
                        "onHold":    bal.OnHold,
                    }
				}
			}
			if accB != nil {
				if bal, err := c.Entity.Accounts.GetBalance(ctx, org.ID, ledger.ID, accB.ID); err == nil && bal != nil {
					apiCalls++
                    dataSummary.BalanceSummaries[models.GetAccountAlias(*accB)] = map[string]any{
                        "asset":     bal.AssetCode,
                        "available": bal.Available,
                        "onHold":    bal.OnHold,
                    }
				}
			}

			report := txpkg.NewGenerationReport(results, "mass-demo-generator", map[string]any{"org": org.ID, "ledger": ledger.ID})
			report.Entities = &reportEntities
			report.PhaseTimings = phaseTimings
			report.APIStats = &txpkg.ReportAPIStats{APICalls: apiCalls}
			report.DataSummary = dataSummary
			if err := report.SaveJSON("./mass-demo-report.json", true); err != nil {
				log.Printf("failed to save report: %v", err)
			}
			if err := report.SaveHTML("./mass-demo-report.html"); err != nil {
				log.Printf("failed to save HTML report: %v", err)
			}

			fmt.Printf("Batch summary: total=%d success=%d errors=%d successRate=%.1f%% tps=%.2f\n",
				summary.TotalTransactions, summary.SuccessCount, summary.ErrorCount, summary.SuccessRate, summary.TransactionsPerSecond)

			// Export entity IDs for reference
			idsPath := "./mass-demo-entities.json"
			_ = saveEntitiesIDs(idsPath, reportEntities.IDs)
			fmt.Println("Entity IDs saved:", idsPath)
		}

		fmt.Println("✅ Phase 3 minimal generation complete.")
	}
}

func strPtr(s string) *string { return &s }

// --- interactive helpers (stdio) ---
func askString(r *bufio.Reader, prompt, def string) string {
	fmt.Printf("%s [%s]: ", prompt, def)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func askInt(r *bufio.Reader, prompt string, def int) int {
	for {
		fmt.Printf("%s [%d]: ", prompt, def)
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			return def
		}
		n, err := strconv.Atoi(line)
		if err == nil {
			return n
		}
		fmt.Println("Please enter a valid integer.")
	}
}

func askBool(r *bufio.Reader, prompt string, def bool) bool {
	suffix := "Y/n"
	if !def {
		suffix = "y/N"
	}
	for {
		fmt.Printf("%s ", strings.ReplaceAll(prompt, "[Y/n]", suffix))
		line, _ := r.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" {
			return def
		}
		if line == "y" || line == "yes" {
			return true
		}
		if line == "n" || line == "no" {
			return false
		}
		fmt.Println("Please answer y or n.")
	}
}

// saveEntitiesIDs writes entity identifiers to a JSON file for quick reference.
func saveEntitiesIDs(path string, ids txpkg.ReportEntityIDs) error {
    data, err := json.MarshalIndent(ids, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(path, data, 0o644)
}
