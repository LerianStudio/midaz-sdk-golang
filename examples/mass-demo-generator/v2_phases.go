package main

// The V2-only phase. Everything in this file exercises a family that exists
// ONLY on the /v2 ledger surface — the families /v1 had removed — plus the /v2
// transaction contract, which is a different shape from its /v1 sibling rather
// than a renamed one.
//
// It is the generator's live-integration proof. The rest of the generator
// tolerates a failed step and reports it, because its job is to produce demo
// data; the transaction cycle here does NOT, because its job is to prove the
// money path. A balance that does not land exactly where double-entry says it
// should fails the whole run.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	gen "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/generator"
)

const (
	// maxV2Holders bounds the CRM holders written per ledger. The phase is a
	// proof, not a volume run — the volume lives in the /v1 batch.
	maxV2Holders = 5

	// balanceSettleTimeout bounds the wait for a posted transaction to be
	// readable on the balance. The ledger applies operations before it answers,
	// so the first read normally already matches; the wait exists so a slower
	// stack reports a real mismatch instead of a race.
	balanceSettleTimeout = 10 * time.Second

	// balancePollInterval is how often the balance is re-read while waiting.
	balancePollInterval = 250 * time.Millisecond

	// demoMetadataIndexEntity and demoMetadataIndexKey name the index the
	// metadata-index cycle creates and drops. The entity has to be one the
	// server accepts — models.IsValidMetadataIndexEntity lists the set.
	demoMetadataIndexEntity = "account"
	demoMetadataIndexKey    = "demo_account_index"

	// defaultBalanceKey names the balance an account carries for an asset unless
	// something opened a keyed one beside it.
	defaultBalanceKey = "default"
)

// The transaction proof's quantities, in WHOLE units of the configured asset.
//
// They live here, once, because the proof POSTS them and its test RE-DERIVES the
// expected balances from them. When the test carried its own copy, a wrong
// production expectation stayed green: both sides were consistent with each
// other and neither was checked against the code that runs.
const (
	v2ProofFundSourceUnits = 100
	v2ProofFundDestUnits   = 50
	v2ProofTransferUnits   = 30
	v2ProofHeldUnits       = 20
	v2ProofCanceledUnits   = 10
)

// v2ProofAmounts is the proof's whole arithmetic at one asset scale: what each
// leg posts, in minor units, and the balances double-entry requires afterwards.
type v2ProofAmounts struct {
	fundSource int64
	fundDest   int64
	transfer   int64
	held       int64
	canceled   int64

	// expectedSource and expectedDest are the only values the run asserts, and
	// they are computed here ONCE, from the quantities above — never adjusted
	// from what a balance read came back with.
	//
	// The canceled hold contributes nothing to either: it takes value off the
	// source's available balance and the cancel puts it back. So a cancel that
	// quietly kept the value, or moved it on to the destination, fails the
	// assertion instead of passing unnoticed.
	expectedSource int64
	expectedDest   int64
}

// v2ProofAmountsFor scales the proof's quantities to one asset's minor units.
func v2ProofAmountsFor(unit int64) v2ProofAmounts {
	amounts := v2ProofAmounts{
		fundSource: v2ProofFundSourceUnits * unit,
		fundDest:   v2ProofFundDestUnits * unit,
		transfer:   v2ProofTransferUnits * unit,
		held:       v2ProofHeldUnits * unit,
		canceled:   v2ProofCanceledUnits * unit,
	}

	amounts.expectedSource = amounts.fundSource - amounts.transfer - amounts.held
	amounts.expectedDest = amounts.fundDest + amounts.transfer + amounts.held

	return amounts
}

