// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestMockgen_NewServiceMocks_AreUsable is the H31 regression: the
// AliasesService //go:generate mockgen directive must produce a mock that
// satisfies the interface and can be programmed via gomock. Epic 5.4 deleted
// the AssetRates/Holders/MetadataIndexes legacy services (now facades) along
// with their mocks, so only the surviving trio member Aliases remains here.
func TestMockgen_NewServiceMocks_AreUsable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Compile-time interface satisfaction. A drift between the source
	// interface and the generated mock surfaces as a build error here.
	var _ AliasesService = (*mocks.MockAliasesService)(nil)

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
}
