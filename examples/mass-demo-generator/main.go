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
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	gen "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/generator"
	integrity "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/integrity"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	txpkg "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/transaction"
	"github.com/joho/godotenv"
)

type workflowState struct {
	demoConfig       demoConfig
	genConfig        gen.GeneratorConfig
	stepTimings      map[string]string
	apiCalls         int
	reportEntities   txpkg.ReportEntities
	accountTxnCounts map[string]int
}

type ledgerContext struct {
	org               *models.Organization
	ledger            *models.Ledger
	assetScales       map[string]int
	baseAccounts      []*models.Account
	hierarchyAccounts []*models.Account
}

func newWorkflowState(cfg demoConfig, genCfg gen.GeneratorConfig) *workflowState {
	return &workflowState{
		demoConfig:       cfg,
		genConfig:        genCfg,
		stepTimings:      make(map[string]string),
		reportEntities:   txpkg.ReportEntities{Counts: txpkg.ReportEntityCounts{}},
		accountTxnCounts: make(map[string]int),
	}
}

func main() {
	userConfig, obsProvider := prepareRun()
	defer shutdownObservability(obsProvider)

	ctx, c, gcfg, shutdownFn := setupSDKAndContext(userConfig, obsProvider)
	defer shutdownFn()

	orgTemplates, assetTemplates, accountTemplates := loadTemplates()

	if userConfig.doDemoVal {
		runGenerationWorkflow(ctx, c, obsProvider, gcfg, userConfig, orgTemplates, assetTemplates, accountTemplates)
	}
}

func formatAmountByScale(amount int64, scale int64) string {
	if scale == 0 {
		return strconv.FormatInt(amount, 10)
	}

	negative := amount < 0
	if negative {
		amount = -amount
	}

	pow := pow10(int(scale))
	whole := amount / pow
	frac := amount % pow
	fracStr := fmt.Sprintf("%0*d", int(scale), frac)

	if negative {
		return fmt.Sprintf("-%d.%s", whole, fracStr)
	}

	return fmt.Sprintf("%d.%s", whole, fracStr)
}

func pow10(scale int) int64 {
	if scale < 0 {
		return 1
	}
	if scale > 18 {
		log.Fatalf("scale %d too large for int64 pow10", scale)
	}
	result := int64(1)
	for i := 0; i < scale; i++ {
		result *= 10
	}
	return result
}

func askString(r *bufio.Reader, prompt, def string) string {
	fmt.Printf("%s [%s]: ", prompt, def)

	line, err := r.ReadString('\n')
	if err != nil {
		log.Printf("warning: failed to read input: %v", err)
		return def
	}

	line = strings.TrimSpace(line)

	if line == "" {
		return def
	}

	return line
}

