// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// transactionsV2Facade serves the /v2 transaction surface. It is the money-write
// crown jewel of this version, and it diverges from its /v1 sibling more than
// any other family — in the URL, in the request body, and in the response.
//
// # The creates are TOP-LEVEL and carry their scope in the body
//
// /v1 posts to .../organizations/{org}/ledgers/{ledger}/transactions/{style} and
// resolves the scope from the URL. /v2 posts to a bare /v2/transactions/{action}
// and resolves the scope from the BODY: every debit and every credit leg names
// the organization and ledger its account belongs to, all of them must name the
// SAME pair, and that pair is the ledger the transaction is created in. A
// request whose legs disagree is refused by the server.
//
// The facade still takes orgID and ledgerID as arguments, and that is a
// deliberate choice with a money-path reason. Making the caller repeat the pair
// on every leg is not just tedious, it is the exact shape of mistake the server
// refuses: one leg out of twelve pointing at the wrong ledger. So the facade
// stamps the addressed pair onto every leg that leaves it empty, and refuses a
// leg that names a different one — the same reconcile-or-refuse rule the billing
// calculation applies between its path ledger and its body ledgerId. The
// argument is the scope; the body is where it travels.
//
// # The four actions differ only by endpoint
//
// direct, hold, block and unblock all take the identical body. Nothing in the
// payload says which one runs: direct settles immediately, hold opens the
// transaction PENDING for a later commit or cancel, and block/unblock label the
// resulting operations. Posting the same bytes to two of them is two different
// transactions, which is why the server folds the action into its own
// no-key idempotency identity — two byte-identical bodies to /direct and /hold
// cannot replay each other's result.
//
// # The response is TransactionV2, not Transaction
//
// It drops four /v1 fields the surface does not serve and keeps two /v1 dropped.
// See models.TransactionV2. The list, the reads, the patch and the three
// lifecycle transitions all answer with it.
type transactionsV2Facade struct {
	ledger *genledger.ClientWithResponses
	// enableIdempotency gates auto-generated X-Idempotency keys on creates; an
	// explicit input/context key stamps regardless, and the lifecycle actions
	// (commit/cancel/revert) are unaffected.
	enableIdempotency bool
}

// newTransactionsV2Facade wires the facade over a ledger plane client.
func newTransactionsV2Facade(ledger *genledger.ClientWithResponses, enableIdempotency bool) *transactionsV2Facade {
	return &transactionsV2Facade{ledger: ledger, enableIdempotency: enableIdempotency}
}

// CreateDirect posts a transaction that settles immediately, via
// POST /v2/transactions/direct.
func (f *transactionsV2Facade) CreateDirect(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.CreateDirect"

	scoped, params, err := prepareCreate(ctx, operation, f.enableIdempotency, orgID, ledgerID, input)
	if err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionV2](ctx, operation, scoped, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionDirectV2WithBody(ctx,
			&genledger.CreateTransactionDirectV2Params{XIdempotency: params.XIdempotency, XTTL: params.XTTL},
			jsonContentType, body))
	})
}

// CreateHold opens a transaction PENDING, holding the value on the debit side
// until a later Commit or Cancel, via POST /v2/transactions/hold.
func (f *transactionsV2Facade) CreateHold(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.CreateHold"

	scoped, params, err := prepareCreate(ctx, operation, f.enableIdempotency, orgID, ledgerID, input)
	if err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionV2](ctx, operation, scoped, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionHoldV2WithBody(ctx,
			&genledger.CreateTransactionHoldV2Params{XIdempotency: params.XIdempotency, XTTL: params.XTTL},
			jsonContentType, body))
	})
}