// runV2Phases exercises the V2-only families for one ledger, in order: CRM,
// the transaction proof, fees, billing, then the read-only extras.
//
// Only the transaction proof is fatal. Everything else logs and continues,
// matching what the surrounding generator already does with a failed step: a
// demo stack missing one optional family should still produce demo data.
func runV2Phases(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext) error {
	prefix := shortID(lc.ledger.ID)
	fmt.Printf("\n=== V2-only phase for ledger %s ===\n", prefix)

	t0 := time.Now()
	defer func() {
		state.stepTimings[fmt.Sprintf("ledger_%s_v2_phase", lc.ledger.ID)] = time.Since(t0).String()
	}()

	holders := runV2CRMPhase(ctx, c, state, lc, prefix)

	// The transaction proof runs before fees and billing on purpose: both of
	// those name an account to credit, and the two accounts it opens are the
	// only ones in this phase with a balance the demo controls.
	demo, err := runV2TransactionProof(ctx, c, state, lc, prefix)
	if err != nil {
		return fmt.Errorf("v2 transaction proof failed for ledger %s: %w", lc.ledger.ID, err)
	}

	runV2FeePhase(ctx, c, state, lc, prefix, demo)
	runV2BillingPhase(ctx, c, state, lc, prefix, demo)
	runV2ReadOnlyPhase(ctx, c, lc, holders)

	return nil
}

// v2DemoAccounts are the two accounts the transaction proof opens and settles
// against. They are dedicated rather than borrowed from the /v1 batch because
// the assertion is on an EXACT balance: the batch funds its accounts with
// random amounts, so nothing borrowed from it has a predictable value.
type v2DemoAccounts struct {
	sourceAlias string
	destAlias   string
	sourceID    string
	destID      string
}

// logV2Step reports a non-fatal V2 step outcome. A failure here is information
// about the stack, not a reason to abandon the run.
func logV2Step(step string, err error) bool {
	if err != nil {
		log.Printf("v2 phase: %s skipped: %v", step, err)
		return false
	}

	return true
}

// runV2CRMPhase writes CRM holders and, for each, a holder-owned account with
// its instrument.
//
// Most instruments are written through the COMPOSITION endpoint, which takes the
// instrument fields inline and links the instrument to the account it opens in
// the same call — which is what an instrument needs anyway: an account to belong
// to. One instrument is then written DIRECTLY through V2.Instruments.Create, so
// the write side of that family is proven against a live stack too.
func runV2CRMPhase(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string) []*models.Holder {
	count := state.demoConfig.accountsPerLedgerVal
	if count > maxV2Holders {
		count = maxV2Holders
	}

	if count < 1 {
		count = 1
	}

	holders := make([]*models.Holder, 0, count)

	for i := 0; i < count; i++ {
		holder := createV2Holder(ctx, c, state, lc, prefix, i)
		if holder == nil {
			continue
		}

		holders = append(holders, holder)
		createV2HolderAccount(ctx, c, state, lc, prefix, holder, i)
	}

	fmt.Printf("V2 CRM: holders created %d\n", len(holders))

	// Write side of the instruments family, direct rather than composed. It gets
	// an account the composition endpoint never touched — see uninstrumentedAccountID.
	if len(holders) > 0 {
		createV2Instrument(ctx, c, state, lc, holders[0], uninstrumentedAccountID(lc))
	}

	// Read side of the instruments family: list what both writers linked.
	if len(holders) > 0 && holders[0].ID != nil {
		instruments, err := c.V2.Instruments.List(ctx, lc.org.ID, holders[0].ID.String(), models.InstrumentsListOpts{})
		if logV2Step("instruments list", err) {
			state.apiCalls++
			fmt.Printf("V2 CRM: instruments listed for first holder: %d\n", len(instruments.Items))
		}
	}

	return holders
}

// uninstrumentedAccountID returns an account in this ledger that CRM has never
// seen, or "" when the ledger has none.
//
// The server allows ONE instrument per account: "An accountId from ledger can
// only be associated with a single related account on CRM" (409). Every account
// the CRM phase opens goes through the composition endpoint with banking details
// present, which writes that account's one instrument — so handing one of those
// to a direct create is a guaranteed conflict, and the live run said so.
//
// The ledger's /v1 base accounts are the answer and cost nothing: they were
// opened through the plain accounts API before any phase gating, so none of them
// carries an instrument, and linking one to a holder is exactly what a direct
// instrument create is for.
func uninstrumentedAccountID(lc *ledgerContext) string {
	for _, account := range lc.baseAccounts {
		if account != nil && account.ID != "" {
			return account.ID
		}
	}

	return ""
}

