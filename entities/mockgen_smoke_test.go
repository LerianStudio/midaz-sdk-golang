// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestMockgen_NewServiceMocks_AreUsable is the H31 regression. Wave 1
// ramped up coverage for AliasesService, AssetRatesService, HoldersService,
// and MetadataIndexesService — but their //go:generate mockgen directives
// were missing, so callers writing unit tests against a Midaz SDK consumer
// could not mock them via entities/mocks/.
//
// This test asserts:
//   - The four new generated mocks satisfy their respective service
//     interfaces (compile-time check via the type assertion below).
//   - A trivial EXPECT/Call round-trip works against each mock so a
//     regression in mockgen output (e.g. a method-signature drift) is
//     caught at test time, not when a downstream test fails on a missing
//     method.
//
// Smoke test only — no semantic assertions about the underlying entity
// behavior. The point is: "the mock exists, satisfies the interface,
// and can be programmed via gomock".
func TestMockgen_NewServiceMocks_AreUsable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Compile-time interface satisfaction. A drift between the source
	// interface and the generated mock surfaces as a build error here.
	var (
		_ AliasesService         = (*mocks.MockAliasesService)(nil)
		_ AssetRatesService      = (*mocks.MockAssetRatesService)(nil)
		_ HoldersService         = (*mocks.MockHoldersService)(nil)
		_ MetadataIndexesService = (*mocks.MockMetadataIndexesService)(nil)
	)

	t.Run("AliasesService mock round-trips a List call", func(t *testing.T) {
		mock := mocks.NewMockAliasesService(ctrl)
		want := &models.ListResponse[models.Alias]{
			Items: []models.Alias{{}, {}},
		}

		mock.EXPECT().
			ListAliases(gomock.Any(), "org-1", gomock.Any()).
			Return(want, nil)

		got, err := mock.ListAliases(context.Background(), "org-1", models.AliasesListOpts{})
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("AssetRatesService mock round-trips a List call", func(t *testing.T) {
		mock := mocks.NewMockAssetRatesService(ctrl)
		want := &models.ListResponse[models.AssetRate]{
			Items: []models.AssetRate{},
		}

		mock.EXPECT().
			ListAssetRatesByAssetCode(gomock.Any(), "org-1", "ledger-1", "USD", gomock.Any()).
			Return(want, nil)

		got, err := mock.ListAssetRatesByAssetCode(
			context.Background(), "org-1", "ledger-1", "USD", models.AssetRatesListOpts{},
		)
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("HoldersService mock round-trips a List call", func(t *testing.T) {
		mock := mocks.NewMockHoldersService(ctrl)
		want := &models.ListResponse[models.Holder]{
			Items: []models.Holder{},
		}

		mock.EXPECT().
			ListHolders(gomock.Any(), "org-1", gomock.Any()).
			Return(want, nil)

		got, err := mock.ListHolders(context.Background(), "org-1", models.HoldersListOpts{})
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("MetadataIndexesService mock round-trips a List call", func(t *testing.T) {
		mock := mocks.NewMockMetadataIndexesService(ctrl)
		want := []models.MetadataIndex{}

		mock.EXPECT().
			ListMetadataIndexes(gomock.Any(), "Account").
			Return(want, nil)

		got, err := mock.ListMetadataIndexes(context.Background(), "Account")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
