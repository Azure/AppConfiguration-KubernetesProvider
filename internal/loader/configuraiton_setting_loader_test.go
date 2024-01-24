// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package loader

import (
	acpv1 "azappconfig/provider/api/v1"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azappconfig"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var (
	mockResolveSecretReference *MockResolveSecretReference
	mockCtrl                   *gomock.Controller
	EndpointName               string = "https://fake-endpoint"
)

func mockGetConfigurationSettings(ctx context.Context, filters []acpv1.Selector, client *azappconfig.Client, c chan []azappconfig.Setting, e chan error) {
	settingsToReturn := make([]azappconfig.Setting, 6)
	settingsToEnd := make([]azappconfig.Setting, 0)
	settingsToReturn[0] = newCommonKeyValueSettings("someKey1", "value1", "label1")
	settingsToReturn[1] = newCommonKeyValueSettings("app:", "value2", "label1")
	settingsToReturn[2] = newCommonKeyValueSettings("test:", "value3", "label1")
	settingsToReturn[3] = newCommonKeyValueSettings("app:someSubKey1:1", "value4", "label1")
	settingsToReturn[4] = newCommonKeyValueSettings("app:test:some", "value5", "label1")
	settingsToReturn[5] = newCommonKeyValueSettings("app:test:", "value6", "label1")

	c <- settingsToReturn
	c <- settingsToEnd
}

func mockGetConfigurationSettingsWithKV(ctx context.Context, filters []acpv1.Selector, client *azappconfig.Client, c chan []azappconfig.Setting, e chan error) {
	settingsToReturn := make([]azappconfig.Setting, 3)
	settingsToEnd := make([]azappconfig.Setting, 0)
	settingsToReturn[0] = newCommonKeyValueSettings("someKey1", "value1", "label1")
	settingsToReturn[1] = newCommonKeyValueSettings("app:someSubKey1:1", "value4", "label1")
	settingsToReturn[2] = newKeyVaultSettings("app:secret:1", "label1")

	c <- settingsToReturn
	c <- settingsToEnd
}

func mockGetConfigurationSettingsThrowError(ctx context.Context, filters []acpv1.Selector, client *azappconfig.Client, c chan []azappconfig.Setting, e chan error) {
	settingsToReturn := make([]azappconfig.Setting, 3)
	err := errors.New("a fake error")
	settingsToReturn[0] = newCommonKeyValueSettings("someKey1", "value1", "label1")
	settingsToReturn[1] = newCommonKeyValueSettings("app:someSubKey1:1", "value4", "label1")
	settingsToReturn[2] = newKeyVaultSettings("app:secret:1", "label1")

	c <- settingsToReturn
	e <- err
}

func newCommonKeyValueSettings(key string, value string, label string) azappconfig.Setting {
	return azappconfig.Setting{
		Key:   &key,
		Value: &value,
		Label: &label,
	}
}