// createV2Instrument writes ONE instrument through V2.Instruments.Create,
// linking a holder to an account that has none yet.
//
// This is the live proof of the create payload. It could not be written before:
// the payload carried a `type` property the endpoint has no slot for — and the
// endpoint rejects unknown properties outright — while carrying neither the
// ledger nor the account it marks required, so no body it produced could be
// accepted by any server. The payload now mirrors the contract, and this call is
// what says so against a real stack rather than against a stub.
//
// Tolerant like its sibling phase-1 calls: a failure here is information about
// the deployment, not a reason to abandon a demo-data run.
func createV2Instrument(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, holder *models.Holder, accountID string) {
	if holder == nil || holder.ID == nil || accountID == "" {
		logV2Step("instrument direct create", errors.New("no account without an instrument in this ledger"))

		return
	}

	branch := "0001"
	number := shortID(accountID)
	accountType := "CHECKING"

	input := models.NewCreateInstrumentInput(lc.ledger.ID, accountID).
		WithBankingDetails(&models.BankingDetails{
			Branch:  &branch,
			Account: &number,
			Type:    &accountType,
		}).
		WithMetadata(map[string]any{
			"source":       "mass-demo-generator",
			"demo_surface": "v2",
			"demo_writer":  "instruments-direct",
		})

	instrument, err := c.V2.Instruments.Create(ctx, lc.org.ID, holder.ID.String(), input)
	if !logV2Step("instrument direct create", err) {
		return
	}

	state.apiCalls++
	state.reportEntities.Counts.Instruments++

	if instrument.ID != nil {
		state.reportEntities.IDs.InstrumentIDs = append(state.reportEntities.IDs.InstrumentIDs, instrument.ID.String())
	}

	fmt.Printf("V2 CRM: instrument created directly on account %s\n", number)
}

// createV2Holder writes one CRM holder. The document is locale-aware: a
// Brazilian CNPJ with valid check digits under "br", a US EIN otherwise. Both
// are COMPANY documents, which is why the holder is a legal person — a natural
// person carrying an EIN would be demo data that lies about its own shape.
func createV2Holder(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string, index int) *models.Holder {
	document := gen.GenerateTaxDocument(state.demoConfig.orgLocaleVal, false)

	input := models.NewCreateHolderInput(
		models.HolderTypeLegalPerson,
		fmt.Sprintf("Demo Holder %s-%02d", prefix, index+1),
		document,
	).
		WithExternalID(fmt.Sprintf("demo-holder-%s-%02d", prefix, index+1)).
		WithMetadata(map[string]any{
			"source":       "mass-demo-generator",
			"demo_ledger":  lc.ledger.ID,
			"demo_index":   index + 1,
			"demo_locale":  state.demoConfig.orgLocaleVal,
			"demo_surface": "v2",
		})

	holder, err := c.V2.Holders.Create(ctx, lc.org.ID, input)
	if !logV2Step(fmt.Sprintf("holder %d create", index+1), err) {
		return nil
	}

	state.apiCalls++
	state.reportEntities.Counts.Holders++

	if holder.ID != nil {
		state.reportEntities.IDs.HolderIDs = append(state.reportEntities.IDs.HolderIDs, holder.ID.String())
	}

	return holder
}

// createV2HolderAccount opens a holder-owned account and its instrument in one
// composition call.
//
// The instrument is written because banking details are present: the server has
// no nested "instrument" object, it writes one if and only if any of banking
// details, regulatory fields or related parties appears on the body.
//
// A populated InstrumentError is NOT a failure of this call. It means the
// account committed and the instrument did not, with no compensating delete —
// the account is real and usable, so the demo records it and reports the
// instrument separately.
func createV2HolderAccount(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string, holder *models.Holder, index int) {
	if holder == nil || holder.ID == nil {
		return
	}

	branch := "0001"
	account := fmt.Sprintf("%s%04d", prefix, index+1)
	accountType := "CHECKING"

	input := models.NewCreateHolderAccountInput(state.demoConfig.assetCodeVal, "deposit").
		WithName(fmt.Sprintf("Holder Account %s-%02d", prefix, index+1)).
		WithAlias(fmt.Sprintf("%s_holder_%02d", prefix, index+1)).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(map[string]any{
			"account_type_key": accountType,
			"source":           "mass-demo-generator",
			"demo_surface":     "v2",
		}).
		WithBankingDetails(&models.BankingDetails{
			Branch:  &branch,
			Account: &account,
			Type:    &accountType,
		})

	resp, err := c.V2.Composition.CreateHolderAccount(ctx, lc.org.ID, lc.ledger.ID, holder.ID.String(), input)
	if !logV2Step(fmt.Sprintf("holder %d account composition", index+1), err) {
		return
	}

	state.apiCalls++

	if resp.Account != nil {
		state.reportEntities.Counts.Accounts++
		state.reportEntities.IDs.AccountIDs = append(state.reportEntities.IDs.AccountIDs, resp.Account.ID)
	}

	switch {
	case resp.Instrument != nil && resp.Instrument.ID != nil:
		state.reportEntities.Counts.Instruments++
		state.reportEntities.IDs.InstrumentIDs = append(state.reportEntities.IDs.InstrumentIDs, resp.Instrument.ID.String())
	case resp.InstrumentError != nil:
		log.Printf("v2 phase: holder %d account committed but its instrument did not (%s: %s)",
			index+1, resp.InstrumentError.Status, resp.InstrumentError.Reason)
	}
}

