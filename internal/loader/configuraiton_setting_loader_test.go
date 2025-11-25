// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azappconfig "github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkcs12 "software.sslmate.com/src/go-pkcs12"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var (
	mockResolveSecretReference    *MockResolveSecretReference
	mockSettingsClient            *MockSettingsClient
	mockCtrl                      *gomock.Controller
	mockCongiurationClientManager *MockClientManager
	endpointName                  string = "https://fake-endpoint"
	fakeClientWrapper                    = ConfigurationClientWrapper{
		Client:         nil,
		Endpoint:       endpointName,
		BackOffEndTime: metav1.Time{},
		FailedAttempts: 0,
	}
)

func mockConfigurationSettings() []azappconfig.Setting {
	settingsToReturn := make([]azappconfig.Setting, 6)
	settingsToReturn[0] = newCommonKeyValueSettings("someKey1", "value1", "label1")
	settingsToReturn[1] = newCommonKeyValueSettings("app:", "value2", "label1")
	settingsToReturn[2] = newCommonKeyValueSettings("test:", "value3", "label1")
	settingsToReturn[3] = newCommonKeyValueSettings("app:someSubKey1:1", "value4", "label1")
	settingsToReturn[4] = newCommonKeyValueSettings("app:test:some", "value5", "label1")
	settingsToReturn[5] = newCommonKeyValueSettings("app:test:", "value6", "label1")

	return settingsToReturn
}

func mockConfigurationSettingsWithKV() []azappconfig.Setting {
	settingsToReturn := make([]azappconfig.Setting, 3)
	settingsToReturn[0] = newCommonKeyValueSettings("someKey1", "value1", "label1")
	settingsToReturn[1] = newCommonKeyValueSettings("app:someSubKey1:1", "value4", "label1")
	settingsToReturn[2] = newKeyVaultSettings("app:secret:1", "label1")

	return settingsToReturn
}

func mockFeatureFlagSettings() []azappconfig.Setting {
	settingsToReturn := make([]azappconfig.Setting, 2)
	settingsToReturn[0] = newFeatureFlagSettings(".appconfig.featureflag/Beta", "label1")
	settingsToReturn[1] = newFeatureFlagSettings(".appconfig.featureflag/Beta", "label1")

	return settingsToReturn
}

func mockVariantFeatureFlagSettings(key, label string, telemetryEnabled bool) []azappconfig.Setting {
	settingsToReturn := make([]azappconfig.Setting, 1)
	settingsToReturn[0] = newFeatureFlagVariant(key, label, telemetryEnabled)

	return settingsToReturn
}

func newKeyValueSelector(key string, label *string) acpv1.ComparableSelector {
	return acpv1.ComparableSelector{
		KeyFilter:   &key,
		LabelFilter: label,
	}
}

func newFeatureFlagSelector(key string, label *string) acpv1.ComparableSelector {
	prefixedKey := FeatureFlagKeyPrefix + key
	return acpv1.ComparableSelector{
		KeyFilter:   &prefixedKey,
		LabelFilter: label,
	}
}

func newCommonKeyValueSettings(key string, value string, label string) azappconfig.Setting {
	return azappconfig.Setting{
		Key:   &key,
		Value: &value,
		Label: &label,
	}
}

func newKeyVaultSettings(key string, label string) azappconfig.Setting {
	vault := "{ \"uri\":\"https://fake-vault/secrets/fakesecret\"}"
	keyVaultContentType := SecretReferenceContentType

	return azappconfig.Setting{
		Key:         &key,
		Value:       &vault,
		Label:       &label,
		ContentType: &keyVaultContentType,
	}
}

func newFeatureFlagSettings(key string, label string) azappconfig.Setting {
	featureFlagContentType := FeatureFlagContentType
	featureFlagId := strings.TrimPrefix(key, ".appconfig.featureflag/")
	featureFlagValue := fmt.Sprintf(`{
		"id": "%s",
		"description": "",
		"enabled": false,
		"conditions": {
			"client_filters": []
		}
	}`, featureFlagId)

	return azappconfig.Setting{
		Key:         &key,
		Value:       &featureFlagValue,
		Label:       &label,
		ContentType: &featureFlagContentType,
	}
}

func newFeatureFlagVariant(key string, label string, telemetryEnabled bool) azappconfig.Setting {
	featureFlagContentType := FeatureFlagContentType
	fakeETag := azcore.ETag("fakeETag")
	featureFlagId := strings.TrimPrefix(key, ".appconfig.featureflag/")
	featureFlagValue := fmt.Sprintf(`{
		"id": "%s",
		"description": "",
		"enabled": false,
		"variants": [
			{
				"name": "Off",
				"configuration_value": false
			},
			{
				"name": "On",
				"configuration_value": true
			}
		],
		"allocation": {
			"percentile": [
				{
					"variant": "Off",
					"from": 0,
					"to": 100
				}
			],
			"default_when_enabled": "Off",
			"default_when_disabled": "Off"
		},
		"telemetry": {
			"enabled": %t
		}
	}`, featureFlagId, telemetryEnabled)

	return azappconfig.Setting{
		Key:         &key,
		Value:       &featureFlagValue,
		Label:       &label,
		ContentType: &featureFlagContentType,
		ETag:        &fakeETag,
	}
}

type MockResolveSecretReference struct {
	ctrl     *gomock.Controller
	recorder *MockResolveSecretReferenceMockRecorder
}

// MockResolveSecretReferenceMockRecorder is the mock recorder for MockResolveSecretReference.
type MockResolveSecretReferenceMockRecorder struct {
	mock *MockResolveSecretReference
}

// NewMockResolveSecretReference creates a new mock instance.
func NewMockResolveSecretReference(ctrl *gomock.Controller) *MockResolveSecretReference {
	mock := &MockResolveSecretReference{ctrl: ctrl}
	mock.recorder = &MockResolveSecretReferenceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResolveSecretReference) EXPECT() *MockResolveSecretReferenceMockRecorder {
	return m.recorder
}

