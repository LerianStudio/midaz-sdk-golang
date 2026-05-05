package mocks

import (
	"context"
	"iter"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/golang/mock/gomock"
)

// MockBalancesService is a mock of BalancesService interface.
type MockBalancesService struct {
	ctrl     *gomock.Controller
	recorder *MockBalancesServiceMockRecorder
}

// MockBalancesServiceMockRecorder is the mock recorder for MockBalancesService.
type MockBalancesServiceMockRecorder struct {
	mock *MockBalancesService
}

// NewMockBalancesService creates a new mock instance.
func NewMockBalancesService(ctrl *gomock.Controller) *MockBalancesService {
	mock := &MockBalancesService{ctrl: ctrl}

	mock.recorder = &MockBalancesServiceMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBalancesService) EXPECT() *MockBalancesServiceMockRecorder {
	return m.recorder
}

// ListBalances mocks base method.
func (m *MockBalancesService) ListBalances(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalances", ctx, orgID, ledgerID, opts)

	var ret0 *models.ListResponse[models.Balance]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Balance]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListBalances indicates an expected call of ListBalances.
func (mr *MockBalancesServiceMockRecorder) ListBalances(ctx, orgID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalances", reflect.TypeOf((*MockBalancesService)(nil).ListBalances), ctx, orgID, ledgerID, opts)
}

// ListAccountBalances mocks base method.
func (m *MockBalancesService) ListAccountBalances(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAccountBalances", ctx, orgID, ledgerID, accountID, opts)

	var ret0 *models.ListResponse[models.Balance]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Balance]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListAccountBalances indicates an expected call of ListAccountBalances.
func (mr *MockBalancesServiceMockRecorder) ListAccountBalances(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAccountBalances", reflect.TypeOf((*MockBalancesService)(nil).ListAccountBalances), ctx, orgID, ledgerID, accountID, opts)
}

// GetBalance mocks base method.
func (m *MockBalancesService) GetBalance(ctx context.Context, orgID, ledgerID, balanceID string) (*models.Balance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetBalance", ctx, orgID, ledgerID, balanceID)

	var ret0 *models.Balance
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Balance) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// GetBalance indicates an expected call of GetBalance.
func (mr *MockBalancesServiceMockRecorder) GetBalance(ctx, orgID, ledgerID, balanceID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetBalance", reflect.TypeOf((*MockBalancesService)(nil).GetBalance), ctx, orgID, ledgerID, balanceID)
}

// UpdateBalance mocks base method.
func (m *MockBalancesService) UpdateBalance(ctx context.Context, orgID, ledgerID, balanceID string, input *models.UpdateBalanceInput) (*models.Balance, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateBalance", ctx, orgID, ledgerID, balanceID, input)

	var ret0 *models.Balance
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Balance) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// UpdateBalance indicates an expected call of UpdateBalance.
func (mr *MockBalancesServiceMockRecorder) UpdateBalance(ctx, orgID, ledgerID, balanceID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateBalance", reflect.TypeOf((*MockBalancesService)(nil).UpdateBalance), ctx, orgID, ledgerID, balanceID, input)
}

