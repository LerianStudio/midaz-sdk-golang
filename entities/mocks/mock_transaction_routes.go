package mocks

import (
	"context"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/golang/mock/gomock"
)

// MockTransactionRoutesService is a mock of TransactionRoutesService interface.
type MockTransactionRoutesService struct {
	ctrl     *gomock.Controller
	recorder *MockTransactionRoutesServiceMockRecorder
}

// MockTransactionRoutesServiceMockRecorder is the mock recorder for MockTransactionRoutesService.
type MockTransactionRoutesServiceMockRecorder struct {
	mock *MockTransactionRoutesService
}

// NewMockTransactionRoutesService creates a new mock instance.
func NewMockTransactionRoutesService(ctrl *gomock.Controller) *MockTransactionRoutesService {
	mock := &MockTransactionRoutesService{ctrl: ctrl}
	mock.recorder = &MockTransactionRoutesServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTransactionRoutesService) EXPECT() *MockTransactionRoutesServiceMockRecorder {
	return m.recorder
}

// CreateTransactionRoute mocks base method.
func (m *MockTransactionRoutesService) CreateTransactionRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionRouteInput) (*models.TransactionRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTransactionRoute", ctx, organizationID, ledgerID, input)
	ret0, _ := ret[0].(*models.TransactionRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTransactionRoute indicates an expected call of CreateTransactionRoute.
func (mr *MockTransactionRoutesServiceMockRecorder) CreateTransactionRoute(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTransactionRoute", reflect.TypeOf((*MockTransactionRoutesService)(nil).CreateTransactionRoute), ctx, organizationID, ledgerID, input)
}

// DeleteTransactionRoute mocks base method.
func (m *MockTransactionRoutesService) DeleteTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteTransactionRoute", ctx, organizationID, ledgerID, transactionRouteID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteTransactionRoute indicates an expected call of DeleteTransactionRoute.
func (mr *MockTransactionRoutesServiceMockRecorder) DeleteTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteTransactionRoute", reflect.TypeOf((*MockTransactionRoutesService)(nil).DeleteTransactionRoute), ctx, organizationID, ledgerID, transactionRouteID)
}

// GetTransactionRoute mocks base method.
func (m *MockTransactionRoutesService) GetTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string) (*models.TransactionRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTransactionRoute", ctx, organizationID, ledgerID, transactionRouteID)
	ret0, _ := ret[0].(*models.TransactionRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetTransactionRoute indicates an expected call of GetTransactionRoute.
func (mr *MockTransactionRoutesServiceMockRecorder) GetTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransactionRoute", reflect.TypeOf((*MockTransactionRoutesService)(nil).GetTransactionRoute), ctx, organizationID, ledgerID, transactionRouteID)
}

// ListTransactionRoutes mocks base method.
func (m *MockTransactionRoutesService) ListTransactionRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.TransactionRoute], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTransactionRoutes", ctx, organizationID, ledgerID, opts)
	ret0, _ := ret[0].(*models.ListResponse[models.TransactionRoute])
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListTransactionRoutes indicates an expected call of ListTransactionRoutes.
func (mr *MockTransactionRoutesServiceMockRecorder) ListTransactionRoutes(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTransactionRoutes", reflect.TypeOf((*MockTransactionRoutesService)(nil).ListTransactionRoutes), ctx, organizationID, ledgerID, opts)
}

// UpdateTransactionRoute mocks base method.
func (m *MockTransactionRoutesService) UpdateTransactionRoute(ctx context.Context, organizationID, ledgerID, transactionRouteID string, input *models.UpdateTransactionRouteInput) (*models.TransactionRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTransactionRoute", ctx, organizationID, ledgerID, transactionRouteID, input)
	ret0, _ := ret[0].(*models.TransactionRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateTransactionRoute indicates an expected call of UpdateTransactionRoute.
func (mr *MockTransactionRoutesServiceMockRecorder) UpdateTransactionRoute(ctx, organizationID, ledgerID, transactionRouteID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTransactionRoute", reflect.TypeOf((*MockTransactionRoutesService)(nil).UpdateTransactionRoute), ctx, organizationID, ledgerID, transactionRouteID, input)
}