// Resolve mocks base method.
func (m *MockResolveSecretReference) Resolve(arg0 KeyVaultSecretMetadata, arg1 context.Context) (azsecrets.GetSecretResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resolve", arg0, arg1)
	ret0, _ := ret[0].(azsecrets.GetSecretResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Resolve indicates an expected call of Resolve.
func (mr *MockResolveSecretReferenceMockRecorder) Resolve(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resolve", reflect.TypeOf((*MockResolveSecretReference)(nil).Resolve), arg0, arg1)
}

// MockClientManager is a mock of ClientManager interface.
type MockClientManager struct {
	ctrl     *gomock.Controller
	recorder *MockClientManagerMockRecorder
}

// MockClientManagerMockRecorder is the mock recorder for MockClientManager.
type MockClientManagerMockRecorder struct {
	mock *MockClientManager
}

// NewMockClientManager creates a new mock instance.
func NewMockClientManager(ctrl *gomock.Controller) *MockClientManager {
	mock := &MockClientManager{ctrl: ctrl}
	mock.recorder = &MockClientManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientManager) EXPECT() *MockClientManagerMockRecorder {
	return m.recorder
}

// GetClients mocks base method.
func (m *MockClientManager) GetClients(arg0 context.Context) ([]*ConfigurationClientWrapper, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetClients", arg0)
	ret0, _ := ret[0].([]*ConfigurationClientWrapper)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetClients indicates an expected call of GetClients.
func (mr *MockClientManagerMockRecorder) GetClients(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetClients", reflect.TypeOf((*MockClientManager)(nil).GetClients), arg0)
}

// RefreshClients mocks base method.
func (m *MockClientManager) RefreshClients(arg0 context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RefreshClients", arg0)
}

// RefreshClients indicates an expected call of RefreshClients.
func (mr *MockClientManagerMockRecorder) RefreshClients(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshClients", reflect.TypeOf((*MockClientManager)(nil).RefreshClients), arg0)
}

// MockSettingsClient is a mock of SettingsClient interface.
type MockSettingsClient struct {
	ctrl     *gomock.Controller
	recorder *MockSettingsClientMockRecorder
}

// MockSettingsClientMockRecorder is the mock recorder for MockSettingsClient.
type MockSettingsClientMockRecorder struct {
	mock *MockSettingsClient
}

// NewMockSettingsClient creates a new mock instance.
func NewMockSettingsClient(ctrl *gomock.Controller) *MockSettingsClient {
	mock := &MockSettingsClient{ctrl: ctrl}
	mock.recorder = &MockSettingsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSettingsClient) EXPECT() *MockSettingsClientMockRecorder {
	return m.recorder
}

// GetSettings mocks base method.
func (m *MockSettingsClient) GetSettings(arg0 context.Context, arg1 *azappconfig.Client) (*SettingsResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetSettings", arg0, arg1)
	ret0, _ := ret[0].(*SettingsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetSettings indicates an expected call of GetSettings.
func (mr *MockSettingsClientMockRecorder) GetSettings(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetSettings", reflect.TypeOf((*MockSettingsClient)(nil).GetSettings), arg0, arg1)
}

const (
	ProviderName      = "test-appconfigurationprovider"
	ProviderNamespace = "default"
	ConfigMapName     = "configmap-to-be-created"
)

func TestLoaderAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Test loader APIs")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	mockResolveSecretReference = NewMockResolveSecretReference(mockCtrl)
	mockCongiurationClientManager = NewMockClientManager(mockCtrl)
	mockSettingsClient = NewMockSettingsClient(mockCtrl)

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterEach(func() {
	By("tearing down the test environment")
	mockCtrl.Finish()
})

var _ = Describe("AppConfiguationProvider Get All Settings", func() {
	var (
		EndpointName = "https://fake-endpoint"
	)

	Context("When get Key Vault Reference Type Settings", func() {
		It("Should put into Secret settings collection", func() {
			By("By resolving the settings from Azure Key Vault")
			managedIdentity := uuid.New().String()
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: "targetSecret",
					},
					Auth: &acpv1.AzureKeyVaultAuth{
						AzureAppConfigurationProviderAuth: &acpv1.AzureAppConfigurationProviderAuth{
							ManagedIdentityClientId: &managedIdentity,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value: &secretValue,
				},
			}

			settingsToReturn := mockConfigurationSettingsWithKV()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(2))
			Expect(len(allSettings.SecretSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1:1"]).Should(Equal("value4"))
			Expect(string(allSettings.SecretSettings["targetSecret"].Data["secret:1"])).Should(Equal(secretValue))
		})

		It("Should put into Secret settings collection", func() {
			By("Following the target secret data type after resolving the settings from Azure Key Vault")
			managedIdentity := uuid.New().String()
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: "targetSecret",
						SecretData: &acpv1.DataOptions{
							Type: acpv1.Json,
							Key:  "settings.json",
						},
					},
					Auth: &acpv1.AzureKeyVaultAuth{
						AzureAppConfigurationProviderAuth: &acpv1.AzureAppConfigurationProviderAuth{
							ManagedIdentityClientId: &managedIdentity,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value: &secretValue,
				},
			}

			settingsToReturn := mockConfigurationSettingsWithKV()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(2))
			Expect(len(allSettings.SecretSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1:1"]).Should(Equal("value4"))
			Expect(string(allSettings.SecretSettings["targetSecret"].Data["settings.json"])).Should(Equal("{\"secret:1\":\"fakeSecretValue\"}"))
		})

		It("Should put into Secret settings collection", func() {
			By("Following the target secret data type with separator")
			managedIdentity := uuid.New().String()
			separator := ":"
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: "targetSecret",
						SecretData: &acpv1.DataOptions{
							Type:      acpv1.Json,
							Separator: &separator,
							Key:       "settings.json",
						},
					},
					Auth: &acpv1.AzureKeyVaultAuth{
						AzureAppConfigurationProviderAuth: &acpv1.AzureAppConfigurationProviderAuth{
							ManagedIdentityClientId: &managedIdentity,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value: &secretValue,
				},
			}

			settingsToReturn := mockConfigurationSettingsWithKV()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(2))
			Expect(len(allSettings.SecretSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1:1"]).Should(Equal("value4"))
			Expect(string(allSettings.SecretSettings["targetSecret"].Data["settings.json"])).Should(Equal("{\"secret\":{\"1\":\"fakeSecretValue\"}}"))
		})

		It("Should throw exception", func() {
			By("By resolving Key Vault reference to fail")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			settingsToReturn := mockConfigurationSettingsWithKV()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(allSettings).Should(BeNil())
			Expect(err.Error()).Should(Equal("a Key Vault reference is found in App Configuration, but 'spec.secret' was not configured in the Azure App Configuration provider 'testName' in namespace 'testNamespace'"))
		})

		It("Should throw unknown content type error", func() {
			By("By getting unknown cert type from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secretName := "targetSecret"
			contentType := "fake-content-type"
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}
			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			_, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)
			Expect(err.Error()).Should(Equal("fail to decode the cert 'targetSecret': unknown content type 'fake-content-type'"))
		})

		It("Should throw unknown content type error", func() {
			By("By getting unknown cert type from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint: &EndpointName,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secretName := "targetSecret"
			contentType := "fake-content-type"
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			_, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err.Error()).Should(Equal("fail to decode the cert 'targetSecret': unknown content type 'fake-content-type'"))
		})

		It("Should throw decode pem block error", func() {
			By("By getting unexpected secret value of pem cert from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secretName := "targetSecret"
			contentType := CertTypePem
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			_, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err.Error()).Should(Equal("fail to decode the cert 'targetSecret': failed to decode pem block"))
		})

		It("Should throw decode pfx error", func() {
			By("By getting unexpected secret value of pfx cert from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue := "fakeSecretValue"
			secretName := "targetSecret"
			contentType := CertTypePfx
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			_, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err.Error()).Should(Equal("fail to decode the cert 'targetSecret': illegal base64 data at input byte 12"))
		})

		It("Succeeded to get tls type secret", func() {
			By("By getting valid pfx cert from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue, _ := createFakePfx()
			secretName := "targetSecret"
			contentType := CertTypePfx
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			secrets, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(secrets.SecretSettings)).Should(Equal(1))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.crt"])).Should(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.key"])).Should(ContainSubstring("BEGIN PRIVATE KEY"))
		})

		It("Succeeded to get target tls type secret", func() {
			By("By getting valid pem cert from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue, _ := createFakePem()
			secretName := "targetSecret"
			contentType := CertTypePem
			var kidStr azsecrets.ID = "fakeKid"
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					KID:         &kidStr,
					ContentType: &contentType,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			secrets, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(secrets.SecretSettings)).Should(Equal(1))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.crt"])).Should(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.key"])).Should(ContainSubstring("BEGIN RSA PRIVATE KEY"))
		})

		It("Succeeded to get tls type secret", func() {
			By("By getting valid non cert based secret from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue, _ := createFakePfx()
			secretName := "targetSecret"
			ct := CertTypePfx
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					ContentType: &ct,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			secrets, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(secrets.SecretSettings)).Should(Equal(1))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.crt"])).Should(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.key"])).Should(ContainSubstring("BEGIN PRIVATE KEY"))
		})

		It("Succeeded to get tls type secret", func() {
			By("By getting valid non cert based pem secret from Azure Key Vault")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			secretValue, _ := createFakePem()
			secretName := "targetSecret"
			ct := CertTypePem
			secret1 := azsecrets.GetSecretResponse{
				Secret: azsecrets.Secret{
					Value:       &secretValue,
					ContentType: &ct,
				},
			}

			secretReferencesToResolve := map[string]*TargetK8sSecretMetadata{
				secretName: {
					Type: corev1.SecretTypeTLS,
					SecretsKeyVaultMetadata: map[string]KeyVaultSecretMetadata{
						secretName: {
							HostName:      "fake-vault",
							SecretName:    "fake-secret",
							SecretVersion: "fake-version",
						},
					},
				},
			}

			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Return(secret1, nil)
			secrets, err := configurationProvider.ResolveSecretReferences(context.Background(), secretReferencesToResolve, mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(secrets.SecretSettings)).Should(Equal(1))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.crt"])).Should(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(string(secrets.SecretSettings[secretName].Data["tls.key"])).Should(ContainSubstring("BEGIN RSA PRIVATE KEY"))
		})
	})

	Context("Get settings when autofailover not enabled", func() {
		It("Succeed to get all configuration settings", func() {
			By("By not trimming any key prefixes")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			settingsToReturn := mockConfigurationSettings()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(6))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["app:"]).Should(Equal("value2"))
			Expect(allSettings.ConfigMapSettings["test:"]).Should(Equal("value3"))
			Expect(allSettings.ConfigMapSettings["app:someSubKey1:1"]).Should(Equal("value4"))
			Expect(allSettings.ConfigMapSettings["app:test:some"]).Should(Equal("value5"))
			Expect(allSettings.ConfigMapSettings["app:test:"]).Should(Equal("value6"))
		})

		It("Succeed to get empty secret setting", func() {
			By("By fetching no key vault reference from Azure App Configuration")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Secret: &acpv1.SecretReference{
					Target: acpv1.SecretGenerationParameters{
						SecretName: "targetSecret",
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			settingsToReturn := mockConfigurationSettings()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(6))
			Expect(len(allSettings.SecretSettings)).Should(Equal(1))
			Expect(len(allSettings.SecretSettings["targetSecret"].Data)).Should(Equal(0))
		})

		It("Succeed to get all configuration settings", func() {
			By("By trimming single key prefix")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			settingsToReturn := mockConfigurationSettings()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(4))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1:1"]).Should(Equal("value4"))
			Expect(allSettings.ConfigMapSettings["test:some"]).Should(Equal("value5"))
			Expect(allSettings.ConfigMapSettings["test:"]).Should(Equal("value6"))
		})

		It("Succeed to get all configuration settings", func() {
			By("By trimming multiple key prefixes")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					TrimKeyPrefixes: []string{"app:", "test:"},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			settingsToReturn := mockConfigurationSettings()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(4))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1:1"]).Should(Equal("value4"))
			Expect(allSettings.ConfigMapSettings["test:some"]).Should(Equal("value5"))
			Expect(allSettings.ConfigMapSettings["test:"]).Should(Equal("value6"))
		})

		It("Succeed to get all configuration settings", func() {
			By("By loading and deduplicating feature flags")
			featureFlagKeyFilter := "*"
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
					ConfigMapData: &acpv1.DataOptions{
						Type: acpv1.Json,
						Key:  "settings.json",
					},
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter: &featureFlagKeyFilter,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			featureFlagsToReturn := mockFeatureFlagSettings()
			featureFlagEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			featureFlagEtags[newFeatureFlagSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: featureFlagsToReturn,
				Etags:    featureFlagEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil).Times(2)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil).Times(2)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["settings.json"]).Should(Equal("{\"feature_management\":{\"feature_flags\":[{\"conditions\":{\"client_filters\":[]},\"description\":\"\",\"enabled\":false,\"id\":\"Beta\"}]}}"))
		})

		It("Succeed to get feature flag settings", func() {
			By("By updating telemetry when telemetry is enabled")
			featureFlagKeyFilter := "*"
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
					ConfigMapData: &acpv1.DataOptions{
						Type: acpv1.Json,
						Key:  "settings.json",
					},
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter: &featureFlagKeyFilter,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			featureFlagsToReturn := mockVariantFeatureFlagSettings(".appconfig.featureflag/Telemetry_2", "Test", true)
			featureFlagEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			featureFlagEtags[newFeatureFlagSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: featureFlagsToReturn,
				Etags:    featureFlagEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil).Times(2)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil).Times(2)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["settings.json"]).Should(
				Equal("{\"feature_management\":{\"feature_flags\":[{\"allocation\":{\"default_when_disabled\":\"Off\",\"default_when_enabled\":\"Off\",\"percentile\":[{\"from\":0,\"to\":100,\"variant\":\"Off\"}]},\"description\":\"\",\"enabled\":false,\"id\":\"Telemetry_2\",\"telemetry\":{\"enabled\":true,\"metadata\":{\"ETag\":\"fakeETag\",\"FeatureFlagReference\":\"/kv/.appconfig.featureflag/Telemetry_2?label=Test\"}},\"variants\":[{\"configuration_value\":false,\"name\":\"Off\"},{\"configuration_value\":true,\"name\":\"On\"}]}]}}"))
		})

		It("Succeed to get feature flag settings", func() {
			By("By not populating telemetry when telemetry is disabled")
			featureFlagKeyFilter := "*"
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
					ConfigMapData: &acpv1.DataOptions{
						Type: acpv1.Json,
						Key:  "settings.json",
					},
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter: &featureFlagKeyFilter,
						},
					},
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			featureFlagsToReturn := mockVariantFeatureFlagSettings(".appconfig.featureflag/Telemetry_2", "Test", false)
			featureFlagEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			featureFlagEtags[newFeatureFlagSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: featureFlagsToReturn,
				Etags:    featureFlagEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil).Times(2)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil).Times(2)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(1))
			Expect(allSettings.ConfigMapSettings["settings.json"]).Should(
				Equal("{\"feature_management\":{\"feature_flags\":[{\"allocation\":{\"default_when_disabled\":\"Off\",\"default_when_enabled\":\"Off\",\"percentile\":[{\"from\":0,\"to\":100,\"variant\":\"Off\"}]},\"description\":\"\",\"enabled\":false,\"id\":\"Telemetry_2\",\"telemetry\":{\"enabled\":false},\"variants\":[{\"configuration_value\":false,\"name\":\"Off\"},{\"configuration_value\":true,\"name\":\"On\"}]}]}}"))
		})

		It("Fail to get all configuration settings", func() {
			By("By getting error from Azure App Configuration")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			err := errors.New("fake error")
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(nil, err)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(allSettings).Should(BeNil())
			Expect(err).ShouldNot(BeNil())
		})
	})

	Context("Get settings when autofailover enabled", func() {
		It("Succeed to get settings when origin endpoint not available", func() {
			By("By Discovering Fallback Clients")
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: true,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
			}
			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			netErr := &net.OpError{Err: errors.New("fake network error")}
			settingsToReturn := mockConfigurationSettings()
			failedClient := ConfigurationClientWrapper{
				Client:         nil,
				Endpoint:       endpointName,
				BackOffEndTime: metav1.Time{},
				FailedAttempts: 0,
			}

			succeededClient := ConfigurationClientWrapper{
				Client:         nil,
				Endpoint:       endpointName,
				BackOffEndTime: metav1.Time{},
				FailedAttempts: 0,
			}

			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			keyValueEtags[newKeyValueSelector("*", nil)] = []*azcore.ETag{}
			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(nil, netErr).Times(1)
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil).Times(1)
			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&failedClient, &succeededClient}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(err).Should(BeNil())
			Expect(failedClient.FailedAttempts).Should(Equal(1))
			Expect(failedClient.BackOffEndTime.IsZero()).Should(BeFalse())
			Expect(succeededClient.FailedAttempts).Should(Equal(-1))
			Expect(succeededClient.BackOffEndTime.IsZero()).Should(BeTrue())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(6))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["app:"]).Should(Equal("value2"))
			Expect(allSettings.ConfigMapSettings["test:"]).Should(Equal("value3"))
			Expect(allSettings.ConfigMapSettings["app:someSubKey1:1"]).Should(Equal("value4"))
			Expect(allSettings.ConfigMapSettings["app:test:some"]).Should(Equal("value5"))
			Expect(allSettings.ConfigMapSettings["app:test:"]).Should(Equal("value6"))
		})
	})
})

