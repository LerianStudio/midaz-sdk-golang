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
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	gen "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/generator"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	txpkg "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/transaction"
	"github.com/google/uuid"
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
		orgLocaleFlag     = flag.String("org-locale", "", "organization locale (us|br)")
		patternsFlag      = flag.Bool("patterns", true, "run DSL pattern demos (subscription/split)")
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
		assetCodeVal         string
		chartGroupVal        string
		orgLocaleVal         string
		runPatternsVal       bool
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
		assetCodeVal = "USD"
		chartGroupVal = "" // use server default chart group
		orgLocaleVal = "us"
		runPatternsVal = true
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
		assetCodeVal = "USD"
		chartGroupVal = "transfer-transactions"
		if runBatchVal {
			assetCodeVal = askString(reader, "Asset code", "USD")
			chartGroupVal = askString(reader, "Chart of accounts group (leave blank for server default)", "")
		}
		orgLocaleVal = strings.ToLower(askString(reader, "Organization locale (us|br)", "us"))
		runPatternsVal = askBool(reader, "Run DSL pattern demos (subscription/split)? [Y/n]", true)
	}

	// CLI flags override (useful for non-interactive runs)
	if *orgLocaleFlag != "" {
		orgLocaleVal = strings.ToLower(*orgLocaleFlag)
	}
	if os.Getenv("DEMO_NON_INTERACTIVE") == "1" {
		runPatternsVal = *patternsFlag
	}

	// Root context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecVal)*time.Second)
	defer cancel()

	// Apply org locale into context for EIN vs CNPJ generation where applicable
	ctx = gen.WithOrgLocale(ctx, strings.ToLower(orgLocaleVal))

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

		// Add a few assets to the ledger and track their scale
		// Asset API requires orgID and ledgerID; pass orgID via context helper
		actx := gen.WithOrgID(ctx, org.ID)
		assetScales := map[string]int{}
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
			assetScales[a.Code] = at.Scale
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

		// Optional DSL pattern demonstrations
		if runPatternsVal {
			fmt.Println("\n▶ Running DSL pattern demos (subscription, split payment)...")
			txGen := gen.NewTransactionGenerator(c.Entity, obsProvider)
			// Subscription: customer -> merchant_main (relies on aliases created by templates)
			sub := data.SubscriptionPattern(assetCodeVal, 25, uuid.New().String(), "demo-sub-1")
			if tx, err := txGen.GenerateWithDSL(ctx, org.ID, ledger.ID, sub); err != nil {
				log.Printf("subscription demo failed: %v", err)
			} else if tx != nil {
				fmt.Println("Subscription tx:", tx.ID)
				reportEntities.Counts.Transactions++
				reportEntities.IDs.TransactionIDs = append(reportEntities.IDs.TransactionIDs, tx.ID)
			}
			// Split: customer -> merchant_main 90%, platform_fee 10%
			splitMap := map[string]int{"@merchant_main": 90, "@platform_fee": 10}
			split := data.SplitPaymentPattern(assetCodeVal, 30, splitMap, uuid.New().String(), "demo-split-1")
			if tx, err := txGen.GenerateWithDSL(ctx, org.ID, ledger.ID, split); err != nil {
				log.Printf("split-payment demo failed: %v", err)
			} else if tx != nil {
				fmt.Println("Split payment tx:", tx.ID)
				reportEntities.Counts.Transactions++
				reportEntities.IDs.TransactionIDs = append(reportEntities.IDs.TransactionIDs, tx.ID)
			}
		}

		if runBatchVal {
			// Ensure Customer A has funds: deposit from @external/<asset>
			aliasA := models.GetAccountAlias(*accA)
			extAlias := fmt.Sprintf("@external/%s", assetCodeVal)
			// Amount auto-sizing based on asset scale
			scale := assetScales[assetCodeVal]
			if scale == 0 { // default to cents
				scale = 2
			}
			amtGen := data.NewAmountGenerator(gcfg.GenerationSeed)
			fundMinor := amtGen.Normal(100.0, 50.0, scale) // e.g., ~100 units
			fundAmtStr := formatAmountByScale(fundMinor, int64(scale))
			fundTx := &models.CreateTransactionInput{
				Description:              "Funding Customer A",
				Amount:                   fundAmtStr,
				AssetCode:                assetCodeVal,
				ChartOfAccountsGroupName: chartGroupVal, // allow server default when blank
				Send: &models.SendInput{
					Asset: assetCodeVal,
					Value: fundAmtStr,
					Source: &models.SourceInput{From: []models.FromToInput{{
						Account: extAlias,
						Amount:  models.AmountInput{Asset: assetCodeVal, Value: fundAmtStr},
					}}},
					Distribute: &models.DistributeInput{To: []models.FromToInput{{
						Account: aliasA,
						Amount:  models.AmountInput{Asset: assetCodeVal, Value: fundAmtStr},
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
				// Auto-sized amount per transaction (normal distribution around 25 units)
				minor := amtGen.Normal(25.0, 10.0, scale)
				amountStr := formatAmountByScale(minor, int64(scale))
				inputs = append(inputs, &models.CreateTransactionInput{
					Description:              desc,
					Amount:                   amountStr,
					AssetCode:                assetCodeVal,
					ChartOfAccountsGroupName: chartGroupVal,
					Send: &models.SendInput{
						Asset: assetCodeVal,
						Value: amountStr,
						Source: &models.SourceInput{From: []models.FromToInput{{
							Account: aliasA,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: amountStr},
						}}},
						Distribute: &models.DistributeInput{To: []models.FromToInput{{
							Account: aliasB,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: amountStr},
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

			// Compensation: handle insufficient funds by topping up and retrying failed ones
			failedIdx := make([]int, 0)
			successTxIDs := make([]string, 0)
			for i, r := range results {
				if r.Error != nil {
					if sdkerrors.IsInsufficientBalanceError(r.Error) {
						failedIdx = append(failedIdx, i)
					}
				} else if r.TransactionID != "" {
					successTxIDs = append(successTxIDs, r.TransactionID)
				}
			}
			if len(failedIdx) > 0 {
				fmt.Printf("Detected %d insufficient-funds errors. Applying compensation...\n", len(failedIdx))
				// Top up source once more
				topupMinor := amtGen.Normal(200.0, 75.0, scale)
				topupStr := formatAmountByScale(topupMinor, int64(scale))
				topup := &models.CreateTransactionInput{
					Description:              "Compensation top-up",
					Amount:                   topupStr,
					AssetCode:                assetCodeVal,
					ChartOfAccountsGroupName: chartGroupVal,
					Send: &models.SendInput{
						Asset: assetCodeVal,
						Value: topupStr,
						Source: &models.SourceInput{From: []models.FromToInput{{
							Account: extAlias,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: topupStr},
						}}},
						Distribute: &models.DistributeInput{To: []models.FromToInput{{
							Account: aliasA,
							Amount:  models.AmountInput{Asset: assetCodeVal, Value: topupStr},
						}}},
					},
					Metadata: map[string]any{"reason": "insufficient_funds_compensation"},
				}
				if _, err := c.Entity.Transactions.CreateTransaction(ctx, org.ID, ledger.ID, topup); err != nil {
					log.Printf("compensation top-up failed: %v", err)
				}

				// Retry failed inputs
				retryInputs := make([]*models.CreateTransactionInput, 0, len(failedIdx))
				for _, idx := range failedIdx {
					retryInputs = append(retryInputs, inputs[idx])
				}
				retryCtx, cancelRetry := context.WithTimeout(ctx, 30*time.Second)
				defer cancelRetry()
				retryResults, _ := txpkg.BatchTransactions(retryCtx, c, org.ID, ledger.ID, retryInputs, options)
				apiCalls += len(retryResults)
				for _, rr := range retryResults {
					if rr.Error == nil && rr.TransactionID != "" {
						successTxIDs = append(successTxIDs, rr.TransactionID)
					}
				}
			}

			// Reversal audit: revert one successful transaction and link metadata
			if len(successTxIDs) > 0 {
				orig := successTxIDs[len(successTxIDs)-1]
				rev, err := c.Entity.Transactions.RevertTransaction(ctx, org.ID, ledger.ID, orig)
				if err == nil && rev != nil {
					// Tag both transactions for audit linking
					_, _ = c.Entity.Transactions.UpdateTransaction(ctx, org.ID, ledger.ID, rev.ID, &models.UpdateTransactionInput{Metadata: map[string]any{
						"reversal_of":     orig,
						"reversal_reason": "demo_compensation",
					}})
					_, _ = c.Entity.Transactions.UpdateTransaction(ctx, org.ID, ledger.ID, orig, &models.UpdateTransactionInput{Metadata: map[string]any{
						"reversed_by": rev.ID,
					}})
				}
			}
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

// formatAmountByScale converts a minor unit value (e.g., cents) into a decimal string
// according to the provided scale (number of decimal places).
func formatAmountByScale(amount int64, scale int64) string {
	if scale == 0 {
		return strconv.FormatInt(amount, 10)
	}
	// Avoid floating-point issues by constructing string manually
	negative := amount < 0
	if negative {
		amount = -amount
	}
	pow := int64(1)
	for i := int64(0); i < scale; i++ {
		pow *= 10
	}
	whole := amount / pow
	frac := amount % pow
	fracStr := strconv.FormatInt(frac+pow, 10)[1:] // left-pad with zeros
	if negative {
		return fmt.Sprintf("-%d.%s", whole, fracStr)
	}
	return fmt.Sprintf("%d.%s", whole, fracStr)
}

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