func newKeyVaultSettings(key string, label string) azappconfig.Setting {
	vault := "{ \"test\":\"https://fake-vault\"}"
	keyVaultContentType := KeyVaultReferenceContentType

	return azappconfig.Setting{
		Key:         &key,
		Value:       &vault,
		Label:       &label,
		ContentType: &keyVaultContentType,
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
func (m *MockResolveSecretReference) Resolve(arg0 KeyVaultSecretUriSegment, arg1 context.Context) (*string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Resolve", arg0, arg1)
	ret0, _ := ret[0].(*string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Resolve indicates an expected call of Resolve.
func (mr *MockResolveSecretReferenceMockRecorder) Resolve(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Resolve", reflect.TypeOf((*MockResolveSecretReference)(nil).Resolve), arg0, arg1)
}

const (
	ProviderName      = "test-appconfigurationprovider"
	ProviderNamespace = "default"
	ConfigMapName     = "configmap-to-be-created"
)

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
	mockResolveSecretReference = NewMockResolveSecretReference(mockCtrl)

	go func() {
		defer GinkgoRecover()
	}()
})

var _ = AfterEach(func() {
	By("tearing down the test environment")
	mockCtrl.Finish()
})

var _ = Describe("AppConfiguationProvider Get All Settings", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals..
	const (
		ProviderName      = "test-appconfigurationprovider"
		ProviderNamespace = "default"
	)

	var (
		EndpointName = "https://fake-endpoint"
	)

	Context("When get Key Vault Reference Type Settings", func() {
		It("Should put into Secret settings collection", func() {
			By("By resolving the settings from Azure Key Vault")
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

			configurationProvider, _ := NewConfigurationSettingLoader(context.Background(), testProvider, mockGetConfigurationSettingsWithKV)
			secretValue := "fakeSecretValue"
			secretValue2 := "fakeSecretValue2"
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Times(0).Return(&secretValue, nil)
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Times(0).Return(&secretValue2, nil)
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(len(allSettings.ConfigMapSettings)).Should(Equal(2))
			Expect(len(allSettings.SecretSettings)).Should(Equal(2))
			Expect(allSettings.ConfigMapSettings["someKey1"]).Should(Equal("value1"))
			Expect(allSettings.ConfigMapSettings["someSubKey1;1"]).Should(Equal("value2"))
			Expect(allSettings.SecretSettings["secret:1"]).Should(Equal(secretValue))
			Expect(allSettings.SecretSettings["someSecret"]).Should(Equal(secretValue2))
			Expect(err).Should(BeNil())
		})

		It("Should throw exception", func() {
			By("By resolving Key Vault reference to fail")
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

			configurationProvider, _ := NewConfigurationSettingLoader(context.Background(), testProvider, mockGetConfigurationSettingsWithKV)

			secretValue := "fakeSecretValue"
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Times(0).Return(&secretValue, nil)
			mockResolveSecretReference.EXPECT().Resolve(gomock.Any(), gomock.Any()).Times(0).Return(nil, errors.New("Some error"))
			allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), mockResolveSecretReference)

			Expect(allSettings).Should(BeNil())
			Expect(err.Error()).Should(Equal("Some error"))
		})
	})
})

func TestGetAllConfigurationSettingsNoTrim(t *testing.T) {
	testSpec := acpv1.AzureAppConfigurationProviderSpec{
		Endpoint: &EndpointName,
		Target: acpv1.ConfigurationGenerationParameters{
			ConfigMapName: ConfigMapName,
		},
	}
	testProvider := acpv1.AzureAppConfigurationProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "azconfig.io/v1",
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
		Spec: testSpec,
	}

	configurationProvider, _ := NewConfigurationSettingLoader(context.TODO(), testProvider, mockGetConfigurationSettings)
	allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), nil)

	assert.Equal(t, 6, len(allSettings.ConfigMapSettings))
	assert.Equal(t, "value1", allSettings.ConfigMapSettings["someKey1"])
	assert.Equal(t, "value2", allSettings.ConfigMapSettings["app:"])
	assert.Equal(t, "value3", allSettings.ConfigMapSettings["test:"])
	assert.Equal(t, "value4", allSettings.ConfigMapSettings["app:someSubKey1:1"])
	assert.Equal(t, "value5", allSettings.ConfigMapSettings["app:test:some"])
	assert.Equal(t, "value6", allSettings.ConfigMapSettings["app:test:"])
	assert.Nil(t, err)
}

func TestGetAllConfigurationSettingsTrimSinglePrefix(t *testing.T) {
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
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
		Spec: testSpec,
	}

	configurationProvider, _ := NewConfigurationSettingLoader(context.TODO(), testProvider, mockGetConfigurationSettings)
	allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), nil)

	assert.Equal(t, 4, len(allSettings.ConfigMapSettings))
	assert.Equal(t, "value1", allSettings.ConfigMapSettings["someKey1"])
	assert.Equal(t, "value4", allSettings.ConfigMapSettings["someSubKey1:1"])
	assert.Equal(t, "value5", allSettings.ConfigMapSettings["test:some"])
	assert.Equal(t, "value6", allSettings.ConfigMapSettings["test:"])
	assert.Nil(t, err)
}

