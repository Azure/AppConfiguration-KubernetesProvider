// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"azappconfig/provider/internal/loader"
	"azappconfig/provider/internal/loader/mocks"
	"context"
	"time"

	acpv1 "azappconfig/provider/api/v1"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("AppConfiguationProvider processor", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		ProviderName      = "test-appconfigurationprovider"
		ProviderNamespace = "default"
	)

	var (
		EndpointName              = "https://fake-endpoint"
		mockCtrl                  *gomock.Controller
		mockConfigurationSettings *mocks.MockConfigurationSettingsRetriever
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockConfigurationSettings = mocks.NewMockConfigurationSettingsRetriever(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Reconcile triggered by one component refresh scenarios", func() {
		It("Should update reconcile state when sentinel Etag updated when configuration refresh enabled", func() {
			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\"}"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
			}

			ctx := context.Background()
			providerName := "test-appconfigurationprovider-sentinel"
			configMapName := "configmap-sentinel"
			testKey := "*"

			sentinelKey1 := "sentinel1"
			sentinelKey2 := "sentinel2"

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "5s",
							Monitoring: &acpv1.RefreshMonitoring{
								Sentinels: []acpv1.Sentinel{
									{
										Key: sentinelKey1,
									},
									{
										Key: sentinelKey2,
									},
								},
							},
						},
					},
				},
			}

			fakeEtag := azcore.ETag("fake-etag")
			fakeResourceVersion := "1"

			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextKeyValueRefreshReconcileTime: metav1.Now(),
					SentinelETags: map[acpv1.Sentinel]*azcore.ETag{
						{
							Key: sentinelKey1,
						}: &fakeEtag,
					},
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.Now(),
				RefreshOptions: &RefreshOptions{},
			}

			newFakeEtag1 := azcore.ETag("fake-etag-1")
			newFakeEtag2 := azcore.ETag("fake-etag-2")
			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(5 * time.Second))

			//Sentinel Etag is updated
			mockConfigurationSettings.EXPECT().CheckAndRefreshSentinels(gomock.Any(), gomock.Any(), gomock.Any()).Return(
				true,
				map[acpv1.Sentinel]*azcore.ETag{
					{
						Key: sentinelKey1,
					}: &newFakeEtag1,
					{
						Key: sentinelKey2,
					}: &newFakeEtag2,
				},
				nil,
			)

			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.SentinelETags[acpv1.Sentinel{
				Key: sentinelKey1,
			}]).Should(Equal(&newFakeEtag1))
			Expect(processor.ReconciliationState.SentinelETags[acpv1.Sentinel{
				Key: sentinelKey2,
			}]).Should(Equal(&newFakeEtag2))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
		})
	})

	Context("Reconcile triggered by two component refresh scenarios", func() {
		// 4 scenarios: Both Etags updated, only keyValue Etag updated, only featureFlag Etag updated, both Etags not updated
		It("Should update reconcile state when featureFlag pageEtag and keyValue pageEtag updated when configuration refresh and feature flag refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-config-ff-etags"
			configMapName := "configmap-config-ff-etags"
			testKey := "*"
			testFeatureFlagSelector := "*"

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "2s",
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testFeatureFlagSelector,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "2s",
							Enabled:  true,
						},
					},
				},
			}

			fakeEtag := azcore.ETag("fake-etag")
			fakeEtag2 := azcore.ETag("fake-etag2")
			fakeResourceVersion := "1"

			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextFeatureFlagRefreshReconcileTime: metav1.Now(),
					NextKeyValueRefreshReconcileTime:    metav1.Now(),
					KeyValueETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &testKey,
						}: {
							&fakeEtag,
						},
					},
					FeatureFlagETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &testFeatureFlagSelector,
						}: {
							&fakeEtag2,
						},
					},
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.Now(),
				RefreshOptions: &RefreshOptions{},
			}

			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(2 * time.Second))
			expectedNextFeatureFlagRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(2 * time.Second))
			newFakeEtag := azcore.ETag("fake-etag-1")
			newFakeEtag2 := azcore.ETag("fake-etag-2")
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag,
				},
			}
			updatedFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testFeatureFlagSelector,
				}: {
					&newFakeEtag2,
				},
			}
			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				FeatureFlagETags:  updatedFeatureFlagEtags,
			}

			//Both Etags are updated
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(expectedNextFeatureFlagRefreshReconcileTime))

			newKeyValueEtag := azcore.ETag("fake-keyValue-etag")
			updatedKeyValueEtags2 := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newKeyValueEtag,
				},
			}

			allSettings2 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags2,
			}

			//Only keyValue Etag is updated
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(false, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings2, nil)

			time.Sleep(2 * time.Second)
			processor.CurrentTime = metav1.Now()

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags2[acpv1.Selector{
				KeyFilter: &testKey,
			}]))

			newFeatureFlagEtag := azcore.ETag("fake-ff-etag")
			updatedFeatureFlagEtags2 := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testFeatureFlagSelector,
				}: {
					&newFeatureFlagEtag,
				},
			}

			allSettings3 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				FeatureFlagETags:  updatedFeatureFlagEtags2,
			}

			//Only featureFlag Etag is updated
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettings3, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(false, nil)

			time.Sleep(2 * time.Second)
			processor.CurrentTime = metav1.Now()

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]).Should(Equal(updatedFeatureFlagEtags2[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags2[acpv1.Selector{
				KeyFilter: &testKey,
			}]))

			//Both Etags are not updated
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(false, nil).Times(2)

			time.Sleep(2 * time.Second)
			processor.CurrentTime = metav1.Now()

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]).Should(Equal(updatedFeatureFlagEtags2[acpv1.Selector{
				KeyFilter: &testFeatureFlagSelector,
			}]))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags2[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
		})

		It("Should update reconcile state when keyValue pageEtag and secret references updated when configuration refresh and secret refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-secret"
			configMapName := "configmap-test"
			secretName := "secret-test"
			fakeSecretResourceVersion := "1"
			testKey := "*"

			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"aKey\":\"testValue\"}"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			existingSecrets := make(map[string]corev1.Secret)
			existingSecrets[secretName] = corev1.Secret{
				Data: secretResult,
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: fakeSecretResourceVersion,
				},
			}

			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
			cachedK8sSecrets := make(map[string]*loader.TargetK8sSecretMetadata)
			cachedK8sSecrets[secretName] = &loader.TargetK8sSecretMetadata{
				Type:                    corev1.SecretType("Opaque"),
				SecretsKeyVaultMetadata: secretMetadata2,
				SecretResourceVersion:   fakeSecretResourceVersion,
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag,
				},
			}

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "5s",
						},
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1m",
							Enabled:  true,
						},
					},
				},
			}

			fakeResourceVersion := "1"
			fakeEtag := azcore.ETag("fake-etag")
			nowTime := metav1.Now()
			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextSecretReferenceRefreshReconcileTime: nowTime,
					NextKeyValueRefreshReconcileTime:        nowTime,
					KeyValueETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &testKey,
						}: {
							&fakeEtag,
						},
					},
					ExistingK8sSecrets:       cachedK8sSecrets,
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.NewTime(nowTime.Time.Add(1 * time.Second)),
				RefreshOptions: &RefreshOptions{},
			}

			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)
			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(5 * time.Second))
			expectedNextSecretReferenceRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(1 * time.Minute))

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.ExistingK8sSecrets).Should(Equal(allSettings.K8sSecrets))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(expectedNextSecretReferenceRefreshReconcileTime))
		})
	})

	Context("Reconcile triggered by three component refresh scenarios", func() {
		It("Should update key value pageEtag and existing secret references when configuration, secret and feature flag refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-secret"
			configMapName := "configmap-test"
			secretName := "secret-test"
			fakeSecretResourceVersion := "1"
			testKey := "*"
			wildcard := "*"

			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			existingSecrets := make(map[string]corev1.Secret)
			existingSecrets[secretName] = corev1.Secret{
				Data: secretResult,
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: fakeSecretResourceVersion,
				},
			}

			fakeFeatureFlagEtag := azcore.ETag("fake-etag-1")
			existingFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&fakeFeatureFlagEtag,
				},
			}

			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
			cachedK8sSecrets := make(map[string]*loader.TargetK8sSecretMetadata)
			cachedK8sSecrets[secretName] = &loader.TargetK8sSecretMetadata{
				Type:                    corev1.SecretType("Opaque"),
				SecretsKeyVaultMetadata: secretMetadata2,
				SecretResourceVersion:   fakeSecretResourceVersion,
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag,
				},
			}

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "10s",
						},
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1h",
							Enabled:  true,
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "2m",
							Enabled:  true,
						},
					},
				},
			}

			fakeResourceVersion := "1"
			fakeEtag := azcore.ETag("fake-etag")
			nowTime := metav1.Now()
			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextSecretReferenceRefreshReconcileTime: metav1.NewTime(nowTime.Time.Add(1 * time.Hour)),
					NextKeyValueRefreshReconcileTime:        nowTime,
					NextFeatureFlagRefreshReconcileTime:     metav1.NewTime(nowTime.Time.Add(2 * time.Minute)),
					KeyValueETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &testKey,
						}: {
							&fakeEtag,
						},
					},
					FeatureFlagETags:         existingFeatureFlagEtags,
					ExistingK8sSecrets:       cachedK8sSecrets,
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.NewTime(nowTime.Time.Add(2 * time.Second)),
				RefreshOptions: &RefreshOptions{},
			}

			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)
			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(10 * time.Second))
			cachedNextSecretReferenceRefreshReconcileTime := processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime
			cachedNextFeatureFlagRefreshReconcileTime := processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.ExistingK8sSecrets).Should(Equal(allSettings.K8sSecrets))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			// FeatureFlag Etag should not be updated
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(existingFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(cachedNextSecretReferenceRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(cachedNextFeatureFlagRefreshReconcileTime))
		})

		It("Should update feature flag pageEtag and existing secret references when configuration, secret and feature flag refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-secret"
			configMapName := "configmap-test"
			secretName := "secret-test"
			fakeSecretResourceVersion := "1"
			testKey := "*"
			wildcard := "*"

			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			existingSecrets := make(map[string]corev1.Secret)
			existingSecrets[secretName] = corev1.Secret{
				Data: secretResult,
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: fakeSecretResourceVersion,
				},
			}

			fakeKeyValueEtag := azcore.ETag("fake-etag-1")
			existingKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&fakeKeyValueEtag,
				},
			}

			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
			cachedK8sSecrets := make(map[string]*loader.TargetK8sSecretMetadata)
			cachedK8sSecrets[secretName] = &loader.TargetK8sSecretMetadata{
				Type:                    corev1.SecretType("Opaque"),
				SecretsKeyVaultMetadata: secretMetadata2,
				SecretResourceVersion:   fakeSecretResourceVersion,
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			updatedFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&newFakeEtag,
				},
			}

			allSettingsReturnedByFeatureFlagRefresh := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				FeatureFlagETags:  updatedFeatureFlagEtags,
			}

			allSettingsReturnedBySecretRefresh := &loader.TargetKeyValueSettings{
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "2m",
						},
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1h",
							Enabled:  true,
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "1m",
							Enabled:  true,
						},
					},
				},
			}

			fakeResourceVersion := "1"
			fakeEtag := azcore.ETag("fake-etag")
			nowTime := metav1.Now()
			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextSecretReferenceRefreshReconcileTime: nowTime,
					NextKeyValueRefreshReconcileTime:        metav1.NewTime(nowTime.Time.Add(2 * time.Minute)),
					NextFeatureFlagRefreshReconcileTime:     nowTime,
					FeatureFlagETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &wildcard,
						}: {
							&fakeEtag,
						},
					},
					KeyValueETags:            existingKeyValueEtags,
					ExistingK8sSecrets:       cachedK8sSecrets,
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.NewTime(nowTime.Time.Add(2 * time.Second)),
				RefreshOptions: &RefreshOptions{},
			}

			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettingsReturnedByFeatureFlagRefresh, nil)
			mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettingsReturnedBySecretRefresh, nil)
			expectedNextFeatureFlagRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(1 * time.Minute))
			expectedNextSecretReferenceRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(1 * time.Hour))
			cachedNextKeyValueRefreshReconcileTime := processor.ReconciliationState.NextKeyValueRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			// KeyValue Etag should not be updated
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(existingKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(cachedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(expectedNextSecretReferenceRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(expectedNextFeatureFlagRefreshReconcileTime))
		})

		It("Should update feature flag pageEtag and keyValue pageEtag when configuration, secret and feature flag refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-secret"
			configMapName := "configmap-test"
			secretName := "secret-test"
			fakeSecretResourceVersion := "1"
			testKey := "*"
			wildcard := "*"

			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			existingSecrets := make(map[string]corev1.Secret)
			existingSecrets[secretName] = corev1.Secret{
				Data: secretResult,
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: fakeSecretResourceVersion,
				},
			}

			fakeKeyValueEtag := azcore.ETag("fake-etag-1")
			existingKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&fakeKeyValueEtag,
				},
			}

			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
			cachedK8sSecrets := make(map[string]*loader.TargetK8sSecretMetadata)
			cachedK8sSecrets[secretName] = &loader.TargetK8sSecretMetadata{
				Type:                    corev1.SecretType("Opaque"),
				SecretsKeyVaultMetadata: secretMetadata2,
				SecretResourceVersion:   fakeSecretResourceVersion,
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			newFakeEtag2 := azcore.ETag("fake-etag-2")
			updatedFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&newFakeEtag,
				},
			}
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag2,
				},
			}

			allSettingsReturnedByFeatureFlagRefresh := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				FeatureFlagETags:  updatedFeatureFlagEtags,
			}

			allSettingsReturnedByKeyValueRefresh := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "70s",
						},
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1h",
							Enabled:  true,
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "1m",
							Enabled:  true,
						},
					},
				},
			}

			fakeResourceVersion := "1"
			fakeEtag := azcore.ETag("fake-etag")
			nowTime := metav1.Now()
			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextSecretReferenceRefreshReconcileTime: metav1.NewTime(nowTime.Time.Add(1 * time.Hour)),
					NextKeyValueRefreshReconcileTime:        nowTime,
					NextFeatureFlagRefreshReconcileTime:     nowTime,
					FeatureFlagETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &wildcard,
						}: {
							&fakeEtag,
						},
					},
					KeyValueETags:            existingKeyValueEtags,
					ExistingK8sSecrets:       cachedK8sSecrets,
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.NewTime(nowTime.Time.Add(2 * time.Second)),
				RefreshOptions: &RefreshOptions{},
			}

			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettingsReturnedByFeatureFlagRefresh, nil)
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettingsReturnedByKeyValueRefresh, nil)
			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(70 * time.Second))
			expectedNextFeatureFlagRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(1 * time.Minute))
			cachedNextSecretReferenceRefreshReconcileTime := processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(cachedNextSecretReferenceRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(expectedNextFeatureFlagRefreshReconcileTime))
		})

		// Should update the corresponding target's reconcile state, and those that shouldn't be updated should be as is.
		It("Should update reconcile state when keyValue pageEtag, featureFlag pageEtag and secret references updated when configuration, secret and feature flag refresh enabled", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-secret"
			configMapName := "configmap-test"
			secretName := "secret-test"
			fakeSecretResourceVersion := "1"
			testKey := "*"
			wildcard := "*"

			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			secretResult := make(map[string][]byte)
			secretResult["testSecretKey"] = []byte("testSecretValue")
			existingSecrets := make(map[string]corev1.Secret)
			existingSecrets[secretName] = corev1.Secret{
				Data: secretResult,
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: fakeSecretResourceVersion,
				},
			}

			secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
			secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
			cachedK8sSecrets := make(map[string]*loader.TargetK8sSecretMetadata)
			cachedK8sSecrets[secretName] = &loader.TargetK8sSecretMetadata{
				Type:                    corev1.SecretType("Opaque"),
				SecretsKeyVaultMetadata: secretMetadata2,
				SecretResourceVersion:   fakeSecretResourceVersion,
			}
			fakeKeyValueEtag := azcore.ETag("fake-etag-1")
			existingKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&fakeKeyValueEtag,
				},
			}
			fakeFeatureFlagEtag := azcore.ETag("fake-etag-2")
			existingFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&fakeFeatureFlagEtag,
				},
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			newFakeEtag2 := azcore.ETag("fake-etag-2")
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag,
				},
			}
			updatedFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&newFakeEtag2,
				},
			}

			allSettingsReturnedByFeatureFlagRefresh := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				FeatureFlagETags:  updatedFeatureFlagEtags,
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			allSettingsReturnedByKeyValueRefresh := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				KeyValueETags:     updatedKeyValueEtags,
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
						SecretResourceVersion:   fakeSecretResourceVersion,
					},
				},
			}

			configProvider := &acpv1.AzureAppConfigurationProvider{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "appconfig.kubernetes.config/v1",
					Kind:       "AzureAppConfigurationProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       providerName,
					Namespace:  ProviderNamespace,
					Generation: 1,
				},
				Spec: acpv1.AzureAppConfigurationProviderSpec{
					Endpoint: &EndpointName,
					Target: acpv1.ConfigurationGenerationParameters{
						ConfigMapName: configMapName,
						ConfigMapData: &acpv1.ConfigMapDataOptions{
							Type: "json",
							Key:  "filestyle.json",
						},
					},
					Configuration: acpv1.AzureAppConfigurationKeyValueOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &testKey,
							},
						},
						Refresh: &acpv1.DynamicConfigurationRefreshParameters{
							Enabled:  true,
							Interval: "45s",
						},
					},
					Secret: &acpv1.SecretReference{
						Target: acpv1.SecretGenerationParameters{
							SecretName: secretName,
						},
						Refresh: &acpv1.RefreshSettings{
							Interval: "1m",
							Enabled:  true,
						},
					},
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "40s",
							Enabled:  true,
						},
					},
				},
			}

			fakeResourceVersion := "1"
			nowTime := metav1.Now()
			processor := AppConfigurationProviderProcessor{
				Context:         ctx,
				Retriever:       mockConfigurationSettings,
				Provider:        configProvider,
				ShouldReconcile: false,
				Settings:        &loader.TargetKeyValueSettings{},
				ReconciliationState: &ReconciliationState{
					NextSecretReferenceRefreshReconcileTime: metav1.NewTime(nowTime.Time.Add(30 * time.Second)),
					NextKeyValueRefreshReconcileTime:        metav1.NewTime(nowTime.Time.Add(5 * time.Second)),
					NextFeatureFlagRefreshReconcileTime:     nowTime,
					KeyValueETags:                           existingKeyValueEtags,
					FeatureFlagETags:                        existingFeatureFlagEtags,
					ExistingK8sSecrets:                      cachedK8sSecrets,
					Generation:                              1,
					ConfigMapResourceVersion:                &fakeResourceVersion,
				},
				CurrentTime:    metav1.NewTime(nowTime.Time.Add(1 * time.Second)),
				RefreshOptions: &RefreshOptions{},
			}

			// only feature flag refresh
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettingsReturnedByFeatureFlagRefresh, nil)
			expectedNextFeatureFlagRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(40 * time.Second))
			cachedNextKeyValueRefreshReconcileTime := processor.ReconciliationState.NextKeyValueRefreshReconcileTime
			cachedNextSecretReferenceRefreshReconcileTime := processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.ExistingK8sSecrets).Should(Equal(cachedK8sSecrets))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(existingKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(cachedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(cachedNextSecretReferenceRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(expectedNextFeatureFlagRefreshReconcileTime))

			processor.CurrentTime = metav1.NewTime(processor.ReconciliationState.NextKeyValueRefreshReconcileTime.Time.Add(1 * time.Second))

			// only key value refresh
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettingsReturnedByKeyValueRefresh, nil)
			expectedNextKeyValueRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(45 * time.Second))
			cachedNextFeatureFlagRefreshReconcileTime := processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.ExistingK8sSecrets).Should(Equal(allSettingsReturnedByKeyValueRefresh.K8sSecrets))
			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(expectedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(cachedNextFeatureFlagRefreshReconcileTime))

			processor.CurrentTime = metav1.NewTime(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime.Time.Add(2 * time.Second))
			processor.RefreshOptions = &RefreshOptions{}

			allSettingsReturnedBySecretRefresh := &loader.TargetKeyValueSettings{
				SecretSettings: map[string]corev1.Secret{
					secretName: {
						Data: secretResult,
						Type: corev1.SecretType("Opaque"),
					},
				},
				K8sSecrets: map[string]*loader.TargetK8sSecretMetadata{
					secretName: {
						Type:                    corev1.SecretType("Opaque"),
						SecretsKeyVaultMetadata: secretMetadata,
					},
				},
			}

			// only secret refresh
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettingsReturnedBySecretRefresh, nil)
			expectedNextSecretReferenceRefreshReconcileTime := metav1.NewTime(processor.CurrentTime.Time.Add(1 * time.Minute))
			cachedNextKeyValueRefreshReconcileTime = processor.ReconciliationState.NextKeyValueRefreshReconcileTime
			cachedNextFeatureFlagRefreshReconcileTime = processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, existingSecrets)

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
			Expect(processor.ReconciliationState.NextSecretReferenceRefreshReconcileTime).Should(Equal(expectedNextSecretReferenceRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextKeyValueRefreshReconcileTime).Should(Equal(cachedNextKeyValueRefreshReconcileTime))
			Expect(processor.ReconciliationState.NextFeatureFlagRefreshReconcileTime).Should(Equal(cachedNextFeatureFlagRefreshReconcileTime))
		})
	})
})