var _ = Describe("TagFilters Support", func() {
	var (
		EndpointName = "https://fake-endpoint"
	)

	Context("When using TagFilters in configuration selectors", func() {
		It("Should filter configuration settings by tags", func() {
			By("By creating a provider with TagFilters selector")
			keyFilter := "myapp:*"
			labelFilter := "production"
			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter:   &keyFilter,
							LabelFilter: &labelFilter,
							TagFilters:  []string{"environment=prod", "team=backend"},
						},
					},
				},
			}

			testProvider := acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "azconfig.io/v1",
					Kind:       "AppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testName",
					Namespace: "testNamespace",
				},
				Spec: testSpec,
			}

			mockCongiurationClientManager.EXPECT().GetClients(gomock.Any()).Return([]*ConfigurationClientWrapper{&fakeClientWrapper}, nil)
			configurationProvider, _ := NewConfigurationSettingLoader(testProvider, mockCongiurationClientManager, mockSettingsClient)

			settingsToReturn := mockConfigurationSettings()
			keyValueEtags := make(map[acpv1.ComparableSelector][]*azcore.ETag)
			selector := newSelectorWithTagFilters(keyFilter, &labelFilter, []string{"environment=prod", "team=backend"})
			keyValueEtags[selector] = []*azcore.ETag{}

			settingsResponse := &SettingsResponse{
				Settings: settingsToReturn,
				Etags:    keyValueEtags,
			}
			mockSettingsClient.EXPECT().GetSettings(gomock.Any(), gomock.Any()).Return(settingsResponse, nil)

			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), nil)

			Expect(err).Should(BeNil())
			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(6))
			Expect(allSettings.KeyValueETags).Should(ContainElement([]*azcore.ETag{}))
		})

		It("Should handle multiple TagFilters in different selectors", func() {
			By("By creating a provider with multiple TagFilters selectors")
			keyFilter1 := "app:config:*"
			keyFilter2 := "feature:*"
			labelFilter := "production"

			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter:   &keyFilter1,
							LabelFilter: &labelFilter,
							TagFilters:  []string{"environment=prod", "component=api"},
						},
						{
							KeyFilter:   &keyFilter2,
							LabelFilter: &labelFilter,
							TagFilters:  []string{"type=feature-flag", "scope=global"},
						},
					},
				},
			}

			filters := GetKeyValueFilters(testSpec)
			Expect(len(filters)).Should(Equal(2))
			Expect(filters[0].TagFilters).Should(ContainElements("environment=prod", "component=api"))
			Expect(filters[1].TagFilters).Should(ContainElements("type=feature-flag", "scope=global"))
		})
	})

	Context("When using TagFilters in feature flag selectors", func() {
		It("Should filter feature flags by tags", func() {
			By("By creating a provider with TagFilters in feature flag selector")
			keyFilter := "my-feature"
			labelFilter := "production"

			testSpec := acpv1.AzureAppConfigurationProviderSpec{
				Endpoint:                &EndpointName,
				ReplicaDiscoveryEnabled: false,
				Target: acpv1.ConfigurationGenerationParameters{
					ConfigMapName: ConfigMapName,
				},
				FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
					Selectors: []acpv1.Selector{
						{
							KeyFilter:   &keyFilter,
							LabelFilter: &labelFilter,
							TagFilters:  []string{"feature=experimental", "audience=internal"},
						},
					},
				},
			}

			filters := GetFeatureFlagFilters(testSpec)
			Expect(len(filters)).Should(Equal(1))
			Expect(*filters[0].KeyFilter).Should(Equal(FeatureFlagKeyPrefix + keyFilter))
			Expect(filters[0].TagFilters).Should(ContainElements("feature=experimental", "audience=internal"))
		})
	})
})