func TestGetAllConfigurationSettingsTrimMultiplePrefix(t *testing.T) {
	testSpec := acpv1.AzureAppConfigurationProviderSpec{
		Endpoint: &EndpointName,
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
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
		Spec: testSpec,
	}

	configurationProvider, _ := NewConfigurationSettingLoader(context.TODO(), testProvider, mockGetConfigurationSettings)
	allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), nil)

	assert.Equal(t, 4, len(allSettings.ConfigMapSettings))
	assert.Equal(t, "value1", allSettings.ConfigMapSettings["someKey1"])
	assert.Equal(t, "value4", allSettings.ConfigMapSettings["someSubKey1:1"])
	assert.Equal(t, "value5", allSettings.ConfigMapSettings["test:some"])
	assert.Equal(t, "value6", allSettings.ConfigMapSettings["test:"])
	assert.Nil(t, err)
}

func TestErrorWhenGetAllConfiguration(t *testing.T) {
	testSpec := acpv1.AzureAppConfigurationProviderSpec{
		Endpoint: &EndpointName,
		Target: acpv1.ConfigurationGenerationParameters{
			ConfigMapName: ConfigMapName,
		},
	}
	testProvider := acpv1.AzureAppConfigurationProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "azconfig.io/v1",
			Kind:       "AzureAppConfigurationProvider",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNamespace",
		},
		Spec: testSpec,
	}

	configurationProvider, _ := NewConfigurationSettingLoader(context.TODO(), testProvider, mockGetConfigurationSettingsThrowError)

	allSettings, err := configurationProvider.CreateTargetSettings(context.Background(), nil)

	assert.Nil(t, allSettings)
	assert.NotNil(t, err)
}

func TestReverse(t *testing.T) {
	empty := make([]acpv1.Selector, 0)
	reverse(empty)
	assert.Empty(t, empty)
	labelString := "test"

	oneElement := []acpv1.Selector{{KeyFilter: "*", LabelFilter: &labelString}}
	reverse(oneElement)
	assert.Len(t, oneElement, 1)
	assert.Equal(t, "*", oneElement[0].KeyFilter)

	oddNumber := []acpv1.Selector{
		{KeyFilter: "one", LabelFilter: &labelString},
		{KeyFilter: "two", LabelFilter: &labelString},
		{KeyFilter: "three", LabelFilter: &labelString}}
	reverse(oddNumber)
	assert.Len(t, oddNumber, 3)
	assert.Equal(t, "three", oddNumber[0].KeyFilter)
	assert.Equal(t, "two", oddNumber[1].KeyFilter)
	assert.Equal(t, "one", oddNumber[2].KeyFilter)

	evenNumber := []acpv1.Selector{
		{KeyFilter: "one", LabelFilter: &labelString},
		{KeyFilter: "two", LabelFilter: &labelString},
		{KeyFilter: "three", LabelFilter: &labelString},
		{KeyFilter: "four", LabelFilter: &labelString}}
	reverse(evenNumber)
	assert.Len(t, evenNumber, 4)
	assert.Equal(t, "four", evenNumber[0].KeyFilter)
	assert.Equal(t, "three", evenNumber[1].KeyFilter)
	assert.Equal(t, "two", evenNumber[2].KeyFilter)
	assert.Equal(t, "one", evenNumber[3].KeyFilter)
}