// DeleteBalance mocks base method.
func (m *MockBalancesService) DeleteBalance(ctx context.Context, orgID, ledgerID, balanceID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteBalance", ctx, orgID, ledgerID, balanceID)

	var ret0 error
	if ret[0] != nil {
		ret0, _ = ret[0].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// DeleteBalance indicates an expected call of DeleteBalance.
func (mr *MockBalancesServiceMockRecorder) DeleteBalance(ctx, orgID, ledgerID, balanceID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteBalance", reflect.TypeOf((*MockBalancesService)(nil).DeleteBalance), ctx, orgID, ledgerID, balanceID)
}

// ListBalancesAll mocks base method.
func (m *MockBalancesService) ListBalancesAll(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesAll", ctx, orgID, ledgerID, opts)

	var ret0 iter.Seq2[models.Balance, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Balance, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesAll indicates an expected call of ListBalancesAll.
func (mr *MockBalancesServiceMockRecorder) ListBalancesAll(ctx, orgID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesAll", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesAll), ctx, orgID, ledgerID, opts)
}

// ListBalancesPages mocks base method.
func (m *MockBalancesService) ListBalancesPages(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesPages", ctx, orgID, ledgerID, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Balance], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Balance], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesPages indicates an expected call of ListBalancesPages.
func (mr *MockBalancesServiceMockRecorder) ListBalancesPages(ctx, orgID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesPages", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesPages), ctx, orgID, ledgerID, opts)
}

// ListAccountBalancesAll mocks base method.
func (m *MockBalancesService) ListAccountBalancesAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAccountBalancesAll", ctx, orgID, ledgerID, accountID, opts)

	var ret0 iter.Seq2[models.Balance, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Balance, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListAccountBalancesAll indicates an expected call of ListAccountBalancesAll.
func (mr *MockBalancesServiceMockRecorder) ListAccountBalancesAll(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAccountBalancesAll", reflect.TypeOf((*MockBalancesService)(nil).ListAccountBalancesAll), ctx, orgID, ledgerID, accountID, opts)
}

// ListAccountBalancesPages mocks base method.
func (m *MockBalancesService) ListAccountBalancesPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAccountBalancesPages", ctx, orgID, ledgerID, accountID, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Balance], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Balance], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListAccountBalancesPages indicates an expected call of ListAccountBalancesPages.
func (mr *MockBalancesServiceMockRecorder) ListAccountBalancesPages(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAccountBalancesPages", reflect.TypeOf((*MockBalancesService)(nil).ListAccountBalancesPages), ctx, orgID, ledgerID, accountID, opts)
}

// ListBalancesByAccountAlias mocks base method.
func (m *MockBalancesService) ListBalancesByAccountAlias(ctx context.Context, orgID, ledgerID, alias string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByAccountAlias", ctx, orgID, ledgerID, alias, opts)

	var ret0 *models.ListResponse[models.Balance]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Balance]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListBalancesByAccountAlias indicates an expected call of ListBalancesByAccountAlias.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByAccountAlias(ctx, orgID, ledgerID, alias, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByAccountAlias", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByAccountAlias), ctx, orgID, ledgerID, alias, opts)
}

// ListBalancesByAccountAliasAll mocks base method.
func (m *MockBalancesService) ListBalancesByAccountAliasAll(ctx context.Context, orgID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByAccountAliasAll", ctx, orgID, ledgerID, alias, opts)

	var ret0 iter.Seq2[models.Balance, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Balance, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesByAccountAliasAll indicates an expected call of ListBalancesByAccountAliasAll.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByAccountAliasAll(ctx, orgID, ledgerID, alias, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByAccountAliasAll", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByAccountAliasAll), ctx, orgID, ledgerID, alias, opts)
}

// ListBalancesByAccountAliasPages mocks base method.
func (m *MockBalancesService) ListBalancesByAccountAliasPages(ctx context.Context, orgID, ledgerID, alias string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByAccountAliasPages", ctx, orgID, ledgerID, alias, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Balance], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Balance], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesByAccountAliasPages indicates an expected call of ListBalancesByAccountAliasPages.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByAccountAliasPages(ctx, orgID, ledgerID, alias, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByAccountAliasPages", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByAccountAliasPages), ctx, orgID, ledgerID, alias, opts)
}

// ListBalancesByExternalCode mocks base method.
func (m *MockBalancesService) ListBalancesByExternalCode(ctx context.Context, orgID, ledgerID, code string, opts models.BalancesListOpts) (*models.ListResponse[models.Balance], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByExternalCode", ctx, orgID, ledgerID, code, opts)

	var ret0 *models.ListResponse[models.Balance]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Balance]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListBalancesByExternalCode indicates an expected call of ListBalancesByExternalCode.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByExternalCode(ctx, orgID, ledgerID, code, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByExternalCode", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByExternalCode), ctx, orgID, ledgerID, code, opts)
}

// ListBalancesByExternalCodeAll mocks base method.
func (m *MockBalancesService) ListBalancesByExternalCodeAll(ctx context.Context, orgID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByExternalCodeAll", ctx, orgID, ledgerID, code, opts)

	var ret0 iter.Seq2[models.Balance, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Balance, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesByExternalCodeAll indicates an expected call of ListBalancesByExternalCodeAll.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByExternalCodeAll(ctx, orgID, ledgerID, code, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByExternalCodeAll", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByExternalCodeAll), ctx, orgID, ledgerID, code, opts)
}

// ListBalancesByExternalCodePages mocks base method.
func (m *MockBalancesService) ListBalancesByExternalCodePages(ctx context.Context, orgID, ledgerID, code string, opts models.BalancesListOpts) iter.Seq2[*models.ListResponse[models.Balance], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListBalancesByExternalCodePages", ctx, orgID, ledgerID, code, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Balance], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Balance], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListBalancesByExternalCodePages indicates an expected call of ListBalancesByExternalCodePages.
func (mr *MockBalancesServiceMockRecorder) ListBalancesByExternalCodePages(ctx, orgID, ledgerID, code, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListBalancesByExternalCodePages", reflect.TypeOf((*MockBalancesService)(nil).ListBalancesByExternalCodePages), ctx, orgID, ledgerID, code, opts)
}