// runV2TransactionProof is the integration proof, and the only fatal step in
// this file.
//
// It opens two dedicated accounts, funds both from the ledger's external
// account, moves value between them with a settled transfer, moves more with a
// hold that it commits, then opens one more hold and CANCELS it — and asserts
// the exact balance double-entry says each account must carry afterwards. A
// mismatch means value went somewhere the SDK did not ask it to, which is not a
// demo-data problem.
//
// The canceled hold is what makes the release path load-bearing: its value has
// to come back to the source for the final assertion to hold, so a cancel that
// stranded the value on hold, or completed it onto the destination, fails the
// run rather than passing quietly.
func runV2TransactionProof(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string) (v2DemoAccounts, error) {
	asset := state.demoConfig.assetCodeVal

	scale := lc.assetScales[asset]
	if scale == 0 {
		scale = 2
	}

	unit := pow10(scale)
	demo := v2DemoAccounts{
		sourceAlias: fmt.Sprintf("%s_v2_src", prefix),
		destAlias:   fmt.Sprintf("%s_v2_dst", prefix),
	}

	var err error
	if demo.sourceID, err = createV2Account(ctx, c, state, lc, demo.sourceAlias, "V2 Demo Source"); err != nil {
		return demo, err
	}

	if demo.destID, err = createV2Account(ctx, c, state, lc, demo.destAlias, "V2 Demo Destination"); err != nil {
		return demo, err
	}

	external := fmt.Sprintf("@external/%s", asset)

	// Minor units, so the arithmetic below is exact at any asset scale.
	amounts := v2ProofAmountsFor(unit)

	steps := []v2ProofStep{
		{"fund source", external, demo.sourceAlias, amounts.fundSource, v2ProofSettled},
		{"fund destination", external, demo.destAlias, amounts.fundDest, v2ProofSettled},
		{"direct transfer", demo.sourceAlias, demo.destAlias, amounts.transfer, v2ProofSettled},
		{"committed hold", demo.sourceAlias, demo.destAlias, amounts.held, v2ProofHoldCommit},
		{"released hold", demo.sourceAlias, demo.destAlias, amounts.canceled, v2ProofHoldCancel},
	}

	if err := postV2ProofSteps(ctx, c, state, lc, prefix, asset, scale, steps); err != nil {
		return demo, err
	}

	// What double-entry says must be true now. The committed hold left its value
	// on the destination; the canceled one gave its value back, so nothing is on
	// hold and neither expectation counts it.
	wantSource := decimal.NewFromInt(amounts.expectedSource).Shift(int32(-scale))
	wantDest := decimal.NewFromInt(amounts.expectedDest).Shift(int32(-scale))

	if err := awaitBalance(ctx, c, lc, demo.sourceID, demo.sourceAlias, asset, wantSource); err != nil {
		return demo, err
	}

	if err := awaitBalance(ctx, c, lc, demo.destID, demo.destAlias, asset, wantDest); err != nil {
		return demo, err
	}

	fmt.Printf("✅ V2 transaction proof: balances match double-entry exactly (source %s, destination %s %s)\n",
		wantSource.String(), wantDest.String(), asset)

	return demo, nil
}

// v2ProofMode is how one leg of the proof reaches its final state.
type v2ProofMode int