func TestReverse(t *testing.T) {
	one := "one"
	two := "two"
	three := "three"
	four := "four"
	wildcard := "*"
	empty := make([]acpv1.Selector, 0)
	reverse(empty)
	assert.Empty(t, empty)
	labelString := "test"

	oneElement := []acpv1.Selector{{KeyFilter: &wildcard, LabelFilter: &labelString}}
	reverse(oneElement)
	assert.Len(t, oneElement, 1)
	assert.Equal(t, "*", *oneElement[0].KeyFilter)

	oddNumber := []acpv1.Selector{
		{KeyFilter: &one, LabelFilter: &labelString},
		{KeyFilter: &two, LabelFilter: &labelString},
		{KeyFilter: &three, LabelFilter: &labelString}}
	reverse(oddNumber)
	assert.Len(t, oddNumber, 3)
	assert.Equal(t, "three", *oddNumber[0].KeyFilter)
	assert.Equal(t, "two", *oddNumber[1].KeyFilter)
	assert.Equal(t, "one", *oddNumber[2].KeyFilter)

	evenNumber := []acpv1.Selector{
		{KeyFilter: &one, LabelFilter: &labelString},
		{KeyFilter: &two, LabelFilter: &labelString},
		{KeyFilter: &three, LabelFilter: &labelString},
		{KeyFilter: &four, LabelFilter: &labelString}}
	reverse(evenNumber)
	assert.Len(t, evenNumber, 4)
	assert.Equal(t, "four", *evenNumber[0].KeyFilter)
	assert.Equal(t, "three", *evenNumber[1].KeyFilter)
	assert.Equal(t, "two", *evenNumber[2].KeyFilter)
	assert.Equal(t, "one", *evenNumber[3].KeyFilter)
}

