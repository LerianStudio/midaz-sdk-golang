package mocks

import (
	"context"
	"reflect"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/golang/mock/gomock"
)

// MockAssetsService is a mock of AssetsService interface.
type MockAssetsService struct {
	ctrl     *gomock.Controller
	recorder *MockAssetsServiceMockRecorder
}

// MockAssetsServiceMockRecorder is the mock recorder for MockAssetsService.
type MockAssetsServiceMockRecorder struct {
	mock *MockAssetsService
}

// NewMockAssetsService creates a new mock instance.
func NewMockAssetsService(ctrl *gomock.Controller) *MockAssetsService {
	mock := &MockAssetsService{ctrl: ctrl}

	mock.recorder = &MockAssetsServiceMockRecorder{mock}

	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAssetsService) EXPECT() *MockAssetsServiceMockRecorder {
	return m.recorder
}

// ListAssets mocks base method.
func (m *MockAssetsService) ListAssets(ctx context.Context, organizationID, ledgerID string, opts *models.ListOptions) (*models.ListResponse[models.Asset], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListAssets", ctx, organizationID, ledgerID, opts)

	var ret0 *models.ListResponse[models.Asset]
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.ListResponse[models.Asset]) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// ListAssets indicates an expected call of ListAssets.
func (mr *MockAssetsServiceMockRecorder) ListAssets(ctx, organizationID, ledgerID, opts any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListAssets", reflect.TypeOf((*MockAssetsService)(nil).ListAssets), ctx, organizationID, ledgerID, opts)
}

// GetAsset mocks base method.
func (m *MockAssetsService) GetAsset(ctx context.Context, organizationID, ledgerID, id string) (*models.Asset, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAsset", ctx, organizationID, ledgerID, id)

	var ret0 *models.Asset
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Asset) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// GetAsset indicates an expected call of GetAsset.
func (mr *MockAssetsServiceMockRecorder) GetAsset(ctx, organizationID, ledgerID, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAsset", reflect.TypeOf((*MockAssetsService)(nil).GetAsset), ctx, organizationID, ledgerID, id)
}

// CreateAsset mocks base method.
func (m *MockAssetsService) CreateAsset(ctx context.Context, organizationID, ledgerID string, input *models.CreateAssetInput) (*models.Asset, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateAsset", ctx, organizationID, ledgerID, input)

	var ret0 *models.Asset
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Asset) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// CreateAsset indicates an expected call of CreateAsset.
func (mr *MockAssetsServiceMockRecorder) CreateAsset(ctx, organizationID, ledgerID, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateAsset", reflect.TypeOf((*MockAssetsService)(nil).CreateAsset), ctx, organizationID, ledgerID, input)
}

// UpdateAsset mocks base method.
func (m *MockAssetsService) UpdateAsset(ctx context.Context, organizationID, ledgerID, id string, input *models.UpdateAssetInput) (*models.Asset, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateAsset", ctx, organizationID, ledgerID, id, input)

	var ret0 *models.Asset
	if ret[0] != nil {
		ret0, _ = ret[0].(*models.Asset) //nolint:errcheck // Type guaranteed by mock setup
	}

	var ret1 error
	if ret[1] != nil {
		ret1, _ = ret[1].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0, ret1
}

// UpdateAsset indicates an expected call of UpdateAsset.
func (mr *MockAssetsServiceMockRecorder) UpdateAsset(ctx, organizationID, ledgerID, id, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateAsset", reflect.TypeOf((*MockAssetsService)(nil).UpdateAsset), ctx, organizationID, ledgerID, id, input)
}

// DeleteAsset mocks base method.
func (m *MockAssetsService) DeleteAsset(ctx context.Context, organizationID, ledgerID, id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAsset", ctx, organizationID, ledgerID, id)

	var ret0 error
	if ret[0] != nil {
		ret0, _ = ret[0].(error) //nolint:errcheck // Type guaranteed by mock setup
	}

	return ret0
}

// DeleteAsset indicates an expected call of DeleteAsset.
func (mr *MockAssetsServiceMockRecorder) DeleteAsset(ctx, organizationID, ledgerID, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAsset", reflect.TypeOf((*MockAssetsService)(nil).DeleteAsset), ctx, organizationID, ledgerID, id)
}