const (
	// v2ProofSettled posts a transaction that settles immediately.
	v2ProofSettled v2ProofMode = iota

	// v2ProofHoldCommit holds the value, then commits — the value lands on the
	// destination.
	v2ProofHoldCommit

	// v2ProofHoldCancel holds the value, then cancels — the value returns to the
	// source and the leg nets to zero.
	v2ProofHoldCancel
)

// v2ProofStep is one leg of the transaction proof: a value moving from one
// alias to another, settled immediately or held and then resolved.
type v2ProofStep struct {
	label  string
	from   string
	to     string
	amount int64
	mode   v2ProofMode
}

// postV2ProofSteps posts each step in order. Order matters and the steps are
// not independent: the transfer spends what the funding put there, so a
// concurrent run would race itself into an insufficient-balance refusal.
func postV2ProofSteps(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix, asset string, scale int, steps []v2ProofStep) error {
	for i, step := range steps {
		amount := formatAmountByScale(step.amount, int64(scale))
		input := &models.CreateTransactionV2Input{
			Asset:          asset,
			Amount:         amount,
			Description:    fmt.Sprintf("V2 demo %s (%s)", step.label, prefix),
			IdempotencyKey: fmt.Sprintf("v2-demo-%s-%d", prefix, i),
			Debits:         []models.TransactionV2Leg{{Alias: step.from, Amount: amount}},
			Credits:        []models.TransactionV2Leg{{Alias: step.to, Amount: amount}},
			Metadata: map[string]any{
				"source":       "mass-demo-generator",
				"demo_surface": "v2",
				"demo_step":    step.label,
			},
		}

		tx, err := postV2Transaction(ctx, c, state, lc, input, step.mode)
		if err != nil {
			return fmt.Errorf("%s: %w", step.label, err)
		}

		fmt.Printf("V2 transactions: %s posted %s %s (%s)\n", step.label, amount, asset, shortID(tx.ID))
	}

	return nil
}

// postV2Transaction posts one /v2 transaction and, for a hold, resolves it.
//
// Resolving is what makes a hold a proof rather than a write: an unresolved hold
// leaves its value on hold, and the balance assertion would be asserting an
// unfinished transaction. Both resolutions are exercised — commit moves the
// value on, cancel gives it back.
func postV2Transaction(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, input *models.CreateTransactionV2Input, mode v2ProofMode) (*models.TransactionV2, error) {
	create := c.V2.Transactions.CreateDirect
	if mode != v2ProofSettled {
		create = c.V2.Transactions.CreateHold
	}

	tx, err := create(ctx, lc.org.ID, lc.ledger.ID, input)
	if err != nil {
		return nil, err
	}

	state.apiCalls++
	state.reportEntities.Counts.V2Transactions++
	state.reportEntities.IDs.V2TransactionIDs = append(state.reportEntities.IDs.V2TransactionIDs, tx.ID)

	switch mode {
	case v2ProofHoldCommit:
		committed, err := c.V2.Transactions.Commit(ctx, lc.org.ID, lc.ledger.ID, tx.ID)
		if err != nil {
			return nil, fmt.Errorf("commit of held transaction %s: %w", tx.ID, err)
		}

		state.apiCalls++

		return committed, nil
	case v2ProofHoldCancel:
		canceled, err := c.V2.Transactions.Cancel(ctx, lc.org.ID, lc.ledger.ID, tx.ID)
		if err != nil {
			return nil, fmt.Errorf("cancel of held transaction %s: %w", tx.ID, err)
		}

		state.apiCalls++

		return canceled, nil
	case v2ProofSettled:
		return tx, nil
	}

	return tx, nil
}

// createV2Account opens one account through the /v2 account surface.
func createV2Account(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, alias, name string) (string, error) {
	input := models.NewCreateAccountInput(name, state.demoConfig.assetCodeVal, "deposit").
		WithAlias(alias).
		WithStatus(models.NewStatus(models.StatusActive)).
		WithMetadata(map[string]any{
			// The ledger binds an account to its account type through this
			// metadata key — the same wiring the /v1 account generator uses.
			"account_type_key": "CHECKING",
			"source":           "mass-demo-generator",
			"demo_surface":     "v2",
		})

	account, err := c.V2.Accounts.Create(ctx, lc.org.ID, lc.ledger.ID, input)
	if err != nil {
		return "", fmt.Errorf("failed to create v2 demo account %s: %w", alias, err)
	}

	state.apiCalls++
	state.reportEntities.Counts.Accounts++
	state.reportEntities.IDs.AccountIDs = append(state.reportEntities.IDs.AccountIDs, account.ID)

	return account.ID, nil
}