func TestGetFilters(t *testing.T) {
	one := "one"
	two := "two"
	three := "three"
	labelString := "test"
	emptyLabel := ""
	testSpec := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &labelString},
				{KeyFilter: &two, LabelFilter: &labelString},
				{KeyFilter: &three, LabelFilter: &labelString}},
		},
		FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &labelString},
				{KeyFilter: &two, LabelFilter: &labelString},
			},
		},
	}

	keyValueFilters := GetKeyValueFilters(testSpec)
	featureFlagFilters := GetFeatureFlagFilters(testSpec)
	assert.Len(t, keyValueFilters, 3)
	assert.Len(t, featureFlagFilters, 2)
	assert.Equal(t, "one", *keyValueFilters[0].KeyFilter)
	assert.Equal(t, "two", *keyValueFilters[1].KeyFilter)
	assert.Equal(t, "three", *keyValueFilters[2].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/one", *featureFlagFilters[0].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/two", *featureFlagFilters[1].KeyFilter)

	testSpec2 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{},
		},
	}

	keyValueFilters2 := GetKeyValueFilters(testSpec2)
	assert.Len(t, keyValueFilters2, 1)
	assert.Equal(t, "*", *keyValueFilters2[0].KeyFilter)
	assert.Nil(t, keyValueFilters2[0].LabelFilter)

	testSpec3 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &labelString},
				{KeyFilter: &two, LabelFilter: &labelString},
				{KeyFilter: &one, LabelFilter: &labelString}},
		},
		FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &labelString},
				{KeyFilter: &two, LabelFilter: &labelString},
				{KeyFilter: &one, LabelFilter: &labelString}},
		},
	}

	keyValueFilters3 := GetKeyValueFilters(testSpec3)
	featureFlagFilters3 := GetFeatureFlagFilters(testSpec3)
	assert.Len(t, keyValueFilters3, 2)
	assert.Len(t, featureFlagFilters3, 2)
	assert.Equal(t, "two", *keyValueFilters3[0].KeyFilter)
	assert.Equal(t, `one`, *keyValueFilters3[1].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/two", *featureFlagFilters3[0].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/one", *featureFlagFilters3[1].KeyFilter)

	testSpec4 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one},
				{KeyFilter: &two, LabelFilter: &labelString},
				{KeyFilter: &one}},
		},
	}

	filters4 := GetKeyValueFilters(testSpec4)
	featureFlagFilters4 := GetFeatureFlagFilters(testSpec4)
	assert.Len(t, filters4, 2)
	assert.Len(t, featureFlagFilters4, 0)
	assert.Equal(t, "two", *filters4[0].KeyFilter)
	assert.Equal(t, "test", *filters4[0].LabelFilter)
	assert.Equal(t, "one", *filters4[1].KeyFilter)
	assert.Equal(t, "\x00", *filters4[1].LabelFilter)

	testSpec5 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one},
				{KeyFilter: &one, LabelFilter: &labelString},
			},
		},
	}

	filters5 := GetKeyValueFilters(testSpec5)
	assert.Len(t, filters5, 2)
	assert.Equal(t, "one", *filters5[0].KeyFilter)
	assert.Equal(t, "one", *filters5[1].KeyFilter)
	assert.Equal(t, "test", *filters5[1].LabelFilter)

	testSpec6 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &labelString},
				{KeyFilter: &one},
			},
		},
	}

	filters6 := GetKeyValueFilters(testSpec6)
	assert.Len(t, filters6, 2)
	assert.Equal(t, "one", *filters6[0].KeyFilter)
	assert.Equal(t, "test", *filters6[0].LabelFilter)
	assert.Equal(t, "one", *filters6[1].KeyFilter)
	assert.Equal(t, "\x00", *filters6[1].LabelFilter)

	testSpec7 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &emptyLabel},
			},
		},
	}

	filters7 := GetKeyValueFilters(testSpec7)
	assert.Len(t, filters7, 1)
	assert.Equal(t, "one", *filters7[0].KeyFilter)
	assert.Equal(t, "\x00", *filters7[0].LabelFilter)

	testSpec8 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Refresh: &acpv1.DynamicConfigurationRefreshParameters{
				Monitoring: &acpv1.RefreshMonitoring{
					Sentinels: []acpv1.Sentinel{
						{
							Key:   one,
							Label: nil,
						},
						{
							Key:   two,
							Label: &emptyLabel,
						},
					},
				},
			},
		},
	}

	sentinels := normalizeSentinels(testSpec8.Configuration.Refresh.Monitoring.Sentinels)
	assert.Len(t, sentinels, 2)
	assert.Equal(t, "one", sentinels[0].Key)
	assert.Equal(t, "\x00", *sentinels[0].Label)
	assert.Equal(t, "two", sentinels[1].Key)
	assert.Equal(t, "\x00", *sentinels[1].Label)

	snapshot := "snapshot"
	testSpec9 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &emptyLabel},
				{SnapshotName: &snapshot},
			},
		},
	}

	filters9 := GetKeyValueFilters(testSpec9)
	assert.Len(t, filters9, 2)
	assert.Equal(t, "one", *filters9[0].KeyFilter)
	assert.Equal(t, "\x00", *filters9[0].LabelFilter)
	assert.Equal(t, "snapshot", *filters9[1].SnapshotName)

	testSpec10 := acpv1.AzureAppConfigurationProviderSpec{
		FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &emptyLabel},
				{SnapshotName: &snapshot},
			},
		},
	}

	filters10 := GetFeatureFlagFilters(testSpec10)
	assert.Len(t, filters10, 2)
	assert.Equal(t, ".appconfig.featureflag/one", *filters10[0].KeyFilter)
	assert.Equal(t, "\x00", *filters10[0].LabelFilter)
	assert.Equal(t, "snapshot", *filters10[1].SnapshotName)

	testSpec11 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: &one, LabelFilter: &emptyLabel, TagFilters: []string{
					"tag1=value1",
					"tag2=value2",
					"tag1=value1",
					"tag2=value2",
				}},
			},
		},
	}

	filters11 := GetKeyValueFilters(testSpec11)
	assert.Len(t, filters11, 1)
	assert.Equal(t, "one", *filters11[0].KeyFilter)
	assert.Equal(t, "\x00", *filters11[0].LabelFilter)
	assert.Len(t, filters11[0].TagFilters, 2)
}

