package entities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboardingContractEntities_UseContractMethodsPathsAndBodies(t *testing.T) {
	var nilContext context.Context

	tests := []struct {
		name        string
		call        func(baseURL string) error
		wantMethod  string
		wantPath    string
		wantBodyKey string
	}{
		{
			name: "organization create",
			call: func(baseURL string) error {
				svc := newOrganizationsEntity(nil, "token", map[string]string{"onboarding": baseURL})
				_, err := svc.CreateOrganization(nilContext, models.NewCreateOrganizationInput("Lerian", "123"))

				return err
			},
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations",
			wantBodyKey: "legalName",
		},
		{
			name: "account type create",
			call: func(baseURL string) error {
				svc := newAccountTypesEntity(nil, "token", map[string]string{"onboarding": baseURL})
				_, err := svc.CreateAccountType(nilContext, "org/1", "ledger/1", models.NewCreateAccountTypeInput("Deposit", "deposit"))

				return err
			},
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/org%2F1/ledgers/ledger%2F1/account-types",
			wantBodyKey: "keyValue",
		},
		{
			name: "portfolio create with optional entity omitted",
			call: func(baseURL string) error {
				svc := newPortfoliosEntity(nil, "token", map[string]string{"onboarding": baseURL})
				_, err := svc.CreatePortfolio(nilContext, "org/1", "ledger/1", models.NewCreatePortfolioInput("", "Retail"))

				return err
			},
			wantMethod:  http.MethodPost,
			wantPath:    "/organizations/org%2F1/ledgers/ledger%2F1/portfolios",
			wantBodyKey: "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan *http.Request, 1)

			var body map[string]any

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen <- r

				if r.Body != nil {
					defer r.Body.Close()

					_ = json.NewDecoder(r.Body).Decode(&body)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			require.NoError(t, tt.call(server.URL+"/"))

			req := <-seen
			assert.Equal(t, tt.wantMethod, req.Method)
			assert.Equal(t, tt.wantPath, req.URL.EscapedPath())
			assert.NotEmpty(t, req.Header.Get("Authorization"))
			assert.Contains(t, body, tt.wantBodyKey)
			assert.NotContains(t, body, "metadata")
		})
	}
}

func TestOnboardingContractAccountBalanceHelpers_UseTransactionRouteAndLimitTwo(t *testing.T) {
	var nilContext context.Context

	onboarding := httptest.NewServer(http.NotFoundHandler())
	defer onboarding.Close()

	transaction := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/organizations/org/ledgers/ledger/accounts/external/USD/balances", r.URL.EscapedPath())
		assert.Equal(t, "2", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"bal-1"}]}`))
	}))
	defer transaction.Close()

	svc := newAccountsEntity(transaction.Client(), "token", map[string]string{
		"onboarding":  onboarding.URL,
		"transaction": transaction.URL,
	})

	balance, err := svc.GetExternalAccountBalance(nilContext, "org", "ledger", "USD")
	require.NoError(t, err)
	assert.Equal(t, "bal-1", balance.ID)
}

func TestOnboardingContractDirectConstructors_CopyBaseURLs(t *testing.T) {
	baseURLs := map[string]string{"onboarding": "https://api.example.com/"}
	svc := newSegmentsEntity(nil, "token", baseURLs).(*segmentsEntity)
	baseURLs["onboarding"] = "https://evil.example.com"

	assert.Equal(t, "https://api.example.com", svc.baseURLs["onboarding"])
}

func TestOnboardingContractUnsafeRetriesRequireCallerIdempotencyKey(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewHTTPClient(server.Client(), "", nil)
	require.NoError(t, client.WithRetryOptions(retry.WithMaxRetries(1), retry.WithInitialDelay(time.Millisecond), retry.WithMaxDelay(time.Millisecond)))

	var out map[string]any

	err := client.doRequest(sdkctx.WithIdempotencyKey(context.Background(), "caller-key"), http.MethodPost, server.URL, nil, map[string]string{"ok": "true"}, &out)
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())

	calls.Store(0)
	client.SetEnableIdempotency(false)
	err = client.doRequest(context.Background(), http.MethodPost, server.URL, nil, map[string]string{"ok": "true"}, &out)
	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load())
}
