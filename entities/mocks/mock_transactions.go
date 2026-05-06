package mocks

import (
	"context"
	"iter"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/golang/mock/gomock"
)

// MockTransactionsService is a mock of TransactionsService interface.
type MockTransactionsService struct {
	ctrl     *gomock.Controller
	recorder *MockTransactionsServiceMockRecorder
}

// MockTransactionsServiceMockRecorder is the mock recorder for MockTransactionsService.
type MockTransactionsServiceMockRecorder struct {
	mock *MockTransactionsService
}

// NewMockTransactionsService creates a new mock instance.
func NewMockTransactionsService(ctrl *gomock.Controller) *MockTransactionsService {
	mock := &MockTransactionsService{ctrl: ctrl}
	mock.recorder = &MockTransactionsServiceMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTransactionsService) EXPECT() *MockTransactionsServiceMockRecorder {
	return m.recorder
}

// CreateTransaction mocks base method.
func (m *MockTransactionsService) CreateTransaction(ctx context.Context, organizationID, ledgerID string, input *models.CreateTransactionInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTransaction", ctx, organizationID, ledgerID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateTransaction indicates an expected call of CreateTransaction.
func (mr *MockTransactionsServiceMockRecorder) CreateTransaction(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CreateTransaction), ctx, organizationID, ledgerID, input)
}

// CreateTransactionWithDSL mocks base method.
func (m *MockTransactionsService) CreateTransactionWithDSL(ctx context.Context, organizationID, ledgerID string, input *models.TransactionDSLInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTransactionWithDSL", ctx, organizationID, ledgerID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateTransactionWithDSL indicates an expected call of CreateTransactionWithDSL.
func (mr *MockTransactionsServiceMockRecorder) CreateTransactionWithDSL(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTransactionWithDSL", reflect.TypeOf((*MockTransactionsService)(nil).CreateTransactionWithDSL), ctx, organizationID, ledgerID, input)
}

// CreateTransactionWithDSLFile mocks base method.
func (m *MockTransactionsService) CreateTransactionWithDSLFile(ctx context.Context, organizationID, ledgerID string, dslContent []byte) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTransactionWithDSLFile", ctx, organizationID, ledgerID, dslContent)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateTransactionWithDSLFile indicates an expected call of CreateTransactionWithDSLFile.
func (mr *MockTransactionsServiceMockRecorder) CreateTransactionWithDSLFile(ctx, organizationID, ledgerID, dslContent any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTransactionWithDSLFile", reflect.TypeOf((*MockTransactionsService)(nil).CreateTransactionWithDSLFile), ctx, organizationID, ledgerID, dslContent)
}

// GetTransaction mocks base method.
func (m *MockTransactionsService) GetTransaction(ctx context.Context, organizationID, ledgerID, transactionID string) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTransaction", ctx, organizationID, ledgerID, transactionID)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// GetTransaction indicates an expected call of GetTransaction.
func (mr *MockTransactionsServiceMockRecorder) GetTransaction(ctx, organizationID, ledgerID, transactionID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTransaction", reflect.TypeOf((*MockTransactionsService)(nil).GetTransaction), ctx, organizationID, ledgerID, transactionID)
}

// ListTransactions mocks base method.
func (m *MockTransactionsService) ListTransactions(ctx context.Context, organizationID, ledgerID string, opts models.TransactionsListOpts) (*models.ListResponse[models.Transaction], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTransactions", ctx, organizationID, ledgerID, opts)

	var ret0 *models.ListResponse[models.Transaction]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Transaction]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListTransactions indicates an expected call of ListTransactions.
func (mr *MockTransactionsServiceMockRecorder) ListTransactions(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTransactions", reflect.TypeOf((*MockTransactionsService)(nil).ListTransactions), ctx, organizationID, ledgerID, opts)
}

// UpdateTransaction mocks base method.
func (m *MockTransactionsService) UpdateTransaction(ctx context.Context, organizationID, ledgerID, transactionID string, input *models.UpdateTransactionInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTransaction", ctx, organizationID, ledgerID, transactionID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// UpdateTransaction indicates an expected call of UpdateTransaction.
func (mr *MockTransactionsServiceMockRecorder) UpdateTransaction(ctx, organizationID, ledgerID, transactionID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTransaction", reflect.TypeOf((*MockTransactionsService)(nil).UpdateTransaction), ctx, organizationID, ledgerID, transactionID, input)
}

// RevertTransaction mocks base method.
func (m *MockTransactionsService) RevertTransaction(ctx context.Context, organizationID, ledgerID, transactionID string) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RevertTransaction", ctx, organizationID, ledgerID, transactionID)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// RevertTransaction indicates an expected call of RevertTransaction.
func (mr *MockTransactionsServiceMockRecorder) RevertTransaction(ctx, organizationID, ledgerID, transactionID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RevertTransaction", reflect.TypeOf((*MockTransactionsService)(nil).RevertTransaction), ctx, organizationID, ledgerID, transactionID)
}

// CommitTransaction mocks base method.
func (m *MockTransactionsService) CommitTransaction(ctx context.Context, organizationID, ledgerID, transactionID string) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitTransaction", ctx, organizationID, ledgerID, transactionID)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CommitTransaction indicates an expected call of CommitTransaction.
func (mr *MockTransactionsServiceMockRecorder) CommitTransaction(ctx, organizationID, ledgerID, transactionID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CommitTransaction), ctx, organizationID, ledgerID, transactionID)
}

