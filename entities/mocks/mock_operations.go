package mocks

import (
	"context"
	"iter"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/golang/mock/gomock"
)

// MockOperationsService is a mock of OperationsService interface.
type MockOperationsService struct {
	ctrl     *gomock.Controller
	recorder *MockOperationsServiceMockRecorder
}

// MockOperationsServiceMockRecorder is the mock recorder for MockOperationsService.
type MockOperationsServiceMockRecorder struct {
	mock *MockOperationsService
}

// NewMockOperationsService creates a new mock instance.
func NewMockOperationsService(ctrl *gomock.Controller) *MockOperationsService {
	mock := &MockOperationsService{ctrl: ctrl}
	mock.recorder = &MockOperationsServiceMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOperationsService) EXPECT() *MockOperationsServiceMockRecorder {
	return m.recorder
}

// ListOperations mocks base method.
func (m *MockOperationsService) ListOperations(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) (*models.ListResponse[models.Operation], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListOperations", ctx, orgID, ledgerID, accountID, opts)

	var ret0 *models.ListResponse[models.Operation]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Operation]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListOperations indicates an expected call of ListOperations.
func (mr *MockOperationsServiceMockRecorder) ListOperations(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperations", reflect.TypeOf((*MockOperationsService)(nil).ListOperations), ctx, orgID, ledgerID, accountID, opts)
}

// GetOperation mocks base method.
func (m *MockOperationsService) GetOperation(ctx context.Context, orgID, ledgerID, accountID, operationID string) (*models.Operation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetOperation", ctx, orgID, ledgerID, accountID, operationID)

	var ret0 *models.Operation
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Operation) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// GetOperation indicates an expected call of GetOperation.
func (mr *MockOperationsServiceMockRecorder) GetOperation(ctx, orgID, ledgerID, accountID, operationID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetOperation", reflect.TypeOf((*MockOperationsService)(nil).GetOperation), ctx, orgID, ledgerID, accountID, operationID)
}

// UpdateTransactionOperation mocks base method.
func (m *MockOperationsService) UpdateTransactionOperation(ctx context.Context, orgID, ledgerID, transactionID, operationID string, input *models.UpdateOperationInput) (*models.Operation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTransactionOperation", ctx, orgID, ledgerID, transactionID, operationID, input)

	var ret0 *models.Operation
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Operation) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// UpdateTransactionOperation indicates an expected call of UpdateTransactionOperation.
func (mr *MockOperationsServiceMockRecorder) UpdateTransactionOperation(ctx, orgID, ledgerID, transactionID, operationID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTransactionOperation", reflect.TypeOf((*MockOperationsService)(nil).UpdateTransactionOperation), ctx, orgID, ledgerID, transactionID, operationID, input)
}

// ListOperationsAll mocks base method.
func (m *MockOperationsService) ListOperationsAll(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[models.Operation, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListOperationsAll", ctx, orgID, ledgerID, accountID, opts)

	var ret0 iter.Seq2[models.Operation, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Operation, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListOperationsAll indicates an expected call of ListOperationsAll.
func (mr *MockOperationsServiceMockRecorder) ListOperationsAll(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperationsAll", reflect.TypeOf((*MockOperationsService)(nil).ListOperationsAll), ctx, orgID, ledgerID, accountID, opts)
}

// ListOperationsPages mocks base method.
func (m *MockOperationsService) ListOperationsPages(ctx context.Context, orgID, ledgerID, accountID string, opts models.OperationsListOpts) iter.Seq2[*models.ListResponse[models.Operation], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListOperationsPages", ctx, orgID, ledgerID, accountID, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Operation], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Operation], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListOperationsPages indicates an expected call of ListOperationsPages.
func (mr *MockOperationsServiceMockRecorder) ListOperationsPages(ctx, orgID, ledgerID, accountID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListOperationsPages", reflect.TypeOf((*MockOperationsService)(nil).ListOperationsPages), ctx, orgID, ledgerID, accountID, opts)
}