// awaitBalance reads an account's balance for one asset until it carries
// exactly wantAvailable with nothing on hold, or the deadline passes.
//
// The wait is bounded and re-reads on an interval rather than spinning. It
// exists so a stack that applies operations a beat behind its response reports
// a real mismatch instead of a race, and it never softens the assertion: the
// values still have to match exactly, and a timeout is a failure carrying the
// last value it actually saw.
func awaitBalance(ctx context.Context, c *midaz.Client, lc *ledgerContext, accountID, alias, asset string, wantAvailable decimal.Decimal) error {
	deadline := time.Now().Add(balanceSettleTimeout)

	var lastSeen string

	for {
		balance, err := readAssetBalance(ctx, c, lc, accountID, asset)
		switch {
		case errors.Is(err, errNoAssetBalance):
			lastSeen = "no balance for the asset yet"
		case err != nil:
			lastSeen = fmt.Sprintf("read failed: %v", err)
		default:
			if balance.Available.Equal(wantAvailable) && balance.OnHold.IsZero() {
				return nil
			}

			lastSeen = fmt.Sprintf("available=%s onHold=%s", balance.Available.String(), balance.OnHold.String())
		}

		if time.Now().After(deadline) {
			return fmt.Errorf(
				"balance assertion failed for account %s (%s): expected available=%s onHold=0 in %s, last read %s",
				alias, accountID, wantAvailable.String(), asset, lastSeen)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(balancePollInterval):
		}
	}
}

// errNoAssetBalance reports that the account exists but carries no balance for
// the asset asked about. It is a distinct condition from a failed read: right
// after an account is opened, the ledger may not have materialized its balance
// yet, so awaitBalance retries on it instead of treating it as an error.
var errNoAssetBalance = errors.New("account carries no balance for the asset")

// readAssetBalance returns the account's DEFAULT balance for one asset.
//
// The default matters: an account can carry several balances for one asset, each
// under its own key. Taking the first match on asset code alone made the
// assertion depend on the order the server happened to list them in — a keyed
// balance (an asset-freeze, say) holds value this demo never moved, so comparing
// against it would fail a perfectly healthy ledger with a message about the
// money path.
func readAssetBalance(ctx context.Context, c *midaz.Client, lc *ledgerContext, accountID, asset string) (*models.Balance, error) {
	balances, err := c.V2.Balances.ListAccountBalances(ctx, lc.org.ID, lc.ledger.ID, accountID, models.BalancesListOpts{})
	if err != nil {
		return nil, err
	}

	if balances == nil {
		return nil, errors.New("empty balance response")
	}

	candidates := defaultAssetBalances(balances.Items, asset)

	switch len(candidates) {
	case 0:
		return nil, errNoAssetBalance
	case 1:
		return candidates[0], nil
	default:
		// Ambiguous rather than wrong: say so instead of picking one and
		// asserting against a balance nobody chose.
		keys := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			keys = append(keys, fmt.Sprintf("%q", candidate.Key))
		}

		return nil, fmt.Errorf("account carries %d balances for %s (keys %s) and none of them is unambiguously the default",
			len(candidates), asset, strings.Join(keys, ", "))
	}
}

// defaultAssetBalances narrows an account's balances to the ones that can be the
// asset's default: the default-keyed ones when any exist, every balance for the
// asset otherwise. An empty key reads as the default too — a deployment that
// does not spell the default out still returns one balance per asset.
func defaultAssetBalances(items []models.Balance, asset string) []*models.Balance {
	var defaults, keyed []*models.Balance

	for i := range items {
		if items[i].AssetCode != asset {
			continue
		}

		if items[i].Key == defaultBalanceKey || items[i].Key == "" {
			defaults = append(defaults, &items[i])

			continue
		}

		keyed = append(keyed, &items[i])
	}

	if len(defaults) > 0 {
		return defaults
	}

	return keyed
}