func TestCompare(t *testing.T) {
	var nilString *string = nil
	stringA := "stringA"
	anotherStringA := "stringA"
	stringB := "stringB"

	assert.True(t, compare(nilString, nilString))
	assert.False(t, compare(nilString, &stringA))
	assert.False(t, compare(&stringB, nilString))
	assert.True(t, compare(&stringA, &anotherStringA))
	assert.False(t, compare(&stringA, &stringB))
}

func TestCreateSecretClients(t *testing.T) {
	configProvider := &acpv1.AzureAppConfigurationProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appconfig.kubernetes.config/v1",
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "providerName",
			Namespace: ProviderNamespace,
			Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
		},
		Spec: acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &endpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: "configMap-test",
			},
			Secret: &acpv1.SecretReference{
				Target: acpv1.SecretGenerationParameters{
					SecretName: "secret-test",
				},
			},
		},
	}
	secretClients, err := createSecretClients(context.Background(), *configProvider)
	length := 0
	secretClients.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	assert.Equal(t, length, 0)
	assert.Nil(t, err)

	testFakeManagedIdentity := "8766e199-e6df-4416-9f23-ce3a7ece0dca"
	configProvider2 := &acpv1.AzureAppConfigurationProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appconfig.kubernetes.config/v1",
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "providerName",
			Namespace: ProviderNamespace,
			Labels:    map[string]string{"foo": "fooValue", "bar": "barValue"},
		},
		Spec: acpv1.AzureAppConfigurationProviderSpec{
			Endpoint: &endpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: "configMap-test",
			},
			Secret: &acpv1.SecretReference{
				Target: acpv1.SecretGenerationParameters{
					SecretName: "secretName",
				},
				Auth: &acpv1.AzureKeyVaultAuth{
					KeyVaults: []acpv1.AzureKeyVaultPerVaultAuth{
						{
							Uri: "HTTPS://FAKE-VAULT/",
							AzureAppConfigurationProviderAuth: &acpv1.AzureAppConfigurationProviderAuth{
								ManagedIdentityClientId: &testFakeManagedIdentity,
							},
						},
						{
							Uri: "https://FAKE-VAULT2",
							AzureAppConfigurationProviderAuth: &acpv1.AzureAppConfigurationProviderAuth{
								ManagedIdentityClientId: &testFakeManagedIdentity,
							},
						},
					},
				},
			},
		},
	}
	secretClients2, err := createSecretClients(context.Background(), *configProvider2)
	r1, _ := secretClients2.Load("fake-vault")
	r2, _ := secretClients2.Load("fake-vault2")
	length = 0
	secretClients2.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	assert.Equal(t, length, 2)
	assert.Nil(t, err)
	assert.NotNil(t, r1)
	assert.NotNil(t, r2)
}

