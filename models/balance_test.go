package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceDecodesMidazSettings(t *testing.T) {
	tests := []struct {
		name string
		body string
		want *BalanceSettings
	}{
		{
			name: "overdraft with limit",
			body: `{"id":"b1","available":"0","settings":{"balanceScope":"transactional","allowOverdraft":true,"overdraftLimitEnabled":true,"overdraftLimit":"1000.00"}}`,
			want: &BalanceSettings{BalanceScope: BalanceScopeTransactional, AllowOverdraft: true, OverdraftLimitEnabled: true, OverdraftLimit: ptrString("1000.00")},
		},
		{
			name: "overdraft off",
			body: `{"id":"b1","available":"0","settings":{"balanceScope":"internal","allowOverdraft":false,"overdraftLimitEnabled":false}}`,
			want: &BalanceSettings{BalanceScope: BalanceScopeInternal},
		},
		{
			name: "no settings",
			body: `{"id":"b1","available":"0"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var balance Balance
			require.NoError(t, json.Unmarshal([]byte(tt.body), &balance))
			assert.Equal(t, tt.want, balance.Settings)

			var history BalanceHistory
			require.NoError(t, json.Unmarshal([]byte(tt.body), &history))
			assert.Equal(t, tt.want, history.Settings)
		})
	}
}