// runV2FeePhase writes one fee package and runs one estimate against it. The
// estimate is a dry run: it returns the fee-adjusted shape of a transaction
// without posting anything.
func runV2FeePhase(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string, demo v2DemoAccounts) {
	asset := state.demoConfig.assetCodeVal

	scale := lc.assetScales[asset]
	if scale == 0 {
		scale = 2
	}

	unit := pow10(scale)
	deductible := true

	fees := map[string]models.Fee{
		"demo_transfer_fee": {
			FeeLabel: "Demo transfer fee",
			CalculationModel: models.FeeCalculationModel{
				ApplicationRule: "flatFee",
				Calculations: []models.Calculation{
					{Type: "flat", Value: formatAmountByScale(unit, int64(scale))},
				},
			},
			ReferenceAmount:  "originalAmount",
			IsDeductibleFrom: &deductible,
			CreditAccount:    demo.destAlias,
		},
	}

	input := models.NewCreatePackageInput(
		fmt.Sprintf("demo-fees-%s", prefix),
		lc.ledger.ID,
		formatAmountByScale(unit, int64(scale)),
		formatAmountByScale(10_000*unit, int64(scale)),
		fees,
	).
		WithDescription("Mass demo generator fee package").
		WithEnable(true)

	pkg, err := c.V2.FeePackages.Create(ctx, lc.org.ID, lc.ledger.ID, input)
	if !logV2Step("fee package create", err) {
		return
	}

	state.apiCalls++
	state.reportEntities.Counts.FeePackages++
	state.reportEntities.IDs.FeePackageIDs = append(state.reportEntities.IDs.FeePackageIDs, pkg.ID)
	fmt.Println("V2 fees: created fee package", shortID(pkg.ID))

	amount := formatAmountByScale(10*unit, int64(scale))
	send := &models.SendInput{
		Asset: asset,
		Value: amount,
		Source: &models.SourceInput{From: []models.FromToInput{{
			AccountAlias: demo.sourceAlias,
			Amount:       models.AmountInput{Asset: asset, Value: amount},
		}}},
		Distribute: &models.DistributeInput{To: []models.FromToInput{{
			AccountAlias: demo.destAlias,
			Amount:       models.AmountInput{Asset: asset, Value: amount},
		}}},
	}

	estimate, err := c.V2.FeeEstimates.EstimateFee(ctx, lc.org.ID, lc.ledger.ID,
		models.NewFeeEstimateInput(pkg.ID, lc.ledger.ID, send).
			WithDescription("Mass demo generator fee estimate"))
	if !logV2Step("fee estimate", err) {
		return
	}

	state.apiCalls++

	// FeesApplied is nil when no rule matched, and that is a valid answer
	// rather than an error — the estimate ran either way.
	fmt.Printf("V2 fees: estimate returned %q (rules matched: %t)\n", estimate.Message, estimate.FeesApplied != nil)
}

// runV2BillingPhase writes one billing package and runs one calculation over
// the current month.
//
// The billing family is LEDGER-scoped on /v2 — it moved there from organization
// scope — so both calls carry the ledger.
func runV2BillingPhase(ctx context.Context, c *midaz.Client, state *workflowState, lc *ledgerContext, prefix string, demo v2DemoAccounts) {
	asset := state.demoConfig.assetCodeVal

	scale := lc.assetScales[asset]
	if scale == 0 {
		scale = 2
	}

	aliases := []string{demo.sourceAlias}
	input := models.NewCreateMaintenanceBillingPackageInput(
		fmt.Sprintf("demo-billing-%s", prefix),
		lc.ledger.ID,
		asset,
		formatAmountByScale(pow10(scale), int64(scale)),
		demo.destAlias,
	).
		WithDescription("Mass demo generator maintenance billing package").
		WithEnable(true).
		WithAccountTarget(models.BillingAccountTarget{Aliases: &aliases})

	pkg, err := c.V2.BillingPackages.Create(ctx, lc.org.ID, lc.ledger.ID, input)
	if !logV2Step("billing package create", err) {
		return
	}

	state.apiCalls++
	state.reportEntities.Counts.BillingPackages++
	state.reportEntities.IDs.BillingPackageIDs = append(state.reportEntities.IDs.BillingPackageIDs, pkg.ID)
	fmt.Println("V2 billing: created billing package", shortID(pkg.ID))

	period := time.Now().UTC().Format("2006-01")

	result, err := c.V2.BillingCalculations.CalculateBilling(ctx, lc.org.ID, lc.ledger.ID,
		models.NewBillingCalculateInput(lc.ledger.ID, period))
	if !logV2Step("billing calculate", err) {
		return
	}

	state.apiCalls++
	fmt.Printf("V2 billing: calculated period %s — %d result(s), net %s\n",
		period, result.Summary.TotalResults, result.Summary.TotalNetAmount)
}