// CreateBlock posts a transaction that BLOCKS value on the accounts it names,
// via POST /v2/transactions/block. It settles immediately, like CreateDirect;
// the block label lands on the resulting operations. Release it with
// CreateUnblock, not with Cancel.
func (f *transactionsV2Facade) CreateBlock(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.CreateBlock"

	scoped, params, err := prepareCreate(ctx, operation, f.enableIdempotency, orgID, ledgerID, input)
	if err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionV2](ctx, operation, scoped, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionBlockV2WithBody(ctx,
			&genledger.CreateTransactionBlockV2Params{XIdempotency: params.XIdempotency, XTTL: params.XTTL},
			jsonContentType, body))
	})
}

// CreateUnblock releases value a block transaction held, via
// POST /v2/transactions/unblock. Same body and same immediate settlement as
// CreateBlock, with the opposing label.
func (f *transactionsV2Facade) CreateUnblock(ctx context.Context, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.CreateUnblock"

	scoped, params, err := prepareCreate(ctx, operation, f.enableIdempotency, orgID, ledgerID, input)
	if err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionV2](ctx, operation, scoped, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.CreateTransactionUnblockV2WithBody(ctx,
			&genledger.CreateTransactionUnblockV2Params{XIdempotency: params.XIdempotency, XTTL: params.XTTL},
			jsonContentType, body))
	})
}

// createParams carries the resolved idempotency headers between prepareCreate
// and each action, which needs them in its own generated params type. The four
// generated types are field-identical and mutually unassignable, so one neutral
// carrier beats four near-copies of the resolution.
type createParams struct {
	XIdempotency *string
	XTTL         *string
}

// prepareCreate runs everything the four create actions share: refuse a bad
// scope, stamp it onto every leg, refuse a bad payload, and resolve the
// idempotency key.
//
// The order matters. Scoping runs BEFORE validation because a leg's scope is
// something the facade fills in — validating first would refuse the very bodies
// the facade exists to complete. Both run before anything reaches the wire.
func prepareCreate(ctx context.Context, operation string, gate bool, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.CreateTransactionV2Input, createParams, error) {
	scoped, err := scopeTransactionV2(operation, orgID, ledgerID, input)
	if err != nil {
		return nil, createParams{}, err
	}

	if err := validationErr(operation, scoped.Validate()); err != nil {
		return nil, createParams{}, err
	}

	var params createParams

	key, ttl := resolveIdempotency(ctx, input.IdempotencyKey, gate)
	applyIdempotency(&params.XIdempotency, &params.XTTL, key, ttl)

	return scoped, params, nil
}