// CancelTransaction mocks base method.
func (m *MockTransactionsService) CancelTransaction(ctx context.Context, organizationID, ledgerID, transactionID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelTransaction", ctx, organizationID, ledgerID, transactionID)

	var ret0 error
	if ret[0] != nil {
		ret0, _ = ret[0].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// CancelTransaction indicates an expected call of CancelTransaction.
func (mr *MockTransactionsServiceMockRecorder) CancelTransaction(ctx, organizationID, ledgerID, transactionID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CancelTransaction), ctx, organizationID, ledgerID, transactionID)
}

// CancelTransactionWithResponse mocks base method.
func (m *MockTransactionsService) CancelTransactionWithResponse(ctx context.Context, organizationID, ledgerID, transactionID string) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelTransactionWithResponse", ctx, organizationID, ledgerID, transactionID)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if len(ret) > 1 && ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CancelTransactionWithResponse indicates an expected call of CancelTransactionWithResponse.
func (mr *MockTransactionsServiceMockRecorder) CancelTransactionWithResponse(ctx, organizationID, ledgerID, transactionID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelTransactionWithResponse", reflect.TypeOf((*MockTransactionsService)(nil).CancelTransactionWithResponse), ctx, organizationID, ledgerID, transactionID)
}

// CreateInflowTransaction mocks base method.
func (m *MockTransactionsService) CreateInflowTransaction(ctx context.Context, organizationID, ledgerID string, input *models.CreateInflowInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateInflowTransaction", ctx, organizationID, ledgerID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateInflowTransaction indicates an expected call of CreateInflowTransaction.
func (mr *MockTransactionsServiceMockRecorder) CreateInflowTransaction(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateInflowTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CreateInflowTransaction), ctx, organizationID, ledgerID, input)
}

// CreateOutflowTransaction mocks base method.
func (m *MockTransactionsService) CreateOutflowTransaction(ctx context.Context, organizationID, ledgerID string, input *models.CreateOutflowInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOutflowTransaction", ctx, organizationID, ledgerID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateOutflowTransaction indicates an expected call of CreateOutflowTransaction.
func (mr *MockTransactionsServiceMockRecorder) CreateOutflowTransaction(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOutflowTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CreateOutflowTransaction), ctx, organizationID, ledgerID, input)
}

// CreateAnnotationTransaction mocks base method.
func (m *MockTransactionsService) CreateAnnotationTransaction(ctx context.Context, organizationID, ledgerID string, input *models.CreateAnnotationInput) (*models.Transaction, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateAnnotationTransaction", ctx, organizationID, ledgerID, input)

	var ret0 *models.Transaction
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Transaction) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateAnnotationTransaction indicates an expected call of CreateAnnotationTransaction.
func (mr *MockTransactionsServiceMockRecorder) CreateAnnotationTransaction(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateAnnotationTransaction", reflect.TypeOf((*MockTransactionsService)(nil).CreateAnnotationTransaction), ctx, organizationID, ledgerID, input)
}

// ListTransactionsAll mocks base method.
func (m *MockTransactionsService) ListTransactionsAll(ctx context.Context, organizationID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[models.Transaction, error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTransactionsAll", ctx, organizationID, ledgerID, opts)

	var ret0 iter.Seq2[models.Transaction, error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[models.Transaction, error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListTransactionsAll indicates an expected call of ListTransactionsAll.
func (mr *MockTransactionsServiceMockRecorder) ListTransactionsAll(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTransactionsAll", reflect.TypeOf((*MockTransactionsService)(nil).ListTransactionsAll), ctx, organizationID, ledgerID, opts)
}

// ListTransactionsPages mocks base method.
func (m *MockTransactionsService) ListTransactionsPages(ctx context.Context, organizationID, ledgerID string, opts models.TransactionsListOpts) iter.Seq2[*models.ListResponse[models.Transaction], error] {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListTransactionsPages", ctx, organizationID, ledgerID, opts)

	var ret0 iter.Seq2[*models.ListResponse[models.Transaction], error]
	if ret[0] != nil {
		ret0, _ = ret[0].(iter.Seq2[*models.ListResponse[models.Transaction], error]) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// ListTransactionsPages indicates an expected call of ListTransactionsPages.
func (mr *MockTransactionsServiceMockRecorder) ListTransactionsPages(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListTransactionsPages", reflect.TypeOf((*MockTransactionsService)(nil).ListTransactionsPages), ctx, organizationID, ledgerID, opts)
}