// runV2ReadOnlyPhase runs the V2-only families that have no safe demo write.
//
// Encryption is READ ONLY here. Provisioning envelope encryption writes real
// key material into the deployment's KMS and keyset store, which is not
// something a throwaway demo should do to a stack it does not own; the status
// read answers the same question — is this organization provisioned — for free.
// A 404 there is information too: it means the whole feature is disabled at the
// deployment level, as opposed to a 200 saying provisioned:false.
//
// The protection audit trail is a plain read and needs nothing provisioned to
// answer.
func runV2ReadOnlyPhase(ctx context.Context, c *midaz.Client, lc *ledgerContext, holders []*models.Holder) {
	status, err := c.V2.Encryption.GetProvisioningStatus(ctx, lc.org.ID)

	switch {
	case sdkerrors.IsFeatureNotAvailable(err):
		// Reported apart from a generic skip because it answers a different
		// question: the deployment does not serve envelope encryption at all,
		// as opposed to a call that failed or an organization that is simply
		// not provisioned yet.
		fmt.Println("V2 encryption: feature not available on this deployment")
	case err != nil:
		logV2Step("encryption status", err)
	default:
		fmt.Printf("V2 encryption: organization provisioned=%t status=%q\n", status.Provisioned, status.Status)
	}

	events, err := c.V2.ProtectionAudit.ListAuditEvents(ctx, lc.org.ID, models.AuditEventsListOpts{})
	if logV2Step("protection audit list", err) {
		fmt.Printf("V2 protection audit: %d event(s) readable\n", len(events.Items))
	}

	if len(holders) > 0 && holders[0].ID != nil {
		// The ledger is passed because the endpoint requires it as a query
		// parameter, even though the holder alone is in the path.
		accounts, err := c.V2.Instruments.ListAccountsByHolder(ctx, lc.org.ID, lc.ledger.ID, holders[0].ID.String(), models.AccountsListOpts{})
		if logV2Step("holder accounts list", err) {
			fmt.Printf("V2 CRM: first holder owns %d account(s)\n", len(accounts.Items))
		}
	}
}

// runV2MetadataIndexPhase creates, lists and drops one metadata index.
//
// It runs ONCE per run rather than per ledger, because the resource is global:
// /v2/settings/metadata-indexes carries no organization or ledger in its path,
// so a per-ledger cycle would race itself creating the same index twice.
func runV2MetadataIndexPhase(ctx context.Context, c *midaz.Client, state *workflowState) {
	t0 := time.Now()

	index, err := c.V2.MetadataIndexes.Create(ctx, demoMetadataIndexEntity,
		models.NewCreateMetadataIndexInput(demoMetadataIndexKey).WithSparse(true))
	if !logV2Step("metadata index create", err) {
		return
	}

	state.apiCalls++
	fmt.Printf("V2 metadata indexes: created %q on %s\n", index.IndexName, demoMetadataIndexEntity)

	indexes, err := c.V2.MetadataIndexes.List(ctx, demoMetadataIndexEntity)
	if logV2Step("metadata index list", err) {
		state.apiCalls++
		fmt.Printf("V2 metadata indexes: %s carries %d index(es)\n", demoMetadataIndexEntity, len(indexes))
	}

	// Dropped again so a re-run starts from the same state it found. The index
	// is global, and leaving it behind would make the second run's create fail
	// on a conflict that says nothing about the SDK.
	//
	// The asymmetry with the fee and billing packages is deliberate: those are
	// per-ledger, uniquely named, and are exactly the demo data this generator
	// exists to leave behind, so they stay enabled. Only the global resource is
	// cleaned up, because only the global resource collides with itself.
	if logV2Step("metadata index delete", c.V2.MetadataIndexes.Delete(ctx, demoMetadataIndexEntity, demoMetadataIndexKey)) {
		state.apiCalls++
		fmt.Println("V2 metadata indexes: dropped the demo index")
	}

	state.stepTimings["v2_metadata_indexes"] = time.Since(t0).String()
}
