package mocks

import (
	"context"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/golang/mock/gomock"
)

// MockOperationRoutesService is a mock of OperationRoutesService interface.
type MockOperationRoutesService struct {
	ctrl     *gomock.Controller
	recorder *MockOperationRoutesServiceMockRecorder
}

// MockOperationRoutesServiceMockRecorder is the mock recorder for MockOperationRoutesService.
type MockOperationRoutesServiceMockRecorder struct {
	mock *MockOperationRoutesService
}

// NewMockOperationRoutesService creates a new mock instance.
func NewMockOperationRoutesService(ctrl *gomock.Controller) *MockOperationRoutesService {
	mock := &MockOperationRoutesService{ctrl: ctrl}
	mock.recorder = &MockOperationRoutesServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOperationRoutesService) EXPECT() *MockOperationRoutesServiceMockRecorder {
	return m.recorder
}

// CreateOperationRoute mocks base method.
func (m *MockOperationRoutesService) CreateOperationRoute(ctx context.Context, organizationID, ledgerID string, input *models.CreateOperationRouteInput) (*models.OperationRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOperationRoute", ctx, organizationID, ledgerID, input)
	ret0, _ := ret[0].(*models.OperationRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOperationRoute indicates an expected call of CreateOperationRoute.
func (mr *MockOperationRoutesServiceMockRecorder) CreateOperationRoute(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOperationRoute", reflect.TypeOf((*MockOperationRoutesService)(nil).CreateOperationRoute), ctx, organizationID, ledgerID, input)
}

// DeleteOperationRoute mocks base method.
func (m *MockOperationRoutesService) DeleteOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteOperationRoute", ctx, organizationID, ledgerID, operationRouteID)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteOperationRoute indicates an expected call of DeleteOperationRoute.
func (mr *MockOperationRoutesServiceMockRecorder) DeleteOperationRoute(ctx, organizationID, ledgerID, operationRouteID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteOperationRoute", reflect.TypeOf((*MockOperationRoutesService)(nil).DeleteOperationRoute), ctx, organizationID, ledgerID, operationRouteID)
}

// GetOperationRoute mocks base method.
func (m *MockOperationRoutesService) GetOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string) (*models.OperationRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOperationRoute", ctx, organizationID, ledgerID, operationRouteID)
	ret0, _ := ret[0].(*models.OperationRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetOperationRoute indicates an expected call of GetOperationRoute.
func (mr *MockOperationRoutesServiceMockRecorder) GetOperationRoute(ctx, organizationID, ledgerID, operationRouteID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOperationRoute", reflect.TypeOf((*MockOperationRoutesService)(nil).GetOperationRoute), ctx, organizationID, ledgerID, operationRouteID)
}

// ListOperationRoutes mocks base method.
func (m *MockOperationRoutesService) ListOperationRoutes(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.OperationRoute], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListOperationRoutes", ctx, organizationID, ledgerID, opts)
	ret0, _ := ret[0].(*models.ListResponse[models.OperationRoute])
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListOperationRoutes indicates an expected call of ListOperationRoutes.
func (mr *MockOperationRoutesServiceMockRecorder) ListOperationRoutes(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperationRoutes", reflect.TypeOf((*MockOperationRoutesService)(nil).ListOperationRoutes), ctx, organizationID, ledgerID, opts)
}

// UpdateOperationRoute mocks base method.
func (m *MockOperationRoutesService) UpdateOperationRoute(ctx context.Context, organizationID, ledgerID, operationRouteID string, input *models.UpdateOperationRouteInput) (*models.OperationRoute, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateOperationRoute", ctx, organizationID, ledgerID, operationRouteID, input)
	ret0, _ := ret[0].(*models.OperationRoute)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateOperationRoute indicates an expected call of UpdateOperationRoute.
func (mr *MockOperationRoutesServiceMockRecorder) UpdateOperationRoute(ctx, organizationID, ledgerID, operationRouteID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateOperationRoute", reflect.TypeOf((*MockOperationRoutesService)(nil).UpdateOperationRoute), ctx, organizationID, ledgerID, operationRouteID, input)
}