// scopeTransactionV2 returns a copy of input whose every leg names orgID and
// ledgerID, refusing any leg that already names a different pair.
//
// It COPIES rather than stamping in place. Debits and Credits are slices, so
// writing through them would mutate the caller's own input — and a caller who
// reuses one input across two ledgers would silently post the second
// transaction against the first ledger.
//
// A leg that already names a pair keeps its own spelling, and the comparison
// ignores letter case, because a UUID's text spelling does: two legs that spell
// one ledger in different cases name one ledger, and refusing that would reject
// a request that never left a single ledger. This is the same rule the server
// applies when it resolves the scope from the body.
func scopeTransactionV2(operation, orgID, ledgerID string, input *models.CreateTransactionV2Input) (*models.CreateTransactionV2Input, error) {
	if input == nil {
		return nil, errors.NewValidationError(operation, "input cannot be nil", nil)
	}

	if strings.TrimSpace(orgID) == "" {
		return nil, errors.NewMissingParameterError(operation, "orgID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	scoped := *input

	debits, err := scopeV2Legs(operation, "debits", orgID, ledgerID, input.Debits)
	if err != nil {
		return nil, err
	}

	credits, err := scopeV2Legs(operation, "credits", orgID, ledgerID, input.Credits)
	if err != nil {
		return nil, err
	}

	scoped.Debits, scoped.Credits = debits, credits

	return &scoped, nil
}

// scopeV2Legs copies one side's legs, filling an empty scope and refusing a
// contradicting one. The refusal names the side and the index so a caller with
// twelve legs knows which one to fix.
func scopeV2Legs(operation, side, orgID, ledgerID string, legs []models.TransactionV2Leg) ([]models.TransactionV2Leg, error) {
	if legs == nil {
		return nil, nil
	}

	out := make([]models.TransactionV2Leg, len(legs))
	copy(out, legs)

	for i := range out {
		if err := scopeLegField(operation, side, i, "organizationId", orgID, &out[i].OrganizationID); err != nil {
			return nil, err
		}

		if err := scopeLegField(operation, side, i, "ledgerId", ledgerID, &out[i].LedgerID); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// scopeLegField fills one leg identifier from the addressed scope, or refuses it
// when the leg names something else.
func scopeLegField(operation, side string, index int, field, addressed string, leg *string) error {
	if strings.TrimSpace(*leg) == "" {
		*leg = addressed

		return nil
	}

	if !strings.EqualFold(*leg, addressed) {
		return errors.NewValidationError(operation, fmt.Sprintf(
			"%s[%d].%s is %q, but the transaction was addressed to %q; every leg of a v2 transaction must name the same organization and ledger",
			side, index, field, *leg, addressed), nil)
	}

	return nil
}

// Get retrieves one transaction by ID.
func (f *transactionsV2Facade) Get(ctx context.Context, orgID, ledgerID, transactionID string) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.Get"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetTransactionV2(ctx, orgID, ledgerID, transactionID)

	return readOne[models.TransactionV2](operation, resp, err)
}

// List retrieves one cursor page of transactions under an org+ledger.
//
// Only the metadata predicate and the date range narrow the result set; the
// other TransactionsFilters fields are sent under their legacy query names and
// ignored by the ledger. That is the /v1 behaviour unchanged — see
// models.TransactionsFilters for which endpoint honours which.
func (f *transactionsV2Facade) List(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (*models.ListResponse[models.TransactionV2], error) {
	const operation = "V2.Transactions.List"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return nil, err
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readList drains and closes the body via readRawResponse.
	resp, err := f.ledger.GetAllTransactionsV2(ctx, orgID, ledgerID, listTransactionsV2Params(opts),
		listTransactionsReqEditors(opts)...)

	return readList[models.TransactionV2](operation, resp, err)
}

// Pages yields one cursor page per iteration, advancing by the response
// next_cursor until it is empty.
func (f *transactionsV2Facade) Pages(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[*models.ListResponse[models.TransactionV2], error] {
	return cursorSeq(ctx, opts,
		func(o *models.TransactionsListOpts) *string { return &o.Cursor },
		func(current models.TransactionsListOpts) (*models.ListResponse[models.TransactionV2], error) {
			return f.List(ctx, orgID, ledgerID, current)
		})
}

// All yields every transaction across cursor pages.
func (f *transactionsV2Facade) All(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[models.TransactionV2, error] {
	return flattenPages(f.Pages(ctx, orgID, ledgerID, opts))
}

// Update patches a transaction's mutable fields — description and metadata.
// The value it moved and the accounts it touched are immutable by design.
func (f *transactionsV2Facade) Update(ctx context.Context, orgID, ledgerID, transactionID string, input *models.UpdateTransactionV2Input) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.Update"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	if err := validationErr(operation, input.Validate()); err != nil {
		return nil, err
	}

	return writeJSON[models.TransactionV2](ctx, operation, input, func(body io.Reader) (*http.Response, []byte, error) {
		return readRawResponse(f.ledger.UpdateTransactionV2WithBody(ctx, orgID, ledgerID, transactionID,
			jsonContentType, body, idempotencyEditors(ctx, f.enableIdempotency)...))
	})
}

// Commit finalizes a PENDING transaction (PENDING → APPROVED).
//
// The action carries no body and is not auto-idempotent: it stamps
// X-Idempotency only when the caller supplied a key through
// sdkctx.WithIdempotencyKey. Committing twice is refused by the ledger on the
// second attempt, so an auto key would buy nothing.
func (f *transactionsV2Facade) Commit(ctx context.Context, orgID, ledgerID, transactionID string) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.Commit"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.CommitTransactionV2(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...)

	return readOne[models.TransactionV2](operation, resp, err)
}

// Revert reverses a committed transaction. It returns the CHILD reversal
// transaction — a new record whose ParentTransactionID points at the original —
// and never mutates the original.
func (f *transactionsV2Facade) Revert(ctx context.Context, orgID, ledgerID, transactionID string) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.Revert"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readOne drains and closes the body via readRawResponse.
	resp, err := f.ledger.RevertTransactionV2(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...)

	return readOne[models.TransactionV2](operation, resp, err)
}

// Cancel aborts a PENDING transaction (PENDING → CANCELED), releasing the value
// the hold reserved.
//
// It is the ONE single-object call that tolerates a 2xx carrying no resource.
// Everything else on both surfaces refuses that shape in decodeOne, because a
// zero-valued model with a nil error is indistinguishable from a real result.
//
// The exemption is NOT "the server's /v2 projection is nil-preserving", which an
// earlier version of this comment claimed. That projection (newTransactionV2,
// components/ledger/internal/adapters/http/in/transaction_v2_output.go:232) is
// shared by the creates, commit, cancel, revert, get and update alike, and its
// nil branch is not reachable from any of their success paths — so it singles
// cancel out from nothing, and the same argument would exempt commit and revert
// too. What actually justifies it is narrower:
//
//	Cancel's answer is fully determined by the request. The transaction the
//	caller named is CANCELED, and the caller supplied its id. Synthesizing
//	that invents nothing.
//
// Commit and Revert are not determined that way. A revert answers with a NEW
// CHILD transaction whose id the caller cannot know, and a commit answers with
// the settled state the caller called in order to observe. Synthesizing either
// would fabricate money data, so both fail loudly through the shared guard.
//
// The tolerated shape is a proxy or gateway that dropped the body, not a
// documented server behaviour — the /v2 cancel declares a populated response —
// and it is kept for parity with the /v1 cancel, where it was observed.
func (f *transactionsV2Facade) Cancel(ctx context.Context, orgID, ledgerID, transactionID string) (*models.TransactionV2, error) {
	const operation = "V2.Transactions.Cancel"

	if err := requirePathIDs(operation, "orgID", orgID, "ledgerID", ledgerID, "transactionID", transactionID); err != nil {
		return nil, err
	}

	//nolint:bodyclose // readRawResponse closes resp.Body via defer before returning.
	resp, body, err := readRawResponse(f.ledger.CancelTransactionV2(ctx, orgID, ledgerID, transactionID, actionIdempotencyEditors(ctx)...))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if !isSuccess(resp.StatusCode) {
		return nil, errors.DecodeProblemJSON(resp.StatusCode, body, requestIDOf(resp))
	}

	if isEmptyBody(body) {
		return &models.TransactionV2{
			ID:     transactionID,
			Status: models.Status{Code: string(models.TransactionStatusCanceled)},
		}, nil
	}

	return decodeOne[models.TransactionV2](operation, resp.StatusCode, body, resp)
}

// Count returns the number of transactions matching the count-endpoint filters.
// Only status, route and the date range are honoured there; cursor, limit and
// sort do not apply to a count.
func (f *transactionsV2Facade) Count(ctx context.Context, orgID, ledgerID string, opts models.TransactionsListOpts) (int, error) {
	if err := requirePathIDs("V2.Transactions.Count", "orgID", orgID, "ledgerID", ledgerID); err != nil {
		return 0, err
	}

	if err := opts.Validate(); err != nil {
		return 0, err
	}

	//nolint:bodyclose // readCount closes resp.Body via defer.
	return readCount(f.ledger.CountTransactionsByFiltersV2(ctx, orgID, ledgerID, countTransactionsV2Params(opts)))
}