func TestGetFilters(t *testing.T) {
	labelString := "test"
	testSpec := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one", LabelFilter: &labelString},
				{KeyFilter: "two", LabelFilter: &labelString},
				{KeyFilter: "three", LabelFilter: &labelString}},
		},
		FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one", LabelFilter: &labelString},
				{KeyFilter: "two", LabelFilter: &labelString},
			},
		},
	}

	keyValueFilters := getKeyValueFilters(testSpec)
	featureFlagFilters := getFeatureFlagFilters(testSpec)
	assert.Len(t, keyValueFilters, 3)
	assert.Len(t, featureFlagFilters, 2)
	assert.Equal(t, "one", keyValueFilters[0].KeyFilter)
	assert.Equal(t, "two", keyValueFilters[1].KeyFilter)
	assert.Equal(t, "three", keyValueFilters[2].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/one", featureFlagFilters[0].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/two", featureFlagFilters[1].KeyFilter)

	testSpec2 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{},
		},
	}

	keyValueFilters2 := getKeyValueFilters(testSpec2)
	assert.Len(t, keyValueFilters2, 1)
	assert.Equal(t, "*", keyValueFilters2[0].KeyFilter)
	assert.Nil(t, keyValueFilters2[0].LabelFilter)

	testSpec3 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one", LabelFilter: &labelString},
				{KeyFilter: "two", LabelFilter: &labelString},
				{KeyFilter: "one", LabelFilter: &labelString}},
		},
		FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one", LabelFilter: &labelString},
				{KeyFilter: "two", LabelFilter: &labelString},
				{KeyFilter: "one", LabelFilter: &labelString}},
		},
	}

	keyValueFilters3 := getKeyValueFilters(testSpec3)
	featureFlagFilters3 := getFeatureFlagFilters(testSpec3)
	assert.Len(t, keyValueFilters3, 2)
	assert.Len(t, featureFlagFilters3, 2)
	assert.Equal(t, "two", keyValueFilters3[0].KeyFilter)
	assert.Equal(t, `one`, keyValueFilters3[1].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/two", featureFlagFilters3[0].KeyFilter)
	assert.Equal(t, ".appconfig.featureflag/one", featureFlagFilters3[1].KeyFilter)

	testSpec4 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one"},
				{KeyFilter: "two", LabelFilter: &labelString},
				{KeyFilter: "one"}},
		},
	}

	filters4 := getKeyValueFilters(testSpec4)
	featureFlagFilters4 := getFeatureFlagFilters(testSpec4)
	assert.Len(t, filters4, 2)
	assert.Len(t, featureFlagFilters4, 0)
	assert.Equal(t, "two", filters4[0].KeyFilter)
	assert.Equal(t, "test", *filters4[0].LabelFilter)
	assert.Equal(t, `one`, filters4[1].KeyFilter)
	assert.Nil(t, filters4[1].LabelFilter)

	testSpec5 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one"},
				{KeyFilter: "one", LabelFilter: &labelString},
			},
		},
	}

	filters5 := getKeyValueFilters(testSpec5)
	assert.Len(t, filters5, 2)
	assert.Equal(t, "one", filters5[0].KeyFilter)
	assert.Equal(t, "one", filters5[1].KeyFilter)
	assert.Equal(t, "test", *filters5[1].LabelFilter)

	testSpec6 := acpv1.AzureAppConfigurationProviderSpec{
		Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
			Selectors: []acpv1.Selector{
				{KeyFilter: "one", LabelFilter: &labelString},
				{KeyFilter: "one"},
			},
		},
	}

	filters6 := getKeyValueFilters(testSpec6)
	assert.Len(t, filters6, 2)
	assert.Equal(t, "one", filters6[0].KeyFilter)
	assert.Equal(t, "test", *filters6[0].LabelFilter)
	assert.Equal(t, "one", filters6[1].KeyFilter)
	assert.Nil(t, filters6[1].LabelFilter)
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
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: "configMap-test",
			},
			Secret: &acpv1.AzureKeyVaultReference{
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
			Endpoint: &EndpointName,
			Target: acpv1.ConfigurationGenerationParameters{
				ConfigMapName: "configMap-test",
			},
			Secret: &acpv1.AzureKeyVaultReference{
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
