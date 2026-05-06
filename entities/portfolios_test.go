package entities

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/entities/mocks"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Portfolio Tests

// \1 performs an operation
func TestListPortfolios(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock portfolios service
	mockService := mocks.NewMockPortfoliosService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	now := time.Now()

	// Create test portfolios list response
	portfoliosList := &models.ListResponse[models.Portfolio]{
		Items: []models.Portfolio{
			{
				ID:             "portfolio-123",
				Name:           "Investment Portfolio",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Status: models.Status{
					Code: "ACTIVE",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:             "portfolio-456",
				Name:           "Savings Portfolio",
				OrganizationID: orgID,
				LedgerID:       ledgerID,
				Status: models.Status{
					Code: "ACTIVE",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Pagination: models.Pagination{
			Total:  2,
			Limit:  10,
			Offset: 0,
		},
	}

	// Setup expectations for default options
	mockService.EXPECT().
		ListPortfolios(gomock.Any(), orgID, ledgerID, gomock.Any()).
		Return(portfoliosList, nil)

	// Test listing portfolios with default options
	result, err := mockService.ListPortfolios(ctx, orgID, ledgerID, models.PortfoliosListOpts{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Pagination.Total)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "portfolio-123", result.Items[0].ID)
	assert.Equal(t, "Investment Portfolio", result.Items[0].Name)
	assert.Equal(t, "ACTIVE", result.Items[0].Status.Code)
	assert.Equal(t, orgID, result.Items[0].OrganizationID)
	assert.Equal(t, ledgerID, result.Items[0].LedgerID)

	// Test with options
	opts := models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 5, Page: 1, OrderBy: "created_at", SortDirection: models.SortDescending}}

	mockService.EXPECT().
		ListPortfolios(gomock.Any(), orgID, ledgerID, opts).
		Return(portfoliosList, nil)

	result, err = mockService.ListPortfolios(ctx, orgID, ledgerID, opts)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Pagination.Total)

	// Test with empty organizationID
	mockService.EXPECT().
		ListPortfolios(gomock.Any(), "", ledgerID, gomock.Any()).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.ListPortfolios(ctx, "", ledgerID, models.PortfoliosListOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		ListPortfolios(gomock.Any(), orgID, "", gomock.Any()).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.ListPortfolios(ctx, orgID, "", models.PortfoliosListOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")
}

func TestPortfoliosEntity_ListPortfolios_HTTPServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/organizations/org-123/ledgers/ledger-456/portfolios", r.URL.Path)

		query := r.URL.Query()
		assert.Equal(t, "25", query.Get("limit"))
		assert.Equal(t, "2", query.Get("page"))
		assert.Equal(t, "desc", query.Get("sort_order"))
		assert.Equal(t, "2026-01-01", query.Get("start_date"))
		assert.Equal(t, "2026-01-31", query.Get("end_date"))
		assert.Equal(t, "Treasury", query.Get("name"))
		assert.Equal(t, "entity-789", query.Get("entity_id"))
		assert.Equal(t, "ACTIVE", query.Get("status"))
		assert.Equal(t, "true", query.Get("include_deleted"))
		assert.Empty(t, query.Get("orderBy"), "OrderBy is retained on opts but not emitted by page query params")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(models.ListResponse[models.Portfolio]{
			Items: []models.Portfolio{
				{
					ID:             "portfolio-123",
					Name:           "Treasury",
					EntityID:       "entity-789",
					OrganizationID: "org-123",
					LedgerID:       "ledger-456",
					Status:         models.Status{Code: "ACTIVE"},
				},
			},
			Pagination: models.Pagination{Total: 1, Limit: 25, Page: 2},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	entity := newPortfoliosEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})
	opts := models.PortfoliosListOpts{
		PageListOpts: models.PageListOpts{
			Limit:         25,
			Page:          2,
			OrderBy:       "createdAt",
			SortDirection: models.SortDescending,
			StartDate:     "2026-01-01",
			EndDate:       "2026-01-31",
		},
		Filters: models.PortfoliosFilters{
			Name:           "Treasury",
			EntityID:       "entity-789",
			Status:         "ACTIVE",
			IncludeDeleted: true,
		},
	}

	result, err := entity.ListPortfolios(context.Background(), "org-123", "ledger-456", opts)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "portfolio-123", result.Items[0].ID)
	assert.Equal(t, "Treasury", result.Items[0].Name)
	assert.Equal(t, "ACTIVE", result.Items[0].Status.Code)
	assert.Equal(t, 1, result.Pagination.Total)
	assert.Equal(t, 25, result.Pagination.Limit)
}

func TestPortfoliosEntity_ListPortfolios_ValidationBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	entity := newPortfoliosEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	_, err := entity.ListPortfolios(context.Background(), "", "ledger-456", models.PortfoliosListOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organizationID")

	_, err = entity.ListPortfolios(context.Background(), "org-123", "", models.PortfoliosListOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledgerID")

	assert.Equal(t, int32(0), requests.Load(), "validation errors must return before sending HTTP requests")
}

func TestPortfoliosEntity_ListPortfoliosPages_NilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/organizations/org-123/ledgers/ledger-456/portfolios", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(models.ListResponse[models.Portfolio]{
			Items: []models.Portfolio{
				{ID: "portfolio-123", Name: "Treasury", Status: models.Status{Code: "ACTIVE"}},
			},
			Pagination: models.Pagination{Total: 1, Limit: 10, Page: 1},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	entity := newPortfoliosEntity(server.Client(), "test-token", map[string]string{"onboarding": server.URL})

	var pages []*models.ListResponse[models.Portfolio]
	var nilCtx context.Context
	for page, err := range entity.ListPortfoliosPages(nilCtx, "org-123", "ledger-456", models.PortfoliosListOpts{PageListOpts: models.PageListOpts{Limit: 10}}) {
		require.NoError(t, err)
		pages = append(pages, page)
	}

	require.Len(t, pages, 1)
	require.Len(t, pages[0].Items, 1)
	assert.Equal(t, "portfolio-123", pages[0].Items[0].ID)
}

// \1 performs an operation
func TestGetPortfolio(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock portfolios service
	mockService := mocks.NewMockPortfoliosService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	portfolioID := "portfolio-123"
	now := time.Now()

	// Create test portfolio
	portfolio := &models.Portfolio{
		ID:             portfolioID,
		Name:           "Investment Portfolio",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Status: models.Status{
			Code: "ACTIVE",
		},
		Metadata:  map[string]any{"type": "investment"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		GetPortfolio(gomock.Any(), orgID, ledgerID, portfolioID).
		Return(portfolio, nil)

	// Test getting a portfolio by ID
	result, err := mockService.GetPortfolio(ctx, orgID, ledgerID, portfolioID)
	require.NoError(t, err)
	assert.Equal(t, portfolioID, result.ID)
	assert.Equal(t, "Investment Portfolio", result.Name)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.Equal(t, orgID, result.OrganizationID)
	assert.Equal(t, ledgerID, result.LedgerID)
	assert.Equal(t, "investment", result.Metadata["type"])

	// Test with empty organizationID
	mockService.EXPECT().
		GetPortfolio(gomock.Any(), "", ledgerID, portfolioID).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.GetPortfolio(ctx, "", ledgerID, portfolioID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		GetPortfolio(gomock.Any(), orgID, "", portfolioID).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.GetPortfolio(ctx, orgID, "", portfolioID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty portfolioID
	mockService.EXPECT().
		GetPortfolio(gomock.Any(), orgID, ledgerID, "").
		Return(nil, errors.New("portfolio ID is required"))

	_, err = mockService.GetPortfolio(ctx, orgID, ledgerID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio ID is required")

	// Test with not found
	mockService.EXPECT().
		GetPortfolio(gomock.Any(), orgID, ledgerID, "not-found").
		Return(nil, errors.New("Portfolio not found"))

	_, err = mockService.GetPortfolio(ctx, orgID, ledgerID, "not-found")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestCreatePortfolio(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock portfolios service
	mockService := mocks.NewMockPortfoliosService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	now := time.Now()

	// Create test input
	input := models.NewCreatePortfolioInput("entity-123", "Retirement Portfolio").
		WithStatus(models.NewStatus("ACTIVE")).
		WithMetadata(map[string]any{"type": "retirement"})

	// Create expected output
	portfolio := &models.Portfolio{
		ID:             "portfolio-new",
		Name:           "Retirement Portfolio",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Status: models.Status{
			Code: "ACTIVE",
		},
		Metadata:  map[string]any{"type": "retirement"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		CreatePortfolio(gomock.Any(), orgID, ledgerID, input).
		Return(portfolio, nil)

	// Test creating a new portfolio
	result, err := mockService.CreatePortfolio(ctx, orgID, ledgerID, input)
	require.NoError(t, err)
	assert.Equal(t, "portfolio-new", result.ID)
	assert.Equal(t, "Retirement Portfolio", result.Name)
	assert.Equal(t, "ACTIVE", result.Status.Code)
	assert.Equal(t, orgID, result.OrganizationID)
	assert.Equal(t, ledgerID, result.LedgerID)
	assert.Equal(t, "retirement", result.Metadata["type"])

	// Test with empty organizationID
	mockService.EXPECT().
		CreatePortfolio(gomock.Any(), "", ledgerID, input).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.CreatePortfolio(ctx, "", ledgerID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		CreatePortfolio(gomock.Any(), orgID, "", input).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.CreatePortfolio(ctx, orgID, "", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with nil input
	mockService.EXPECT().
		CreatePortfolio(gomock.Any(), orgID, ledgerID, nil).
		Return(nil, errors.New("portfolio input cannot be nil"))

	_, err = mockService.CreatePortfolio(ctx, orgID, ledgerID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio input cannot be nil")
}

// \1 performs an operation
func TestUpdatePortfolio(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock portfolios service
	mockService := mocks.NewMockPortfoliosService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	portfolioID := "portfolio-123"
	now := time.Now()

	// Create test input
	input := models.NewUpdatePortfolioInput().
		WithName("Updated Investment Portfolio").
		WithStatus(models.NewStatus("INACTIVE")).
		WithMetadata(map[string]any{"type": "updated-investment"})

	// Create expected output
	portfolio := &models.Portfolio{
		ID:             portfolioID,
		Name:           "Updated Investment Portfolio",
		OrganizationID: orgID,
		LedgerID:       ledgerID,
		Status: models.Status{
			Code: "INACTIVE",
		},
		Metadata:  map[string]any{"type": "updated-investment"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Setup expectation for successful case
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), orgID, ledgerID, portfolioID, input).
		Return(portfolio, nil)

	// Test updating a portfolio
	result, err := mockService.UpdatePortfolio(ctx, orgID, ledgerID, portfolioID, input)
	require.NoError(t, err)
	assert.Equal(t, portfolioID, result.ID)
	assert.Equal(t, "Updated Investment Portfolio", result.Name)
	assert.Equal(t, "INACTIVE", result.Status.Code)
	assert.Equal(t, orgID, result.OrganizationID)
	assert.Equal(t, ledgerID, result.LedgerID)
	assert.Equal(t, "updated-investment", result.Metadata["type"])

	// Test with empty organizationID
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), "", ledgerID, portfolioID, input).
		Return(nil, errors.New("organization ID is required"))

	_, err = mockService.UpdatePortfolio(ctx, "", ledgerID, portfolioID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), orgID, "", portfolioID, input).
		Return(nil, errors.New("ledger ID is required"))

	_, err = mockService.UpdatePortfolio(ctx, orgID, "", portfolioID, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty portfolioID
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), orgID, ledgerID, "", input).
		Return(nil, errors.New("portfolio ID is required"))

	_, err = mockService.UpdatePortfolio(ctx, orgID, ledgerID, "", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio ID is required")

	// Test with nil input
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), orgID, ledgerID, portfolioID, nil).
		Return(nil, errors.New("portfolio input cannot be nil"))

	_, err = mockService.UpdatePortfolio(ctx, orgID, ledgerID, portfolioID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio input cannot be nil")

	// Test with not found
	mockService.EXPECT().
		UpdatePortfolio(gomock.Any(), orgID, ledgerID, "not-found", input).
		Return(nil, errors.New("Portfolio not found"))

	_, err = mockService.UpdatePortfolio(ctx, orgID, ledgerID, "not-found", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// \1 performs an operation
func TestDeletePortfolio(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock portfolios service
	mockService := mocks.NewMockPortfoliosService(ctrl)

	// Test data
	ctx := context.Background()
	orgID := "org-123"
	ledgerID := "ledger-123"
	portfolioID := "portfolio-123"

	// Setup expectation for successful case
	mockService.EXPECT().
		DeletePortfolio(gomock.Any(), orgID, ledgerID, portfolioID).
		Return(nil)

	// Test deleting a portfolio
	err := mockService.DeletePortfolio(ctx, orgID, ledgerID, portfolioID)
	require.NoError(t, err)

	// Test with empty organizationID
	mockService.EXPECT().
		DeletePortfolio(gomock.Any(), "", ledgerID, portfolioID).
		Return(errors.New("organization ID is required"))

	err = mockService.DeletePortfolio(ctx, "", ledgerID, portfolioID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "organization ID is required")

	// Test with empty ledgerID
	mockService.EXPECT().
		DeletePortfolio(gomock.Any(), orgID, "", portfolioID).
		Return(errors.New("ledger ID is required"))

	err = mockService.DeletePortfolio(ctx, orgID, "", portfolioID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledger ID is required")

	// Test with empty portfolioID
	mockService.EXPECT().
		DeletePortfolio(gomock.Any(), orgID, ledgerID, "").
		Return(errors.New("portfolio ID is required"))

	err = mockService.DeletePortfolio(ctx, orgID, ledgerID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "portfolio ID is required")

	// Test with not found
	mockService.EXPECT().
		DeletePortfolio(gomock.Any(), orgID, ledgerID, "not-found").
		Return(errors.New("Portfolio not found"))

	err = mockService.DeletePortfolio(ctx, orgID, ledgerID, "not-found")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
