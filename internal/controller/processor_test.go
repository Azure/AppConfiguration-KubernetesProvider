// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package controller

import (
	"azappconfig/provider/internal/loader"
	"context"
	"time"

	acpv1 "azappconfig/provider/api/v1"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
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
		EndpointName = "https://fake-endpoint"
	)

	Context("Reconcile triggered by refresh scenarios", func() {
		It("Should update reconcile state when sentinel Etag updated", func() {
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
		})

		It("Should update reconcile state when configuration page Etag updated", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-config-etags"
			configMapName := "configmap-config-etags"
			testKey := "*"

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
					KeyValueETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &testKey,
						}: {
							&fakeEtag,
						},
					},
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.Now(),
				RefreshOptions: &RefreshOptions{},
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			updatedKeyValueEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &testKey,
				}: {
					&newFakeEtag,
				},
			}
			mapResult2 := make(map[string]string)
			mapResult2["filestyle.json"] = "{\"testKey\":\"newValue\"}"

			allSettings2 := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult2,
				KeyValueETags:     updatedKeyValueEtags,
			}

			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings2, nil)

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
				KeyFilter: &testKey,
			}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
				KeyFilter: &testKey,
			}]))
		})

		It("Should update reconcile state when featureFlag pageEtag updated", func() {
			ctx := context.Background()
			providerName := "test-appconfigurationprovider-ff-etags"
			configMapName := "configmap-ff-etags"
			wildcard := "*"

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
					FeatureFlag: &acpv1.AzureAppConfigurationFeatureFlagOptions{
						Selectors: []acpv1.Selector{
							{
								KeyFilter: &wildcard,
							},
						},
						Refresh: &acpv1.FeatureFlagRefreshSettings{
							Interval: "1s",
							Enabled:  true,
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
					NextFeatureFlagRefreshReconcileTime: metav1.Now(),
					FeatureFlagETags: map[acpv1.Selector][]*azcore.ETag{
						{
							KeyFilter: &wildcard,
						}: {
							&fakeEtag,
						},
					},
					Generation:               1,
					ConfigMapResourceVersion: &fakeResourceVersion,
				},
				CurrentTime:    metav1.Now(),
				RefreshOptions: &RefreshOptions{},
			}

			newFakeEtag := azcore.ETag("fake-etag-1")
			updatedFeatureFlagEtags := map[acpv1.Selector][]*azcore.ETag{
				{
					KeyFilter: &wildcard,
				}: {
					&newFakeEtag,
				},
			}
			mapResult := make(map[string]string)
			mapResult["filestyle.json"] = "{\"testKey\":\"testValue\",\"feature_management\":{\"feature_flags\":[{\"id\": \"testFeatureFlag\",\"enabled\": true,\"conditions\": {\"client_filters\": []}}]}}"

			allSettings := &loader.TargetKeyValueSettings{
				ConfigMapSettings: mapResult,
				FeatureFlagETags:  updatedFeatureFlagEtags,
			}

			//Feature flag Etag is updated
			mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
			mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)

			_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeResourceVersion,
			}}, make(map[string]corev1.Secret))

			_, _ = processor.Finish()

			Expect(processor.ReconciliationState.FeatureFlagETags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]).Should(Equal(updatedFeatureFlagEtags[acpv1.Selector{
				KeyFilter: &wildcard,
			}]))
		})

		// 4 scenarios: Both Etags updated, only keyValue Etag updated, only featureFlag Etag updated, both Etags not updated
		It("Should update reconcile state when featureFlag pageEtag and keyValue pageEtag updated", func() {
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
	})

	It("Should update reconcile state when secret references updated", func() {
		ctx := context.Background()
		providerName := "test-appconfigurationprovider-secret"
		configMapName := "configmap-test"
		secretName := "secret-test"
		fakeSecretResourceVersion := "1"

		secretResult := make(map[string][]byte)
		secretResult["testSecretKey"] = []byte("testSecretValue")
		existingSecrets := make(map[string]corev1.Secret)
		existingSecrets[secretName] = corev1.Secret{
			Data: secretResult,
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeSecretResourceVersion,
			},
		}

		var fakeId azsecrets.ID = "fakeSecretId"
		var cachedFakeId azsecrets.ID = "cachedFakeSecretId"

		secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
		secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
		cachedSecretReferences := make(map[string]*loader.TargetSecretReference)
		secretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &fakeId,
		}
		secretMetadata2["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &cachedFakeId,
		}
		cachedSecretReferences[secretName] = &loader.TargetSecretReference{
			Type:                  corev1.SecretType("Opaque"),
			SecretsMetadata:       secretMetadata2,
			SecretResourceVersion: fakeSecretResourceVersion,
		}

		allSettings := &loader.TargetKeyValueSettings{
			SecretSettings: map[string]corev1.Secret{
				secretName: {
					Data: secretResult,
					Type: corev1.SecretType("Opaque"),
				},
			},
			SecretReferences: map[string]*loader.TargetSecretReference{
				secretName: {
					Type:            corev1.SecretType("Opaque"),
					SecretsMetadata: secretMetadata,
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
		tmpTime := metav1.Now()
		processor := AppConfigurationProviderProcessor{
			Context:         ctx,
			Retriever:       mockConfigurationSettings,
			Provider:        configProvider,
			ShouldReconcile: false,
			Settings:        &loader.TargetKeyValueSettings{},
			ReconciliationState: &ReconciliationState{
				NextSecretReferenceRefreshReconcileTime: tmpTime,
				ExistingSecretReferences:                cachedSecretReferences,
				Generation:                              1,
				ConfigMapResourceVersion:                &fakeResourceVersion,
			},
			CurrentTime:    metav1.NewTime(tmpTime.Time.Add(1 * time.Minute)),
			RefreshOptions: &RefreshOptions{},
		}

		mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

		_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: fakeResourceVersion,
		}}, existingSecrets)

		_, _ = processor.Finish()

		Expect(processor.ReconciliationState.ExistingSecretReferences).Should(Equal(allSettings.SecretReferences))
	})

	It("Secret refresh can work with multiple version secrets", func() {
		ctx := context.Background()
		providerName := "test-appconfigurationprovider-secret-2"
		configMapName := "configmap-test-2"
		secretName := "secret-test-2"
		fakeSecretResourceVersion := "1"

		secretResult := make(map[string][]byte)
		secretResult["testSecretKey"] = []byte("testSecretValue")
		secretResult["testSecretKey2"] = []byte("testSecretValue2")
		existingSecrets := make(map[string]corev1.Secret)
		existingSecrets[secretName] = corev1.Secret{
			Data: secretResult,
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: fakeSecretResourceVersion,
			},
		}

		var fakeId azsecrets.ID = "fakeSecretId"
		var cachedFakeId azsecrets.ID = "cachedFakeSecretId"

		secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
		secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
		cachedSecretReferences := make(map[string]*loader.TargetSecretReference)
		// multiple version secrets
		secretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId:      &fakeId,
			SecretVersion: "",
		}
		secretMetadata2["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId:      &cachedFakeId,
			SecretVersion: "",
		}
		secretMetadata2["testSecretKey2"] = loader.KeyVaultSecretMetadata{
			SecretId:      &cachedFakeId,
			SecretVersion: "fakeVersion",
		}
		cachedSecretReferences[secretName] = &loader.TargetSecretReference{
			Type:                  corev1.SecretType("Opaque"),
			SecretsMetadata:       secretMetadata2,
			SecretResourceVersion: fakeSecretResourceVersion,
		}

		allSettings := &loader.TargetKeyValueSettings{
			SecretSettings: map[string]corev1.Secret{
				secretName: {
					Data: secretResult,
					Type: corev1.SecretType("Opaque"),
				},
			},
			SecretReferences: map[string]*loader.TargetSecretReference{
				secretName: {
					Type:            corev1.SecretType("Opaque"),
					SecretsMetadata: secretMetadata,
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
		tmpTime := metav1.Now()
		processor := AppConfigurationProviderProcessor{
			Context:         ctx,
			Retriever:       mockConfigurationSettings,
			Provider:        configProvider,
			ShouldReconcile: false,
			Settings:        &loader.TargetKeyValueSettings{},
			ReconciliationState: &ReconciliationState{
				NextSecretReferenceRefreshReconcileTime: tmpTime,
				ExistingSecretReferences:                cachedSecretReferences,
				Generation:                              1,
				ConfigMapResourceVersion:                &fakeResourceVersion,
			},
			CurrentTime:    metav1.NewTime(tmpTime.Time.Add(1 * time.Minute)),
			RefreshOptions: &RefreshOptions{},
		}

		// Only resolve non-version Key Vault references
		mockConfigurationSettings.EXPECT().ResolveSecretReferences(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

		_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: fakeResourceVersion,
		}}, existingSecrets)

		_, _ = processor.Finish()

		Expect(processor.ReconciliationState.ExistingSecretReferences[secretName].SecretsMetadata["testSecretKey"]).Should(Equal(allSettings.SecretReferences[secretName].SecretsMetadata["testSecretKey"]))
		Expect(processor.ReconciliationState.ExistingSecretReferences[secretName].SecretsMetadata["testSecretKey2"]).Should(Equal(cachedSecretReferences[secretName].SecretsMetadata["testSecretKey2"]))
	})

	It("Should update reconcile state when keyValue pageEtag and secret references updated", func() {
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

		var fakeId azsecrets.ID = "fakeSecretId"
		var cachedFakeId azsecrets.ID = "cachedFakeSecretId"

		secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
		secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
		cachedSecretReferences := make(map[string]*loader.TargetSecretReference)
		secretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &fakeId,
		}
		secretMetadata2["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &cachedFakeId,
		}
		cachedSecretReferences[secretName] = &loader.TargetSecretReference{
			Type:                  corev1.SecretType("Opaque"),
			SecretsMetadata:       secretMetadata2,
			SecretResourceVersion: fakeSecretResourceVersion,
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
			SecretReferences: map[string]*loader.TargetSecretReference{
				secretName: {
					Type:            corev1.SecretType("Opaque"),
					SecretsMetadata: secretMetadata,
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
				ExistingSecretReferences: cachedSecretReferences,
				Generation:               1,
				ConfigMapResourceVersion: &fakeResourceVersion,
			},
			CurrentTime:    metav1.NewTime(nowTime.Time.Add(1 * time.Minute)),
			RefreshOptions: &RefreshOptions{},
		}

		mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
		mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

		_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: fakeResourceVersion,
		}}, existingSecrets)

		_, _ = processor.Finish()

		Expect(processor.ReconciliationState.ExistingSecretReferences).Should(Equal(allSettings.SecretReferences))
		Expect(processor.ReconciliationState.KeyValueETags[acpv1.Selector{
			KeyFilter: &testKey,
		}]).Should(Equal(updatedKeyValueEtags[acpv1.Selector{
			KeyFilter: &testKey,
		}]))
	})

	It("Should update reconcile state when keyValue pageEtag, featureFlag pageEtag and secret references updated", func() {
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

		var fakeId azsecrets.ID = "fakeSecretId"
		var cachedFakeId azsecrets.ID = "cachedFakeSecretId"

		secretMetadata := make(map[string]loader.KeyVaultSecretMetadata)
		secretMetadata2 := make(map[string]loader.KeyVaultSecretMetadata)
		cachedSecretReferences := make(map[string]*loader.TargetSecretReference)
		secretMetadata["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &fakeId,
		}
		secretMetadata2["testSecretKey"] = loader.KeyVaultSecretMetadata{
			SecretId: &cachedFakeId,
		}
		cachedSecretReferences[secretName] = &loader.TargetSecretReference{
			Type:                  corev1.SecretType("Opaque"),
			SecretsMetadata:       secretMetadata2,
			SecretResourceVersion: fakeSecretResourceVersion,
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

		allSettings := &loader.TargetKeyValueSettings{
			ConfigMapSettings: mapResult,
			KeyValueETags:     updatedKeyValueEtags,
			FeatureFlagETags:  updatedFeatureFlagEtags,
			SecretSettings: map[string]corev1.Secret{
				secretName: {
					Data: secretResult,
					Type: corev1.SecretType("Opaque"),
				},
			},
			SecretReferences: map[string]*loader.TargetSecretReference{
				secretName: {
					Type:            corev1.SecretType("Opaque"),
					SecretsMetadata: secretMetadata,
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
						Interval: "1m",
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
						Interval: "1m",
						Enabled:  true,
					},
				},
			},
		}

		fakeResourceVersion := "1"
		fakeEtag := azcore.ETag("fake-etag")
		fakeEtag2 := azcore.ETag("fake-etag2")
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
				NextFeatureFlagRefreshReconcileTime:     nowTime,
				KeyValueETags: map[acpv1.Selector][]*azcore.ETag{
					{
						KeyFilter: &testKey,
					}: {
						&fakeEtag,
					},
				},
				FeatureFlagETags: map[acpv1.Selector][]*azcore.ETag{
					{
						KeyFilter: &wildcard,
					}: {
						&fakeEtag2,
					},
				},
				ExistingSecretReferences: cachedSecretReferences,
				Generation:               1,
				ConfigMapResourceVersion: &fakeResourceVersion,
			},
			CurrentTime:    metav1.NewTime(nowTime.Time.Add(1 * time.Minute)),
			RefreshOptions: &RefreshOptions{},
		}

		mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
		mockConfigurationSettings.EXPECT().RefreshFeatureFlagSettings(gomock.Any(), gomock.Any()).Return(allSettings, nil)
		mockConfigurationSettings.EXPECT().CheckPageETags(gomock.Any(), gomock.Any()).Return(true, nil)
		mockConfigurationSettings.EXPECT().RefreshKeyValueSettings(gomock.Any(), gomock.Any(), gomock.Any()).Return(allSettings, nil)

		_ = processor.PopulateSettings(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: fakeResourceVersion,
		}}, existingSecrets)

		_, _ = processor.Finish()

		Expect(processor.ReconciliationState.ExistingSecretReferences).Should(Equal(allSettings.SecretReferences))
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
	})
})
