// SPDX-License-Identifier: Elastic-2.0

package contract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	v3mmodel "github.com/LerianStudio/midaz/v3/pkg/mmodel"
	v4mmodel "github.com/LerianStudio/midaz/v4/pkg/mmodel"
)

var (
	crmItemID    = uuid.MustParse("0190a5f5-4d5e-7f60-8b12-23456789abcd")
	crmHolderID  = uuid.MustParse("0190a5f5-3c4d-7e5f-8a01-123456789abc")
	crmLedgerID  = uuid.MustParse("0190a5f5-5e6f-7071-8c23-3456789abcde")
	crmAccountID = uuid.MustParse("0190a5f5-6f70-7182-8d34-456789abcdef")
	crmCreatedAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	crmDeletedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
)

// decodeCRMPage marshals item inside the CRM list envelope and decodes it as CRMReader does.
func decodeCRMPage[T any](t *testing.T, item any) (*models.ListResponse[T], error) {
	t.Helper()

	body, err := json.Marshal(map[string]any{"items": []any{item}, "limit": 100, "page": 1})
	require.NoError(t, err)

	var page models.ListResponse[T]

	return &page, json.Unmarshal(body, &page)
}

// TestCRMInstrumentDecodesFromServer pins the CRM 3.x alias (/v1) and the ledger 4.x instrument (/v2)
// to models.Instrument, so a server rename of a field the engines read fails here.
func TestCRMInstrumentDecodesFromServer(t *testing.T) {
	want := models.Instrument{
		ID: &crmItemID, Document: new("12345678909"), Type: new("NATURAL_PERSON"),
		LedgerID: &crmLedgerID, AccountID: &crmAccountID, HolderID: crmHolderID,
		Metadata: map[string]any{"midazAccountAlias": "@ana"},
		BankingDetails: &models.BankingDetails{
			Branch: new("0001"), Account: new("123450"), Type: new("CACC"), OpeningDate: new("2025-01-01"), BankID: new("13935893"),
		},
		CreatedAt: crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
	}

	for name, server := range map[string]any{
		"CRM 3.x alias": v3mmodel.Alias{
			ID: &crmItemID, Document: new("12345678909"), Type: new("NATURAL_PERSON"),
			LedgerID: new(crmLedgerID.String()), AccountID: new(crmAccountID.String()), HolderID: &crmHolderID,
			Metadata: map[string]any{"midazAccountAlias": "@ana"},
			BankingDetails: &v3mmodel.BankingDetails{
				Branch: new("0001"), Account: new("123450"), Type: new("CACC"), OpeningDate: new("2025-01-01"), BankID: new("13935893"),
			},
			CreatedAt: crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
		},
		"ledger 4.x instrument": v4mmodel.Instrument{
			ID: &crmItemID, Document: new("12345678909"), Type: new("NATURAL_PERSON"),
			LedgerID: new(crmLedgerID.String()), AccountID: new(crmAccountID.String()), HolderID: &crmHolderID,
			Metadata: map[string]any{"midazAccountAlias": "@ana"},
			BankingDetails: &v4mmodel.BankingDetails{
				Branch: new("0001"), Account: new("123450"), Type: new("CACC"), OpeningDate: new("2025-01-01"), BankID: new("13935893"),
			},
			CreatedAt: crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
		},
	} {
		t.Run(name, func(t *testing.T) {
			page, err := decodeCRMPage[models.Instrument](t, server)
			require.NoError(t, err)
			assert.Equal(t, 100, page.Pagination.Limit)
			assert.Equal(t, []models.Instrument{want}, page.Items)
		})
	}
}

// TestCRMHolderDecodesFromServer pins the CRM 3.x holder (/v1) and the ledger 4.x holder (/v2) to
// models.Holder.
func TestCRMHolderDecodesFromServer(t *testing.T) {
	want := models.Holder{
		ID: &crmHolderID, ExternalID: new("ext-1"), Type: new("LEGAL_PERSON"), Name: new("Ana Ltda"), Document: new("12345678000190"),
		Addresses: &models.Addresses{Primary: &models.Address{
			Line1: "Rua A", ZipCode: "01000-000", City: "Sao Paulo", State: "SP", Country: "BR",
		}},
		LegalPerson: &models.LegalPerson{TradeName: new("Ana")},
		CreatedAt:   crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
	}

	for name, server := range map[string]any{
		"CRM 3.x holder": v3mmodel.Holder{
			ID: &crmHolderID, ExternalID: new("ext-1"), Type: new("LEGAL_PERSON"), Name: new("Ana Ltda"), Document: new("12345678000190"),
			Addresses: &v3mmodel.Addresses{Primary: &v3mmodel.Address{
				Line1: "Rua A", ZipCode: "01000-000", City: "Sao Paulo", State: "SP", Country: "BR",
			}},
			LegalPerson: &v3mmodel.LegalPerson{TradeName: new("Ana")},
			CreatedAt:   crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
		},
		"ledger 4.x holder": v4mmodel.Holder{
			ID: &crmHolderID, ExternalID: new("ext-1"), Type: new("LEGAL_PERSON"), Name: new("Ana Ltda"), Document: new("12345678000190"),
			Addresses: &v4mmodel.Addresses{Primary: &v4mmodel.Address{
				Line1: "Rua A", ZipCode: "01000-000", City: "Sao Paulo", State: "SP", Country: "BR",
			}},
			LegalPerson: &v4mmodel.LegalPerson{TradeName: new("Ana")},
			CreatedAt:   crmCreatedAt, UpdatedAt: crmCreatedAt, DeletedAt: &crmDeletedAt,
		},
	} {
		t.Run(name, func(t *testing.T) {
			page, err := decodeCRMPage[models.Holder](t, server)
			require.NoError(t, err)
			assert.Equal(t, []models.Holder{want}, page.Items)
		})
	}
}

// TestCRM3AliasWithNonUUIDAccountIDFailsThePage pins today's fail-closed decode: CRM 3.x stores any
// string as accountId, models.Instrument types it as a UUID, so one such alias fails the whole page.
func TestCRM3AliasWithNonUUIDAccountIDFailsThePage(t *testing.T) {
	_, err := decodeCRMPage[models.Instrument](t, v3mmodel.Alias{AccountID: new("acc-legacy-1"), HolderID: &crmHolderID})
	require.ErrorContains(t, err, "UUID")
}
