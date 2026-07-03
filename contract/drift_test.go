// SPDX-License-Identifier: Elastic-2.0

// Package contract pins the SDK's transaction-status vocabulary and lifecycle
// error codes against the live Midaz server contract
// (github.com/LerianStudio/midaz/v3/pkg/constant — the server source of truth).
//
// It deliberately lives in a SEPARATE nested Go module (contract/go.mod). The
// drift test imports the full Midaz server module; isolating it here keeps that
// dependency — and the server's entire transitive dependency graph — out of the
// SDK's published go.mod, so SDK consumers never inherit it. The `go build
// ./...` / `go test ./...` invocations at the SDK root skip this directory
// because it is a module boundary.
//
// Run with: make test-contract
package contract

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	srvconst "github.com/LerianStudio/midaz/v3/pkg/constant"
)

// TestTransactionStatusMatchesServer pins every SDK transaction-status constant
// to the server's source-of-truth value. A server rename or removal breaks
// compilation here (loud); a value change fails the assertion (loud).
//
// Coverage gap, stated explicitly: Go package-level consts are not enumerable
// at runtime, so this cannot auto-detect a NEW server status the SDK has not
// mirrored. If the server adds a status, add it to both this table and
// models.TransactionStatusCode.
func TestTransactionStatusMatchesServer(t *testing.T) {
	cases := []struct {
		name   string
		sdk    models.TransactionStatusCode
		server string
	}{
		{"created", models.TransactionStatusCreated, srvconst.CREATED},
		{"pending", models.TransactionStatusPending, srvconst.PENDING},
		{"approved", models.TransactionStatusApproved, srvconst.APPROVED},
		{"canceled", models.TransactionStatusCanceled, srvconst.CANCELED},
		{"noted", models.TransactionStatusNoted, srvconst.NOTED},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.server, string(c.sdk),
				"SDK transaction status %q drifted from server constant", c.name)
		})
	}
}

// TestLifecycleErrorCodesMatchServer pins the SDK's typed lifecycle / revert API
// codes to the server's error values (the server declares them as
// errors.New("00NN"), so the code is the error string).
//
// Same enumeration gap as above: a new server lifecycle error code is not
// auto-detected; add it to both pkg/errors and this table.
func TestLifecycleErrorCodesMatchServer(t *testing.T) {
	cases := []struct {
		name   string
		sdk    string
		server error
	}{
		{"parent-not-found", sdkerrors.APICodeParentTransactionIDNotFound, srvconst.ErrParentTransactionIDNotFound},
		{"revert-already-exists", sdkerrors.APICodeRevertAlreadyExists, srvconst.ErrTransactionIDHasAlreadyParentTransaction},
		{"already-a-revert", sdkerrors.APICodeAlreadyARevert, srvconst.ErrTransactionIDIsAlreadyARevert},
		{"cannot-revert", sdkerrors.APICodeCannotRevert, srvconst.ErrTransactionCantRevert},
		{"ambiguous-revert", sdkerrors.APICodeAmbiguousRevert, srvconst.ErrTransactionAmbiguous},
		{"parent-id-same-id", sdkerrors.APICodeParentIDSameID, srvconst.ErrParentIDSameID},
		{"status-precondition", sdkerrors.APICodeStatusPreconditionFailed, srvconst.ErrCommitTransactionNotPending},
		{"revert-only-bidirectional", sdkerrors.APICodeRevertOnlyBidirectional, srvconst.ErrRevertOnlyBidirectional},
		{"holder-not-found", sdkerrors.APICodeHolderNotFound, srvconst.ErrHolderNotFound},
		// 0490 (ErrSkipNotPermitted) and 0491 (ErrHolderRequired) exist only in
		// unreleased midaz — absent from the pinned v3.7.5 AND v3.8.0-rc.3, so they
		// cannot be pinned here yet. The SDK-side literals are asserted in
		// pkg/errors/catalog_test.go; add the server pins when a midaz/v3 release
		// ships ErrSkipNotPermitted/ErrHolderRequired.
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.server.Error(), c.sdk,
				"SDK lifecycle API code %q drifted from server constant", c.name)
		})
	}
}
