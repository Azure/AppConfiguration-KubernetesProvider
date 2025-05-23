// Code generated by MockGen. DO NOT EDIT.
// Source: azappconfig/provider/internal/loader (interfaces: ConfigurationSettingsRetriever)

// Package mocks is a generated GoMock package.
package mocks

import (
	v1 "azappconfig/provider/api/v1"
	loader "azappconfig/provider/internal/loader"
	context "context"
	reflect "reflect"

	azcore "github.com/Azure/azure-sdk-for-go/sdk/azcore"
	gomock "github.com/golang/mock/gomock"
)

// MockConfigurationSettingsRetriever is a mock of ConfigurationSettingsRetriever interface.
type MockConfigurationSettingsRetriever struct {
	ctrl     *gomock.Controller
	recorder *MockConfigurationSettingsRetrieverMockRecorder
}

// MockConfigurationSettingsRetrieverMockRecorder is the mock recorder for MockConfigurationSettingsRetriever.
type MockConfigurationSettingsRetrieverMockRecorder struct {
	mock *MockConfigurationSettingsRetriever
}

// NewMockConfigurationSettingsRetriever creates a new mock instance.
func NewMockConfigurationSettingsRetriever(ctrl *gomock.Controller) *MockConfigurationSettingsRetriever {
	mock := &MockConfigurationSettingsRetriever{ctrl: ctrl}
	mock.recorder = &MockConfigurationSettingsRetrieverMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfigurationSettingsRetriever) EXPECT() *MockConfigurationSettingsRetrieverMockRecorder {
	return m.recorder
}

// CheckAndRefreshSentinels mocks base method.
func (m *MockConfigurationSettingsRetriever) CheckAndRefreshSentinels(arg0 context.Context, arg1 *v1.AzureAppConfigurationProvider, arg2 map[v1.Sentinel]*azcore.ETag) (bool, map[v1.Sentinel]*azcore.ETag, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckAndRefreshSentinels", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(map[v1.Sentinel]*azcore.ETag)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CheckAndRefreshSentinels indicates an expected call of CheckAndRefreshSentinels.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) CheckAndRefreshSentinels(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckAndRefreshSentinels", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).CheckAndRefreshSentinels), arg0, arg1, arg2)
}

// CheckPageETags mocks base method.
func (m *MockConfigurationSettingsRetriever) CheckPageETags(arg0 context.Context, arg1 map[v1.Selector][]*azcore.ETag) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CheckPageETags", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CheckPageETags indicates an expected call of CheckPageETags.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) CheckPageETags(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CheckPageETags", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).CheckPageETags), arg0, arg1)
}

// CreateTargetSettings mocks base method.
func (m *MockConfigurationSettingsRetriever) CreateTargetSettings(arg0 context.Context, arg1 loader.SecretReferenceResolver) (*loader.TargetKeyValueSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTargetSettings", arg0, arg1)
	ret0, _ := ret[0].(*loader.TargetKeyValueSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateTargetSettings indicates an expected call of CreateTargetSettings.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) CreateTargetSettings(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTargetSettings", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).CreateTargetSettings), arg0, arg1)
}

// RefreshFeatureFlagSettings mocks base method.
func (m *MockConfigurationSettingsRetriever) RefreshFeatureFlagSettings(arg0 context.Context, arg1 *map[string]string) (*loader.TargetKeyValueSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshFeatureFlagSettings", arg0, arg1)
	ret0, _ := ret[0].(*loader.TargetKeyValueSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RefreshFeatureFlagSettings indicates an expected call of RefreshFeatureFlagSettings.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) RefreshFeatureFlagSettings(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshFeatureFlagSettings", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).RefreshFeatureFlagSettings), arg0, arg1)
}

// RefreshKeyValueSettings mocks base method.
func (m *MockConfigurationSettingsRetriever) RefreshKeyValueSettings(arg0 context.Context, arg1 *map[string]string, arg2 loader.SecretReferenceResolver) (*loader.TargetKeyValueSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshKeyValueSettings", arg0, arg1, arg2)
	ret0, _ := ret[0].(*loader.TargetKeyValueSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RefreshKeyValueSettings indicates an expected call of RefreshKeyValueSettings.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) RefreshKeyValueSettings(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshKeyValueSettings", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).RefreshKeyValueSettings), arg0, arg1, arg2)
}

// ResolveSecretReferences mocks base method.
func (m *MockConfigurationSettingsRetriever) ResolveSecretReferences(arg0 context.Context, arg1 map[string]*loader.TargetK8sSecretMetadata, arg2 loader.SecretReferenceResolver) (*loader.TargetKeyValueSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResolveSecretReferences", arg0, arg1, arg2)
	ret0, _ := ret[0].(*loader.TargetKeyValueSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResolveSecretReferences indicates an expected call of ResolveSecretReferences.
func (mr *MockConfigurationSettingsRetrieverMockRecorder) ResolveSecretReferences(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResolveSecretReferences", reflect.TypeOf((*MockConfigurationSettingsRetriever)(nil).ResolveSecretReferences), arg0, arg1, arg2)
}