func TestEndpointValidation(t *testing.T) {
	specifiedEndpoint := "https://fake.azconfig.io"
	validDomain := getValidDomain(specifiedEndpoint)

	assert.True(t, isValidEndpoint("azure.azconfig.io", validDomain))
	assert.True(t, isValidEndpoint("appconfig.azconfig.io", validDomain))
	assert.True(t, isValidEndpoint("azure.privatelink.azconfig.io", validDomain))
	assert.True(t, isValidEndpoint("azure-replica.azconfig.io", validDomain))
	assert.False(t, isValidEndpoint("azure.badazconfig.io", validDomain))
	assert.False(t, isValidEndpoint("azure.azconfigbad.io", validDomain))
	assert.False(t, isValidEndpoint("azure.appconfig.azure.com", validDomain))
	assert.False(t, isValidEndpoint("azure.azconfig.bad.io", validDomain))

	specifiedEndpoint2 := "https://foobar.appconfig.azure.com"
	validDomain2 := getValidDomain(specifiedEndpoint2)

	assert.True(t, isValidEndpoint("azure.appconfig.azure.com", validDomain2))
	assert.True(t, isValidEndpoint("azure.z1.appconfig.azure.com", validDomain2))
	assert.True(t, isValidEndpoint("azure-replia.z1.appconfig.azure.com", validDomain2))
	assert.True(t, isValidEndpoint("azure.privatelink.appconfig.azure.com", validDomain2))
	assert.True(t, isValidEndpoint("azconfig.appconfig.azure.com", validDomain2))
	assert.False(t, isValidEndpoint("azure.azconfig.io", validDomain2))
	assert.False(t, isValidEndpoint("azure.badappconfig.azure.com", validDomain2))
	assert.False(t, isValidEndpoint("azure.appconfigbad.azure.com", validDomain2))

	specifiedEndpoint3 := "https://foobar.azconfig-test.io"
	assert.False(t, isValidEndpoint("azure.azconfig.io", getValidDomain(specifiedEndpoint3)))
}

func createFakeKeyPem(key *rsa.PrivateKey) (string, error) {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	// PEM encoding of private key
	keyPEM := string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: keyBytes,
		},
	))

	return keyPEM, nil
}

func createFakeCertPem(key *rsa.PrivateKey) (string, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * 10 * time.Hour)

	//Create certificate templet
	template := x509.Certificate{
		SerialNumber:          big.NewInt(0),
		Subject:               pkix.Name{CommonName: "localhost"},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	//Create certificate using templet
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return "", err

	}
	//pem encoding of certificate
	certPem := string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		},
	))

	return certPem, nil
}

func createFakePem() (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", err
	}

	keyPEM, err := createFakeKeyPem(key)
	if err != nil {
		return "", err
	}

	certPem, err := createFakeCertPem(key)
	if err != nil {
		return "", err
	}

	return keyPEM + "\n" + certPem, nil
}

func createFakePfx() (string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", err
	}

	keyPEM, err := createFakeKeyPem(key)
	if err != nil {
		return "", err
	}

	certPem, err := createFakeCertPem(key)
	if err != nil {
		return "", err
	}

	return createPFXFromPEM(keyPEM, certPem)
}

