package mocks

import (
	"context"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/golang/mock/gomock"
)

// MockAccountTypesService is a mock of AccountTypesService interface.
type MockAccountTypesService struct {
	ctrl     *gomock.Controller
	recorder *MockAccountTypesServiceMockRecorder
}

// MockAccountTypesServiceMockRecorder is the mock recorder for MockAccountTypesService.
type MockAccountTypesServiceMockRecorder struct {
	mock *MockAccountTypesService
}

// NewMockAccountTypesService creates a new mock instance.
func NewMockAccountTypesService(ctrl *gomock.Controller) *MockAccountTypesService {
	mock := &MockAccountTypesService{ctrl: ctrl}

	mock.recorder = &MockAccountTypesServiceMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAccountTypesService) EXPECT() *MockAccountTypesServiceMockRecorder {
	return m.recorder
}

// ListAccountTypes mocks base method.
func (m *MockAccountTypesService) ListAccountTypes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.AccountType], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAccountTypes", ctx, organizationID, ledgerID, opts)
	ret0, _ := ret[0].(*models.ListResponse[models.AccountType])
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// ListAccountTypes indicates an expected call of ListAccountTypes.
func (mr *MockAccountTypesServiceMockRecorder) ListAccountTypes(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAccountTypes", reflect.TypeOf((*MockAccountTypesService)(nil).ListAccountTypes), ctx, organizationID, ledgerID, opts)
}

// GetAccountType mocks base method.
func (m *MockAccountTypesService) GetAccountType(ctx context.Context, organizationID, ledgerID, id string) (*models.AccountType, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccountType", ctx, organizationID, ledgerID, id)
	ret0, _ := ret[0].(*models.AccountType)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// GetAccountType indicates an expected call of GetAccountType.
func (mr *MockAccountTypesServiceMockRecorder) GetAccountType(ctx, organizationID, ledgerID, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccountType", reflect.TypeOf((*MockAccountTypesService)(nil).GetAccountType), ctx, organizationID, ledgerID, id)
}

// CreateAccountType mocks base method.
func (m *MockAccountTypesService) CreateAccountType(ctx context.Context, organizationID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateAccountType", ctx, organizationID, ledgerID, input)
	ret0, _ := ret[0].(*models.AccountType)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// CreateAccountType indicates an expected call of CreateAccountType.
func (mr *MockAccountTypesServiceMockRecorder) CreateAccountType(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateAccountType", reflect.TypeOf((*MockAccountTypesService)(nil).CreateAccountType), ctx, organizationID, ledgerID, input)
}

// UpdateAccountType mocks base method.
func (m *MockAccountTypesService) UpdateAccountType(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAccountTypeInput) (*models.AccountType, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateAccountType", ctx, organizationID, ledgerID, id, input)
	ret0, _ := ret[0].(*models.AccountType)
	ret1, _ := ret[1].(error)

	return ret0, ret1
}

// UpdateAccountType indicates an expected call of UpdateAccountType.
func (mr *MockAccountTypesServiceMockRecorder) UpdateAccountType(ctx, organizationID, ledgerID, id, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAccountType", reflect.TypeOf((*MockAccountTypesService)(nil).UpdateAccountType), ctx, organizationID, ledgerID, id, input)
}

// DeleteAccountType mocks base method.
func (m *MockAccountTypesService) DeleteAccountType(ctx context.Context, organizationID, ledgerID, id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAccountType", ctx, organizationID, ledgerID, id)
	ret0, _ := ret[0].(error)

	return ret0
}

// DeleteAccountType indicates an expected call of DeleteAccountType.
func (mr *MockAccountTypesServiceMockRecorder) DeleteAccountType(ctx, organizationID, ledgerID, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()

	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAccountType", reflect.TypeOf((*MockAccountTypesService)(nil).DeleteAccountType), ctx, organizationID, ledgerID, id)
}