func askInt(r *bufio.Reader, prompt string, def int) int {
	for {
		fmt.Printf("%s [%d]: ", prompt, def)

		line, err := r.ReadString('\n')
		if err != nil {
			log.Printf("warning: failed to read input: %v", err)
			return def
		}

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
	for {
		label := "[y/N]"
		if def {
			label = "[Y/n]"
		}

		fmt.Printf("%s %s: ", prompt, label)

		line, err := r.ReadString('\n')
		if err != nil {
			log.Printf("warning: failed to read input: %v", err)
			return def
		}

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

func prepareRun() (demoConfig, observability.Provider) {
	fileDefaults := getDemoFileDefaults()

	timeoutDefault := coalesceIntPtr(fileDefaults.Timeout, 120)
	orgsDefault := coalesceIntPtr(fileDefaults.Orgs, 2)
	ledgersDefault := coalesceIntPtr(fileDefaults.LedgersPerOrg, 2)
	accountsDefault := coalesceIntPtr(fileDefaults.AccountsPerLedger, 50)
	txDefault := coalesceIntPtr(fileDefaults.TxPerAccount, 20)
	concurrencyDefault := coalesceIntPtr(fileDefaults.Concurrency, 0)
	batchDefault := coalesceIntPtr(fileDefaults.BatchSize, 50)
	localeDefault := coalesceStringPtr(fileDefaults.Locale, "")

	var (
		timeoutSec        = flag.Int("timeout", timeoutDefault, "overall generation timeout in seconds")
		orgs              = flag.Int("orgs", orgsDefault, "number of organizations to create")
		ledgersPerOrg     = flag.Int("ledgers", ledgersDefault, "number of ledgers per organization")
		accountsPerLedger = flag.Int("accounts", accountsDefault, "number of accounts per ledger")
		txPerAccount      = flag.Int("tx", txDefault, "number of transactions per account (demo batch)")
		concurrency       = flag.Int("concurrency", concurrencyDefault, "worker pool size (0 = auto)")
		batchSize         = flag.Int("batch", batchDefault, "batch size for parallel ops")
		orgLocaleFlag     = flag.String("org-locale", localeDefault, "organization locale (us|br)")
	)

	flag.Parse()

	if err := godotenv.Load("examples/mass-demo-generator/.env"); err != nil {
		log.Printf("note: could not load examples/mass-demo-generator/.env: %v", err)
	}

	if err := godotenv.Load(); err != nil {
		log.Printf("note: could not load .env: %v", err)
	}

	obsProvider, err := observability.New(context.Background(),
		observability.WithServiceName("mass-demo-generator"),
		observability.WithServiceVersion("0.1.0"),
		observability.WithEnvironment("local"),
		observability.WithComponentEnabled(true, true, true),
	)
	if err != nil {
		log.Fatalf("failed to create observability provider: %v", err)
	}

	userConfig := gatherUserConfiguration(timeoutSec, orgs, ledgersPerOrg, accountsPerLedger, txPerAccount, concurrency, batchSize, orgLocaleFlag)

	return userConfig, obsProvider
}

func shutdownObservability(obsProvider observability.Provider) {
	sdCtx, sdCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer sdCancel()

	if err := obsProvider.Shutdown(sdCtx); err != nil {
		log.Printf("warning: observability shutdown failed: %v", err)
	}
}

func loadTemplates() ([]data.OrgTemplate, []data.AssetTemplate, []data.AccountTemplate) {
	orgTemplates := data.DefaultOrganizations()
	assetTemplates := data.AllAssetTemplates()
	accountTemplates := data.AllAccountTemplates()

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

	fmt.Printf("Templates loaded: orgs=%d assets=%d accounts=%d\n", len(orgTemplates), len(assetTemplates), len(accountTemplates))
	fmt.Println("Templates validated: data constraints look good.")

	return orgTemplates, assetTemplates, accountTemplates
}

func runGenerationWorkflow(ctx context.Context, c *client.Client, obsProvider observability.Provider, gcfg gen.GeneratorConfig, userConfig demoConfig, orgTemplates []data.OrgTemplate, assetTemplates []data.AssetTemplate, accountTemplates []data.AccountTemplate) {
	fmt.Println("\n🚀 Running generation workflow (org + ledger + assets + accounts + transactions)...")

	state := newWorkflowState(userConfig, gcfg)

	orgGen := gen.NewOrganizationGenerator(c.Entity, obsProvider)
	ledGen := gen.NewLedgerGenerator(c.Entity, obsProvider, "")
	assetGen := gen.NewAssetGenerator(c.Entity, obsProvider)

	ledgerContexts := make([]*ledgerContext, 0, userConfig.orgsVal*userConfig.ledgersPerOrgVal)

	for orgIdx := 0; orgIdx < userConfig.orgsVal; orgIdx++ {
		template := orgTemplates[orgIdx%len(orgTemplates)]
		org, ledgers := createOrganizationResources(ctx, orgGen, ledGen, assetGen, state, template, assetTemplates, orgIdx, userConfig.ledgersPerOrgVal)

		for _, lc := range ledgers {
			accounts, portfolio, segNA, segEU := createAccountResources(ctx, c, obsProvider, state, org, lc.ledger, accountTemplates)
			lc.baseAccounts = accounts

			if state.demoConfig.createHierarchyVal {
				lc.hierarchyAccounts = createAccountHierarchy(ctx, c, obsProvider, state, org, lc.ledger, portfolio, segNA, segEU)
			}

			ledgerContexts = append(ledgerContexts, lc)
		}
	}

	allResults := make([]txpkg.BatchResult, 0)
	allAccounts := make([]*models.Account, 0)
	var reportOrg *models.Organization
	var reportLedger *models.Ledger

	if state.demoConfig.runBatchVal {
		for _, lc := range ledgerContexts {
			results := runAccountTransactions(ctx, c, state, lc.org, lc.ledger, lc.assetScales, lc.baseAccounts)
			allResults = append(allResults, results...)
			allAccounts = append(allAccounts, lc.baseAccounts...)
			if reportOrg == nil {
				reportOrg = lc.org
				reportLedger = lc.ledger
			}
		}

		if len(allResults) > 0 && reportOrg != nil && reportLedger != nil {
			generateFinalReport(ctx, c, state, reportOrg, reportLedger, allResults, allAccounts)
		}
	}

	configuredTarget := state.demoConfig.orgsVal * state.demoConfig.ledgersPerOrgVal * state.demoConfig.accountsPerLedgerVal * state.demoConfig.txPerAccountVal
	generated := 0
	for _, count := range state.accountTxnCounts {
		generated += count
	}

	fmt.Printf("Configured transaction target: %d\n", configuredTarget)
	fmt.Printf("Transactions generated: %d across %d accounts\n", generated, len(state.accountTxnCounts))
	if generated != configuredTarget {
		fmt.Println("⚠️  Generated transaction count did not match the configured target. Check logs above for errors.")
	} else {
		fmt.Println("✅ Generated transaction volume matches configured target.")
	}

	fmt.Println("✅ Generation run complete.")
}

func createOrganizationResources(ctx context.Context, orgGen gen.OrganizationGenerator, ledGen gen.LedgerGenerator, assetGen gen.AssetGenerator, state *workflowState, tpl data.OrgTemplate, assetTemplates []data.AssetTemplate, orgIdx int, ledgersPerOrg int) (*models.Organization, []*ledgerContext) {
	t0 := time.Now()

	orgTemplate := tpl
	orgTemplate.LegalName = fmt.Sprintf("%s %d", tpl.LegalName, orgIdx+1)

	org, err := orgGen.Generate(ctx, orgTemplate)
	if err != nil {
		log.Fatalf("organization generation failed: %v", err)
	}

	state.apiCalls++
	state.reportEntities.Counts.Organizations++
	state.reportEntities.IDs.OrganizationIDs = append(state.reportEntities.IDs.OrganizationIDs, org.ID)
	fmt.Println("Created org:", org.ID, org.LegalName)

	ledgerContexts := make([]*ledgerContext, 0, ledgersPerOrg)

	for ledgerIdx := 0; ledgerIdx < ledgersPerOrg; ledgerIdx++ {
		ledgerTemplate := data.LedgerTemplate{
			Name:     fmt.Sprintf("Demo Ledger %d-%d", orgIdx+1, ledgerIdx+1),
			Status:   models.NewStatus(models.StatusActive),
			Metadata: map[string]any{"purpose": "operational", "demo_index": ledgerIdx + 1},
		}

		ledger, err := ledGen.Generate(ctx, org.ID, ledgerTemplate)
		if err != nil {
			log.Fatalf("ledger generation failed: %v", err)
		}

		state.apiCalls++
		state.reportEntities.Counts.Ledgers++
		state.reportEntities.IDs.LedgerIDs = append(state.reportEntities.IDs.LedgerIDs, ledger.ID)
		fmt.Println("Created ledger:", ledger.ID, ledger.Name)

		assetCtx := gen.WithOrgID(ctx, org.ID)
		assetScales := map[string]int{}
		assetLimit := state.demoConfig.assetsCountVal
		if assetLimit <= 0 {
			assetLimit = 1
		}

		for i := 0; i < assetLimit; i++ {
			tpl := assetTemplates[i%len(assetTemplates)]

			asset, err := assetGen.Generate(assetCtx, ledger.ID, tpl)
			if err != nil {
				log.Fatalf("asset generation failed for %s: %v", tpl.Code, err)
			}

			state.apiCalls++
			state.reportEntities.Counts.Assets++
			state.reportEntities.IDs.AssetIDs = append(state.reportEntities.IDs.AssetIDs, asset.ID)
			assetScales[asset.Code] = tpl.Scale

			fmt.Println("Created asset:", asset.ID, asset.Code)
		}

		ledgerContexts = append(ledgerContexts, &ledgerContext{org: org, ledger: ledger, assetScales: assetScales})
	}

	state.stepTimings[fmt.Sprintf("org_%d_setup", orgIdx+1)] = time.Since(t0).String()

	return org, ledgerContexts
}

func createAccountResources(ctx context.Context, c *client.Client, obsProvider observability.Provider, state *workflowState, org *models.Organization, ledger *models.Ledger, accountTemplates []data.AccountTemplate) ([]*models.Account, *models.Portfolio, *models.Segment, *models.Segment) {
	atGen := gen.NewAccountTypeGenerator(c.Entity, obsProvider)
	if _, err := atGen.GenerateDefaults(ctx, org.ID, ledger.ID); err != nil {
		log.Fatalf("account type generation failed: %v", err)
	}

	state.apiCalls++
	fmt.Println("Created default account types")

	orGen := gen.NewOperationRouteGenerator(c.Entity, obsProvider)
	opRoutes, err := orGen.GenerateDefaults(ctx, org.ID, ledger.ID)
	if err != nil {
		log.Fatalf("operation routes generation failed: %v", err)
	}

	state.apiCalls += len(opRoutes)
	fmt.Printf("Created operation routes: %d\n", len(opRoutes))

	trGen := gen.NewTransactionRouteGenerator(c.Entity, obsProvider)
	troutes, err := trGen.GenerateDefaults(ctx, org.ID, ledger.ID, opRoutes)
	if err != nil {
		log.Fatalf("transaction routes generation failed: %v", err)
	}

	state.apiCalls += len(troutes)
	fmt.Printf("Created transaction routes: %d\n", len(troutes))

	accGen := gen.NewAccountGenerator(c.Entity, obsProvider)
	totalAccounts := state.demoConfig.accountsPerLedgerVal
	if totalAccounts <= 0 {
		totalAccounts = 1
	}

	batch := make([]data.AccountTemplate, 0, totalAccounts)
	prefix := ledger.ID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	for i := 0; i < totalAccounts; i++ {
		base := accountTemplates[i%len(accountTemplates)]
		clone := cloneAccountTemplate(base)
		alias := fmt.Sprintf("%s_acct_%03d", prefix, i+1)
		clone.Alias = &alias
		if clone.Metadata == nil {
			clone.Metadata = map[string]any{}
		}
		clone.Metadata["demo_account_index"] = i + 1
		batch = append(batch, clone)
	}

	tAcc := time.Now()
	created, err := accGen.GenerateBatch(ctx, org.ID, ledger.ID, state.demoConfig.assetCodeVal, batch)
	if err != nil {
		log.Fatalf("account generation failed: %v", err)
	}

	state.apiCalls += len(created)
	state.reportEntities.Counts.Accounts += len(created)
	for _, account := range created {
		state.reportEntities.IDs.AccountIDs = append(state.reportEntities.IDs.AccountIDs, account.ID)
	}

	state.stepTimings[fmt.Sprintf("ledger_%s_accounts", ledger.ID)] = time.Since(tAcc).String()
	fmt.Println("Created accounts:", len(created))

	pGen := gen.NewPortfolioGenerator(c.Entity, obsProvider)
	tPS := time.Now()
	portfolioName := fmt.Sprintf("Customer Portfolio %s", prefix)
	portfolio, err := pGen.Generate(ctx, org.ID, ledger.ID, portfolioName, fmt.Sprintf("demo-entity-%s", prefix), map[string]any{"category": "customer"})
	if err != nil {
		log.Fatalf("portfolio generation failed: %v", err)
	}

	state.apiCalls++
	state.reportEntities.Counts.Portfolios++
	state.reportEntities.IDs.PortfolioIDs = append(state.reportEntities.IDs.PortfolioIDs, portfolio.ID)
	fmt.Println("Created portfolio:", portfolio.ID)

	sGen := gen.NewSegmentGenerator(c.Entity, obsProvider)
	segNA, err := sGen.Generate(ctx, org.ID, ledger.ID, fmt.Sprintf("NA-%s", prefix), map[string]any{"region": "north_america", "ledger": ledger.ID})
	if err != nil {
		log.Fatalf("segment generation failed: %v", err)
	}

	state.apiCalls++
	segEU, err := sGen.Generate(ctx, org.ID, ledger.ID, fmt.Sprintf("EU-%s", prefix), map[string]any{"region": "europe", "ledger": ledger.ID})
	if err != nil {
		log.Fatalf("segment generation failed: %v", err)
	}

	state.apiCalls++
	state.reportEntities.Counts.Segments += 2
	state.reportEntities.IDs.SegmentIDs = append(state.reportEntities.IDs.SegmentIDs, segNA.ID, segEU.ID)
	state.stepTimings[fmt.Sprintf("ledger_%s_portfolio_segments", ledger.ID)] = time.Since(tPS).String()
	fmt.Println("Created segments:", segNA.ID, segEU.ID)

	return created, portfolio, segNA, segEU
}

func cloneAccountTemplate(base data.AccountTemplate) data.AccountTemplate {
	clone := base
	if base.Metadata != nil {
		meta := make(map[string]any, len(base.Metadata))
		for k, v := range base.Metadata {
			meta[k] = v
		}
		clone.Metadata = meta
	}
	clone.Alias = nil
	return clone
}

func createAccountHierarchy(ctx context.Context, c *client.Client, obsProvider observability.Provider, state *workflowState, org *models.Organization, ledger *models.Ledger, portfolio *models.Portfolio, segNA *models.Segment, segEU *models.Segment) []*models.Account {
	accGen := gen.NewAccountGenerator(c.Entity, obsProvider)
	hGen := gen.NewAccountHierarchyGenerator(accGen)

	customersRootAlias := fmt.Sprintf("customers_root_%s", ledger.ID[:8])
	customerAAlias := fmt.Sprintf("customer_a_%s", ledger.ID[:8])
	customerBAlias := fmt.Sprintf("customer_b_%s", ledger.ID[:8])

	nodes := []gen.AccountNode{
		{
			Template: data.AccountTemplate{
				Name:           "Customers Root",
				Type:           "deposit",
				Status:         models.NewStatus(models.StatusActive),
				Alias:          &customersRootAlias,
				AccountTypeKey: data.StrPtr("CHECKING"),
				PortfolioID:    &portfolio.ID,
				SegmentID:      &segNA.ID,
				Metadata:       map[string]any{"role": "internal", "group": "customers"},
			},
			Children: []gen.AccountNode{
				{Template: data.AccountTemplate{Name: "Customer A", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerAAlias, AccountTypeKey: data.StrPtr("CHECKING"), PortfolioID: &portfolio.ID, SegmentID: &segNA.ID, Metadata: map[string]any{"role": "customer"}}},
				{Template: data.AccountTemplate{Name: "Customer B", Type: "deposit", Status: models.NewStatus(models.StatusActive), Alias: &customerBAlias, AccountTypeKey: data.StrPtr("CHECKING"), PortfolioID: &portfolio.ID, SegmentID: &segEU.ID, Metadata: map[string]any{"role": "customer"}}},
			},
		},
	}

	tHier := time.Now()
	createdTree, err := hGen.GenerateTree(ctx, org.ID, ledger.ID, state.demoConfig.assetCodeVal, nodes)
	if err != nil {
		log.Fatalf("account hierarchy generation failed: %v", err)
	}

	fmt.Println("Created account hierarchy nodes:", len(createdTree))

	state.apiCalls += len(createdTree)
	state.reportEntities.Counts.Accounts += len(createdTree)
	for _, account := range createdTree {
		state.reportEntities.IDs.AccountIDs = append(state.reportEntities.IDs.AccountIDs, account.ID)
	}

	state.stepTimings[fmt.Sprintf("ledger_%s_hierarchy", ledger.ID)] = time.Since(tHier).String()

	return createdTree
}

func runAccountTransactions(ctx context.Context, c *client.Client, state *workflowState, org *models.Organization, ledger *models.Ledger, assetScales map[string]int, accounts []*models.Account) []txpkg.BatchResult {
	if len(accounts) == 0 {
		fmt.Println("No accounts available for transaction demo; skipping batch run")
		return nil
	}

	return processAccountTransactions(ctx, c, state, org, ledger, assetScales, accounts)
}

func processAccountTransactions(ctx context.Context, c *client.Client, state *workflowState, org *models.Organization, ledger *models.Ledger, assetScales map[string]int, accounts []*models.Account) []txpkg.BatchResult {
	scale := assetScales[state.demoConfig.assetCodeVal]
	if scale == 0 {
		scale = 2
	}

	amtGen := data.NewAmountGenerator(state.genConfig.GenerationSeed)
	inputs := buildAccountTransactions(state, accounts, scale, amtGen)

	if len(inputs) == 0 {
		fmt.Println("No transaction inputs generated; skipping batch run")
		return nil
	}

	if state.demoConfig.txPerAccountVal > 0 {
		fmt.Printf("Previewing first transaction for ledger %s\n", ledger.ID)
		if payloadData, err := json.MarshalIndent(inputs[0].ToLibTransaction(), "", "  "); err == nil {
			fmt.Println(string(payloadData))
		}
	}

	options := txpkg.DefaultBatchOptions()
	if state.genConfig.ConcurrencyLevel > 0 {
		options.Concurrency = state.genConfig.ConcurrencyLevel
	}
	if state.genConfig.BatchSize > 0 {
		options.BatchSize = state.genConfig.BatchSize
	}

	batchCtx, batchCancel := context.WithTimeout(ctx, 45*time.Second)
	defer batchCancel()

	tBatch := time.Now()
	fmt.Printf("Submitting %d transactions for ledger %s (batch size %d, concurrency %d)\n", len(inputs), ledger.ID, options.BatchSize, options.Concurrency)
	results, err := txpkg.BatchTransactions(batchCtx, c, org.ID, ledger.ID, inputs, options)
	if err != nil {
		log.Printf("batch encountered errors: %v", err)
	}

	state.stepTimings[fmt.Sprintf("ledger_%s_batch", ledger.ID)] = time.Since(tBatch).String()
	state.apiCalls += len(results)

	for _, res := range results {
		if res.TransactionID != "" {
			state.reportEntities.Counts.Transactions++
			state.reportEntities.IDs.TransactionIDs = append(state.reportEntities.IDs.TransactionIDs, res.TransactionID)
		}
	}

	printSampleErrors(results)

	return results
}

func buildAccountTransactions(state *workflowState, accounts []*models.Account, scale int, amtGen *data.AmountGenerator) []*models.CreateTransactionInput {
	perAccount := state.demoConfig.txPerAccountVal
	total := len(accounts) * perAccount
	inputs := make([]*models.CreateTransactionInput, 0, total)
	extAlias := fmt.Sprintf("@external/%s", state.demoConfig.assetCodeVal)

	for _, account := range accounts {
		alias := models.GetAccountAlias(*account)
		if alias == "" {
			alias = account.ID
		}

		for i := 0; i < perAccount; i++ {
			sequence := state.accountTxnCounts[alias] + 1
			minor := amtGen.Normal(25.0, 10.0, scale)
			if minor <= 0 {
				minor = pow10(scale)
			}
			amountStr := formatAmountByScale(minor, int64(scale))

			tx := &models.CreateTransactionInput{
				Description:              fmt.Sprintf("Demo funding for %s #%d", alias, sequence),
				Amount:                   amountStr,
				AssetCode:                state.demoConfig.assetCodeVal,
				ChartOfAccountsGroupName: state.demoConfig.chartGroupVal,
				IdempotencyKey:           fmt.Sprintf("demo-%s-%05d", account.ID, sequence),
				Send: &models.SendInput{
					Asset: state.demoConfig.assetCodeVal,
					Value: amountStr,
					Source: &models.SourceInput{From: []models.FromToInput{{
						Account: extAlias,
						Amount:  models.AmountInput{Asset: state.demoConfig.assetCodeVal, Value: amountStr},
					}}},
					Distribute: &models.DistributeInput{To: []models.FromToInput{{
						Account: alias,
						Amount:  models.AmountInput{Asset: state.demoConfig.assetCodeVal, Value: amountStr},
					}}},
				},
				Metadata: map[string]any{
					"demo_account_id":    account.ID,
					"demo_account_alias": alias,
					"demo_sequence":      sequence,
				},
			}

			inputs = append(inputs, tx)
			state.accountTxnCounts[alias] = sequence
		}
	}

	return inputs
}

func generateFinalReport(ctx context.Context, c *client.Client, state *workflowState, org *models.Organization, ledger *models.Ledger, results []txpkg.BatchResult, accounts []*models.Account) {
	summary := txpkg.GetBatchSummary(results)
	state.reportEntities.Counts.Transactions = summary.SuccessCount

	reportDataSummary := buildReportDataSummary(state, accounts, summary.SuccessCount)
	fetchAccountBalances(ctx, c, state, org.ID, ledger.ID, accounts, reportDataSummary)

	report := createGenerationReport(ctx, c, results, state, org.ID, ledger.ID, reportDataSummary)
	saveReportFiles(report, state.reportEntities.IDs)
	printBatchSummary(summary)
}

func buildReportDataSummary(state *workflowState, accounts []*models.Account, successCount int) *txpkg.ReportDataSummary {
	reportDataSummary := &txpkg.ReportDataSummary{
		TransactionVolumeByAccount: map[string]int{},
		AccountDistributionByType:  map[string]int{},
		AssetUsage:                 map[string]int{},
		BalanceSummaries:           map[string]map[string]any{},
	}

	totalGenerated := 0
	for alias, count := range state.accountTxnCounts {
		reportDataSummary.TransactionVolumeByAccount[alias] = count
		totalGenerated += count
	}

	for _, account := range accounts {
		reportDataSummary.AccountDistributionByType[strings.ToUpper(account.Type)]++
	}

	if totalGenerated == 0 {
		totalGenerated = successCount
	}
	reportDataSummary.AssetUsage[state.demoConfig.assetCodeVal] = totalGenerated

	return reportDataSummary
}

func fetchAccountBalances(ctx context.Context, c *client.Client, state *workflowState, orgID, ledgerID string, accounts []*models.Account, reportDataSummary *txpkg.ReportDataSummary) {
	balancesFetched := 0

	for _, account := range accounts {
		if balancesFetched >= 2 {
			break
		}

		alias := models.GetAccountAlias(*account)
		if alias == "" {
			alias = account.ID
		}

		bal, err := c.Entity.Accounts.GetBalance(ctx, orgID, ledgerID, account.ID)
		if err != nil || bal == nil {
			continue
		}

		state.apiCalls++
		reportDataSummary.BalanceSummaries[alias] = map[string]any{
			"asset":     bal.AssetCode,
			"available": bal.Available,
			"onHold":    bal.OnHold,
		}
		balancesFetched++
	}
}

func createGenerationReport(ctx context.Context, c *client.Client, results []txpkg.BatchResult, state *workflowState, orgID, ledgerID string, reportDataSummary *txpkg.ReportDataSummary) *txpkg.GenerationReport {
	report := txpkg.NewGenerationReport(results, "mass-demo-generator", map[string]any{
		"org":    orgID,
		"ledger": ledgerID,
	})

	chk := integrity.NewChecker(c.Entity)
	if br, err := chk.GenerateLedgerReport(ctx, orgID, ledgerID); err == nil {
		for alias, summary := range br.ToSummaryMap() {
			reportDataSummary.BalanceSummaries[alias] = summary
		}
		fmt.Println("Integrity: generated balance summary (per asset)")
	} else {
		fmt.Printf("Integrity check skipped: %v\n", err)
	}

	report.Entities = &state.reportEntities
	report.StepTimings = state.stepTimings
	report.APIStats = &txpkg.ReportAPIStats{APICalls: state.apiCalls}
	report.DataSummary = reportDataSummary

	return report
}

func saveReportFiles(report *txpkg.GenerationReport, ids txpkg.ReportEntityIDs) {
	if err := report.SaveJSON("./mass-demo-report.json", true); err != nil {
		log.Printf("failed to save report: %v", err)
	}

	if err := report.SaveHTML("./mass-demo-report.html"); err != nil {
		log.Printf("failed to save HTML report: %v", err)
	}

	idsPath := "./mass-demo-entities.json"
	if err := saveEntitiesIDs(idsPath, ids); err != nil {
		log.Printf("warning: failed to save entity IDs: %v", err)
	} else {
		fmt.Println("Entity IDs saved:", idsPath)
	}
}

func printBatchSummary(summary txpkg.BatchSummary) {
	fmt.Printf("Batch summary: total=%d success=%d errors=%d successRate=%.1f%% tps=%.2f\n",
		summary.TotalTransactions, summary.SuccessCount, summary.ErrorCount, summary.SuccessRate, summary.TransactionsPerSecond)
}

func printSampleErrors(results []txpkg.BatchResult) {
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
}

func saveEntitiesIDs(path string, ids txpkg.ReportEntityIDs) error {
	entityData, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, entityData, 0o600)
}