func createPFXFromPEM(pemPrivateKey, pemCertificate string) (string, error) {
	// Decode private key PEM
	block, _ := pem.Decode([]byte(pemPrivateKey))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return "", fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	// Decode certificate PEM
	block, _ = pem.Decode([]byte(pemCertificate))
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	// Create PKCS12 structure
	pfxData, err := pkcs12.Legacy.Encode(privateKey, cert, []*x509.Certificate{}, "")
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pfxData), nil
}

// TestTagFiltersComparison tests the ComparableSelector with TagFilters
func TestTagFiltersComparison(t *testing.T) {
	t.Run("TagFilters with same tags in different order should be equal", func(t *testing.T) {
		selector1 := acpv1.Selector{
			TagFilters: []string{"tag1", "tag2", "tag3"},
		}
		selector2 := acpv1.Selector{
			TagFilters: []string{"tag3", "tag1", "tag2"},
		}

		comparable1 := acpv1.MakeComparable(selector1)
		comparable2 := acpv1.MakeComparable(selector2)

		assert.Equal(t, comparable1, comparable2, "Selectors with same tags in different order should be equal")
	})

	t.Run("TagFilters with different tags should not be equal", func(t *testing.T) {
		selector1 := acpv1.Selector{
			TagFilters: []string{"tag1", "tag2"},
		}
		selector2 := acpv1.Selector{
			TagFilters: []string{"tag1", "tag3"},
		}

		comparable1 := acpv1.MakeComparable(selector1)
		comparable2 := acpv1.MakeComparable(selector2)

		assert.NotEqual(t, comparable1, comparable2, "Selectors with different tags should not be equal")
	})

	t.Run("Empty TagFilters should be handled correctly", func(t *testing.T) {
		selector1 := acpv1.Selector{}
		selector2 := acpv1.Selector{TagFilters: []string{}}

		comparable1 := acpv1.MakeComparable(selector1)
		comparable2 := acpv1.MakeComparable(selector2)

		assert.Equal(t, comparable1, comparable2, "Empty TagFilters should be equal")
		assert.Nil(t, comparable1.TagFilters, "Empty TagFilters should result in nil TagFilters field")
	})

	t.Run("Single tag should be preserved", func(t *testing.T) {
		selector := acpv1.Selector{
			TagFilters: []string{"single-tag"},
		}

		comparable := acpv1.MakeComparable(selector)
		restored := acpv1.FromComparable(comparable)

		assert.Equal(t, selector.TagFilters, restored.TagFilters, "Single tag should be preserved through conversion")
	})

	t.Run("Round-trip conversion should preserve TagFilters", func(t *testing.T) {
		original := acpv1.Selector{
			TagFilters: []string{"environment:prod", "team:backend", "service:api"},
		}

		comparable := acpv1.MakeComparable(original)
		restored := acpv1.FromComparable(comparable)

		// Tags should be preserved, but order might change due to sorting
		assert.ElementsMatch(t, original.TagFilters, restored.TagFilters, "TagFilters should be preserved through round-trip conversion")
	})
}

// TestTagFiltersInSelectors tests TagFilters usage in selector scenarios
func TestTagFiltersInSelectors(t *testing.T) {
	t.Run("GetKeyValueFilters should handle TagFilters", func(t *testing.T) {
		keyFilter := "myapp:*"
		labelFilter := "production"
		testSpec := acpv1.AzureAppConfigurationProviderSpec{
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{
					{
						KeyFilter:   &keyFilter,
						LabelFilter: &labelFilter,
						TagFilters:  []string{"environment=prod", "team=backend"},
					},
				},
			},
		}

		filters := GetKeyValueFilters(testSpec)
		assert.Len(t, filters, 1)
		assert.Equal(t, keyFilter, *filters[0].KeyFilter)
		assert.Equal(t, labelFilter, *filters[0].LabelFilter)
		assert.ElementsMatch(t, []string{"environment=prod", "team=backend"}, filters[0].TagFilters)
	})

	t.Run("GetFeatureFlagFilters should handle TagFilters", func(t *testing.T) {
		keyFilter := "feature-toggle"
		labelFilter := "production"
		testSpec := acpv1.AzureAppConfigurationProviderSpec{
			FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
				Selectors: []acpv1.Selector{
					{
						KeyFilter:   &keyFilter,
						LabelFilter: &labelFilter,
						TagFilters:  []string{"feature:experimental", "scope:global"},
					},
				},
			},
		}

		filters := GetFeatureFlagFilters(testSpec)
		assert.Len(t, filters, 1)
		expectedKey := FeatureFlagKeyPrefix + keyFilter
		assert.Equal(t, expectedKey, *filters[0].KeyFilter)
		assert.Equal(t, labelFilter, *filters[0].LabelFilter)
		assert.ElementsMatch(t, []string{"feature:experimental", "scope:global"}, filters[0].TagFilters)
	})

	t.Run("Duplicate selectors with same TagFilters should be deduplicated", func(t *testing.T) {
		keyFilter := "myapp"
		testSpec := acpv1.AzureAppConfigurationProviderSpec{
			Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
				Selectors: []acpv1.Selector{
					{
						KeyFilter:  &keyFilter,
						TagFilters: []string{"env:prod", "team:backend"},
					},
					{
						KeyFilter:  &keyFilter,
						TagFilters: []string{"team:backend", "env:prod"}, // Same tags, different order
					},
				},
			},
		}

		filters := GetKeyValueFilters(testSpec)
		assert.Len(t, filters, 1, "Duplicate selectors with same TagFilters should be deduplicated")
	})
}

// Helper function to create a ComparableSelector with TagFilters for testing
func newSelectorWithTagFilters(key string, label *string, tagFilters []string) acpv1.ComparableSelector {
	selector := acpv1.Selector{
		KeyFilter:   &key,
		LabelFilter: label,
		TagFilters:  tagFilters,
	}
	return acpv1.MakeComparable(selector)
}
